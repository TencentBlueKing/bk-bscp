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

	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// ProcessInstance xxx
type ProcessInstance interface {
	// Update updates a process instance.
	Update(kit *kit.Kit, processInstance *table.ProcessInstance) error
	// UpdateSelectedFields update selected fields
	UpdateSelectedFields(kit *kit.Kit, bizID uint32, data map[string]any, conds ...rawgen.Condition) error
	// BatchUpdate batch updates process instances.
	BatchUpdate(kit *kit.Kit, instances []*table.ProcessInstance) error
	// GetByID gets process instances by ID.
	GetByID(kit *kit.Kit, bizID, id uint32) (*table.ProcessInstance, error)
	// BatchCreateWithTx batch create client instances with transaction.
	BatchCreateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.ProcessInstance) error
	// GetByProcessIDs gets process instances by proccessIDs.
	GetByProcessIDs(kit *kit.Kit, bizID uint32, processIDs []uint32) ([]*table.ProcessInstance, error)
	// GetCountTx 查询指定进程的实例数量.
	GetCountTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32, processID uint32) (int64, error)
	// Delete ..
	Delete(kit *kit.Kit, bizID, id uint32) error
	// GetMaxModuleInstSeqTx 查询模块下所有进程的最大 ModuleInstSeq
	GetMaxModuleInstSeqTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32, processIDs []uint32) (int, error)
	// GetMaxHostInstSeqTx 查询主机下所有进程的最大 HostInstSeq
	GetMaxHostInstSeqTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32, processIDs []uint32) (int, error)
	// DeleteStoppedUnmanagedWithTx deletes process instances that are stopped or unmanaged.
	DeleteStoppedUnmanagedWithTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32, processIDs []uint32) error
	// BatchUpdateWithTx batch updates process instances.
	BatchUpdateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.ProcessInstance) error
	ListByProcessIDTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32, processID uint32) ([]*table.ProcessInstance, error)
	// ListByProcessIDOrderBySeqDescTx 查询某业务下某进程，按 module_inst_seq 倒序，取前 N 条实例
	ListByProcessIDOrderBySeqDescTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32, processID uint32, limit int) ([]*table.ProcessInstance, error)
	// BatchDeleteByIDsWithTx batch create client instances with transaction.
	BatchDeleteByIDsWithTx(kit *kit.Kit, tx *gen.QueryTx, ids []uint32) error
}

var _ ProcessInstance = new(processInstanceDao)

type processInstanceDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
}

// BatchDeleteByIDsWithTx implements [ProcessInstance].
func (dao *processInstanceDao) BatchDeleteByIDsWithTx(kit *kit.Kit, tx *gen.QueryTx, ids []uint32) error {
	m := dao.genQ.ProcessInstance
	_, err := tx.ProcessInstance.WithContext(kit.Ctx).
		Where(m.ID.In(ids...)).Delete()
	if err != nil {
		return err
	}

	return nil
}

// ListByProcessIDOrderBySeqDescTx implements [ProcessInstance].
func (dao *processInstanceDao) ListByProcessIDOrderBySeqDescTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32, processID uint32, limit int) (
	[]*table.ProcessInstance, error) {
	m := dao.genQ.ProcessInstance
	if limit <= 0 {
		return nil, nil
	}

	result, err := tx.ProcessInstance.WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID), m.ProcessID.Eq(processID)).
		Order(m.ModuleInstSeq.Desc()).
		Limit(limit).
		Find()
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ListByProcessIDTx implements [ProcessInstance].
func (dao *processInstanceDao) ListByProcessIDTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32,
	processID uint32) ([]*table.ProcessInstance, error) {
	m := dao.genQ.ProcessInstance

	result, err := tx.ProcessInstance.
		WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID), m.ProcessID.Eq(processID)).
		Find()
	if err != nil {
		return nil, err
	}

	return result, err
}

// BatchUpdateWithTx implements ProcessInstance.
func (dao *processInstanceDao) BatchUpdateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.ProcessInstance) error {
	if len(data) == 0 {
		return nil
	}

	q := tx.ProcessInstance.WithContext(kit.Ctx)

	// 按批次更新，每次最多 500 条
	batchSize := 500
	for i := 0; i < len(data); i += batchSize {
		end := min(i+batchSize, len(data))
		batch := data[i:end]

		if err := q.Save(batch...); err != nil {
			return err
		}
	}

	return nil
}

// DeleteStoppedUnmanagedWithTx deletes process instances that are stopped or unmanaged.
func (dao *processInstanceDao) DeleteStoppedUnmanagedWithTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32, processIDs []uint32) error {
	m := dao.genQ.ProcessInstance

	_, err := tx.ProcessInstance.WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID), m.ProcessID.In(processIDs...),
			m.Status.In(table.ProcessStatusStopped.String(), ""), m.ManagedStatus.In(table.ProcessManagedStatusUnmanaged.String(), "")).
		Delete()
	if err != nil {
		return err
	}

	return nil
}

