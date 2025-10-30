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
	"time"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/cmd/data-service/service"
	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/internal/processor/cmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	defaultResourceWatcherTime = 10 * time.Second
)

// NewCmdbResourceWatcher 监听cmdb资源变化
func NewCmdbResourceWatcher(dao dao.Set, sd serviced.Service, cmdb bkcmdb.Service,
	svc *service.Service) *cmdbResourceWatcher {
	return &cmdbResourceWatcher{
		dao:   dao,
		state: sd,
		cmdb:  cmdb,
		svc:   svc,
	}
}

type cmdbResourceWatcher struct {
	dao   dao.Set
	state serviced.Service
	cmdb  bkcmdb.Service
	svc   *service.Service
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
				// 顺序监听每种资源类型
				for _, res := range []bkcmdb.ResourceType{
					bkcmdb.ResourceSet,
					bkcmdb.ResourceModule,
					bkcmdb.ResourceProcess,
				} {
					if err := c.watchCMDBResources(kt, res); err != nil {
						logs.Errorf("[CMDB Watch] watch %s resource failed: %v", res.String(), err)
					}
				}

			}

		}
	}()
}

// watchCMDBResources 监听并处理指定资源类型
func (c *cmdbResourceWatcher) watchCMDBResources(kt *kit.Kit, resource bkcmdb.ResourceType) error {
	fields := []string{}
	cursorKey := fmt.Sprintf("resource:%s:cursor", resource.String())
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

		if !resp.BkWatched {
			// 没有新的事件，结束循环
			logs.Infof("[CMDB Watch] resource=%s no new events, stop watching", resource)
			break
		}

		// 处理事件
		for _, event := range resp.BkEvents {
			logs.Infof("[CMDB Watch] resource=%s event=%s cursor=%s", event.BkResource, event.BkEventType, event.BkCursor)

			c.handleEvent(kt, event)
			// 更新游标
			cursor = event.BkCursor
			config := &table.Config{
				Key:   cursorKey,
				Value: cursor,
			}
			if err := c.dao.Config().UpsertConfig(kt, []*table.Config{config}); err != nil {
				logs.Errorf("[CMDB][Watch] update cursor failed, resource=%v, err=%v", resource, err)
			}
		}

		// 若最后一个事件的 detail 为空，也说明已经没有新的事件
		lastEvent := resp.BkEvents[len(resp.BkEvents)-1]
		if lastEvent.BkDetail == nil {
			logs.Infof("[CMDB Watch] resource=%s detail is empty, all events processed", resource)
			break
		}
	}

	return nil
}

