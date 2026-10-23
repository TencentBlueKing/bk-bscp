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
	"fmt"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/processor/cmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

type watcherTestCMDB struct {
	bkcmdb.Service
	watch func(context.Context, *bkcmdb.WatchResourceRequest) (*bkcmdb.WatchData, error)
}

func (c watcherTestCMDB) ResourceWatchLongPolling(ctx context.Context, req *bkcmdb.WatchResourceRequest,
	_ time.Duration) (*bkcmdb.WatchData, error) {
	return c.watch(ctx, req)
}

// newTestWatcher 构造仅用于链管理测试的 watcher（不依赖 dao/cc/cmdb）
func newTestWatcher() *cmdbResourceWatcher {
	return NewCmdbResourceWatcher(nil, nil, nil, nil, nil, nil, nil,
		cc.WatchCmdbResourceConfig{QpsLimit: 1000, IdleInterval: "1ns"})
}

func TestWatcherRetiresInactiveTenantQueues(t *testing.T) {
	c := newTestWatcher()
	t.Cleanup(c.stopQueues)

	activeQueue := c.syncQueueFor("active")
	staleQueue := c.syncQueueFor("stale")
	c.stopQueuesExcept(map[string]struct{}{"active": {}})

	if _, ok := c.queues["active"]; !ok {
		t.Fatal("active tenant queue was removed")
	}
	if _, ok := c.queues["stale"]; ok {
		t.Fatal("inactive tenant queue was retained")
	}
	if staleQueue.ctx.Err() == nil {
		t.Fatal("inactive tenant queue context was not canceled")
	}
	if activeQueue.ctx.Err() != nil {
		t.Fatal("active tenant queue context was canceled")
	}
}

func TestWatcherAdvancesCursorPastFailedPage(t *testing.T) {
	// 事件处理失败仅告警，游标照常推进，漏掉的变更由一键全量同步兜底
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var cursors []string
	client := watcherTestCMDB{watch: func(ctx context.Context, req *bkcmdb.WatchResourceRequest) (*bkcmdb.WatchData, error) {
		cursors = append(cursors, req.BkCursor)
		switch req.BkCursor {
		case "":
			return &bkcmdb.WatchData{BkEvents: []bkcmdb.BkEventObj{{
				BkResource: bkcmdb.ResourceProcess, BkEventType: bkcmdb.EventDelete, BkCursor: "event",
				BkDetail: json.RawMessage(`{"bk_biz_id":-1}`), // 非法业务 ID，事件页处理失败
			}}}, nil
		default:
			cancel()
			return nil, ctx.Err()
		}
	}}
	c := NewCmdbResourceWatcher(nil, nil, client, nil, nil, nil, nil,
		cc.WatchCmdbResourceConfig{QpsLimit: 1000, IdleInterval: "1ns"})
	c.runWatchChain(ctx, watchChain{resource: bkcmdb.ResourceProcess})
	if len(cursors) < 2 || cursors[1] != "event" {
		t.Fatalf("cursor should advance past failed page, cursors=%v", cursors)
	}
}

func TestWatcherProcessEventsBatchSkipsBadEvent(t *testing.T) {
	// 坏事件只跳过该条：聚合错误上抛告警，同页合法业务照常投递并被同步
	var (
		mu     sync.Mutex
		synced []int
	)
	c := newTestWatcher()
	t.Cleanup(c.stopQueues)
	c.syncBizFn = func(_ *kit.Kit, bizID int) error {
		mu.Lock()
		defer mu.Unlock()
		synced = append(synced, bizID)
		return nil
	}
	events := []bkcmdb.BkEventObj{
		{BkResource: bkcmdb.ResourceProcess, BkEventType: bkcmdb.EventDelete, BkCursor: "bad",
			BkDetail: json.RawMessage(`{`)},
		{BkResource: bkcmdb.ResourceProcess, BkEventType: bkcmdb.EventCreate, BkCursor: "ok",
			BkDetail: json.RawMessage(`{"bk_biz_id":1}`)},
	}
	err := c.handleProcessEventsBatch("", events)
	if err == nil {
		t.Fatal("bad event should be reported as aggregated error")
	}

	waitUntil(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(synced) == 1
	})
	mu.Lock()
	defer mu.Unlock()
	if synced[0] != 1 {
		t.Fatalf("valid biz should still be synced, got %v", synced)
	}
}

