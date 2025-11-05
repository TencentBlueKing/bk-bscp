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
		} else {
			pbProc.ProcInst = []*pbpi.ProcInst{}
		}
		result = append(result, pbProc)
	}
	return result
}
