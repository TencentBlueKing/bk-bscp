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

			// 正常事件: 处理 + 更新游标
			c.handleEvent(kt, event)
			cursor = event.BkCursor
			if err := c.dao.Config().UpsertConfig(kt, []*table.Config{{
				Key:   cursorKey,
				Value: cursor,
			}}); err != nil {
				logs.Errorf("[CMDB][Watch] update cursor failed, resource=%v, err=%v", resource, err)
			}
		}

		// 如果刚才的循环被空事件 break，就退出到下一个资源
		// 若检测到空事件，跳出外层循环
		if done {
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
		if err := c.svc.SynchronizeCmdbData(kt.Ctx, []int{p.BkBizID}); err != nil {
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
			logs.Errorf("[ERROR] sync process and instance data failed for biz %d: %v", bizID, err)
			return
		}

		if err := tx.Commit(); err != nil {
			logs.Errorf("commit failed for biz %d: %v", bizID, err)
			return
		}

	case bkcmdb.EventDelete:
		tx := c.dao.GenQuery().Begin()
		err := cmdb.DeleteInstanceStoppedUnmanaged(kt, c.dao, tx, bizID, []uint32{procs.ID})
		if err != nil {
			logs.Errorf("[ERROR] delete stopped/unmanaged failed for bizID=%d, processIDs=%v: %v",
				bizID, procs.ID, err)
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
		logs.Warnf("%s unknown event: %s", logPrefix, resource.BkEventType)
	}
}

// handleProcessUpdate 处理进程的更新逻辑，包括别名变化、进程数变化、副表写入等。
func (c *cmdbResourceWatcher) handleProcessUpdate(kt *kit.Kit, tx *gen.QueryTx,
	p *bkcmdb.ProcessInfo, old *table.Process) error {

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
	sourceData, err := json.Marshal(info)
	if err != nil {
		logs.Errorf("biz %d: marshal process info failed, processID=%d, err=%v, data=%+v",
			old.Attachment.BizID, old.ID, err, info)
		return err
	}

	now := time.Now().UTC()
	newSpec := *old.Spec
	newP := &table.Process{
		Attachment: old.Attachment,
		Spec:       &newSpec,
		Revision: &table.Revision{
			CreatedAt: now,
		},
	}
	newP.Attachment.CcProcessID = uint32(p.BkProcessID)
	newP.Attachment.ServiceInstanceID = uint32(p.ServiceInstanceID)
	newP.Spec.Alias = p.BkProcessName
	newP.Spec.ProcNum = uint(p.ProcNum)
	newP.Spec.SourceData = string(sourceData)

	toAdd, toUpdate, toDelete, procInst, err := cmdb.BuildProcessChanges(kt, c.dao, tx, newP, old, now, map[[2]int]int{}, map[[2]int]int{})
	if err != nil {
		logs.Errorf("biz %d: build process changes failed, processID=%d, err=%v, new=%+v, old=%+v",
			old.Attachment.BizID, old.ID, err, newP, old)
		return err
	}

	idMap := make(map[string]uint32)
	// 插入
	if toAdd != nil {
		if err := c.dao.Process().BatchCreateWithTx(kt, tx, []*table.Process{toAdd}); err != nil {
			logs.Errorf("[ProcessSync] biz=%d: insert process failed: name=%s, ccProcessID=%d, err=%v",
				p.BkBizID, toAdd.Spec.Alias, toAdd.Attachment.CcProcessID, err)
			return fmt.Errorf("insert failed: %w", err)
		}
		toAddKey := fmt.Sprintf("%s-%d-%d", toAdd.Attachment.TenantID, p.BkBizID, toAdd.Attachment.CcProcessID)
		idMap[toAddKey] = toAdd.ID
	}

	// 更新
	if toUpdate != nil {
		if err := c.dao.Process().BatchUpdateWithTx(kt, tx, []*table.Process{toUpdate}); err != nil {
			logs.Errorf("[ProcessSync] biz=%d: update process failed: id=%d, name=%s, err=%v",
				p.BkBizID, toUpdate.ID, toUpdate.Spec.Alias, err)
			return fmt.Errorf("update failed: %w", err)
		}
		toUpdatekey := fmt.Sprintf("%s-%d-%d", toUpdate.Attachment.TenantID, p.BkBizID, toUpdate.Attachment.CcProcessID)
		idMap[toUpdatekey] = toUpdate.ID
	}

	// 删除
	if toDelete > 0 {
		if err := c.dao.Process().UpdateSyncStatusWithTx(kt, tx, string(table.Deleted), []uint32{toDelete}); err != nil {
			logs.Errorf("[ProcessSync] biz=%d: mark deleted failed: processID=%d, err=%v",
				old.Attachment.BizID, toDelete, err)
			return fmt.Errorf("mark deleted failed: %w", err)
		}
	}

	// 回填 ProcessID 给 Instance
	for _, inst := range procInst {
		key := fmt.Sprintf("%s-%d-%d", inst.Attachment.TenantID, inst.Attachment.BizID, inst.Attachment.CcProcessID)
		if pid, ok := idMap[key]; ok && pid != 0 {
			inst.Attachment.ProcessID = pid
		}
	}

	if len(procInst) == 0 {
		logs.Infof("[ProcessSync] biz=%d: no process instances to insert", old.Attachment.BizID)
		return nil
	}

	if err := c.dao.ProcessInstance().BatchCreateWithTx(kt, tx, procInst); err != nil {
		logs.Errorf("biz %d: insert process instances failed, count=%d, err=%v, data=%+v",
			old.Attachment.BizID, len(procInst), err, procInst)
		return fmt.Errorf("insert process instances failed: %w", err)
	}

	logs.Infof("[ProcessSync] biz=%d: successfully handled process update")

	return nil
}
