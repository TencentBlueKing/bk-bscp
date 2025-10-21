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

// Package pbapp provides application core protocol struct and convert functions.
package pbpi

import (
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

// Process convert pb Process to table Process
func (p *ProcInst) Process() (*table.ProcessInstance, error) {
	if p == nil {
		return nil, nil
	}

	return &table.ProcessInstance{
		ID:         p.Id,
		Spec:       p.Spec.ProcInstSpec(),
		Attachment: p.Attachment.ProcInstAttachment(),
	}, nil
}

// ProcessSpec convert pb process to table ProcessSpec
func (p *ProcInstSpec) ProcInstSpec() *table.ProcessInstanceSpec {
	if p == nil {
		return nil
	}

	return &table.ProcessInstanceSpec{
		LocalInstID:     p.LocalInstId,
		InstID:          p.InstId,
		Status:          table.ProcessStatus(p.Status),
		ManagedStatus:   table.ManagedStatus(p.ManagedStatus),
		StatusUpdatedAt: p.GetStatusUpdatedAt().AsTime().UTC(),
	}
}

// PbProcessSpec convert table ProcessSpec to pb ProcessSpec
func PbProcInstSpec(spec *table.ProcessInstanceSpec) *ProcInstSpec {
	if spec == nil {
		return nil
	}

	return &ProcInstSpec{
		LocalInstId:     spec.LocalInstID,
		InstId:          spec.InstID,
		Status:          spec.Status.String(),
		ManagedStatus:   spec.ManagedStatus.String(),
		StatusUpdatedAt: timestamppb.New(spec.StatusUpdatedAt),
	}
}

// ProcessAttachment convert pb process to table ProcessAttachment
func (p *ProcInstAttachment) ProcInstAttachment() *table.ProcessInstanceAttachment {
	if p == nil {
		return nil
	}

	return &table.ProcessInstanceAttachment{
		TenantID:    p.TenantId,
		BizID:       p.BizId,
		ProcessID:   p.ProcessId,
		CcProcessID: p.CcProcessId,
	}
}

// PbProcessAttachment convert table ProcessAttachment to pb ProcessAttachment
func PbProcInstAttachment(attachment *table.ProcessInstanceAttachment) *ProcInstAttachment {
	if attachment == nil {
		return nil
	}

	return &ProcInstAttachment{
		BizId:       attachment.BizID,
		TenantId:    attachment.TenantID,
		CcProcessId: attachment.CcProcessID,
		ProcessId:   attachment.ProcessID,
	}
}

// PbClient convert table Process to pb Process
func PbProcInst(c *table.ProcessInstance) *ProcInst {
	if c == nil {
		return nil
	}

	return &ProcInst{
		Id:         c.ID,
		Spec:       PbProcInstSpec(c.Spec),
		Attachment: PbProcInstAttachment(c.Attachment),
	}
}

// PbProcesses convert table Process to pb Process
func PbProcInsts(c []*table.ProcessInstance) []*ProcInst {
	if c == nil {
		return make([]*ProcInst, 0)
	}
	result := make([]*ProcInst, 0)
	for _, v := range c {
		result = append(result, PbProcInst(v))
	}
	return result
}
