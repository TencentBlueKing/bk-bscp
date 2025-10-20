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

package dao

import (
	"gorm.io/gorm/clause"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// Process xxx
type Process interface {
	// List released config items with options.
	List(kit *kit.Kit, bizID uint32) ([]*table.Process, int64, error)
	// BatcheUpsertWithTx 批量更新插入数据
	BatcheUpsertWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.Process) error
	// BatchCreateWithTx batch create client instances with transaction.
	BatchCreateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.Process) error
	// BatchUpdateWithTx batch update client instances with transaction.
	BatchUpdateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.Process) error
	ListProcByBizIDWithTx(kit *kit.Kit, tx *gen.QueryTx, tenantID string, bizID uint32) ([]*table.Process, error)
	UpdateSyncStatus(kit *kit.Kit, tx *gen.QueryTx, state string, ids []uint32) error
}

var _ Process = new(processDao)

type processDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
}

// UpdateSyncStatus implements Process.
func (dao *processDao) UpdateSyncStatus(kit *kit.Kit, tx *gen.QueryTx, state string, ids []uint32) error {
	m := dao.genQ.Process
	_, err := dao.genQ.Client.WithContext(kit.Ctx).
		Where(m.ID.In(ids...)).
		Update(m.CcSyncStatus, state)
	return err
}

// ListProcByBizID implements Process.
func (dao *processDao) ListProcByBizIDWithTx(kit *kit.Kit, tx *gen.QueryTx, tenantID string,
	bizID uint32) ([]*table.Process, error) {
	m := dao.genQ.Process

	return tx.Process.WithContext(kit.Ctx).Where(m.TenantID.Eq(tenantID), m.BizID.Eq(bizID)).Find()
}

// BatchUpdateWithTx implements Process.
func (dao *processDao) BatchUpdateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.Process) error {
	if len(data) == 0 {
		return nil
	}
	return tx.Process.WithContext(kit.Ctx).Save(data...)
}

// BatchCreateWithTx implements Process.
func (dao *processDao) BatchCreateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.Process) error {
	if len(data) == 0 {
		return nil
	}

	ids, err := dao.idGen.Batch(kit, table.ProcessTable, len(data))
	if err != nil {
		return err
	}
	for k, v := range data {
		v.ID = ids[k]
	}

	return tx.Process.WithContext(kit.Ctx).CreateInBatches(data, 500)
}

// BatcheUpsertWithTx implements Process.
func (dao *processDao) BatcheUpsertWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.Process) error {
	q := tx.Process.WithContext(kit.Ctx)

	return q.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "biz_id"}, {Name: "cc_process_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"cc_sync_status", "cc_sync_updated_at"}),
	}).CreateInBatches(data, 500)
}

// List implements Process.
func (dao *processDao) List(kit *kit.Kit, bizID uint32) ([]*table.Process, int64, error) {
	m := dao.genQ.Process
	q := dao.genQ.Process.WithContext(kit.Ctx)

	result, err := q.Where(m.BizID.Eq(bizID)).Find()
	if err != nil {
		return nil, 0, err
	}

	return result, int64(len(result)), err
}
