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
	"google.golang.org/protobuf/proto"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	commonExecutor "github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/process"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbtb "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/task_batch"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// TaskStatusIgnored 任务展示状态：GSE 报「进程已处于目标态」（828 重复启动 / 829 无需停止），
// 操作结果即目标态，视为幂等成功但不计入 SUCCESS，独立转译为 IGNORED 展示。
// IGNORED 不入库，仅由 GsePayload.ErrorCode 转译得出。
const TaskStatusIgnored = "IGNORED"

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
		return nil, errf.Errorf(errf.InvalidParameter, "%s",
			i18n.T(kt, "validate page parameters failed, err: %v", err))
	}

	filter, err := buildTaskBatchListFilter(kt, req)
	if err != nil {
		logs.Errorf("build task batch list filter failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	res, count, err := s.dao.TaskBatch().List(kt, req.BizId, filter, opt)
	if err != nil {
		logs.Errorf("list task batch failed, err: %v, rid: %s", err, kt.Rid)
		return nil, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "list task batch failed, err: %v", err))
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
	for _, choice := range table.GetTaskObjectChoices(kt) {
		taskObjectChoices = append(taskObjectChoices, &pbtb.Choice{
			Id:   choice.ID,
			Name: choice.Name,
		})
	}

	// 获取任务动作查询选项
	taskActionChoices := make([]*pbtb.Choice, 0)
	for _, choice := range table.GetTaskActionChoices(kt) {
		taskActionChoices = append(taskActionChoices, &pbtb.Choice{
			Id:   choice.ID,
			Name: choice.Name,
		})
	}

	// 获取执行状态查询选项
	statusChoices := make([]*pbtb.Choice, 0)
	for _, choice := range table.GetTaskBatchStatusChoices(kt) {
		statusChoices = append(statusChoices, &pbtb.Choice{
			Id:   choice.ID,
			Name: choice.Name,
		})
	}

	// 获取执行帐户查询选项
	executorChoices := make([]*pbtb.Choice, 0)
	executors, err := dao.ListExecutors(kt, bizID)
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "list distinct executors failed, err: %v", err))
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
func buildTaskBatchListFilter(kt *kit.Kit, req *pbds.ListTaskBatchReq) (*dao.TaskBatchListFilter, error) {
	filter := &dao.TaskBatchListFilter{
		TaskObjects: req.GetTaskObjects(),
		TaskActions: req.GetTaskActions(),
		Statuses:    req.GetStatuses(),
		Executors:   req.GetExecutors(),
	}

	// 解析时间范围参数
	var err error
	if filter.TimeRangeStart, err = parseTimeIfNotEmpty(kt, req.TimeRangeStart, "time_range_start"); err != nil {
		return nil, err
	}
	if filter.TimeRangeEnd, err = parseTimeIfNotEmpty(kt, req.TimeRangeEnd, "time_range_end"); err != nil {
		return nil, err
	}

	return filter, nil
}

