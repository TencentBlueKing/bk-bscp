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
	defaultResourceWatcherTime = 10 * time.Second
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
}

func (c *cmdbResourceWatcher) Run() {
	logs.Infof("Start listening cmdb resource changes")
	notifier := shutdown.AddNotifier()

	go func() {
		ticker := time.NewTicker(defaultResourceWatcherTime)
		defer ticker.Stop()
		for {

			kt := kit.New()
			ctx, cancel := context.WithCancel(kt.Ctx)
			kt.Ctx = ctx

			select {
			case <-notifier.Signal:
				logs.Infof("stop synchronizing cmdb data success")
				cancel()
				notifier.Done()
				return
			case <-ticker.C:
				if !c.state.IsMaster() {
					logs.Infof("current service instance is slave, skip sync cmdb")
					continue
				}
				c.watchCMDBResourcesByTenant(kt)
			}

		}
	}()
}

// watchCMDBResourcesByTenant 按租户监听 CMDB 资源变化
func (c *cmdbResourceWatcher) watchCMDBResourcesByTenant(kt *kit.Kit) {
	// 多租户模式：获取所有启用的租户并逐个监听
	if cc.DataService().FeatureFlags.EnableMultiTenantMode {
		tenants, err := bkuser.ListEnabledTenants(kt.Ctx)
		if err != nil {
			logs.Errorf("[CMDB Watch] failed to list tenants: %v", err)
			return
		}

		if len(tenants) == 0 {
			logs.Warnf("[CMDB Watch] no enabled tenants found")
			return
		}

		for _, tenant := range tenants {
			kt.TenantID = tenant.ID
			c.watchResourcesForTenant(kt)
		}
		return
	}

	// 单租户模式
	c.watchResourcesForTenant(kt)
}

// watchResourcesForTenant 为单个租户监听所有资源类型
func (c *cmdbResourceWatcher) watchResourcesForTenant(kt *kit.Kit) {
	// 顺序监听每种资源类型
	for _, res := range []bkcmdb.ResourceType{
		bkcmdb.ResourceSet,
		bkcmdb.ResourceModule,
		bkcmdb.ResourceProcess,
	} {
		if err := c.watchCMDBResources(kt, res); err != nil {
			logs.Errorf("[CMDB Watch] tenant=%s watch %s resource failed: %v", kt.TenantID, res.String(), err)
		}
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
	done := false
	for {
		resp, err := c.cmdb.ResourceWatch(kt.Ctx, &bkcmdb.WatchResourceRequest{
			BkCursor:          cursor,
			BkResource:        resource.String(),
			BkEventTypes:      []string{bkcmdb.EventCreate.String(), bkcmdb.EventUpdate.String(), bkcmdb.EventDelete.String()},
			BkFields:          fields,
			BkStartFrom:       new(int64),
			BkSupplierAccount: "0",
		})
		if err != nil {
			return fmt.Errorf("request CMDB watch for %s failed: %w", resource, err)
		}

		processEvents := map[bkcmdb.EventType][]bkcmdb.BkEventObj{}

		// 处理事件
		for _, event := range resp.BkEvents {
			logs.Infof("[CMDB Watch] resource=%s event=%s cursor=%s", event.BkResource, event.BkEventType, event.BkCursor)

			// 空事件: 仅更新游标并退出到下一个资源
			if event.BkEventType == "" {
				logs.Infof("[CMDB Watch] resource=%s: empty event detected, update cursor=%s and break", resource, event.BkCursor)
				cursor = event.BkCursor
				if err := c.dao.Config().UpsertConfig(kt, []*table.Config{{
					Key:   cursorKey,
					Value: cursor,
				}}); err != nil {
					logs.Errorf("[CMDB][Watch] update cursor failed, resource=%v, err=%v", resource, err)
				}
				done = true
				break
			}

			switch resource {
			case bkcmdb.ResourceProcess:
				processEvents[event.BkEventType] = append(processEvents[event.BkEventType], event)
			default:
				// 非 process 资源即时处理
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

		// 如果刚才的循环被空事件 break，就退出到下一个资源
		// 若检测到空事件，跳出外层循环
		if done {
			break
		}

	}

	return nil
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
