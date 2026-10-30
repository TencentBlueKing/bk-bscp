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

package iam

import (
	"context"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server/types"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// ListInstances query instances based on filter criteria.
// Keyword 透传给 data-service：V3 的 list_instance 通常为空，search_instance 会带搜索词。
func (i *IAM) ListInstances(kt *kit.Kit, resType client.TypeID, filter *types.ListInstanceFilter,
	page types.Page) (*types.ListInstanceResult, error) {

	if filter == nil {
		filter = &types.ListInstanceFilter{}
	}

	// 权限中心探测资源类型时会不带 parent 调用 list_instance。项目、环境、服务的实例分别挂在
	// 业务、项目和环境下，缺 parent 无法定位上层实例，返回空列表而不是让下游解析空 ID 而报错。
	if (resType == sys.Project || resType == sys.Environment || resType == sys.Application) &&
		(filter.Parent == nil || filter.Parent.ID == "") {
		return &types.ListInstanceResult{Count: 0, Results: make([]types.InstanceResource, 0)}, nil
	}

	// 未带分页参数时给默认分页，避免下游因 limit 非法中断回调。
	if page.Limit == 0 {
		page.Limit = client.BkIAMMaxPageSize
	}

	req := &pbds.ListInstancesReq{
		ResourceType: string(resType),
		Page:         page.PbPage(),
		Keyword:      filter.Keyword,
	}
	if filter.Parent != nil {
		req.ParentType = string(filter.Parent.Type)
		req.ParentId = filter.Parent.ID
	}
	resp, err := i.ds.ListInstances(kt.RpcCtx(), req)
	if err != nil {
		return nil, err
	}

	instances := make([]types.InstanceResource, 0)
	for _, one := range resp.Details {
		instances = append(instances, types.InstanceResource{
			ID:          one.Id,
			DisplayName: one.Name,
		})
	}

	result := &types.ListInstanceResult{
		Count:   resp.Count,
		Results: instances,
	}
	return result, nil
}

// ListInstancesWithAttributes list resource instances that user is privileged to access by policy, returns id list.
func (i *IAM) ListInstancesWithAttributes(ctx context.Context, opts *client.ListWithAttributes) (idList []string,
	err error) {

	// Note implement this when attribute auth is enabled
	return make([]string, 0), nil
}
