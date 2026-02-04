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
	"github.com/TencentBlueKing/bk-bscp/internal/dal/utils"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbproc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
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
	ListProcByBizIDWithTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32) ([]*table.Process, error)
	UpdateSyncStatusWithTx(kit *kit.Kit, tx *gen.QueryTx, state string, ids []uint32) error
	ListBizFilterOptions(kit *kit.Kit, bizID uint32, fields ...field.Expr) ([]*table.Process, error)
	// UpdateSelectedFields 更新指定字段
	UpdateSelectedFields(kit *kit.Kit, bizID uint32, data map[string]any, conds ...rawgen.Condition) error
	// GetProcByBizScvProc 按业务、服务实例、进程 ID 查询进程
	GetProcByBizScvProc(kit *kit.Kit, bizID, svcInstID, processID uint32) (*table.Process, error)
	// ListActiveProcesses 获取所有未删除的数据
	ListActiveProcesses(kit *kit.Kit, bizID uint32) ([]*table.Process, error)
	// GetByModuleID 查询模块下所有进程 ID.
	GetByModuleIDWithTx(kit *kit.Kit, tx *gen.QueryTx, bizID, moduleID uint32) ([]uint32, error)
	// GetByHostIDWithTx 查询主机下所有进程 ID.
	GetByHostIDWithTx(kit *kit.Kit, tx *gen.QueryTx, bizID, hostID uint32) ([]uint32, error)
	// GetBySetIDWithTx queries all process IDs under a set.
	GetBySetIDWithTx(kit *kit.Kit, tx *gen.QueryTx, bizID, setID uint32) ([]uint32, error)
	ProcessCountByServiceInstance(kit *kit.Kit, bizID, serviceInstanceID uint32) (int64, error)
	ProcessCountByServiceTemplate(kit *kit.Kit, bizID, serviceTemplateID uint32) (int64, error)
	// GetByOperateRange 根据操作范围查询进程
	GetByOperateRange(kit *kit.Kit, bizID uint32, operateRange *pbproc.OperateRange) ([]*table.Process, error)
	// GetByCcProcessIDAndAliasTx 查找同 CcProcessID + 同新别名
	GetByCcProcessIDAndAliasTx(kit *kit.Kit, tx *gen.QueryTx, bizID, ccProcessID uint32, alias string) (*table.Process, error)
	// ListByModuleIDAndAliasWithTx 按模块ID和别名查询进程ID列表
	ListByModuleIDAndAliasWithTx(kit *kit.Kit, tx *gen.QueryTx, bizID, moduleID uint32, alias string) ([]uint32, error)
}

var _ Process = new(processDao)

type processDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
}

// GetByCcProcessIDAndAliasTx 查找同 CcProcessID + 同新别名
func (dao *processDao) GetByCcProcessIDAndAliasTx(kit *kit.Kit, tx *gen.QueryTx, bizID,
	ccProcessID uint32, alias string) (*table.Process, error) {
	m := dao.genQ.Process

	return tx.Process.WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID), m.CcProcessID.Eq(ccProcessID), m.Alias_.Eq(alias)).
		Take()
}

// ProcessCountByServiceTemplate implements Process.
func (dao *processDao) ProcessCountByServiceTemplate(kit *kit.Kit, bizID uint32, serviceTemplateID uint32) (int64, error) {
	m := dao.genQ.Process
	q := dao.genQ.Process.WithContext(kit.Ctx)

	return q.Where(m.BizID.Eq(bizID),
		m.ServiceTemplateID.Eq(serviceTemplateID),
		m.CcSyncStatus.Neq(table.Deleted.String())).Count()
}

// ProcessCountByServiceInstance implements Process.
func (dao *processDao) ProcessCountByServiceInstance(kit *kit.Kit, bizID, serviceInstanceID uint32) (int64, error) {
	m := dao.genQ.Process
	q := dao.genQ.Process.WithContext(kit.Ctx)

	return q.Where(m.BizID.Eq(bizID),
		m.ServiceInstanceID.Eq(serviceInstanceID),
		m.CcSyncStatus.Neq(table.Deleted.String())).Count()
}

