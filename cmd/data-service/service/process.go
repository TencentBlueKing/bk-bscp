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
	"strconv"
	"time"

	"gorm.io/gen/field"

	"github.com/TencentBlueKing/bk-bscp/internal/task"
	processBuilder "github.com/TencentBlueKing/bk-bscp/internal/task/builder/process"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbproc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// ListProcess implements pbds.DataServer.
func (s *Service) ListProcess(ctx context.Context, req *pbds.ListProcessReq) (*pbds.ListProcessResp, error) {
	kt := kit.FromGrpcContext(ctx)

	res, count, err := s.dao.Process().List(kt, req.BizId, req.GetSearch(), &types.BasePage{
		Start: req.Start,
		Limit: uint(req.Limit),
		All:   req.GetAll(),
	})
	if err != nil {
		return nil, err
	}

	processIDs := make([]uint32, 0, len(res))
	for _, v := range res {
		processIDs = append(processIDs, v.ID)
	}

	procInst, err := s.dao.ProcessInstance().GetByProcessIDs(kt, req.GetBizId(), processIDs)
	if err != nil {
		return nil, err
	}

	// 将 procInst 按 process_id 分组
	procInstMap := make(map[uint32][]*table.ProcessInstance)
	for _, inst := range procInst {
		procInstMap[inst.Attachment.ProcessID] = append(procInstMap[inst.Attachment.ProcessID], inst)
	}

	processes := pbproc.PbProcessesWithInstances(res, procInstMap)

	return &pbds.ListProcessResp{
		Count:   uint32(count),
		Process: processes,
	}, nil
}

