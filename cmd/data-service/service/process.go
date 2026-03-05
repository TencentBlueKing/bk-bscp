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
	"sort"
	"strconv"
	"time"

	istore "github.com/Tencent/bk-bcs/bcs-common/common/task/stores/iface"
	taskTypes "github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"gorm.io/gen/field"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/task"
	processBuilder "github.com/TencentBlueKing/bk-bscp/internal/task/builder/process"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbct "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/config-template"
	pbproc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
	pbtb "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/task_batch"
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
	ccProcessIDs := map[uint32]uint32{}
	ccTemplateProcessIDs := map[uint32]uint32{}
	for _, v := range res {
		processIDs = append(processIDs, v.ID)
		ccProcessIDs[v.ID] = v.Attachment.CcProcessID
		ccTemplateProcessIDs[v.ID] = v.Attachment.ProcessTemplateID
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

	// 查询实例进程关联的模板ID
	bindTemplateIds := map[uint32][]uint32{}
	for k, v := range ccProcessIDs {
		templateIDs, errP := s.dao.ConfigTemplate().ListByCCProcessID(kt, req.GetBizId(), v)
		if errP != nil {
			return nil, errP
		}
		bindTemplateIds[k] = append(bindTemplateIds[k], templateIDs...)
	}
	// 查询模板进程关联的模板ID
	for k, v := range ccTemplateProcessIDs {
		templateIDs, errT := s.dao.ConfigTemplate().ListByCCTemplateProcessID(kt, req.GetBizId(), v)
		if errT != nil {
			return nil, errT
		}
		bindTemplateIds[k] = append(bindTemplateIds[k], templateIDs...)
	}

	for k, ids := range bindTemplateIds {
		bindTemplateIds[k] = uniqueUint32(ids)
	}

	processes := pbproc.PbProcessesWithInstances(res, procInstMap, bindTemplateIds)

	filterOptions, err := s.buildfilterOptions(kt, req.GetBizId())
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf(pbproc.CmdbProcessConfigURL, cc.G().CMDB.WebHost, req.GetBizId())

	return &pbds.ListProcessResp{
		Count:                uint32(count),
		Process:              processes,
		FilterOptions:        filterOptions,
		CmdbProcessConfigUrl: url,
	}, nil
}

func uniqueUint32(arr []uint32) []uint32 {
	m := make(map[uint32]struct{})
	res := make([]uint32, 0, len(arr))
	for _, v := range arr {
		if _, ok := m[v]; !ok {
			m[v] = struct{}{}
			res = append(res, v)
		}
	}
	return res
}

func (s *Service) buildfilterOptions(kt *kit.Kit, bizID uint32) (*pbproc.FilterOptions, error) {

	ips, err := s.dao.Process().ListBizFilterOptions(kt, bizID, field.NewString("", "inner_ip"))
	if err != nil {
		return nil, err
	}

	// Inner IP 选项
	ipsOptions := make([]*pbtb.Choice, 0, len(ips))
	for _, v := range ips {
		ipsOptions = append(ipsOptions, &pbtb.Choice{
			Id:   v.Spec.InnerIP,
			Name: v.Spec.InnerIP,
		})
	}

	makeChoices := func(values map[string]string) []*pbtb.Choice {
		choices := make([]*pbtb.Choice, 0, len(values))
		for k, v := range values {
			choices = append(choices, &pbtb.Choice{
				Id:   k,
				Name: v,
			})
		}
		return choices
	}

	// Process Status 选项
	processStatusValues := map[string]string{
		table.ProcessStatusRunning.String():       "运行中",
		table.ProcessStatusPartlyRunning.String(): "部分运行",
		table.ProcessStatusStarting.String():      "启动中",
		table.ProcessStatusRestarting.String():    "重启中",
		table.ProcessStatusStopping.String():      "停止中",
		table.ProcessStatusReloading.String():     "重载中",
		table.ProcessStatusStopped.String():       "已停止",
	}
	psOptions := makeChoices(processStatusValues)

	// Managed Status 选项
	managedStatusValues := map[string]string{
		table.ProcessManagedStatusStarting.String():      "启动中",
		table.ProcessManagedStatusStopping.String():      "停止中",
		table.ProcessManagedStatusManaged.String():       "已托管",
		table.ProcessManagedStatusUnmanaged.String():     "未托管",
		table.ProcessManagedStatusPartlyManaged.String(): "部分托管",
	}
	msOptions := makeChoices(managedStatusValues)

	// CC Sync Status 选项
	ccSyncStatusValues := map[string]string{
		table.Synced.String():  "已同步",
		table.Deleted.String(): "已删除",
		table.Updated.String(): "已更新",
	}
	ccSyncOptions := makeChoices(ccSyncStatusValues)

	filterOptions := &pbproc.FilterOptions{
		InnerIps:        ipsOptions,
		ProcessStatuses: psOptions,
		ManagedStatuses: msOptions,
		CcSyncStatuses:  ccSyncOptions,
	}
	return filterOptions, nil
}

