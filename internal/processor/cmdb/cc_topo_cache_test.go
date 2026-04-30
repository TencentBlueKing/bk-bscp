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
	"errors"
	"sync"
	"testing"
	"time"

	"golang.org/x/sync/singleflight"

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

type cancelAwareObjectAttrCMDB struct {
	bkcmdb.Service
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newCancelAwareObjectAttrCMDB() *cancelAwareObjectAttrCMDB {
	return &cancelAwareObjectAttrCMDB{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (m *cancelAwareObjectAttrCMDB) SearchObjectAttr(ctx context.Context,
	req bkcmdb.SearchObjectAttrReq) ([]bkcmdb.ObjectAttrInfo, error) {
	m.once.Do(func() {
		close(m.started)
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-m.release:
		return []bkcmdb.ObjectAttrInfo{
			{
				BkBizID:        req.BkBizID,
				BkObjID:        req.BkObjID,
				BkPropertyID:   req.BkObjID + "_custom",
				BkPropertyName: req.BkObjID + " custom",
			},
		}, nil
	}
}

type reacquirableRenderCache struct {
	RenderCache
	released chan struct{}
	ttl      time.Duration

	mu           sync.Mutex
	acquireCount int
}

func newReacquirableRenderCache(ttl time.Duration) *reacquirableRenderCache {
	return &reacquirableRenderCache{
		released: make(chan struct{}),
		ttl:      ttl,
	}
}

func (c *reacquirableRenderCache) GetTopoXML(_ context.Context, _ string, _ int, _ string) (string, bool) {
	return "", false
}

func (c *reacquirableRenderCache) GetBizObjectAttributes(
	_ context.Context, _ string, _ int) (map[string][]ObjectAttribute, bool) {
	return nil, false
}

func (c *reacquirableRenderCache) AcquireBuildLock(
	_ context.Context, _ string, _ int, _ string, _ string) (bool, error) {
	c.mu.Lock()
	c.acquireCount++
	c.mu.Unlock()

	select {
	case <-c.released:
		return true, nil
	default:
		return false, nil
	}
}

func (c *reacquirableRenderCache) BuildLockTTL() time.Duration {
	return c.ttl
}

func (c *reacquirableRenderCache) countAcquireBuildLock() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.acquireCount
}

type ttlRenderCache struct {
	RenderCache
	ttl time.Duration
}

func (c ttlRenderCache) BuildLockTTL() time.Duration {
	return c.ttl
}

type lockedRenderCache struct {
	RenderCache
	buildLockTTL time.Duration
	buildWaitTTL time.Duration
	buildTimeout time.Duration
	renewCount   int
	mu           sync.Mutex
}

func (c lockedRenderCache) GetTopoXML(_ context.Context, _ string, _ int, _ string) (string, bool) {
	return "", false
}

func (c lockedRenderCache) SetTopoXML(_ context.Context, _ string, _ int, _ string, _ string) {
}

func (c lockedRenderCache) GetBizObjectAttributes(
	_ context.Context, _ string, _ int) (map[string][]ObjectAttribute, bool) {
	return nil, false
}

func (c lockedRenderCache) SetBizObjectAttributes(_ context.Context, _ string, _ int, _ map[string][]ObjectAttribute) {
}

func (c lockedRenderCache) InvalidateBiz(_ context.Context, _ string, _ int) error {
	return nil
}

func (c lockedRenderCache) AcquireBuildLock(_ context.Context, _ string, _ int, _ string, _ string) (bool, error) {
	return false, nil
}

func (c lockedRenderCache) ReleaseBuildLock(_ context.Context, _ string, _ int, _ string, _ string) error {
	return nil
}

func (c lockedRenderCache) BuildLockTTL() time.Duration {
	return c.buildLockTTL
}

func (c lockedRenderCache) BuildWaitTTL() time.Duration {
	return c.buildWaitTTL
}

func (c lockedRenderCache) BuildTimeout() time.Duration {
	return c.buildTimeout
}

func (c *lockedRenderCache) RenewBuildLock(_ context.Context, _ string, _ int, _ string, _ string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.renewCount++
	return true, nil
}

func (c *lockedRenderCache) countRenewBuildLock() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.renewCount
}

type longBuildRenewCache struct {
	RenderCache
	buildLockTTL time.Duration
	buildWaitTTL time.Duration
	buildTimeout time.Duration

	mu         sync.Mutex
	renewCount int
}

func (c *longBuildRenewCache) AcquireBuildLock(_ context.Context, _ string, _ int, _ string, _ string) (bool, error) {
	return true, nil
}

func (c *longBuildRenewCache) ReleaseBuildLock(_ context.Context, _ string, _ int, _ string, _ string) error {
	return nil
}

func (c *longBuildRenewCache) RenewBuildLock(_ context.Context, _ string, _ int, _ string, _ string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.renewCount++
	return true, nil
}

func (c *longBuildRenewCache) BuildLockTTL() time.Duration {
	return c.buildLockTTL
}

func (c *longBuildRenewCache) BuildWaitTTL() time.Duration {
	return c.buildWaitTTL
}

func (c *longBuildRenewCache) BuildTimeout() time.Duration {
	return c.buildTimeout
}

func (c *longBuildRenewCache) countRenewBuildLock() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.renewCount
}

func TestCacheBuildWaitTTLHonorsConfiguredLockTTL(t *testing.T) {
	const configuredTTL = 30 * time.Second

	if got := cacheBuildWaitTTL(ttlRenderCache{ttl: configuredTTL}); got != configuredTTL {
		t.Fatalf("cacheBuildWaitTTL = %v, want %v", got, configuredTTL)
	}
}

func TestCCTopoXMLService_WaitTimeoutDoesNotBuildWithoutLock(t *testing.T) {
	const (
		tenantID = "tenant-a"
		bizID    = 42
	)
	cache := &lockedRenderCache{
		buildLockTTL: 200 * time.Millisecond,
		buildWaitTTL: 30 * time.Millisecond,
		buildTimeout: 200 * time.Millisecond,
	}
	mockSvc := newCountingObjectAttrCMDB()
	svc := NewCCTopoXMLServiceWithTenant(tenantID, bizID, mockSvc, cache)

	if _, err := svc.GetBizObjectAttributes(context.Background()); err == nil {
		t.Fatal("GetBizObjectAttributes should fail when waiting for existing builder times out")
	}
	for _, objID := range []string{BK_SET_OBJ_ID, BK_MODULE_OBJ_ID, BK_HOST_OBJ_ID} {
		if got := mockSvc.callCount(objID); got != 0 {
			t.Fatalf("SearchObjectAttr for %s called %d times, want 0", objID, got)
		}
	}
}

func TestCCTopoXMLService_RenewsBuildLockDuringLongBuild(t *testing.T) {
	const (
		tenantID = "tenant-a"
		bizID    = 42
	)
	cache := NewMemoryCMDBRenderCache()
	lockCache := &longBuildRenewCache{
		RenderCache:  cache,
		buildLockTTL: 30 * time.Millisecond,
		buildWaitTTL: 30 * time.Millisecond,
		buildTimeout: 300 * time.Millisecond,
	}
	mockSvc := newCountingObjectAttrCMDB()
	mockSvc.delay = 25 * time.Millisecond
	svc := NewCCTopoXMLServiceWithTenant(tenantID, bizID, mockSvc, lockCache)

	if _, err := svc.GetBizObjectAttributes(context.Background()); err != nil {
		t.Fatalf("GetBizObjectAttributes failed: %v", err)
	}
	if got := lockCache.countRenewBuildLock(); got == 0 {
		t.Fatal("RenewBuildLock should be called during long cache build")
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

func TestCCTopoXMLService_GetBizObjectAttributesReleasesBuildLock(t *testing.T) {
	const (
		tenantID = "tenant-a"
		bizID    = 42
	)
	store := newFakeRenderCacheStore()
	cache := newRedisCMDBRenderCacheWithStore(store, DefaultRenderCacheOptions())
	mockSvc := newCountingObjectAttrCMDB()
	svc := NewCCTopoXMLServiceWithTenant(tenantID, bizID, mockSvc, cache)

	if _, err := svc.GetBizObjectAttributes(context.Background()); err != nil {
		t.Fatalf("GetBizObjectAttributes failed: %v", err)
	}

	locked, err := cache.AcquireBuildLock(context.Background(), tenantID, bizID, renderCacheKindBizGlobalVariables, "")
	if err != nil {
		t.Fatalf("AcquireBuildLock failed: %v", err)
	}
	if !locked {
		t.Fatal("build lock should be released after GetBizObjectAttributes builds cache")
	}
}

func TestCCTopoXMLService_GetBizObjectAttributesLeaderCancelDoesNotFailFollower(t *testing.T) {
	const (
		tenantID = "tenant-a"
		bizID    = 42
	)
	renderCacheFlight = singleflight.Group{}
	t.Cleanup(func() {
		renderCacheFlight = singleflight.Group{}
	})

	cache := NewMemoryCMDBRenderCache()
	mockSvc := newCancelAwareObjectAttrCMDB()
	leader := NewCCTopoXMLServiceWithTenant(tenantID, bizID, mockSvc, cache)
	follower := NewCCTopoXMLServiceWithTenant(tenantID, bizID, mockSvc, cache)

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leaderErr := make(chan error, 1)
	go func() {
		_, err := leader.GetBizObjectAttributes(leaderCtx)
		leaderErr <- err
	}()

	<-mockSvc.started

	followerErr := make(chan error, 1)
	go func() {
		_, err := follower.GetBizObjectAttributes(context.Background())
		followerErr <- err
	}()

	time.Sleep(10 * time.Millisecond)
	cancelLeader()

	if err := <-leaderErr; !errors.Is(err, context.Canceled) {
		t.Fatalf("leader GetBizObjectAttributes err = %v, want context.Canceled", err)
	}
	close(mockSvc.release)

	if err := <-followerErr; err != nil {
		t.Fatalf("follower GetBizObjectAttributes failed: %v", err)
	}
}

func TestDoRenderCacheFlightCancelsBuildAtConfiguredTimeout(t *testing.T) {
	renderCacheFlight = singleflight.Group{}
	t.Cleanup(func() {
		renderCacheFlight = singleflight.Group{}
	})

	const buildTimeout = 30 * time.Millisecond
	started := make(chan struct{})
	start := time.Now()
	value, err := doRenderCacheFlight(context.Background(), "build-timeout", buildTimeout,
		func(buildCtx context.Context) (interface{}, error) {
			close(started)
			<-buildCtx.Done()
			return nil, buildCtx.Err()
		})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("doRenderCacheFlight err = %v, want context.DeadlineExceeded", err)
	}
	if value != nil {
		t.Fatalf("doRenderCacheFlight value = %v, want nil", value)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("doRenderCacheFlight elapsed = %v, want bounded by build timeout", elapsed)
	}
	select {
	case <-started:
	default:
		t.Fatal("singleflight build function was not called")
	}
}

func TestCCTopoXMLService_WaitBizObjectAttributesCacheReacquiresReleasedLock(t *testing.T) {
	const (
		tenantID = "tenant-a"
		bizID    = 42
	)
	cache := newReacquirableRenderCache(300 * time.Millisecond)
	svc := NewCCTopoXMLServiceWithTenant(tenantID, bizID, newCountingObjectAttrCMDB(), cache)

	go func() {
		time.Sleep(20 * time.Millisecond)
		close(cache.released)
	}()

	cacheReady, lockAcquired := svc.acquireOrWaitBizObjectAttributesCache(context.Background())
	if !cacheReady || !lockAcquired {
		t.Fatalf("acquireOrWaitBizObjectAttributesCache = (%v, %v), want (true, true)",
			cacheReady, lockAcquired)
	}
	if got := cache.countAcquireBuildLock(); got < 2 {
		t.Fatalf("AcquireBuildLock calls = %d, want at least 2", got)
	}
}

func TestCCTopoXMLService_WaitTopoCacheReacquiresReleasedLock(t *testing.T) {
	const (
		tenantID = "tenant-a"
		bizID    = 42
		setEnv   = "3"
	)
	cache := newReacquirableRenderCache(300 * time.Millisecond)
	svc := NewCCTopoXMLServiceWithTenant(tenantID, bizID, newCountingObjectAttrCMDB(), cache)

	go func() {
		time.Sleep(20 * time.Millisecond)
		close(cache.released)
	}()

	cacheReady, lockAcquired := svc.acquireOrWaitTopoCache(context.Background(), setEnv)
	if !cacheReady || !lockAcquired {
		t.Fatalf("acquireOrWaitTopoCache = (%v, %v), want (true, true)", cacheReady, lockAcquired)
	}
	if got := cache.countAcquireBuildLock(); got < 2 {
		t.Fatalf("AcquireBuildLock calls = %d, want at least 2", got)
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
