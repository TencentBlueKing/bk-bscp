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
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"

	"github.com/TencentBlueKing/bk-bscp/cmd/data-service/service"
	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/bkuser"
	gsecomponents "github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/processor/cmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/internal/task"
	"github.com/TencentBlueKing/bk-bscp/internal/task/builder/gse"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	// errCursorExpiredSign 游标过期的错误特征（CC 错误码 1103007，事件节点不存在）。
	// 游标只存在于内存并按响应持续推进，正常运行不会过期；只有链长时间停滞
	// （超过 CC 事件保留期）后才可能命中，此时重置游标并按 startFromOffset 回溯。
	errCursorExpiredSign = "1103007"
	errCursorMissingSign = "事件节点不存在"

	// keepAliveThreshold 判断 CC 是否 hold 长轮询连接的耗时阈值：
	// 响应无事件且耗时 < 该值时视为 CC 未 hold（兼容短轮询环境），退避 idleInterval 后再续连
	keepAliveThreshold = 1 * time.Second

	// defaultMaxRequestBackoff 请求失败退避的上限
	defaultMaxRequestBackoff = time.Minute
	cmdbBizIDField           = "bk_biz_id"

	// chainStopWaitTimeout 停止 watch 链时等待其退出的最长时间。
	// 链退出前可能正处于慢请求或全量同步中，无限等待会阻塞对账与 shutdown。
	chainStopWaitTimeout = 10 * time.Second
)

// NewCmdbResourceWatcher 监听cmdb资源变化。
// redLock 用于协调跨实例的同业务全量进程同步（可续期 Redis 租约），未注入时退化为进程内互斥。
func NewCmdbResourceWatcher(dao dao.Set, sd serviced.Service, cmdb bkcmdb.Service, gse *gsecomponents.Service,
	svc *service.Service, taskManager *task.TaskManager, redLock *lock.TokenLock,
	conf cc.WatchCmdbResourceConfig) *cmdbResourceWatcher {

	// 构造函数也接受零值配置，避免并发额度为零导致请求阻塞。
	if conf.QpsLimit <= 0 || math.IsNaN(conf.QpsLimit) || math.IsInf(conf.QpsLimit, 0) {
		logs.Warnf("[CMDB Watch] invalid qpsLimit %f, fallback to 20", conf.QpsLimit)
		conf.QpsLimit = 20
	}
	if conf.MaxConcurrency < 1 {
		logs.Warnf("[CMDB Watch] invalid maxConcurrency %d, fallback to 8", conf.MaxConcurrency)
		conf.MaxConcurrency = 8
	}
	if conf.HandlerConcurrency < 1 {
		logs.Warnf("[CMDB Watch] invalid handlerConcurrency %d, fallback to 4", conf.HandlerConcurrency)
		conf.HandlerConcurrency = 4
	}

	requestTimeout := watchDuration(conf.RequestTimeout, 35*time.Second)
	if requestTimeout <= 20*time.Second {
		requestTimeout = 35 * time.Second
	}
	if conf.MaxWatchChains < 1 {
		conf.MaxWatchChains = 100
	}

	// 全量同步队列的 ctx 独立于 watch 链：链停止只取消拉取，正在执行的同步继续跑完
	syncCtx, syncCancel := context.WithCancel(context.Background())

	return &cmdbResourceWatcher{
		dao:         dao,
		state:       sd,
		cmdb:        cmdb,
		svc:         svc,
		gse:         gse,
		taskManager: taskManager,
		redLock:     redLock,
		conf:        conf,

		limiter:         rate.NewLimiter(rate.Limit(conf.QpsLimit), 1),
		watchSlots:      semaphore.NewWeighted(int64(conf.MaxConcurrency)),
		interval:        watchDuration(conf.Interval, time.Minute),
		idleInterval:    max(watchDuration(conf.IdleInterval, time.Second), time.Second),
		startFromOffset: watchDuration(conf.StartFromOffset, 5*time.Minute),
		requestTimeout:  requestTimeout,
		chains:          make(map[watchChain]watchChainState),
		chainStopWait:   chainStopWaitTimeout,
		queues:          make(map[string]*bizSyncQueue),
		syncCtx:         syncCtx,
		syncCancel:      syncCancel,
	}
}

type cmdbResourceWatcher struct {
	dao         dao.Set
	state       serviced.Service
	cmdb        bkcmdb.Service
	gse         *gsecomponents.Service
	svc         *service.Service
	taskManager *task.TaskManager
	// redLock 同一业务全量进程同步的分布式租约（持锁期间续期），为空时退化为进程内互斥
	redLock *lock.TokenLock
	conf    cc.WatchCmdbResourceConfig
	// syncBizFn 覆盖单业务同步入口，仅供测试观测「页内坏事件不影响其他业务」，为空时走 syncBizProcesses
	syncBizFn func(kt *kit.Kit, bizID int) error

	// queues 仅保留实际运行 process watch 链的租户；租户下线或被链数上限淘汰时回收。
	// queueMu 保护 queues；队列使用 watcher 级 ctx 的子 context，与单条链生命周期解耦。
	queueMu    sync.Mutex
	queues     map[string]*bizSyncQueue
	syncCtx    context.Context
	syncCancel context.CancelFunc

	// limiter 限制 watch 请求频率；watchSlots 限制并发请求数（含等待限速）。
	limiter    *rate.Limiter
	watchSlots *semaphore.Weighted

	interval        time.Duration
	idleInterval    time.Duration
	startFromOffset time.Duration
	requestTimeout  time.Duration
	chainStopWait   time.Duration

	startOnce sync.Once
	// chains 由 Run 的管理 goroutine 独占；链 goroutine 只负责关闭 done。
	// 停止超时的链仍保留在表内，确认退出后才允许重建同 key 链。
	chains map[watchChain]watchChainState
}

