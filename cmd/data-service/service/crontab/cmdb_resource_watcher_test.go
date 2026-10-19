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

package crontab

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/processor/cmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

type watcherTestDAO struct {
	dao.Set
	config  dao.Config
	process dao.Process
}

func (d watcherTestDAO) Config() dao.Config   { return d.config }
func (d watcherTestDAO) Process() dao.Process { return d.process }

type watcherTestConfig struct {
	dao.Config
	get func(*kit.Kit, string) (*table.Config, error)
	put func(*kit.Kit, []*table.Config) error
}

func (d watcherTestConfig) GetConfig(kt *kit.Kit, key string) (*table.Config, error) {
	return d.get(kt, key)
}
func (d watcherTestConfig) UpsertConfig(kt *kit.Kit, values []*table.Config) error {
	return d.put(kt, values)
}

type watcherTestProcess struct {
	dao.Process
	get func(*kit.Kit, uint32, uint32, uint32) (*table.Process, error)
}

func (d watcherTestProcess) GetProcByBizScvProc(kt *kit.Kit, bizID, svcID, procID uint32) (*table.Process, error) {
	return d.get(kt, bizID, svcID, procID)
}

type watcherTestCMDB struct {
	bkcmdb.Service
	watch func(context.Context, *bkcmdb.WatchResourceRequest) (*bkcmdb.WatchData, error)
}

func (c watcherTestCMDB) ResourceWatchLongPolling(ctx context.Context, req *bkcmdb.WatchResourceRequest,
	_ time.Duration) (*bkcmdb.WatchData, error) {
	return c.watch(ctx, req)
}

func TestWatcherHeartbeatDoesNotCommitPastFailedPage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	commits := make(chan string, 8)
	blocked := make(chan struct{})
	client := watcherTestCMDB{watch: func(ctx context.Context, req *bkcmdb.WatchResourceRequest) (*bkcmdb.WatchData, error) {
		switch req.BkCursor {
		case "saved":
			return &bkcmdb.WatchData{BkEvents: []bkcmdb.BkEventObj{{
				BkResource: bkcmdb.ResourceProcess, BkEventType: bkcmdb.EventDelete, BkCursor: "event",
				BkDetail: json.RawMessage(`{"bk_biz_id":1,"bk_process_id":2,"service_instance_id":3}`),
			}}}, nil
		case "event":
			return &bkcmdb.WatchData{BkEvents: []bkcmdb.BkEventObj{{BkCursor: "heartbeat"}}}, nil
		default:
			close(blocked)
			<-ctx.Done()
			return nil, ctx.Err()
		}
	}}
	config := watcherTestConfig{
		get: func(*kit.Kit, string) (*table.Config, error) { return &table.Config{Value: "saved"}, nil },
		put: func(_ *kit.Kit, values []*table.Config) error { commits <- values[0].Value; return nil },
	}
	process := watcherTestProcess{get: func(kt *kit.Kit, _, _, _ uint32) (*table.Process, error) {
		<-kt.Ctx.Done()
		return nil, kt.Ctx.Err()
	}}
	c := NewCmdbResourceWatcher(watcherTestDAO{config: config, process: process}, nil, client, nil, nil, nil,
		cc.WatchCmdbResourceConfig{QpsLimit: 1000, IdleInterval: "1ns"})
	done := make(chan struct{})
	go func() { defer close(done); c.runWatchChain(ctx, watchChain{resource: bkcmdb.ResourceProcess}) }()
	select {
	case <-blocked:
	case <-time.After(3 * time.Second):
		t.Fatal("watch did not reach heartbeat")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("chain did not stop")
	}
	select {
	case cursor := <-commits:
		t.Fatalf("committed %q past failed page", cursor)
	default:
	}
}

func TestWatcherRetriesCursorBeforeNextPage(t *testing.T) {
	var attempts []string
	config := watcherTestConfig{put: func(_ *kit.Kit, values []*table.Config) error {
		attempts = append(attempts, values[0].Value)
		if len(attempts) == 1 {
			return errors.New("database unavailable")
		}
		return nil
	}}
	c := &cmdbResourceWatcher{dao: watcherTestDAO{config: config}}
	pages := make(chan watchPage, 2)
	pages <- watchPage{cursor: "first"}
	pages <- watchPage{cursor: "second"}
	close(pages)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.processPages(ctx, watchChain{resource: bkcmdb.ResourceSet}, kit.New(), pages)
	if !reflect.DeepEqual(attempts, []string{"first", "first", "second"}) {
		t.Fatalf("commit attempts: %v", attempts)
	}
}

