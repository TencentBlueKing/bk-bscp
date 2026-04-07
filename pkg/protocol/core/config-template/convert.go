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

// Package pbct provides config template core protocol struct and convert functions.
package pbct

import (
	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	pbbase "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
)

// PbConfigTemplate convert table.ConfigTemplate to pb ConfigTemplate
func PbConfigTemplate(ct *table.ConfigTemplate, fullPath string, isProcBound,
	isConfigReleased bool) *ConfigTemplate {
	if ct == nil {
		return nil
	}

	return &ConfigTemplate{
		Id:               ct.ID,
		Spec:             PbConfigTemplateSpec(ct.Spec, fullPath),
		Attachment:       PbConfigTemplateAttachment(ct.Attachment),
		Revision:         pbbase.PbRevision(ct.Revision),
		IsProcBound:      isProcBound,
		IsConfigReleased: isConfigReleased,
	}
}

// PbConfigTemplateSpec convert table.ConfigTemplateSpec to pb ConfigTemplateSpec
func PbConfigTemplateSpec(spec *table.ConfigTemplateSpec, fullPath string) *ConfigTemplateSpec {
	if spec == nil {
		return nil
	}

	return &ConfigTemplateSpec{
		TemplateName:   spec.Name,
		FullPath:       fullPath,
		HighlightStyle: string(spec.HighlightStyle),
	}
}

// PbConfigTemplateAttachment convert table.ConfigTemplateAttachment to pb ConfigTemplateAttachment
func PbConfigTemplateAttachment(att *table.ConfigTemplateAttachment) *ConfigTemplateAttachment {
	if att == nil {
		return nil
	}

	return &ConfigTemplateAttachment{
		BizId:                att.BizID,
		TemplateId:           att.TemplateID,
		CcTemplateProcessIds: att.CcTemplateProcessIDs,
		CcProcessIds:         att.CcProcessIDs,
		TenantId:             att.TenantID,
	}
}

// PbConfigTemplates convert []*table.ConfigTemplate to []*pb ConfigTemplate
func PbConfigTemplates(src []*table.ConfigTemplate, fullPath map[uint32]string,
	releasedMap map[uint32]bool) []*ConfigTemplate {
	if src == nil {
		return nil
	}
	res := make([]*ConfigTemplate, 0, len(src))
	for _, ct := range src {
		isProcBound := len(ct.Attachment.CcProcessIDs) > 0 || len(ct.Attachment.CcTemplateProcessIDs) > 0

		res = append(res, PbConfigTemplate(ct, fullPath[ct.Attachment.TemplateID], isProcBound, releasedMap[ct.ID]))
	}
	return res
}

// ConvertTopoBriefNodes 批量转换 []*bkcmdb.TopoBriefNode -> []*BizTopoNode
func ConvertTopoBriefNodes(src []*bkcmdb.TopoBriefNode) []*BizTopoNode {
	if len(src) == 0 {
		return nil
	}

	res := make([]*BizTopoNode, len(src))
	for i, n := range src {
		res[i] = ConvertTopoBriefNode(n)
	}
	return res
}

// ConvertTopoBriefNode 单个转换 *bkcmdb.TopoBriefNode -> *BizTopoNode
func ConvertTopoBriefNode(src *bkcmdb.TopoBriefNode) *BizTopoNode {
	if src == nil {
		return nil
	}

	objID, objName, objIcon := mapObjMeta(src.Obj)

	dst := &BizTopoNode{
		BkInstId:   uint32(src.ID),
		BkInstName: src.Name,
		BkObjId:    objID,
		BkObjName:  objName,
		BkObjIcon:  objIcon,
		Default:    uint32(src.Default),
	}

	// 递归 children
	if len(src.Nodes) > 0 {
		dst.Child = make([]*BizTopoNode, len(src.Nodes))
		for i, c := range src.Nodes {
			dst.Child[i] = ConvertTopoBriefNode(c)
		}
	}

	return dst
}

// mapObjMeta 将 cmdb 的 obj 转换为前端需要的 id、name、icon
func mapObjMeta(obj string) (id, name, icon string) {
	switch obj {
	case "set":
		return "set", "集群", "icon-set"
	case "module":
		return "module", "模块", "icon-module"
	default:
		return obj, obj, ""
	}
}