type watchChainState struct {
	cancel   context.CancelFunc
	done     <-chan struct{}
	stopping bool
}

// watchDuration 在初始化时解析配置；运行期间直接使用 duration。
func watchDuration(value string, fallback time.Duration) time.Duration {
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return fallback
	}
	return duration
}

// watchChain 标识一个 (tenant, resource) 维度的常驻 watch 链
type watchChain struct {
	tenantID string
	resource bkcmdb.ResourceType
}

// Run 启动 watch 任务：按 interval 周期做主从判定与租户发现，
// 为每个 (tenant, resource) 维度维持一条常驻长轮询 watch 链。
func (c *cmdbResourceWatcher) Run() {
	c.startOnce.Do(c.run)
}

func (c *cmdbResourceWatcher) run() {
	logs.Infof("start watching cmdb resource changes, resources=%v, maxConcurrency=%d, qpsLimit=%f",
		c.resources(), c.conf.MaxConcurrency, c.conf.QpsLimit)

	notifier := shutdown.AddNotifier()
	rootCtx, cancel := context.WithCancel(context.Background())

	go func() {
		defer notifier.Done()
		defer cancel()
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		// 启动后立即做一次对账，不等第一个 tick
		c.reconcileChains(rootCtx)
		for {
			select {
			case <-notifier.Signal:
				cancel()
				c.stopAllChains()
				c.stopQueues()
				logs.Infof("stop watching cmdb resource success")
				return
			case <-ticker.C:
				c.reconcileChains(rootCtx)
			}
		}
	}()
}

// reconcileChains 对账 watch 链：master 时补启缺失链、停掉失效链（租户禁用/资源下线）；slave 时全部停止
func (c *cmdbResourceWatcher) reconcileChains(rootCtx context.Context) {
	if !c.state.IsMaster() {
		logs.V(3).Infof("[CMDB Watch] current instance is slave, stop watch chains")
		c.stopAllChains()
		c.stopQueuesExcept(nil)
		return
	}

	targets := make(map[watchChain]struct{})
	if cc.DataService().FeatureFlags.EnableMultiTenantMode {
		tenants, err := bkuser.ListEnabledTenants(rootCtx)
		if err != nil {
			logs.Errorf("[CMDB Watch] failed to list tenants: %v", err)
			return
		}
		if len(tenants) == 0 {
			logs.Warnf("[CMDB Watch] no enabled tenants found")
		}
		for _, tenant := range tenants {
			for _, res := range c.resources() {
				targets[watchChain{tenantID: tenant.ID, resource: res}] = struct{}{}
			}
		}
	} else {
		for _, res := range c.resources() {
			targets[watchChain{resource: res}] = struct{}{}
		}
	}

	// 链数量上限：租户过多时 租户×资源 的链数量可能失控（内存 + CC 空闲请求量），
	// 超限的链确定性截断（排序后保留前 maxWatchChains 个）并告警，需显式调大配置
	chainList := make([]watchChain, 0, len(targets))
	for ch := range targets {
		chainList = append(chainList, ch)
	}
	sort.Slice(chainList, func(i, j int) bool {
		if chainList[i].tenantID != chainList[j].tenantID {
			return chainList[i].tenantID < chainList[j].tenantID
		}
		return chainList[i].resource < chainList[j].resource
	})
	// 长轮询占用并发槽；链数超过额度时会排队，增加事件延迟。
	if len(chainList) > c.conf.MaxConcurrency {
		logs.Warnf("[CMDB Watch] watch chains (%d) exceed maxConcurrency (%d): long-polling will queue, "+
			"event latency may grow to minutes", len(chainList), c.conf.MaxConcurrency)
	}

	maxChains := c.conf.MaxWatchChains
	if len(chainList) > maxChains {
		skipped := len(chainList) - maxChains
		chainList = chainList[:maxChains]
		logs.Warnf("[CMDB Watch] watch chains %d exceed maxWatchChains %d, %d chains will NOT run, "+
			"please increase crontab.watchCmdbResource.maxWatchChains or reduce resources",
			len(targets), maxChains, skipped)
	}
	// 移除被上限裁掉的目标，否则租户列表变化时旧链会占用额外额度。
	targets = make(map[watchChain]struct{}, len(chainList))
	for _, ch := range chainList {
		targets[ch] = struct{}{}
	}

	c.cleanExitedChains()

	var stale []watchChain
	for ch := range c.chains {
		if _, ok := targets[ch]; !ok && !c.chains[ch].stopping {
			stale = append(stale, ch)
		}
	}
	// 所有失效链共享停止超时，避免等待时间随链数累加。
	c.stopChains(stale)
	activeProcessTenants := make(map[string]struct{})
	for _, ch := range chainList {
		if ch.resource == bkcmdb.ResourceProcess {
			activeProcessTenants[ch.tenantID] = struct{}{}
		}
	}
	c.stopQueuesExcept(activeProcessTenants)

	for _, ch := range chainList {
		if _, ok := c.chains[ch]; ok {
			continue
		}

		chainCtx, cancel := context.WithCancel(rootCtx)
		done := make(chan struct{})
		c.chains[ch] = watchChainState{cancel: cancel, done: done}
		go func() {
			defer close(done)
			c.runWatchChain(chainCtx, ch)
		}()
		logs.Infof("[CMDB Watch] started watch chain, tenant=%s, resource=%s", ch.tenantID, ch.resource)
	}
}

