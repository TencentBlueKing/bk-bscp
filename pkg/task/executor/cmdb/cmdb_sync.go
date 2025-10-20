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
	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"
)

const (
	// SyncCMDB xxx
	SyncCMDB istep.StepName = "SyncCMDB"
)

// NewSyncBizExecutor xxx
func NewSyncBizExecutor() *SyncBizExecutor {
	return &SyncBizExecutor{}
}

// HelloExecutor hello step executor
type SyncBizExecutor struct {
}

// SyncCMDB implements istep.Step.
func (s *SyncBizExecutor) SyncCMDB(c *istep.Context) (err error) {

	// bizID, _ := c.GetParam("bizID")

	// // 同步业务逻辑
	// bizList, err := s.cmdb.SearchBusinessByAccount(c.Context(), bkcmdb.SearchSetReq{
	// 	BkSupplierAccount: "0",
	// 	Fields:            []string{"bk_biz_id", "bk_biz_name"},
	// })
	// if err != nil {
	// 	return fmt.Errorf("get business data failed: %v", err)
	// }

	// var business bkcmdb.Business
	// if err := bizList.Decode(&business); err != nil {
	// 	return fmt.Errorf("parse business data: %v", err)
	// }

	// id, err := strconv.Atoi(bizID)
	// if err != nil {
	// 	return err
	// }

	// cmdb := bkcmdb.BizCMDB{
	// 	Svc:   s.cmdb,
	// 	BizID: id,
	// }

	// data, err := cmdb.SyncSingleBiz(c.Context())
	// if err != nil {
	// 	return err
	// }

	// bizs := make(bkcmdb.Bizs)
	// bizs[id] = data

	// processBatch, processInstanceBatch := bkcmdb.BuildProcessAndInstance(bizs)

	// tx := s.dao.GenQuery().Begin()

	// kt := kit.New()
	// kt.Ctx = c.Context()

	// if err := s.syncProcessAndInstanceData(kt, tx, processBatch, processInstanceBatch); err != nil {
	// 	log.Fatalf("sync process data failed: %v", err)
	// 	return err
	// }

	// // Use defer to ensure transaction is properly handled
	// committed := false
	// defer func() {
	// 	if !committed {
	// 		rErr := tx.Rollback()
	// 		if rErr != nil {
	// 			logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kt.Rid)
	// 			err = fmt.Errorf("rollback failed: %v, original error: %w", rErr, err)
	// 		}
	// 	}
	// }()

	// if e := tx.Commit(); e != nil {
	// 	logs.Errorf("commit transaction failed, err: %v, rid: %s", e, kt.Rid)
	// 	return e
	// }
	// committed = true

	return nil
}

// Register register step
func Register(s *SyncBizExecutor) {
	istep.Register(SyncCMDB, istep.StepExecutorFunc(s.SyncCMDB))
}

// func diffProcesses(dbProcesses []*table.Process, newProcesses []*table.Process) (toAdd, toUpdate []*table.Process,
// 	toDelete []uint32) {

// 	dbMap := make(map[uint32]*table.Process)
// 	for _, p := range dbProcesses {
// 		dbMap[p.Attachment.CcProcessID] = p
// 	}

// 	newMap := make(map[uint32]*table.Process)
// 	for _, p := range newProcesses {
// 		newMap[p.Attachment.CcProcessID] = p
// 	}

// 	for _, newP := range newProcesses {
// 		dbP, exists := dbMap[newP.Attachment.CcProcessID]
// 		if !exists {
// 			// DB 没有，新数据 ⇒ 新增
// 			toAdd = append(toAdd, newP)
// 			continue
// 		}

// 		// 如果 alias 变更 ⇒ 标记旧为 deleted，新增新记录
// 		if dbP.Spec.Alias != newP.Spec.Alias {
// 			dbP.Spec.CcSyncStatus = "deleted"
// 			toDelete = append(toDelete, dbP.ID)
// 			toAdd = append(toAdd, newP)
// 			continue
// 		}