func TestWatcherKeepsProcessEventOrder(t *testing.T) {
	firstErr := errors.New("delete lookup failed")
	process := watcherTestProcess{get: func(*kit.Kit, uint32, uint32, uint32) (*table.Process, error) { return nil, firstErr }}
	c := &cmdbResourceWatcher{dao: watcherTestDAO{process: process}}
	events := []bkcmdb.BkEventObj{
		{BkResource: bkcmdb.ResourceProcess, BkEventType: bkcmdb.EventDelete,
			BkDetail: json.RawMessage(`{"bk_biz_id":1,"bk_process_id":2,"service_instance_id":3}`)},
		{BkResource: bkcmdb.ResourceProcess, BkEventType: bkcmdb.EventCreate, BkDetail: json.RawMessage(`{`)},
	}
	if err := c.processPage(watchChain{resource: bkcmdb.ResourceProcess}, kit.New(), events); !errors.Is(err, firstErr) {
		t.Fatalf("first event error = %v, want %v", err, firstErr)
	}
}

func TestWatcherLoadCursorPreservesDatabaseErrors(t *testing.T) {
	dbErr := errors.New("database unavailable")
	for _, want := range []error{dbErr, gorm.ErrRecordNotFound} {
		c := &cmdbResourceWatcher{dao: watcherTestDAO{config: watcherTestConfig{
			get: func(*kit.Kit, string) (*table.Config, error) { return nil, want },
		}}}
		_, err := c.loadCursor(kit.New(), "cursor")
		if want == dbErr && !errors.Is(err, dbErr) {
			t.Fatalf("error = %v", err)
		}
		if want == gorm.ErrRecordNotFound && err != nil {
			t.Fatalf("missing cursor error = %v", err)
		}
	}
}

func TestWatcherRejectsInvalidProcessIdentity(t *testing.T) {
	for _, detail := range []string{`{`, `null`, `{}`,
		`{"bk_biz_id":-1,"bk_process_id":2,"service_instance_id":3}`,
		`{"bk_biz_id":4294967297,"bk_process_id":2,"service_instance_id":3}`,
	} {
		_, err := groupProcessEventsByBiz([]bkcmdb.BkEventObj{{BkDetail: json.RawMessage(detail)}})
		if err == nil {
			t.Fatalf("accepted invalid detail %s", detail)
		}
	}
}

func TestWatcherUpdateProcessPreservesDatabaseErrors(t *testing.T) {
	dbErr := errors.New("database unavailable")
	process := watcherTestProcess{get: func(*kit.Kit, uint32, uint32, uint32) (*table.Process, error) { return nil, dbErr }}
	svc := cmdb.NewSyncCMDBService("tenant", 1, nil, nil, watcherTestDAO{process: process})
	kt := kit.New()
	_, err := svc.UpdateProcess(kit.WithKit(kt.Ctx, kt), []bkcmdb.ProcessInfo{
		{BkBizID: 1, BkProcessID: 2, ServiceInstanceID: 3},
	})
	if !errors.Is(err, dbErr) {
		t.Fatalf("update error = %v, want %v", err, dbErr)
	}
}

func TestWatcherDoesNotSkipFailedPageAfterRetryThreshold(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	attempts, commits := 0, 0
	c := &cmdbResourceWatcher{dao: watcherTestDAO{
		config: watcherTestConfig{put: func(*kit.Kit, []*table.Config) error { commits++; return nil }},
		process: watcherTestProcess{get: func(*kit.Kit, uint32, uint32, uint32) (*table.Process, error) {
			attempts++
			if attempts > pageRetryLimit {
				cancel()
			}
			return nil, errors.New("database unavailable")
		}},
	}}
	page := watchPage{cursor: "unprocessed", events: []bkcmdb.BkEventObj{{
		BkResource: bkcmdb.ResourceProcess, BkEventType: bkcmdb.EventDelete,
		BkDetail: json.RawMessage(`{"bk_biz_id":1,"bk_process_id":2,"service_instance_id":3}`),
	}}}
	c.processPageWithRetry(ctx, watchChain{resource: bkcmdb.ResourceProcess}, kit.New(), "cursor", page)
	if commits != 0 || attempts <= pageRetryLimit {
		t.Fatalf("attempts=%d commits=%d, failed page must remain pending", attempts, commits)
	}
}
