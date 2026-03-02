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
	"sync"
	"time"

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
	defaultMasterCheckInterval   = 10 * time.Second
	defaultTenantRefreshInterval = 30 * time.Second
	defaultInitialBackoff        = 1 * time.Second
	defaultMaxBackoff            = 60 * time.Second
)

// NewCmdbResourceWatcher 监听cmdb资源变化
func NewCmdbResourceWatcher(dao dao.Set, sd serviced.Service, cmdb bkcmdb.Service, gse *gsecomponents.Service,
	svc *service.Service, taskManager *task.TaskManager) *cmdbResourceWatcher {
	return &cmdbResourceWatcher{
		dao:         dao,
		state:       sd,
		cmdb:        cmdb,
		svc:         svc,
		gse:         gse,
		taskManager: taskManager,
	}
}

type cmdbResourceWatcher struct {
	dao         dao.Set
	state       serviced.Service
	cmdb        bkcmdb.Service
	gse         *gsecomponents.Service
	svc         *service.Service
	taskManager *task.TaskManager
	wg          sync.WaitGroup
}

func (c *cmdbResourceWatcher) Run() {
	logs.Infof("Start listening cmdb resource changes")
	notifier := shutdown.AddNotifier()
	ctx, cancel := context.WithCancel(context.Background())

	if cc.DataService().FeatureFlags.EnableMultiTenantMode {
		c.wg.Add(1)
		go c.manageTenantWatchers(ctx)
	} else {
		c.startResourceWatchers(ctx, "")
	}

	go func() {
		<-notifier.Signal
		logs.Infof("stopping cmdb resource watchers...")
		cancel()
		c.wg.Wait()
		logs.Infof("all cmdb resource watchers stopped")
		notifier.Done()
	}()
}

// startResourceWatchers 为指定租户的每种资源类型启动独立的 watch goroutine
func (c *cmdbResourceWatcher) startResourceWatchers(ctx context.Context, tenantID string) {
	for _, res := range []bkcmdb.ResourceType{
		bkcmdb.ResourceSet,
		bkcmdb.ResourceModule,
		bkcmdb.ResourceProcess,
	} {
		c.wg.Add(1)
		go c.watchLoop(ctx, tenantID, res)
	}
}

// manageTenantWatchers 多租户模式下动态管理每个租户的 watch goroutine
func (c *cmdbResourceWatcher) manageTenantWatchers(ctx context.Context) {
	defer c.wg.Done()
	activeTenants := make(map[string]context.CancelFunc)
	defer func() {
		for _, cancelFn := range activeTenants {
			cancelFn()
		}
	}()

	c.refreshTenantWatchers(ctx, activeTenants)

	ticker := time.NewTicker(defaultTenantRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refreshTenantWatchers(ctx, activeTenants)
		}
	}
}

// refreshTenantWatchers 刷新租户列表，启动新租户的 watcher、停止已移除租户的 watcher
func (c *cmdbResourceWatcher) refreshTenantWatchers(ctx context.Context, activeTenants map[string]context.CancelFunc) {
	tenants, err := bkuser.ListEnabledTenants(ctx)
	if err != nil {
		logs.Errorf("[CMDB Watch] failed to list tenants: %v", err)
		return
	}

	if len(tenants) == 0 {
		logs.Warnf("[CMDB Watch] no enabled tenants found")
	}

	// currentIDs 记录本次查询到的租户 ID，用于与 activeTenants 对比，清理已移除的租户
	currentIDs := make(map[string]bool, len(tenants))
	for _, t := range tenants {
		currentIDs[t.ID] = true
		if _, exists := activeTenants[t.ID]; !exists {
			tenantCtx, tenantCancel := context.WithCancel(ctx)
			activeTenants[t.ID] = tenantCancel
			c.startResourceWatchers(tenantCtx, t.ID)
			logs.Infof("[CMDB Watch] started watchers for tenant %s", t.ID)
		}
	}

	// 清理已移除的租户
	for id, cancelFn := range activeTenants {
		if !currentIDs[id] {
			cancelFn()
			delete(activeTenants, id)
			logs.Infof("[CMDB Watch] stopped watchers for removed tenant %s", id)
		}
	}
}