// 		// 其他字段变动 ⇒ 更新
// 		if !reflect.DeepEqual(dbP.Spec, newP.Spec) {
// 			newP.ID = dbP.ID // 保留原 id 更新
// 			toUpdate = append(toUpdate, newP)
// 		}
// 	}

// 	// DB 有但新数据没有 ⇒ 删除
// 	for _, dbP := range dbProcesses {
// 		if _, exists := newMap[dbP.Attachment.CcProcessID]; !exists {
// 			dbP.Spec.CcSyncStatus = "deleted"
// 			toDelete = append(toDelete, dbP.ID)
// 		}
// 	}

// 	return
// }

// func (s *SyncBizExecutor) syncProcessAndInstanceData(kit *kit.Kit, tx *gen.QueryTx, processBatch []*table.Process,
// 	processInstanceBatch []*table.ProcessInstance) error {
// 	// 1. 按租户 + 业务分组 Process
// 	grouped := make(map[string][]*table.Process)
// 	instGrouped := make(map[string][]*table.ProcessInstance)

// 	for _, p := range processBatch {
// 		key := fmt.Sprintf("%s-%d", p.Attachment.TenantID, p.Attachment.BizID)
// 		grouped[key] = append(grouped[key], p)
// 	}

// 	for _, inst := range processInstanceBatch {
// 		key := fmt.Sprintf("%s-%d", inst.Attachment.TenantID, inst.Attachment.BizID)
// 		instGrouped[key] = append(instGrouped[key], inst)
// 	}

// 	// 2. 每组分别处理
// 	for key, batch := range grouped {
// 		tenantID := batch[0].Attachment.TenantID
// 		bizID := batch[0].Attachment.BizID

// 		// 2.1 查询数据库中已有数据

// 		dbProcesses, err := s.dao.Process().ListProcByBizIDWithTx(kit, tx, tenantID, bizID)
// 		if err != nil {
// 			return err
// 		}

// 		// 2.2 比对
// 		toAdd, toUpdate, toDelete := diffProcesses(dbProcesses, batch)

// 		// 2.3 数据库操作
// 		if len(toAdd) > 0 {
// 			if err := s.dao.Process().BatchCreateWithTx(kit, tx, toAdd); err != nil {
// 				return fmt.Errorf("insert failed for %s: %w", key, err)
// 			}
// 		}

// 		if len(toUpdate) > 0 {
// 			if err := s.dao.Process().BatchUpdateWithTx(kit, tx, toUpdate); err != nil {
// 				return fmt.Errorf("update failed for %s: %w", key, err)
// 			}
// 		}

// 		if len(toDelete) > 0 {
// 			if err := s.dao.Process().UpdateSyncStatus(kit, tx, "deleted", toDelete); err != nil {
// 				return fmt.Errorf("mark deleted failed for %s: %w", key, err)
// 			}
// 		}

// 		idMap := make(map[string]uint32)
// 		for _, p := range toAdd {
// 			key := fmt.Sprintf("%s-%d-%d", p.Attachment.TenantID, p.Attachment.BizID, p.Attachment.CcProcessID)
// 			idMap[key] = p.ID
// 		}

// 		// 回填 ProcessID
// 		for _, inst := range processInstanceBatch {
// 			key := fmt.Sprintf("%s-%d-%d", inst.Attachment.TenantID, inst.Attachment.BizID, inst.Attachment.CcProcessID)
// 			if pid, ok := idMap[key]; ok {
// 				inst.Attachment.ProcessID = pid
// 			}
// 		}

// 		// 只插入当前业务对应的 process instances
// 		if err := s.dao.ProcessInstance().BatchCreateWithTx(kit, tx, instGrouped[key]); err != nil {
// 			return err
// 		}

// 	}

// 	return nil
// }