func TestWatcherProcessEventsBatchAllInvalid(t *testing.T) {
	// 整页事件都非法时不触发任何同步，但仍要上抛错误供调用方告警
	c := newTestWatcher()
	t.Cleanup(c.stopQueues)
	c.syncBizFn = func(_ *kit.Kit, _ int) error {
		t.Error("no biz should be synced when every event is invalid")
		return nil
	}
	events := []bkcmdb.BkEventObj{
		{BkResource: bkcmdb.ResourceProcess, BkEventType: bkcmdb.EventCreate, BkDetail: json.RawMessage(`{`)},
		{BkResource: bkcmdb.ResourceProcess, BkEventType: bkcmdb.EventUpdate,
			BkDetail: json.RawMessage(`{"bk_biz_id":-1}`)},
	}
	if err := c.handleProcessEventsBatch("", events); err == nil {
		t.Fatal("all-invalid batch should return aggregated error")
	}
}

// TestWatcherProcessEventsBatchReturnsBeforeSync 校验拉取与执行解耦：
// 同步阻塞时投递仍立即返回，链不会被慢同步堵住（停链超时后无法重建的根因）
func TestWatcherProcessEventsBatchReturnsBeforeSync(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	c := newTestWatcher()
	t.Cleanup(c.stopQueues)
	c.syncBizFn = func(_ *kit.Kit, _ int) error {
		close(started)
		<-release
		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- c.handleProcessEventsBatch("", []bkcmdb.BkEventObj{{
			BkResource: bkcmdb.ResourceProcess, BkEventType: bkcmdb.EventUpdate,
			BkDetail: json.RawMessage(`{"bk_biz_id":8}`),
		}})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("enqueue should not fail: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("event batch should return without waiting for the sync to finish")
	}

	<-started // 同步已在后台开始
	close(release)
}

func TestWatcherRejectsInvalidProcessIdentity(t *testing.T) {
	for _, detail := range []string{`{`, `null`, `{}`,
		`{"bk_biz_id":-1,"bk_process_id":2,"service_instance_id":3}`,
		`{"bk_biz_id":4294967297,"bk_process_id":2,"service_instance_id":3}`,
	} {
		bizIDs, err := groupProcessEventsByBiz([]bkcmdb.BkEventObj{{BkDetail: json.RawMessage(detail)}})
		if err == nil {
			t.Fatalf("accepted invalid detail %s", detail)
		}
		if len(bizIDs) != 0 {
			t.Fatalf("invalid detail %s should not yield biz, got %v", detail, bizIDs)
		}
	}
}

// TestWatcherProcessPageKeepsProcessEventsOnMixedResource 校验资源类型守卫只跳过异常事件：
// 同页的进程事件仍要触发全量同步（整页丢弃会让合法业务的变更一直等到定时同步）
func TestWatcherProcessPageKeepsProcessEventsOnMixedResource(t *testing.T) {
	var (
		mu     sync.Mutex
		synced []int
	)
	c := newTestWatcher()
	t.Cleanup(c.stopQueues)
	c.syncBizFn = func(_ *kit.Kit, bizID int) error {
		mu.Lock()
		defer mu.Unlock()
		synced = append(synced, bizID)
		return nil
	}
	events := []bkcmdb.BkEventObj{
		{BkResource: bkcmdb.ResourceModule, BkEventType: bkcmdb.EventUpdate, BkCursor: "module"},
		{BkResource: bkcmdb.ResourceProcess, BkEventType: bkcmdb.EventUpdate, BkCursor: "process",
			BkDetail: json.RawMessage(`{"bk_biz_id":2}`)},
	}
	err := c.processPage(watchChain{resource: bkcmdb.ResourceProcess}, kit.New(), events)
	if err == nil {
		t.Fatal("unexpected resource event should be reported")
	}

	waitUntil(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(synced) == 1
	})
	mu.Lock()
	defer mu.Unlock()
	if synced[0] != 2 {
		t.Fatalf("process event should still be synced, got %v", synced)
	}
}