func (c *cmdbResourceWatcher) stopAllChains() {
	c.cleanExitedChains()
	keys := make([]watchChain, 0, len(c.chains))
	for ch, state := range c.chains {
		if !state.stopping {
			keys = append(keys, ch)
		}
	}
	c.stopChains(keys)
}

// stopChains 先取消所有指定链，再共享一份等待预算；超时后保留状态，禁止重建。
func (c *cmdbResourceWatcher) stopChains(keys []watchChain) {
	if len(keys) == 0 {
		return
	}
	for _, ch := range keys {
		state, ok := c.chains[ch]
		if !ok || state.stopping {
			continue
		}
		state.cancel()
		state.stopping = true
		c.chains[ch] = state
	}

	wait := c.chainStopWait
	if wait <= 0 {
		wait = chainStopWaitTimeout
	}
	deadline, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	for _, ch := range keys {
		state, ok := c.chains[ch]
		if !ok {
			continue
		}
		select {
		case <-state.done:
			delete(c.chains, ch)
		case <-deadline.Done():
			logs.Warnf("[CMDB Watch] chain is still stopping, tenant=%s, resource=%s", ch.tenantID, ch.resource)
		}
	}
}

// cleanExitedChains 非阻塞回收已退出的链，包括停止超时后退出的链。
func (c *cmdbResourceWatcher) cleanExitedChains() {
	for ch, state := range c.chains {
		select {
		case <-state.done:
			state.cancel()
			delete(c.chains, ch)
		default:
		}
	}
}

// resources 返回固定订阅的资源：模块事件同步名称，进程事件触发业务全量同步。
func (c *cmdbResourceWatcher) resources() []bkcmdb.ResourceType {
	return []bkcmdb.ResourceType{bkcmdb.ResourceModule, bkcmdb.ResourceProcess}
}

// fieldsFor 返回资源订阅的 bk_fields，与事件处理逻辑耦合，固定在代码内：
//   - module 事件需要拓扑 ID 与模块名才能更新进程表；
//   - process 事件仅用于定位受影响业务再触发全量同步，明细只需业务 ID。
func (c *cmdbResourceWatcher) fieldsFor(resource bkcmdb.ResourceType) []string {
	switch resource {
	case bkcmdb.ResourceModule:
		return []string{cmdbBizIDField, "bk_set_id", "bk_module_id", "bk_module_name"}
	case bkcmdb.ResourceProcess:
		return []string{cmdbBizIDField}
	default:
		return []string{}
	}
}

// eventTypesFor 仅订阅模块更新；进程增删改均触发业务全量同步。
func (c *cmdbResourceWatcher) eventTypesFor(resource bkcmdb.ResourceType) []string {
	switch resource {
	case bkcmdb.ResourceModule:
		return []string{bkcmdb.EventUpdate.String()}
	case bkcmdb.ResourceProcess:
		return []string{
			bkcmdb.EventCreate.String(), bkcmdb.EventUpdate.String(), bkcmdb.EventDelete.String(),
		}
	default:
		return nil
	}
}