// GetTaskBatchDetail implements pbds.DataServer.
func (s *Service) GetTaskBatchDetail(ctx context.Context, req *pbds.GetTaskBatchDetailReq) (*pbds.GetTaskBatchDetailResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 构建响应
	resp := &pbds.GetTaskBatchDetailResp{
		FilterOptions: getTaskDetailFilterOptions(),
	}

	// 默认分页参数
	limit := int64(req.GetLimit())
	if limit == 0 {
		limit = 50
	}

	taskStorage := taskpkg.GetGlobalStorage()
	if taskStorage == nil {
		return nil, errf.Errorf(errf.Unknown, "%s",
			i18n.T(kt, "task storage is not initialized"))
	}

	// 1. 查询 TaskBatch 信息
	taskBatch, err := s.dao.TaskBatch().GetByID(kt, req.GetBizId(), req.GetBatchId())
	if err != nil {
		logs.Errorf("get task batch failed, batchID: %d, err: %v, rid: %s", req.GetBatchId(), err, kt.Rid)
		return nil, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "get task batch %d failed, err: %v", req.GetBatchId(), err))
	}

	if taskBatch == nil {
		return resp, nil
	}

	// 需忽略的错误码集合，命中该错误码的失败任务只影响批次整体状态判定
	ignoreSet := buildIgnoreErrorCodeSet(req.GetIgnoreErrorCodes())

	taskIndex := fmt.Sprintf("%d", req.GetBatchId())
	taskType := string(taskBatch.Spec.TaskAction)

	// IGNORED 展示状态不入库（由 GSE 幂等错误码 828/829 转译），存储层无法按其过滤与分页，
	// 须全量枚举候选任务内存过滤后再分页
	isIgnoredQuery := req.GetStatus() == TaskStatusIgnored

	var taskDetails []*pbtb.TaskDetail
	var totalCount uint32
	if isIgnoredQuery {
		// 过滤后总数独立计算并按过滤结果分页，避免用页内命中数充当总数导致无法翻页
		taskDetails, totalCount, err = listIgnoredTaskDetails(kt, taskStorage, taskIndex, taskType,
			int64(req.GetStart()), limit)
		if err != nil {
			return nil, err
		}
	} else {
		// 构建查询选项与过滤条件
		listOpt := &istore.ListOption{
			TaskIndex: taskIndex,
			TaskType:  taskType,
			Limit:     limit,
			Offset:    int64(req.GetStart()),
		}
		if req.GetStatus() != "" {
			// 支持状态过滤
			listOpt.StatusList = expandTaskStatusForQuery(req.GetStatus())
		}

		// TODO: 支持其他过滤条件
		// - SetNames: 集群名称列表过滤
		// - ModuleNames: 模块名称列表过滤
		// - ServiceNames: 服务名称列表过滤
		// - ProcessAliases: 进程别名列表过滤
		// - CcProcessIds: CC进程ID列表过滤
		// - InstIds: 实例ID列表过滤
		// 这些过滤条件需要从 CommonPayload 中查询，等表设计支持后再实现

		pagination, listErr := taskStorage.ListTask(ctx, listOpt)
		if listErr != nil {
			logs.Errorf("list tasks failed, err: %v, rid: %s", listErr, kt.Rid)
			return nil, errf.Errorf(errf.Unknown, "%s",
				i18n.T(kt, "list tasks from task storage failed, err: %v", listErr))
		}
		if pagination == nil {
			logs.Errorf("list tasks returned nil pagination, rid: %s", kt.Rid)
			return nil, errf.Errorf(errf.RecordNotFound, "%s",
				i18n.T(kt, "list tasks returned nil pagination"))
		}

		// 解析每个 task 的 CommonPayload，构建 TaskDetail
		taskDetails = make([]*pbtb.TaskDetail, 0, len(pagination.Items))
		for _, task := range pagination.Items {
			detail, convErr := convertTaskToDetail(kt, task)
			if convErr != nil {
				logs.Errorf("convert task to detail failed, taskID: %s, err: %v", task.TaskID, convErr)
				return nil, convErr
			}
			taskDetails = append(taskDetails, detail)
		}
		totalCount = uint32(pagination.Count)
	}

	// 计算状态统计
	statistics, err := getTaskStatusStatistics(kt, taskIndex, taskType)
	if err != nil {
		logs.Errorf("get task status statistics failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 转换为 proto TaskBatch 以获取字段
	resp.TaskBatch = pbtb.PbTaskBatch(taskBatch)
	// 根据 ignore_error_codes 重判批次整体状态（适配标准运维插件）
	if err = overrideTaskBatchStatusForIgnore(kt, resp.TaskBatch, taskBatch,
		taskIndex, taskType, statistics, ignoreSet); err != nil {
		logs.Errorf("override task batch status for ignore failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	resp.Statistics = statistics
	resp.Count = totalCount
	resp.Tasks = taskDetails

	return resp, nil
}

// overrideTaskBatchStatusForIgnore 在忽略错误码后，重判批次整体状态
// 把命中忽略错误码的失败任务视为成功后，重新计算批次整体状态
// 仅当批次已处于终态（失败/部分失败）时生效
func overrideTaskBatchStatusForIgnore(kt *kit.Kit, pbBatch *pbtb.TaskBatch, taskBatch *table.TaskBatch,
	taskIndex, taskType string, statistics []*pbtb.TaskStatusStatItem, ignoreSet map[int32]bool) error {
	if pbBatch == nil || len(ignoreSet) == 0 || taskBatch.Spec == nil {
		return nil
	}
	// 只重判终态批次，运行中/初始化批次保持原状态
	if taskBatch.Spec.Status != table.TaskBatchStatusFailed &&
		taskBatch.Spec.Status != table.TaskBatchStatusPartlyFailed {
		return nil
	}

	var success, failure, running, initializing uint32
	for _, s := range statistics {
		switch s.Status {
		case taskTypes.TaskStatusSuccess:
			success = s.Count
		case taskTypes.TaskStatusFailure:
			failure = s.Count
		case taskTypes.TaskStatusRunning:
			running = s.Count
		case taskTypes.TaskStatusInit:
			initializing = s.Count
		}
	}
	// 仍有未完成任务则不重判（终态批次理论上不会出现，防御处理）
	if running > 0 || initializing > 0 {
		return nil
	}
	// 没有失败任务，无需重判
	if failure == 0 {
		return nil
	}

	taskStorage := taskpkg.GetGlobalStorage()
	if taskStorage == nil {
		return errf.Errorf(errf.Unknown, "%s",
			i18n.T(kt, "task storage is not initialized"))
	}
	ignored, err := countIgnoredFailedTasks(kt, taskStorage, taskIndex, taskType, failure, ignoreSet)
	if err != nil {
		return err
	}
	if ignored == 0 {
		return nil
	}

	// 把命中忽略错误码的失败任务计入成功后重判
	effectiveFailure := failure - ignored
	effectiveSuccess := success + ignored
	switch {
	case effectiveFailure == 0:
		pbBatch.Status = string(table.TaskBatchStatusSucceed)
	case effectiveSuccess == 0:
		pbBatch.Status = string(table.TaskBatchStatusFailed)
	default:
		pbBatch.Status = string(table.TaskBatchStatusPartlyFailed)
	}
	return nil
}

// buildIgnoreErrorCodeSet 构建需忽略的错误码集合
func buildIgnoreErrorCodeSet(codes []int32) map[int32]bool {
	if len(codes) == 0 {
		return nil
	}
	set := make(map[int32]bool, len(codes))
	for _, c := range codes {
		set[c] = true
	}
	return set
}

// isIgnorableFailedTask 判断失败任务的 GSE 错误码是否命中忽略集合
func isIgnorableFailedTask(payload *commonExecutor.TaskPayload, ignoreSet map[int32]bool) bool {
	if len(ignoreSet) == 0 || payload == nil || payload.GsePayload == nil {
		return false
	}
	return ignoreSet[int32(payload.GsePayload.ErrorCode)]
}

// convertTaskToDetail 将 task 转换为 pb 数据结构 TaskDetail
func convertTaskToDetail(kt *kit.Kit, task *taskTypes.Task) (*pbtb.TaskDetail, error) {
	if task == nil {
		return nil, errf.Errorf(errf.RecordNotFound, "%s",
			i18n.T(kt, "task is nil"))
	}

	// 解析 CommonPayload 为 ProcessPayload
	var processPayload commonExecutor.TaskPayload
	err := task.GetCommonPayload(&processPayload)
	if err != nil {
		return nil, errf.Errorf(errf.Unknown, "%s",
			i18n.T(kt, "get common payload from task failed, err: %v", err))
	}
	// 构建返回的 TaskDetail
	detail := &pbtb.TaskDetail{
		TaskId:        task.TaskID,
		Status:        convertTaskStatus(task.Status),
		Message:       task.Message,
		Creator:       task.Creator,
		ExecutionTime: float32(task.ExecutionTime) / 1000.0,
	}
	// GSE 错误码 828/829（重复启动 / 无需停止）转译为 IGNORED 展示状态，
	// message 复用写入时记录的 GSE 错误信息；转译只看 ErrorCode、不看框架状态，
	// 运行中任务的负载尚无 GsePayload，天然不命中
	if processPayload.GsePayload != nil && process.IsIdempotentOperateError(processPayload.GsePayload.ErrorCode) {
		detail.Status = TaskStatusIgnored
		detail.Message = processPayload.GsePayload.ErrorMsg
	}
	if processPayload.ProcessPayload != nil {
		detail.TaskPayload = &pbtb.TaskPayload{
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
		}
		// 历史任务负载没有优先级，保持字段缺省，由前端展示为 --
		if p := processPayload.ProcessPayload.Priority; p != nil {
			detail.TaskPayload.Priority = proto.Int32(int32(*p))
		}
	}

	if processPayload.ConfigPayload != nil {
		detail.CompareStatus = string(processPayload.ConfigPayload.CompareStatus)
	}

	return detail, nil
}

// listIgnoredTaskDetails 列出 IGNORED 展示状态的任务：IGNORED 不入库（由 GSE 幂等错误码
// 828/829 转译），存储层无法按其过滤与分页，须按框架状态（SUCCESS / 存量 FAILURE、TIMEOUT）
// 全量流式枚举候选任务后内存过滤。
// 返回过滤后的总数 total 与 [start, start+limit) 页内任务明细；仅页窗口内任务执行
// convertTaskToDetail 全量转换，控制大批次下的解析开销。
func listIgnoredTaskDetails(kt *kit.Kit, taskStorage istore.Store, taskIndex, taskType string,
	start, limit int64) ([]*pbtb.TaskDetail, uint32, error) {

	const pageSize = int64(1000)
	pageEnd := start + limit

	details := make([]*pbtb.TaskDetail, 0)
	var filteredCnt int64

	offset := int64(0)
	for {
		pagination, err := taskStorage.ListTask(kt.Ctx, &istore.ListOption{
			TaskIndex: taskIndex,
			TaskType:  taskType,
			StatusList: []string{
				taskTypes.TaskStatusSuccess,
				taskTypes.TaskStatusFailure,
				taskTypes.TaskStatusTimeout,
			},
			Offset: offset,
			Limit:  pageSize,
		})
		if err != nil {
			return nil, 0, errf.Errorf(errf.Unknown, "%s",
				i18n.T(kt, "list tasks from task storage failed when paging ignored tasks, err: %v", err))
		}
		if len(pagination.Items) == 0 {
			break
		}

		for _, task := range pagination.Items {
			var payload commonExecutor.TaskPayload
			if err = task.GetCommonPayload(&payload); err != nil {
				// 单个任务 payload 解析失败不阻断查询，按未命中处理
				logs.Warnf("get common payload failed when paging ignored tasks, taskID: %s, err: %v, rid: %s",
					task.TaskID, err, kt.Rid)
				continue
			}
			// 与 convertTaskToDetail 的 IGNORED 转译同一判定口径：GSE 幂等错误码（828/829）
			if payload.GsePayload == nil || !process.IsIdempotentOperateError(payload.GsePayload.ErrorCode) {
				continue
			}

			// 仅对请求页窗口内的命中任务做完整转换
			if filteredCnt >= start && filteredCnt < pageEnd {
				detail, convErr := convertTaskToDetail(kt, task)
				if convErr != nil {
					logs.Errorf("convert task to detail failed, taskID: %s, err: %v, rid: %s",
						task.TaskID, convErr, kt.Rid)
					return nil, 0, convErr
				}
				details = append(details, detail)
			}
			filteredCnt++
		}

		offset += int64(len(pagination.Items))
		if offset >= pagination.Count {
			break
		}
	}

	return details, uint32(filteredCnt), nil
}

// getTaskStatusStatistics 获取任务状态统计信息（五类：INIT / RUNNING / SUCCESS / IGNORED / FAILURE）
// IGNORED 不入库：从 SUCCESS 与存量 FAILURE/TIMEOUT 任务中解析 GsePayload.ErrorCode（828/829），
// 命中的从原状态扣除并计入 IGNORED
func getTaskStatusStatistics(kt *kit.Kit, taskIndex, taskType string) ([]*pbtb.TaskStatusStatItem, error) {
	taskStorage := taskpkg.GetGlobalStorage()
	if taskStorage == nil {
		return nil, errf.Errorf(errf.Unknown, "%s",
			i18n.T(kt, "task storage is not initialized"))
	}

	// 定义需要统计的四种状态及其对应的实际查询状态列表（IGNORED 不入库，不参与计数查询）
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
			TaskType:   taskType,
			StatusList: statusList,
			Limit:      1, // 只需要统计数量，不需要实际数据
			Offset:     0,
		}

		pagination, err := taskStorage.ListTask(kt.Ctx, listOpt)
		if err != nil {
			return nil, errf.Errorf(errf.Unknown, "%s",
				i18n.T(kt, "list tasks from task storage failed for status %s, err: %v", normalizedStatus, err))
		}

		statusCounts[normalizedStatus] = uint32(pagination.Count)
	}

	// 统计 IGNORED：拉取 SUCCESS 与存量 FAILURE/TIMEOUT 任务，解析 GSE 幂等错误码（828/829）
	successIgnored, failureIgnored, err := countIgnoredDisplayStatus(kt, taskStorage, taskIndex, taskType,
		statusCounts[taskTypes.TaskStatusSuccess], statusCounts[taskTypes.TaskStatusFailure])
	if err != nil {
		return nil, err
	}

	// 成功数与失败数扣除命中的 IGNORED
	successCount := statusCounts[taskTypes.TaskStatusSuccess] - successIgnored
	failureCount := statusCounts[taskTypes.TaskStatusFailure] - failureIgnored
	ignoredCount := successIgnored + failureIgnored

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
			Count:   successCount,
			Message: "任务成功",
		},
		{
			Status:  TaskStatusIgnored,
			Count:   ignoredCount,
			Message: "任务已忽略",
		},
		{
			Status:  taskTypes.TaskStatusFailure,
			Count:   failureCount,
			Message: "任务失败",
		},
	}

	return statistics, nil
}