func isStartSemantic(operateType string) bool {
	switch table.TaskAction(operateType) {
	case table.TaskActionRegister,
		table.TaskActionStart,
		table.TaskActionRestart,
		table.TaskActionReload:
		return true
	default:
		return false
	}
}

// OperateProcess implements pbds.DataServer.
func (s *Service) OperateProcess(ctx context.Context, req *pbds.OperateProcessReq) (*pbds.OperateProcessResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 校验请求参数
	if err := validateOperateRequest(req); err != nil {
		return nil, err
	}

	// 获取进程和进程实例
	processes, processInstances, err := getProcessesAndInstances(kt, s.dao, req)
	if err != nil {
		return nil, err
	}

	// 启动语义下，过滤缩容实例
	if isStartSemantic(req.GetOperateType()) {
		processes, processInstances = filterInstancesForStart(
			processes,
			processInstances,
		)
	}

	// 构建 processMap，用于后续快速查找进程信息
	processMap := make(map[uint32]*table.Process, len(processes))
	for _, p := range processes {
		processMap[p.ID] = p
	}

	// 预处理，提前分类出需要下发任务的实例和需要直接删除的实例
	toDispatch, toDelete, err := preResolveInstances(processInstances, processMap, table.ProcessOperateType(req.OperateType))
	if err != nil {
		return nil, err
	}

	// 先执行直接删除（不需要任务批次）
	if errB := batchDeleteProcessInstances(kt, s.dao, toDelete); errB != nil {
		return nil, errB
	}
	logs.Infof("direct delete %d process instances, rid: %s", len(toDelete), kt.Rid)
	// 没有需要下发的任务，直接返回
	if len(toDispatch) == 0 {
		return &pbds.OperateProcessResp{}, nil
	}

	// 构建操作范围，totalCount 只计入真正需要下发任务的实例
	totalCount := uint32(len(toDispatch))
	operateRange := buildOperateRange(processes, req)
	environment := processes[0].Spec.Environment
	if req.OperateRange != nil {
		environment = req.OperateRange.GetEnvironment()
	}

	// 创建任务批次
	batchID, err := createTaskBatch(kt, s.dao, req.OperateType, environment, operateRange, totalCount)
	if err != nil {
		return nil, err
	}
	logs.Infof("create task batch success, batchID: %d, totalCount: %d, rid: %s", batchID, totalCount, kt.Rid)

	// 记录实际下发的任务数
	var dispatchedCount uint32

	// 如果任务创建过程出错，需要处理部分创建的情况
	defer func() {
		if dispatchedCount == totalCount {
			// 所有任务都已创建，由 Callback 机制处理状态更新
			return
		}

		// 计算未创建的任务数
		failedToCreate := totalCount - dispatchedCount
		logs.Warnf("task batch %d partially created: %d/%d tasks dispatched, %d failed to create, rid: %s",
			batchID, dispatchedCount, totalCount, failedToCreate, kt.Rid)

		// 将未创建的任务直接计为失败
		if updateErr := s.dao.TaskBatch().AddFailedCount(kt, batchID, failedToCreate); updateErr != nil {
			logs.Errorf("add failed count for batch %d error, err: %v, rid: %s", batchID, updateErr, kt.Rid)
		}
	}()

	// 下发任务
	dispatchedCount, err = dispatchProcessTasks(
		kt,
		s.dao,
		s.taskManager,
		kt.BizID,
		batchID,
		req.OperateType,
		toDispatch,
		req.GetEnableProcessRestart(),
	)
	if err != nil {
		return nil, err
	}

	return &pbds.OperateProcessResp{BatchID: batchID}, nil
}