// TestWatcherKeepsCursorOnEmptyResponse 空响应（bk_events 为空数组）不携带游标，
// 不能清空已有游标，否则每轮都会按 startFromOffset 回溯、重复重放事件
func TestWatcherKeepsCursorOnEmptyResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cursors []string
	client := watcherTestCMDB{watch: func(_ context.Context, req *bkcmdb.WatchResourceRequest) (*bkcmdb.WatchData, error) {
		cursors = append(cursors, req.BkCursor)
		switch len(cursors) {
		case 1:
			// 非法业务 ID：事件处理失败，但游标照常推进
			return &bkcmdb.WatchData{BkEvents: []bkcmdb.BkEventObj{{
				BkResource: bkcmdb.ResourceProcess, BkEventType: bkcmdb.EventUpdate, BkCursor: "cursor-1",
				BkDetail: json.RawMessage(`{"bk_biz_id":-1}`),
			}}}, nil
		case 2:
			return &bkcmdb.WatchData{}, nil
		default:
			cancel()
			return nil, ctx.Err()
		}
	}}

	c := NewCmdbResourceWatcher(nil, nil, client, nil, nil, nil, nil,
		cc.WatchCmdbResourceConfig{QpsLimit: 1000, IdleInterval: "1ns"})
	c.runWatchChain(ctx, watchChain{resource: bkcmdb.ResourceProcess})

	if len(cursors) < 3 {
		t.Fatalf("expect at least 3 watch requests, cursors=%v", cursors)
	}
	if cursors[1] != "cursor-1" || cursors[2] != "cursor-1" {
		t.Fatalf("cursor should be kept on empty response, cursors=%v", cursors)
	}
}

// TestWatcherStartsFromOffsetOnChainStart 游标不落库：链启动时没有历史游标，
// 首轮请求必须带 bk_start_from 按 startFromOffset 回溯（重启后同样走这里）
func TestWatcherStartsFromOffsetOnChainStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	requests := make(chan *bkcmdb.WatchResourceRequest, 1)
	client := watcherTestCMDB{watch: func(_ context.Context, req *bkcmdb.WatchResourceRequest) (
		*bkcmdb.WatchData, error) {
		requests <- req
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	c := NewCmdbResourceWatcher(nil, nil, client, nil, nil, nil, nil,
		cc.WatchCmdbResourceConfig{QpsLimit: 1000, IdleInterval: "1ns", StartFromOffset: "10m"})
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.runWatchChain(ctx, watchChain{resource: bkcmdb.ResourceProcess})
	}()

	req := <-requests
	if req.BkCursor != "" || req.BkStartFrom == nil {
		t.Fatalf("expect startFromOffset lookback on chain start, cursor=%q, startFrom=%v",
			req.BkCursor, req.BkStartFrom)
	}
	want := time.Now().Add(-10 * time.Minute).Unix()
	if diff := *req.BkStartFrom - want; diff < -5 || diff > 5 {
		t.Fatalf("startFrom = %d, want about %d (startFromOffset 10m)", *req.BkStartFrom, want)
	}
	cancel()
	<-done
}

