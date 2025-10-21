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

package service

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

type Set struct {
	ID     int
	Name   string
	Module []Module
}

type Module struct {
	ID      int
	Name    string
	Host    []Host
	SvcInst []SvcInst
}
type Host struct {
	ID   int
	Name string
	IP   string
}
type SvcInst struct {
	ID       int
	Name     string
	ProcInst []ProcInst
}
type ProcInst struct {
	ID      int
	HostID  int
	Name    string
	ProcNum int
	table.ProcessInfo
}

type Biz map[int][]Set

// SyncCMDB implements pbds.DataServer.
func (s *Service) SyncCMDB(ctx context.Context, req *pbds.SyncCMDBReq) (*pbds.SyncCMDBResp, error) {
	// grpcKit := kit.FromGrpcContext(ctx)

	return &pbds.SyncCMDBResp{
		TaskId: 0,
	}, nil

}

func (s *Service) SynchronizeCmdbData(ctx context.Context, bizIDs []int) error {
	grpcKit := kit.FromGrpcContext(ctx)
	// 不指定业务同步，表示同步所有业务
	if len(bizIDs) == 0 {
		bizList, err := s.cmdb.SearchBusinessByAccount(ctx, bkcmdb.SearchSetReq{
			BkSupplierAccount: "0",
			Fields:            []string{"bk_biz_id", "bk_biz_name"},
		})
		if err != nil {
			return fmt.Errorf("get business data failed: %v", err)
		}

		var business bkcmdb.Business
		if err := bizList.Decode(&business); err != nil {
			return fmt.Errorf("parse business data: %v", err)
		}

		for _, item := range business.Info {
			bizIDs = append(bizIDs, item.BkBizID)
		}
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(5)

	// 业务数据集合
	var (
		mu   sync.Mutex
		bizs = make(bkcmdb.Bizs)
	)

	for _, id := range bizIDs {
		bizID := id
		g.Go(func() error {

			cmdb := bkcmdb.BizCMDB{
				Svc:   s.cmdb,
				BizID: id,
			}
			data, err := cmdb.SyncSingleBiz(gctx)
			if err != nil {
				return fmt.Errorf("sync biz %d failed: %v", bizID, err)
			}
			mu.Lock()
			bizs[bizID] = data
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	processBatch, processInstanceBatch := bkcmdb.BuildProcessAndInstance(bizs)

	tx := s.dao.GenQuery().Begin()

	if err := s.syncProcessAndInstanceData(grpcKit, tx, processBatch, processInstanceBatch); err != nil {
		log.Fatalf("sync process data failed: %v", err)
		return err
	}

	// Use defer to ensure transaction is properly handled
	committed := false
	defer func() {
		if !committed {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, grpcKit.Rid)
			}
		}
	}()

	if e := tx.Commit(); e != nil {
		logs.Errorf("commit transaction failed, err: %v, rid: %s", e, grpcKit.Rid)
		return e
	}
	committed = true

	return nil
}

func diffProcesses(dbProcesses []*table.Process, newProcesses []*table.Process) (toAdd, toUpdate []*table.Process,
	toDelete []uint32) {

	dbMap := make(map[uint32]*table.Process)
	for _, p := range dbProcesses {
		dbMap[p.Attachment.CcProcessID] = p
	}

	newMap := make(map[uint32]*table.Process)
	for _, p := range newProcesses {
		newMap[p.Attachment.CcProcessID] = p
	}

	for _, newP := range newProcesses {
		dbP, exists := dbMap[newP.Attachment.CcProcessID]
		if !exists {
			// DB 没有，新数据 ⇒ 新增
			toAdd = append(toAdd, newP)
			continue
		}

		// 如果 alias 变更 ⇒ 标记旧为 deleted，新增新记录
		if dbP.Spec.Alias != newP.Spec.Alias {
			dbP.Spec.CcSyncStatus = "deleted"
			toDelete = append(toDelete, dbP.ID)
			toAdd = append(toAdd, newP)
			continue
		}

		// 其他字段变动 ⇒ 更新
		if !reflect.DeepEqual(dbP.Spec, newP.Spec) {
			newP.ID = dbP.ID // 保留原 id 更新
			toUpdate = append(toUpdate, newP)
		}
	}

	// DB 有但新数据没有 ⇒ 删除
	for _, dbP := range dbProcesses {
		if _, exists := newMap[dbP.Attachment.CcProcessID]; !exists {
			dbP.Spec.CcSyncStatus = "deleted"
			toDelete = append(toDelete, dbP.ID)
		}
	}

	return toAdd, toUpdate, toDelete
}

func (s *Service) syncProcessAndInstanceData(kit *kit.Kit, tx *gen.QueryTx, processBatch []*table.Process,
	processInstanceBatch []*table.ProcessInstance) error {
	// 1. 按租户 + 业务分组 Process
	grouped := make(map[string][]*table.Process)
	instGrouped := make(map[string][]*table.ProcessInstance)

	for _, p := range processBatch {
		key := fmt.Sprintf("%s-%d", p.Attachment.TenantID, p.Attachment.BizID)
		grouped[key] = append(grouped[key], p)
	}

	for _, inst := range processInstanceBatch {
		key := fmt.Sprintf("%s-%d", inst.Attachment.TenantID, inst.Attachment.BizID)
		instGrouped[key] = append(instGrouped[key], inst)
	}

	// 2. 每组分别处理
	for key, batch := range grouped {
		tenantID := batch[0].Attachment.TenantID
		bizID := batch[0].Attachment.BizID

		// 2.1 查询数据库中已有数据
		dbProcesses, err := s.dao.Process().ListProcByBizIDWithTx(kit, tx, tenantID, bizID)
		if err != nil {
			return err
		}

		// 2.2 比对
		toAdd, toUpdate, toDelete := diffProcesses(dbProcesses, batch)

		// 2.3 数据库操作
		if len(toAdd) > 0 {
			if err := s.dao.Process().BatchCreateWithTx(kit, tx, toAdd); err != nil {
				return fmt.Errorf("insert failed for %s: %w", key, err)
			}
		}

		if len(toUpdate) > 0 {
			if err := s.dao.Process().BatchUpdateWithTx(kit, tx, toUpdate); err != nil {
				return fmt.Errorf("update failed for %s: %w", key, err)
			}
		}

		if len(toDelete) > 0 {
			if err := s.dao.Process().UpdateSyncStatus(kit, tx, "deleted", toDelete); err != nil {
				return fmt.Errorf("mark deleted failed for %s: %w", key, err)
			}
		}

		idMap := make(map[string]uint32)
		for _, p := range toAdd {
			idMap[fmt.Sprintf("%s-%d-%d", p.Attachment.TenantID, p.Attachment.BizID, p.Attachment.CcProcessID)] = p.ID
		}

		// 回填 ProcessID
		for _, inst := range processInstanceBatch {
			if pid, ok := idMap[fmt.Sprintf("%s-%d-%d", inst.Attachment.TenantID, inst.Attachment.BizID,
				inst.Attachment.CcProcessID)]; ok {
				inst.Attachment.ProcessID = pid
			}
		}

		// 只插入当前业务对应的 process instances
		if err := s.dao.ProcessInstance().BatchCreateWithTx(kit, tx, instGrouped[key]); err != nil {
			return err
		}

	}

	return nil
}