// countIgnoredDisplayStatus 统计任务中 GSE 错误码命中幂等忽略（828/829）的数量，
// 分别返回 SUCCESS 类与 FAILURE 类（含存量 TIMEOUT）中命中的数量，用于从原状态扣除并计入 IGNORED。
func countIgnoredDisplayStatus(kt *kit.Kit, taskStorage istore.Store, taskIndex, taskType string,
	successCount, failureCount uint32) (successIgnored uint32, failureIgnored uint32, err error) {

	countIgnored := func(statusList []string, total uint32) (uint32, error) {
		if total == 0 {
			return 0, nil
		}

		var ignored uint32
		offset := int64(0)
		limit := int64(1000)
		for offset < int64(total) {
			listOpt := &istore.ListOption{
				TaskIndex:  taskIndex,
				TaskType:   taskType,
				StatusList: statusList,
				Offset:     offset,
				Limit:      limit,
			}
			pagination, listErr := taskStorage.ListTask(kt.Ctx, listOpt)
			if listErr != nil {
				return 0, errf.Errorf(errf.Unknown, "%s",
					i18n.T(kt, "list tasks from task storage failed when counting ignored, err: %v", listErr))
			}
			if len(pagination.Items) == 0 {
				break
			}

			for _, t := range pagination.Items {
				var payload commonExecutor.TaskPayload
				if err = t.GetCommonPayload(&payload); err != nil {
					// 单个任务 payload 解析失败不阻断统计，按未命中处理
					logs.Warnf("get common payload failed when counting ignored status, taskID: %s, err: %v, rid: %s",
						t.TaskID, err, kt.Rid)
					continue
				}
				if payload.GsePayload != nil && process.IsIdempotentOperateError(payload.GsePayload.ErrorCode) {
					ignored++
				}
			}

			offset += int64(len(pagination.Items))
		}
		return ignored, nil
	}

	successIgnored, err = countIgnored([]string{taskTypes.TaskStatusSuccess}, successCount)
	if err != nil {
		return 0, 0, err
	}

	failureIgnored, err = countIgnored([]string{taskTypes.TaskStatusFailure, taskTypes.TaskStatusTimeout}, failureCount)
	if err != nil {
		return 0, 0, err
	}

	return successIgnored, failureIgnored, nil
}

