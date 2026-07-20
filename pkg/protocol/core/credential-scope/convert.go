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

// Package pbcrs provides credential scope core protocol struct and convert functions.
package pbcrs

import (
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	pbbase "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
)

// CredentialAttachment convert pb CredentialAttachment to table CredentialScopeAttachment
func (m *CredentialScopeAttachment) CredentialAttachment() *table.CredentialScopeAttachment {
	if m == nil {
		return nil
	}

	return &table.CredentialScopeAttachment{
		BizID:        m.BizId,
		CredentialId: m.CredentialId,
		ProjectID:    m.ProjectId,
		EnvID:        m.EnvId,
	}
}

// PbCredentialScopes convert pb CredentialScope to table CredentialScope
func PbCredentialScopes(s []*table.CredentialScope, envMap map[uint32]*table.EnvironmentSpec) ([]*CredentialScopeList, error) {
	if s == nil {
		return make([]*CredentialScopeList, 0), nil
	}

	result := make([]*CredentialScopeList, 0)
	for _, one := range s {
		credentialScope, err := PbCredentialScope(one, envMap)
		if err != nil {
			return nil, err
		}
		result = append(result, credentialScope)
	}

	return result, nil
}

// PbCredentialScope convert table CredentialScope to pb PbCredentialScope
func PbCredentialScope(s *table.CredentialScope, envMap map[uint32]*table.EnvironmentSpec) (*CredentialScopeList, error) {
	if s == nil {
		return nil, nil
	}

	spec, err := PbCredentialScopeSpec(s, envMap)
	if err != nil {
		return nil, err
	}

	return &CredentialScopeList{
		Id:         s.ID,
		Spec:       spec,
		Attachment: PbCredentialScopeAttachment(s.Attachment),
		Revision:   pbbase.PbRevision(s.Revision),
	}, nil
}

// PbCredentialScopeSpec convert table CredentialScopeSpec to pb CredentialScopeSpec
func PbCredentialScopeSpec(s *table.CredentialScope, envMap map[uint32]*table.EnvironmentSpec) (*CredentialScopeSpec, error) {
	if s == nil || s.Spec == nil {
		return nil, nil
	}

	app, scope, err := s.Spec.CredentialScope.Split()
	if err != nil {
		return nil, err
	}

	// 提取 EnvID
	var envID uint32
	var envType, envName string

	if s.Attachment != nil {
		envID = s.Attachment.EnvID
		// 从映射中匹配环境的 Type 和 Name
		if detail, exists := envMap[envID]; exists && detail != nil {
			envType = detail.Type.String()
			envName = detail.Name
		}
	}

	return &CredentialScopeSpec{
		App:     app,
		Scope:   scope,
		EnvId:   envID,
		EnvType: envType,
		EnvName: envName,
	}, nil
}

// PbCredentialScopeAttachment convert table CredentialScopeAttachment to pb CredentialScopeAttachment
func PbCredentialScopeAttachment(at *table.CredentialScopeAttachment) *CredentialScopeAttachment {
	if at == nil {
		return nil
	}

	return &CredentialScopeAttachment{
		BizId:        at.BizID,
		CredentialId: at.CredentialId,
		ProjectId:    at.ProjectID,
		EnvId:        at.EnvID,
	}
}
