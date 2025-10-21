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

	"github.com/TencentBlueKing/bk-bscp/internal/task"
	processBuilder "github.com/TencentBlueKing/bk-bscp/internal/task/builder/process"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbproc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// ListProcess implements pbds.DataServer.
func (s *Service) ListProcess(ctx context.Context, req *pbds.ListProcessReq) (*pbds.ListProcessResp, error) {
	kt := kit.FromGrpcContext(ctx)

	res, count, err := s.dao.Process().List(kt, req.BizId)
	if err != nil {
		return nil, err
	}

	return &pbds.ListProcessResp{
		Count:   uint32(count),
		Process: pbproc.PbProcesses(res),
	}, nil
}

// OperateProcess implements pbds.DataServer.
func (s *Service) OperateProcess(ctx context.Context, req *pbds.OperateProcessReq) (*pbds.OperateProcessResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 1、查询进程对应的进程实例，进行任务下发
	process, err := s.dao.Process().GetByID(kt, req.BizId, req.ProcessId)
	if err != nil {
		logs.Errorf("get process failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	processInstances, err := s.dao.ProcessInstance().GetByID(kt, req.BizId, []uint32{req.ProcessId})
	if err != nil {
		logs.Errorf("get process instance failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	if len(processInstances) == 0 {
		return nil, fmt.Errorf("no process instances found for process id %d", req.ProcessId)
	}

	// 2、先写入task_batch获取一个batchID，然后写入任务并开启
	now := time.Now()
	taskBatchSpec := &table.TaskBatchSpec{
		TaskObject: table.TaskObjectProcess,
		Status:     table.TaskBatchStatusRunning,
		StartAt:    &now,
	}
	taskBatchSpec.SetTaskData(&table.ProcessTaskData{
		Environment:  process.Spec.Environment,
		OperateRange: table.OperateRange{
			// TODO : 增加对应的范围ID
			// SetID:       process.Spec.SetID,
			// ModuleID:    process.Spec.ModuleID,
			// ServiceID:   process.Spec.ServiceID,
			// CCProcessID: process.Spec.CCProcessID,
		},
	})
	batchID, err := s.dao.TaskBatch().Create(kt, &table.TaskBatch{
		Attachment: &table.TaskBatchAttachment{
			BizID: kt.BizID,
		},
		Spec: taskBatchSpec,
	})
	if err != nil {
		logs.Errorf("create task batch failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 如果是托管/取消托管,设置托管状态
	managedStatus := getProcessManagedStatus(table.ProcessOperateType(req.OperateType))
	processStauts := getProcessStatus(table.ProcessOperateType(req.OperateType))
	// 3、写入并开启任务
	for _, processInstance := range processInstances {
		// 更新任务状态为进行中
		if managedStatus != "" {
			processInstance.Spec.ManagedStatus = managedStatus
		}
		if processStauts != "" {
			processInstance.Spec.Status = processStauts
		}

		// 更新状态
		err = s.dao.ProcessInstance().Update(kt, processInstance)
		if err != nil {
			logs.Errorf("update process instance failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		// 写入任务
		processOperateTask, err := task.NewByTaskBuilder(
			processBuilder.NewOperateTask(s.dao, batchID, processInstance.ID, processInstance.ID,
				table.ProcessOperateType(req.OperateType), kt.User, true))
		if err != nil {
			logs.Errorf("create process operate task failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		// 启动任务
		s.taskManager.Dispatch(processOperateTask)
	}

	return &pbds.OperateProcessResp{
		BatchID: batchID,
	}, nil
}

func getProcessManagedStatus(operateType table.ProcessOperateType) table.ProcessManagedStatus {
	switch operateType {
	case table.RegisterProcessOperate:
		return table.ProcessManagedStatusStarting
	case table.UnregisterProcessOperate:
		return table.ProcessManagedStatusStopping
	default:
		return ""
	}
}

func getProcessStatus(operateType table.ProcessOperateType) table.ProcessStatus {
	switch operateType {
	case table.StartProcessOperate:
		return table.ProcessStatusStarting
	case table.StopProcessOperate:
		return table.ProcessStatusStopped
	case table.RestartProcessOperate:
		return table.ProcessStatusRestarting
	case table.ReloadProcessOperate:
		return table.ProcessStatusReloading
	case table.KillProcessOperate:
		return table.ProcessStatusStopping
	default:
		return ""
	}
}