// countIgnoredFailedTasks 统计该批次失败任务中 GSE 错误码命中忽略集合的数量
func countIgnoredFailedTasks(kt *kit.Kit, taskStorage istore.Store, taskIndex, taskType string,
	failureCount uint32, ignoreSet map[int32]bool) (uint32, error) {
	listOpt := &istore.ListOption{
		TaskIndex:  taskIndex,
		TaskType:   taskType,
		StatusList: []string{taskTypes.TaskStatusFailure, taskTypes.TaskStatusTimeout},
		Limit:      int64(failureCount),
		Offset:     0,
	}

	pagination, err := taskStorage.ListTask(kt.Ctx, listOpt)
	if err != nil {
		return 0, errf.Errorf(errf.Unknown, "%s",
			i18n.T(kt, "list failed tasks from task storage failed, err: %v", err))
	}

	var ignored uint32
	for _, task := range pagination.Items {
		var payload commonExecutor.TaskPayload
		if err = task.GetCommonPayload(&payload); err != nil {
			// 单个任务 payload 解析失败不阻断统计，按未命中处理
			logs.Warnf("get common payload failed when counting ignored tasks, taskID: %s, err: %v, rid: %s",
				task.TaskID, err, kt.Rid)
			continue
		}
		if isIgnorableFailedTask(&payload, ignoreSet) {
			ignored++
		}
	}

	return ignored, nil
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
	case TaskStatusIgnored:
		// IGNORED 不入库：新任务幂等成功（828/829）落在 SUCCESS，
		// 存量失败任务（ValidateOperate / 旧 Compare 步骤记录）落在 FAILURE/TIMEOUT，
		// 由 convertTaskToDetail 按 GsePayload.ErrorCode 转译过滤展示
		return []string{
			taskTypes.TaskStatusSuccess,
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
func parseTimeIfNotEmpty(kt *kit.Kit, timeStr, fieldName string) (*time.Time, error) {
	if timeStr == "" {
		return nil, nil
	}
	t, err := parseTime(timeStr)
	if err != nil {
		return nil, errf.Errorf(errf.InvalidParameter, "%s",
			i18n.T(kt, "invalid %s format: %v", fieldName, err))
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

// RetryTasks implements pbds.DataServer.
func (s *Service) RetryTasks(ctx context.Context, req *pbds.RetryTasksReq) (*pbds.RetryTasksResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 获取 task storage
	taskStorage := taskpkg.GetGlobalStorage()
	if taskStorage == nil {
		return nil, errf.Errorf(errf.Unknown, "%s",
			i18n.T(kt, "task storage is not initialized"))
	}

	var err error
	var retryCount uint32

	// 查询任务批次信息
	taskBatch, err := s.dao.TaskBatch().GetByID(kt, req.GetBizId(), req.GetBatchId())
	if err != nil {
		logs.Errorf("get task batch failed, batchID: %d, err: %v, rid: %s", req.GetBatchId(), err, kt.Rid)
		return nil, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "get task batch %d failed, err: %v", req.GetBatchId(), err))
	}

	if taskBatch == nil {
		return nil, errf.Errorf(errf.RecordNotFound, "%s",
			i18n.T(kt, "task batch %d does not exist", req.GetBatchId()))
	}

	switch taskBatch.Spec.TaskAction {
	case table.TaskActionConfigCheck:
		// 处理配置检查任务
		retryCount, err = s.retryCheckConfigTask(kt, taskStorage, taskBatch)
	case table.TaskActionConfigGenerate:
		// 处理配置生成任务
		err = s.retry(kt, taskStorage, taskBatch, "")
	case table.TaskActionConfigPublish:
		// 处理配置下发任务
		retryCount, err = s.retryPushConfigTask(kt, taskStorage, taskBatch)
	default:
		// 处理进程操作任务
		retryCount, err = s.retryProcessTask(kt, taskStorage, req.GetBizId(), taskBatch)
	}
	if err != nil {
		return nil, err
	}

	return &pbds.RetryTasksResp{RetryCount: retryCount}, nil
}

func (s *Service) retryProcessTask(kt *kit.Kit, taskStorage istore.Store, bizID uint32, taskBatch *table.TaskBatch) (uint32, error) {

	// 如果任务批次状态为成功，则拒绝重试
	if taskBatch.Spec.Status == table.TaskBatchStatusSucceed {
		logs.Infof("task batch %d is already succeed, skip retry, rid: %s", taskBatch.ID, kt.Rid)
		return 0, nil
	}

	// 查询该批次所有失败的任务（含超时与被级联阻断的任务）
	failedTasks, err := queryFailedTasks(kt, taskStorage, taskBatch.ID, string(taskBatch.Spec.TaskAction))
	if err != nil {
		logs.Errorf("query failed tasks failed, batchID: %d, err: %v, rid: %s", taskBatch.ID, err, kt.Rid)
		return 0, err
	}

	if len(failedTasks) == 0 {
		logs.Infof("no failed tasks to retry, batchID: %d, rid: %s", taskBatch.ID, kt.Rid)
		return 0, nil
	}

	// 先解析全部失败任务对应的实例（此时实例携带的还是操作前的真实状态）
	type retryItem struct {
		task   *taskTypes.Task
		inst   *table.ProcessInstance
		opType table.ProcessOperateType
	}
	items := make([]retryItem, 0, len(failedTasks))
	for _, failedTask := range failedTasks {
		inst, opType, skip := s.resolveRetryTask(kt, bizID, failedTask)
		if skip {
			logs.Warnf("skip retry task %s because process instance is gone, batchID: %d, rid: %s",
				failedTask.TaskID, taskBatch.ID, kt.Rid)
			continue
		}
		items = append(items, retryItem{task: failedTask, inst: inst, opType: opType})
	}

	if len(items) == 0 {
		logs.Infof("no valid failed tasks to retry, batchID: %d, rid: %s", taskBatch.ID, kt.Rid)
		return 0, nil
	}

	// 批量拉取 CMDB 最新配置快照，重试前刷新失败任务的 LatestConfigData（避免配置已变更仍使用旧快照）；
	// 不管进程是否已在 CMDB 删除都重跑任务，快照缺失由任务执行侧处理：
	// 停止操作回退 DB 配置强停，其余操作报错
	ccProcessIDs := make([]uint32, 0, len(items))
	seen := make(map[uint32]struct{}, len(items))
	for _, item := range items {
		if item.inst == nil {
			continue
		}
		id := item.inst.Attachment.CcProcessID
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ccProcessIDs = append(ccProcessIDs, id)
	}
	configSnapshot := refreshConfigDataSnapshot(kt, s.cmdb, bizID, ccProcessIDs)

	retryTasks := make([]*taskTypes.Task, 0, len(items))
	for _, item := range items {
		if item.inst != nil {
			// 必须在置中间态之前刷新 payload，此时 inst 携带的还是实例操作前的真实状态
			if err = s.refreshRetryTaskPayload(kt, item.task, item.inst, configSnapshot); err != nil {
				return 0, err
			}
			if err = updateProcessInstanceStatus(kt, s.dao, item.opType, item.inst, true); err != nil {
				return 0, err
			}
		}
		retryTasks = append(retryTasks, item.task)
	}

	if len(retryTasks) == 0 {
		logs.Infof("no valid failed tasks to retry, batchID: %d, rid: %s", taskBatch.ID, kt.Rid)
		return 0, nil
	}

	retryCount := uint32(len(retryTasks))
	if err = s.dao.TaskBatch().ResetCountsForRetry(kt, taskBatch.ID, retryCount); err != nil {
		logs.Errorf("reset counts for retry failed, batchID: %d, err: %v, rid: %s", taskBatch.ID, err, kt.Rid)
		return 0, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "reset task batch counts for retry failed, batchID: %d, err: %v", taskBatch.ID, err))
	}

	// 任务组按原有阶段顺序重试：只有未成功的任务会被重新下发，
	// 已成功的任务保持不变，阶段之间仍然串行且失败继续阻断后续。
	groupID, err := taskGroupIDOf(taskBatch)
	if err != nil {
		return 0, err
	}
	if err = s.taskManager.RetryGroup(kt.Ctx, groupID); err != nil {
		logs.Errorf("retry task group failed, batchID: %d, groupID: %s, err: %v, rid: %s",
			taskBatch.ID, groupID, err, kt.Rid)
		return 0, errf.Errorf(errf.Unknown, "%s",
			i18n.T(kt, "retry task group %s failed, err: %v", groupID, err))
	}

	return retryCount, nil
}

