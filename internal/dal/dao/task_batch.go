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

// TaskBatch xxx
type TaskBatch interface {
	Create(kit *kit.Kit, taskBatch *table.TaskBatch) (uint32, error)
	GetByID(kit *kit.Kit, batchID uint32) (*table.TaskBatch, error)
}

var _ TaskBatch = new(taskBatchDao)

type taskBatchDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
}

// Create 创建任务批次
func (dao *taskBatchDao) Create(kit *kit.Kit, taskBatch *table.TaskBatch) (uint32, error) {
	if taskBatch == nil {
		return 0, nil
	}
	q := dao.genQ.TaskBatch.WithContext(kit.Ctx)

	if err := taskBatch.ValidateCreate(); err != nil {
		return 0, err
	}

	// 生成ID
	id, err := dao.idGen.One(kit, table.Name(table.Table))
	if err != nil {
		return 0, err
	}
	taskBatch.ID = id

	return id, q.Create(taskBatch)
}

// GetByID 根据ID获取批次信息
func (dao *taskBatchDao) GetByID(kit *kit.Kit, batchID uint32) (*table.TaskBatch, error) {
	m := dao.genQ.TaskBatch
	q := dao.genQ.TaskBatch.WithContext(kit.Ctx)

	taskBatch, err := q.Where(m.ID.Eq(batchID)).Take()
	if err != nil {
		return nil, err
	}

	return taskBatch, nil
}
