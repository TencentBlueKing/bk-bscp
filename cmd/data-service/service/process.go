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

	taskpkg "github.com/Tencent/bk-bcs/bcs-common/common/task"
	istore "github.com/Tencent/bk-bcs/bcs-common/common/task/stores/iface"
	taskTypes "github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"gorm.io/gen/field"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
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
func (s *Service) OperateProcess(ctx context.Context, req *pbds.OperateProcessReq) (*pbds.OperateProcessResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 校验请求参数
	if err := validateOperateRequest(req); err != nil {
		return nil, err
	}

	// 获取 task storage
	taskStorage := taskpkg.GetGlobalStorage()
	if taskStorage == nil {
		// 获取全局 task storage 失败则不下发任务
		return nil, fmt.Errorf("task storage not initialized, rid: %s", kt.Rid)
	}

	// 获取进程和进程实例
	processes, processInstances, err := getProcessesAndInstances(kt, s.dao, req)
	if err != nil {
		return nil, err
	}

	// 构建操作范围
	operateRange := buildOperateRange(processes)
	environment := processes[0].Spec.Environment

	// 创建任务批次
	batchID, err := createTaskBatch(kt, s.dao, req.OperateType, environment, operateRange)
	if err != nil {
		return nil, err
	}
	logs.Infof("create task batch success, batchID: %d, rid: %s", batchID, kt.Rid)

	// 更新进程实例状态
	// TODO：异步任务失败要回滚进程实例状态
	operateType := table.ProcessOperateType(req.OperateType)
	if err := updateProcessInstances(kt, s.dao, operateType, processInstances); err != nil {
		return nil, err
	}

	// 创建并分发任务
	if err := dispatchProcessTasks(kt, s.dao, s.taskManager,
		req, batchID, operateType, processInstances); err != nil {
		return nil, err
	}

	// 检查任务完成状态并更新TaskBatch
	go monitorTaskBatchStatus(s.dao, taskStorage, batchID)

	return &pbds.OperateProcessResp{BatchID: batchID}, nil
}

// validateOperateRequest 校验操作请求参数
func validateOperateRequest(req *pbds.OperateProcessReq) error {
	// 指定实例时，只能指定一个进程ID
	if len(req.ProcessIds) > 1 && req.InstId != 0 {
		return fmt.Errorf("invalid request: when InstId is specified, only one processId is allowed")
	}

	// todo：验证操作类型是否有效，目前只支持 start、stop、query_status、register、unregister、restart、reload、kill
	return nil
}

// getProcessesAndInstances 获取进程和进程实例
func getProcessesAndInstances(kt *kit.Kit, dao dao.Set, req *pbds.OperateProcessReq) (
	[]*table.Process, []*table.ProcessInstance, error) {
	// 指定实例
	if req.InstId != 0 {
		return getByInstanceID(kt, dao, req.BizId, req.InstId)
	}
	return getByProcessIDs(kt, dao, req.BizId, req.ProcessIds)
}

