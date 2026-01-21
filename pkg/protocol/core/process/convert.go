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

// Package pbproc provides process core protocol struct and convert functions.
package pbproc

import (
	"encoding/json"
	"fmt"
	"time"

	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbpi "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process-instance"
)

const (
	DisableReasonNone                = ""                       // 无原因 / 可操作
	DisableReasonTaskRunning         = "TASK_RUNNING"           // 运行中 / ing 状态
	DisableReasonCmdNotConfigured    = "CMD_NOT_CONFIGURED"     // 尚未配置操作命令
	DisableReasonUnknownProcessState = "UNKNOWN_PROCESS_STATUS" // 进程或托管状态未知
	DisableReasonNoRegisterUpdate    = "NO_REGISTER_UPDATE"     // 无需更新托管信息
	DisableReasonNoNeedOperate       = "NO_NEED_OPERATE"        // 当前状态无需执行该操作
	DisableReasonProcessDeleted      = "PROCESS_DELETED"        // 进程已删除
)

var (
	ProcessConfigViewUrl = "%s/#/business/%d/index?tab=serviceInstance&node=module-%d"
	CmdbProcessConfigURL = "%s/#/business/%d/service/template"
)

// Process convert pb Process to table Process
func (p *Process) Process() (*table.Process, error) {
	if p == nil {
		return nil, nil
	}

	return &table.Process{
		ID:         p.Id,
		Spec:       p.Spec.ProcessSpec(),
		Attachment: p.Attachment.ProcessAttachment(),
	}, nil
}

// ProcessSpec convert pb process to table ProcessSpec
func (p *ProcessSpec) ProcessSpec() *table.ProcessSpec {
	if p == nil {
		return nil
	}

	return &table.ProcessSpec{
		SetName:         p.SetName,
		ModuleName:      p.ModuleName,
		ServiceName:     p.ServiceName,
		Environment:     p.Environment,
		Alias:           p.Alias,
		InnerIP:         p.InnerIp,
		CcSyncStatus:    table.CCSyncStatus(p.CcSyncStatus),
		CcSyncUpdatedAt: timePtrFromProto(p.CcSyncUpdatedAt),
		SourceData:      p.SourceData,
		ProcNum:         uint(p.ProcNum),
	}
}

// PbProcessSpec convert table ProcessSpec to pb ProcessSpec
func PbProcessSpec(spec *table.ProcessSpec, bindTemplateIds []uint32, url string) *ProcessSpec {
	if spec == nil {
		return nil
	}

	return &ProcessSpec{
		SetName:              spec.SetName,
		ModuleName:           spec.ModuleName,
		ServiceName:          spec.ServiceName,
		Environment:          spec.Environment,
		Alias:                spec.Alias,
		InnerIp:              spec.InnerIP,
		CcSyncStatus:         spec.CcSyncStatus.String(),
		CcSyncUpdatedAt:      toProtoTimestamp(spec.CcSyncUpdatedAt),
		SourceData:           spec.SourceData,
		PrevData:             spec.PrevData,
		ProcNum:              uint32(spec.ProcNum),
		BindTemplateIds:      bindTemplateIds,
		ProcessConfigViewUrl: url,
	}
}

// timePtrFromProto 将 protobuf 的 *timestamppb.Timestamp 转换为 Go 的 *time.Time
func timePtrFromProto(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime().UTC()
	return &t
}

// toProtoTimestamp 将 Go 的 *time.Time 转换为 protobuf 的 *timestamppb.Timestamp
func toProtoTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// ProcessAttachment convert pb process to table ProcessAttachment
func (p *ProcessAttachment) ProcessAttachment() *table.ProcessAttachment {
	if p == nil {
		return nil
	}

	return &table.ProcessAttachment{
		TenantID:          p.TenantId,
		BizID:             p.BizId,
		CcProcessID:       p.CcProcessId,
		SetID:             p.SetId,
		ModuleID:          p.ModuleId,
		ServiceInstanceID: p.ServiceInstanceId,
		HostID:            p.HostId,
		CloudID:           p.CloudId,
		AgentID:           p.AgentId,
		ProcessTemplateID: p.ProcessTemplateId,
	}
}

