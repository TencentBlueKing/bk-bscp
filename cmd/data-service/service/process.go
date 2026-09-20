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
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"time"

	istore "github.com/Tencent/bk-bcs/bcs-common/common/task/stores/iface"
	taskTypes "github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"gorm.io/gen/field"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/expression"
	"github.com/TencentBlueKing/bk-bscp/internal/task"
	processBuilder "github.com/TencentBlueKing/bk-bscp/internal/task/builder/process"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	processExecutor "github.com/TencentBlueKing/bk-bscp/internal/task/executor/process"
	"github.com/TencentBlueKing/bk-bscp/internal/task/priority"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
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
		// 表达式非法等入参错误由 DAO 归类为 InvalidParameter，此处透传保留其错误码。
		if ef, ok := err.(*errf.ErrorF); ok {
			return nil, ef
		}
		return nil, errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kt, "list processes failed, err: %v", err))
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
		return nil, errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kt, "get process instances by process IDs failed, err: %v", err))
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
			return nil, errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kt, "list config templates by CC process ID failed, err: %v", errP))
		}
		bindTemplateIds[k] = append(bindTemplateIds[k], templateIDs...)
	}
	// 查询模板进程关联的模板ID
	for k, v := range ccTemplateProcessIDs {
		templateIDs, errT := s.dao.ConfigTemplate().ListByCCTemplateProcessID(kt, req.GetBizId(), v)
		if errT != nil {
			return nil, errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kt, "list config templates by CC template process ID failed, err: %v", errT))
		}
		bindTemplateIds[k] = append(bindTemplateIds[k], templateIDs...)
	}

	for k, ids := range bindTemplateIds {
		bindTemplateIds[k] = uniqueUint32(ids)
	}

	processes := pbproc.PbProcessesWithInstances(res, procInstMap, bindTemplateIds)

	// environment 由前端放在 search 条件中传递，而非顶层字段。
	filterOptions, err := s.buildfilterOptions(kt, req.GetBizId(), req.GetSearch().GetEnvironment())
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

// ListProcessInnerIPs implements pbds.DataServer.
// 按 expression_scope 过滤命中进程，返回去重后的内网 IP 列表（对齐 gsekit process_status，全量返回不分页）。
func (s *Service) ListProcessInnerIPs(ctx context.Context, req *pbds.ListProcessInnerIPsReq) (
	*pbds.ListProcessInnerIPsResp, error) {
	kt := kit.FromGrpcContext(ctx)

	search := req.GetSearch()
	// 表达式范围模式下环境类型必填
	if err := validateExpressionEnv(search); err != nil {
		return nil, err
	}

	res, _, err := s.dao.Process().List(kt, req.GetBizId(), search, &types.BasePage{All: true})
	if err != nil {
		// 表达式非法等入参错误由 DAO 归类为 InvalidParameter，此处透传保留其错误码。
		if ef, ok := err.(*errf.ErrorF); ok {
			return nil, ef
		}
		return nil, errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kt, "list processes failed, err: %v", err))
	}

	return &pbds.ListProcessInnerIPsResp{Ips: dedupInnerIPs(res)}, nil
}

// validateExpressionEnv 校验表达式范围模式下环境类型必填。
func validateExpressionEnv(search *pbproc.ProcessSearchCondition) error {
	if search.GetExpressionScope() != nil && search.GetEnvironment() == "" {
		return errf.Errorf(errf.InvalidParameter, "%s", "environment is required for expression scope")
	}
	return nil
}

// dedupInnerIPs 从命中进程集合中提取内网 IP，保序去重并跳过空值。
func dedupInnerIPs(processes []*table.Process) []string {
	ips := make([]string, 0, len(processes))
	seen := make(map[string]struct{}, len(processes))
	for _, p := range processes {
		ip := p.Spec.InnerIP
		if ip == "" {
			continue
		}
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		ips = append(ips, ip)
	}
	return ips
}

// OperateProcess implements pbds.DataServer.
// 进程操作：start、stop、register、unregister、restart、reload、kill。
func (s *Service) OperateProcess(ctx context.Context, req *pbds.OperateProcessReq) (*pbds.OperateProcessResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 校验操作类型（query_status 仅用于服务端查询，不作为客户端操作类型）
	if err := validateOperateRequest(kt, req.GetOperateType()); err != nil {
		return nil, err
	}

	// 获取进程和进程实例
	processes, processInstances, err := getProcessesAndInstances(kt, s.dao,
		req.GetBizId(), req.GetProcessIds(), nil, req.GetOperateRange())
	if err != nil {
		return nil, err
	}

	// 启动语义下，过滤缩容实例
	if isStartSemantic(req.GetOperateType()) {
		processes, processInstances = filterInstancesForStart(processes, processInstances)
	}

	toDispatch, err := resolveDispatchItems(kt, processes, processInstances,
		table.ProcessOperateType(req.GetOperateType()))
	if err != nil {
		return nil, err
	}

	// 创建任务批次并下发通用操作任务
	batchID, err := s.createBatchAndDispatch(kt, req.GetOperateType(), processes,
		req.GetOperateRange(), toDispatch, false, newOperateTask, nil)
	if err != nil {
		return nil, err
	}

	return &pbds.OperateProcessResp{BatchID: batchID}, nil
}

// validateOperateRequest 校验通用进程操作的操作类型
// update_register / delete 由独立接口 OperateUpdateRegisterProcess / OperateDeleteProcess 承载。
func validateOperateRequest(kt *kit.Kit, operateType string) error {
	switch table.ProcessOperateType(operateType) {
	case table.StartProcessOperate, table.StopProcessOperate, table.RegisterProcessOperate,
		table.UnregisterProcessOperate, table.RestartProcessOperate, table.ReloadProcessOperate,
		table.KillProcessOperate:
		return nil
	default:
		return errf.Errorf(errf.InvalidParameter, "%s",
			i18n.T(kt, "operate type is not supported: %s", operateType))
	}
}

// getProcessesAndInstances 获取进程和进程实例
// 优先级：指定实例 > 操作范围 > 进程 ID
func getProcessesAndInstances(kt *kit.Kit, dao dao.Set, bizID uint32,
	processIDs, processInstanceIDs []uint32, operateRange *pbproc.OperateRange) (
	[]*table.Process, []*table.ProcessInstance, error) {
	// 指定实例
	if len(processInstanceIDs) != 0 {
		return getByProcessInstanceIDs(kt, dao, bizID, processInstanceIDs)
	}
	// 根据操作范围获取进程和进程实例（适配进程配置管理插件）
	if operateRange != nil {
		return getByOperateRanges(kt, dao, bizID, operateRange)
	}
	return getByProcessIDs(kt, dao, bizID, processIDs)
}