// getByInstanceID 根据实例ID获取进程和进程实例
func getByInstanceID(kt *kit.Kit, dao dao.Set, bizID, instID uint32) (
	[]*table.Process, []*table.ProcessInstance, error) {
	// 查询指定的进程实例
	inst, err := dao.ProcessInstance().GetByID(kt, bizID, instID)
	if err != nil {
		logs.Errorf("get process instance by id failed, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}
	if inst == nil {
		return nil, nil, fmt.Errorf("process instance not found for id %d", instID)
	}

	// 查询进程信息
	process, err := dao.Process().GetByID(kt, bizID, inst.Attachment.ProcessID)
	if err != nil {
		logs.Errorf("get process failed, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}
	if process == nil {
		return nil, nil, fmt.Errorf("process not found for id %d", inst.Attachment.ProcessID)
	}

	return []*table.Process{process}, []*table.ProcessInstance{inst}, nil
}

// getByProcessIDs 根据进程ID列表获取进程和进程实例
func getByProcessIDs(kt *kit.Kit, dao dao.Set, bizID uint32, processIDs []uint32) (
	[]*table.Process, []*table.ProcessInstance, error) {
	// 查询进程列表
	processes, err := dao.Process().GetByIDs(kt, bizID, processIDs)
	if err != nil {
		logs.Errorf("get processes failed, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}
	if len(processes) == 0 {
		return nil, nil, fmt.Errorf("no processes found for biz %d with provided process IDs", bizID)
	}

	// 查询进程实例列表
	processInstances, err := dao.ProcessInstance().GetByProcessIDs(kt, bizID, processIDs)
	if err != nil {
		logs.Errorf("get process instances failed, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}
	if len(processInstances) == 0 {
		return nil, nil, fmt.Errorf("no process instances found for process IDs %v", processIDs)
	}

	return processes, processInstances, nil
}

// buildOperateRange 从进程列表构建操作范围
func buildOperateRange(processes []*table.Process) table.OperateRange {
	operateRange := table.OperateRange{
		SetNames:     make([]string, 0, len(processes)),
		ModuleNames:  make([]string, 0, len(processes)),
		ServiceNames: make([]string, 0, len(processes)),
		ProcessAlias: make([]string, 0, len(processes)),
		CCProcessID:  make([]uint32, 0, len(processes)),
	}

	for _, process := range processes {
		operateRange.SetNames = append(operateRange.SetNames, process.Spec.SetName)
		operateRange.ModuleNames = append(operateRange.ModuleNames, process.Spec.ModuleName)
		operateRange.ServiceNames = append(operateRange.ServiceNames, process.Spec.ServiceName)
		operateRange.ProcessAlias = append(operateRange.ProcessAlias, process.Spec.Alias)
		operateRange.CCProcessID = append(operateRange.CCProcessID, process.Attachment.CcProcessID)
	}

	return operateRange
}

// createTaskBatch 创建任务批次
func createTaskBatch(kt *kit.Kit, dao dao.Set, operateType string, environment string,
	operateRange table.OperateRange) (uint32, error) {
	now := time.Now()
	taskBatchSpec := &table.TaskBatchSpec{
		TaskObject: table.TaskObjectProcess,
		TaskAction: table.TaskAction(operateType),
		Status:     table.TaskBatchStatusRunning,
		StartAt:    &now,
	}
	taskBatchSpec.SetTaskData(&table.ProcessTaskData{
		Environment:  environment,
		OperateRange: operateRange,
	})

	batchID, err := dao.TaskBatch().Create(kt, &table.TaskBatch{
		Attachment: &table.TaskBatchAttachment{
			BizID: kt.BizID,
		},
		Spec: taskBatchSpec,
		Revision: &table.Revision{
			Creator:   kt.User,
			Reviser:   kt.User,
			CreatedAt: now,
			UpdatedAt: now,
		},
	})
	if err != nil {
		logs.Errorf("create task batch failed, err: %v, rid: %s", err, kt.Rid)
		return 0, err
	}

	return batchID, nil
}

// updateProcessInstances 批量更新进程实例
func updateProcessInstances(kt *kit.Kit, dao dao.Set, operateType table.ProcessOperateType,
	processInstances []*table.ProcessInstance) error {
	if len(processInstances) == 0 {
		return nil
	}

	managedStatus := getProcessManagedStatus(operateType)
	processStatus := getProcessStatus(operateType)

	// 设置状态字段
	for _, inst := range processInstances {
		if managedStatus != "" {
			inst.Spec.ManagedStatus = managedStatus
		}
		if processStatus != "" {
			inst.Spec.Status = processStatus
		}
	}

	if err := dao.ProcessInstance().BatchUpdate(kt, processInstances); err != nil {
		logs.Errorf("batch update process instances failed, count: %d, err: %v, rid: %s",
			len(processInstances), err, kt.Rid)
		return err
	}

	return nil
}

// dispatchProcessTasks 下发进程操作任务
func dispatchProcessTasks(kt *kit.Kit, dao dao.Set, taskManager *task.TaskManager,
	req *pbds.OperateProcessReq, batchID uint32, operateType table.ProcessOperateType,
	processInstances []*table.ProcessInstance) error {
	for _, inst := range processInstances {
		// 确定进程ID
		procID := inst.Attachment.ProcessID
		if len(req.ProcessIds) == 1 {
			procID = req.ProcessIds[0]
		}

		// 创建任务
		taskObj, err := task.NewByTaskBuilder(
			processBuilder.NewOperateTask(
				dao, req.GetBizId(), batchID, procID, inst.ID, operateType, kt.User, true,
			))
		if err != nil {
			logs.Errorf("create process operate task failed, err: %v, rid: %s", err, kt.Rid)
			return err
		}
		// 下发任务
		logs.Infof("dispatch process operate task, taskObj: %+v", taskObj)
		taskManager.Dispatch(taskObj)
	}

	return nil
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

// monitorTaskBatchStatus 异步监控任务批次状态并更新
func monitorTaskBatchStatus(
	dao dao.Set, taskStorage istore.Store, batchID uint32) {
	kt := kit.New()
	// 定时检查任务状态，直到所有任务完成
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// 设置超时时间为10分钟
	timeout := time.After(10 * time.Minute)

	for {
		select {
		case <-timeout:
			logs.Warnf("monitor task batch %d timeout after 1 hour, rid: %s", batchID, kt.Rid)
			// todo：task_batch 增加超时状态，当任务超时则处理为超时状态
			return

		case <-ticker.C:
			// 查询该批次下所有任务的状态
			const pageSize = 100
			var allTasks []*taskTypes.Task
			offset := int64(0)
			for {
				listOpt := &istore.ListOption{
					TaskIndex: fmt.Sprintf("%d", batchID),
					Limit:     pageSize,
					Offset:    offset,
				}

				pagination, err := taskStorage.ListTask(kt.Ctx, listOpt)
				if err != nil {
					logs.Errorf("list tasks for batch %d failed, offset: %d, err: %v, rid: %s",
						batchID, offset, err, kt.Rid)
					break
				}
				// 没有更多数据，退出循环
				if len(pagination.Items) == 0 {
					break
				}
				allTasks = append(allTasks, pagination.Items...)

				// 如果返回的数量少于 pageSize，说明已经是最后一页
				if len(pagination.Items) < int(pageSize) {
					break
				}
				offset += pageSize
			}

			if len(allTasks) == 0 {
				logs.Warnf("no tasks found for batch %d, rid: %s", batchID, kt.Rid)
				continue
			}

			logs.Infof("found %d tasks for batch %d, rid: %s", len(allTasks), batchID, kt.Rid)

			// 检查是否所有任务都已完成
			if !allTasksCompleted(allTasks) {
				logs.Infof("batch %d still has running tasks, continue monitoring, rid: %s", batchID, kt.Rid)
				continue
			}

			// 所有任务已完成，更新TaskBatch状态
			if err := updateTaskBatch(kt, batchID, dao, allTasks); err != nil {
				logs.Errorf("update task batch %d status failed, err: %v, rid: %s", batchID, err, kt.Rid)
				return
			}

			logs.Infof("successfully updated task batch %d status, monitoring completed, rid: %s",
				batchID, kt.Rid)
			return
		}
	}
}

// allTasksCompleted 检查是否所有任务都已完成
func allTasksCompleted(tasks []*taskTypes.Task) bool {
	if len(tasks) == 0 {
		return false
	}

	for _, task := range tasks {
		// 未完成的状态：init, running, not_started, revoked
		if task.Status == taskTypes.TaskStatusInit ||
			task.Status == taskTypes.TaskStatusRunning ||
			task.Status == taskTypes.TaskStatusNotStarted ||
			task.Status == taskTypes.TaskStatusRevoked {
			return false
		}
	}

	return true
}

// updateTaskBatchStatusByTasks 根据任务状态更新TaskBatch状态
func updateTaskBatch(kt *kit.Kit, batchID uint32,
	dao dao.Set, tasks []*taskTypes.Task) error {

	if len(tasks) == 0 {
		logs.Warnf("no tasks found for batch %d, rid: %s", batchID, kt.Rid)
		return nil
	}

	// 统计任务状态
	var successCount, failureCount int
	totalCount := len(tasks)

	for _, task := range tasks {
		switch task.Status {
		case taskTypes.TaskStatusSuccess:
			successCount++
		case taskTypes.TaskStatusFailure, taskTypes.TaskStatusTimeout:
			failureCount++
		}
	}

	// 所有任务都已完成，根据结果确定批次状态
	var batchStatus table.TaskBatchStatus
	switch {
	case successCount == totalCount:
		// 所有任务都成功
		batchStatus = table.TaskBatchStatusSucceed
	case successCount == 0:
		// 所有任务都失败或超时
		batchStatus = table.TaskBatchStatusFailed
	default:
		// 部分成功，部分失败
		batchStatus = table.TaskBatchStatusPartlyFailed
	}

	// 更新TaskBatch状态
	if err := dao.TaskBatch().UpdateStatus(kt, batchID, batchStatus); err != nil {
		return fmt.Errorf("update task batch status failed: %w", err)
	}

	failedCount := totalCount - successCount
	logs.Infof("updated task batch %d status to %s (success=%d, failed=%d, total=%d), rid: %s",
		batchID, batchStatus, successCount, failedCount, totalCount, kt.Rid)

	return nil
}

// RetryTasks implements pbds.DataServer.
func (s *Service) RetryTasks(ctx context.Context, req *pbds.RetryTasksReq) (*pbds.RetryTasksResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 获取 task storage
	taskStorage := taskpkg.GetGlobalStorage()
	if taskStorage == nil {
		return nil, fmt.Errorf("task storage not initialized, rid: %s", kt.Rid)
	}

	// 查询任务批次信息，获取操作类型和任务数据
	taskBatch, err := s.dao.TaskBatch().GetByID(kt, req.BatchId)
	if err != nil {
		logs.Errorf("get task batch failed, batchID: %d, err: %v, rid: %s", req.BatchId, err, kt.Rid)
		return nil, fmt.Errorf("get task batch failed: %v", err)
	}

	// 查询该批次所有失败的任务
	failedTasks, err := queryFailedTasks(ctx, taskStorage, req.BatchId)
	if err != nil {
		logs.Errorf("query failed tasks failed, batchID: %d, err: %v, rid: %s", req.BatchId, err, kt.Rid)
		return nil, fmt.Errorf("query failed tasks failed: %v", err)
	}

	if len(failedTasks) == 0 {
		logs.Infof("no failed tasks to retry, batchID: %d, rid: %s", req.BatchId, kt.Rid)
		return &pbds.RetryTasksResp{RetryCount: 0}, nil
	}

	// 重试每个失败的任务
	retryCount := uint32(0)
	operateType := table.ProcessOperateType(taskBatch.Spec.TaskAction)
	logs.Infof("operate type: %s, rid: %s", operateType, kt.Rid)
	for _, failedTask := range failedTasks {
		logs.Infof("retry failed task, taskID: %s, rid: %s", failedTask.TaskID, kt.Rid)

		retryCount++
	}

	// todo：重试失败的任务
	// 可行的方案之一：删除失败任务，创建新的任务执行
	// 异步监控任务批次状态
	// go monitorTaskBatchStatus(s.dao, taskStorage, req.BatchId)

	logs.Infof("retry tasks completed, batchID: %d, retry count: %d, rid: %s", req.BatchId, retryCount, kt.Rid)
	return &pbds.RetryTasksResp{RetryCount: retryCount}, nil
}

// queryFailedTasks 查询批次中所有失败的任务
func queryFailedTasks(ctx context.Context, taskStorage istore.Store, batchID uint32) ([]*taskTypes.Task, error) {
	var failedTasks []*taskTypes.Task

	offset := int64(0)
	limit := int64(1000)

	for {
		listOpt := &istore.ListOption{
			TaskIndex: fmt.Sprintf("%d", batchID),
			Status:    taskTypes.TaskStatusFailure,
			Offset:    offset,
			Limit:     limit,
		}

		pagination, err := taskStorage.ListTask(ctx, listOpt)
		if err != nil {
			return nil, fmt.Errorf("list tasks failed: %v", err)
		}

		// 如果没有更多任务，退出循环
		if len(pagination.Items) < int(limit) {
			break
		}

		offset += limit
	}

	return failedTasks, nil
}