// PbProcessAttachment convert table ProcessAttachment to pb ProcessAttachment
func PbProcessAttachment(attachment *table.ProcessAttachment) *ProcessAttachment {
	if attachment == nil {
		return nil
	}

	return &ProcessAttachment{
		BizId:             attachment.BizID,
		TenantId:          attachment.TenantID,
		CcProcessId:       attachment.CcProcessID,
		SetId:             attachment.SetID,
		ModuleId:          attachment.ModuleID,
		ServiceInstanceId: attachment.ServiceInstanceID,
		HostId:            attachment.HostID,
		CloudId:           attachment.CloudID,
		AgentId:           attachment.AgentID,
		ProcessTemplateId: attachment.ProcessTemplateID,
	}
}

// PbClient convert table Process to pb Process
func PbProcess(c *table.Process, bindTemplateIds []uint32) *Process {
	if c == nil {
		return nil
	}

	url := fmt.Sprintf(ProcessConfigViewUrl, cc.G().CMDB.WebHost, c.Attachment.BizID, c.Attachment.ModuleID)
	return &Process{
		Id:         c.ID,
		Spec:       PbProcessSpec(c.Spec, bindTemplateIds, url),
		Attachment: PbProcessAttachment(c.Attachment),
	}
}

// PbProcesses convert table Process to pb Process
func PbProcesses(c []*table.Process, bindTemplateIds map[uint32][]uint32) []*Process {
	if c == nil {
		return make([]*Process, 0)
	}
	result := make([]*Process, 0)
	for _, v := range c {
		result = append(result, PbProcess(v, bindTemplateIds[v.ID]))
	}
	return result
}

// PbProcessesWithInstances convert table Process to pb Process
func PbProcessesWithInstances(procs []*table.Process, procInstMap map[uint32][]*table.ProcessInstance,
	bindTemplateIds map[uint32][]uint32) []*Process {

	if procs == nil {
		return []*Process{}
	}

	result := make([]*Process, 0, len(procs))

	for _, p := range procs {

		// 1. 解析进程运行时配置
		var processInfo table.ProcessInfo
		if err := json.Unmarshal([]byte(p.Spec.SourceData), &processInfo); err != nil {
			logs.Errorf("unmarshal process source data failed: %v", err)
			continue
		}

		pbProc := PbProcess(p, bindTemplateIds[p.ID])
		insts := procInstMap[p.ID]
		pbProc.ProcInst = pbpi.PbProcInsts(insts)

		agg := aggregateInstanceState(insts)
		pbProc.Spec.Status = agg.procStatus
		pbProc.Spec.ManagedStatus = agg.managedStatus

		pbProc.Spec.Actions = buildProcessActions(
			pbProc.Spec.Status,
			pbProc.Spec.ManagedStatus,
			p.Spec.CcSyncStatus.String(),
			processInfo,
			agg.hasUnknown,
		)

		buildProcessInstanceActions(p, pbProc.ProcInst, processInfo)

		result = append(result, pbProc)
	}
	return result
}

// buildProcessInstanceActions 构建实例级操作按钮
// 规则：
// 1. 仅缩容场景展示实例级按钮
// 2. 仅最后一个实例允许操作
// 3. Unknown / ing 状态冻结
// 4. 实例级按钮仅用于缩容兜底
func buildProcessInstanceActions(proc *table.Process, insts []*pbpi.ProcInst, processInfo table.ProcessInfo) {
	// 非缩容，直接返回（默认已禁用）
	if proc.Spec.ProcNum >= uint(len(insts)) {
		return
	}

	// 缩容：只处理最后一个实例
	lastInst := insts[len(insts)-1]

	// Unknown / ing 状态冻结
	if lastInst.Spec.Status == "" ||
		lastInst.Spec.ManagedStatus == "" ||
		isProcessInProgress(lastInst.Spec.Status) ||
		isManagedInProgress(lastInst.Spec.ManagedStatus) {
		return
	}

	// 构建兜底实例操作能力
	lastInst.Spec.Actions = buildInstanceActions(
		processInfo,
		lastInst.Spec.Status,
		lastInst.Spec.ManagedStatus,
		proc.Spec.CcSyncStatus.String(),
	)
}