// resolveDispatchItems 组装待下发任务项
func resolveDispatchItems(kt *kit.Kit, processes []*table.Process, processInstances []*table.ProcessInstance,
	operateType table.ProcessOperateType) ([]resolvedInstance, error) {

	// 构建 processMap，用于后续快速查找进程信息
	processMap := make(map[uint32]*table.Process, len(processes))
	for _, p := range processes {
		processMap[p.ID] = p
	}

	items := make([]resolvedInstance, 0, len(processInstances))
	for _, inst := range processInstances {
		proc, ok := processMap[inst.Attachment.ProcessID]
		if !ok {
			return nil, errf.Errorf(errf.Internal, "%s",
				i18n.T(kt, "process not found in processMap, processID=%d", inst.Attachment.ProcessID))
		}
		items = append(items, resolvedInstance{
			instance:        inst,
			finalOpType:     operateType,
			proc:            proc,
			originalStatus:  inst.Spec.Status,
			originalManaged: inst.Spec.ManagedStatus,
		})
	}
	return items, nil
}

// createBatchAndDispatch 创建任务批次并下发任务组，返回批次 ID。
// toDispatch 为空时视为无效操作直接报错，不建批次；下发中途任务部分创建失败时把未创建任务计为批次失败。
// operateRangeInput 仅通用进程操作的插件路径传入，其余操作传 nil（按命中进程构建表达式）。
// buildTask 为该操作链路的任务构建函数（OperateTask / UpdateRegisterTask / DeleteTask 各自独立）。
// configSnapshot 为调用方预取的 CMDB 配置快照（如更新托管链路预检时已拉取，复用避免重复查询），传 nil 时内部批量拉取。
func (s *Service) createBatchAndDispatch(kt *kit.Kit, operateType string, processes []*table.Process,
	operateRangeInput *pbproc.OperateRange, toDispatch []resolvedInstance, enableProcessRestart bool,
	buildTask processTaskBuilder, configSnapshot map[uint32]string) (uint32, error) {

	// 没有需要下发的任务（如启动语义下实例全部被缩容过滤），视为无效操作直接报错
	if len(toDispatch) == 0 {
		return 0, errf.Errorf(errf.FailedPrecondition, "%s",
			i18n.T(kt, "no process instances need to operate"))
	}

	// 下发前批量拉取 CMDB 最新进程配置快照；不管进程是否已在 CMDB 删除都下发任务，
	// 快照缺失由任务执行侧处理：停止 / 强停 / 取消托管回退 DB 配置放行，其余操作报错
	if configSnapshot == nil {
		configSnapshot = refreshOperateSnapshot(kt, s.cmdb, kt.BizID, toDispatch)
	}

	// 构建操作范围，totalCount 只计入真正需要下发任务的实例
	totalCount := uint32(len(toDispatch))
	operateRange := buildOperateRange(processes, operateRangeInput)
	environment := processes[0].Spec.Environment
	if operateRangeInput != nil {
		environment = operateRangeInput.GetEnvironment()
	}

	// 创建任务批次
	batchID, err := createTaskBatch(kt, s.dao, operateType, environment, operateRange, totalCount)
	if err != nil {
		return 0, err
	}
	logs.Infof("create task batch success, batchID: %d, totalCount: %d, rid: %s", batchID, totalCount, kt.Rid)

	// 会由 Callback 推进的任务数；下发中途失败时已落库任务会被就地终结，此处为 0
	var callbackDrivenCount uint32

	// 不会有回调推进的那部分任务在此一次性计为失败，保证批次能达到完成计数并终结
	defer func() {
		if callbackDrivenCount == totalCount {
			// 所有任务都已创建，由 Callback 机制处理状态更新
			return
		}

		// 计算无人推进的任务数并计为失败
		unaccounted := totalCount - callbackDrivenCount
		logs.Warnf("task batch %d partially created: %d/%d tasks dispatched, %d failed to create, rid: %s",
			batchID, callbackDrivenCount, totalCount, unaccounted, kt.Rid)
		if updateErr := s.dao.TaskBatch().AddFailedCount(kt, batchID, unaccounted); updateErr != nil {
			logs.Errorf("add failed count for batch %d error, err: %v, rid: %s", batchID, updateErr, kt.Rid)
		}
	}()

	// 下发任务
	callbackDrivenCount, err = dispatchProcessTasks(
		kt,
		s.dao,
		s.taskManager,
		kt.BizID,
		batchID,
		operateType,
		toDispatch,
		configSnapshot,
		enableProcessRestart,
		buildTask,
	)
	if err != nil {
		return 0, err
	}

	return batchID, nil
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
		// 表达式非法 / env 缺失等入参错误由 DAO 归类为 InvalidParameter，此处透传保留其错误码。
		if ef, ok := err.(*errf.ErrorF); ok {
			return nil, nil, ef
		}
		return nil, nil, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "get processes by operate range failed, err: %v", err))
	}

	if len(processes) == 0 {
		return nil, nil, errf.Errorf(errf.RecordNotFound, "%s",
			i18n.T(kt, "no processes found for biz %d with provided operate range", bizID))
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
		return nil, nil, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "get process instances failed, err: %v", err))
	}

	if len(processInstances) == 0 {
		return nil, nil, errf.Errorf(errf.RecordNotFound, "%s",
			i18n.T(kt, "no process instances found for processes matching operate range"))
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
		return nil, nil, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "get process instances by IDs failed, err: %v", err))
	}
	if len(insts) == 0 {
		return nil, nil, errf.Errorf(errf.RecordNotFound, "%s",
			i18n.T(kt, "process instances not found for IDs %v", processInstanceIDs))
	}

	// 指定实例时这些实例只属于同一个进程
	process, err := dao.Process().GetByID(kt, bizID, insts[0].Attachment.ProcessID)
	if err != nil {
		logs.Errorf("get process failed, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "get process failed, err: %v", err))
	}
	if process == nil {
		return nil, nil, errf.Errorf(errf.RecordNotFound, "%s",
			i18n.T(kt, "process not found for id %d", insts[0].Attachment.ProcessID))
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
		return nil, nil, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "get processes failed, err: %v", err))
	}
	if len(processes) == 0 {
		return nil, nil, errf.Errorf(errf.RecordNotFound, "%s",
			i18n.T(kt, "no processes found for biz %d with provided process IDs", bizID))
	}

	// 查询进程实例列表
	processInstances, err := dao.ProcessInstance().GetByProcessIDs(kt, bizID, processIDs)
	if err != nil {
		logs.Errorf("get process instances failed, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "get process instances failed, err: %v", err))
	}
	if len(processInstances) == 0 {
		return nil, nil, errf.Errorf(errf.RecordNotFound, "%s",
			i18n.T(kt, "no process instances found for process IDs %v", processIDs))
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

// buildOperateRange 从操作范围 / 进程列表构建操作范围（gsekit 风格五段表达式，缺省段为 "*"）。
// 插件路径：原样记录请求 expression_scope 五段（对齐 gsekit API 路径，不解析）；
// 非插件路径：把命中进程 CC 进程 ID 拼成压缩表达式记入 process_id，其余段 "*"（对齐 gsekit 页面路径）。
func buildOperateRange(processes []*table.Process, operateRange *pbproc.OperateRange) table.OperateRange {
	if operateRange != nil {
		es := operateRange.GetExpressionScope()
		return table.OperateRange{
			SetName:      orWildcard(es.GetSetName()),
			ModuleName:   orWildcard(es.GetModuleName()),
			ServiceName:  orWildcard(es.GetServiceName()),
			ProcessAlias: orWildcard(es.GetProcessAlias()),
			ProcessID:    orWildcard(es.GetProcessId()),
		}
	}

	ccProcessIDs := make([]uint32, 0, len(processes))
	for _, process := range processes {
		ccProcessIDs = append(ccProcessIDs, process.Attachment.CcProcessID)
	}
	return table.OperateRange{
		SetName:      "*",
		ModuleName:   "*",
		ServiceName:  "*",
		ProcessAlias: "*",
		ProcessID:    expression.IDsToExpr(ccProcessIDs),
	}
}

// orWildcard 空表达式段回退为 "*"（对齐 gsekit expression_scope 缺省语义）。
func orWildcard(seg string) string {
	if seg == "" {
		return "*"
	}
	return seg
}

// createTaskBatch 创建任务批次
func createTaskBatch(kt *kit.Kit, dao dao.Set, operateType string, environment string,
	operateRange table.OperateRange, totalCount uint32) (uint32, error) {
	now := time.Now()
	taskBatchSpec := &table.TaskBatchSpec{
		TaskObject: table.TaskObjectProcess,
		TaskAction: taskActionOfOperateType(table.ProcessOperateType(operateType)),
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
			TenantID: kt.TenantID,
			BizID:    kt.BizID,
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
		return 0, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "create task batch failed, err: %v", err))
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
		return errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "update process instance status failed, err: %v", err))
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

