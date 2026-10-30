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

// listProjectInstances 返回某个业务下的项目实例列表。
// 项目是环境权限的授权维度，权限中心在申请环境权限时需要按业务维度列出可选项目，
// 因此这里要求 parent 必须是 biz。
func (i *IAM) listProjectInstances(kt *kit.Kit, ft Filter, p Page) (*ListInstanceResult, error) {
	if ft.Parent == nil || ft.Parent.ID == "" {
		return nil, errors.New("listing project instances requires a biz parent")
	}

	if ft.Parent.Type != model.ResourceTypeBiz {
		return nil, fmt.Errorf("unexpected parent type for project: %s", ft.Parent.Type)
	}

	p = p.normalize()
	resp, err := i.apps.ListInstances(kt.RpcCtx(), &pbds.ListInstancesReq{
		ResourceType: model.ResourceTypeProject,
		ParentType:   model.ResourceTypeBiz,
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

// fetchProjectInstanceInfo 批量返回项目详情。
// 项目的 _bk_iam_path_ 声明它所属的业务，权限中心据此在页面上回显资源拓扑；
// 环境按项目授权，也依赖这条路径让按业务授权的用户覆盖到业务下所有项目及其环境。
func (i *IAM) fetchProjectInstanceInfo(kt *kit.Kit, ids []string, selector attrSelector) (
	[]instanceInfo, error) {

	resp, err := i.apps.FetchInstanceInfo(kt.RpcCtx(), &pbds.FetchInstanceInfoReq{
		ResourceType: model.ResourceTypeProject,
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