func buildInstanceActions(processInfo table.ProcessInfo, instStatus, instManagedStatus,
	syncStatus string) map[string]bool {

	stopAllowed, _, _ := CanProcessOperate(
		table.StopProcessOperate,
		processInfo,
		instStatus,
		instManagedStatus,
		syncStatus,
	)

	unregisterAllowed, _, _ := CanProcessOperate(
		table.UnregisterProcessOperate,
		processInfo,
		instStatus,
		instManagedStatus,
		syncStatus,
	)

	return map[string]bool{
		"stop":       stopAllowed,
		"unregister": unregisterAllowed,
	}
}

// buildProcessActions 构建所有操作类型的可用性
func buildProcessActions(processState, managedState, syncStatus string,
	info table.ProcessInfo, hasUnknownInstance bool) map[string]*ActionAvailability {

	actions := make(map[string]*ActionAvailability)

	ops := []table.ProcessOperateType{
		table.RegisterProcessOperate,
		table.UnregisterProcessOperate,
		table.StartProcessOperate,
		table.StopProcessOperate,
		table.RestartProcessOperate,
		table.ReloadProcessOperate,
		table.KillProcessOperate,
		table.PullProcessOperate,
		table.UpdateRegisterProcessOperate,
	}

	for _, op := range ops {
		allowed, _, reason := CanProcessOperate(op, info, processState, managedState, syncStatus)
		if hasUnknownInstance {
			allowed = false
			reason = DisableReasonUnknownProcessState
		}

		actions[string(op)] = &ActionAvailability{
			Enabled: allowed,
			Reason:  reason,
		}
	}

	return actions
}

func hasBaseRuntimeInfo(info table.ProcessInfo) bool {
	return info.WorkPath != "" &&
		info.PidFile != "" &&
		info.User != ""
}

// HasOperateCommand 验证操作命令
func HasOperateCommand(op table.ProcessOperateType, info table.ProcessInfo) bool {
	switch op {

	case table.StartProcessOperate:
		return info.StartCmd != ""

	case table.StopProcessOperate:
		return info.StopCmd != ""

	case table.RestartProcessOperate:
		return info.RestartCmd != ""

	case table.ReloadProcessOperate:
		return info.ReloadCmd != ""

	case table.KillProcessOperate:
		return info.FaceStopCmd != ""

	default:
		// register / unregister / pull
		return true
	}
}

// isProcessInProgress 判断进程状态是否处于 ing 状态
func isProcessInProgress(state string) bool {
	switch state {
	case table.ProcessStatusStarting.String(),
		table.ProcessStatusStopping.String(),
		table.ProcessStatusReloading.String(),
		table.ProcessStatusRestarting.String():
		return true
	default:
		return false
	}
}

// isManagedInProgress 判断托管状态是否处于 ing 状态
func isManagedInProgress(state string) bool {
	switch state {
	case table.ProcessManagedStatusStarting.String(),
		table.ProcessManagedStatusStopping.String():
		return true
	default:
		return false
	}
}

// operateNeedCommand 判断某个操作是否需要依赖命令
func operateNeedCommand(op table.ProcessOperateType) bool {
	switch op {
	case table.StartProcessOperate,
		table.StopProcessOperate,
		table.KillProcessOperate,
		table.RestartProcessOperate,
		table.ReloadProcessOperate:
		return true
	default:
		return false
	}
}

