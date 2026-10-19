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

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/cmd/data-service/service"
	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/bkuser"
	gsecomponents "github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/processor/cmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/internal/task"
	"github.com/TencentBlueKing/bk-bscp/internal/task/builder/gse"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	// errCursorExpiredSign 游标过期的错误特征（CC 错误码 1103007，事件节点不存在）
	errCursorExpiredSign = "1103007"
	errCursorMissingSign = "事件节点不存在"

	// pageRetryLimit 连续失败的告警阈值，超过后继续退避重试，不能跳过未处理事件。
	pageRetryLimit = 3

	// pageHandoffQueueLen watch 循环与事件处理器之间的页队列长度，队列满时 watch 循环阻塞产生背压，
	// 避免在 CC hold 会话期间无界积压未处理页
	pageHandoffQueueLen = 2

	// keepAliveThreshold 判断 CC 是否 hold 长轮询连接的耗时阈值：
	// 响应无事件且耗时 < 该值时视为 CC 未 hold（兼容短轮询环境），退避 idleInterval 后再续连
	keepAliveThreshold = 1 * time.Second

	// cursorExpiredAlarmThreshold 单位时间内连续游标过期的告警阈值，超过说明 CC 侧事件流异常
	cursorExpiredAlarmThreshold = 5

	// defaultMaxRequestBackoff 请求失败退避的上限
	defaultMaxRequestBackoff = time.Minute
	cmdbBizIDField           = "bk_biz_id"
)

// NewCmdbResourceWatcher 监听cmdb资源变化
func NewCmdbResourceWatcher(dao dao.Set, sd serviced.Service, cmdb bkcmdb.Service, gse *gsecomponents.Service,
	svc *service.Service, taskManager *task.TaskManager, conf cc.WatchCmdbResourceConfig) *cmdbResourceWatcher {

	// 防御性兜底：调用方绕过 trySetDefault 传入零值时，避免 limiter burst=0 导致
	// 请求被永久拒绝、信号量/并发度为 0 导致链静默卡死
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

	return &cmdbResourceWatcher{
		dao:         dao,
		state:       sd,
		cmdb:        cmdb,
		svc:         svc,
		gse:         gse,
		taskManager: taskManager,
		conf:        conf,

		limiter:   rate.NewLimiter(rate.Limit(conf.QpsLimit), 1),
		semaphore: make(chan struct{}, conf.MaxConcurrency),

		chains: make(map[watchChain]watchChainState),
	}
}

type cmdbResourceWatcher struct {
	dao         dao.Set
	state       serviced.Service
	cmdb        bkcmdb.Service
	gse         *gsecomponents.Service
	svc         *service.Service
	taskManager *task.TaskManager
	conf        cc.WatchCmdbResourceConfig

	// limiter 全局 watch 请求 QPS 限流（保护 CC，主要收敛追积压时的突发）
	limiter *rate.Limiter
	// semaphore 同时向 CC 发起的 watch 请求数上限，同时也是 CC watch session（hold 连接）数上限
	semaphore chan struct{}

	// chains 运行中的 (tenant, resource) watch 链
	mu     sync.Mutex
	chains map[watchChain]watchChainState
}

type watchChainState struct {
	cancel context.CancelFunc
	done   <-chan struct{}
}

// watchChain 标识一个 (tenant, resource) 维度的常驻 watch 链
type watchChain struct {
	tenantID string
	resource bkcmdb.ResourceType
}

// watchPage 一页待处理事件及其游标
type watchPage struct {
	cursor string
	events []bkcmdb.BkEventObj
}