// runWatchChain 串行拉取并处理同一租户、资源的事件页。
// 游标只保存在内存中：进程重启后丢失，回退到 startFromOffset 回溯；
// 回溯窗口之外漏掉的变更由定时全量同步或一键全量同步补偿（全量同步幂等）。
// 处理失败仍推进游标（重放整页会让页内其他业务反复全量同步）。
// 进程事件只投递到 bizSyncQueue、不在此等待同步完成：慢同步不会阻塞拉页，
// 停链时被取消的也只是拉取，正在跑的同步按队列自己的生命周期跑完（见 bizSyncQueue）。
func (c *cmdbResourceWatcher) runWatchChain(chainCtx context.Context, ch watchChain) {
	kt := kit.New()
	kt.TenantID = ch.tenantID
	// 保留链取消信号，并携带 Kit 和 RPC metadata，供租户隔离与日志使用。
	kt.Ctx = kit.WithKit(chainCtx, kt)
	ctx := kt.InternalRpcCtx()
	kt.Ctx = ctx

	// 游标只存在于内存：链启动时为空，首轮请求按 startFromOffset 回溯
	var (
		cursor         string
		requestBackoff time.Duration
	)
	fields := c.fieldsFor(ch.resource)
	logs.Infof("[CMDB Watch] chain started, tenant=%s, resource=%s", ch.tenantID, ch.resource)

	for {
		if err := ctx.Err(); err != nil {
			break
		}

		resp, elapsed, watchErr := c.fetchWatchPage(ctx, ch, fields, cursor)
		if watchErr != nil {
			if ctx.Err() != nil {
				break
			}
			// 内存游标随每次响应推进，正常运行不会过期；只有链长时间停滞
			// （超过 CC 事件保留期）后才可能命中，重置后下一轮按 startFromOffset 回溯
			if isCursorExpiredError(watchErr) {
				logs.Warnf("[CMDB Watch] cursor expired, reset and fall back to startFromOffset, tenant=%s, "+
					"resource=%s, err=%v", ch.tenantID, ch.resource, watchErr)
				cursor = ""
			} else {
				logs.Errorf("[CMDB Watch] watch request failed, tenant=%s, resource=%s, err=%v",
					ch.tenantID, ch.resource, watchErr)
			}

			// 统一指数退避（1s -> 2s -> 4s -> ... 上限 1m），退避期间不占用对 CC 的额度；
			// 游标过期也走这里，避免 CC 异常时紧密重试
			if requestBackoff == 0 {
				requestBackoff = time.Second
			}
			if !waitWatchRetry(ctx, requestBackoff) {
				break
			}
			requestBackoff = min(requestBackoff*2, defaultMaxRequestBackoff)
			continue
		}

		// 请求成功，重置退避状态
		requestBackoff = 0

		// 拆页：keep-alive 空事件（bk_event_type 为空）代表 CC 当前事件流位置，游标要推进但不能当数据处理
		events, pageCursor, hasEmptyEvent := splitEvents(resp.BkEvents)

		// 请求返回与取消可能同时发生；取消后不再处理本页。
		if ctx.Err() != nil {
			break
		}

		// 处理失败仍推进游标：整页重放会让页内其他业务反复做全量同步。进程事件只做投递，
		// 投递成功即可推进游标；同步失败/抢锁跳过由队列重试与定时、一键全量同步补偿。
		if err := c.processPage(ch, kt, events); err != nil {
			logs.Errorf("[CMDB Watch] process page failed, tenant=%s, resource=%s, events=%d, err=%v",
				ch.tenantID, ch.resource, len(events), err)
		}
		// 空响应（bk_events 为空数组）不携带游标：保留旧游标，避免退化成每轮
		// 按 startFromOffset 回溯、反复重放同一批事件
		if pageCursor != "" {
			cursor = pageCursor
		}

		// 空响应或纯 keep-alive 且 CC 未 hold（响应耗时过短）：退避 idleInterval 后再续连，
		// 避免紧密循环打爆 CC；否则立即续连（长轮询 hold 到期场景）
		if len(events) == 0 && (!hasEmptyEvent || elapsed < keepAliveThreshold) {
			// 空闲态每 idleInterval 一次，属常态，降级为调试日志
			logs.V(3).Infof("[CMDB Watch] no events, tenant=%s, resource=%s, cc_held=%v, idle=%s",
				ch.tenantID, ch.resource, elapsed >= keepAliveThreshold, c.idleInterval)
			if !waitWatchRetry(ctx, c.idleInterval) {
				break
			}
		}
	}

	logs.Infof("[CMDB Watch] chain stopped, tenant=%s, resource=%s", ch.tenantID, ch.resource)
}

// fetchWatchPage 在并发和频率限制下请求一页，返回不含排队时间的请求耗时。
func (c *cmdbResourceWatcher) fetchWatchPage(ctx context.Context, ch watchChain, fields []string, cursor string) (
	*bkcmdb.WatchData, time.Duration, error) {

	req := &bkcmdb.WatchResourceRequest{
		BkResource:   ch.resource.String(),
		BkEventTypes: c.eventTypesFor(ch.resource),
		BkFields:     fields,
	}
	if cursor == "" {
		// 无有效游标（首次启动或游标过期重置），按 startFromOffset 回溯，规避 CC 3 小时事件保留限制
		t := time.Now().Add(-c.startFromOffset).Unix()
		req.BkStartFrom = &t
	} else {
		req.BkCursor = cursor
	}

	if err := c.watchSlots.Acquire(ctx, 1); err != nil {
		return nil, 0, err
	}
	defer c.watchSlots.Release(1)
	// 先取得并发槽再限速，避免等待槽位的请求同时发出。
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, 0, fmt.Errorf("limiter wait failed: %w", err)
	}

	// 排队时间不代表 CC hold 时长，空闲退避只依据实际请求耗时。
	start := time.Now()
	resp, err := c.cmdb.ResourceWatchLongPolling(ctx, req, c.requestTimeout)
	return resp, time.Since(start), err
}

// splitEvents 拆分响应事件：keep-alive 空事件（bk_event_type 为空）单独标记，
// 其游标代表 CC 当前事件流位置，需要推进但不能当数据处理。
// 空事件之后若仍有真实事件必须继续处理：游标会推进到整页末尾，
// 提前退出会让这些事件永久丢失。
func splitEvents(respEvents []bkcmdb.BkEventObj) (events []bkcmdb.BkEventObj, pageCursor string, hasEmptyEvent bool) {
	for _, event := range respEvents {
		pageCursor = event.BkCursor
		if event.BkEventType == "" {
			hasEmptyEvent = true
			continue
		}
		events = append(events, event)
	}
	return events, pageCursor, hasEmptyEvent
}