/*
CanProcessOperate 判断某个操作是否允许执行

整体判定流程如下：

1. 状态未知校验
  - 进程状态或托管状态为空，视为未知状态
  - 所有操作一律禁止

2. 基础运行信息校验
  - workPath / pidFile / user 等基础运行信息不完整
  - 所有操作一律禁止

3. ing（中间态）校验
  - 进程状态或托管状态处于 ing 状态（如 starting / stopping / registering 等）
  - 所有操作一律禁止

4. 命令依赖型操作校验
  - 对依赖命令的操作（如 start / stop / restart 等）
  - 若对应命令不存在，则禁止操作

5. 已删除（syncStatus = Deleted）状态的特殊规则
  - 停止操作：
    · 运行中 或 部分运行 → 允许停止
    · 已停止 → 禁止（无需操作）
  - 取消托管操作：
    · 已托管 或 部分托管 → 允许取消托管
    · 未托管 → 禁止（无需操作）
  - 其他操作：
    · 一律禁止，返回进程已删除

6. 正常状态下的操作判定逻辑

  - 注册操作（Register）：
    · 未托管 或 部分托管 → 允许注册
    · 已托管 → 禁止（无需操作）

  - 取消托管操作（Unregister）：
    · 已托管 或 部分托管 → 允许取消托管
    · 未托管 → 禁止（无需操作）

  - 启动操作（Start）：
    · 已停止 或 部分运行 → 允许启动
    · 运行中 → 禁止（无需操作）

  - 重启 / 重载操作（Restart / Reload）：
    · 不区分当前运行状态，一律允许

  - 停止 / 杀死操作（Stop / Kill）：
    · 运行中 或 部分运行 → 允许
    · 已停止 → 禁止（无需操作）

  - 拉取操作（Pull）：
    · 不依赖运行 / 托管状态，一律允许

  - 更新托管信息操作（UpdateRegister）：
    · syncStatus = Updated → 允许更新
    · 非 Updated → 禁止（无需操作）
*/
func CanProcessOperate(op table.ProcessOperateType, info table.ProcessInfo, processState,
	managedState, syncStatus string) (bool, string, string) {

	// 1. 状态未知
	if processState == "" || managedState == "" {
		return false,
			"original process status or managed status is empty, cannot operate",
			DisableReasonUnknownProcessState
	}

	// 2. 基础运行信息
	if !hasBaseRuntimeInfo(info) {
		return false,
			"workPath, PidFile, and User cannot be empty, cannot operate",
			DisableReasonCmdNotConfigured
	}

	// 3. ing 状态禁止所有操作
	if isProcessInProgress(processState) || isManagedInProgress(managedState) {
		return false,
			"process is in intermediate state, cannot operate",
			DisableReasonTaskRunning
	}

	// 4. 命令依赖型操作：统一校验命令是否存在
	if operateNeedCommand(op) && !HasOperateCommand(op, info) {
		return false,
			fmt.Sprintf("the %s command does not exist", op),
			DisableReasonCmdNotConfigured
	}

	// 状态归一化
	isRunning := processState == table.ProcessStatusRunning.String()
	isStopped := processState == table.ProcessStatusStopped.String()
	isPartlyRunning := processState == table.ProcessStatusPartlyRunning.String()
	isManaged := managedState == table.ProcessManagedStatusManaged.String()
	isUnmanaged := managedState == table.ProcessManagedStatusUnmanaged.String()
	isPartlyManaged := managedState == table.ProcessManagedStatusPartlyManaged.String()
	isDeleted := syncStatus == table.Deleted.String()

	// 5. 已删除状态的特殊规则
	if isDeleted {
		switch op {
		case table.StopProcessOperate:
			if isRunning || isPartlyRunning {
				return true, "", DisableReasonNone
			}
			return false,
				"process is already stopped, no need to stop",
				DisableReasonNoNeedOperate

		case table.UnregisterProcessOperate:
			if isManaged || isPartlyManaged {
				return true, "", DisableReasonNone
			}
			return false,
				"process is already unregistered, no need to unregister",
				DisableReasonNoNeedOperate

		default:
			return false,
				"process is deleted, cannot operate",
				DisableReasonProcessDeleted
		}
	}

	// 6. 正常状态逻辑
	switch op {
	case table.RegisterProcessOperate:
		// 允许：未托管 或 部分托管
		if isUnmanaged || isPartlyManaged {
			return true, "", DisableReasonNone
		}
		return false,
			"process is already managed, no need to register",
			DisableReasonNoNeedOperate

	case table.UnregisterProcessOperate:
		// 允许：已托管 或 部分托管
		if isManaged || isPartlyManaged {
			return true, "", DisableReasonNone
		}
		return false,
			"process is already unmanaged, no need to unregister",
			DisableReasonNoNeedOperate

	case table.StartProcessOperate:
		// 允许：已停止 或 部分运行
		if isStopped || isPartlyRunning {
			return true, "", DisableReasonNone
		}
		return false,
			"process is already started, no need to start",
			DisableReasonNoNeedOperate

	case table.RestartProcessOperate, table.ReloadProcessOperate:
		return true, "", DisableReasonNone
	case table.StopProcessOperate, table.KillProcessOperate:
		// 允许：运行中 或 部分运行
		if isRunning || isPartlyRunning {
			return true, "", DisableReasonNone
		}
		return false,
			"process is already stopped, no need to stop or kill",
			DisableReasonNoNeedOperate

	case table.PullProcessOperate:
		return true, "", DisableReasonNone

	case table.UpdateRegisterProcessOperate:
		if syncStatus == table.Updated.String() {
			return true, "", DisableReasonNone
		}
		return false,
			"process is not updated, cannot update register info",
			DisableReasonNoNeedOperate

	default:
		return false,
			"process cannot operate",
			DisableReasonUnknownProcessState
	}
}

