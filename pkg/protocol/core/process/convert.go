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

	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	pbpi "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process-instance"
)

const (
	DisableReasonNone                = ""
	DisableReasonTaskRunning         = "TASK_RUNNING"           // 运行中
	DisableReasonCmdNotConfigured    = "CMD_NOT_CONFIGURED"     // 尚未配置操作命令
	DisableReasonUnknownProcessState = "UNKNOWN_PROCESS_STATUS" // 进程和托管状态空
	DisableReasonNoNeedOperate       = "NO_NEED_OPERATE"        // 当前状态无需执行该操作
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

func PbProcessesWithInstances(procs []*table.Process, procInstMap map[uint32][]*table.ProcessInstance,
	bindTemplateIds map[uint32][]uint32) []*Process {
	if procs == nil {
		return []*Process{}
	}

	result := make([]*Process, 0, len(procs))
	for _, p := range procs {
		//  解析 SourceData 获取运行时配置
		var processInfo table.ProcessInfo
		if err := json.Unmarshal([]byte(p.Spec.SourceData), &processInfo); err != nil {
			logs.Errorf("unmarshal process source data failed: %v", err)
			continue
		}
		pbProc := PbProcess(p, bindTemplateIds[p.ID])
		if insts, ok := procInstMap[p.ID]; ok {
			pbProc.ProcInst = pbpi.PbProcInsts(insts)

			// 根据实例状态计算 Process 状态
			statusSet := make(map[string]struct{})
			managedStatusSet := make(map[string]struct{})
			for _, inst := range insts {
				if inst.Spec != nil {
					statusSet[inst.Spec.Status.String()] = struct{}{}
					managedStatusSet[inst.Spec.ManagedStatus.String()] = struct{}{}
				}
			}

			pbProc.Spec.Status = deriveProcessStatus(statusSet)
			pbProc.Spec.ManagedStatus = deriveManagedStatus(managedStatusSet)

			isScaleDown := p.Spec.ProcNum < uint(len(pbProc.ProcInst))

			for i, inst := range pbProc.ProcInst {

				// 1. ing 状态：实例级全部冻结（基于实例自身状态）
				if isProcessInProgress(inst.Spec.Status) ||
					isManagedInProgress(inst.Spec.ManagedStatus) {
					inst.Spec.Actions = emptyInstanceActions()
					continue
				}

				// 2. 缩容：最后一个实例特殊处理，其余实例照常处理
				if isScaleDown && i == len(pbProc.ProcInst)-1 {

					actions := buildInstanceActions(
						processInfo,
						inst.Spec.Status,
						inst.Spec.ManagedStatus,
						p.Spec.CcSyncStatus.String(),
					)

					// 缩容兜底：保证至少能 unregister
					if !actions["stop"] && !actions["unregister"] {
						actions["unregister"] = true
					}

					inst.Spec.Actions = actions
					continue
				}

				// 3. 普通实例能力判断
				inst.Spec.Actions = buildInstanceActions(
					processInfo,
					inst.Spec.Status,
					inst.Spec.ManagedStatus,
					p.Spec.CcSyncStatus.String(),
				)
			}

		} else {
			pbProc.ProcInst = []*pbpi.ProcInst{}
		}

		pbProc.Spec.Actions = buildProcessActions(pbProc.Spec.Status, pbProc.Spec.ManagedStatus,
			pbProc.Spec.CcSyncStatus, processInfo)

		result = append(result, pbProc)
	}
	return result
}

func emptyInstanceActions() map[string]bool {
	return map[string]bool{
		"stop":       false,
		"unregister": false,
	}
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
	info table.ProcessInfo) map[string]*ActionAvailability {

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
	}

	for _, op := range ops {
		actions[string(op)] = BuildActionAvailability(
			op,
			processState,
			managedState,
			syncStatus,
			info,
		)
	}

	return actions
}

func hasBaseRuntimeInfo(info table.ProcessInfo) bool {
	return info.WorkPath != "" &&
		info.PidFile != "" &&
		info.User != ""
}

func BuildActionAvailability(
	op table.ProcessOperateType,
	processState, managedState, syncStatus string,
	info table.ProcessInfo,
) *ActionAvailability {

	can, _, reason := CanProcessOperate(op, info, processState, managedState, syncStatus)
	return &ActionAvailability{
		Enabled: can,
		Reason:  reason,
	}
}

