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
	"time"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/internal/task"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// TaskRecordFilter 任务记录查询条件。
// 任务记录表由任务框架写入，字段与框架保持一致，此处只做只读查询。
// 人员多选是框架 store 的 ListOption 不具备的能力（其 Creator 为单值），
// 因此统计链路必须直接查询该表。
type TaskRecordFilter struct {
	// TaskIndexType 任务索引类型，统计链路固定为 task_batch，用于排除其它来源的任务
	TaskIndexType string
	// TaskIndexes 任务索引值列表，统计链路传入批次 ID，为空表示不限
	TaskIndexes []string
	// Creators 人员多选，为空表示不限
	Creators []string
	// Statuses 任务状态列表，为空表示不限
	Statuses []string
	// ExcludeStatuses 排除的任务状态列表：用于「除成功与幂等忽略外都算失败」的判定
	ExcludeStatuses []string
	// StartTime / EndTime 按任务开始时间过滤，未开始的任务回退到创建时间
	StartTime *time.Time
	EndTime   *time.Time
}

// TaskFailureFilter 失败明细查询条件，以任务批次表为驱动表。
// 租户、业务、任务对象与批次终态都落在 task_batches 上，task_records 只按 task_index 被 join，
// 因此不存在批次 ID 的 IN 列表，分页与排序由数据库一次完成。
type TaskFailureFilter struct {
	// TenantID 租户 ID，为空表示不限；平台级统计必须传入，否则会跨租户取数
	TenantID string
	// TaskIndexType 任务索引类型，固定 task_batch
	TaskIndexType string
	// TaskObjects 任务对象列表，为空表示不限
	TaskObjects []string
	// BatchStatuses 批次状态列表，为空表示不限
	BatchStatuses []string
	// BatchStartTime / BatchEndTime 按批次创建时间过滤
	BatchStartTime *time.Time
	BatchEndTime   *time.Time
	// Creators 批次创建人，为空表示不限
	Creators []string
	// ExcludeStatuses 排除的任务状态：用于「除成功与幂等忽略外都算失败」的判定
	ExcludeStatuses []string
}

// TaskFailureRow 失败明细基础行：只包含筛选与展示所需的轻量列，不含 common_payload。
// 明细的扩展筛选（错误码、失败信息、时间等）在服务层基于这批行计算，
// 因此这里不读取 payload 大字段，命中分页后再单独回填。
type TaskFailureRow struct {
	TaskID    string    `gorm:"column:task_id"`
	TaskIndex string    `gorm:"column:task_index"`
	Status    string    `gorm:"column:status"`
	Message   string    `gorm:"column:message"`
	Creator   string    `gorm:"column:creator"`
	Start     time.Time `gorm:"column:start"`
	End       time.Time `gorm:"column:end"`
	BizID     uint32    `gorm:"column:biz_id"`
}

// TaskStatusCount 任务状态计数
type TaskStatusCount struct {
	Status string `gorm:"column:status"`
	Count  int64  `gorm:"column:count"`
}

// TaskRecord 任务记录只读 DAO
type TaskRecord interface {
	// StatusCount 按状态统计任务数，用于概览卡片
	StatusCount(kit *kit.Kit, filter *TaskRecordFilter) (map[string]int64, error)
	// ListForStatistics 查询用于图表聚合的轻量列（不含 payload / common_payload）
	ListForStatistics(kit *kit.Kit, filter *TaskRecordFilter, limit int) ([]*task.Record, error)
	// ListFailureRows 以批次表为驱动表查询基础筛选范围内的失败任务轻量行
	ListFailureRows(kit *kit.Kit, filter *TaskFailureFilter, limit int) ([]*TaskFailureRow, error)
	// ListFailurePayloads 按任务 ID 查询 common_payload，用于明细分页行回填
	ListFailurePayloads(kit *kit.Kit, taskIDs []string) (map[string]string, error)
}

type taskRecordDao struct {
	genQ *gen.Query
}

// taskRecordTable 返回指向 task_records 的查询入口。
// 任务记录表由任务框架维护，没有 gen 模型，这里借 TaskBatch 的 Query 再覆盖表名。
// 软删除条件（`task_records`.`deleted_at` IS NULL）由 GORM 依据模型的 DeletedAt 字段自动追加，
// 不要再手写 deleted_at IS NULL，否则同一条 SQL 里会出现两遍过滤。
func (dao *taskRecordDao) taskRecordTable(kit *kit.Kit) *gorm.DB {
	return dao.genQ.TaskBatch.WithContext(kit.Ctx).UnderlyingDB().WithContext(kit.Ctx).
		Table("task_records")
}

