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

package pbas

import (
	"testing"

	pbstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server/types"
)

// 权限中心逐级展开资源选择器时可能不带分页（limit=0），此时必须放行，
// 否则回调返回 400，选择器会整页提示"接入系统资源接口请求失败"。
func TestPullResourceReqAllowsEmptyPageLimit(t *testing.T) {
	filter := new(pbstruct.Struct)
	require.NoError(t, filter.UnmarshalJSON([]byte(`{"parent":{"type":"biz","id":"2"}}`)))

	req := &PullResourceReq{
		Type:   string(client.TypeID("project")),
		Method: string(types.ListInstanceMethod),
		Filter: filter,
		Page:   &Page{Limit: 0, Offset: 0},
	}

	parsed, err := req.PullResourceReq()
	require.NoError(t, err)
	require.Equal(t, uint(0), parsed.Page.Limit)

	filterParsed, ok := parsed.Filter.(types.ListInstanceFilter)
	require.True(t, ok, "filter should be parsed as list instance filter")
	require.NotNil(t, filterParsed.Parent)
	require.Equal(t, "2", filterParsed.Parent.ID)
}

// 超过权限中心单页上限时仍然拒绝，避免一次拉取过多实例。
func TestPullResourceReqRejectsOversizedPageLimit(t *testing.T) {
	req := &PullResourceReq{
		Type:   string(client.TypeID("project")),
		Method: string(types.ListInstanceMethod),
		Page:   &Page{Limit: client.BkIAMMaxPageSize + 1},
	}

	_, err := req.PullResourceReq()
	require.ErrorContains(t, err, "limit should")
}

// 资源项缺少 basic 时不能触发 nil pointer panic：转换后退化为空资源，
// 由鉴权入口按参数错误返回。
func TestResourceAttributesToleratesMissingBasic(t *testing.T) {
	converted := ResourceAttributes([]*ResourceAttribute{
		{BizId: 2},
		nil,
	})

	require.Len(t, converted, 2)
	require.NotNil(t, converted[0])
	require.Equal(t, meta.Basic{}, converted[0].Basic)
	require.Equal(t, uint32(2), converted[0].BizID)
	require.Nil(t, converted[1])
}

// 项目查看的申请弹窗要展示整条需关联的资源实例，格式与权限中心一致：业务/项目。
// 只展示叶子节点时，项目名称缺失会让「关联的资源实例」整列空白。
func TestPermissionApplyResourcesShowsProjectInstancePath(t *testing.T) {
	perm := &IamPermission{
		Actions: []*IamAction{{
			Id:   "project_view",
			Name: "项目查看",
			RelatedResourceTypes: []*IamResourceType{{
				Type:     "project",
				TypeName: "项目",
				Instances: []*IamResourceInstances{{
					Instances: []*IamResourceInstance{
						{Type: "biz", Id: "3", Name: "czl测试"},
						{Type: "project", Id: "8", Name: "11111"},
					},
				}},
			}},
		}},
	}

	details := PermissionApplyResources(perm)

	require.Len(t, details, 1)
	require.Equal(t, "project_view", details[0].Action)
	require.Equal(t, "项目查看", details[0].ActionName)
	require.Equal(t, "project", details[0].Type)
	require.Equal(t, "8", details[0].ResourceId)
	require.Equal(t, "czl测试/11111", details[0].ResourceName)
}

// 项目名称暂时查不到时，仍要展示已解析的业务祖先，不能把整列留空。
func TestPermissionApplyResourcesKeepsAncestorWhenLeafNameMissing(t *testing.T) {
	perm := &IamPermission{
		Actions: []*IamAction{{
			Id:   "project_view",
			Name: "项目查看",
			RelatedResourceTypes: []*IamResourceType{{
				Type: "project",
				Instances: []*IamResourceInstances{{
					Instances: []*IamResourceInstance{
						{Type: "biz", Id: "3", Name: "czl测试"},
						{Type: "project", Id: "8"},
					},
				}},
			}},
		}},
	}

	details := PermissionApplyResources(perm)

	require.Len(t, details, 1)
	require.Equal(t, "czl测试", details[0].ResourceName)
	require.Equal(t, "8", details[0].ResourceId)
}
