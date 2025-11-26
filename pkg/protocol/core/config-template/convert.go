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
	"encoding/json"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// ConvertBizTopoNodes 批量转换
func ConvertBizTopoNodes(src []*bkcmdb.BizTopoNode) []*BizTopoNode {
	if src == nil {
		return nil
	}
	res := make([]*BizTopoNode, 0, len(src))
	for _, n := range src {
		res = append(res, ConvertBizTopoNode(n))
	}

	return res
}

// ConvertBizTopoNode 单个节点转换（递归）
func ConvertBizTopoNode(src *bkcmdb.BizTopoNode) *BizTopoNode {
	if src == nil {
		return nil
	}

	dst := &BizTopoNode{
		BkInstId:   uint32(src.BkInstID),
		BkInstName: src.BkInstName,
		BkObjIcon:  src.BkObjIcon,
		BkObjId:    src.BkObjID,
		BkObjName:  src.BkObjName,
		Default:    uint32(src.Default),
	}

	// 递归处理 children
	if len(src.Child) > 0 {
		dst.Child = make([]*BizTopoNode, 0, len(src.Child))
		for _, c := range src.Child {
			dst.Child = append(dst.Child, ConvertBizTopoNode(c))
		}
	}

	return dst
}

// ConvertServiceTemplates 批量转换 []*bkcmdb.ServiceTemplate -> []*ServiceTemplate
func ConvertServiceTemplates(src []*bkcmdb.ServiceTemplate) []*ServiceTemplate {
	if src == nil {
		return nil
	}
	res := make([]*ServiceTemplate, 0, len(src))
	for _, s := range src {
		res = append(res, ConvertServiceTemplate(s))
	}
	return res
}

// ConvertServiceTemplate 单个转换
func ConvertServiceTemplate(s *bkcmdb.ServiceTemplate) *ServiceTemplate {
	if s == nil {
		return nil
	}

	return &ServiceTemplate{
		BkBizId:           uint32(s.BkBizID),
		Id:                uint32(s.ID),
		Name:              s.Name,
		ServiceCategoryId: uint32(s.ServiceCategoryID),
		Creator:           s.Creator,
		Modifier:          s.Modifier,
		CreateTime:        s.CreateTime,
		LastTime:          s.LastTime,
		BkSupplierAccount: s.BkSupplierAccount,
		HostApplyEnabled:  s.HostApplyEnabled,
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

	// map[string]PropertyField -> map[string]*PropertyField
	pbProperty := make(map[string]*PropertyField, len(p.Property))
	for k, v := range p.Property {
		pbProperty[k] = ConvertPropertyField(v)
	}

	return &ProcTemplate{
		Id:                uint32(p.ID),
		BkProcessName:     p.BkProcessName,
		BkBizId:           uint32(p.BkBizID),
		ServiceTemplateId: uint32(p.ServiceTemplateID),
		Property:          pbProperty,
		Creator:           p.Creator,
		Modifier:          p.Modifier,
		CreateTime:        p.CreateTime,
		LastTime:          p.LastTime,
		BkSupplierAccount: p.BkSupplierAccount,
	}
}

// ConvertPropertyField 将 PropertyField 转为 pb.PropertyField
func ConvertPropertyField(src bkcmdb.PropertyField) *PropertyField {
	valueStr := ""
	if src.Value != "" {
		b, err := json.Marshal(src.Value)
		if err != nil {
			logs.Errorf("ConvertPropertyField json marshal value failed, err: %v", err)
			return nil
		}
		valueStr = string(b)
	}

	return &PropertyField{
		Value:          valueStr,
		AsDefaultValue: src.AsDefaultValue,
	}
}