// GetBySetIDWithTx queries all process IDs under a set.
func (dao *processDao) GetBySetIDWithTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32, setID uint32) ([]uint32, error) {
	m := dao.genQ.Process
	q := tx.Process.WithContext(kit.Ctx)

	var result []uint32
	if err := q.Select(m.ID).
		Where(m.BizID.Eq(bizID), m.SetID.Eq(setID)).
		Pluck(m.ID, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetByHostIDWithTx implements Process.
func (dao *processDao) GetByHostIDWithTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32, hostID uint32) ([]uint32, error) {
	m := dao.genQ.Process
	q := tx.Process.WithContext(kit.Ctx)

	var result []uint32
	if err := q.Select(m.ID).
		Where(m.BizID.Eq(bizID), m.HostID.Eq(hostID)).
		Pluck(m.ID, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetByModuleIDWithTx implements Process.
func (dao *processDao) GetByModuleIDWithTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32, moduleID uint32) ([]uint32, error) {
	m := dao.genQ.Process
	q := tx.Process.WithContext(kit.Ctx)

	var result []uint32
	if err := q.Select(m.ID).
		Where(m.BizID.Eq(bizID), m.ModuleID.Eq(moduleID)).
		Pluck(m.ID, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetProcByBizScvProc implements Process.
func (dao *processDao) GetProcByBizScvProc(kit *kit.Kit, bizID uint32, svcInstID uint32,
	processID uint32) (*table.Process, error) {
	m := dao.genQ.Process

	return dao.genQ.Process.WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID), m.ServiceInstanceID.Eq(svcInstID), m.CcProcessID.Eq(processID),
			m.CcSyncStatus.Neq(table.Deleted.String())).
		Take()
}

// UpdateSelectedFields implements Process.
func (dao *processDao) UpdateSelectedFields(kit *kit.Kit, bizID uint32, data map[string]any, conds ...rawgen.Condition) error {
	m := dao.genQ.Process

	_, err := dao.genQ.WithContext(kit.Ctx).Process.
		Where(m.BizID.Eq(bizID)).
		Where(conds...).
		Updates(data)
	if err != nil {
		return err
	}

	return nil
}

// ListActiveProcesses implements Process.
func (dao *processDao) ListActiveProcesses(kit *kit.Kit, bizID uint32) ([]*table.Process, error) {
	m := dao.genQ.Process

	return dao.genQ.Process.WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID), m.CcSyncStatus.Neq(table.Deleted.String())).
		Find()
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
	sql := `processes.cc_sync_status != ?
			OR EXISTS (
			SELECT 1
			FROM process_instances AS pl
			WHERE pl.process_id = processes.id
			AND (pl.status = ? OR pl.managed_status = ?)
			)`

	q := dao.genQ.Process.WithContext(kit.Ctx).
		Where(dao.genQ.Process.BizID.Eq(bizID), utils.RawCond(sql, "deleted", "running", "managed"))

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
	q := tx.Process.WithContext(kit.Ctx).Where(m.ID.In(ids...))

	update := map[string]any{
		m.CcSyncStatus.ColumnName().String(): state,
	}

	_, err := q.Updates(update)
	return err
}

// ListProcByBizID implements Process.
func (dao *processDao) ListProcByBizIDWithTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32) ([]*table.Process, error) {
	m := dao.genQ.Process

	return tx.Process.WithContext(kit.Ctx).Where(m.BizID.Eq(bizID)).Find()
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
	q := tx.Process.WithContext(kit.Ctx).Where(dao.genQ.Process.CcSyncStatus.Neq(table.Deleted.String()))

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

	// processes 状态不能是删除状态
	// process_instances 的进程状态和托管状态只要有一条数据是运行中或者托管中的都需要显示
	sql := `processes.cc_sync_status != ?
			OR EXISTS (
			SELECT 1
			FROM process_instances AS pl
			WHERE pl.process_id = processes.id
			AND (pl.status = ? OR pl.managed_status = ?)
			)`
	conds = append(conds, q.Where(utils.RawCond(sql, "deleted", "running", "managed")))

	d := q.Where(m.BizID.Eq(bizID)).Where(conds...).Order(m.ID.Desc())

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

	if len(search.GetSets()) != 0 {
		conds = append(conds, m.SetName.In(search.GetSets()...))
	}

	if len(search.GetModules()) != 0 {
		conds = append(conds, m.ModuleName.In(search.GetModules()...))
	}

	if len(search.GetServiceInstances()) != 0 {
		conds = append(conds, m.ServiceName.In(search.GetServiceInstances()...))
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

	if len(search.GetProcessTemplateIds()) != 0 {
		conds = append(conds, m.ProcessTemplateID.In(search.GetProcessTemplateIds()...))
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

// GetByOperateRange 根据操作范围查询进程
func (dao *processDao) GetByOperateRange(kit *kit.Kit, bizID uint32, operateRange *pbproc.OperateRange) ([]*table.Process, error) {
	m := dao.genQ.Process
	q := dao.genQ.Process.WithContext(kit.Ctx)

	// 构建查询条件
	var conds []rawgen.Condition
	conds = append(conds, m.BizID.Eq(bizID))

	if operateRange.GetEnvironment() != "" {
		conds = append(conds, m.Environment.Eq(operateRange.GetEnvironment()))
	}

	if operateRange.GetSetName() != "" {
		conds = append(conds, m.SetName.Eq(operateRange.GetSetName()))
	}

	if operateRange.GetModuleName() != "" {
		conds = append(conds, m.ModuleName.Eq(operateRange.GetModuleName()))
	}

	if operateRange.GetServiceName() != "" {
		conds = append(conds, m.ServiceName.Eq(operateRange.GetServiceName()))
	}

	if operateRange.GetProcessAlias() != "" {
		conds = append(conds, m.Alias_.Eq(operateRange.GetProcessAlias()))
	}

	if operateRange.GetCcProcessId() != 0 {
		conds = append(conds, m.CcProcessID.Eq(operateRange.GetCcProcessId()))
	}

	// 只查询未删除的进程
	conds = append(conds, m.CcSyncStatus.Neq(table.Deleted.String()))

	return q.Where(conds...).Find()
}

// ListByModuleIDAndAliasWithTx 按模块ID和别名查询进程ID列表
func (dao *processDao) ListByModuleIDAndAliasWithTx(kit *kit.Kit, tx *gen.QueryTx, bizID,
	moduleID uint32, alias string) ([]uint32, error) {
	m := dao.genQ.Process
	q := tx.Process.WithContext(kit.Ctx)

	var result []uint32
	if err := q.Select(m.ID).
		Where(m.BizID.Eq(bizID), m.ModuleID.Eq(moduleID), m.Alias_.Eq(alias),
			m.CcSyncStatus.Neq(table.Deleted.String())).
		Pluck(m.ID, &result); err != nil {
		return nil, err
	}

	return result, nil
}