// StatusCount implements TaskRecord.
// 展开后的 SQL 形态（批次 ID 由调用方按 500 一组分片传入）：
//
//	SELECT status, COUNT(*) AS count FROM `task_records`
//	WHERE task_index_type = 'task_batch'
//	  AND task_index IN ('1001','1002','1003')
//	  AND `task_records`.`deleted_at` IS NULL
//	GROUP BY `status`
func (dao *taskRecordDao) StatusCount(kit *kit.Kit, filter *TaskRecordFilter) (map[string]int64, error) {
	// 聚合结果扫描到自定义结构体时，GORM 不会追加模型的软删除条件（只有 dest 是带 DeletedAt
	// 的模型时才会），这里显式过滤，避免已删除记录被算进总数、与明细口径不一致。
	db := dao.applyFilter(dao.taskRecordTable(kit), filter).
		Where("`task_records`.`deleted_at` IS NULL")

	var rows []TaskStatusCount
	if err := db.Select("status, COUNT(*) AS count").Group("status").Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[string]int64, len(rows))
	for _, row := range rows {
		result[row.Status] = row.Count
	}
	return result, nil
}

// ListForStatistics implements TaskRecord.
// 只返回聚合所需的 task_index / message / creator，不读 payload / common_payload。
// limit <= 0 表示不限，由调用方按批次分片保证单次结果集大小。
//
//	SELECT task_index, message, creator
//	FROM `task_records`
//	WHERE task_index_type = 'task_batch'
//	  AND task_index IN ('1001','1002','1003')
//	  AND status NOT IN ('SUCCESS','IGNORED')
//	  AND `task_records`.`deleted_at` IS NULL
//	ORDER BY created_at DESC, id DESC
func (dao *taskRecordDao) ListForStatistics(kit *kit.Kit, filter *TaskRecordFilter,
	limit int) ([]*task.Record, error) {
	db := dao.applyFilter(dao.taskRecordTable(kit), filter)

	records := make([]*task.Record, 0)
	db = db.Select(aggregationColumns).Order("created_at DESC, id DESC")
	if limit > 0 {
		db = db.Limit(limit)
	}
	if err := db.Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// ListFailureRows implements TaskRecord.
// 只按基础筛选条件（租户、任务对象、批次终态、时间窗、批次创建人）取数，
// 扩展筛选（任务 ID、状态、业务、错误码、失败信息等）由服务层在同一批行上完成，
// 以保证 filter_options 始终基于基础范围生成，不随扩展筛选收缩。
// limit <= 0 表示不限，调用方通过 limit+1 判断是否超出扫描上限。
func (dao *taskRecordDao) ListFailureRows(kit *kit.Kit, filter *TaskFailureFilter,
	limit int) ([]*TaskFailureRow, error) {
	// 按任务记录创建时间倒序：未执行的任务 start 为空值，用 start 排序会把它们沉到最后，
	// 而这些任务往往是最新产生的失败。
	db := dao.failureDetailQuery(kit, filter).
		Select(failureRowColumns).
		Order("r.created_at DESC, r.id DESC")
	if limit > 0 {
		db = db.Limit(limit)
	}

	rows := make([]*TaskFailureRow, 0)
	if err := db.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListFailurePayloads implements TaskRecord.
// 只在明细分页命中后调用，避免为全量筛选扫描 common_payload（配置快照数 KB~十几 KB）。
// 结果扫描到自定义结构体，GORM 不会自动追加软删除条件，这里显式过滤。
func (dao *taskRecordDao) ListFailurePayloads(kit *kit.Kit, taskIDs []string) (map[string]string, error) {
	result := make(map[string]string, len(taskIDs))
	if len(taskIDs) == 0 {
		return result, nil
	}

	rows := make([]*taskFailurePayload, 0, len(taskIDs))
	err := dao.taskRecordTable(kit).
		Select("task_id, common_payload").
		Where("task_id IN ?", taskIDs).
		Where("`task_records`.`deleted_at` IS NULL").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.TaskID] = row.CommonPayload
	}
	return result, nil
}

