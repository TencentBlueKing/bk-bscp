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
	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
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
	GetByID(kit *kit.Kit, bizID, batchID uint32) (*table.TaskBatch, error)
	List(kit *kit.Kit, bizID uint32, filter *TaskBatchListFilter, opt *types.BasePage) ([]*table.TaskBatch, int64, error)
	// ListRefs 查询统计所需的批次引用（ID / 业务ID / 创建时间）。
	// 与 List 的区别：不按单个 biz_id 强约束，用于平台级跨业务统计；不分页，limit <= 0 表示不限。
	ListRefs(kit *kit.Kit, filter *TaskBatchListFilter, bizIDs []uint32,
		limit int) ([]*table.TaskBatch, error)
	UpdateStatus(kit *kit.Kit, batchID uint32, status table.TaskBatchStatus) error
	ListExecutors(kit *kit.Kit, bizID uint32) ([]string, error)
	// IncrementCompletedCount 增加完成任务计数，当所有任务完成时自动更新批次状态
	IncrementCompletedCount(kit *kit.Kit, batchID uint32, isSuccess bool) (bool, error)
	// FinishBatch 以权威计数收敛批次终态。
	// 由任务组回调调用：任务框架在单个事务内维护任务组计数，比逐个任务累加的进度更准确，
	// 也覆盖了被级联阻断、从未执行因而不会触发自身回调的那部分任务。
	FinishBatch(kit *kit.Kit, batchID uint32, successCount, failedCount uint32) error
	// ResetCountsForRetry 重置计数字段用于重试
	ResetCountsForRetry(kit *kit.Kit, batchID uint32, totalCount uint32) error
	// AddFailedCount 增加失败计数（用于任务创建失败的场景），同时增加 CompletedCount 和 FailedCount
	AddFailedCount(kit *kit.Kit, batchID uint32, count uint32) error
	// HasRunningConfigPushTasks 检查是否有指定配置模板的运行中的配置下发任务
	HasRunningConfigPushTasks(kit *kit.Kit, bizID uint32, configTemplateIDs []uint32) (bool, error)
	// UpdateExtraData 更新批次的 ExtraData 字段
	UpdateExtraData(kit *kit.Kit, batchID uint32, extraData string) error
}

var _ TaskBatch = new(taskBatchDao)

type taskBatchDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
}

