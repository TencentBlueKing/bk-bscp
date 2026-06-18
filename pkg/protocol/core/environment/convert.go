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

// Package pbenvironment provides environment core protocol struct and convert functions.
package pbenvironment

import (
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	pbbase "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
)

// EnvironmentSpec convert pb EnvironmentSpec to table EnvironmentSpec
func (e *EnvironmentSpec) EnvironmentSpec() *table.EnvironmentSpec {
	if e == nil {
		return nil
	}

	return &table.EnvironmentSpec{
		Name:      e.Name,
		Type:      e.EnvironmentSpec().Type,
		Memo:      e.Memo,
		Protected: e.Protected,
	}
}

// PbEnvironmentSpec convert table EnvironmentSpec to pb EnvironmentSpec
func PbEnvironmentSpec(spec *table.EnvironmentSpec, appCount uint32) *EnvironmentSpec {
	if spec == nil {
		return nil
	}

	return &EnvironmentSpec{
		Name:      spec.Name,
		Type:      spec.Type.String(),
		Memo:      spec.Memo,
		Protected: spec.Protected,
		AppCount:  appCount,
	}
}

// EnvironmentAttachment convert pb EnvironmentAttachment to table EnvironmentAttachment
func (e *EnvironmentAttachment) EnvironmentAttachment() *table.EnvironmentAttachment {
	if e == nil {
		return nil
	}

	return &table.EnvironmentAttachment{
		TenantID:  e.TenantId,
		BizID:     e.BizId,
		ProjectID: e.ProjectId,
	}
}

// PbEnvironmentAttachment convert table EnvironmentAttachment to pb EnvironmentAttachment
func PbEnvironmentAttachment(p *table.EnvironmentAttachment) *EnvironmentAttachment {
	if p == nil {
		return nil
	}

	return &EnvironmentAttachment{
		TenantId:  p.TenantID,
		BizId:     p.BizID,
		ProjectId: p.ProjectID,
	}
}

// Environment convert pb Environment to table Environment
func (e *Environment) Environment() (*table.Environment, error) {
	if e == nil {
		return nil, nil
	}

	return &table.Environment{
		ID:         e.Id,
		Spec:       e.Spec.EnvironmentSpec(),
		Attachment: e.Attachment.EnvironmentAttachment(),
	}, nil
}

// PbEnvironment convert table Environment to pb Environment
func PbEnvironment(p *table.Environment, appCount uint32) *Environment {
	if p == nil {
		return nil
	}

	return &Environment{
		Id:         p.ID,
		Spec:       PbEnvironmentSpec(p.Spec, appCount),
		Attachment: PbEnvironmentAttachment(p.Attachment),
		Revision:   pbbase.PbRevision(p.Revision),
	}
}

// PbEnvironments convert table Environments to pb Environments
func PbEnvironments(p []*table.Environment, appCounts map[uint32]uint32) []*Environment {
	if p == nil {
		return nil
	}

	environments := make([]*Environment, 0, len(p))
	for _, env := range p {
		ac := appCounts[env.ID]
		environments = append(environments, PbEnvironment(env, ac))
	}
	return environments
}