// handleEvent 根据资源和事件类型分派处理
func (c *cmdbResourceWatcher) handleEvent(kt *kit.Kit, resource bkcmdb.BkEventObj) {
	switch resource.BkResource {
	case bkcmdb.ResourceSet:
		c.handleSetEvent(kt, resource)
	case bkcmdb.ResourceModule:
		c.handleModuleEvent(kt, resource)
	case bkcmdb.ResourceProcess:
		c.handleProcessEvent(kt, resource)
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

	update := func(data map[string]any, action string) {
		if err := c.dao.Process().UpdateSelectedFields(kt, bizID, data, c.dao.GenQuery().Process.SetID.Eq(setID)); err != nil {
			logs.Errorf("%s failed to %s, data=%+v, err=%v", logPrefix, action, data, err)
			return
		}
		logs.Infof("%s success: %s", logPrefix, action)
	}

	switch eventType {
	case bkcmdb.EventUpdate:
		update(map[string]any{"set_name": setName}, "update set name")
	case bkcmdb.EventDelete:
		update(map[string]any{"cc_sync_status": table.Deleted}, "mark as deleted")
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

	update := func(data map[string]any, action string) {
		if err := c.dao.Process().UpdateSelectedFields(kt, bizID, data, c.dao.GenQuery().Process.SetID.Eq(setID),
			c.dao.GenQuery().Process.ModuleID.Eq(modID)); err != nil {
			logs.Errorf("%s failed to %s, data=%+v, err=%v", logPrefix, action, data, err)
			return
		}
		logs.Infof("%s success: %s", logPrefix, action)
	}

	switch eventType {
	case bkcmdb.EventUpdate:
		update(map[string]any{"module_name": moduleName}, "update module name")
	case bkcmdb.EventDelete:
		update(map[string]any{"cc_sync_status": table.Deleted}, "mark as deleted")
	default:
		logs.Warnf("%s unknown event type: %s", logPrefix, eventType.String())
	}
}

// handleProcessEvent 处理 CMDB 进程事件（创建、更新、删除）并同步至本地数据库。
// 1. 进程别名发生变化，需删除旧进程，新增新的进程
// 2. 更新进程数量
// 3. 更新源数据
func (c *cmdbResourceWatcher) handleProcessEvent(kt *kit.Kit, resource bkcmdb.BkEventObj) {
	var p bkcmdb.ProcessInfo
	if err := resource.Decode(&p); err != nil {
		logs.Errorf("[CMDB][ProcessSync] decode process failed, err=%v, resource=%v", err, resource)
		return
	}

	bizID, svcID, procID := uint32(p.BkBizID), uint32(p.ServiceInstanceID), uint32(p.BkProcessID)
	logPrefix := fmt.Sprintf("[CMDB][ProcessSync][biz=%d][svc=%d][proc=%d][event=%s]", bizID, svcID, procID, resource.BkEventType)

	// 查询一下表，如果数据一致，直接跳过
	procs, err := c.dao.Process().GetProcByBizScvProc(kt, bizID, svcID, procID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logs.Errorf("%s list process failed: %v", logPrefix, err)
		return
	}

	// 如果是空全量同步
	if procs == nil {
		if err := c.svc.SynchronizeCmdbData(kt.RpcCtx(), []int{p.BkBizID}); err != nil {
			logs.Errorf("sync cmdb data failed: %v", err)
		}
		return
	}

	switch resource.BkEventType {
	case bkcmdb.EventCreate:
		if err := c.svc.SynchronizeCmdbData(kt.Ctx, []int{p.BkBizID}); err != nil {
			logs.Errorf("%s sync cmdb data failed: %v", logPrefix, err)
			return
		}
	case bkcmdb.EventUpdate:
		tx := c.dao.GenQuery().Begin()
		err := c.handleProcessUpdate(kt, tx, &p, procs)
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				logs.Errorf("[ERROR] rollback failed for bizID=%d: %v", bizID, rbErr)
				return
			}
			logs.Errorf("sync process and instance data failed for biz %d: %v", bizID, err)
			return
		}

		if err := tx.Commit(); err != nil {
			logs.Errorf("commit failed for biz %d: %v", bizID, err)
			return
		}

	case bkcmdb.EventDelete:
		if err := c.dao.Process().UpdateSelectedFields(kt, bizID,
			map[string]any{"cc_sync_status": table.Deleted},
			c.dao.GenQuery().Process.ID.Eq(procs.ID),
		); err != nil {
			logs.Errorf("%s delete process failed: %v", logPrefix, err)
			return
		}
	default:
		logs.Warnf("%s unknown event: %s", logPrefix, resource.BkEventType)
	}
}