// ConvertServiceTemplates 批量转换 []*bkcmdb.ServiceTemplate -> []*ServiceTemplate
func ConvertServiceTemplates(src []*bkcmdb.ServiceTemplate, processesCount map[int]uint32) []*ServiceTemplate {
	if src == nil {
		return nil
	}
	res := make([]*ServiceTemplate, 0, len(src))
	for _, s := range src {
		res = append(res, ConvertServiceTemplate(s, processesCount[s.ID]))
	}
	return res
}

// ConvertServiceTemplate 单个转换
func ConvertServiceTemplate(s *bkcmdb.ServiceTemplate, count uint32) *ServiceTemplate {
	if s == nil {
		return nil
	}

	return &ServiceTemplate{
		Id:           uint32(s.ID),
		Name:         s.Name,
		ProcessCount: count,
	}
}

// ConvertProcTemplates 批量转换
func ConvertProcTemplates(src []*bkcmdb.ProcTemplate) []*ProcTemplate {
	if src == nil {
		return nil
	}
	res := make([]*ProcTemplate, 0, len(src))
	for _, p := range src {
		res = append(res, ConvertProcTemplate(p))
	}
	return res
}

// ConvertProcTemplate 单个转换
func ConvertProcTemplate(p *bkcmdb.ProcTemplate) *ProcTemplate {
	if p == nil {
		return nil
	}

	return &ProcTemplate{
		Id:                uint32(p.ID),
		BkProcessName:     p.BkProcessName,
		BkBizId:           uint32(p.BkBizID),
		ServiceTemplateId: uint32(p.ServiceTemplateID),
	}
}

// ConvertServiceInstances 批量转换： []*bkcmdb.ServiceInstanceInfo → []*pb.ServiceInstanceInfo
func ConvertServiceInstances(src []*bkcmdb.ServiceInstanceInfo) []*ServiceInstanceInfo {
	if src == nil {
		return nil
	}

	res := make([]*ServiceInstanceInfo, 0, len(src))
	for _, inst := range src {
		res = append(res, ConvertServiceInstance(inst))
	}
	return res
}

// ConvertServiceInstance 单个转换 *bkcmdb.ServiceInstanceInfo → *pb.ServiceInstanceInfo
func ConvertServiceInstance(s *bkcmdb.ServiceInstanceInfo) *ServiceInstanceInfo {
	if s == nil {
		return nil
	}

	// map[string]string 转换（protobuf 支持直接赋值）
	labels := make(map[string]string, len(s.Labels))
	for k, v := range s.Labels {
		labels[k] = v
	}

	return &ServiceInstanceInfo{
		Id:   int32(s.ID),
		Name: s.Name,
	}
}

// ConvertProcessInfo converts *bkcmdb.ProcessInfo → *pb.ProcessInfo
func ConvertProcessInfo(src *bkcmdb.ProcessInfo) *ProcessInfo {
	if src == nil {
		return nil
	}

	return &ProcessInfo{
		BkFuncName:    src.BkFuncName,
		BkProcessId:   int32(src.BkProcessID),
		BkProcessName: src.BkProcessName,
	}
}

// ConvertRelation converts *bkcmdb.Relation → *pb.Relation
func ConvertRelation(src *bkcmdb.Relation) *Relation {
	if src == nil {
		return nil
	}

	return &Relation{
		BkBizId:           int32(src.BkBizID),
		BkProcessId:       int32(src.BkProcessID),
		ServiceInstanceId: int32(src.ServiceInstanceID),
		ProcessTemplateId: int32(src.ProcessTemplateID),
		BkHostId:          int32(src.BkHostID),
		BkSupplierAccount: src.BkSupplierAccount,
	}
}

// ConvertProcessInstance converts *bkcmdb.ListProcessInstance → *pb.ListProcessInstance
func ConvertProcessInstance(src *bkcmdb.ListProcessInstance) *ListProcessInstance {
	if src == nil {
		return nil
	}

	return &ListProcessInstance{
		Property: ConvertProcessInfo(src.Property),
		Relation: ConvertRelation(src.Relation),
	}
}

// ConvertProcessInstances converts []*bkcmdb.ListProcessInstance to []*ListProcessInstance
func ConvertProcessInstances(src []*bkcmdb.ListProcessInstance) []*ListProcessInstance {
	if src == nil {
		return nil
	}

	result := make([]*ListProcessInstance, 0, len(src))
	for _, item := range src {
		converted := ConvertProcessInstance(item)
		if converted != nil {
			result = append(result, converted)
		}
	}

	return result
}