// OperateUpdateRegisterProcess implements pbds.DataServer.
// 更新托管信息操作：用 CMDB 最新进程配置重新托管，enable_process_restart 决定是否先停旧进程、
// 托管后用新配置拉起新进程；下发更新托管任务（UpdateRegisterTask）。
// 下发前预检，没有必须更新的直接报错不下发任务：
//  1. cc_sync_status != updated 说明 CMDB 变更无需 GSE 侧重新托管，报错
//  2. prev_data / source_data 与 CMDB 最新配置三者一致时任务内各步骤必然全部跳过，
//     直接收敛 cc_sync_status 为 synced（与任务成功后的 Callback 收敛效果一致），报错
func (s *Service) OperateUpdateRegisterProcess(ctx context.Context, req *pbds.OperateUpdateRegisterProcessReq) (
	*pbds.OperateProcessResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 获取进程和进程实例
	processes, processInstances, err := getProcessesAndInstances(kt, s.dao,
		req.GetBizId(), []uint32{req.GetProcessId()}, nil, nil)
	if err != nil {
		return nil, err
	}

	// 未发生 CC 变更（cc_sync_status != updated）的进程无需更新托管，直接报错
	proc := processes[0]
	if proc.Spec.CcSyncStatus != table.Updated {
		logs.Infof("process cc_sync_status is %s, skip update register dispatch, bizID: %d, processID: %d, rid: %s",
			proc.Spec.CcSyncStatus, kt.BizID, proc.ID, kt.Rid)
		return nil, errf.Errorf(errf.FailedPrecondition, "%s",
			i18n.T(kt, "process cc sync status is %s, no need to update register", proc.Spec.CcSyncStatus))
	}

	toDispatch, err := resolveDispatchItems(kt, processes, processInstances, table.UpdateRegisterProcessOperate)
	if err != nil {
		return nil, err
	}

	// 下发前批量拉取 CMDB 最新配置快照，同时用于一致性预检与任务 LatestConfigData 写入
	configSnapshot := refreshOperateSnapshot(kt, s.cmdb, kt.BizID, toDispatch)

	// prev_data / source_data 与 CMDB 最新配置一致时无需更新托管，直接收敛同步状态（快照未命中时
	// 无法预检，保持下发任务由任务执行侧兜底报错）
	if latest, ok := configSnapshot[proc.Attachment.CcProcessID]; ok &&
		processConfigsEqual(proc.Spec.PrevData, proc.Spec.SourceData) &&
		processConfigsEqual(proc.Spec.SourceData, latest) {
		if err = s.dao.Process().UpdateSelectedFields(kt, kt.BizID, map[string]any{
			"cc_sync_status": table.Synced,
		}, s.dao.GenQuery().Process.ID.Eq(proc.ID)); err != nil {
			return nil, errf.Errorf(errf.DBOpFailed, "%s",
				i18n.T(kt, "update process cc sync status failed, err: %v", err))
		}
		logs.Infof("process configs all equal, skip update register dispatch and mark synced, "+
			"bizID: %d, processID: %d, rid: %s", kt.BizID, proc.ID, kt.Rid)
		return nil, errf.Errorf(errf.FailedPrecondition, "%s",
			i18n.T(kt, "process configs equal to cmdb latest config, no need to update register"))
	}

	// 创建任务批次并下发更新托管任务（复用预取快照，避免重复查询 CMDB）
	batchID, err := s.createBatchAndDispatch(kt, string(table.UpdateRegisterProcessOperate),
		processes, nil, toDispatch, req.GetEnableProcessRestart(),
		newUpdateRegisterTask(req.GetEnableProcessRestart()), configSnapshot)
	if err != nil {
		return nil, err
	}

	return &pbds.OperateProcessResp{BatchID: batchID}, nil
}

