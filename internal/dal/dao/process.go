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
	rawgen "gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm/clause"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	process "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// Process xxx
type Process interface {
	// GetByIDs get client by ids.
	GetByIDs(kit *kit.Kit, bizID uint32, id []uint32) ([]*table.Process, error)
	// GetByID get client by id.
	GetByID(kit *kit.Kit, bizID uint32, id uint32) (*table.Process, error)
	// List released config items with options.
	List(kit *kit.Kit, bizID uint32, search *process.ProcessSearchCondition,
		opt *types.BasePage) ([]*table.Process, int64, error)
	// BatcheUpsertWithTx 批量更新插入数据
	BatcheUpsertWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.Process) error
	// BatchCreateWithTx batch create client instances with transaction.
	BatchCreateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.Process) error
	// BatchUpdateWithTx batch update client instances with transaction.
	BatchUpdateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.Process) error
	ListProcByBizIDWithTx(kit *kit.Kit, tx *gen.QueryTx, tenantID string, bizID uint32) ([]*table.Process, error)
	UpdateSyncStatusWithTx(kit *kit.Kit, tx *gen.QueryTx, state string, ids []uint32) error
	ListBizFilterOptions(kit *kit.Kit, bizID uint32, fields ...field.Expr) ([]*table.Process, error)
}

var _ Process = new(processDao)

type processDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
}

// GetByID implements Process.
func (dao *processDao) GetByID(kit *kit.Kit, bizID uint32, id uint32) (*table.Process, error) {
	m := dao.genQ.Process

	return dao.genQ.Process.WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID), m.ID.Eq(id)).
		Take()
}

// ListBizFilterOptions implements Process.
// fields = append(fields, field.NewString("", "id"))
func (dao *processDao) ListBizFilterOptions(kit *kit.Kit, bizID uint32, fields ...field.Expr) (
	[]*table.Process, error) {
	q := dao.genQ.Process.WithContext(kit.Ctx)

	return q.Distinct(fields...).Select(fields...).Find()
}

func (dao *processDao) GetByIDs(kit *kit.Kit, bizID uint32, id []uint32) ([]*table.Process, error) {
	m := dao.genQ.Process
	q := dao.genQ.Process.WithContext(kit.Ctx)

	result, err := q.Where(m.ID.In(id...), m.BizID.Eq(bizID)).Find()
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, nil
	}

	return result, nil
}

// UpdateSyncStatus implements Process.
func (dao *processDao) UpdateSyncStatusWithTx(kit *kit.Kit, tx *gen.QueryTx, state string, ids []uint32) error {
	m := dao.genQ.Process
	_, err := tx.Process.WithContext(kit.Ctx).Where(m.ID.In(ids...)).Update(m.CcSyncStatus, state)
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

	ids, err := dao.idGen.Batch(kit, table.ProcessesTable, len(data))
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
func (dao *processDao) List(kit *kit.Kit, bizID uint32, search *process.ProcessSearchCondition,
	opt *types.BasePage) ([]*table.Process, int64, error) {
	m := dao.genQ.Process
	q := dao.genQ.Process.WithContext(kit.Ctx)

	var err error
	var conds []rawgen.Condition
	if search.String() != "" {
		conds, err = dao.handleSearch(kit, search)
		if err != nil {
			return nil, 0, err
		}
	}

	d := q.Where(m.BizID.Eq(bizID)).Where(conds...)

	if opt.All {
		result, err := d.Find()
		if err != nil {
			return nil, 0, err
		}
		return result, int64(len(result)), err
	}
	return d.FindByPage(opt.Offset(), opt.LimitInt())
}

func (dao *processDao) handleSearch(kit *kit.Kit, search *process.ProcessSearchCondition) ([]rawgen.Condition, error) {
	var conds []rawgen.Condition
	m := dao.genQ.Process
	q := dao.genQ.Process.WithContext(kit.Ctx)

	if len(search.GetEnvironment()) != 0 {
		conds = append(conds, m.Environment.Eq(search.GetEnvironment()))
	}

	if len(search.GetSetNames()) != 0 {
		conds = append(conds, m.SetName.In(search.GetSetNames()...))
	}

	if len(search.GetModuleNames()) != 0 {
		conds = append(conds, m.ModuleName.In(search.GetModuleNames()...))
	}

	if len(search.GetServiceInstanceNames()) != 0 {
		conds = append(conds, m.ServiceName.In(search.GetServiceInstanceNames()...))
	}

	if len(search.GetCcProcessIds()) != 0 {
		conds = append(conds, m.CcProcessID.In(search.GetCcProcessIds()...))
	}

	if len(search.GetProcessAliases()) != 0 {
		conds = append(conds, m.Alias_.In(search.GetProcessAliases()...))
	}

	if len(search.GetInnerIps()) != 0 {
		conds = append(conds, m.InnerIP.In(search.GetInnerIps()...))
	}

	if len(search.GetCcSyncStatuses()) != 0 {
		conds = append(conds, m.CcSyncStatus.In(search.GetCcSyncStatuses()...))
	}

	if len(search.GetManagedStatuses()) != 0 {
		managedStatus, err := dao.handleManagedStatus(kit, q, search.GetManagedStatuses())
		if err != nil {
			return nil, err
		}
		conds = append(conds, managedStatus...)
	}

	if len(search.GetProcessStatuses()) != 0 {
		status, err := dao.handleProcessStatus(kit, q, search.GetProcessStatuses())
		if err != nil {
			return nil, err
		}
		conds = append(conds, status...)
	}

	return conds, nil
}

func (dao *processDao) handleProcessStatus(kit *kit.Kit, q gen.IProcessDo, status []string) (
	[]rawgen.Condition, error) {
	// 1. 先根据实例表的状态查询到processID
	// 2. 再根据查询到的processID做搜索
	m := dao.genQ.Process
	instQ := dao.genQ.ProcessInstance

	var pid []uint32
	err := instQ.WithContext(kit.Ctx).
		Distinct(instQ.ProcessID).
		Where(instQ.Status.In(status...)).
		Pluck(instQ.ProcessID, &pid)
	if err != nil {
		return nil, err
	}

	return []rawgen.Condition{q.Where(m.ID.In(pid...))}, nil
}

func (dao *processDao) handleManagedStatus(kit *kit.Kit, q gen.IProcessDo, status []string) (
	[]rawgen.Condition, error) {
	// 1. 先根据实例表的状态查询到processID
	// 2. 再根据查询到的processID做搜索
	m := dao.genQ.Process
	instQ := dao.genQ.ProcessInstance

	var pid []uint32
	err := instQ.WithContext(kit.Ctx).
		Distinct(instQ.ProcessID).
		Where(instQ.ManagedStatus.In(status...)).
		Pluck(instQ.ProcessID, &pid)
	if err != nil {
		return nil, err
	}

	return []rawgen.Condition{q.Where(m.ID.In(pid...))}, nil
}
