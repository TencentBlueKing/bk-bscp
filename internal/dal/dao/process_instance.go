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
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// ProcessInstance xxx
type ProcessInstance interface {
	// Update updates a process instance.
	Update(kit *kit.Kit, processInstance *table.ProcessInstance) error
	// GetByID gets process instances by ID.
	GetByID(kit *kit.Kit, bizID, id uint32) (*table.ProcessInstance, error)
	// BatchCreateWithTx batch create client instances with transaction.
	BatchCreateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.ProcessInstance) error
	// GetByProcessIDs gets process instances by proccessIDs.
	GetByProcessIDs(kit *kit.Kit, bizID uint32, processIDs []uint32) ([]*table.ProcessInstance, error)
}

var _ ProcessInstance = new(processInstanceDao)

type processInstanceDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
}

// GetByProcessIDs implements ProcessInstance.
func (dao *processInstanceDao) GetByProcessIDs(kit *kit.Kit, bizID uint32, processIDs []uint32) (
	[]*table.ProcessInstance, error) {
	m := dao.genQ.ProcessInstance
	q := dao.genQ.ProcessInstance.WithContext(kit.Ctx)

	result, err := q.Where(m.BizID.Eq(bizID), m.ProcessID.In(processIDs...)).Find()
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

// GetByID implements ProcessInstance.
func (dao *processInstanceDao) GetByID(kit *kit.Kit, bizID, id uint32) (*table.ProcessInstance, error) {
	m := dao.genQ.ProcessInstance

	return dao.genQ.ProcessInstance.WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID), m.ID.Eq(id)).
		Take()
}

// BatchCreateWithTx implements ProcessInstance.
func (dao *processInstanceDao) BatchCreateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.ProcessInstance) error {
	// generate an config item id and update to config item.
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
