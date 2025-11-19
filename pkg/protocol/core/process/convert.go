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
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	pbpi "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process-instance"
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
		CcSyncUpdatedAt: p.CcSyncUpdatedAt.AsTime().UTC(),
		SourceData:      p.SourceData,
		ProcNum:         uint(p.ProcNum),
	}
}

// PbProcessSpec convert table ProcessSpec to pb ProcessSpec
func PbProcessSpec(spec *table.ProcessSpec) *ProcessSpec {
	if spec == nil {
		return nil
	}

	var procNum uint = 1
	if spec.ProcNum != 0 {
		procNum = spec.ProcNum
	}

	return &ProcessSpec{
		SetName:         spec.SetName,
		ModuleName:      spec.ModuleName,
		ServiceName:     spec.ServiceName,
		Environment:     spec.Environment,
		Alias:           spec.Alias,
		InnerIp:         spec.InnerIP,
		CcSyncStatus:    spec.CcSyncStatus.String(),
		CcSyncUpdatedAt: timestamppb.New(spec.CcSyncUpdatedAt),
		SourceData:      spec.SourceData,
		ProcNum:         uint32(procNum),
	}
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
	}
}

// PbClient convert table Process to pb Process
func PbProcess(c *table.Process) *Process {
	if c == nil {
		return nil
	}

	return &Process{
		Id:         c.ID,
		Spec:       PbProcessSpec(c.Spec),
		Attachment: PbProcessAttachment(c.Attachment),
	}
}

// PbProcesses convert table Process to pb Process
func PbProcesses(c []*table.Process) []*Process {
	if c == nil {
		return make([]*Process, 0)
	}
	result := make([]*Process, 0)
	for _, v := range c {
		result = append(result, PbProcess(v))
	}
	return result
}

func PbProcessesWithInstances(procs []*table.Process, procInstMap map[uint32][]*table.ProcessInstance) []*Process {
	if procs == nil {
		return []*Process{}
	}

	result := make([]*Process, 0, len(procs))
	for _, p := range procs {
		pbProc := PbProcess(p)
		if insts, ok := procInstMap[p.ID]; ok {
			pbProc.ProcInst = pbpi.PbProcInsts(insts)
			// 新增逻辑：根据实例状态计算 Process 状态
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

			// 生成对应的按钮
			pbProc.Spec.Actions = buildProcessActions(pbProc.Spec.Status, pbProc.Spec.ManagedStatus,
				pbProc.Spec.CcSyncStatus)

		} else {
			pbProc.ProcInst = []*pbpi.ProcInst{}
		}

		result = append(result, pbProc)
	}
	return result
}

func buildProcessActions(processState, managedState, syncStatus string) map[string]bool {
	actions := map[string]bool{
		"register":   false,
		"unregister": false,
		"start":      false,
		"stop":       false,
		"restart":    false,
		"reload":     false,
		"kill":       false,
		"push":       false,
	}

	// 使用 CanProcessOperate 判断每一个动作
	actions["register"] = CanProcessOperate(table.RegisterProcessOperate, processState, managedState, syncStatus)
	actions["unregister"] = CanProcessOperate(table.UnregisterProcessOperate, processState, managedState, syncStatus)

	actions["start"] = CanProcessOperate(table.StartProcessOperate, processState, managedState, syncStatus)
	actions["stop"] = CanProcessOperate(table.StopProcessOperate, processState, managedState, syncStatus)
	actions["restart"] = CanProcessOperate(table.RestartProcessOperate, processState, managedState, syncStatus)
	actions["reload"] = CanProcessOperate(table.ReloadProcessOperate, processState, managedState, syncStatus)
	actions["kill"] = CanProcessOperate(table.KillProcessOperate, processState, managedState, syncStatus)
	actions["push"] = CanProcessOperate(table.PullProcessOperate, processState, managedState, syncStatus)

	return actions
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

// CanProcessOperate 判断某个操作是否允许执行
//  1. 进程启动中、停止中、重启中、重载中禁止所有操作
//  2. 正在托管中、取消托管中禁止所有操作
//  3. 已删除状态下运行中的进程只能停止，托管中的进程只能取消托管
//  4. 正常状态下的逻辑：
//     已停止允许启动、重启、重载
//     已启动允许停止、强制停止、重启、重载
//     未托管允许托管
//     已托管允许取消托管
//     未删除可以下发
func CanProcessOperate(op table.ProcessOperateType, processState, managedState, syncStatus string) bool {
	// 1. ing 状态禁止所有操作
	isStartingOrStopping := processState == table.ProcessStatusStarting.String() ||
		processState == table.ProcessStatusStopping.String() ||
		processState == table.ProcessStatusReloading.String() || processState == table.ProcessStatusRestarting.String()
	isManagedStartingOrStopping := managedState == table.ProcessManagedStatusStarting.String() ||
		managedState == table.ProcessManagedStatusStopping.String()

	if isStartingOrStopping || isManagedStartingOrStopping {
		return false
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

	// 2. 已删除状态下的额外限制
	if isDeleted {
		switch op {
		case table.StopProcessOperate:
			return isRunning // 运行中才能 stop
		case table.UnregisterProcessOperate:
			return isManaged // 托管中才能 unregister
		default:
			return false // 其他全部禁用
		}
	}

	// 3. 正常状态逻辑
	switch op {
	case table.RegisterProcessOperate: // 未托管：可托管
		return isUnmanaged
	case table.UnregisterProcessOperate: // 已托管：可取消托管
		return isManaged
	case table.StartProcessOperate: // 进程已停止：可启动
		return isStopped
	case table.RestartProcessOperate, table.ReloadProcessOperate: // 进程已停止或已启动：可重启、重载
		return isStopped || isRunning
	case table.StopProcessOperate, table.KillProcessOperate: // 进程已启动：可停止、强制停止
		return isRunning
	case table.PullProcessOperate: // 下发： 只要求未被删除
		return !isDeleted
	default:
		return false
	}
}
