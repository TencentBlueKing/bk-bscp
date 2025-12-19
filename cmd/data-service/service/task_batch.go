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

package service

import (
	"context"
	"fmt"
	"time"

	taskpkg "github.com/Tencent/bk-bcs/bcs-common/common/task"
	istore "github.com/Tencent/bk-bcs/bcs-common/common/task/stores/iface"
	taskTypes "github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	commonExecutor "github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbtb "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/task_batch"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// ListTaskBatch implements pbds.DataServer.
func (s *Service) ListTaskBatch(ctx context.Context, req *pbds.ListTaskBatchReq) (*pbds.ListTaskBatchResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 默认分页参数
	limit := uint(req.Limit)
	if limit == 0 {
		limit = 50
	}

	opt := &types.BasePage{
		Start: req.Start,
		Limit: limit,
	}
	// 转换proto的SortRule为types.BasePage的Sort和Order
	if req.Sort != nil && req.Sort.Field != "" {
		opt.Sort = req.Sort.Field
		if req.Sort.Order == string(types.Ascending) {
			opt.Order = types.Ascending
		} else {
			opt.Order = types.Descending // 默认倒序
		}
	}

	// 验证分页参数
	if err := opt.Validate(types.DefaultPageOption); err != nil {
		return nil, err
	}

	filter, err := buildTaskBatchListFilter(req)
	if err != nil {
		logs.Errorf("build task batch list filter failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	res, count, err := s.dao.TaskBatch().List(kt, req.BizId, filter, opt)
	if err != nil {
		logs.Errorf("list task batch failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 获取查询过滤选项
	filterOptions, err := getFilterOptions(kt, req.BizId, s.dao.TaskBatch())
	if err != nil {
		logs.Errorf("get filter options failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return &pbds.ListTaskBatchResp{
		Count:         uint32(count),
		List:          pbtb.PbTaskBatches(res),
		FilterOptions: filterOptions,
	}, nil
}

// getFilterOptions 获取查询过滤选项
func getFilterOptions(kt *kit.Kit, bizID uint32, dao dao.TaskBatch) (*pbtb.FilterOptions, error) {
	// 获取任务对象查询选项
	taskObjectChoices := make([]*pbtb.Choice, 0)
	for _, choice := range table.GetTaskObjectChoices() {
		taskObjectChoices = append(taskObjectChoices, &pbtb.Choice{
			Id:   choice.ID,
			Name: choice.Name,
		})
	}

	// 获取任务动作查询选项
	taskActionChoices := make([]*pbtb.Choice, 0)
	for _, choice := range table.GetTaskActionChoices() {
		taskActionChoices = append(taskActionChoices, &pbtb.Choice{
			Id:   choice.ID,
			Name: choice.Name,
		})
	}

	// 获取执行状态查询选项
	statusChoices := make([]*pbtb.Choice, 0)
	for _, choice := range table.GetTaskBatchStatusChoices() {
		statusChoices = append(statusChoices, &pbtb.Choice{
			Id:   choice.ID,
			Name: choice.Name,
		})
	}

	// 获取执行帐户查询选项
	executorChoices := make([]*pbtb.Choice, 0)
	executors, err := dao.ListExecutors(kt, bizID)
	if err != nil {
		return nil, fmt.Errorf("list distinct executors failed: %v", err)
	}
	for _, executor := range executors {
		executorChoices = append(executorChoices, &pbtb.Choice{
			Id:   executor,
			Name: executor,
		})
	}

	return &pbtb.FilterOptions{
		TaskObjectChoices: taskObjectChoices,
		TaskActionChoices: taskActionChoices,
		StatusChoices:     statusChoices,
		ExecutorChoices:   executorChoices,
	}, nil
}

// buildTaskBatchListFilter 构建任务批次列表过滤条件
func buildTaskBatchListFilter(req *pbds.ListTaskBatchReq) (*dao.TaskBatchListFilter, error) {
	filter := &dao.TaskBatchListFilter{
		TaskObjects: req.GetTaskObjects(),
		TaskActions: req.GetTaskActions(),
		Statuses:    req.GetStatuses(),
		Executors:   req.GetExecutors(),
	}

	// 解析时间范围参数
	var err error
	if filter.TimeRangeStart, err = parseTimeIfNotEmpty(req.TimeRangeStart, "time_range_start"); err != nil {
		return nil, err
	}
	if filter.TimeRangeEnd, err = parseTimeIfNotEmpty(req.TimeRangeEnd, "time_range_end"); err != nil {
		return nil, err
	}

	return filter, nil
}

// GetTaskBatchDetail implements pbds.DataServer.
func (s *Service) GetTaskBatchDetail(
	ctx context.Context,
	req *pbds.GetTaskBatchDetailReq,
) (*pbds.GetTaskBatchDetailResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 默认分页参数
	limit := int64(req.GetLimit())
	if limit == 0 {
		limit = 50
	}

	taskStorage := taskpkg.GetGlobalStorage()
	if taskStorage == nil {
		return nil, fmt.Errorf("task storage not initialized")
	}

	// 构建查询选项
	listOpt := &istore.ListOption{
		TaskIndex: fmt.Sprintf("%d", req.GetBatchId()),
		Limit:     limit,
		Offset:    int64(req.GetStart()),
	}

	// 构建过滤条件
	if req.GetStatus() != "" {
		// 支持状态过滤
		statusList := expandTaskStatusForQuery(req.GetStatus())
		listOpt.StatusList = statusList
	}
	// TODO: 支持其他过滤条件
	// - SetNames: 集群名称列表过滤
	// - ModuleNames: 模块名称列表过滤
	// - ServiceNames: 服务名称列表过滤
	// - ProcessAliases: 进程别名列表过滤
	// - CcProcessIds: CC进程ID列表过滤
	// - InstIds: 实例ID列表过滤
	// 这些过滤条件需要从 CommonPayload 中查询，等表设计支持后再实现

	pagination, err := taskStorage.ListTask(ctx, listOpt)
	if err != nil {
		logs.Errorf("list tasks failed, err: %v, rid: %s", err, kt.Rid)
		return nil, fmt.Errorf("list tasks failed: %v", err)
	}
	if pagination == nil {
		logs.Errorf("list tasks returned nil pagination, rid: %s", kt.Rid)
		return nil, fmt.Errorf("list tasks returned nil pagination")
	}
	// 解析每个 task 的 CommonPayload，构建 TaskDetail
	taskDetails := make([]*pbtb.TaskDetail, 0, len(pagination.Items))
	var detail *pbtb.TaskDetail
	for _, task := range pagination.Items {
		detail, err = convertTaskToDetail(task)
		if err != nil {
			logs.Errorf("convert task to detail failed, taskID: %s, err: %v", task.TaskID, err)
			return nil, fmt.Errorf("convert task to detail failed: %v", err)
		}
		if detail == nil {
			continue
		}
		taskDetails = append(taskDetails, detail)
	}

	// 计算状态统计
	statistics, err := getTaskStatusStatistics(ctx, fmt.Sprintf("%d", req.GetBatchId()))
	if err != nil {
		logs.Errorf("get task status statistics failed, err: %v, rid: %s", err, kt.Rid)
		return nil, fmt.Errorf("get task status statistics failed: %v", err)
	}

	// 获取任务详情过滤选项
	filterOptions := getTaskDetailFilterOptions()

	// 查询 TaskBatch 信息
	taskBatch, err := s.dao.TaskBatch().GetByID(kt, req.GetBatchId())
	if err != nil {
		logs.Errorf("get task batch failed, batchID: %d, err: %v, rid: %s", req.GetBatchId(), err, kt.Rid)
		return nil, fmt.Errorf("get task batch failed: %v", err)
	}

	// 转换为 proto TaskBatch 以获取字段
	pbTaskBatch := pbtb.PbTaskBatch(taskBatch)

	// 构建响应
	resp := &pbds.GetTaskBatchDetailResp{
		Tasks:         taskDetails,
		Count:         uint32(pagination.Count),
		Statistics:    statistics,
		FilterOptions: filterOptions,
		TaskBatch:     pbTaskBatch,
	}

	return resp, nil
}

// convertTaskToDetail 将 task 转换为 pb 数据结构 TaskDetail
func convertTaskToDetail(task *taskTypes.Task) (*pbtb.TaskDetail, error) {
	if task == nil {
		return nil, fmt.Errorf("task is nil")
	}

	// 解析 CommonPayload 为 ProcessPayload
	var processPayload commonExecutor.TaskPayload
	err := task.GetCommonPayload(&processPayload)
	if err != nil {
		return nil, fmt.Errorf("get common payload failed: %v", err)
	}
	if processPayload.ProcessPayload == nil {
		logs.Infof(
			"skip task convert, process payload is nil, taskID: %s, taskType: %s",
			task.TaskID, task.GetTaskType(),
		)
		return nil, nil
	}

	// 构建返回的 TaskDetail
	detail := &pbtb.TaskDetail{
		TaskId:        task.TaskID,
		Status:        convertTaskStatus(task.Status),
		Message:       task.Message,
		Creator:       task.Creator,
		ExecutionTime: float32(task.ExecutionTime) / 1000.0,
		TaskPayload: &pbtb.TaskPayload{
			SetName:       processPayload.ProcessPayload.SetName,
			ModuleName:    processPayload.ProcessPayload.ModuleName,
			ServiceName:   processPayload.ProcessPayload.ServiceName,
			Environment:   processPayload.ProcessPayload.Environment,
			Alias:         processPayload.ProcessPayload.Alias,
			FuncName:      processPayload.ProcessPayload.FuncName,
			InnerIp:       processPayload.ProcessPayload.InnerIP,
			AgentId:       processPayload.ProcessPayload.AgentID,
			CcProcessId:   processPayload.ProcessPayload.CcProcessID,
			HostInstSeq:   processPayload.ProcessPayload.HostInstSeq,
			ModuleInstSeq: processPayload.ProcessPayload.ModuleInstSeq,
			ConfigData:    processPayload.ProcessPayload.ConfigData,
		},
	}

	return detail, nil
}

// getTaskStatusStatistics 获取任务状态统计信息
func getTaskStatusStatistics(ctx context.Context, taskIndex string) ([]*pbtb.TaskStatusStatItem, error) {
	taskStorage := taskpkg.GetGlobalStorage()
	if taskStorage == nil {
		return nil, fmt.Errorf("task storage not initialized")
	}

	// 定义需要统计的四种状态及其对应的实际查询状态列表
	statusQueries := map[string][]string{
		taskTypes.TaskStatusInit:    {taskTypes.TaskStatusInit},
		taskTypes.TaskStatusRunning: {taskTypes.TaskStatusRunning, taskTypes.TaskStatusRevoked, taskTypes.TaskStatusNotStarted},
		taskTypes.TaskStatusSuccess: {taskTypes.TaskStatusSuccess},
		taskTypes.TaskStatusFailure: {taskTypes.TaskStatusFailure, taskTypes.TaskStatusTimeout},
	}

	statusCounts := make(map[string]uint32)
	for normalizedStatus, statusList := range statusQueries {
		listOpt := &istore.ListOption{
			TaskIndex:  taskIndex,
			StatusList: statusList,
			Limit:      1, // 只需要统计数量，不需要实际数据
			Offset:     0,
		}

		pagination, err := taskStorage.ListTask(ctx, listOpt)
		if err != nil {
			return nil, fmt.Errorf("list tasks failed for status %s: %v", normalizedStatus, err)
		}

		statusCounts[normalizedStatus] = uint32(pagination.Count)
	}

	// 构建返回结果
	statistics := []*pbtb.TaskStatusStatItem{
		{
			Status:  taskTypes.TaskStatusInit,
			Count:   statusCounts[taskTypes.TaskStatusInit],
			Message: "任务初始化",
		},
		{
			Status:  taskTypes.TaskStatusRunning,
			Count:   statusCounts[taskTypes.TaskStatusRunning],
			Message: "任务运行中",
		},
		{
			Status:  taskTypes.TaskStatusSuccess,
			Count:   statusCounts[taskTypes.TaskStatusSuccess],
			Message: "任务成功",
		},
		{
			Status:  taskTypes.TaskStatusFailure,
			Count:   statusCounts[taskTypes.TaskStatusFailure],
			Message: "任务失败",
		},
	}

	return statistics, nil
}

// expandTaskStatusForQuery 将用户查询的状态扩展为实际要查询的状态列表
// 例如：查询 RUNNING 状态时，实际要查询 RUNNING、REVOKED、NOT_STARTED 三种状态
func expandTaskStatusForQuery(status string) []string {
	switch status {
	case taskTypes.TaskStatusRunning:
		// RUNNING 状态需要查询三种状态：RUNNING、REVOKED、NOT_STARTED
		return []string{
			taskTypes.TaskStatusRunning,
			taskTypes.TaskStatusRevoked,
			taskTypes.TaskStatusNotStarted,
		}
	case taskTypes.TaskStatusFailure:
		// FAILURE 状态包含 FAILURE 和 TIMEOUT
		return []string{
			taskTypes.TaskStatusFailure,
			taskTypes.TaskStatusTimeout,
		}
	case taskTypes.TaskStatusInit, taskTypes.TaskStatusSuccess:
		// INIT 和 SUCCESS 直接返回
		return []string{status}
	default:
		// 未知状态默认返回原状态
		return []string{status}
	}
}

// convertTaskStatus 将任务状态转换为四类：INIT, RUNNING, SUCCESS, FAILURE
func convertTaskStatus(status string) string {
	switch status {
	case taskTypes.TaskStatusInit:
		return taskTypes.TaskStatusInit
	case taskTypes.TaskStatusRunning, taskTypes.TaskStatusRevoked, taskTypes.TaskStatusNotStarted:
		// RUNNING、REVOKED、NOT_STARTED 都归类为 RUNNING
		return taskTypes.TaskStatusRunning
	case taskTypes.TaskStatusSuccess:
		return taskTypes.TaskStatusSuccess
	case taskTypes.TaskStatusFailure, taskTypes.TaskStatusTimeout:
		// FAILURE 和 TIMEOUT 都归类为 FAILURE
		return taskTypes.TaskStatusFailure
	default:
		// 未知状态默认为 FAILURE
		return taskTypes.TaskStatusFailure
	}
}

// parseTime 解析 RFC3339 格式的时间字符串
func parseTime(timeStr string) (time.Time, error) {
	return time.Parse(time.RFC3339, timeStr)
}

// parseTimeIfNotEmpty 如果时间字符串非空则解析，否则返回 nil
func parseTimeIfNotEmpty(timeStr, fieldName string) (*time.Time, error) {
	if timeStr == "" {
		return nil, nil
	}
	t, err := parseTime(timeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid %s format: %v", fieldName, err)
	}
	return &t, nil
}

// getTaskDetailFilterOptions 获取任务详情过滤选项
func getTaskDetailFilterOptions() *pbtb.TaskDetailFilterOptions {
	// todo: 等表设计支持从 CommonPayload 查询后再填充
	return &pbtb.TaskDetailFilterOptions{
		SetNameChoices:     []*pbtb.Choice{},
		ModuleNameChoices:  []*pbtb.Choice{},
		ServiceNameChoices: []*pbtb.Choice{},
		AliasChoices:       []*pbtb.Choice{},
		CcProcessIdChoices: []*pbtb.Choice{},
		InstIdChoices:      []*pbtb.Choice{},
	}
}
