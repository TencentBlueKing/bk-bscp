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

// Package pbproject provides project core protocol struct and convert functions.
package pbproject

import (
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	pbbase "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
)

// ProjectSpec convert pb ProjectSpec to table ProjectSpec
func (p *ProjectSpec) ProjectSpec() *table.ProjectSpec {
	if p == nil {
		return nil
	}

	return &table.ProjectSpec{
		Name:      p.Name,
		Key:       p.Key,
		Memo:      p.Memo,
		Protected: p.Protected,
	}
}

// PbProjectSpec convert table ProjectSpec to pb ProjectSpec
func PbProjectSpec(spec *table.ProjectSpec) *ProjectSpec {
	if spec == nil {
		return nil
	}

	return &ProjectSpec{
		Name:      spec.Name,
		Key:       spec.Key,
		Memo:      spec.Memo,
		Protected: spec.Protected,
	}
}

// ProjectAttachment convert pb ProjectAttachment to table ProjectAttachment
func (p *ProjectAttachment) ProjectAttachment() *table.ProjectAttachment {
	if p == nil {
		return nil
	}

	return &table.ProjectAttachment{
		TenantID: p.TenantId,
		BizID:    p.BizId,
	}
}

// PbProjectAttachment convert table ProjectAttachment to pb ProjectAttachment
func PbProjectAttachment(p *table.ProjectAttachment) *ProjectAttachment {
	if p == nil {
		return nil
	}

	return &ProjectAttachment{
		TenantId: p.TenantID,
		BizId:    p.BizID,
	}
}

// Project convert pb Project to table Project
func (p *Project) Project() (*table.Project, error) {
	if p == nil {
		return nil, nil
	}

	return &table.Project{
		ID:         p.Id,
		Spec:       p.Spec.ProjectSpec(),
		Attachment: p.Attachment.ProjectAttachment(),
	}, nil
}

// PbProject convert table Project to pb Project
func PbProject(p *table.Project) *Project {
	if p == nil {
		return nil
	}

	return &Project{
		Id:         p.ID,
		Spec:       PbProjectSpec(p.Spec),
		Attachment: PbProjectAttachment(p.Attachment),
		Revision:   pbbase.PbRevision(p.Revision),
	}
}

// PbProjects convert table Projects to pb Projects
func PbProjects(p []*table.Project) []*Project {
	if p == nil {
		return nil
	}

	projects := make([]*Project, 0, len(p))
	for _, proj := range p {
		projects = append(projects, PbProject(proj))
	}
	return projects
}
