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
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// TaskBatchListFilter task batch list filter
type TaskBatchListFilter struct {
	TaskObject table.TaskObject      // 任务对象
	TaskAction table.TaskAction      // 任务动作
	Status     table.TaskBatchStatus // 执行状态
	Executor   string                // 执行帐户（创建者）
}

// TaskBatch xxx
type TaskBatch interface {
	Create(kit *kit.Kit, taskBatch *table.TaskBatch) (uint32, error)
	GetByID(kit *kit.Kit, batchID uint32) (*table.TaskBatch, error)
	List(kit *kit.Kit, bizID uint32, filter *TaskBatchListFilter, opt *types.BasePage) ([]*table.TaskBatch, int64, error)
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
	id, err := dao.idGen.One(kit, table.Name(taskBatch.TableName()))
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

// List 查询任务历史列表
func (dao *taskBatchDao) List(kit *kit.Kit, bizID uint32, filter *TaskBatchListFilter,
	opt *types.BasePage) ([]*table.TaskBatch, int64, error) {
	m := dao.genQ.TaskBatch
	q := dao.genQ.TaskBatch.WithContext(kit.Ctx)

	// 构建查询条件
	q = q.Where(m.BizID.Eq(bizID))

	if filter != nil {
		if filter.TaskObject != "" {
			q = q.Where(m.TaskObject.Eq(string(filter.TaskObject)))
		}
		if filter.TaskAction != "" {
			q = q.Where(m.TaskAction.Eq(string(filter.TaskAction)))
		}
		if filter.Status != "" {
			q = q.Where(m.Status.Eq(string(filter.Status)))
		}
		if filter.Executor != "" {
			q = q.Where(m.Creator.Eq(filter.Executor))
		}
	}

	// 按创建时间倒序排列
	q = q.Order(m.ID.Desc())

	// 分页查询
	result, count, err := q.FindByPage(opt.Offset(), opt.LimitInt())
	if err != nil {
		return nil, 0, err
	}

	return result, count, nil
}