// watchLoop 持续长轮询循环：不断调用 watchCMDBResources，含 Master 检查与指数退避
func (c *cmdbResourceWatcher) watchLoop(ctx context.Context, tenantID string, resource bkcmdb.ResourceType) {
	defer c.wg.Done()
	backoff := defaultInitialBackoff
	logs.Infof("[CMDB Watch] tenant=%s starting watch loop for %s", tenantID, resource)

	for {
		select {
		case <-ctx.Done():
			logs.Infof("[CMDB Watch] tenant=%s watch loop for %s stopped", tenantID, resource)
			return
		default:
		}

		// 非 Master 时不消费事件，定期检查角色变化
		if !c.state.IsMaster() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(defaultMasterCheckInterval):
			}
			continue
		}

		kt := kit.New()
		kt.Ctx = ctx
		kt.TenantID = tenantID

		// cmdb api 支持长链接，无事件时会 hold 连接约 20s
		if err := c.watchCMDBResources(kt, resource); err != nil {
			logs.Errorf("[CMDB Watch] tenant=%s watch %s failed: %v, retry after %v",
				tenantID, resource, err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, defaultMaxBackoff)
			continue
		}
		backoff = defaultInitialBackoff
	}
}

// watchCMDBResources 监听并处理指定资源类型
func (c *cmdbResourceWatcher) watchCMDBResources(kt *kit.Kit, resource bkcmdb.ResourceType) error {
	fields := []string{}
	// 生成带租户前缀的游标 key
	cursorKey := fmt.Sprintf("resource:%s:cursor", resource.String())
	if kt.TenantID != "" {
		cursorKey = fmt.Sprintf("%s-%s", kt.TenantID, cursorKey)
	}
	var cursor string
	switch resource {
	case bkcmdb.ResourceSet:
		fields = []string{"bk_biz_id", "bk_set_id", "bk_set_name", "bk_set_env", "set_template_id"}
	case bkcmdb.ResourceModule:
		fields = []string{"bk_biz_id", "bk_set_id", "bk_module_id", "bk_module_name"}
	}

	// 第一次从表中读取游标
	existing, err := c.dao.Config().GetConfig(kt, cursorKey)
	if err != nil {
		logs.Warnf("[CMDB Watch] get cursor from db failed, key=%s, err=%v", cursorKey, err)
	} else if existing != nil {
		cursor = existing.Value
		logs.Infof("[CMDB Watch] loaded cursor from db: resource=%s cursor=%s", resource, cursor)
	}
	for {
		select {
		case <-kt.Ctx.Done():
			return nil
		default:
		}

		resp, err := c.cmdb.ResourceWatch(kt.Ctx, &bkcmdb.WatchResourceRequest{
			BkCursor:     cursor,
			BkResource:   resource.String(),
			BkEventTypes: []string{bkcmdb.EventCreate.String(), bkcmdb.EventUpdate.String(), bkcmdb.EventDelete.String()},
			BkFields:     fields,
			BkStartFrom:  new(int64),
		})
		if err != nil {
			return fmt.Errorf("request CMDB watch for %s failed: %w", resource, err)
		}

		// 异常防护：正常情况下 CMDB 总会返回至少一条事件（真实事件或长轮询超时的空事件）
		if len(resp.BkEvents) == 0 {
			return fmt.Errorf("CMDB watch for %s returned empty events (BkWatched=%v)", resource, resp.BkWatched)
		}

		processEvents := map[bkcmdb.EventType][]bkcmdb.BkEventObj{}
		// caughtUp 标记是否已消费完所有积压事件（收到空事件），为 true 时返回让外层发起下一轮长轮询
		caughtUp := false

		for _, event := range resp.BkEvents {
			logs.Infof("[CMDB Watch] resource=%s event=%s cursor=%s", event.BkResource, event.BkEventType, event.BkCursor)

			if event.BkEventType == "" {
				logs.Infof("[CMDB Watch] resource=%s: no pending events, cursor=%s", resource, event.BkCursor)
				cursor = event.BkCursor
				if err := c.dao.Config().UpsertConfig(kt, []*table.Config{{
					Key:   cursorKey,
					Value: cursor,
				}}); err != nil {
					logs.Errorf("[CMDB][Watch] update cursor failed, resource=%v, err=%v", resource, err)
				}
				caughtUp = true
				break
			}

			switch resource {
			case bkcmdb.ResourceProcess:
				processEvents[event.BkEventType] = append(processEvents[event.BkEventType], event)
			default:
				c.handleEvent(kt, event)
			}

			cursor = event.BkCursor
			if err := c.dao.Config().UpsertConfig(kt, []*table.Config{{
				Key:   cursorKey,
				Value: cursor,
			}}); err != nil {
				logs.Errorf("[CMDB][Watch] update cursor failed, resource=%v, err=%v", resource, err)
			}
		}

		if events := processEvents[bkcmdb.EventCreate]; len(events) > 0 {
			c.handleProcessCreateEventsBatch(kt, events)
		}

		if events := processEvents[bkcmdb.EventDelete]; len(events) > 0 {
			c.handleProcessDeleteEventsBatch(kt, events)
		}

		if events := processEvents[bkcmdb.EventUpdate]; len(events) > 0 {
			c.handleProcessUpdateEventsBatch(kt, events)
		}

		if caughtUp {
			return nil
		}
	}
}