func hasOperateCommand(op table.ProcessOperateType, info table.ProcessInfo) bool {
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

func isManagedInProgress(state string) bool {
	switch state {
	case table.ProcessManagedStatusStarting.String(),
		table.ProcessManagedStatusStopping.String():
		return true
	default:
		return false
	}
}

// CanProcessOperate 判断某个操作是否允许执行
//  1. 检测进程和托管状态
//  2. 检测workPath、PidFile、User是否为空
//  3. 进程启动中、停止中、重启中、重载中禁止所有操作
//  4. 正在托管中、取消托管中禁止所有操作
//  5. 已删除状态下运行中的进程只能停止，托管中的进程只能取消托管
//  6. 正常状态下的逻辑：
//     已停止允许启动、重启、重载，需要判断操作命令
//     已启动允许停止、强制停止、重启、重载，需要判断操作命令
//     未托管允许托管
//     已托管允许取消托管
//     未删除可以下发
func CanProcessOperate(op table.ProcessOperateType, info table.ProcessInfo, processState,
	managedState, syncStatus string) (bool, string, string) {
	// 1. 状态未知校验
	if processState == "" || managedState == "" {
		return false, "original process status or managed status is empty, cannot operate", DisableReasonUnknownProcessState
	}

	// 2. 基础信息校验
	if !hasBaseRuntimeInfo(info) {
		return false, "workPath, PidFile, and User cannot be empty, cannot operate", DisableReasonCmdNotConfigured
	}

	// 3. ing 状态禁止所有操作
	if isProcessInProgress(processState) || isManagedInProgress(managedState) {
		return false, "process is in intermediate state, cannot operate", DisableReasonTaskRunning
	}

	// 运行中
	isRunning := processState == table.ProcessStatusRunning.String()

	// 已停止
	isStopped := processState == table.ProcessStatusStopped.String()

	// 托管中
	isManaged := managedState == table.ProcessManagedStatusManaged.String()

	// 未托管
	isUnmanaged := managedState == table.ProcessManagedStatusUnmanaged.String()

	// 已删除
	isDeleted := syncStatus == table.Deleted.String()

	// 3. 已删除状态下的额外限制
	if isDeleted {
		switch op {
		case table.StopProcessOperate:
			if isRunning {
				// stop 需要命令：先检查命令是否存在
				if !hasOperateCommand(op, info) {
					return false, "the stop command does not exist", DisableReasonCmdNotConfigured
				}
				return true, "", DisableReasonNone
			}
			return false, "process is already stopped, no need to stop", DisableReasonNoNeedOperate
		case table.UnregisterProcessOperate:
			if isManaged {
				return true, "", DisableReasonNone
			}
			return false, "process is already unregistered, no need to unregister", DisableReasonNoNeedOperate
		default:
			return false, "process cannot operate", DisableReasonNone
		}
	}

	// 4. 正常状态逻辑
	switch op {
	case table.RegisterProcessOperate: // 未托管：可托管
		if isUnmanaged {
			return true, "", DisableReasonNone
		}
		return false, "process is already unmanaged, no need to register", DisableReasonNoNeedOperate
	case table.UnregisterProcessOperate: // 已托管：可取消托管
		if isManaged {
			return true, "", DisableReasonNone
		}
		return false, "process is already managed, no need to unregister", DisableReasonNoNeedOperate
	case table.StartProcessOperate: // 进程已停止：可启动
		// start 需要命令
		if !hasOperateCommand(op, info) {
			return false, "the start command does not exist", DisableReasonCmdNotConfigured
		}
		if isStopped {
			return true, "", DisableReasonNone
		}
		return false, "process is already started, no need to start", DisableReasonNoNeedOperate
	case table.RestartProcessOperate, table.ReloadProcessOperate: // 进程启动或停止均可执行重启、重载操作
		// restart/reload 需要命令（按你的需求）
		if !hasOperateCommand(op, info) {
			return false, "the restart/reload command does not exist", DisableReasonCmdNotConfigured
		}
		return true, "", DisableReasonNone
	case table.StopProcessOperate, table.KillProcessOperate: // 进程已启动：可停止、强制停止
		// stop/kill 需要命令
		if !hasOperateCommand(op, info) {
			return false, "the stop/kill command does not exist", DisableReasonCmdNotConfigured
		}
		if isRunning {
			return true, "", DisableReasonNone
		}
		return false, "process is already stopped, no need to stop or kill", DisableReasonNoNeedOperate
	case table.PullProcessOperate: // 下发： 只要求未被删除
		if !isDeleted {
			return true, "", DisableReasonNone
		}
		return false, "process is deleted, cannot pull", DisableReasonUnknownProcessState
	default:
		return false, "process cannot operate", DisableReasonUnknownProcessState
	}
}

// deriveProcessStatus 根据多个实例状态推导主进程状态
func deriveProcessStatus(statusSet map[string]struct{}) string {
	if len(statusSet) == 1 {
		for s := range statusSet {
			return s
		}
	}
	// 存在多个不同状态，说明混合
	return table.ProcessStatusPartlyRunning.String()
}

// deriveManagedStatus 根据多个实例状态推导主托管状态
func deriveManagedStatus(statusSet map[string]struct{}) string {
	if len(statusSet) == 1 {
		for s := range statusSet {
			return s
		}
	}
	// 存在多个不同状态，说明混合
	return table.ProcessManagedStatusPartlyManaged.String()
}
