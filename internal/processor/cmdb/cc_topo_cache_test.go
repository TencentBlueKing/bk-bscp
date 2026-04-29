/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmdb

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
)

type countingObjectAttrCMDB struct {
	bkcmdb.Service
	mu    sync.Mutex
	calls map[string]int
	delay time.Duration
}

func newCountingObjectAttrCMDB() *countingObjectAttrCMDB {
	return &countingObjectAttrCMDB{calls: make(map[string]int)}
}

func (m *countingObjectAttrCMDB) SearchObjectAttr(_ context.Context,
	req bkcmdb.SearchObjectAttrReq) ([]bkcmdb.ObjectAttrInfo, error) {
	m.mu.Lock()
	m.calls[req.BkObjID]++
	m.mu.Unlock()

	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	return []bkcmdb.ObjectAttrInfo{
		{
			BkBizID:             req.BkBizID,
			BkObjID:             req.BkObjID,
			BkPropertyID:        req.BkObjID + "_custom",
			BkPropertyName:      req.BkObjID + " custom",
			BkPropertyGroupName: "custom",
			BkPropertyType:      "singlechar",
		},
	}, nil
}

func (m *countingObjectAttrCMDB) callCount(objID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls[objID]
}

type panicTopoCMDB struct {
	bkcmdb.Service
}

func (m *panicTopoCMDB) FindTopoBrief(_ context.Context, _ int) (*bkcmdb.TopoBriefResp, error) {
	panic("FindTopoBrief should not be called when topo XML cache hits")
}

type ttlRenderCache struct {
	RenderCache
	ttl time.Duration
}

func (c ttlRenderCache) BuildLockTTL() time.Duration {
	return c.ttl
}

func TestCacheBuildWaitTTLHonorsConfiguredLockTTL(t *testing.T) {
	const configuredTTL = 30 * time.Second

	if got := cacheBuildWaitTTL(ttlRenderCache{ttl: configuredTTL}); got != configuredTTL {
		t.Fatalf("cacheBuildWaitTTL = %v, want %v", got, configuredTTL)
	}
}

func TestCCTopoXMLService_GetTopoTreeXMLUsesRenderCache(t *testing.T) {
	const (
		tenantID = "tenant-a"
		bizID    = 42
		setEnv   = "3"
		xml      = `<?xml version="1.0" encoding="UTF-8"?><Application></Application>`
	)
	cache := NewMemoryCMDBRenderCache()
	cache.SetTopoXML(context.Background(), tenantID, bizID, setEnv, xml)

	svc := NewCCTopoXMLServiceWithTenant(tenantID, bizID, &panicTopoCMDB{}, cache)
	got, err := svc.GetTopoTreeXML(context.Background(), setEnv)
	if err != nil {
		t.Fatalf("GetTopoTreeXML failed: %v", err)
	}
	if got != xml {
		t.Fatalf("GetTopoTreeXML = %q, want %q", got, xml)
	}
}

func TestCCTopoXMLService_GetBizObjectAttributesUsesRenderCache(t *testing.T) {
	const (
		tenantID = "tenant-a"
		bizID    = 42
	)
	cache := NewMemoryCMDBRenderCache()
	cache.SetBizObjectAttributes(context.Background(), tenantID, bizID, map[string][]ObjectAttribute{
		BK_SET_OBJ_ID: {
			{BkPropertyID: "set_cached", BkPropertyName: "set cached", BkObjID: BK_SET_OBJ_ID, BkBizID: bizID},
		},
	})

	mockSvc := newCountingObjectAttrCMDB()
	svc := NewCCTopoXMLServiceWithTenant(tenantID, bizID, mockSvc, cache)
	attrs, err := svc.GetBizObjectAttributes(context.Background())
	if err != nil {
		t.Fatalf("GetBizObjectAttributes failed: %v", err)
	}

	if got := attrs[BK_SET_OBJ_ID][0].BkPropertyID; got != "set_cached" {
		t.Fatalf("cached set attr = %q, want set_cached", got)
	}
	for _, objID := range []string{BK_SET_OBJ_ID, BK_MODULE_OBJ_ID, BK_HOST_OBJ_ID} {
		if got := mockSvc.callCount(objID); got != 0 {
			t.Fatalf("SearchObjectAttr for %s called %d times, want 0", objID, got)
		}
	}
}

func TestCCTopoXMLService_GetBizObjectAttributesStoresRenderCache(t *testing.T) {
	const (
		tenantID = "tenant-a"
		bizID    = 42
	)
	cache := NewMemoryCMDBRenderCache()
	mockSvc := newCountingObjectAttrCMDB()
	svc := NewCCTopoXMLServiceWithTenant(tenantID, bizID, mockSvc, cache)

	if _, err := svc.GetBizGlobalVariablesMap(context.Background()); err != nil {
		t.Fatalf("GetBizGlobalVariablesMap failed: %v", err)
	}

	for _, objID := range []string{BK_SET_OBJ_ID, BK_MODULE_OBJ_ID, BK_HOST_OBJ_ID} {
		if got := mockSvc.callCount(objID); got != 1 {
			t.Fatalf("SearchObjectAttr for %s called %d times, want 1", objID, got)
		}
	}

	second := NewCCTopoXMLServiceWithTenant(tenantID, bizID, mockSvc, cache)
	if _, err := second.GetBizGlobalVariablesMap(context.Background()); err != nil {
		t.Fatalf("second GetBizGlobalVariablesMap failed: %v", err)
	}

	for _, objID := range []string{BK_SET_OBJ_ID, BK_MODULE_OBJ_ID, BK_HOST_OBJ_ID} {
		if got := mockSvc.callCount(objID); got != 1 {
			t.Fatalf("cached SearchObjectAttr for %s called %d times, want 1", objID, got)
		}
	}
}

func TestCCTopoXMLService_GetBizObjectAttributesCoalescesConcurrentCacheMiss(t *testing.T) {
	const (
		tenantID = "tenant-a"
		bizID    = 42
		workers  = 8
	)
	cache := NewMemoryCMDBRenderCache()
	mockSvc := newCountingObjectAttrCMDB()
	mockSvc.delay = 20 * time.Millisecond

	start := make(chan struct{})
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-start
			svc := NewCCTopoXMLServiceWithTenant(tenantID, bizID, mockSvc, cache)
			_, err := svc.GetBizObjectAttributes(context.Background())
			errs <- err
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("GetBizObjectAttributes failed: %v", err)
		}
	}

	for _, objID := range []string{BK_SET_OBJ_ID, BK_MODULE_OBJ_ID, BK_HOST_OBJ_ID} {
		if got := mockSvc.callCount(objID); got != 1 {
			t.Fatalf("concurrent SearchObjectAttr for %s called %d times, want 1", objID, got)
		}
	}
}