// processPage 逐条处理模块事件；进程事件按业务归并后投递给全量同步队列（不等待执行）。
func (c *cmdbResourceWatcher) processPage(ch watchChain, kt *kit.Kit, events []bkcmdb.BkEventObj) error {
	if len(events) == 0 {
		return nil
	}

	if ch.resource == bkcmdb.ResourceProcess {
		// 资源类型守卫：混合资源事件不能被当作进程事件触发全量同步；
		// 但异常事件只跳过该条并累计错误，不连带丢弃同页的进程事件
		procEvents := make([]bkcmdb.BkEventObj, 0, len(events))
		var resourceErrs []error
		for _, event := range events {
			if event.BkResource != ch.resource {
				resourceErrs = append(resourceErrs, fmt.Errorf("unexpected resource %s in %s watch chain, cursor=%s",
					event.BkResource, ch.resource, event.BkCursor))
				continue
			}
			procEvents = append(procEvents, event)
		}
		resourceErr := errors.Join(resourceErrs...)
		if len(procEvents) == 0 {
			return resourceErr
		}
		return errors.Join(resourceErr, c.handleProcessEventsBatch(ch.tenantID, procEvents))
	}

	// module 事件逐条独立：单条失败不能中断本页其余事件。
	// 游标在页处理结束后照常推进，被跳过的事件不会再被重放，因此这里只聚合错误、不提前返回
	var errs []error
	for _, event := range events {
		if event.BkResource != ch.resource {
			errs = append(errs, fmt.Errorf("unexpected resource %s in %s watch chain, cursor=%s",
				event.BkResource, ch.resource, event.BkCursor))
			continue
		}
		if err := c.handleModuleEvent(kt, event); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// waitWatchRetry 等待退避结束；取消时立即返回 false。
func waitWatchRetry(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// isCursorExpiredError 判断是否为游标过期错误（CC 1103007，事件节点不存在）
func isCursorExpiredError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, errCursorExpiredSign) || strings.Contains(msg, errCursorMissingSign)
}

// groupProcessEventsByBiz 归并事件涉及的业务 ID：进程事件明细经 bk_fields 裁剪仅含 bk_biz_id，
// 不区分事件类型，任一事件都触发该业务的全量同步。
// 单条明细解码失败或业务 ID 非法时只跳过该条并累计错误：CC 一页可含多个业务，坏事件不能连带
// 丢弃同页其他业务的变更（游标在页处理后照常推进，被跳过的事件不会再被重放）
func groupProcessEventsByBiz(events []bkcmdb.BkEventObj) (map[int]struct{}, error) {
	bizIDs := make(map[int]struct{})
	var errs []error
	for _, event := range events {
		var detail struct {
			BkBizID int `json:"bk_biz_id"`
		}
		if err := event.Decode(&detail); err != nil {
			errs = append(errs, fmt.Errorf("decode process event, cursor=%s: %w", event.BkCursor, err))
			continue
		}
		if !validCMDBID(detail.BkBizID) {
			errs = append(errs, fmt.Errorf("invalid process event bizID, cursor=%s", event.BkCursor))
			continue
		}
		bizIDs[detail.BkBizID] = struct{}{}
	}
	return bizIDs, errors.Join(errs...)
}

// validCMDBID 防止缺失、负数或溢出的 ID 被转换为数据库资源 ID。
func validCMDBID(id int) bool {
	return id > 0 && uint64(id) <= math.MaxUint32
}

// dispatchProcessStateSyncTasks 为变更进程派发状态同步任务，并按进程和别名去重。
func (c *cmdbResourceWatcher) dispatchProcessStateSyncTasks(res *cmdb.SyncProcessResult) {
	if res == nil {
		return
	}
	// 同一 (ccProcessID, alias) 在一次同步结果中只派发一次，避免别名重建残留等场景重复入队
	dispatched := make(map[processStateSyncKey]struct{}, len(res.Items))
	for _, item := range res.Items {
		if item == nil || item.Process == nil || item.Process.Attachment == nil || item.Process.Spec == nil {
			logs.Errorf("[CMDB][ProcessSync] skip invalid process sync result item")
			continue
		}
		bizID := item.Process.Attachment.BizID
		tenantID := item.Process.Attachment.TenantID
		key := processStateSyncKey{
			tenantID:    tenantID,
			bizID:       bizID,
			ccProcessID: item.Process.Attachment.CcProcessID,
			alias:       item.Process.Spec.Alias,
		}
		if _, ok := dispatched[key]; ok {
			continue
		}
		dispatched[key] = struct{}{}

		taskObj, err := task.NewByTaskBuilder(
			gse.NewProcessStateSyncTask(c.dao, tenantID, bizID, item.Process, item.Instances),
		)
		if err != nil {
			logs.Errorf("[CMDB][ProcessSync] create gse task failed, bizID=%d, err=%v", bizID, err)
			continue
		}
		c.taskManager.Dispatch(taskObj)
	}
}

// processStateSyncKey 进程状态同步任务的去重键
type processStateSyncKey struct {
	tenantID    string
	bizID       uint32
	ccProcessID uint32
	alias       string
}

// handleModuleEvent 处理模块(Module)事件：watch 链仅订阅 update 事件，
// 用于将模块名变更同步到进程表。这是模块名生效的快路径，不是唯一写入点：
// 本函数失败时游标仍会推进，漏掉的改名由全量同步的 BuildProcessChanges 拓扑 diff 兜底
func (c *cmdbResourceWatcher) handleModuleEvent(kt *kit.Kit, resource bkcmdb.BkEventObj) error {
	var result bkcmdb.ModuleInfo
	if err := resource.Decode(&result); err != nil {
		logs.Errorf("[CMDB][ModuleSync] decode module resource failed, resource=%v, err=%v", resource, err)
		return fmt.Errorf("decode module resource failed: %w", err)
	}
	if !validCMDBID(result.BkBizID) || !validCMDBID(result.BkSetID) || !validCMDBID(result.BkModuleID) {
		return fmt.Errorf("invalid module event identity, cursor=%s", resource.BkCursor)
	}
	if resource.BkEventType != bkcmdb.EventUpdate {
		logs.Warnf("[CMDB][ModuleSync] unexpected module event type %s, skip, biz=%d, set=%d, module=%d",
			resource.BkEventType, result.BkBizID, result.BkSetID, result.BkModuleID)
		return nil
	}

	bizID := uint32(result.BkBizID)
	setID := uint32(result.BkSetID)
	modID := uint32(result.BkModuleID)
	moduleName := result.BkModuleName

	logPrefix := fmt.Sprintf("[CMDB][ModuleSync][biz=%d][set=%d][module=%d]", bizID, setID, modID)
	if err := c.dao.Process().UpdateSelectedFields(kt, bizID, map[string]any{"module_name": moduleName},
		c.dao.GenQuery().Process.SetID.Eq(setID),
		c.dao.GenQuery().Process.ModuleID.Eq(modID)); err != nil {
		logs.Errorf("update module name failed to %s, module_name=%s, err=%v", logPrefix, moduleName, err)
		return fmt.Errorf("update module name failed: %w", err)
	}
	logs.Infof("update module name success: %s", logPrefix)

	return nil
}

// syncBizProcesses 全量同步单个业务并派发进程状态同步任务。
// 返回 cmdb.ErrSyncSkipped 表示本轮未执行（锁被其他入口占用），调用方不能当作「已同步」。
func (c *cmdbResourceWatcher) syncBizProcesses(kt *kit.Kit, bizID int) error {
	if !c.svc.IsProcessConfigViewEnabled(uint32(bizID)) {
		logs.Infof("[CMDB][ProcessSync] skip biz %d: process config view not enabled", bizID)
		return nil
	}

	svc := cmdb.NewSyncCMDBService(kt.TenantID, bizID, c.cmdb, c.gse, c.dao, c.redLock)
	res, err := svc.SyncBizProcesses(kt.Ctx)
	if err != nil {
		if cmdb.IsSyncSkipped(err) {
			return err
		}
		return fmt.Errorf("sync biz %d processes failed: %w", bizID, err)
	}

	c.dispatchProcessStateSyncTasks(res)
	return nil
}

// handleProcessEventsBatch 按业务去重后投递到该租户的同步队列，不等待同步完成。
// 投递是纯内存操作，因此本页处理几乎瞬时返回，游标随即推进；真正的全量同步在队列 worker
// 中执行（单业务失败不影响其他业务，抢锁跳过由 retrier 补一轮，见 bizSyncQueue）。
// 明细非法的坏事件只跳过该条，合法业务照常投递，聚合错误最后统一上抛告警。
func (c *cmdbResourceWatcher) handleProcessEventsBatch(tenantID string, events []bkcmdb.BkEventObj) error {
	if len(events) == 0 {
		return nil
	}
	bizIDs, decodeErr := groupProcessEventsByBiz(events)
	if decodeErr != nil {
		logs.Warnf("[CMDB][ProcessSync] skip invalid process events, events=%d, bizCount=%d, err=%v",
			len(events), len(bizIDs), decodeErr)
	}
	if len(bizIDs) == 0 {
		return decodeErr
	}

	queue := c.syncQueueFor(tenantID)
	for bizID := range bizIDs {
		queue.enqueue(bizID)
	}
	logs.Infof("[CMDB][ProcessSync] process events enqueued, events=%d, bizCount=%d", len(events), len(bizIDs))
	return decodeErr
}

// syncBiz 单业务全量同步入口；syncBizFn 非空时优先（测试注入用），生产路径恒为 syncBizProcesses
func (c *cmdbResourceWatcher) syncBiz(kt *kit.Kit, bizID int) error {
	if c.syncBizFn != nil {
		return c.syncBizFn(kt, bizID)
	}
	return c.syncBizProcesses(kt, bizID)
}

// syncQueueFor 返回租户对应的同步队列，不存在时创建并启动 worker。
// 链短暂重启时沿用队列；租户不再有 process watch 链时由 reconcile 回收。
func (c *cmdbResourceWatcher) syncQueueFor(tenantID string) *bizSyncQueue {
	c.queueMu.Lock()
	defer c.queueMu.Unlock()
	if c.queues == nil {
		c.queues = make(map[string]*bizSyncQueue)
	}
	if queue, ok := c.queues[tenantID]; ok {
		return queue
	}
	queueCtx, cancel := context.WithCancel(c.syncCtx)
	queue := newBizSyncQueue(queueCtx, tenantID, c.conf.HandlerConcurrency,
		skippedBizRetryInterval, c.syncBiz)
	queue.cancel = cancel
	c.queues[tenantID] = queue
	logs.Infof("[CMDB][ProcessSync] sync queue started, tenant=%s, workers=%d",
		tenantID, c.conf.HandlerConcurrency)
	return queue
}

// stopQueuesExcept 回收不再有运行中 process watch 链的租户队列，避免租户长期变更时
// queues map 和每队列的 worker/retrier goroutine 随历史租户数无界增长。
func (c *cmdbResourceWatcher) stopQueuesExcept(activeTenants map[string]struct{}) {
	c.queueMu.Lock()
	var stale []*bizSyncQueue
	for tenantID, queue := range c.queues {
		if _, active := activeTenants[tenantID]; active {
			continue
		}
		delete(c.queues, tenantID)
		stale = append(stale, queue)
	}
	c.queueMu.Unlock()

	for _, queue := range stale {
		queue.stop()
	}
}

// stopQueues 取消全部队列并等待 worker 退出（仅进程退出时调用）。
// 队列 ctx 与链 ctx 相互独立：主从切换只停链，队列里正在执行的同步照常跑完，不阻塞停链。
func (c *cmdbResourceWatcher) stopQueues() {
	if c.syncCancel != nil {
		c.syncCancel()
	}

	c.queueMu.Lock()
	queues := make([]*bizSyncQueue, 0, len(c.queues))
	for _, queue := range c.queues {
		queues = append(queues, queue)
	}
	c.queues = make(map[string]*bizSyncQueue)
	c.queueMu.Unlock()

	for _, queue := range queues {
		queue.stop()
	}
}

const (
	// skippedBizRetryInterval 抢锁跳过后的补同步间隔：远小于定时全量同步（默认 30 分钟），
	// 又不会在持锁方长事务期间形成压力
	skippedBizRetryInterval = 30 * time.Second
	// skippedBizRetryMaxAttempts 单个业务的补同步次数上限（约 20 分钟）：
	// 期间持锁方若一直未结束（异常残留锁最多 10 分钟过期），超过后放弃并告警，
	// 残留变更由定时全量同步（默认 30 分钟）覆盖
	skippedBizRetryMaxAttempts = 40
)

// skippedBizRetrier 收集「因抢锁失败而跳过」的业务，按固定间隔重试，成功后移出队列。
//
// 为什么需要它：跳过的本轮没有执行，而持锁方正在进行的同步快照可能早于本条事件
// （锁覆盖「拉取全量 → diff → 写库」全过程，事件可能发生在对方快照取走之后），
// 那轮同步不会包含这条变更；不补一轮就要等下一轮定时同步（默认 30 分钟）。
//
// 为什么不让整页失败、保留游标：游标是 (tenant, resource) 维度的，重放整页会让页内
// 其他业务反复做全量同步（跳过时每次还要等锁 10s），代价远高于只补跳过的业务。
// 队列只存在于内存：链重启或进程重启后丢失，由定时/一键全量同步兜底。
type skippedBizRetrier struct {
	ctx      context.Context
	sync     func(bizID int) error
	interval time.Duration
	maxTries int

	mu      sync.Mutex
	pending map[int]int // bizID -> 已重试次数
	stopped bool

	stop chan struct{}
	done chan struct{}
}

func newSkippedBizRetrier(ctx context.Context, interval time.Duration,
	sync func(bizID int) error) *skippedBizRetrier {

	retrier := &skippedBizRetrier{
		ctx:      ctx,
		sync:     sync,
		interval: interval,
		maxTries: skippedBizRetryMaxAttempts,
		pending:  make(map[int]int),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go retrier.run()
	return retrier
}

// add 登记需要补一轮的业务；同一业务重复登记只保留一份，避免每页事件都新起一轮重试
func (r *skippedBizRetrier) add(bizID int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return
	}
	if _, ok := r.pending[bizID]; !ok {
		r.pending[bizID] = 0
	}
}

// Stop 停止重试并等待 goroutine 退出；可重复调用
func (r *skippedBizRetrier) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	r.stopped = true
	r.mu.Unlock()

	close(r.stop)
	<-r.done
}