// Run 启动 watch 任务：按 interval 周期做主从判定与租户发现，
// 为每个 (tenant, resource) 维度维持一条常驻长轮询 watch 链。
func (c *cmdbResourceWatcher) Run() {
	logs.Infof("start watching cmdb resource changes, resources=%v, maxConcurrency=%d, qpsLimit=%f",
		c.conf.Resources, c.conf.MaxConcurrency, c.conf.QpsLimit)

	notifier := shutdown.AddNotifier()
	// rootCtx 是任务级父 ctx：一次创建、shutdown 时取消一次，作为所有链 ctx 的父节点
	rootCtx, cancel := context.WithCancel(context.Background())

	interval, err := time.ParseDuration(c.conf.Interval)
	if err != nil || interval <= 0 {
		logs.Errorf("[CMDB Watch] invalid interval %q, fallback to 1m", c.conf.Interval)
		interval = time.Minute
	}

	go func() {
		defer notifier.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// 启动后立即做一次对账，不等第一个 tick
		c.reconcileChains(rootCtx)
		for {
			select {
			case <-notifier.Signal:
				c.stopAllChains()
				cancel()
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
		if len(c.chains) > 0 {
			logs.Infof("[CMDB Watch] current instance is slave, stop all watch chains")
		} else {
			logs.Infof("[CMDB Watch] current instance is slave, skip watching cmdb resource")
		}
		c.stopAllChains()
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
	maxChains := c.maxWatchChains()
	skipped := 0
	if len(chainList) > maxChains {
		skipped = len(chainList) - maxChains
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

	c.mu.Lock()
	defer c.mu.Unlock()

	for ch, state := range c.chains {
		if _, ok := targets[ch]; !ok {
			state.cancel()
			<-state.done
			delete(c.chains, ch)
			logs.Infof("[CMDB Watch] stopped watch chain, tenant=%s, resource=%s", ch.tenantID, ch.resource)
		}
	}

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
	c.mu.Lock()
	defer c.mu.Unlock()

	for ch, state := range c.chains {
		state.cancel()
		logs.Infof("[CMDB Watch] stopped watch chain, tenant=%s, resource=%s", ch.tenantID, ch.resource)
	}
	for _, state := range c.chains {
		<-state.done
	}
	c.chains = make(map[watchChain]watchChainState)
}

// resources 返回要监听的资源类型列表，过滤不合法配置
func (c *cmdbResourceWatcher) resources() []bkcmdb.ResourceType {
	res := make([]bkcmdb.ResourceType, 0, len(c.conf.Resources))
	for _, r := range c.conf.Resources {
		rt := bkcmdb.ResourceType(r)
		if rt != bkcmdb.ResourceSet && rt != bkcmdb.ResourceModule && rt != bkcmdb.ResourceProcess {
			logs.Errorf("[CMDB Watch] invalid resource type %q in config, skip", r)
			continue
		}
		res = append(res, rt)
	}
	return res
}

// fieldsFor 返回资源的 bk_fields 裁剪列表，未配置的资源回退到代码内默认值
func (c *cmdbResourceWatcher) fieldsFor(resource bkcmdb.ResourceType) []string {
	if c.conf.Fields != nil {
		if fields, ok := c.conf.Fields[resource.String()]; ok {
			return fields
		}
	}

	switch resource {
	case bkcmdb.ResourceSet:
		return []string{cmdbBizIDField, "bk_set_id", "bk_set_name", "bk_set_env", "set_template_id"}
	case bkcmdb.ResourceModule:
		return []string{cmdbBizIDField, "bk_set_id", "bk_module_id", "bk_module_name"}
	default:
		return []string{}
	}
}

// runWatchChain 单条 (tenant, resource) watch 链：长轮询续连循环 + 同资源 FIFO 事件处理器。
//
// CC resource_watch 为短长链设计：带游标 watch 且无事件时，CC 保持连接至多 20s，
// 期间有变更即刻推回；超时返回 keep-alive 空事件。因此客户端在每次响应后立即带着
// 最新游标续连即可获得准实时推送，仅在 CC 未 hold 时（兼容短轮询环境）按 idleInterval 退避。
func (c *cmdbResourceWatcher) runWatchChain(chainCtx context.Context, ch watchChain) {
	kt := kit.New()
	if ch.tenantID != "" {
		kt.TenantID = ch.tenantID
	}

	// ctx 组合了三个语义，缺一不可：
	// 1. 链级取消：chainCtx 派生自 rootCtx，shutdown 或对账停止时级联取消
	// 2. KitKey：多租户模式下 bkcmdb.doRequestWithClient 通过 kit.MustGetKit(ctx) 读取
	//    租户 ID，ctx 缺失 kit 时会 panic
	// 3. Incoming metadata（rid/租户）：InternalRpcCtx 注入，供请求日志关联与多租户链路使用
	kt.Ctx = kit.WithKit(chainCtx, kt)
	ctx, cancel := context.WithCancel(kt.InternalRpcCtx())
	kt.Ctx = ctx
	defer cancel()

	cursorKey := getResourceCursorKey(kt.TenantID, ch.resource.String())
	cursor, err := c.loadCursorWithRetry(kt, cursorKey)
	if err != nil {
		return
	}
	fields := c.fieldsFor(ch.resource)
	logs.Infof("[CMDB Watch] chain started, tenant=%s, resource=%s, cursor=%s", ch.tenantID, ch.resource, cursor)

	// 同资源事件 FIFO 串行处理；页队列满时 watch 循环阻塞，形成背压
	pages := make(chan watchPage, pageHandoffQueueLen)
	processorDone := make(chan struct{})
	go func() {
		defer close(processorDone)
		c.processPages(ctx, ch, kt, pages)
	}()

	idleInterval, err := time.ParseDuration(c.conf.IdleInterval)
	if err != nil || idleInterval < time.Second {
		idleInterval = time.Second
	}

	var (
		requestBackoff     time.Duration
		consecutiveExpired int
	)

loop:
	for {
		if err := ctx.Err(); err != nil {
			break
		}

		startAt := time.Now()
		resp, watchErr := c.fetchWatchPage(ctx, ch, fields, cursor)
		if watchErr != nil {
			if ctx.Err() != nil {
				break
			}
			if isCursorExpiredError(watchErr) {
				// 游标过期：重置后按 startFromOffset 回溯
				consecutiveExpired++
				if consecutiveExpired >= cursorExpiredAlarmThreshold {
					logs.Errorf("[CMDB Watch] cursor expired %d times consecutively, tenant=%s, resource=%s, "+
						"CC event stream may be abnormal", consecutiveExpired, ch.tenantID, ch.resource)
				} else {
					logs.Warnf("[CMDB Watch] cursor expired, tenant=%s, resource=%s, expired_cursor=%s, err=%v",
						ch.tenantID, ch.resource, cursor, watchErr)
				}
				cursor = ""
				// 仅重置请求游标；落库必须由 FIFO 处理器完成，不能越过待处理页。
				continue
			}

			// 请求失败：指数退避（1s -> 2s -> 4s -> ... 上限 1m），退避期间不占用对 CC 的额度
			if requestBackoff == 0 {
				requestBackoff = time.Second
			}
			logs.Errorf("[CMDB Watch] watch request failed, tenant=%s, resource=%s, backoff=%s, err=%v",
				ch.tenantID, ch.resource, requestBackoff, watchErr)
			select {
			case <-ctx.Done():
				break loop
			case <-time.After(requestBackoff):
			}
			requestBackoff = min(requestBackoff*2, defaultMaxRequestBackoff)
			continue
		}

		// 请求成功，重置退避状态
		requestBackoff = 0
		consecutiveExpired = 0
		elapsed := time.Since(startAt)

		// 拆页：keep-alive 空事件（bk_event_type 为空）代表 CC 当前事件流位置，游标要推进但不能当数据处理
		events, pageCursor, hasEmptyEvent := splitEvents(resp.BkEvents)

		if len(events) == 0 {
			// 空响应或纯 keep-alive：推进游标并立即续连（长轮询 hold 到期场景）；
			// 若 CC 未 hold（响应耗时过短），退避 idleInterval 后再续连，避免紧密循环打爆 CC
			if pageCursor != "" && pageCursor != cursor {
				select {
				case pages <- watchPage{cursor: pageCursor}:
				case <-ctx.Done():
					break loop
				}
				cursor = pageCursor
			}
			if !hasEmptyEvent || elapsed < keepAliveThreshold {
				logs.Infof("[CMDB Watch] no events, tenant=%s, resource=%s, cc_held=%v, idle=%s",
					ch.tenantID, ch.resource, elapsed >= keepAliveThreshold, idleInterval)
				select {
				case <-ctx.Done():
					break loop
				case <-time.After(idleInterval):
				}
			}
			continue
		}

		// 有事件页：按 maxEventsPerPage 切块后交给本资源的 FIFO 处理器，
		// 队列满时阻塞产生背压。每块游标取块内最后一个事件的游标，
		// 块级处理和游标持久化成功后才能处理下一块
		chunkSize := c.maxEventsPerPage()
		for start := 0; start < len(events); start += chunkSize {
			end := min(start+chunkSize, len(events))
			chunkPage := watchPage{cursor: events[end-1].BkCursor, events: events[start:end]}
			if end == len(events) {
				chunkPage.cursor = pageCursor
			}
			select {
			case pages <- chunkPage:
			case <-ctx.Done():
				break loop
			}
		}
		cursor = pageCursor
	}

	<-processorDone
	logs.Infof("[CMDB Watch] chain stopped, tenant=%s, resource=%s", ch.tenantID, ch.resource)
}

// fetchWatchPage 执行一次长轮询 watch 请求：QPS 限流 + 并发信号量约束后发起请求。
// goroutine 数可以多于信号量额度，请求前 acquire，请求结束后释放
func (c *cmdbResourceWatcher) fetchWatchPage(ctx context.Context, ch watchChain, fields []string, cursor string) (
	*bkcmdb.WatchData, error) {

	req := &bkcmdb.WatchResourceRequest{
		BkResource: ch.resource.String(),
		BkEventTypes: []string{
			bkcmdb.EventCreate.String(), bkcmdb.EventUpdate.String(), bkcmdb.EventDelete.String(),
		},
		BkFields: fields,
	}
	if cursor == "" {
		// 无有效游标（首次启动或游标过期重置），按 startFromOffset 回溯，规避 CC 3 小时事件保留限制
		t := time.Now().Add(-c.startFromOffset()).Unix()
		req.BkStartFrom = &t
	} else {
		req.BkCursor = cursor
	}

	select {
	case c.semaphore <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-c.semaphore }()
	// 取得并发槽后再限速，防止等待槽位的请求积攒令牌后同时发出。
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("limiter wait failed: %w", err)
	}

	return c.cmdb.ResourceWatchLongPolling(ctx, req, c.requestTimeout())
}

// splitEvents 拆分响应事件：keep-alive 空事件（bk_event_type 为空）单独标记，
// 其游标代表 CC 当前事件流位置，需要推进但不能当数据处理
func splitEvents(respEvents []bkcmdb.BkEventObj) (events []bkcmdb.BkEventObj, pageCursor string, hasEmptyEvent bool) {
	for _, event := range respEvents {
		pageCursor = event.BkCursor
		if event.BkEventType == "" {
			hasEmptyEvent = true
			break
		}
		events = append(events, event)
	}
	return events, pageCursor, hasEmptyEvent
}

// processPages 按资源维度 FIFO 串行处理事件页，页处理成功后推进游标落库
func (c *cmdbResourceWatcher) processPages(ctx context.Context, ch watchChain, kt *kit.Kit, pages <-chan watchPage) {
	cursorKey := getResourceCursorKey(kt.TenantID, ch.resource.String())

	for {
		select {
		case <-ctx.Done():
			return
		case page, ok := <-pages:
			if !ok {
				return
			}
			c.processPageWithRetry(ctx, ch, kt, cursorKey, page)
		}
	}
}

// processPageWithRetry 失败时保留当前页并退避重试，防止数据库故障造成事件丢失。
func (c *cmdbResourceWatcher) processPageWithRetry(ctx context.Context, ch watchChain, kt *kit.Kit,
	cursorKey string, page watchPage) {

	processed := false
	backoff := time.Second
	for attempt := 1; ctx.Err() == nil; attempt++ {
		var err error
		if !processed {
			err = c.processPage(ch, kt, page.events)
			processed = err == nil
		}
		if processed {
			err = c.persistCursor(kt, cursorKey, page.cursor)
			if err == nil {
				return
			}
		}
		logs.Errorf("[CMDB Watch] process page failed, tenant=%s, resource=%s, events=%d, attempt=%d/%d, err=%v",
			ch.tenantID, ch.resource, len(page.events), attempt, pageRetryLimit, err)
		if attempt == pageRetryLimit {
			logs.Errorf("[CMDB Watch] page stalled, tenant=%s, resource=%s, cursor=%s; retrying without skipping",
				ch.tenantID, ch.resource, page.cursor)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, defaultMaxRequestBackoff)
	}
}

// processPage 按事件顺序处理，仅将连续同类型的 process 事件合并为批次。
func (c *cmdbResourceWatcher) processPage(ch watchChain, kt *kit.Kit, events []bkcmdb.BkEventObj) error {
	for start := 0; start < len(events); {
		event := events[start]
		if event.BkResource != ch.resource {
			return fmt.Errorf("unexpected resource %s in %s watch chain", event.BkResource, ch.resource)
		}
		if ch.resource != bkcmdb.ResourceProcess {
			if err := c.handleEvent(kt, event); err != nil {
				return err
			}
			start++
			continue
		}

		end := start + 1
		for end < len(events) && events[end].BkEventType == event.BkEventType && events[end].BkResource == ch.resource {
			end++
		}
		var err error
		switch event.BkEventType {
		case bkcmdb.EventCreate:
			err = c.handleProcessCreateEventsBatch(kt, events[start:end])
		case bkcmdb.EventUpdate:
			err = c.handleProcessUpdateEventsBatch(kt, events[start:end])
		case bkcmdb.EventDelete:
			err = c.handleProcessDeleteEventsBatch(kt, events[start:end])
		default:
			return fmt.Errorf("unknown process event type: %s", event.BkEventType)
		}
		if err != nil {
			return err
		}
		start = end
	}
	return nil
}

// startFromOffset 返回游标丢失/过期时的回溯偏移
func (c *cmdbResourceWatcher) startFromOffset() time.Duration {
	offset, err := time.ParseDuration(c.conf.StartFromOffset)
	if err != nil || offset <= 0 {
		logs.Errorf("[CMDB Watch] invalid startFromOffset %q, fallback to 5m", c.conf.StartFromOffset)
		return 5 * time.Minute
	}
	return offset
}

// requestTimeout 返回单次长轮询请求超时，必须覆盖 CC 20s hold 窗口
func (c *cmdbResourceWatcher) requestTimeout() time.Duration {
	timeout, err := time.ParseDuration(c.conf.RequestTimeout)
	if err != nil || timeout <= 20*time.Second {
		logs.Errorf("[CMDB Watch] invalid requestTimeout %q, fallback to 35s", c.conf.RequestTimeout)
		return 35 * time.Second
	}
	return timeout
}

// maxWatchChains 返回允许同时运行的 watch 链数量上限（租户×资源），防御零值
func (c *cmdbResourceWatcher) maxWatchChains() int {
	if c.conf.MaxWatchChains < 1 {
		logs.Warnf("[CMDB Watch] invalid maxWatchChains %d, fallback to 100", c.conf.MaxWatchChains)
		return 100
	}
	return c.conf.MaxWatchChains
}

// maxEventsPerPage 返回单批处理的最大事件数，超过则切块 hand-off，防御零值。
// CC 协议单页上限为 200，此配置用于防御未来上限调整并控制单批处理量
func (c *cmdbResourceWatcher) maxEventsPerPage() int {
	if c.conf.MaxEventsPerPage < 1 {
		logs.Warnf("[CMDB Watch] invalid maxEventsPerPage %d, fallback to 200", c.conf.MaxEventsPerPage)
		return 200
	}
	return c.conf.MaxEventsPerPage
}

// loadCursor 从 config 表加载游标
func (c *cmdbResourceWatcher) loadCursorWithRetry(kt *kit.Kit, cursorKey string) (string, error) {
	for kt.Ctx.Err() == nil {
		cursor, err := c.loadCursor(kt, cursorKey)
		if err == nil {
			return cursor, nil
		}
		select {
		case <-kt.Ctx.Done():
			return "", kt.Ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return "", kt.Ctx.Err()
}

func (c *cmdbResourceWatcher) loadCursor(kt *kit.Kit, cursorKey string) (string, error) {
	existing, err := c.dao.Config().GetConfig(kt, cursorKey)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logs.Warnf("[CMDB Watch] get cursor from db failed, key=%s, err=%v", cursorKey, err)
		return "", err
	}
	if existing == nil || existing.Value == "" {
		return "", nil
	}

	logs.Infof("[CMDB Watch] loaded cursor from db: key=%s, cursor=%s", cursorKey, existing.Value)
	return existing.Value, nil
}

// persistCursor 游标写穿落库，失败交给处理器重试。
func (c *cmdbResourceWatcher) persistCursor(kt *kit.Kit, cursorKey, cursor string) error {
	if err := c.dao.Config().UpsertConfig(kt, []*table.Config{{Key: cursorKey, Value: cursor}}); err != nil {
		logs.Errorf("[CMDB Watch] update cursor failed, key=%s, cursor=%s, err=%v", cursorKey, cursor, err)
		return err
	}
	return nil
}

// isCursorExpiredError 判断是否为游标过期错误（CC 1103007，事件节点不存在）
func isCursorExpiredError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, errCursorExpiredSign) || strings.Contains(msg, errCursorMissingSign)
}

// runBizGroups 按业务分组有限并发执行，并发度受 handlerConcurrency 约束，返回首个失败错误。
// 不使用 errgroup.WithContext 派生 ctx 中断兄弟任务：单业务失败不影响其他业务执行完毕，
// 整页失败由上游 processPageWithRetry 统一重试
func (c *cmdbResourceWatcher) runBizGroups(processesByBiz map[int][]bkcmdb.ProcessInfo,
	fn func(bizID int, procs []bkcmdb.ProcessInfo) error) error {

	if len(processesByBiz) == 0 {
		return nil
	}

	g := new(errgroup.Group)
	g.SetLimit(c.conf.HandlerConcurrency)

	for bizID, procs := range processesByBiz {
		if len(procs) == 0 {
			continue
		}
		g.Go(func() error {
			return fn(bizID, procs)
		})
	}

	return g.Wait()
}

func groupProcessEventsByBiz(events []bkcmdb.BkEventObj) (map[int][]bkcmdb.ProcessInfo, error) {
	result := make(map[int][]bkcmdb.ProcessInfo)
	for _, event := range events {
		var p bkcmdb.ProcessInfo
		if err := event.Decode(&p); err != nil {
			return nil, fmt.Errorf("decode process event: %w", err)
		}
		if !validCMDBID(p.BkBizID) || !validCMDBID(p.BkProcessID) || !validCMDBID(p.ServiceInstanceID) {
			return nil, fmt.Errorf("invalid process event identity, cursor=%s", event.BkCursor)
		}
		result[p.BkBizID] = append(result[p.BkBizID], p)
	}
	return result, nil
}

// validCMDBID 防止缺失、负数或溢出的 ID 被转换为数据库资源 ID。
func validCMDBID(id int) bool {
	return id > 0 && uint64(id) <= math.MaxUint32
}

// dispatchProcessStateSyncTasks 将进程状态同步任务派发给 TaskManager。
// 量控制说明：Dispatch 仅为异步入队（machinery/etcd broker），实际执行由全局
// worker 池（defaultWorkerNum=200）消费，入队速率不会直接打爆下游；
// 单次派发量由事件页大小（maxEventsPerPage）间接约束
func (c *cmdbResourceWatcher) dispatchProcessStateSyncTasks(res *cmdb.SyncProcessResult) {
	for _, item := range res.Items {
		bizID := item.Process.Attachment.BizID
		tenantID := ""
		if item.Process != nil && item.Process.Attachment != nil {
			tenantID = item.Process.Attachment.TenantID
		}
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

// handleEvent 根据资源和事件类型分派处理
func (c *cmdbResourceWatcher) handleEvent(kt *kit.Kit, resource bkcmdb.BkEventObj) error {
	switch resource.BkResource {
	case bkcmdb.ResourceSet:
		return c.handleSetEvent(kt, resource)
	case bkcmdb.ResourceModule:
		return c.handleModuleEvent(kt, resource)
	default:
		logs.Warnf("unknown CMDB resource type: %s", resource.BkResource)
		return nil
	}
}

// handleSetEvent 处理集群(Set)事件
func (c *cmdbResourceWatcher) handleSetEvent(kt *kit.Kit, resource bkcmdb.BkEventObj) error {
	var result bkcmdb.SetInfo
	if err := resource.Decode(&result); err != nil {
		logs.Errorf("[CMDB][SetSync] decode set resource failed, resource=%v, err=%v", resource, err)
		return fmt.Errorf("decode set resource failed: %w", err)
	}
	if !validCMDBID(result.BkBizID) || !validCMDBID(result.BkSetID) {
		return fmt.Errorf("invalid set event identity, cursor=%s", resource.BkCursor)
	}
	bizID := uint32(result.BkBizID)
	setID := uint32(result.BkSetID)
	setName := result.BkSetName
	eventType := resource.BkEventType

	logPrefix := fmt.Sprintf("[CMDB][SetSync][biz=%d][set=%d][event=%s]", bizID, setID, eventType)

	switch eventType {
	case bkcmdb.EventUpdate:
		if err := c.dao.Process().UpdateSelectedFields(kt, bizID, map[string]any{"set_name": setName},
			c.dao.GenQuery().Process.SetID.Eq(setID)); err != nil {
			logs.Errorf("update set name failed to %s, setName=%s, err=%v", logPrefix, setName, err)
			return fmt.Errorf("update set name failed: %w", err)
		}
		logs.Infof("update set name success: %s", logPrefix)
	case bkcmdb.EventDelete:
		// 获取属于该集群下的进程
		tx := c.dao.GenQuery().Begin()
		processIDs, err := c.dao.Process().GetBySetIDWithTx(kt, tx, bizID, setID)
		if err != nil {
			logs.Errorf("[ERROR] failed to query processIDs for bizID=%d, setID=%d: %v", bizID, setID, err)
			if rbErr := tx.Rollback(); rbErr != nil {
				logs.Errorf("[ERROR] rollback failed for bizID=%d: %v", bizID, rbErr)
			}
			return fmt.Errorf("query processes by set failed: %w", err)
		}
		err = cmdb.MarkProcessDeletedAndCleanInstancesTx(kt, c.dao, tx, bizID, processIDs)
		if err != nil {
			logs.Errorf("[ERROR] delete stopped/unmanaged failed for bizID=%d, setID=%d, processIDs=%v: %v",
				bizID, setID, processIDs, err)
			if rbErr := tx.Rollback(); rbErr != nil {
				logs.Errorf("[ERROR] rollback failed for bizID=%d: %v", bizID, rbErr)
			}
			return fmt.Errorf("mark processes deleted failed: %w", err)
		}
		if err := tx.Commit(); err != nil {
			logs.Errorf("commit failed for biz %d: %v", bizID, err)
			return fmt.Errorf("commit set delete failed: %w", err)
		}
	default:
		logs.Warnf("%s unknown event type: %s", logPrefix, eventType.String())
	}

	return nil
}

// handleModuleEvent 处理模块(Module)事件
func (c *cmdbResourceWatcher) handleModuleEvent(kt *kit.Kit, resource bkcmdb.BkEventObj) error {
	var result bkcmdb.ModuleInfo
	if err := resource.Decode(&result); err != nil {
		logs.Errorf("[CMDB][ModuleSync] decode module resource failed, resource=%v, err=%v", resource, err)
		return fmt.Errorf("decode module resource failed: %w", err)
	}
	if !validCMDBID(result.BkBizID) || !validCMDBID(result.BkSetID) || !validCMDBID(result.BkModuleID) {
		return fmt.Errorf("invalid module event identity, cursor=%s", resource.BkCursor)
	}
	bizID := uint32(result.BkBizID)
	setID := uint32(result.BkSetID)
	modID := uint32(result.BkModuleID)
	moduleName := result.BkModuleName
	eventType := resource.BkEventType

	logPrefix := fmt.Sprintf("[CMDB][ModuleSync][biz=%d][set=%d][module=%d][event=%s]", bizID, modID, setID, eventType)

	switch eventType {
	case bkcmdb.EventUpdate:
		if err := c.dao.Process().UpdateSelectedFields(kt, bizID, map[string]any{"module_name": moduleName},
			c.dao.GenQuery().Process.SetID.Eq(setID),
			c.dao.GenQuery().Process.ModuleID.Eq(modID)); err != nil {
			logs.Errorf("update module name failed to %s, module_name=%s, err=%v", logPrefix, moduleName, err)
			return fmt.Errorf("update module name failed: %w", err)
		}
		logs.Infof("update module name success: %s", logPrefix)
	case bkcmdb.EventDelete:
		// 获取属于该模块下的进程
		tx := c.dao.GenQuery().Begin()
		processIDs, err := c.dao.Process().GetByModuleIDWithTx(kt, tx, bizID, modID)
		if err != nil {
			logs.Errorf("[ERROR] failed to query processIDs for bizID=%d, modID=%d: %v", bizID, modID, err)
			if rbErr := tx.Rollback(); rbErr != nil {
				logs.Errorf("[ERROR] rollback failed for bizID=%d: %v", bizID, rbErr)
			}
			return fmt.Errorf("query processes by module failed: %w", err)
		}
		err = cmdb.MarkProcessDeletedAndCleanInstancesTx(kt, c.dao, tx, bizID, processIDs)
		if err != nil {
			logs.Errorf("[ERROR] delete stopped/unmanaged failed for bizID=%d, modID=%d, processIDs=%v: %v",
				bizID, modID, processIDs, err)
			if rbErr := tx.Rollback(); rbErr != nil {
				logs.Errorf("[ERROR] rollback failed for bizID=%d: %v", bizID, rbErr)
			}
			return fmt.Errorf("mark processes deleted failed: %w", err)
		}
		if err := tx.Commit(); err != nil {
			logs.Errorf("commit failed for biz %d: %v", bizID, err)
			return fmt.Errorf("commit module delete failed: %w", err)
		}
	default:
		logs.Warnf("%s unknown event type: %s", logPrefix, eventType.String())
	}

	return nil
}

func (c *cmdbResourceWatcher) handleProcessCreateEventsBatch(kt *kit.Kit, events []bkcmdb.BkEventObj) error {
	if len(events) == 0 {
		return nil
	}
	logs.Infof("[ProcessSync] create triggered, events=%v", events)
	processesByBiz, err := groupProcessEventsByBiz(events)
	if err != nil {
		return err
	}
	if len(processesByBiz) == 0 {
		return nil
	}

	return c.runBizGroups(processesByBiz, func(bizID int, procs []bkcmdb.ProcessInfo) error {
		if !c.svc.IsProcessConfigViewEnabled(uint32(bizID)) {
			logs.Infof("[CMDB][ProcessSync] skip biz %d: process config view not enabled", bizID)
			return nil
		}

		svc := cmdb.NewSyncCMDBService(kt.TenantID, bizID, c.cmdb, c.gse, c.dao)
		res, err := svc.SyncBizProcessesByIDsWithMode(kt.Ctx, procs)
		if err != nil {
			logs.Errorf("[CMDB][ProcessSync] create sync failed, bizID=%d, err=%v", bizID, err)
			return fmt.Errorf("sync biz %d processes failed: %w", bizID, err)
		}

		c.dispatchProcessStateSyncTasks(res)
		return nil
	})
}

func (c *cmdbResourceWatcher) collectDeleteProcessIDs(kt *kit.Kit,
	processesByBiz map[int][]bkcmdb.ProcessInfo) (map[uint32][]uint32, error) {

	result := make(map[uint32][]uint32)

	for bizID, procs := range processesByBiz {
		bid := uint32(bizID)

		for _, proc := range procs {
			dbProc, err := c.dao.Process().GetProcByBizScvProc(kt, bid, uint32(proc.ServiceInstanceID), uint32(proc.BkProcessID))
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return nil, fmt.Errorf("query process for biz %d: %w", bizID, err)
			}

			if dbProc.ID > 0 {
				result[bid] = append(result[bid], dbProc.ID)
			}
		}
	}

	return result, nil
}

func (c *cmdbResourceWatcher) handleProcessDeleteEventsBatch(kt *kit.Kit, events []bkcmdb.BkEventObj) error {
	if len(events) == 0 {
		return nil
	}
	processesByBiz, err := groupProcessEventsByBiz(events)
	if err != nil {
		return err
	}
	if len(processesByBiz) == 0 {
		return nil
	}

	processIDsByBiz, err := c.collectDeleteProcessIDs(kt, processesByBiz)
	if err != nil {
		return err
	}
	if len(processIDsByBiz) == 0 {
		return nil
	}

	g := new(errgroup.Group)
	g.SetLimit(c.conf.HandlerConcurrency)

	for bizID, procIDs := range processIDsByBiz {
		if len(procIDs) == 0 {
			continue
		}
		g.Go(func() error {
			if !c.svc.IsProcessConfigViewEnabled(bizID) {
				logs.Infof("[CMDB][ProcessSync] skip biz %d: process config view not enabled", bizID)
				return nil
			}

			tx := c.dao.GenQuery().Begin()
			if err := cmdb.MarkProcessDeletedAndCleanInstancesTx(kt, c.dao, tx, bizID, procIDs); err != nil {
				_ = tx.Rollback()
				logs.Errorf("[CMDB][ProcessSync] delete failed, bizID=%d, procIDs=%v, err=%v", bizID, procIDs, err)
				return fmt.Errorf("delete biz %d processes failed: %w", bizID, err)
			}

			if err := cmdb.CheckAndMarkHostAliasConflicts(kt, c.dao, tx, bizID); err != nil {
				_ = tx.Rollback()
				logs.Errorf("[CMDB][ProcessSync] conflict check failed, bizID=%d, err=%v", bizID, err)
				return fmt.Errorf("check host alias conflicts for biz %d failed: %w", bizID, err)
			}

			if err := tx.Commit(); err != nil {
				logs.Errorf("[CMDB][ProcessSync] commit failed, bizID=%d, err=%v", bizID, err)
				return fmt.Errorf("commit delete for biz %d failed: %w", bizID, err)
			}
			return nil
		})
	}

	return g.Wait()
}

func (c *cmdbResourceWatcher) handleProcessUpdateEventsBatch(kt *kit.Kit, events []bkcmdb.BkEventObj) error {
	if len(events) == 0 {
		return nil
	}
	logs.Infof("[UpdateProcess] triggered, events=%v", events)
	processesByBiz, err := groupProcessEventsByBiz(events)
	if err != nil {
		return err
	}

	return c.runBizGroups(processesByBiz, func(bizID int, procs []bkcmdb.ProcessInfo) error {
		if !c.svc.IsProcessConfigViewEnabled(uint32(bizID)) {
			logs.Infof("[CMDB][ProcessSync] skip biz %d: process config view not enabled", bizID)
			return nil
		}

		svc := cmdb.NewSyncCMDBService(kt.TenantID, bizID, c.cmdb, c.gse, c.dao)
		res, err := svc.UpdateProcess(kt.Ctx, procs)
		if err != nil {
			logs.Errorf("[CMDB][ProcessSync] update sync failed, bizID=%d, err=%v", bizID, err)
			return fmt.Errorf("update biz %d processes failed: %w", bizID, err)
		}

		c.dispatchProcessStateSyncTasks(res)
		return nil
	})
}

func getResourceCursorKey(tenantID, resource string) string {
	cursorKey := fmt.Sprintf("resource:%s:cursor", resource)
	if tenantID != "" {
		cursorKey = fmt.Sprintf("%s-%s", tenantID, cursorKey)
	}
	return cursorKey
}