// refreshRetryTaskPayload 用实例当前状态改写任务各步骤 payload 里的原始状态字段，
// 并同步刷新 CommonPayload 中的 CMDB 最新配置快照（LatestConfigData；ConfigData 保留 DB source_data）。
//
// payload 里的原始状态是首次下发时的快照，仅用于失败回滚与前置校验。任务失败后回滚、
// CMDB/GSE 状态同步、其他批次操作都会让实例真实状态与该快照脱节，此时沿用旧值会让
// 前置校验按过期状态判定操作非法（例如实例已回滚为运行中，快照仍是已停止，重试停止
// 会被判为「已停止无需再停」而永久失败）。
//
// configSnapshot 为 nil（CMDB 查询失败或无可查 ID，刷新降级）时保留原 LatestConfigData；
// 刷新成功但未命中说明进程已在 CMDB 删除，清空原 LatestConfigData，由执行侧决定放行
// （停止操作回退 DB 配置）或报错。
//
// 必须在 updateProcessInstanceStatus 之前调用，否则读到的是操作中间态。
func (s *Service) refreshRetryTaskPayload(kt *kit.Kit, task *taskTypes.Task,
	inst *table.ProcessInstance, configSnapshot map[uint32]string) error {

	refreshed := false
	// 更新托管任务的步骤 payload 是 UpdateRegisterPayload，若按 OperatePayload 解析回写
	// 会丢失 OperateType / EnableProcessRestart / CCSyncStatus 等字段，需按任务类型区分
	isUpdateRegister := false
	if _, ok := task.GetStep(process.OperationCompletedStepName.String()); ok {
		isUpdateRegister = true
	}

	for _, step := range task.Steps {
		if isUpdateRegister {
			payload := &process.UpdateRegisterPayload{}
			if err := step.GetPayload(payload); err != nil {
				logs.Warnf("get step %s payload failed, taskID: %s, err: %v, rid: %s",
					step.GetName(), task.TaskID, err, kt.Rid)
				continue
			}
			if payload.ProcessInstanceID != inst.ID {
				continue
			}
			if payload.OriginalProcStatus == inst.Spec.Status &&
				payload.OriginalProcManagedStatus == inst.Spec.ManagedStatus {
				continue
			}
			payload.OriginalProcStatus = inst.Spec.Status
			payload.OriginalProcManagedStatus = inst.Spec.ManagedStatus
			if err := step.SetPayload(payload); err != nil {
				logs.Errorf("set step %s payload failed, taskID: %s, err: %v, rid: %s",
					step.GetName(), task.TaskID, err, kt.Rid)
				return errf.Errorf(errf.Internal, "%s", i18n.T(kt,
					"refresh retry task %s payload failed, err: %v", task.TaskID, err))
			}
			refreshed = true
			continue
		}

		payload := &process.OperatePayload{}
		if err := step.GetPayload(payload); err != nil {
			logs.Warnf("get step %s payload failed, taskID: %s, err: %v, rid: %s",
				step.GetName(), task.TaskID, err, kt.Rid)
			continue
		}
		if payload.ProcessInstanceID != inst.ID {
			continue
		}
		if payload.OriginalProcStatus == inst.Spec.Status &&
			payload.OriginalProcManagedStatus == inst.Spec.ManagedStatus {
			continue
		}

		payload.OriginalProcStatus = inst.Spec.Status
		payload.OriginalProcManagedStatus = inst.Spec.ManagedStatus
		if err := step.SetPayload(payload); err != nil {
			logs.Errorf("set step %s payload failed, taskID: %s, err: %v, rid: %s",
				step.GetName(), task.TaskID, err, kt.Rid)
			return errf.Errorf(errf.Internal, "%s", i18n.T(kt,
				"refresh retry task %s payload failed, err: %v", task.TaskID, err))
		}
		refreshed = true
	}

	// 刷新 CMDB 最新配置快照；刷新成功但未命中说明进程已在 CMDB 删除，需清空旧快照
	commonPayload := &commonExecutor.TaskPayload{}
	if err := task.GetCommonPayload(commonPayload); err != nil {
		logs.Warnf("get common payload failed when refreshing retry task, taskID: %s, err: %v, rid: %s",
			task.TaskID, err, kt.Rid)
	} else if commonPayload.ProcessPayload != nil {
		latest := retryLatestConfigData(configSnapshot, commonPayload.ProcessPayload.CcProcessID,
			commonPayload.ProcessPayload.LatestConfigData)
		if latest != commonPayload.ProcessPayload.LatestConfigData {
			commonPayload.ProcessPayload.LatestConfigData = latest
			if err := task.SetCommonPayload(commonPayload); err != nil {
				logs.Errorf("set common payload failed, taskID: %s, err: %v, rid: %s",
					task.TaskID, err, kt.Rid)
				return errf.Errorf(errf.Internal, "%s", i18n.T(kt,
					"refresh retry task %s payload failed, err: %v", task.TaskID, err))
			}
			refreshed = true
		}
	}

	if !refreshed {
		return nil
	}

	if err := s.taskManager.UpdateTask(kt.Ctx, task); err != nil {
		logs.Errorf("update retry task payload failed, taskID: %s, err: %v, rid: %s",
			task.TaskID, err, kt.Rid)
		return errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kt,
			"update retry task %s payload failed, err: %v", task.TaskID, err))
	}
	return nil
}