// processConfigsEqual 解析两份 ProcessInfo JSON 并对比字段是否一致
// （对齐任务执行侧 processInfoChanged 的对比语义，解析失败视为不一致）。
func processConfigsEqual(dataA, dataB string) bool {
	var a, b table.ProcessInfo
	if json.Unmarshal([]byte(dataA), &a) != nil || json.Unmarshal([]byte(dataB), &b) != nil {
		return false
	}
	return reflect.DeepEqual(a, b)
}

// newUpdateRegisterTask 更新托管信息的任务构建（UpdateRegisterTask），
// enable_process_restart 属于请求级参数，由入口闭包捕获。
func newUpdateRegisterTask(enableProcessRestart bool) processTaskBuilder {
	return func(daoSet dao.Set, item resolvedInstance, tenantID string,
		bizID, batchID uint32, user, taskType string, syncStatus table.CCSyncStatus) (*taskTypes.Task, error) {
		return task.NewByTaskBuilder(
			processBuilder.NewUpdateRegisterTask(
				daoSet,
				tenantID,
				bizID,
				batchID,
				item.instance.Attachment.ProcessID,
				item.instance.ID,
				user,
				item.originalManaged,
				item.originalStatus,
				syncStatus,
				enableProcessRestart,
			),
		)
	}
}

// OperateDeleteProcess implements pbds.DataServer.
// 一键清除进程缩容实例：无需指定实例 ID，服务端自动筛选缩容实例并从最后一个实例开始清除，
// 按实例状态拆解为停止（运行中）/ 取消托管（已停止且已托管）/ 直接删除（已停止且未托管），
// 下发删除任务（DeleteTask）；进程数量与实例数量一致时无缩容，报错不建批次。
func (s *Service) OperateDeleteProcess(ctx context.Context, req *pbds.OperateDeleteProcessReq) (
	*pbds.OperateProcessResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 获取进程和进程实例
	processes, processInstances, err := getProcessesAndInstances(kt, s.dao,
		req.GetBizId(), []uint32{req.GetProcessId()}, nil, nil)
	if err != nil {
		return nil, err
	}

	// 筛选缩容实例：实例按 host_inst_seq 升序排列后序位超过 proc_num 的才是缩容实例，
	// proc_num 与实例数量一致时无缩容，实例由 CMDB 同步管理，不能被删除
	scaledDown := filterScaledDownInstances(processes[0], processInstances)

	// 无缩容实例（进程数量与实例数量一致）时无需清除，直接报错
	if len(scaledDown) == 0 {
		return nil, errf.Errorf(errf.FailedPrecondition, "%s",
			i18n.T(kt, "no scaled down process instances to delete, processID: %d", processes[0].ID))
	}

	// 预处理，提前分类出需要下发删除任务的实例和需要直接删除的实例
	toDispatch, toDelete, err := preResolveInstances(kt, scaledDown, processes, table.DeleteProcessOperate)
	if err != nil {
		return nil, err
	}

	// 先执行直接删除（不需要任务批次）
	if errB := batchDeleteProcessInstances(kt, s.dao, toDelete); errB != nil {
		return nil, errB
	}

	// 创建任务批次并下发删除任务
	batchID, err := s.createBatchAndDispatch(kt, string(table.DeleteProcessOperate),
		processes, nil, toDispatch, false, newDeleteTask, nil)
	if err != nil {
		return nil, err
	}

	return &pbds.OperateProcessResp{BatchID: batchID}, nil
}

// filterScaledDownInstances 筛选缩容实例：实例按 host_inst_seq 升序排列后，
// 序位（下标+1）超过 proc_num 的实例才是缩容实例（与前端"一键清除缩容实例"口径一致），
// 并按 host_inst_seq 降序返回，保证一键清除从最后一个实例开始。
// proc_num 与实例数量一致时无缩容，此时所有实例均由 CMDB 同步管理，不能被清除删除。
func filterScaledDownInstances(proc *table.Process,
	processInstances []*table.ProcessInstance) []*table.ProcessInstance {

	// 按 host_inst_seq 升序确定实例序位
	sorted := make([]*table.ProcessInstance, len(processInstances))
	copy(sorted, processInstances)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Spec.HostInstSeq < sorted[j].Spec.HostInstSeq
	})

	// 序位超过 proc_num 的实例为缩容实例，从最后一个实例开始倒序收集
	scaledDown := make([]*table.ProcessInstance, 0, len(sorted))
	for idx := len(sorted) - 1; idx >= 0; idx-- {
		if uint32(idx+1) > uint32(proc.Spec.ProcNum) {
			scaledDown = append(scaledDown, sorted[idx])
		}
	}

	return scaledDown
}