func (r *skippedBizRetrier) run() {
	defer close(r.done)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-r.stop:
			return
		case <-ticker.C:
			r.retryOnce()
		}
	}
}

// retryOnce 对当前队列里的业务各重试一次；仍在同步中的留在队列等下一轮
func (r *skippedBizRetrier) retryOnce() {
	for _, bizID := range r.snapshot() {
		err := r.sync(bizID)
		if err != nil && cmdb.IsSyncSkipped(err) {
			if r.bump(bizID) {
				continue
			}
			logs.Errorf("[CMDB][ProcessSync] skipped sync retry exhausted, give up until periodic full sync, "+
				"bizID=%d, attempts=%d", bizID, r.maxTries)
			continue
		}
		if err != nil {
			logs.Errorf("[CMDB][ProcessSync] skipped sync retry failed, bizID=%d, err=%v", bizID, err)
		} else {
			logs.Infof("[CMDB][ProcessSync] skipped sync retried, bizID=%d", bizID)
		}
		r.remove(bizID)
	}
}

func (r *skippedBizRetrier) snapshot() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	bizIDs := make([]int, 0, len(r.pending))
	for bizID := range r.pending {
		bizIDs = append(bizIDs, bizID)
	}
	return bizIDs
}

// bump 记录一次重试；达到上限时移出队列并返回 false
func (r *skippedBizRetrier) bump(bizID int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	tries := r.pending[bizID] + 1
	if tries >= r.maxTries {
		delete(r.pending, bizID)
		return false
	}
	r.pending[bizID] = tries
	return true
}