// retryLatestConfigData 依据重试前刷新的 CMDB 快照计算任务应携带的 LatestConfigData：
//   - 快照命中：采用 CMDB 最新配置；
//   - 刷新成功但未命中：进程已在 CMDB 删除，返回空串清空旧快照，避免执行侧把过期快照
//     当作最新配置，对已删除进程误执行 start/register/restart；
//   - 快照为 nil（CMDB 查询失败或无可查 ID，刷新降级）：保留原值。
func retryLatestConfigData(configSnapshot map[uint32]string, ccProcessID uint32, original string) string {
	if configSnapshot == nil {
		return original
	}
	return configSnapshot[ccProcessID]
}

// taskGroupIDOf 取出批次关联的任务组 ID
func taskGroupIDOf(taskBatch *table.TaskBatch) (string, error) {
	extra, err := taskBatch.Spec.GetExtraData()
	if err != nil {
		return "", errf.Errorf(errf.DBOpFailed, "get task batch %d extra data failed, err: %v", taskBatch.ID, err)
	}
	if extra.GroupID == "" {
		return "", errf.Errorf(errf.InvalidParameter,
			"task batch %d has no task group, retry is not supported", taskBatch.ID)
	}
	return extra.GroupID, nil
}

// resolveRetryTask 解析失败任务对应的进程实例与操作类型，用于重试前重置实例状态。
// 实例已删除时 skip=true，该任务不再重试。
func (s *Service) resolveRetryTask(kt *kit.Kit, bizID uint32, failedTask *taskTypes.Task) (
	*table.ProcessInstance, table.ProcessOperateType, bool) {

	if finalizeStep, ok := failedTask.GetStep(process.FinalizeOperateProcessStepName.String()); ok {
		var processPayload process.OperatePayload
		if err := finalizeStep.GetPayload(&processPayload); err != nil {
			logs.Warnf("get operate payload failed, taskID: %s, err: %v, rid: %s", failedTask.TaskID, err, kt.Rid)
			return nil, "", false
		}

		processInstance, err := s.dao.ProcessInstance().GetByID(kt, bizID, processPayload.ProcessInstanceID)
		if err != nil || processInstance == nil {
			return nil, processPayload.OperateType, true
		}
		return processInstance, processPayload.OperateType, false
	}

	// 更新托管任务没有 Finalize 步骤，从 OperationCompleted 步骤解析实例与操作类型
	operateCompletedStep, ok := failedTask.GetStep(process.OperationCompletedStepName.String())
	if !ok {
		return nil, "", false
	}
	var registerPayload process.UpdateRegisterPayload
	if err := operateCompletedStep.GetPayload(&registerPayload); err != nil {
		logs.Warnf("get update register payload failed, taskID: %s, err: %v, rid: %s",
			failedTask.TaskID, err, kt.Rid)
		return nil, "", false
	}

	processInstance, err := s.dao.ProcessInstance().GetByID(kt, bizID, registerPayload.ProcessInstanceID)
	if err != nil || processInstance == nil {
		return nil, table.UpdateRegisterProcessOperate, true
	}
	return processInstance, table.UpdateRegisterProcessOperate, false
}