// OperateProcess implements pbds.DataServer.
// nolint:funlen
func (s *Service) OperateProcess(ctx context.Context, req *pbds.OperateProcessReq) (*pbds.OperateProcessResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 校验请求：如果指定实例，则进程ID只能有一条
	if len(req.ProcessIds) > 1 && req.InstId != 0 {
		return nil, fmt.Errorf("invalid request: when InstId != 0, only one processId is allowed")
	}

	// 1、查询进程对应的进程实例，进行任务下发
	processes, err := s.dao.Process().GetByIDs(kt, req.BizId, req.ProcessIds)
	if err != nil {
		logs.Errorf("get process failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	if len(processes) == 0 {
		return nil, fmt.Errorf("no process found for biz %d", req.BizId)
	}

	// 2. 查询进程实例
	var processInstances []*table.ProcessInstance
	if req.InstId != 0 {
		// 指定了单个实例
		inst, errI := s.dao.ProcessInstance().GetByID(kt, req.BizId, req.InstId)
		if errI != nil {
			logs.Errorf("get process instance by id failed, err: %v, rid: %s", errI, kt.Rid)
			return nil, errI
		}
		if inst == nil {
			return nil, fmt.Errorf("no process instance found for id %d", req.InstId)
		}
		processInstances = append(processInstances, inst)
	} else {
		// 未指定实例：查询所有进程对应的实例
		processIDs := make([]uint32, 0, len(processes))
		for _, p := range processes {
			processIDs = append(processIDs, p.ID)
		}

		processInstances, err = s.dao.ProcessInstance().GetByProcessIDs(kt, req.BizId, processIDs)
		if err != nil {
			logs.Errorf("get process instances failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		if len(processInstances) == 0 {
			return nil, fmt.Errorf("no process instances found for processes %+v", processIDs)
		}
	}

	// 2、先写入task_batch获取一个batchID，然后写入任务并开启
	now := time.Now()
	taskBatchSpec := &table.TaskBatchSpec{
		TaskObject: table.TaskObjectProcess,
		Status:     table.TaskBatchStatusRunning,
		StartAt:    &now,
	}
	taskBatchSpec.SetTaskData(&table.ProcessTaskData{
		// TODO: 操作的环境
		// Environment:  process.Spec.Environment,
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
		Revision: &table.Revision{
			Creator:   kt.User,
			CreatedAt: now,
			UpdatedAt: now,
		},
	})
	if err != nil {
		logs.Errorf("create task batch failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 4. 计算托管/状态
	managedStatus := getProcessManagedStatus(table.ProcessOperateType(req.OperateType))
	processStatus := getProcessStatus(table.ProcessOperateType(req.OperateType))

	// 5. 遍历进程实例，更新状态并下发任务
	for _, inst := range processInstances {
		if managedStatus != "" {
			inst.Spec.ManagedStatus = managedStatus
		}
		if processStatus != "" {
			inst.Spec.Status = processStatus
		}

		if err := s.dao.ProcessInstance().Update(kt, inst); err != nil {
			logs.Errorf("update process instance failed, id: %d, err: %v, rid: %s", inst.ID, err, kt.Rid)
			return nil, err
		}

		// 找到对应进程（仅用于日志或任务参数）
		var procID uint32
		if len(req.ProcessIds) == 1 {
			procID = req.ProcessIds[0]
		} else {
			procID = inst.Attachment.ProcessID
		}

		// 创建任务
		taskObj, err := task.NewByTaskBuilder(
			processBuilder.NewOperateTask(
				s.dao, req.GetBizId(), batchID, procID, inst.ID,
				table.ProcessOperateType(req.OperateType), kt.User, true,
			))
		if err != nil {
			logs.Errorf("create process operate task failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		// 启动任务
		s.taskManager.Dispatch(taskObj)
	}

	return &pbds.OperateProcessResp{BatchID: batchID}, nil
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

// ProcessFilterOptions implements pbds.DataServer.
func (s *Service) ProcessFilterOptions(ctx context.Context, req *pbds.ProcessFilterOptionsReq) (
	*pbds.ProcessFilterOptionsResp, error) {
	kt := kit.FromGrpcContext(ctx)
	sets, err := s.dao.Process().ListBizFilterOptions(kt, req.GetBizId(),
		field.NewUint32("", "set_id"), field.NewString("", "set_name"))
	if err != nil {
		return nil, err
	}
	setOptions := make([]*pbproc.ProcessFilterOption, 0, len(sets))
	for _, v := range sets {
		setOptions = append(setOptions, &pbproc.ProcessFilterOption{
			Id:   v.Attachment.SetID,
			Name: v.Spec.SetName,
		})
	}

	modules, err := s.dao.Process().ListBizFilterOptions(kt, req.GetBizId(),
		field.NewUint32("", "module_id"), field.NewString("", "module_name"))
	if err != nil {
		return nil, err
	}
	moduleOptions := make([]*pbproc.ProcessFilterOption, 0, len(modules))
	for _, v := range modules {
		moduleOptions = append(moduleOptions, &pbproc.ProcessFilterOption{
			Id:   v.Attachment.ModuleID,
			Name: v.Spec.ModuleName,
		})
	}

	svcInsts, err := s.dao.Process().ListBizFilterOptions(kt, req.GetBizId(),
		field.NewUint32("", "service_instance_id"), field.NewString("", "service_name"))
	if err != nil {
		return nil, err
	}
	svcInstOptions := make([]*pbproc.ProcessFilterOption, 0, len(svcInsts))
	for _, v := range svcInsts {
		svcInstOptions = append(svcInstOptions, &pbproc.ProcessFilterOption{
			Id:   v.Attachment.ServiceInstanceID,
			Name: v.Spec.ServiceName,
		})
	}

	processIds, err := s.dao.Process().ListBizFilterOptions(kt, req.GetBizId(), field.NewUint32("", "cc_process_id"))
	if err != nil {
		return nil, err
	}
	processIDOptions := make([]*pbproc.ProcessFilterOption, 0, len(processIds))
	for _, v := range processIds {
		processIDOptions = append(processIDOptions, &pbproc.ProcessFilterOption{
			Id:   v.Attachment.CcProcessID,
			Name: strconv.Itoa(int(v.Attachment.CcProcessID)),
		})
	}

	aliases, err := s.dao.Process().ListBizFilterOptions(kt, req.GetBizId(), field.NewString("", "alias"))
	if err != nil {
		return nil, err
	}
	processAliasesOptions := make([]*pbproc.ProcessFilterOption, 0, len(aliases))
	for k, v := range aliases {
		processAliasesOptions = append(processAliasesOptions, &pbproc.ProcessFilterOption{
			Id:   uint32(k + 1),
			Name: v.Spec.Alias,
		})
	}

	return &pbds.ProcessFilterOptionsResp{
		Sets:             setOptions,
		Modules:          moduleOptions,
		ServiceInstances: svcInstOptions,
		ProcessAliases:   processAliasesOptions,
		CcProcessIds:     processIDOptions,
	}, nil
}