func (r *skippedBizRetrier) remove(bizID int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.pending, bizID)
}

// size 当前待补同步的业务数量（观测与测试用）
func (r *skippedBizRetrier) size() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.pending)
}

// bizSyncQueue 单租户「单业务全量同步」执行队列，把 watch 链的拉取周期与同步执行解耦：
//   - 链只负责拉页 + 投递业务 ID，慢同步或卡住的 DB 调用不再阻塞后续事件页；
//   - 停链只取消拉取，正在执行的同步按队列自己的生命周期（watcher 级 ctx）跑完，
//     不会让链长期停留在 stopping、导致同一 key 的链无法重建；
//   - 同一业务同时最多排队一次；执行期间再次投递会追加一轮，覆盖「事件晚于当前快照」的情况；
//   - 并发度 = handlerConcurrency，业务之间互不影响，单业务失败不影响其他业务。
//
// 队列只存在于内存：进程重启后待执行项丢失，由定时/一键全量同步兜底（全量同步幂等）。
type bizSyncQueue struct {
	ctx      context.Context
	cancel   context.CancelFunc
	tenantID string
	syncBiz  func(kt *kit.Kit, bizID int) error
	retrier  *skippedBizRetrier
	work     chan struct{}

	mu      sync.Mutex
	pending map[int]struct{}
	running map[int]struct{}
	closed  bool
	workers sync.WaitGroup
}

