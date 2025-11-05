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
			pbProc.Spec.Actions = deriveActions(pbProc.Spec.Status, pbProc.Spec.ManagedStatus,
				pbProc.Spec.CcSyncStatus)

		} else {
			pbProc.ProcInst = []*pbpi.ProcInst{}
		}

		result = append(result, pbProc)
	}
	return result
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

func deriveActions(status, managedStatus, syncStatus string) map[string]bool {
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

	// starting/stopping 状态禁止操作
	if status == table.ProcessStatusStarting.String() || status == table.ProcessStatusStopping.String() {
		return actions
	}

	// 运行中状态
	if status == table.ProcessStatusRunning.String() {
		actions["stop"] = true
		actions["kill"] = true

		if syncStatus != table.Deleted.String() {
			actions["push"] = true // 配置下发
		}
	}

	// 部分运行
	if status == table.ProcessStatusPartlyRunning.String() {
		if managedStatus == table.ProcessManagedStatusPartlyManaged.String() {
			actions["push"] = true
			actions["stop"] = true
		}
	}

	// 停止状态
	if status == table.ProcessStatusStopped.String() {
		if managedStatus == table.ProcessManagedStatusUnmanaged.String() {
			actions["start"] = true
			actions["register"] = true
		}
	}

	// 运行中 或 部分运行 + 托管中 或 部分托管 + 同步状态为 updated → 允许 reload
	if (status == table.ProcessStatusRunning.String() || status == table.ProcessStatusPartlyRunning.String()) &&
		(managedStatus == table.ProcessManagedStatusManaged.String() ||
			managedStatus == table.ProcessManagedStatusPartlyManaged.String()) &&
		syncStatus == table.Updated.String() {
		actions["reload"] = true
	}

	return actions
}