func groupProcessEventsByBiz(events []bkcmdb.BkEventObj) map[int][]bkcmdb.ProcessInfo {
	result := make(map[int][]bkcmdb.ProcessInfo)

	for _, event := range events {
		if len(event.BkDetail) == 0 {
			logs.Warnf("[CMDB][ProcessSync] empty detail, resource=%s, event=%s",
				event.BkResource.String(), event.BkEventType)
			continue
		}

		var p bkcmdb.ProcessInfo
		if err := event.Decode(&p); err != nil {
			logs.Errorf(
				"[CMDB][ProcessSync] decode process failed, err=%v, resource=%s, event=%s",
				err, event.BkResource.String(), event.BkEventType,
			)
			continue
		}

		result[p.BkBizID] = append(result[p.BkBizID], p)
	}

	return result
}

func (c *cmdbResourceWatcher) dispatchProcessStateSyncTasks(res *cmdb.SyncProcessResult) {
	for _, item := range res.Items {
		bizID := item.Process.Attachment.BizID
		taskObj, err := task.NewByTaskBuilder(
			gse.NewProcessStateSyncTask(c.dao, bizID, item.Process, item.Instances),
		)
		if err != nil {
			logs.Errorf("[CMDB][ProcessSync] create gse task failed, bizID=%d, err=%v", bizID, err)
			continue
		}
		c.taskManager.Dispatch(taskObj)
	}
}

// handleEvent 根据资源和事件类型分派处理
func (c *cmdbResourceWatcher) handleEvent(kt *kit.Kit, resource bkcmdb.BkEventObj) {
	switch resource.BkResource {
	case bkcmdb.ResourceSet:
		c.handleSetEvent(kt, resource)
	case bkcmdb.ResourceModule:
		c.handleModuleEvent(kt, resource)
	default:
		logs.Warnf("unknown CMDB resource type: %s", resource)
	}
}

// handleSetEvent 处理集群(Set)事件
func (c *cmdbResourceWatcher) handleSetEvent(kt *kit.Kit, resource bkcmdb.BkEventObj) {
	result := new(bkcmdb.SetInfo)
	if err := resource.Decode(&result); err != nil {
		logs.Errorf("[CMDB][SetSync] decode set resource failed, resource=%v, err=%v", resource, err)
		return
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
			return
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
				return
			}
			return
		}
		err = cmdb.DeleteInstanceStoppedUnmanaged(kt, c.dao, tx, bizID, processIDs)
		if err != nil {
			logs.Errorf("[ERROR] delete stopped/unmanaged failed for bizID=%d, setID=%d, processIDs=%v: %v",
				bizID, setID, processIDs, err)
			if rbErr := tx.Rollback(); rbErr != nil {
				logs.Errorf("[ERROR] rollback failed for bizID=%d: %v", bizID, rbErr)
				return
			}
			return
		}
		if err := tx.Commit(); err != nil {
			logs.Errorf("commit failed for biz %d: %v", bizID, err)
			return
		}
	default:
		logs.Warnf("%s unknown event type: %s", logPrefix, eventType.String())
	}

}

// handleModuleEvent 处理模块(Module)事件
func (c *cmdbResourceWatcher) handleModuleEvent(kt *kit.Kit, resource bkcmdb.BkEventObj) {
	var result bkcmdb.ModuleInfo
	if err := resource.Decode(&result); err != nil {
		logs.Errorf("[CMDB][ModuleSync] decode module resource failed, resource=%v, err=%v", resource, err)
		return
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
			return
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
				return
			}
			return
		}
		err = cmdb.DeleteInstanceStoppedUnmanaged(kt, c.dao, tx, bizID, processIDs)
		if err != nil {
			logs.Errorf("[ERROR] delete stopped/unmanaged failed for bizID=%d, modID=%d, processIDs=%v: %v",
				bizID, modID, processIDs, err)
			if rbErr := tx.Rollback(); rbErr != nil {
				logs.Errorf("[ERROR] rollback failed for bizID=%d: %v", bizID, rbErr)
				return
			}
			return
		}
		if err := tx.Commit(); err != nil {
			logs.Errorf("commit failed for biz %d: %v", bizID, err)
			return
		}
	default:
		logs.Warnf("%s unknown event type: %s", logPrefix, eventType.String())
	}
}