// handleProcessUpdate 处理进程的更新逻辑，包括别名变化、进程数变化、副表写入等。
func (c *cmdbResourceWatcher) handleProcessUpdate(kt *kit.Kit, tx *gen.QueryTx,
	p *bkcmdb.ProcessInfo, old *table.Process) error {

	now := time.Now().UTC()
	var toAdd, toUpdate []*table.Process
	var toDelete []uint32

	var toAddProcInst []*table.ProcessInstance

	// 别名变更 -> 删除旧数据 + 新增新进程 + 新增进程实例
	if p.BkProcessName != old.Spec.Alias {
		info := table.ProcessInfo{
			BkStartParamRegex: p.BkStartParamRegex,
			WorkPath:          p.WorkPath,
			PidFile:           p.PidFile,
			User:              p.User,
			ReloadCmd:         p.ReloadCmd,
			RestartCmd:        p.RestartCmd,
			StartCmd:          p.StartCmd,
			StopCmd:           p.StopCmd,
			FaceStopCmd:       p.FaceStopCmd,
			Timeout:           p.Timeout,
		}
		data, _ := json.Marshal(info)

		toDelete = append(toDelete, old.ID)
		toAdd = append(toAdd, &table.Process{
			Attachment: old.Attachment,
			Spec: &table.ProcessSpec{
				SetName:         old.Spec.SetName,
				ModuleName:      old.Spec.ModuleName,
				ServiceName:     old.Spec.ServiceName,
				Environment:     old.Spec.Environment,
				Alias:           p.BkProcessName,
				InnerIP:         old.Spec.InnerIP,
				CcSyncStatus:    table.Synced,
				CcSyncUpdatedAt: now,
				SourceData:      string(data),
				PrevData:        old.Spec.PrevData,
				ProcNum:         uint(p.ProcNum),
			},
			Revision: &table.Revision{CreatedAt: now},
		})

		procInst := cmdb.BuildInstances(&cmdb.ProcInst{
			ID:          p.BkProcessID,
			HostID:      int(old.Attachment.HostID),
			Name:        p.BkProcessName,
			ProcNum:     p.ProcNum,
			ProcessInfo: info,
		}, p.BkBizID, int(old.Attachment.ModuleID), now, map[int]int{}, map[int]int{})
		toAddProcInst = append(toAddProcInst, procInst...)
	}

	// 进程数变化 -> 更新
	if old.Spec.ProcNum != uint(p.ProcNum) {
		old.Spec.ProcNum = uint(p.ProcNum)
		toUpdate = append(toUpdate, &table.Process{
			ID:         old.ID,
			Attachment: old.Attachment,
			Spec:       old.Spec,
			Revision:   &table.Revision{UpdatedAt: now},
		})
	}

	// 删除
	if len(toDelete) > 0 {
		if err := c.dao.Process().UpdateSyncStatusWithTx(kt, tx, string(table.Deleted), toDelete); err != nil {
			return fmt.Errorf("mark deleted failed: %w", err)
		}
	}

	// 插入
	if len(toAdd) > 0 {
		if err := c.dao.Process().BatchCreateWithTx(kt, tx, toAdd); err != nil {
			return fmt.Errorf("insert failed: %w", err)
		}
	}

	// 更新
	if len(toUpdate) > 0 {
		if err := c.dao.Process().BatchUpdateWithTx(kt, tx, toUpdate); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
	}

	// 构建 ProcessID 映射
	idMap := make(map[string]uint32)
	for _, p := range toAdd {
		key := fmt.Sprintf("%s-%d-%d", p.Attachment.TenantID, p.Attachment.BizID, p.Attachment.CcProcessID)
		idMap[key] = p.ID
	}

	// 构建要写入的实例
	var toWriteInstances []*table.ProcessInstance
	for _, inst := range toAddProcInst {
		key := fmt.Sprintf("%s-%d-%d", inst.Attachment.TenantID, inst.Attachment.BizID, inst.Attachment.CcProcessID)
		if pid, ok := idMap[key]; ok && pid != 0 {
			inst.Attachment.ProcessID = pid
			toWriteInstances = append(toWriteInstances, inst)
		}
	}

	if len(toWriteInstances) == 0 {
		return nil
	}

	if err := c.dao.ProcessInstance().BatchCreateWithTx(kt, tx, toWriteInstances); err != nil {
		return fmt.Errorf("insert process instances failed: %w", err)
	}

	return nil
}