type aggState struct {
	// 主进程“进程状态”聚合结果
	procStatus string

	// 主进程“托管状态”聚合结果
	managedStatus string

	// 是否存在未知状态（空字符串 / 非法值）
	hasUnknown bool

	// 原始集合（可选：调试/按钮裁决用）
	procSet    map[string]struct{}
	managedSet map[string]struct{}
}

// aggregateInstanceState 聚合实例状态，推导主进程状态
func aggregateInstanceState(insts []*table.ProcessInstance) aggState {
	agg := aggState{
		procSet:    map[string]struct{}{},
		managedSet: map[string]struct{}{},
	}

	// 无实例，状态未知
	if len(insts) == 0 {
		agg.hasUnknown = true
		return agg
	}

	for _, inst := range insts {
		ps := inst.Spec.Status
		ms := inst.Spec.ManagedStatus

		if ps == "" {
			agg.hasUnknown = true
		} else {
			agg.procSet[ps.String()] = struct{}{}
		}

		if ms == "" {
			agg.hasUnknown = true
		} else {
			agg.managedSet[ms.String()] = struct{}{}
		}
	}

	agg.procStatus = deriveUnifiedOrPartly(agg.procSet, agg.hasUnknown, table.ProcessStatusPartlyRunning.String())
	agg.managedStatus = deriveUnifiedOrPartly(agg.managedSet, agg.hasUnknown, table.ProcessManagedStatusPartlyManaged.String())

	return agg
}

// 核心推导：
// - 如果 hasUnknown=true -> 返回空字符串
// - 如果 set 里只有 1 种 -> 返回该状态
// - 如果多种 -> partly_running
func deriveUnifiedOrPartly(set map[string]struct{}, hasUnknown bool, partlyStatus string) string {
	// 存在未知状态
	if hasUnknown {
		return ""
	}
	if len(set) == 1 {
		for k := range set {
			return k
		}
	}
	return partlyStatus
}
