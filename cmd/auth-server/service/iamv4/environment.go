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

// listEnvironmentInstances 返回某个项目下的环境实例列表。
// 环境挂在项目下，权限中心按项目维度逐层选择资源，因此 parent 必须是项目。
func (i *IAM) listEnvironmentInstances(kt *kit.Kit, ft Filter, p Page) (*ListInstanceResult, error) {
	if ft.Parent == nil || ft.Parent.ID == "" {
		return nil, errors.New("listing environment instances requires a project parent")
	}

	if ft.Parent.Type != model.ResourceTypeProject {
		return nil, fmt.Errorf("unexpected parent type for environment: %s", ft.Parent.Type)
	}

	p = p.normalize()
	resp, err := i.apps.ListInstances(kt.RpcCtx(), &pbds.ListInstancesReq{
		ResourceType: model.ResourceTypeEnvironment,
		ParentType:   model.ResourceTypeProject,
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

// fetchEnvironmentInstanceInfo 批量返回环境详情。
// 环境的 _bk_iam_path_ 是 /biz,{bizID}/project,{projectID}/，权限中心据此在页面上回显
// 「业务 → 项目 → 环境」的拓扑，也让按业务/项目维度的授权覆盖到具体环境。
func (i *IAM) fetchEnvironmentInstanceInfo(kt *kit.Kit, ids []string, selector attrSelector) (
	[]instanceInfo, error) {

	resp, err := i.apps.FetchInstanceInfo(kt.RpcCtx(), &pbds.FetchInstanceInfoReq{
		ResourceType: model.ResourceTypeEnvironment,
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