// TestWatcherResetsCursorOnExpiredError 内存游标只在链长时间停滞（超过 CC 事件保留期）
// 后才会过期：过期后必须重置游标改走 startFromOffset 回溯，否则链会带着无效游标永久重试
func TestWatcherResetsCursorOnExpiredError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var reqs []*bkcmdb.WatchResourceRequest
		client := watcherTestCMDB{watch: func(_ context.Context, req *bkcmdb.WatchResourceRequest) (
			*bkcmdb.WatchData, error) {
			reqs = append(reqs, req)
			switch len(reqs) {
			case 1:
				// 非法业务 ID：事件页处理失败，但游标照常推进到 stale
				return &bkcmdb.WatchData{BkEvents: []bkcmdb.BkEventObj{{
					BkResource: bkcmdb.ResourceProcess, BkEventType: bkcmdb.EventUpdate, BkCursor: "stale",
					BkDetail: json.RawMessage(`{"bk_biz_id":-1}`),
				}}}, nil
			case 2:
				return nil, errors.New("1103007 事件节点不存在")
			default:
				cancel()
				return nil, ctx.Err()
			}
		}}
		c := NewCmdbResourceWatcher(nil, nil, client, nil, nil, nil, nil,
			cc.WatchCmdbResourceConfig{QpsLimit: 1000, IdleInterval: "1ns", StartFromOffset: "10m"})
		c.runWatchChain(ctx, watchChain{resource: bkcmdb.ResourceProcess})

		if len(reqs) < 3 {
			t.Fatalf("expect at least 3 watch requests, got %d", len(reqs))
		}
		if reqs[1].BkCursor != "stale" {
			t.Fatalf("second request should reuse in-memory cursor, cursor=%q", reqs[1].BkCursor)
		}
		if reqs[2].BkCursor != "" || reqs[2].BkStartFrom == nil {
			t.Fatalf("cursor should be reset after expiry, cursor=%q, startFrom=%v",
				reqs[2].BkCursor, reqs[2].BkStartFrom)
		}
	})
}

// TestSplitEventsKeepsEventsAfterKeepAlive keep-alive 空事件之后的事件不能被丢弃，
// 因为游标会推进到整页末尾
func TestSplitEventsKeepsEventsAfterKeepAlive(t *testing.T) {
	events, cursor, hasEmptyEvent := splitEvents([]bkcmdb.BkEventObj{
		{BkCursor: "c1", BkResource: bkcmdb.ResourceProcess, BkEventType: bkcmdb.EventUpdate},
		{BkCursor: "c2", BkResource: bkcmdb.ResourceProcess},
		{BkCursor: "c3", BkResource: bkcmdb.ResourceProcess, BkEventType: bkcmdb.EventCreate},
	})
	if !hasEmptyEvent {
		t.Fatal("keep-alive empty event should be marked")
	}
	if cursor != "c3" {
		t.Fatalf("cursor = %s, want c3", cursor)
	}
	if len(events) != 2 || events[0].BkCursor != "c1" || events[1].BkCursor != "c3" {
		t.Fatalf("events after keep-alive were dropped: %v", events)
	}
}

// 单业务失败不能影响队列里其他业务。
func TestBizSyncQueueContinuesAfterFailure(t *testing.T) {
	var mu sync.Mutex
	seen := make(map[int]bool)
	wantErr := errors.New("sync failed")
	q := newTestQueue(t, 2, time.Minute, func(_ *kit.Kit, bizID int) error {
		mu.Lock()
		seen[bizID] = true
		mu.Unlock()
		if bizID == 1 {
			return wantErr
		}
		return nil
	})
	for _, bizID := range []int{1, 2, 3} {
		q.enqueue(bizID)
	}
	waitUntil(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(seen) == 3
	})
	mu.Lock()
	defer mu.Unlock()
	for _, bizID := range []int{1, 2, 3} {
		if !seen[bizID] {
			t.Fatalf("biz %d was not synced, executed=%v", bizID, seen)
		}
	}
}

// TestBizSyncQueueCoalescesAndSerializes 同一业务并发投递只保留一份排队；
// 执行期间再次投递追加一轮，且同一业务不会被两个 worker 同时执行
func TestBizSyncQueueCoalescesAndSerializes(t *testing.T) {
	synctest.Test(t, testBizSyncQueueCoalescesAndSerializes)
}