// UpdateExtraData implements [TaskBatch].
func (dao *taskBatchDao) UpdateExtraData(kit *kit.Kit, batchID uint32, extraData string) error {
	m := dao.genQ.TaskBatch
	_, err := dao.genQ.TaskBatch.WithContext(kit.Ctx).Where(m.ID.Eq(batchID)).Updates(map[string]interface{}{
		m.ExtraData.ColumnName().String(): extraData,
	})
	if err != nil {
		return err
	}

	return nil
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
func (dao *taskBatchDao) GetByID(kit *kit.Kit, bizID, batchID uint32) (*table.TaskBatch, error) {
	m := dao.genQ.TaskBatch
	q := dao.genQ.TaskBatch.WithContext(kit.Ctx)

	taskBatch, err := q.Where(m.ID.Eq(batchID), m.BizID.Eq(bizID)).Take()
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

// ListRefs 查询统计所需的批次引用。
// 时间过滤使用批次创建时间：批次的 start_at / end_at 只在任务真正开始执行后才写入，
// 用它们过滤会把"已创建但未开始"的批次整体排除，导致统计总数偏小。
func (dao *taskBatchDao) ListRefs(kit *kit.Kit, filter *TaskBatchListFilter, bizIDs []uint32,
	limit int) ([]*table.TaskBatch, error) {
	m := dao.genQ.TaskBatch
	q := dao.genQ.TaskBatch.WithContext(kit.Ctx)

	var conds []rawgen.Condition
	// 不按 biz_id 收窄后，必须用 tenant_id 兜住租户边界，否则平台级统计会跨租户取数，
	// 同时命中 idx_tenantID_bizID_ccProcessID 的最左前缀。
	// 默认租户存在空串与 "default" 两种历史写法，必须同时匹配。
	switch kit.TenantID {
	case "", constant.DefaultTenantID:
		conds = append(conds, m.TenantID.In("", constant.DefaultTenantID))
	default:
		conds = append(conds, m.TenantID.Eq(kit.TenantID))
	}
	if filter != nil {
		if len(filter.TaskObjects) > 0 {
			conds = append(conds, m.TaskObject.In(filter.TaskObjects...))
		}
		if len(filter.TaskActions) > 0 {
			conds = append(conds, m.TaskAction.In(filter.TaskActions...))
		}
		if filter.TimeRangeStart != nil {
			conds = append(conds, m.CreatedAt.Gte(*filter.TimeRangeStart))
		}
		if filter.TimeRangeEnd != nil {
			conds = append(conds, m.CreatedAt.Lte(*filter.TimeRangeEnd))
		}
		if len(filter.Statuses) > 0 {
			conds = append(conds, m.Status.In(filter.Statuses...))
		}
		if len(filter.Executors) > 0 {
			conds = append(conds, m.Creator.In(filter.Executors...))
		}
	}
	if len(bizIDs) > 0 {
		conds = append(conds, m.BizID.In(bizIDs...))
	}

	query := q.Where(conds...).
		Select(m.ID, m.BizID, m.CreatedAt).
		Order(m.ID.Desc())
	if limit > 0 {
		query = query.Limit(limit)
	}
	return query.Find()
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

// IncrementCompletedCount 增加完成任务计数，当所有任务完成时自动更新批次状态
func (dao *taskBatchDao) IncrementCompletedCount(kit *kit.Kit, batchID uint32, isSuccess bool) (bool, error) {
	m := dao.genQ.TaskBatch
	var allCompleted bool

	txFunc := func(tx *gen.Query) error {
		q := tx.TaskBatch.WithContext(kit.Ctx)

		// 更新完成任务计数
		updates := map[string]interface{}{
			"completed_count": gorm.Expr("completed_count + 1"),
		}
		if isSuccess {
			updates["success_count"] = gorm.Expr("success_count + 1")
		} else {
			updates["failed_count"] = gorm.Expr("failed_count + 1")
		}

		_, err := q.Where(m.ID.Eq(batchID)).Updates(updates)
		if err != nil {
			return fmt.Errorf("increment completed count failed: %w", err)
		}

		// 查询更新后的批次信息，判断是否所有任务都已完成
		batch, err := q.Where(m.ID.Eq(batchID)).Take()
		if err != nil {
			return fmt.Errorf("get task batch failed: %w", err)
		}

		// 如果所有任务都已完成，更新批次状态
		if batch.Spec.CompletedCount >= batch.Spec.TotalCount && batch.Spec.TotalCount > 0 {
			allCompleted = true
			var newStatus table.TaskBatchStatus
			if batch.Spec.FailedCount == 0 {
				newStatus = table.TaskBatchStatusSucceed
			} else if batch.Spec.SuccessCount == 0 {
				newStatus = table.TaskBatchStatusFailed
			} else {
				newStatus = table.TaskBatchStatusPartlyFailed
			}

			now := time.Now()
			_, err = q.Where(m.ID.Eq(batchID)).Updates(map[string]interface{}{
				"status": newStatus,
				"end_at": &now,
			})
			if err != nil {
				return fmt.Errorf("update batch status failed: %w", err)
			}
		}

		return nil
	}

	if err := dao.genQ.Transaction(txFunc); err != nil {
		return false, err
	}

	return allCompleted, nil
}

// ResetCountsForRetry 重置计数字段用于重试，根据重试数量扣除完成计数和失败计数
func (dao *taskBatchDao) ResetCountsForRetry(kit *kit.Kit, batchID uint32, retryCount uint32) error {
	m := dao.genQ.TaskBatch
	q := dao.genQ.TaskBatch.WithContext(kit.Ctx)

	_, err := q.Where(m.ID.Eq(batchID)).Updates(map[string]interface{}{
		"completed_count": gorm.Expr("completed_count - ?", retryCount),
		"failed_count":    gorm.Expr("failed_count - ?", retryCount),
		"status":          table.TaskBatchStatusRunning,
		"end_at":          nil,
	})
	if err != nil {
		return fmt.Errorf("reset counts for retry failed: %w", err)
	}

	return nil
}

// AddFailedCount 增加失败计数（用于任务创建失败的场景），同时增加 CompletedCount 和 FailedCount
func (dao *taskBatchDao) AddFailedCount(kit *kit.Kit, batchID uint32, count uint32) error {
	m := dao.genQ.TaskBatch

	txFunc := func(tx *gen.Query) error {
		q := tx.TaskBatch.WithContext(kit.Ctx)

		// 增加 completed_count 和 failed_count
		_, err := q.Where(m.ID.Eq(batchID)).Updates(map[string]interface{}{
			"completed_count": gorm.Expr("completed_count + ?", count),
			"failed_count":    gorm.Expr("failed_count + ?", count),
		})
		if err != nil {
			return fmt.Errorf("add failed count failed: %w", err)
		}

		// 查询更新后的批次，判断是否所有任务都已完成
		batch, err := q.Where(m.ID.Eq(batchID)).Take()
		if err != nil {
			return fmt.Errorf("get task batch failed: %w", err)
		}

		// 如果所有任务都已完成，更新批次状态
		if batch.Spec.CompletedCount >= batch.Spec.TotalCount && batch.Spec.TotalCount > 0 {
			var newStatus table.TaskBatchStatus
			if batch.Spec.FailedCount == 0 {
				newStatus = table.TaskBatchStatusSucceed
			} else if batch.Spec.SuccessCount == 0 {
				newStatus = table.TaskBatchStatusFailed
			} else {
				newStatus = table.TaskBatchStatusPartlyFailed
			}

			now := time.Now()
			_, err = q.Where(m.ID.Eq(batchID)).Updates(map[string]interface{}{
				"status": newStatus,
				"end_at": &now,
			})
			if err != nil {
				return fmt.Errorf("update batch status failed: %w", err)
			}
		}

		return nil
	}

	if err := dao.genQ.Transaction(txFunc); err != nil {
		return err
	}

	return nil
}

// HasRunningConfigPushTasks 检查是否有指定配置模板的运行中的配置下发任务
// 通过查询 task_batch 表的 task_data 字段来判断
func (dao *taskBatchDao) HasRunningConfigPushTasks(kit *kit.Kit, bizID uint32, configTemplateIDs []uint32) (bool, error) {
	if len(configTemplateIDs) == 0 {
		return false, nil
	}

	m := dao.genQ.TaskBatch
	q := dao.genQ.TaskBatch.WithContext(kit.Ctx)

	// 查询运行中的配置下发任务批次
	batches, err := q.Where(
		m.BizID.Eq(bizID),
		m.TaskAction.Eq(string(table.TaskActionConfigPublish)),
		m.Status.Eq(string(table.TaskBatchStatusRunning)),
	).Find()
	if err != nil {
		return false, fmt.Errorf("query running config push tasks failed: %w", err)
	}

	// 如果没有运行中的任务，直接返回
	if len(batches) == 0 {
		return false, nil
	}

	// 构建待检查的配置模板ID集合，用于判断配置模版是否在运行中的任务中
	templateIDSet := make(map[uint32]struct{}, len(configTemplateIDs))
	for _, id := range configTemplateIDs {
		templateIDSet[id] = struct{}{}
	}

	// 遍历运行中的批次，检查是否有冲突的配置模板
	for _, batch := range batches {
		// 解析 task_data 中的配置模板ID列表
		taskData, err := batch.Spec.GetTaskExecutionData()
		if err != nil {
			return false, fmt.Errorf("get task execution data failed: %w", err)
		}

		// 检查是否有交集
		for _, runningTemplateID := range taskData.ConfigTemplateIDs {
			if _, exists := templateIDSet[runningTemplateID]; exists {
				// 找到冲突的配置模版
				return true, nil
			}
		}
	}

	return false, nil
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

// FinishBatch 以任务组的权威计数收敛批次终态
func (dao *taskBatchDao) FinishBatch(kit *kit.Kit, batchID uint32, successCount, failedCount uint32) error {
	m := dao.genQ.TaskBatch

	return dao.genQ.Transaction(func(tx *gen.Query) error {
		q := tx.TaskBatch.WithContext(kit.Ctx)

		var status table.TaskBatchStatus
		switch {
		case failedCount == 0:
			status = table.TaskBatchStatusSucceed
		case successCount == 0:
			status = table.TaskBatchStatusFailed
		default:
			status = table.TaskBatchStatusPartlyFailed
		}

		now := time.Now()
		_, err := q.Where(m.ID.Eq(batchID)).Updates(map[string]interface{}{
			"completed_count": successCount + failedCount,
			"success_count":   successCount,
			"failed_count":    failedCount,
			"status":          status,
			"end_at":          &now,
		})
		if err != nil {
			return fmt.Errorf("finish task batch failed: %w", err)
		}
		return nil
	})
}