// retryPushConfigTask 重试下发
func (s *Service) retryPushConfigTask(kt *kit.Kit, taskStorage istore.Store, taskBatch *table.TaskBatch) (uint32, error) {
	// 如果任务批次状态为成功，则拒绝重试
	if taskBatch.Spec.Status == table.TaskBatchStatusSucceed {
		logs.Infof("task batch %d is already succeed, skip retry, rid: %s", taskBatch.ID, kt.Rid)
		return 0, nil
	}

	// 查询该批次所有失败的任务
	failedTasks, err := queryGenerateConfigFailedTasks(kt.Ctx, taskStorage, taskBatch.ID, "",
		string(table.TaskActionConfigPublish))
	if err != nil {
		logs.Errorf("query failed tasks failed, batchID: %d, err: %v, rid: %s", taskBatch.ID, err, kt.Rid)
		return 0, err
	}

	if len(failedTasks) == 0 {
		logs.Infof("no failed tasks to retry, batchID: %d, rid: %s", taskBatch.ID, kt.Rid)
		return 0, nil
	}

	// 重置计数字段用于重试
	retryCount := uint32(len(failedTasks))
	if err = s.dao.TaskBatch().ResetCountsForRetry(kt, taskBatch.ID, retryCount); err != nil {
		logs.Errorf("reset counts for retry failed, batchID: %d, err: %v, rid: %s", taskBatch.ID, err, kt.Rid)
		return 0, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "reset task batch counts for retry failed, batchID: %d, err: %v", taskBatch.ID, err))
	}

	// 重试每个失败的任务
	for _, failedTask := range failedTasks {
		if err = s.taskManager.RetryAll(failedTask); err != nil {
			logs.Errorf("retry failed task failed, taskID: %s, err: %v, rid: %s", failedTask.TaskID, err, kt.Rid)
			return 0, errf.Errorf(errf.Unknown, "%s",
				i18n.T(kt, "retry failed task %s failed, err: %v", failedTask.TaskID, err))
		}
	}

	return retryCount, nil
}