// GetCountTx implements ProcessInstance.
func (dao *processInstanceDao) GetCountTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32, processID uint32) (int64, error) {
	m := dao.genQ.ProcessInstance
	return tx.ProcessInstance.WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID), m.ProcessID.Eq(processID)).
		Count()
}

// GetMaxModuleInstSeqTx implements ProcessInstance.
func (dao *processInstanceDao) GetMaxModuleInstSeqTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32, processIDs []uint32) (int, error) {
	m := dao.genQ.ProcessInstance
	q := tx.ProcessInstance.WithContext(kit.Ctx)
	var result struct {
		MaxID int `gorm:"column:max_id"`
	}
	err := q.Where(m.BizID.Eq(bizID), m.ProcessID.In(processIDs...)).Select(m.ModuleInstSeq.Max().As("max_id")).Scan(&result)
	if err != nil {
		return 0, err
	}

	return result.MaxID, nil
}

// GetMaxHostInstSeqTx implements ProcessInstance.
func (dao *processInstanceDao) GetMaxHostInstSeqTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32, processIDs []uint32) (int, error) {
	m := dao.genQ.ProcessInstance
	q := tx.ProcessInstance.WithContext(kit.Ctx)
	var result struct {
		MaxID int `gorm:"column:max_id"`
	}
	err := q.Where(m.BizID.Eq(bizID), m.ProcessID.In(processIDs...)).Select(m.HostInstSeq.Max().As("max_id")).Scan(&result)
	if err != nil {
		return 0, err
	}

	return result.MaxID, nil
}

// Delete implements ProcessInstance.
func (dao *processInstanceDao) Delete(kit *kit.Kit, bizID uint32, id uint32) error {
	m := dao.genQ.ProcessInstance
	_, err := dao.genQ.ProcessInstance.WithContext(kit.Ctx).Where(m.BizID.Eq(bizID), m.ID.Eq(id)).Delete()
	if err != nil {
		return err
	}

	return nil
}

// GetByProcessIDs implements ProcessInstance.
func (dao *processInstanceDao) GetByProcessIDs(kit *kit.Kit, bizID uint32, processIDs []uint32) (
	[]*table.ProcessInstance, error) {
	m := dao.genQ.ProcessInstance
	q := dao.genQ.ProcessInstance.WithContext(kit.Ctx)

	result, err := q.Where(m.BizID.Eq(bizID), m.ProcessID.In(processIDs...)).Order(m.HostInstSeq).Find()
	if err != nil {
		return nil, err
	}

	return result, err
}

// Update implements ProcessInstance.
func (dao *processInstanceDao) Update(kit *kit.Kit, processInstance *table.ProcessInstance) error {
	m := dao.genQ.ProcessInstance
	q := dao.genQ.ProcessInstance.WithContext(kit.Ctx)

	if _, err := q.Where(m.ID.Eq(processInstance.ID)).Updates(processInstance); err != nil {
		return err
	}

	return nil
}

// UpdateSelectedFields 更新指定字段
func (dao *processInstanceDao) UpdateSelectedFields(kit *kit.Kit, bizID uint32, data map[string]any, conds ...rawgen.Condition) error {
	m := dao.genQ.ProcessInstance
	q := dao.genQ.ProcessInstance.WithContext(kit.Ctx)

	query := q.Where(m.BizID.Eq(bizID))
	if len(conds) > 0 {
		query = query.Where(conds...)
	}

	if _, err := query.Updates(data); err != nil {
		return err
	}

	return nil
}

// BatchUpdate implements ProcessInstance.
func (dao *processInstanceDao) BatchUpdate(kit *kit.Kit, instances []*table.ProcessInstance) error {
	if len(instances) == 0 {
		return nil
	}

	q := dao.genQ.ProcessInstance.WithContext(kit.Ctx)

	// 按批次更新，每次最多 500 条
	batchSize := 500
	for i := 0; i < len(instances); i += batchSize {
		end := i + batchSize
		if end > len(instances) {
			end = len(instances)
		}
		batch := instances[i:end]

		if err := q.Save(batch...); err != nil {
			return err
		}
	}

	return nil
}

// GetByID implements ProcessInstance.
func (dao *processInstanceDao) GetByID(kit *kit.Kit, bizID, id uint32) (*table.ProcessInstance, error) {
	m := dao.genQ.ProcessInstance

	return dao.genQ.ProcessInstance.WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID), m.ID.Eq(id)).
		Take()
}

// BatchCreateWithTx implements ProcessInstance.
func (dao *processInstanceDao) BatchCreateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.ProcessInstance) error {
	if len(data) == 0 {
		return nil
	}

	ids, err := dao.idGen.Batch(kit, table.ProcessInstancesTable, len(data))
	if err != nil {
		return err
	}
	for k, v := range data {
		v.ID = ids[k]
	}

	return tx.ProcessInstance.WithContext(kit.Ctx).CreateInBatches(data, 500)
}