func testBizSyncQueueCoalescesAndSerializes(t *testing.T) {
	var (
		mu       sync.Mutex
		calls    int
		inFlight int
		maxSeen  int
	)
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	q := newTestQueue(t, 2, time.Minute, func(_ *kit.Kit, _ int) error {
		mu.Lock()
		calls++
		inFlight++
		maxSeen = max(maxSeen, inFlight)
		mu.Unlock()
		started <- struct{}{}

		<-release

		mu.Lock()
		inFlight--
		mu.Unlock()
		return nil
	})

	q.enqueue(7)
	<-started
	// 执行期间重复投递：合并为一轮追加执行
	q.enqueue(7)
	q.enqueue(7)
	synctest.Wait()

	pending, running := q.size()
	if pending != 1 || running != 1 {
		t.Errorf("pending=%d running=%d, want 1/1 (coalesced)", pending, running)
	}

	close(release)
	waitUntil(t, 2*time.Second, func() bool {
		pending, running := q.size()
		return pending == 0 && running == 0
	})

	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("sync calls = %d, want 2 (initial + one coalesced rerun)", calls)
	}
	if maxSeen != 1 {
		t.Fatalf("same biz ran concurrently, max in-flight = %d", maxSeen)
	}
}

// 重试与普通事件必须共享同一业务的执行额度。
func TestBizSyncQueueRetryDoesNotOverlapWorker(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started := make(chan struct{}, 2)
		release := make(chan struct{})
		defer close(release)
		q := newTestQueue(t, 2, time.Hour, func(_ *kit.Kit, _ int) error {
			started <- struct{}{}
			<-release
			return nil
		})
		q.enqueue(7)
		<-started
		q.retrier.add(7)
		go q.retrier.retryOnce()
		synctest.Wait()
		select {
		case <-started:
			t.Error("retry overlapped the running worker")
		default:
		}
	})
}

// 一页批量入队时，所有 worker 都应能领取不同业务。
func TestBizSyncQueueBatchUsesAllWorkers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started := make(chan int, 3)
		release := make(chan struct{})
		defer close(release)
		q := newTestQueue(t, 3, time.Hour, func(_ *kit.Kit, bizID int) error {
			started <- bizID
			<-release
			return nil
		})
		q.enqueue(1)
		q.enqueue(2)
		q.enqueue(3)
		synctest.Wait()
		if got := len(started); got != 3 {
			t.Errorf("started %d businesses, want 3", got)
		}
	})
}

// TestBizSyncQueueRetriesSkipped 抢锁跳过由 retrier 补一轮，重试同样经过同步入口
func TestBizSyncQueueRetriesSkipped(t *testing.T) {
	var (
		mu       sync.Mutex
		attempts int
	)
	q := newTestQueue(t, 1, time.Millisecond, func(_ *kit.Kit, _ int) error {
		mu.Lock()
		defer mu.Unlock()
		attempts++
		if attempts <= 2 {
			return fmt.Errorf("wrapped: %w", cmdb.ErrSyncSkipped)
		}
		return nil
	})

	q.enqueue(3)
	waitUntil(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return attempts >= 3
	})
}

// newTestQueue 构造可注入同步函数、可控制重试间隔的测试队列，Cleanup 时先取消 ctx 再等 worker 退出
func newTestQueue(t *testing.T, workers int, retryInterval time.Duration,
	syncBiz func(*kit.Kit, int) error) *bizSyncQueue {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	queue := newBizSyncQueue(ctx, "", workers, retryInterval, syncBiz)
	t.Cleanup(func() {
		cancel()
		queue.stop()
	})
	return queue
}

// TestProcessPageModuleContinuesAfterFailure 单条 module 事件失败不影响本页其余事件：
// 游标在页处理结束后照常推进，被跳过的事件不会再被重放，因此失败必须聚合而不是提前返回
func TestProcessPageModuleContinuesAfterFailure(t *testing.T) {
	c := &cmdbResourceWatcher{}
	events := []bkcmdb.BkEventObj{
		{BkResource: bkcmdb.ResourceModule, BkEventType: bkcmdb.EventUpdate, BkCursor: "c1",
			BkDetail: json.RawMessage(`{"bk_biz_id":-1,"bk_set_id":1,"bk_module_id":1}`)},
		{BkResource: bkcmdb.ResourceModule, BkEventType: bkcmdb.EventUpdate, BkCursor: "c2",
			BkDetail: json.RawMessage(`{"bk_biz_id":-2,"bk_set_id":1,"bk_module_id":1}`)},
	}

	err := c.processPage(watchChain{resource: bkcmdb.ResourceModule}, kit.New(), events)
	if err == nil {
		t.Fatal("expected aggregated error for invalid module events")
	}
	for _, cursor := range []string{"c1", "c2"} {
		if !strings.Contains(err.Error(), cursor) {
			t.Fatalf("error should cover all failed events, missing %s: %v", cursor, err)
		}
	}
}