// validateOperateRequest 校验操作请求参数
func validateOperateRequest(req *pbds.OperateProcessReq) error {
	// 当指定了多个 processId 时，禁止指定 processInstanceId，因为一个实例只能对应一个进程，且无法支持多个进程的实例级操作
	if len(req.ProcessIds) > 1 && len(req.ProcessInstanceIds) > 0 {
		return fmt.Errorf("invalid request: when processInstanceId is specified, only one processId is allowed")
	}

	// 验证操作类型是否有效，目前只支持 start、stop、register、unregister、restart、reload、kill、update_register、delete
	// delete 操作是用于取消托管或者停止多个实例
	if err := table.ValidateOperateType(table.ProcessOperateType(req.OperateType)); err != nil {
		return fmt.Errorf("invalid request: operate type is not supported: %w", err)
	}
	// query_status 操作仅用于服务端查询，不作为客户端操作类型
	if req.OperateType == string(table.QueryStatusProcessOperate) {
		return fmt.Errorf("query_status operation is not supported")
	}
	return nil
}

// getProcessesAndInstances 获取进程和进程实例
func getProcessesAndInstances(kt *kit.Kit, dao dao.Set, req *pbds.OperateProcessReq) (
	[]*table.Process, []*table.ProcessInstance, error) {
	// 指定实例
	if len(req.ProcessInstanceIds) != 0 {
		return getByProcessInstanceIDs(kt, dao, req.BizId, req.ProcessInstanceIds)
	}
	// 根据操作范围获取进程和进程实例（适配进程配置管理插件）
	if req.OperateRange != nil {
		return getByOperateRanges(kt, dao, req.BizId, req.OperateRange)
	}
	return getByProcessIDs(kt, dao, req.BizId, req.ProcessIds)
}

