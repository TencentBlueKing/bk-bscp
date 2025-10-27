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

	// 解析每个 task 的 CommonPayload，构建 TaskDetail 并应用过滤
	taskDetails := make([]*pbtb.TaskDetail, 0, len(pagination.Items))
	for _, task := range pagination.Items {
		detail, err := convertTaskToDetail(task, req)
		if err != nil {
			logs.Errorf("convert task to detail failed, taskID: %s, err: %v", task.TaskID, err)
			return nil, fmt.Errorf("convert task to detail failed: %v", err)
		}
		if detail != nil {
			taskDetails = append(taskDetails, detail)
		}
	}

	return &pbds.GetTaskBatchDetailResp{
		Tasks: taskDetails,
		Count: uint32(len(taskDetails)),
	}, nil
}

// convertTaskToDetail 将 task 转换为 pb 数据结构 TaskDetail，并根据请求参数进行过滤
func convertTaskToDetail(task *taskTypes.Task, req *pbds.GetTaskBatchDetailReq) (*pbtb.TaskDetail, error) {
	if task == nil {
		return nil, fmt.Errorf("task is nil")
	}

	// 解析 CommonPayload 为 ProcessPayload
	var processPayload commonExecutor.ProcessPayload
	err := task.GetCommonPayload(&processPayload)
	if err != nil {
		return nil, fmt.Errorf("get common payload failed: %v", err)
	}

	// 应用 ProcessPayload 字段过滤
	if req.SetName != "" && processPayload.SetName != req.SetName {
		return nil, nil
	}
	if req.ModuleName != "" && processPayload.ModuleName != req.ModuleName {
		return nil, nil
	}
	if req.ServiceName != "" && processPayload.ServiceName != req.ServiceName {
		return nil, nil
	}
	if req.Alias != "" && processPayload.Alias != req.Alias {
		return nil, nil
	}
	if req.CcProcessId != "" && processPayload.CcProcessID != req.CcProcessId {
		return nil, nil
	}
	if req.InnerIp != "" && processPayload.InnerIP != req.InnerIp {
		return nil, nil
	}

	// 构建返回的 TaskDetail
	detail := &pbtb.TaskDetail{
		Status:        task.Status,
		Message:       task.Message,
		Creator:       task.Creator,
		ExecutionTime: task.ExecutionTime,
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