// waitUntil 轮询等待条件成立，超时则失败
func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met before timeout")
		}
		time.Sleep(time.Millisecond)
	}
}

// TestSkippedBizRetrierRetriesUntilSuccess 抢锁跳过的业务必须等锁释放后补一轮：
// 持锁方正在进行的同步快照可能早于本条事件，不能当作补偿
func TestSkippedBizRetrierRetriesUntilSuccess(t *testing.T) {
	var (
		mu       sync.Mutex
		attempts int
	)
	retrier := newSkippedBizRetrier(t.Context(), time.Millisecond, func(int) error {
		mu.Lock()
		defer mu.Unlock()
		attempts++
		if attempts <= 2 {
			return fmt.Errorf("wrapped: %w", cmdb.ErrSyncSkipped)
		}
		return nil
	})
	defer retrier.Stop()

	retrier.add(7)
	waitUntil(t, 2*time.Second, func() bool { return retrier.size() == 0 })

	mu.Lock()
	defer mu.Unlock()
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3 (2 skipped + 1 success)", attempts)
	}
}

// TestSkippedBizRetrierGivesUpAfterMaxAttempts 持锁方长时间未结束时重试次数有上限，
// 超过后放弃并交由定时全量同步兜底，避免队列无限占用
func TestSkippedBizRetrierGivesUpAfterMaxAttempts(t *testing.T) {
	var (
		mu       sync.Mutex
		attempts int
	)
	retrier := newSkippedBizRetrier(t.Context(), time.Millisecond, func(int) error {
		mu.Lock()
		defer mu.Unlock()
		attempts++
		return fmt.Errorf("wrapped: %w", cmdb.ErrSyncSkipped)
	})
	defer retrier.Stop()

	retrier.add(7)
	retrier.add(7) // 同一业务重复登记只保留一份
	waitUntil(t, 5*time.Second, func() bool { return retrier.size() == 0 })

	mu.Lock()
	defer mu.Unlock()
	if attempts != skippedBizRetryMaxAttempts {
		t.Fatalf("attempts = %d, want %d", attempts, skippedBizRetryMaxAttempts)
	}
}

// TestSkippedBizRetrierStopIsIdempotent 链停止后不得再触发补同步，重复 Stop 安全；
// nil 队列（未启用进程链的场景）调用不 panic
func TestSkippedBizRetrierStopIsIdempotent(t *testing.T) {
	var (
		mu    sync.Mutex
		calls int
	)
	retrier := newSkippedBizRetrier(t.Context(), time.Millisecond, func(int) error {
		mu.Lock()
		defer mu.Unlock()
		calls++
		return nil
	})

	retrier.Stop()
	retrier.Stop()
	retrier.add(1)
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if calls != 0 {
		t.Fatalf("retrier ran after stop, calls=%d", calls)
	}

	var nilRetrier *skippedBizRetrier
	nilRetrier.add(1)
	nilRetrier.Stop()
	if nilRetrier.size() != 0 {
		t.Fatal("nil retrier should report empty queue")
	}
}

// 停止超时后保留同 key 状态，直到链真正退出才能回收并重建。
func TestWatcherRetainsStoppingChainUntilExit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newTestWatcher()
		c.chainStopWait = time.Second
		ch := watchChain{resource: bkcmdb.ResourceProcess}
		done := make(chan struct{})
		canceled := 0
		c.chains[ch] = watchChainState{cancel: func() { canceled++ }, done: done}

		c.stopChains([]watchChain{ch})
		if state, ok := c.chains[ch]; !ok || !state.stopping || canceled != 1 {
			t.Fatalf("chain must remain stopping, state=%+v, exists=%v, cancels=%d", state, ok, canceled)
		}
		c.cleanExitedChains()
		if _, ok := c.chains[ch]; !ok {
			t.Fatal("chain was removed before it exited")
		}
		start := time.Now()
		c.stopAllChains()
		if time.Since(start) != 0 || canceled != 1 {
			t.Fatal("repeated reconciliation must not wait for or cancel a stopping chain again")
		}
		close(done)
		c.cleanExitedChains()
		if _, ok := c.chains[ch]; ok {
			t.Fatal("exited chain was not removed")
		}
	})
}