// getByOperateRanges 根据操作范围获取进程和进程实例（适配进程配置管理插件）
// 启动阶段会过滤缩容实例
func getByOperateRanges(kt *kit.Kit, dao dao.Set, bizID uint32, operateRange *pbproc.OperateRange) (
	[]*table.Process, []*table.ProcessInstance, error) {
	// 根据操作范围查询进程列表
	processes, err := dao.Process().GetByOperateRange(
		kt,
		bizID,
		operateRange,
	)
	if err != nil {
		logs.Errorf("get processes by operate range failed, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}

	if len(processes) == 0 {
		return nil, nil, fmt.Errorf("no processes found for biz %d with provided operate range", bizID)
	}

	// 提取进程ID列表
	processIDs := make([]uint32, 0, len(processes))
	for _, process := range processes {
		processIDs = append(processIDs, process.ID)
	}

	// 查询进程实例列表
	processInstances, err := dao.ProcessInstance().GetByProcessIDs(kt, bizID, processIDs)
	if err != nil {
		logs.Errorf("get process instances failed, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}

	if len(processInstances) == 0 {
		return nil, nil, fmt.Errorf("no process instances found for processes matching operate range")
	}

	return processes, processInstances, nil
}

// getByInstanceID 根据实例ID获取进程和进程实例
func getByProcessInstanceIDs(kt *kit.Kit, dao dao.Set, bizID uint32, processInstanceIDs []uint32) (
	[]*table.Process, []*table.ProcessInstance, error) {
	// 查询指定的进程实例
	insts, err := dao.ProcessInstance().GetByIDs(kt, bizID, processInstanceIDs)
	if err != nil {
		logs.Errorf("get process instances by ids failed, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}
	if len(insts) == 0 {
		return nil, nil, fmt.Errorf("process instances not found for ids %v", processInstanceIDs)
	}

	// 查询进程信息
	process, err := dao.Process().GetByID(kt, bizID, insts[0].Attachment.ProcessID)
	if err != nil {
		logs.Errorf("get process failed, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}
	if process == nil {
		return nil, nil, fmt.Errorf("process not found for id %d", insts[0].Attachment.ProcessID)
	}

	return []*table.Process{process}, insts, nil
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

// filterInstancesForStart 用于启动 / 重启 / 批量启动场景的实例过滤。
// 注意：
// 1. 仅用于启动语义
// 2. 会因缩容导致实例和进程数量减少
// 3. 非启动链路严禁调用
func filterInstancesForStart(processes []*table.Process, processInstances []*table.ProcessInstance) (
	[]*table.Process, []*table.ProcessInstance) {

	// 按 processID 分组实例
	procInstMap := make(map[uint32][]*table.ProcessInstance)
	for _, inst := range processInstances {
		procInstMap[inst.Attachment.ProcessID] = append(
			procInstMap[inst.Attachment.ProcessID],
			inst,
		)
	}

	// 启动可用实例
	filteredInstances := make([]*table.ProcessInstance, 0, len(processInstances))
	// 启动涉及的进程（有实例参与启动）
	filteredProcesses := make([]*table.Process, 0, len(processes))

	for _, process := range processes {

		insts := procInstMap[process.ID]
		if len(insts) == 0 {
			// 没有实例的进程，不参与本次启动
			continue
		}

		filteredProcesses = append(filteredProcesses, process)

		// 非缩容：全部保留
		if uint(len(insts)) <= process.Spec.ProcNum {
			filteredInstances = append(filteredInstances, insts...)
			continue
		}

		// 缩容：按 module_inst_seq 升序取前 ProcNum
		sort.Slice(insts, func(i, j int) bool {
			return insts[i].Spec.ModuleInstSeq < insts[j].Spec.ModuleInstSeq
		})

		filteredInstances = append(filteredInstances, insts[:process.Spec.ProcNum]...)
	}

	return filteredProcesses, filteredInstances
}

// buildOperateRange 从进程列表构建操作范围
func buildOperateRange(processes []*table.Process, req *pbds.OperateProcessReq) table.OperateRange {
	operateRange := table.OperateRange{
		SetNames:     make([]string, 0, len(processes)),
		ModuleNames:  make([]string, 0, len(processes)),
		ServiceNames: make([]string, 0, len(processes)),
		ProcessAlias: make([]string, 0, len(processes)),
		CCProcessID:  make([]uint32, 0, len(processes)),
	}

	// 仅插件操作需要构建完整操作范围
	if req.OperateRange != nil {
		if setName := req.OperateRange.GetSetName(); setName != "" {
			operateRange.SetNames = []string{setName}
		}
		if moduleName := req.OperateRange.GetModuleName(); moduleName != "" {
			operateRange.ModuleNames = []string{moduleName}
		}
		if serviceName := req.OperateRange.GetServiceName(); serviceName != "" {
			operateRange.ServiceNames = []string{serviceName}
		}
		if processAlias := req.OperateRange.GetProcessAlias(); processAlias != "" {
			operateRange.ProcessAlias = []string{processAlias}
		}
		if ccProcessID := req.OperateRange.GetCcProcessId(); ccProcessID != 0 {
			operateRange.CCProcessID = []uint32{ccProcessID}
		}
	} else {
		for _, process := range processes {
			operateRange.CCProcessID = append(operateRange.CCProcessID, process.Attachment.CcProcessID)
		}
	}

	return operateRange
}

// createTaskBatch 创建任务批次
func createTaskBatch(kt *kit.Kit, dao dao.Set, operateType string, environment string,
	operateRange table.OperateRange, totalCount uint32) (uint32, error) {
	now := time.Now()
	taskBatchSpec := &table.TaskBatchSpec{
		TaskObject: table.TaskObjectProcess,
		TaskAction: table.TaskAction(operateType),
		Status:     table.TaskBatchStatusRunning,
		StartAt:    &now,
		TotalCount: totalCount, // 设置总任务数，用于 Callback 机制判断批次完成
		ExtraData:  "{}",
	}
	taskBatchSpec.SetTaskData(&table.TaskExecutionData{
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

// updateProcessInstanceStatus 更新进程实例状态
// 根据操作类型和是否启用进程重启来决定最终状态
// operateType: 操作类型
// processInstances: 进程实例对象
// enableProcessRestart: 是否启用进程重启
func updateProcessInstanceStatus(
	kt *kit.Kit,
	dao dao.Set,
	operateType table.ProcessOperateType,
	processInstances *table.ProcessInstance,
	enableProcessRestart bool,
) error {

	processStatus := table.GetProcessStatusByOpType(operateType, processInstances.Spec.Status, enableProcessRestart)
	managedStatus := table.GetProcessManagedStatusByOpType(operateType, processInstances.Spec.ManagedStatus)
	m := dao.GenQuery().ProcessInstance
	if err := dao.ProcessInstance().UpdateSelectedFields(kt, processInstances.Attachment.BizID, map[string]any{
		"managed_status":    managedStatus,
		"status":            processStatus,
		"status_updated_at": time.Now(),
	}, m.ID.Eq(processInstances.ID)); err != nil {
		logs.Errorf("update process instance failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// resolvedInstance 预处理后的实例信息
type resolvedInstance struct {
	instance        *table.ProcessInstance
	finalOpType     table.ProcessOperateType
	proc            *table.Process
	originalStatus  table.ProcessStatus
	originalManaged table.ProcessManagedStatus
}

// preResolveInstances 预先解析每个实例的真实操作类型，并分类
// 返回：需要下发任务的实例列表、需要直接删除的实例列表
func preResolveInstances(processInstances []*table.ProcessInstance, processMap map[uint32]*table.Process,
	operateType table.ProcessOperateType) (toDispatch []resolvedInstance, toDelete []*table.ProcessInstance, err error) {

	for _, inst := range processInstances {
		proc, ok := processMap[inst.Attachment.ProcessID]
		if !ok {
			return nil, nil, fmt.Errorf("process not found in processMap, processID=%d", inst.Attachment.ProcessID)
		}

		originalStatus := inst.Spec.Status
		originalManaged := inst.Spec.ManagedStatus
		finalOpType, err := resolveOperateType(operateType, originalStatus, originalManaged)
		if err != nil {
			return nil, nil, err
		}

		if finalOpType == table.DeleteProcessOperate {
			// 直接删除，无需下发任务
			toDelete = append(toDelete, inst)
		} else {
			toDispatch = append(toDispatch, resolvedInstance{
				instance:        inst,
				finalOpType:     finalOpType,
				proc:            proc,
				originalStatus:  originalStatus,
				originalManaged: originalManaged,
			})
		}
	}
	return toDispatch, toDelete, nil
}

// dispatchProcessTasks 下发进程操作任务，返回实际下发的任务数
// enableProcessRestart 用于判断是否启用进程重启，影响状态更新和任务构建逻辑
func dispatchProcessTasks(kt *kit.Kit, dao dao.Set, taskManager *task.TaskManager, bizID, batchID uint32, taskType string,
	toDispatch []resolvedInstance, enableProcessRestart bool) (uint32, error) {
	var dispatchedCount uint32
	for _, item := range toDispatch {
		// 更新进程实例状态
		if err := updateProcessInstanceStatus(kt, dao, item.finalOpType, item.instance, enableProcessRestart); err != nil {
			logs.Errorf("update process instance status failed, err: %v, rid: %s", err, kt.Rid)
			return dispatchedCount, err
		}

		// 构建任务（finalOpType 已确定，不再是 Delete）
		taskObj, err := buildProcessTask(
			dao,
			bizID,
			batchID,
			item.instance.Attachment.ProcessID,
			item.instance.ID,
			item.finalOpType, // 直接使用预解析的类型
			kt.User,
			taskType, // 任务批次的操作类型保持不变，任务内使用 finalOpType 来区分实际操作
			item.proc.Spec.CcSyncStatus,
			item.originalManaged,
			item.originalStatus,
			enableProcessRestart,
		)
		if err != nil {
			logs.Errorf("create process operate task failed, err: %v, rid: %s", err, kt.Rid)
			return dispatchedCount, err
		}

		logs.Infof("dispatch process operate task, taskObj: %+v", taskObj)
		taskManager.Dispatch(taskObj)
		dispatchedCount++
	}

	return dispatchedCount, nil
}

// resolveOperateType 判断最终的操作类型
// 规则：
// 1. 非 delete 操作，保持不变
// 2. delete 操作，根据进程状态和托管状态决定最终操作类型
func resolveOperateType(operateType table.ProcessOperateType, status table.ProcessStatus,
	managed table.ProcessManagedStatus) (table.ProcessOperateType, error) {

	if operateType != table.DeleteProcessOperate {
		return operateType, nil
	}

	return getDeleteProcessOperateType(status, managed)
}

// buildProcessTask 构建进程操作任务
func buildProcessTask(dao dao.Set, bizID, batchID, procID, instID uint32, operateType table.ProcessOperateType,
	user, taskType string, ccSyncStatus table.CCSyncStatus, originalManaged table.ProcessManagedStatus, originalStatus table.ProcessStatus,
	enableRestart bool) (*taskTypes.Task, error) {

	if operateType == table.UpdateRegisterProcessOperate {
		return task.NewByTaskBuilder(
			processBuilder.NewUpdateRegisterTask(
				dao,
				bizID,
				batchID,
				procID,
				instID,
				user,
				originalManaged,
				originalStatus,
				ccSyncStatus,
				enableRestart,
			),
		)
	}

	return task.NewByTaskBuilder(
		processBuilder.NewOperateTask(
			dao,
			bizID,
			batchID,
			procID,
			instID,
			operateType,
			user,
			needCMDBCompare(ccSyncStatus, operateType),
			originalManaged,
			originalStatus,
			ccSyncStatus,
			taskType,
		),
	)
}

// needCMDBCompare 判断是否需要与 CMDB 进行配置对比
// 规则：
// 1. 未删除的进程，需要进行 CMDB 对比
// 2. 已删除的进程，在 停止 / 强制停止 / 取消托管 操作时，跳过 CMDB 对比
// 3. 已删除进程的其他操作，统一跳过（防御式处理）
func needCMDBCompare(ccSyncStatus table.CCSyncStatus, op table.ProcessOperateType) bool {

	// 1. 未删除的进程：需要对比
	if ccSyncStatus != table.Deleted {
		return true
	}

	// 2. 已删除进程：特定操作跳过
	switch op {
	case table.KillProcessOperate,
		table.StopProcessOperate,
		table.UnregisterProcessOperate:
		return false
	default:
		// 3. 已删除进程的其他操作：防御式跳过
		return false
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

// queryFailedTasks 查询批次中所有失败的任务
func queryFailedTasks(ctx context.Context, taskStorage istore.Store, batchID uint32, taskType string) ([]*taskTypes.Task, error) {
	var failedTasks []*taskTypes.Task

	offset := int64(0)
	limit := int64(1000)

	for {
		listOpt := &istore.ListOption{
			TaskIndex: fmt.Sprintf("%d", batchID),
			TaskType:  taskType,
			Status:    taskTypes.TaskStatusFailure,
			Offset:    offset,
			Limit:     limit,
		}

		pagination, err := taskStorage.ListTask(ctx, listOpt)
		if err != nil {
			return nil, fmt.Errorf("list tasks failed: %v", err)
		}

		// 将查询到的任务添加到结果集
		failedTasks = append(failedTasks, pagination.Items...)

		// 如果没有更多任务，退出循环
		if len(pagination.Items) < int(limit) {
			break
		}

		offset += limit
	}

	return failedTasks, nil
}

// GetProcessInstanceTopo implements [pbds.DataServer].
func (s *Service) GetProcessInstanceTopo(ctx context.Context, req *pbds.GetProcessInstanceTopoReq) (
	*pbds.GetProcessInstanceTopoResp, error) {
	kt := kit.FromGrpcContext(ctx)

	processes, count, err := s.dao.Process().List(kt, req.BizId, nil, &types.BasePage{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("list processes failed: %w", err)
	}

	if count == 0 {
		return &pbds.GetProcessInstanceTopoResp{}, nil
	}

	setMap := make(map[uint32]*pbct.BizTopoNode)

	for _, p := range processes {
		// 1. 没有进程数量，直接跳过（最重要的裁剪点）
		if p.Spec.ProcNum == 0 {
			continue
		}

		// 集群
		setNode := getOrCreateSetNode(setMap, p)

		// 模块
		moduleNode := getOrCreateChild(
			setNode,
			p.Attachment.ModuleID,
			p.Spec.ModuleName,
			constant.BK_MODULE_OBJ_ID,
			"模块",
		)

		// 实例
		instanceNode := getOrCreateChild(
			moduleNode,
			p.Attachment.ServiceInstanceID,
			p.Spec.ServiceName,
			constant.BK_SERVICE_OBJ_ID,
			"实例",
		)

		// 进程
		processNode := &pbct.BizTopoNode{
			BkInstId:   p.Attachment.CcProcessID,
			BkInstName: p.Spec.Alias,
			BkObjId:    constant.BK_PROCESS_OBJ_ID,
			BkObjName:  "进程",
		}

		instanceNode.Child = append(instanceNode.Child, processNode)
	}

	var result []*pbct.BizTopoNode
	for _, setNode := range setMap {
		pruneEmptyNode(setNode)
		if len(setNode.Child) == 0 {
			continue
		}

		// 回填 process_count
		fillProcessCount(setNode)

		result = append(result, setNode)
	}

	return &pbds.GetProcessInstanceTopoResp{
		BizTopoNodes: result,
	}, nil
}

// fillProcessCount 返回：该节点下包含的 process 节点数量
func fillProcessCount(node *pbct.BizTopoNode) uint32 {

	// process 节点：自身计 1
	if node.BkObjId == constant.BK_PROCESS_OBJ_ID {
		node.ProcessCount = 1
		return 1
	}

	var total uint32
	for _, child := range node.Child {
		total += fillProcessCount(child)
	}

	node.ProcessCount = total
	return total
}

func getOrCreateChild(parent *pbct.BizTopoNode, id uint32, name string, objID string,
	objName string) *pbct.BizTopoNode {

	for _, c := range parent.Child {
		if c.BkInstId == id && c.BkObjId == objID {
			return c
		}
	}

	child := &pbct.BizTopoNode{
		BkInstId:   id,
		BkInstName: name,
		BkObjId:    objID,
		BkObjName:  objName,
	}

	parent.Child = append(parent.Child, child)
	return child
}

func getOrCreateSetNode(setMap map[uint32]*pbct.BizTopoNode, p *table.Process) *pbct.BizTopoNode {
	if node, ok := setMap[p.Attachment.SetID]; ok {
		return node
	}

	node := &pbct.BizTopoNode{
		BkInstId:   p.Attachment.SetID,
		BkInstName: p.Spec.SetName,
		BkObjId:    constant.BK_SET_OBJ_ID,
		BkObjName:  "集群",
	}
	setMap[p.Attachment.SetID] = node
	return node
}

func pruneEmptyNode(node *pbct.BizTopoNode) {
	if len(node.Child) == 0 {
		return
	}

	var kept []*pbct.BizTopoNode
	for _, c := range node.Child {
		pruneEmptyNode(c)
		if len(c.Child) > 0 || c.BkObjId == constant.BK_PROCESS_OBJ_ID {
			kept = append(kept, c)
		}
	}
	node.Child = kept
}

// getDeleteProcessOperateType 根据进程状态和托管状态，判断 delete 操作最终需要执行的实际操作类型。
// 规则：
// 1. 如果进程状态是 Running（运行中），只需要执行 Stop 操作。
// 2. 如果进程状态是 Stopped（已停止）：
//   - 托管状态为 Managed（已托管）：执行 Unregister（取消托管）
//   - 托管状态为 Unmanaged（未托管）：执行 Delete（删除）
//
// 3. 其他状态均视为非法操作，返回错误。
func getDeleteProcessOperateType(
	status table.ProcessStatus,
	managedStatus table.ProcessManagedStatus,
) (table.ProcessOperateType, error) {

	switch status {

	// 运行中的进程，只需要停止
	case table.ProcessStatusRunning:
		return table.StopProcessOperate, nil

	// 已停止的进程，根据托管状态决定
	case table.ProcessStatusStopped:
		if managedStatus == table.ProcessManagedStatusManaged {
			return table.UnregisterProcessOperate, nil
		}
		if managedStatus == table.ProcessManagedStatusUnmanaged {
			return table.DeleteProcessOperate, nil
		}
		return "", fmt.Errorf("unsupported managed status: %s", managedStatus)
	// 其他状态全部视为非法
	default:
		return "", fmt.Errorf(
			"unsupported process state for delete: status=%s managedStatus=%s",
			status, managedStatus,
		)
	}
}

// batchDeleteProcessInstances 批量直接删除实例（无需任务）
func batchDeleteProcessInstances(kt *kit.Kit, dao dao.Set, instances []*table.ProcessInstance) error {
	for _, inst := range instances {
		if err := dao.ProcessInstance().Delete(kt, kt.BizID, inst.ID); err != nil {
			logs.Errorf("direct delete process instance %d failed, err: %v, rid: %s", inst.ID, err, kt.Rid)
			return err
		}
	}
	return nil
}