// newBizSyncQueue 创建并启动队列；ctx 由 watcher 持有（不用链 ctx），retryInterval 供抢锁跳过后补一轮。
func newBizSyncQueue(ctx context.Context, tenantID string, workers int,
	retryInterval time.Duration, syncBiz func(kt *kit.Kit, bizID int) error) *bizSyncQueue {

	workers = max(workers, 1)
	if ctx == nil {
		// 防御：调用方绕过构造函数直接组装 watcher（仅测试场景）时不能 panic
		ctx = context.Background()
	}
	queue := &bizSyncQueue{
		ctx:      ctx,
		tenantID: tenantID,
		syncBiz:  syncBiz,
		work:     make(chan struct{}, 1),
		pending:  make(map[int]struct{}),
		running:  make(map[int]struct{}),
	}
	// 抢锁跳过 = 本轮未执行，持锁方快照可能早于事件，必须补一轮；重试次数上限沿用 retrier
	queue.retrier = newSkippedBizRetrier(ctx, retryInterval, func(bizID int) error {
		queue.mu.Lock()
		if queue.closed {
			queue.mu.Unlock()
			return context.Canceled
		}
		if _, running := queue.running[bizID]; running {
			queue.mu.Unlock()
			return cmdb.ErrSyncSkipped
		}
		queue.running[bizID] = struct{}{}
		queue.mu.Unlock()
		return queue.execute(bizID)
	})

	queue.workers.Add(workers)
	for i := 0; i < workers; i++ {
		go queue.worker()
	}
	return queue
}

// newKit 构造队列级 Kit：ctx 来自队列而非链，链停止后仍可完成本次同步的日志与 RPC 上下文
func (q *bizSyncQueue) newKit() *kit.Kit {
	kt := kit.New()
	kt.TenantID = q.tenantID
	kt.Ctx = kit.WithKit(q.ctx, kt)
	kt.Ctx = kt.InternalRpcCtx()
	return kt
}

func (q *bizSyncQueue) worker() {
	defer q.workers.Done()
	for {
		select {
		case <-q.ctx.Done():
			return
		case <-q.work:
			q.drain()
		}
	}
}

// drain 依次执行当前排队的业务，直到队列为空；执行期间新投递的项由下一轮循环取走
func (q *bizSyncQueue) drain() {
	for {
		if q.ctx.Err() != nil {
			return
		}
		bizID, ok := q.pop()
		if !ok {
			return
		}
		q.run(bizID)
	}
}

// enqueue 投递一次同步：已在排队则合并；正在执行时追加一轮，避免「事件晚于快照」被合并掉
func (q *bizSyncQueue) enqueue(bizID int) {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	if _, ok := q.pending[bizID]; ok {
		q.mu.Unlock()
		return
	}
	q.pending[bizID] = struct{}{}
	q.mu.Unlock()
	q.wake()
}

func (q *bizSyncQueue) wake() {
	select {
	case q.work <- struct{}{}:
	default:
		// 已有唤醒信号未消费，worker 会取走本次投递的任务
	}
}

func (q *bizSyncQueue) pop() (int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for bizID := range q.pending {
		if _, running := q.running[bizID]; running {
			continue
		}
		delete(q.pending, bizID)
		// 领取任务与占用业务必须在同一临界区内完成。
		q.running[bizID] = struct{}{}
		// 一页可同时投递多个业务，继续唤醒其他 worker 领取剩余任务。
		q.wake()
		return bizID, true
	}
	return 0, false
}

func (q *bizSyncQueue) run(bizID int) {
	err := q.execute(bizID)
	switch {
	case err == nil:
		return
	case cmdb.IsSyncSkipped(err):
		logs.Warnf("[CMDB][ProcessSync] another sync is running, retry after it finishes, bizID=%d", bizID)
		q.retrier.add(bizID)
	default:
		logs.Errorf("[CMDB][ProcessSync] sync failed, bizID=%d, err=%v", bizID, err)
	}
}

// execute 执行已占用的业务；普通 worker 和补同步共用执行状态。
func (q *bizSyncQueue) execute(bizID int) error {
	defer func() {
		q.mu.Lock()
		delete(q.running, bizID)
		_, pending := q.pending[bizID]
		q.mu.Unlock()
		if pending {
			q.wake()
		}
	}()
	return q.syncBiz(q.newKit(), bizID)
}

// stop 停止收任务并等待 worker 退出；调用前必须已取消队列 ctx，否则 worker 会阻塞在取任务上
func (q *bizSyncQueue) stop() {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	q.mu.Unlock()
	if q.cancel != nil {
		q.cancel()
	}

	q.retrier.Stop()
	q.workers.Wait()
}

// size 返回排队中与执行中的业务数（观测与测试用）
func (q *bizSyncQueue) size() (pending, running int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending), len(q.running)
}
