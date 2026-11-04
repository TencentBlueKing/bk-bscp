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

package iamv4

import (
	"errors"
	"fmt"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/model"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbbase "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// listAppInstances 返回某个上级下的服务实例列表。
// 服务的祖先链是「业务 → 项目 → 环境」，正常按环境逐级展开；
// 但角色的展示层级可能配置在业务或项目上，权限中心会用对应的 parent 回调，
// 这里必须一并支持，否则授权页面会因回调报错而只加载到业务层。
// data-service 的 ListInstances 支持可选 keyword，对 name 做包含匹配、对 id 做精确匹配，
// 因此关键字与分页都直接下推，不再在 auth-server 内存过滤。
func (i *IAM) listAppInstances(kt *kit.Kit, ft Filter, p Page) (*ListInstanceResult, error) {
	if ft.Parent == nil || ft.Parent.ID == "" {
		return nil, errors.New("listing app instances requires a parent")
	}

	switch ft.Parent.Type {
	case model.ResourceTypeBiz, model.ResourceTypeProject, model.ResourceTypeEnvironment:
	default:
		return nil, fmt.Errorf("unexpected parent type for app: %s", ft.Parent.Type)
	}

	p = p.normalize()
	resp, err := i.apps.ListInstances(kt.RpcCtx(), &pbds.ListInstancesReq{
		ResourceType: model.ResourceTypeApp,
		ParentType:   ft.Parent.Type,
		ParentId:     ft.Parent.ID,
		Keyword:      ft.Keyword,
		Page:         &pbbase.BasePage{Start: uint32(p.offset()), Limit: uint32(p.PageSize)},
	})
	if err != nil {
		return nil, err
	}

	results := make([]InstanceBrief, 0, len(resp.Details))
	for _, one := range resp.Details {
		results = append(results, InstanceBrief{ID: one.Id, DisplayName: one.Name})
	}

	return &ListInstanceResult{Count: int(resp.Count), Results: results}, nil
}

// fetchAppInstanceInfo 批量返回服务详情。
// app 的 _bk_iam_path_ 声明「业务 → 项目 → 环境」三级归属，权限中心据此在页面上回显资源拓扑。
func (i *IAM) fetchAppInstanceInfo(kt *kit.Kit, ids []string, selector attrSelector) (
	[]instanceInfo, error) {

	resp, err := i.apps.FetchInstanceInfo(kt.RpcCtx(), &pbds.FetchInstanceInfoReq{
		ResourceType: model.ResourceTypeApp,
		Ids:          ids,
	})
	if err != nil {
		return nil, err
	}

	results := make([]instanceInfo, 0, len(resp.Details))
	for _, detail := range resp.Details {
		info := instanceInfo{"id": detail.Id}

		if selector.need(AttrDisplayName) {
			info[AttrDisplayName] = detail.DisplayName
		}

		if selector.need(AttrIAMPath) {
			// V3 的 path 是数组，V4 只接受单个字符串。同一实例的完整拓扑只由一条路径表达，取首项即可。
			info[AttrIAMPath] = firstPath(detail.Path)
		}

		if selector.need(AttrIAMApprovers) {
			approvers := detail.Approver
			if approvers == nil {
				approvers = []string{}
			}
			info[AttrIAMApprovers] = approvers
		}

		results = append(results, info)
	}

	return results, nil
}

// firstPath 取 V3 风格路径数组的首项。为空时返回空串，由权限中心按无拓扑处理。
func firstPath(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	return paths[0]
}