// preResolveInstances 清除实例操作的预处理：按实例状态拆解每个实例的真实操作类型，并分类
// 返回：需要下发删除任务的实例列表、需要直接删除的实例列表
func preResolveInstances(kt *kit.Kit, processInstances []*table.ProcessInstance, processes []*table.Process,
	operateType table.ProcessOperateType) (toDispatch []resolvedInstance, toDelete []*table.ProcessInstance, err error) {

	// 构建 processMap，用于后续快速查找进程信息
	processMap := make(map[uint32]*table.Process, len(processes))
	for _, p := range processes {
		processMap[p.ID] = p
	}

	for _, inst := range processInstances {
		proc, ok := processMap[inst.Attachment.ProcessID]
		if !ok {
			return nil, nil, errf.Errorf(errf.Internal, "%s",
				i18n.T(kt, "process not found in processMap, processID=%d", inst.Attachment.ProcessID))
		}

		originalStatus := inst.Spec.Status
		originalManaged := inst.Spec.ManagedStatus
		finalOpType, err := resolveOperateType(kt, operateType, originalStatus, originalManaged)
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

// resolveOperateType 判断最终的操作类型
// 规则：
// 1. 非 delete 操作，保持不变
// 2. delete 操作，根据进程状态和托管状态决定最终操作类型
func resolveOperateType(kt *kit.Kit, operateType table.ProcessOperateType, status table.ProcessStatus,
	managed table.ProcessManagedStatus) (table.ProcessOperateType, error) {

	if operateType != table.DeleteProcessOperate {
		return operateType, nil
	}

	return getDeleteProcessOperateType(kt, status, managed)
}

// getDeleteProcessOperateType 根据进程状态和托管状态，判断 delete 操作最终需要执行的实际操作类型。
// 规则：
// 1. 如果进程状态是 Running（运行中），只需要执行 Stop 操作。
// 2. 如果进程状态是 Stopped（已停止）：
//   - 托管状态为 Managed（已托管）：执行 Unregister（取消托管）
//   - 托管状态为 Unmanaged（未托管）：执行 Delete（删除）
//
// 3. 其他状态均视为非法操作，返回错误。
func getDeleteProcessOperateType(kt *kit.Kit, status table.ProcessStatus,
	managedStatus table.ProcessManagedStatus) (table.ProcessOperateType, error) {

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
		return "", errf.Errorf(errf.FailedPrecondition, "%s",
			i18n.T(kt, "unsupported managed status for delete: %s", managedStatus))
	// 其他状态全部视为非法
	default:
		return "", errf.Errorf(errf.FailedPrecondition, "%s",
			i18n.T(kt, "unsupported process state for delete: status=%s managedStatus=%s",
				status, managedStatus))
	}
}

// batchDeleteProcessInstances 批量直接删除实例（无需任务）
func batchDeleteProcessInstances(kt *kit.Kit, daoSet dao.Set, instances []*table.ProcessInstance) error {
	for _, inst := range instances {
		if err := daoSet.ProcessInstance().Delete(kt, kt.BizID, inst.ID); err != nil {
			logs.Errorf("direct delete process instance %d failed, err: %v, rid: %s", inst.ID, err, kt.Rid)
			return errf.Errorf(errf.DBOpFailed, "%s",
				i18n.T(kt, "delete process instance %d failed, err: %v", inst.ID, err))
		}
	}
	return nil
}

// newDeleteTask 清除进程实例的任务构建（DeleteTask），任务内按拆解后的实际操作执行
func newDeleteTask(daoSet dao.Set, item resolvedInstance, tenantID string,
	bizID, batchID uint32, user, taskType string, syncStatus table.CCSyncStatus) (*taskTypes.Task, error) {
	return task.NewByTaskBuilder(
		processBuilder.NewDeleteTask(
			daoSet,
			tenantID,
			bizID,
			batchID,
			item.instance.Attachment.ProcessID,
			item.instance.ID,
			item.finalOpType, // delete 拆解后的实际操作（停止 / 取消托管）
			user,
			item.originalManaged,
			item.originalStatus,
			syncStatus,
			taskType,
		),
	)
}

// refreshOperateSnapshot 下发前批量拉取 CMDB 最新进程配置快照（ccProcessID -> ConfigData JSON）。
// 不管进程是否已在 CMDB 删除都下发任务：快照未命中的任务 LatestConfigData 为空，
// 由任务执行侧 ValidateOperate 处理——停止 / 强停 / 取消托管回退 DB 配置放行，其余操作报错。
func refreshOperateSnapshot(kt *kit.Kit, cmdbService bkcmdb.Service, bizID uint32,
	toDispatch []resolvedInstance) map[uint32]string {

	ccProcessIDs := make([]uint32, 0, len(toDispatch))
	seen := make(map[uint32]struct{}, len(toDispatch))
	for _, item := range toDispatch {
		id := item.proc.Attachment.CcProcessID
		if id == 0 {
			// 无 CC 进程 ID 属异常数据，快照必不命中，记录后跳过，任务照常下发
			logs.Warnf("process has no cc process id, skip pulling cmdb snapshot, bizID: %d, processID: %d, rid: %s",
				bizID, item.proc.ID, kt.Rid)
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ccProcessIDs = append(ccProcessIDs, id)
	}
	return refreshConfigDataSnapshot(kt, cmdbService, bizID, ccProcessIDs)
}

// refreshConfigDataSnapshot 批量拉取 CMDB 最新进程配置快照（ccProcessID -> ConfigData JSON）。
// 通过 process 下的 bk_process_id 匹配 ccProcessIDs；已在 CMDB 删除的进程不会出现在返回结果中，
// 不做剔除与标记，由任务执行侧 CompareWithCMDBProcessInfo 步骤决定放行（停止）或报错。
// 查询失败或无可查 ID 时返回空快照，任务执行降级使用 DB source_data，不阻断下发。
func refreshConfigDataSnapshot(kt *kit.Kit, cmdbService bkcmdb.Service, bizID uint32,
	ccProcessIDs []uint32) map[uint32]string {

	if len(ccProcessIDs) == 0 {
		return nil
	}

	// bk_process_id 仅作为匹配键，其余为需要与 DB source_data（ProcessInfo）对比的配置字段
	fields := []string{"bk_process_id", "bk_start_param_regex", "work_path", "pid_file", "user",
		"reload_cmd", "restart_cmd", "start_cmd", "stop_cmd", "face_stop_cmd", "timeout", "bk_start_check_secs"}

	bkProcessIDs := make([]int64, 0, len(ccProcessIDs))
	seen := make(map[uint32]struct{}, len(ccProcessIDs))
	for _, id := range ccProcessIDs {
		seen[id] = struct{}{}
		bkProcessIDs = append(bkProcessIDs, int64(id))
	}

	infos, err := fetchAllProcessRelatedInfo(kt, cmdbService, int(bizID), bkProcessIDs, fields)
	if err != nil {
		logs.Errorf("batch list process related info from cmdb failed, bizID: %d, count: %d, err: %v, rid: %s",
			bizID, len(bkProcessIDs), err, kt.Rid)
		// 快照置空（查询失败不代表进程已删除）
		return nil
	}

	snapshot := make(map[uint32]string, len(ccProcessIDs))
	for _, item := range infos {
		if item.Process == nil {
			continue
		}
		info := item.Process
		// process 下的 bk_process_id 匹配 ccProcessIDs，存在才纳入快照
		if _, ok := seen[uint32(info.BkProcessID)]; !ok {
			continue
		}
		tableInfo := table.ProcessInfo{
			BkStartParamRegex: info.BkStartParamRegex,
			WorkPath:          info.WorkPath,
			PidFile:           info.PidFile,
			User:              info.User,
			ReloadCmd:         info.ReloadCmd,
			RestartCmd:        info.RestartCmd,
			StartCmd:          info.StartCmd,
			StopCmd:           info.StopCmd,
			FaceStopCmd:       info.FaceStopCmd,
			Timeout:           info.Timeout,
			StartCheckSecs:    info.BkStartCheckSecs,
		}
		configData, err := json.Marshal(tableInfo)
		if err != nil {
			logs.Errorf("marshal cmdb process info failed, ccProcessID: %d, err: %v, rid: %s",
				info.BkProcessID, err, kt.Rid)
			continue
		}
		snapshot[uint32(info.BkProcessID)] = string(configData)
	}

	// CMDB 未返回（已删除）的进程不做剔除与标记，由任务执行侧决定放行（停止）或报错
	for _, id := range ccProcessIDs {
		if _, ok := snapshot[id]; !ok {
			logs.Warnf("process not found in cmdb, dispatch task anyway, bizID: %d, ccProcessID: %d, rid: %s",
				bizID, id, kt.Rid)
		}
	}
	return snapshot
}

// fetchAllProcessRelatedInfo 按 bk_process_id 批量拉取全量进程关联信息（对齐 gsekit batch_request 语义）。
// CMDB 单次过滤与分页上限均为 500，超出自动分批、批内翻页拉全；已在 CMDB 删除的进程不会出现在
// 返回结果中，由调用方处理。
func fetchAllProcessRelatedInfo(kt *kit.Kit, cmdbService bkcmdb.Service, bizID int,
	bkProcessIDs []int64, fields []string) ([]*bkcmdb.ProcessRelatedInfoItem, error) {

	const cmdbBatchSize = 500

	all := make([]*bkcmdb.ProcessRelatedInfoItem, 0, len(bkProcessIDs))
	for start := 0; start < len(bkProcessIDs); start += cmdbBatchSize {
		end := start + cmdbBatchSize
		if end > len(bkProcessIDs) {
			end = len(bkProcessIDs)
		}
		batchIDs := bkProcessIDs[start:end]

		for pageStart := 0; ; pageStart += cmdbBatchSize {
			req := &bkcmdb.ListProcessRelatedInfoReq{
				BkBizID: bizID,
				Page: &bkcmdb.PageParam{
					Start: pageStart,
					Limit: cmdbBatchSize,
				},
				ProcessPropertyFilter: &bkcmdb.ProcessPropertyFilter{
					Condition: "AND",
					Rules: []bkcmdb.ProcessFilterRule{{
						Field:    "bk_process_id",
						Operator: "in",
						Value:    batchIDs,
					}},
				},
				Fields: fields,
			}

			resp, err := cmdbService.ListProcessRelatedInfo(kt.Ctx, req)
			if err != nil {
				return nil, err
			}
			all = append(all, resp.Info...)
			if len(resp.Info) < cmdbBatchSize {
				break
			}
		}
	}
	return all, nil
}

// processTaskBuilder 单个待下发项的任务构建函数，各操作链路传入各自的 Builder
// （通用操作 OperateTask / 更新托管 UpdateRegisterTask / 清除实例 DeleteTask），避免集中分发。
type processTaskBuilder func(daoSet dao.Set, item resolvedInstance, tenantID string,
	bizID, batchID uint32, user, taskType string, syncStatus table.CCSyncStatus) (*taskTypes.Task, error)

// dispatchProcessTasks 把一次进程操作的全部实例任务作为一个任务组下发，按启动优先级分阶段执行。
// toDispatch 不管进程是否已在 CMDB 删除都会下发任务，CMDB 最新配置快照由调用方通过
// refreshOperateSnapshot 批量拉取后写入各任务 LatestConfigData。
//
// 返回值是「会由回调推进的任务数」：任务组落库失败时不会有任何任务被创建，
// 此时回滚已写入的实例中间态并返回 0，由调用方把整个批次计为失败。
func dispatchProcessTasks(kt *kit.Kit, daoSet dao.Set, taskManager *task.TaskManager,
	bizID, batchID uint32, taskType string,
	toDispatch []resolvedInstance, configSnapshot map[uint32]string, enableProcessRestart bool,
	buildTask processTaskBuilder) (uint32, error) {

	// 阶段一：全内存构建任务，此阶段失败不产生任何副作用
	tasks := make([]*taskTypes.Task, 0, len(toDispatch))
	items := make([]priority.TaskItem, 0, len(toDispatch))
	for _, item := range toDispatch {
		// 构建任务（finalOpType 已确定，不再是 Delete）
		taskObj, err := buildTask(
			daoSet,
			item,
			kt.TenantID,
			bizID,
			batchID, // 任务组都使用同一个批次 ID
			kt.User,
			taskType,                    // 任务批次的操作类型保持不变，任务内使用 finalOpType 来区分实际操作
			item.proc.Spec.CcSyncStatus, // 下发时刻快照，供 Validate 状态类校验使用
		)
		if err != nil {
			logs.Errorf("create process operate task failed, err: %v, rid: %s", err, kt.Rid)
			return 0, errf.Errorf(errf.Internal, "%s",
				i18n.T(kt, "build process operate task failed, err: %v", err))
		}

		// 记录 CMDB 最新配置快照（LatestConfigData），ConfigData 保留下发时取的 DB 配置
		// （普通操作为 source_data，更新托管为 prev_data，见 UpdateRegisterTask.FinalizeTask）；
		// 未命中（含 CMDB 降级）LatestConfigData 为空，由任务执行侧 ValidateOperate 决定
		// 用 DB 配置放行（停止操作）或报错
		if data, ok := configSnapshot[item.proc.Attachment.CcProcessID]; ok {
			commonPayload := &common.TaskPayload{}
			if err = taskObj.GetCommonPayload(commonPayload); err != nil {
				return 0, errf.Errorf(errf.Internal, "%s",
					i18n.T(kt, "get task common payload failed, err: %v", err))
			}
			if commonPayload.ProcessPayload != nil {
				commonPayload.ProcessPayload.LatestConfigData = data
				if err = taskObj.SetCommonPayload(commonPayload); err != nil {
					return 0, errf.Errorf(errf.Internal, "%s",
						i18n.T(kt, "set task common payload failed, err: %v", err))
				}
			}
		}

		prio := 0
		if item.proc != nil && item.proc.Spec != nil {
			prio = item.proc.Spec.Priority
		}
		tasks = append(tasks, taskObj)
		items = append(items, priority.TaskItem{
			TaskID:   taskObj.TaskID,
			Priority: prio,
			OpType:   item.finalOpType,
		})
	}

	// 阶段二：按优先级排出阶段序列，并把任务归入各自阶段
	plan := priority.BuildStages(items)
	for _, taskObj := range tasks {
		taskObj.StageSeq = plan.StageSeq(taskObj.TaskID)
	}

	group, err := buildProcessTaskGroup(kt, plan, batchID, bizID, taskType, toDispatch)
	if err != nil {
		return 0, err
	}

	// 阶段三：置实例中间态。任务组落库是原子的，因此只有实例状态需要补偿
	for i, item := range toDispatch {
		if err = updateProcessInstanceStatus(kt, daoSet, item.finalOpType,
			item.instance, enableProcessRestart); err != nil {
			logs.Errorf("update process instance status failed, err: %v, rid: %s", err, kt.Rid)
			// 当前这条更新失败未落库，只需回滚它之前已改动的实例
			rollbackInstanceStatus(kt, daoSet, toDispatch[:i])
			return 0, err
		}
	}

	// 阶段四：任务组、阶段与全部任务在同一个事务内落库，随后只下发首个阶段
	if err = taskManager.DispatchGroup(kt.Ctx, group, tasks); err != nil {
		logs.Errorf("dispatch process operate task group failed, batchID: %d, err: %v, rid: %s",
			batchID, err, kt.Rid)
		rollbackInstanceStatus(kt, daoSet, toDispatch)
		return 0, errf.Errorf(errf.Internal, "%s",
			i18n.T(kt, "dispatch process operate task group failed, err: %v", err))
	}

	if err = saveTaskGroupID(kt, daoSet, batchID, group.GroupID); err != nil {
		// 任务组已在执行，这里只影响批次与任务组的关联展示，不阻断本次操作
		logs.Errorf("save task group id failed, batchID: %d, groupID: %s, err: %v, rid: %s",
			batchID, group.GroupID, err, kt.Rid)
	}

	logs.Infof("dispatch process operate task group success, batchID: %d, groupID: %s, stages: %d, rid: %s",
		batchID, group.GroupID, len(group.Stages), kt.Rid)
	return uint32(len(tasks)), nil
}

// buildProcessTaskGroup 构建进程操作任务组，负载中带上批次收尾所需的信息
func buildProcessTaskGroup(kt *kit.Kit, plan *priority.StagePlan, batchID, bizID uint32, taskType string,
	toDispatch []resolvedInstance) (*taskTypes.TaskGroup, error) {

	// 以批次 ID 作为任务组索引，便于按批次反查编排进度
	group := taskTypes.NewTaskGroup(taskTypes.GroupInfo{
		GroupType:      string(table.TaskObjectProcess),
		GroupName:      taskType,
		GroupIndex:     strconv.FormatUint(uint64(batchID), 10),
		GroupIndexType: "task_batch",
		Creator:        kt.User,
	}, taskTypes.WithGroupCallback(processExecutor.ProcessOperateGroupCallbackName.String()))

	// 装入按优先级排好序的阶段
	for _, stage := range plan.Stages {
		group.AddStage(stage)
	}

	// 去重收集本次涉及的进程，批次收尾时需要按进程更新 CMDB 模块实例序列
	processIDs := make([]uint32, 0, len(toDispatch))
	seen := make(map[uint32]struct{}, len(toDispatch))
	for _, item := range toDispatch {
		procID := item.instance.Attachment.ProcessID
		if _, ok := seen[procID]; ok {
			continue
		}
		seen[procID] = struct{}{}
		processIDs = append(processIDs, procID)
	}

	// 任务组回调脱离请求上下文，批次收尾所需的信息只能随负载带上
	if err := group.SetCommonPayload(&processExecutor.GroupPayload{
		TenantID:    kt.TenantID,
		BizID:       bizID,
		BatchID:     batchID,
		OperateUser: kt.User,
		ProcessIDs:  processIDs,
	}); err != nil {
		return nil, errf.Errorf(errf.Internal, "%s",
			i18n.T(kt, "set task group payload failed, err: %v", err))
	}
	return group, nil
}

// rollbackInstanceStatus 把实例恢复为操作前状态，用于任务组下发失败时收敛已写入的中间态
func rollbackInstanceStatus(kt *kit.Kit, daoSet dao.Set, items []resolvedInstance) {
	m := daoSet.GenQuery().ProcessInstance
	for _, item := range items {
		if err := daoSet.ProcessInstance().UpdateSelectedFields(kt, item.instance.Attachment.BizID, map[string]any{
			"status":            item.originalStatus,
			"managed_status":    item.originalManaged,
			"status_updated_at": time.Now(),
		}, m.ID.Eq(item.instance.ID)); err != nil {
			logs.Errorf("rollback process instance %d status failed, err: %v, rid: %s",
				item.instance.ID, err, kt.Rid)
		}
	}
}

// saveTaskGroupID 记录批次与任务框架任务组的关联
func saveTaskGroupID(kt *kit.Kit, daoSet dao.Set, batchID uint32, groupID string) error {
	batch, err := daoSet.TaskBatch().GetByID(kt, kt.BizID, batchID)
	if err != nil {
		return err
	}
	extra, err := batch.Spec.GetExtraData()
	if err != nil {
		return err
	}
	extra.GroupID = groupID
	if err = batch.Spec.SetExtraData(extra); err != nil {
		return err
	}
	return daoSet.TaskBatch().UpdateExtraData(kt, batchID, batch.Spec.ExtraData)
}

// taskActionOfOperateType 进程操作类型映射为任务批次动作
func taskActionOfOperateType(operateType table.ProcessOperateType) table.TaskAction {
	switch operateType {
	case table.StartProcessOperate:
		return table.TaskActionStart
	case table.StopProcessOperate:
		return table.TaskActionStop
	case table.RegisterProcessOperate:
		return table.TaskActionRegister
	case table.UnregisterProcessOperate:
		return table.TaskActionUnregister
	case table.RestartProcessOperate:
		return table.TaskActionRestart
	case table.ReloadProcessOperate:
		return table.TaskActionReload
	case table.KillProcessOperate:
		return table.TaskActionKill
	case table.UpdateRegisterProcessOperate:
		return table.TaskActionUpdateRegister
	case table.DeleteProcessOperate:
		return table.TaskActionDelete
	default:
		return table.TaskAction(operateType)
	}
}

// newOperateTask 通用进程操作的任务构建（OperateTask）
func newOperateTask(daoSet dao.Set, item resolvedInstance, tenantID string,
	bizID, batchID uint32, user, taskType string, syncStatus table.CCSyncStatus) (*taskTypes.Task, error) {
	return task.NewByTaskBuilder(
		processBuilder.NewOperateTask(
			daoSet,
			tenantID,
			bizID,
			batchID,
			item.instance.Attachment.ProcessID,
			item.instance.ID,
			item.finalOpType,
			user,
			item.originalManaged,
			item.originalStatus,
			syncStatus,
			taskType,
		),
	)
}

// ProcessFilterOptions implements pbds.DataServer.
func (s *Service) ProcessFilterOptions(ctx context.Context, req *pbds.ProcessFilterOptionsReq) (
	*pbds.ProcessFilterOptionsResp, error) {
	kt := kit.FromGrpcContext(ctx)

	sets, err := s.dao.Process().ListBizFilterOptions(kt, req.GetBizId(), req.GetEnvironment(),
		field.NewUint32("", "set_id"), field.NewString("", "set_name"))
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "list process filter options (sets) failed, err: %v", err))
	}
	setOptions := make([]*pbproc.ProcessFilterOption, 0, len(sets))
	for _, v := range sets {
		setOptions = append(setOptions, &pbproc.ProcessFilterOption{
			Id:   v.Attachment.SetID,
			Name: v.Spec.SetName,
		})
	}

	modules, err := s.dao.Process().ListBizFilterOptions(kt, req.GetBizId(), req.GetEnvironment(),
		field.NewUint32("", "module_id"), field.NewString("", "module_name"))
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "list process filter options (modules) failed, err: %v", err))
	}
	moduleOptions := make([]*pbproc.ProcessFilterOption, 0, len(modules))
	for _, v := range modules {
		moduleOptions = append(moduleOptions, &pbproc.ProcessFilterOption{
			Id:   v.Attachment.ModuleID,
			Name: v.Spec.ModuleName,
		})
	}

	svcInsts, err := s.dao.Process().ListBizFilterOptions(kt, req.GetBizId(), req.GetEnvironment(),
		field.NewUint32("", "service_instance_id"), field.NewString("", "service_name"))
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "list process filter options (service instances) failed, err: %v", err))
	}
	svcInstOptions := make([]*pbproc.ProcessFilterOption, 0, len(svcInsts))
	for _, v := range svcInsts {
		svcInstOptions = append(svcInstOptions, &pbproc.ProcessFilterOption{
			Id:   v.Attachment.ServiceInstanceID,
			Name: v.Spec.ServiceName,
		})
	}

	processIds, err := s.dao.Process().ListBizFilterOptions(kt, req.GetBizId(), req.GetEnvironment(),
		field.NewUint32("", "cc_process_id"))
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "list process filter options (CC process IDs) failed, err: %v", err))
	}
	processIDOptions := make([]*pbproc.ProcessFilterOption, 0, len(processIds))
	for _, v := range processIds {
		processIDOptions = append(processIDOptions, &pbproc.ProcessFilterOption{
			Id:   v.Attachment.CcProcessID,
			Name: strconv.Itoa(int(v.Attachment.CcProcessID)),
		})
	}

	aliases, err := s.dao.Process().ListBizFilterOptions(kt, req.GetBizId(), req.GetEnvironment(),
		field.NewString("", "alias"))
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "list process filter options (aliases) failed, err: %v", err))
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
func queryFailedTasks(kt *kit.Kit, taskStorage istore.Store, batchID uint32, taskType string) ([]*taskTypes.Task, error) {
	var failedTasks []*taskTypes.Task

	offset := int64(0)
	limit := int64(1000)

	for {
		listOpt := &istore.ListOption{
			TaskIndex: fmt.Sprintf("%d", batchID),
			TaskType:  taskType,
			StatusList: []string{
				taskTypes.TaskStatusFailure,
				taskTypes.TaskStatusTimeout,
			},
			Offset: offset,
			Limit:  limit,
		}

		pagination, err := taskStorage.ListTask(kt.Ctx, listOpt)
		if err != nil {
			return nil, errf.Errorf(errf.Internal, "%s",
				i18n.T(kt, "list failed tasks from task storage failed, err: %v", err))
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

	// 拓扑构建仅需少量字段，使用列裁剪的精简查询，避免全字段（含 source_data/prev_data
	// 大字段）全量拉取；过滤条件与进程列表一致（未删除或存在运行中/托管中实例）。
	processes, err := s.dao.Process().ListTopoProcesses(kt, req.BizId)
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, "%s",
			i18n.T(kt, "list processes failed for process instance topo, err: %v", err))
	}

	if len(processes) == 0 {
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

// buildfilterOptions 组装 ListProcess 返回的基础过滤选项（不含环境相关维度）。
func (s *Service) buildfilterOptions(kt *kit.Kit, bizID uint32, environment string) (*pbproc.FilterOptions, error) {

	ips, err := s.dao.Process().ListBizFilterOptions(kt, bizID, environment, field.NewString("", "inner_ip"))
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kt, "list process filter options (inner IP) failed, err: %v", err))
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
		table.ProcessStatusRunning.String():       i18n.T(kt, "Running"),
		table.ProcessStatusPartlyRunning.String(): i18n.T(kt, "PartiallyRunning"),
		table.ProcessStatusStarting.String():      i18n.T(kt, "Starting"),
		table.ProcessStatusRestarting.String():    i18n.T(kt, "Restarting"),
		table.ProcessStatusStopping.String():      i18n.T(kt, "Stopping"),
		table.ProcessStatusReloading.String():     i18n.T(kt, "Reloading"),
		table.ProcessStatusStopped.String():       i18n.T(kt, "NotRunning"),
	}
	psOptions := makeChoices(processStatusValues)

	// Managed Status 选项
	managedStatusValues := map[string]string{
		table.ProcessManagedStatusStarting.String():      i18n.T(kt, "StartingManagement"),
		table.ProcessManagedStatusStopping.String():      i18n.T(kt, "StoppingManagement"),
		table.ProcessManagedStatusManaged.String():       i18n.T(kt, "Managed"),
		table.ProcessManagedStatusUnmanaged.String():     i18n.T(kt, "Unmanaged"),
		table.ProcessManagedStatusPartlyManaged.String(): i18n.T(kt, "PartiallyManaged"),
	}
	msOptions := makeChoices(managedStatusValues)

	// CC Sync Status 选项
	ccSyncStatusValues := map[string]string{
		table.Synced.String():   i18n.T(kt, "Normal"),
		table.Deleted.String():  i18n.T(kt, "Deleted"),
		table.Updated.String():  i18n.T(kt, "Updated"),
		table.Abnormal.String(): i18n.T(kt, "Abnormal"),
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