func (c *cmdbResourceWatcher) handleProcessCreateEventsBatch(kt *kit.Kit, events []bkcmdb.BkEventObj) {
	logs.Infof("[ProcessSync] create triggered, events=%v", events)
	processesByBiz := groupProcessEventsByBiz(events)
	if len(processesByBiz) == 0 {
		return
	}

	for bizID, procs := range processesByBiz {
		if len(procs) == 0 {
			continue
		}
		if !cc.DataService().FeatureFlags.IsProcessConfigViewEnabled(uint32(bizID)) {
			logs.Infof("[CMDB][ProcessSync] skip biz %d: process config view not enabled", bizID)
			continue
		}

		svc := cmdb.NewSyncCMDBService(kt.TenantID, bizID, c.cmdb, c.dao)
		res, err := svc.SyncByProcessIDs(kt.Ctx, procs)
		if err != nil {
			logs.Errorf("[CMDB][ProcessSync] create sync failed, bizID=%d, err=%v", bizID, err)
			continue
		}

		c.dispatchProcessStateSyncTasks(res)
	}
}

func (c *cmdbResourceWatcher) collectDeleteProcessIDs(kt *kit.Kit, processesByBiz map[int][]bkcmdb.ProcessInfo) map[uint32][]uint32 {

	result := make(map[uint32][]uint32)

	for bizID, procs := range processesByBiz {
		bid := uint32(bizID)

		for _, proc := range procs {
			dbProc, err := c.dao.Process().GetProcByBizScvProc(kt, bid, uint32(proc.ServiceInstanceID), uint32(proc.BkProcessID))
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				logs.Errorf("[CMDB][ProcessSync] query process failed, biz=%d, err=%v", bizID, err)
				continue
			}

			if dbProc.ID > 0 {
				result[bid] = append(result[bid], dbProc.ID)
			}
		}
	}

	return result
}

func (c *cmdbResourceWatcher) handleProcessDeleteEventsBatch(kt *kit.Kit, events []bkcmdb.BkEventObj) {
	processesByBiz := groupProcessEventsByBiz(events)
	if len(processesByBiz) == 0 {
		return
	}

	processIDsByBiz := c.collectDeleteProcessIDs(kt, processesByBiz)

	for bizID, procIDs := range processIDsByBiz {
		if len(procIDs) == 0 {
			continue
		}
		if !cc.DataService().FeatureFlags.IsProcessConfigViewEnabled(bizID) {
			logs.Infof("[CMDB][ProcessSync] skip biz %d: process config view not enabled", bizID)
			continue
		}

		tx := c.dao.GenQuery().Begin()
		if err := cmdb.DeleteInstanceStoppedUnmanaged(kt, c.dao, tx, bizID, procIDs); err != nil {
			_ = tx.Rollback()
			logs.Errorf("[CMDB][ProcessSync] delete failed, bizID=%d, procIDs=%v, err=%v", bizID, procIDs, err)
			continue
		}

		if err := cmdb.CheckAndMarkHostAliasConflicts(kt, c.dao, tx, bizID); err != nil {
			_ = tx.Rollback()
			logs.Errorf("[CMDB][ProcessSync] conflict check failed, bizID=%d, err=%v", bizID, err)
			continue
		}

		if err := tx.Commit(); err != nil {
			logs.Errorf("[CMDB][ProcessSync] commit failed, bizID=%d, err=%v", bizID, err)
		}
	}
}

func (c *cmdbResourceWatcher) handleProcessUpdateEventsBatch(kt *kit.Kit, events []bkcmdb.BkEventObj) {
	logs.Infof("[UpdateProcess] triggered, events=%v", events)
	processesByBiz := groupProcessEventsByBiz(events)

	for bizID, procs := range processesByBiz {
		if len(procs) == 0 {
			continue
		}
		if !cc.DataService().FeatureFlags.IsProcessConfigViewEnabled(uint32(bizID)) {
			logs.Infof("[CMDB][ProcessSync] skip biz %d: process config view not enabled", bizID)
			continue
		}

		svc := cmdb.NewSyncCMDBService(kt.TenantID, bizID, c.cmdb, c.dao)
		res, err := svc.UpdateProcess(kt.Ctx, procs)
		if err != nil {
			logs.Errorf("[CMDB][ProcessSync] update sync failed, bizID=%d, err=%v", bizID, err)
			continue
		}

		c.dispatchProcessStateSyncTasks(res)
	}
}
