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
	"fmt"
	"time"

	rawgen "gorm.io/gen"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// TaskBatchListFilter task batch list filter
type TaskBatchListFilter struct {
	TaskObjects    []string   // 任务对象列表
	TaskActions    []string   // 任务动作列表
	Statuses       []string   // 执行状态列表
	Executors      []string   // 执行帐户列表（创建者）
	TimeRangeStart *time.Time // 时间范围起点
	TimeRangeEnd   *time.Time // 时间范围终点
}

// TaskBatch xxx
type TaskBatch interface {
	Create(kit *kit.Kit, taskBatch *table.TaskBatch) (uint32, error)
	GetByID(kit *kit.Kit, batchID uint32) (*table.TaskBatch, error)
	List(kit *kit.Kit, bizID uint32, filter *TaskBatchListFilter, opt *types.BasePage) ([]*table.TaskBatch, int64, error)
	UpdateStatus(kit *kit.Kit, batchID uint32, status table.TaskBatchStatus) error
	ListExecutors(kit *kit.Kit, bizID uint32) ([]string, error)
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

	// 构建过滤条件
	conds := dao.buildFilterConditions(filter)
	q = q.Where(m.BizID.Eq(bizID)).Where(conds...)

	// 排序
	if opt != nil && opt.Sort != "" {
		orderCol, ok := m.GetFieldByName(opt.Sort)
		if !ok {
			return nil, 0, fmt.Errorf("table task_batches doesn't contains column %s", opt.Sort)
		}

		if opt.Order == types.Ascending {
			q = q.Order(orderCol)
		} else {
			q = q.Order(orderCol.Desc())
		}
	} else {
		// 未传入排序参数时，默认按 ID 倒序排列
		q = q.Order(m.ID.Desc())
	}

	// 分页查询
	result, count, err := q.FindByPage(opt.Offset(), opt.LimitInt())
	if err != nil {
		return nil, 0, err
	}

	return result, count, nil
}

// UpdateStatus 更新任务批次状态
func (dao *taskBatchDao) UpdateStatus(kit *kit.Kit, batchID uint32, status table.TaskBatchStatus) error {
	m := dao.genQ.TaskBatch
	q := dao.genQ.TaskBatch.WithContext(kit.Ctx)

	updates := make(map[string]interface{})
	updates["status"] = status

	// 如果状态是完成状态，设置结束时间
	if status == table.TaskBatchStatusSucceed || status == table.TaskBatchStatusFailed ||
		status == table.TaskBatchStatusPartlyFailed {
		now := time.Now()
		updates["end_at"] = &now
	}

	_, err := q.Where(m.ID.Eq(batchID)).Updates(updates)
	return err
}

// ListExecutors 查询指定业务下所有的执行帐户
func (dao *taskBatchDao) ListExecutors(kit *kit.Kit, bizID uint32) ([]string, error) {
	m := dao.genQ.TaskBatch
	q := dao.genQ.TaskBatch.WithContext(kit.Ctx)

	var executors []string
	err := q.Select(m.Creator.Distinct()).
		Where(m.BizID.Eq(bizID)).
		Pluck(m.Creator, &executors)
	if err != nil {
		return nil, err
	}

	return executors, nil
}

// buildFilterConditions 构建任务批次过滤条件
func (dao *taskBatchDao) buildFilterConditions(filter *TaskBatchListFilter) []rawgen.Condition {
	var conds []rawgen.Condition
	if filter == nil {
		return conds
	}

	m := dao.genQ.TaskBatch

	// 任务对象类型过滤
	if len(filter.TaskObjects) > 0 {
		conds = append(conds, m.TaskObject.In(filter.TaskObjects...))
	}

	// 任务动作过滤
	if len(filter.TaskActions) > 0 {
		conds = append(conds, m.TaskAction.In(filter.TaskActions...))
	}

	// 执行状态过滤
	if len(filter.Statuses) > 0 {
		conds = append(conds, m.Status.In(filter.Statuses...))
	}

	// 执行帐户过滤
	if len(filter.Executors) > 0 {
		conds = append(conds, m.Creator.In(filter.Executors...))
	}

	// 时间范围过滤：任务的开始时间或结束时间在指定范围内
	if filter.TimeRangeStart != nil {
		// 任务的结束时间 >= 查询起点
		conds = append(conds, m.EndAt.Gte(*filter.TimeRangeStart))
	}
	if filter.TimeRangeEnd != nil {
		// 任务的开始时间 <= 查询终点
		conds = append(conds, m.StartAt.Lte(*filter.TimeRangeEnd))
	}

	return conds
}
