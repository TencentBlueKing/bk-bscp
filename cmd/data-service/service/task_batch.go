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

	opt := &types.BasePage{
		Start: req.Start,
		Limit: uint(req.Limit),
	}

	filter := &dao.TaskBatchListFilter{
		TaskObject: table.TaskObject(req.TaskObject),
		TaskAction: table.TaskAction(req.TaskAction),
		Status:     table.TaskBatchStatus(req.Status),
		Executor:   req.Executor,
	}
	res, count, err := s.dao.TaskBatch().List(kt, req.BizId, filter, opt)
	if err != nil {
		logs.Errorf("list task batch failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return &pbds.ListTaskBatchResp{
		Count: uint32(count),
		// 转换为 protobuf 格式
		List: pbtb.PbTaskBatches(res),
	}, nil
}

// GetTaskBatchDetail implements pbds.DataServer.
func (s *Service) GetTaskBatchDetail(
	ctx context.Context,
	req *pbds.GetTaskBatchDetailReq,
) (*pbds.GetTaskBatchDetailResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 从 task store 查询所有相关任务（通过 taskIndex = batchID）
	taskStorage := taskpkg.GetGlobalStorage()
	if taskStorage == nil {
		return nil, fmt.Errorf("task storage not initialized")
	}

	listOpt := &istore.ListOption{
		TaskIndex: fmt.Sprintf("%d", req.GetBatchId()),
		Offset:    int64(req.GetStart()),
		Limit:     int64(req.GetLimit()),
		Status:    req.GetStatus(),
	}

	pagination, err := taskStorage.ListTask(ctx, listOpt)
	if err != nil {
		logs.Errorf("list tasks failed, err: %v, rid: %s", err, kt.Rid)
		return nil, fmt.Errorf("list tasks failed: %v", err)
	}

	// 解析每个 task 的 CommonPayload，构建 TaskDetail
	taskDetails := make([]*pbtb.TaskDetail, 0, len(pagination.Items))
	for _, task := range pagination.Items {
		detail, err := convertTaskToDetail(task)
		if err != nil {
			logs.Errorf("convert task to detail failed, taskID: %s, err: %v", task.TaskID, err)
			return nil, fmt.Errorf("convert task to detail failed: %v", err)
		}
		taskDetails = append(taskDetails, detail)
	}

	return &pbds.GetTaskBatchDetailResp{
		Tasks: taskDetails,
		Count: uint32(pagination.Count),
	}, nil
}

// convertTaskToDetail 将 task 转换为 pb 数据结构 TaskDetail
func convertTaskToDetail(task *taskTypes.Task) (*pbtb.TaskDetail, error) {
	if task == nil {
		return nil, fmt.Errorf("task is nil")
	}

	// 解析 CommonPayload 为 ProcessPayload
	var processPayload commonExecutor.ProcessPayload
	err := task.GetCommonPayload(&processPayload)
	if err != nil {
		return nil, fmt.Errorf("get common payload failed: %v", err)
	}

	// 构建返回的 TaskDetail
	detail := &pbtb.TaskDetail{
		Status:        convertTaskStatus(task.Status),
		Message:       task.Message,
		Creator:       task.Creator,
		ExecutionTime: float32(task.ExecutionTime) / 1000.0,
		ProcessPayload: &pbtb.ProcessPayload{
			SetName:     processPayload.SetName,
			ModuleName:  processPayload.ModuleName,
			ServiceName: processPayload.ServiceName,
			Environment: processPayload.Environment,
			Alias:       processPayload.Alias,
			InnerIp:     processPayload.InnerIP,
			AgentId:     processPayload.AgentID,
			CcProcessId: processPayload.CcProcessID,
			LocalInstId: processPayload.LocalInstID,
			InstId:      processPayload.InstID,
			ConfigData:  processPayload.ConfigData,
		},
	}

	return detail, nil
}

// GetTaskStatusStatistics implements pbds.DataServer.
func (s *Service) GetTaskStatusStatistics(
	ctx context.Context,
	req *pbds.GetTaskStatusStatisticsReq,
) (*pbds.GetTaskStatusStatisticsResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 从 task store 查询所有相关任务（通过 taskIndex = batchID）
	taskStorage := taskpkg.GetGlobalStorage()
	if taskStorage == nil {
		return nil, fmt.Errorf("task storage not initialized")
	}

	// 初始化状态计数器
	statusCounts := map[string]uint32{
		taskTypes.TaskStatusInit:    0,
		taskTypes.TaskStatusRunning: 0,
		taskTypes.TaskStatusSuccess: 0,
		taskTypes.TaskStatusFailure: 0,
	}

	// 分页查询所有任务，进行统计
	offset := int64(0)
	limit := int64(1000) // 每次查询1000条

	for {
		listOpt := &istore.ListOption{
			TaskIndex: fmt.Sprintf("%d", req.GetBatchId()),
			Offset:    offset,
			Limit:     limit,
		}

		pagination, err := taskStorage.ListTask(ctx, listOpt)
		if err != nil {
			logs.Errorf("list tasks failed, err: %v, rid: %s", err, kt.Rid)
			return nil, fmt.Errorf("list tasks failed: %v", err)
		}

		// 统计每个任务的状态
		for _, task := range pagination.Items {
			normalizedStatus := convertTaskStatus(task.Status)
			statusCounts[normalizedStatus]++
		}

		if len(pagination.Items) < int(limit) {
			break
		}

		offset += limit
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

	return &pbds.GetTaskStatusStatisticsResp{
		Statistics: statistics,
	}, nil
}

// convertTaskStatus 将任务状态转换为四类：INITIALIZING, RUNNING, SUCCESS, FAILURE
func convertTaskStatus(status string) string {
	switch status {
	case taskTypes.TaskStatusInit:
		return taskTypes.TaskStatusInit
	case taskTypes.TaskStatusRunning:
		return taskTypes.TaskStatusRunning
	case taskTypes.TaskStatusRevoked, taskTypes.TaskStatusNotStarted:
		// revoke和notstarted认为是running
		return taskTypes.TaskStatusRunning
	case taskTypes.TaskStatusSuccess:
		return taskTypes.TaskStatusSuccess
	case taskTypes.TaskStatusFailure, taskTypes.TaskStatusTimeout:
		// 超时认为是失败
		return taskTypes.TaskStatusFailure
	default:
		// 未知状态默认为失败
		return taskTypes.TaskStatusFailure
	}
}