// retryCheckConfigTask 重试检查
func (s *Service) retryCheckConfigTask(kt *kit.Kit, taskStorage istore.Store, taskBatch *table.TaskBatch) (uint32, error) {
	// 如果任务批次状态为成功，则拒绝重试
	if taskBatch.Spec.Status == table.TaskBatchStatusSucceed {
		logs.Infof("task batch %d is already succeed, skip retry, rid: %s", taskBatch.ID, kt.Rid)
		return 0, nil
	}

	// 查询该批次所有失败的任务
	failedTasks, err := queryGenerateConfigFailedTasks(kt.Ctx, taskStorage, taskBatch.ID, "", string(table.TaskActionConfigCheck))
	if err != nil {
		logs.Errorf("query failed tasks failed, batchID: %d, err: %v, rid: %s", taskBatch.ID, err, kt.Rid)
		return 0, err
	}

	if len(failedTasks) == 0 {
		logs.Infof("no failed tasks to retry, batchID: %d, rid: %s", taskBatch.ID, kt.Rid)
		return 0, nil
	}

	// 重置计数字段用于重试
	retryCount := uint32(len(failedTasks))
	if err = s.dao.TaskBatch().ResetCountsForRetry(kt, taskBatch.ID, retryCount); err != nil {
		logs.Errorf("reset counts for retry failed, batchID: %d, err: %v, rid: %s", taskBatch.ID, err, kt.Rid)
		return 0, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "reset task batch counts for retry failed, batchID: %d, err: %v", taskBatch.ID, err))
	}

	// 重试每个失败的任务
	for _, failedTask := range failedTasks {
		if err = s.taskManager.RetryAll(failedTask); err != nil {
			logs.Errorf("retry failed task failed, taskID: %s, err: %v, rid: %s", failedTask.TaskID, err, kt.Rid)
			return 0, errf.Errorf(errf.Unknown, "%s",
				i18n.T(kt, "retry failed task %s failed, err: %v", failedTask.TaskID, err))
		}
	}

	return retryCount, nil
}