func TestWatcherRemovesStoppedChain(t *testing.T) {
	c := newTestWatcher()
	ch := watchChain{resource: bkcmdb.ResourceModule}
	done := make(chan struct{})
	close(done)
	c.chains[ch] = watchChainState{cancel: func() {}, done: done}
	c.stopChains([]watchChain{ch})
	if _, ok := c.chains[ch]; ok {
		t.Fatal("exited chain must be removed")
	}
}

// 等待预算由所有链共享，不随链数累加。
func TestWatcherStopChainsUsesSharedTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newTestWatcher()
		c.chainStopWait = time.Second
		keys := make([]watchChain, 0, 5)
		for i := 0; i < 5; i++ {
			ch := watchChain{tenantID: "t" + string(rune('a'+i)), resource: bkcmdb.ResourceProcess}
			keys = append(keys, ch)
			c.chains[ch] = watchChainState{cancel: func() {}, done: make(chan struct{})}
		}
		start := time.Now()
		c.stopChains(keys)
		if elapsed := time.Since(start); elapsed != c.chainStopWait {
			t.Fatalf("stop chains took %s, want shared budget %s", elapsed, c.chainStopWait)
		}
		if len(c.chains) != len(keys) {
			t.Fatal("stopping chains must be retained")
		}
		for _, state := range c.chains {
			if !state.stopping {
				t.Fatal("all chains must be canceled before waiting")
			}
		}
	})
}

func TestWatchRequestElapsedExcludesQueueTime(t *testing.T) {
	for _, queue := range []string{"slots", "rate"} {
		t.Run(queue, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				conf := cc.WatchCmdbResourceConfig{MaxConcurrency: 1, QpsLimit: 1000}
				if queue == "rate" {
					conf.QpsLimit = 0.5
				}
				client := watcherTestCMDB{watch: func(context.Context, *bkcmdb.WatchResourceRequest) (*bkcmdb.WatchData, error) {
					return &bkcmdb.WatchData{}, nil
				}}
				c := NewCmdbResourceWatcher(nil, nil, client, nil, nil, nil, nil, conf)
				if queue == "slots" {
					if err := c.watchSlots.Acquire(t.Context(), 1); err != nil {
						t.Fatal(err)
					}
					go func() {
						time.Sleep(2 * time.Second)
						c.watchSlots.Release(1)
					}()
				} else if !c.limiter.Allow() {
					t.Fatal("initial rate token missing")
				}
				start := time.Now()
				_, elapsed, err := c.fetchWatchPage(t.Context(), watchChain{resource: bkcmdb.ResourceProcess}, nil, "cursor")
				if err != nil {
					t.Fatal(err)
				}
				if time.Since(start) < 2*time.Second || elapsed != 0 {
					t.Fatalf("queue time=%s, HTTP time=%s", time.Since(start), elapsed)
				}
				if !c.watchSlots.TryAcquire(1) {
					t.Fatal("watch slot leaked after request")
				}
				c.watchSlots.Release(1)
			})
		})
	}
}

func TestWatchRequestCancellationWhileQueued(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newTestWatcher()
		if err := c.watchSlots.Acquire(t.Context(), int64(c.conf.MaxConcurrency)); err != nil {
			t.Fatal(err)
		}
		defer c.watchSlots.Release(int64(c.conf.MaxConcurrency))
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		result := make(chan error, 1)
		go func() {
			_, _, err := c.fetchWatchPage(ctx, watchChain{resource: bkcmdb.ResourceProcess}, nil, "cursor")
			result <- err
		}()
		synctest.Wait()
		cancel()
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("queued request returned %v, want canceled", err)
		}
	})
}
