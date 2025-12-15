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

// Package pbcin provides config instance core protocol struct and convert functions.
package pbcin

import (
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	pbbase "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
)

// PbConfigInstanceAttachment convert table ConfigInstanceAttachment to pb ConfigInstanceAttachment
func PbConfigInstanceAttachment(at *table.ConfigInstanceAttachment) *ConfigInstance {
	if at == nil {
		return nil
	}

	return &ConfigInstance{
		BizId:            at.BizID,
		ConfigTemplateId: at.ConfigTemplateID,
		ModuleInstSeq:    at.ModuleInstSeq,
	}
}

// PbConfigInstance convert table ConfigInstance to pb ConfigInstance
func PbConfigInstance(ci *table.ConfigInstance) *ConfigInstance {
	if ci == nil {
		return nil
	}

	return &ConfigInstance{
		BizId:            ci.Attachment.BizID,
		ConfigTemplateId: ci.Attachment.ConfigTemplateID,
		ModuleInstSeq:    ci.Attachment.ModuleInstSeq,
		CcProcessId:      ci.Attachment.CcProcessID,
		Revision:         pbbase.PbRevision(ci.Revision),
	}
}

// PbConfigInstanceWithDetails convert table ConfigInstance to pb ConfigInstance with additional details
func PbConfigInstanceWithDetails(
	ci *table.ConfigInstance,
	configTemplateName string,
	process *table.Process,
	configVersionName string,
	configVersionMemo string,
	configFileName string,
	latestTemplateRevisionName string,
) *ConfigInstance {
	if ci == nil {
		return nil
	}

	pbCI := &ConfigInstance{
		BizId:                      ci.Attachment.BizID,
		ConfigTemplateId:           ci.Attachment.ConfigTemplateID,
		ConfigTemplateName:         configTemplateName,
		FileName:                   configFileName,
		ModuleInstSeq:              ci.Attachment.ModuleInstSeq,
		CcProcessId:                ci.Attachment.CcProcessID,
		ConfigVersionName:          configVersionName,
		ConfigVersionMemo:          configVersionMemo,
		LatestTemplateRevisionName: latestTemplateRevisionName,
		Revision:                   pbbase.PbRevision(ci.Revision),
	}

	// Fill process related fields
	if process != nil && process.Spec != nil {
		pbCI.Set = process.Spec.SetName
		pbCI.Module = process.Spec.ModuleName
		pbCI.ServiceInstance = process.Spec.ServiceName
		pbCI.ProcessAlias = process.Spec.Alias
	}

	return pbCI
}

// PbConfigInstances convert table ConfigInstances to pb ConfigInstances
func PbConfigInstances(cis []*table.ConfigInstance) []*ConfigInstance {
	if cis == nil {
		return make([]*ConfigInstance, 0)
	}

	result := make([]*ConfigInstance, 0, len(cis))
	for _, ci := range cis {
		result = append(result, PbConfigInstance(ci))
	}

	return result
}