// failureDetailQuery 构造批次表驱动的失败明细查询。
// join 用 CAST(b.id AS CHAR)：批次表先按租户、任务对象、批次终态与时间窗收窄，再由批次主键
// 驱动 task_records，task_index 侧因此能走 idx_task_index，一条 SQL 完成 join 与排序。
// CAST 出来的字符串沿用连接默认排序规则，与 task_index 列比较时两端排序规则必须一致，否则报
// Error 1267 Illegal mix of collations。已确认列与连接都是 utf8mb4_unicode_ci，这里显式
// COLLATE 对齐，不再做运行时探测；换库后若复现 1267，应同步调整此处与库的排序规则。
//
// 时间窗与人员条件展开后的 SQL 形态：
//
//	SELECT r.task_id, r.task_name, r.task_index, r.status, r.message, r.creator, r.start, r.end, b.biz_id
//	FROM task_batches AS b
//	JOIN task_records AS r
//	  ON r.task_index = CAST(b.id AS CHAR) COLLATE utf8mb4_unicode_ci
//	 AND r.task_index_type = 'task_batch'
//	WHERE r.deleted_at IS NULL
//	  AND b.tenant_id = 'default'
//	  AND b.task_object IN ('process','config_file')
//	  AND b.status IN ('succeed','failed','partly_failed')
//	  AND b.created_at >= '2026-11-18 00:00:00'
//	  AND b.created_at <= '2026-11-25 00:00:00'
//	  AND r.status NOT IN ('SUCCESS','IGNORED')
//	ORDER BY r.created_at DESC, r.id DESC
//	LIMIT 120001
func (dao *taskRecordDao) failureDetailQuery(kit *kit.Kit, filter *TaskFailureFilter) *gorm.DB {
	joinCond := "JOIN task_records AS r ON r.task_index = CAST(b.id AS CHAR) COLLATE utf8mb4_unicode_ci"
	joinArgs := make([]interface{}, 0, 1)
	if filter.TaskIndexType != "" {
		joinCond += " AND r.task_index_type = ?"
		joinArgs = append(joinArgs, filter.TaskIndexType)
	}

	// Unscoped 关闭 GORM 的软删除注入：这里用 Table 指定了别名，模型自带的软删除条件会
	// 拼到 task_batches 上（该表无 deleted_at 列），记录表的软删除条件改为显式书写。
	db := dao.genQ.TaskBatch.WithContext(kit.Ctx).UnderlyingDB().Unscoped().
		Table("task_batches AS b").
		Joins(joinCond, joinArgs...).
		Where("r.deleted_at IS NULL")
	// 默认租户在历史数据里存在空串与 "default" 两种写法，两者都要匹配。
	switch filter.TenantID {
	case "", constant.DefaultTenantID:
		db = db.Where("b.tenant_id IN ?", []string{"", constant.DefaultTenantID})
	default:
		db = db.Where("b.tenant_id = ?", filter.TenantID)
	}
	if len(filter.TaskObjects) > 0 {
		db = db.Where("b.task_object IN ?", filter.TaskObjects)
	}
	if len(filter.BatchStatuses) > 0 {
		db = db.Where("b.status IN ?", filter.BatchStatuses)
	}
	if filter.BatchStartTime != nil {
		db = db.Where("b.created_at >= ?", *filter.BatchStartTime)
	}
	if filter.BatchEndTime != nil {
		db = db.Where("b.created_at <= ?", *filter.BatchEndTime)
	}
	if len(filter.Creators) > 0 {
		db = db.Where("b.creator IN ?", filter.Creators)
	}
	if len(filter.ExcludeStatuses) > 0 {
		db = db.Where("r.status NOT IN ?", filter.ExcludeStatuses)
	}
	return db
}

// aggregationColumns 图表聚合所需的最小列集合：只用于算错误码、人员与业务计数。
// 含 message（错误码来源）与 creator，其余列不返回，减少内存占用；
// 刻意不包含 payload / common_payload，后者含配置快照（数 KB~十几 KB）。
const aggregationColumns = "task_index, message, creator"

// failureRowColumns 失败明细基础行所需列，带 join 别名。
// task_index 用于还原批次 ID；common_payload 不在基础扫描中读取，分页命中后按任务 ID 回填。
// id、task_name、task_index_type、execution_time、created_at 不返回。
const failureRowColumns = "r.task_id, r.task_index, r.status, r.message, r.creator, " +
	"r.start, r.end, b.biz_id"

// taskFailurePayload 明细分页行回填所需的 common_payload。
type taskFailurePayload struct {
	TaskID        string `gorm:"column:task_id"`
	CommonPayload string `gorm:"column:common_payload"`
}

// applyFilter 应用过滤条件。
// 时间统一按 created_at 过滤：统计以任务记录的创建时间为准，
// 未执行的任务 start 为空值，按 start 过滤会把它们整体排除。
func (dao *taskRecordDao) applyFilter(db *gorm.DB, filter *TaskRecordFilter) *gorm.DB {
	if filter == nil {
		return db
	}
	if filter.TaskIndexType != "" {
		db = db.Where("task_index_type = ?", filter.TaskIndexType)
	}
	if len(filter.TaskIndexes) > 0 {
		db = db.Where("task_index IN ?", filter.TaskIndexes)
	}
	if len(filter.Creators) > 0 {
		db = db.Where("creator IN ?", filter.Creators)
	}
	if len(filter.Statuses) > 0 {
		db = db.Where("status IN ?", filter.Statuses)
	}
	if len(filter.ExcludeStatuses) > 0 {
		db = db.Where("status NOT IN ?", filter.ExcludeStatuses)
	}
	if filter.StartTime != nil {
		db = db.Where("created_at >= ?", *filter.StartTime)
	}
	if filter.EndTime != nil {
		db = db.Where("created_at <= ?", *filter.EndTime)
	}
	return db
}
