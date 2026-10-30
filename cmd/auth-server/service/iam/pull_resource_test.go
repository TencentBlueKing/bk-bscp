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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	pbas "github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server"
	"github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server/types"
)

// 权限中心生成申请链接时会不带 ids 调用 fetch_instance_info，
// 此时必须返回空列表而不是错误，否则申请链接生成会因 400 失败。
func TestFetchInstanceInfoWithoutIDs(t *testing.T) {
	cases := map[string]*types.FetchInstanceInfoFilter{
		"nil filter": nil,
		"empty ids":  {},
		"only attrs": {Attrs: []string{"_bk_iam_path_"}},
	}

	i := new(IAM)
	for name, ft := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := i.FetchInstanceInfo(nil, client.TypeID("project"), ft)
			require.NoError(t, err)
			assert.NotNil(t, got)
			assert.Empty(t, got)
		})
	}
}

// 回调请求缺少 filter 时同样返回空数据，不能返回错误码。
func TestPullResourceFetchInstanceInfoWithoutFilter(t *testing.T) {
	i := new(IAM)
	resp, err := i.PullResource(context.Background(), &pbas.PullResourceReq{
		Type:   "project",
		Method: "fetch_instance_info",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	fields := resp.AsMap()
	assert.EqualValues(t, 0, fields["code"])

	data, ok := fields["data"].([]interface{})
	require.True(t, ok, "data should be a list, got %v", fields["data"])
	assert.Empty(t, data)
}

// 权限中心探测资源类型时会不带 parent 调用 list_instance，
// 项目的实例挂在业务下，缺 parent 应返回空结果而不是解析空业务 ID 报错。
func TestListProjectInstancesWithoutParent(t *testing.T) {
	cases := map[string]*types.ListInstanceFilter{
		"nil filter": nil,
		"no parent":  {},
		"empty parent": {
			Parent: &types.ParentFilter{Type: sys.Business},
		},
	}

	i := new(IAM)
	for name, ft := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := i.ListInstances(nil, sys.Project, ft, types.Page{})
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Zero(t, got.Count)
			assert.Empty(t, got.Results)
		})
	}
}

// 回调请求缺少 filter 时返回空结果，不能返回错误码。
func TestPullResourceListInstanceWithoutFilter(t *testing.T) {
	i := new(IAM)
	resp, err := i.PullResource(context.Background(), &pbas.PullResourceReq{
		Type:   "project",
		Method: "list_instance",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	fields := resp.AsMap()
	assert.EqualValues(t, 0, fields["code"])

	data, ok := fields["data"].(map[string]interface{})
	require.True(t, ok, "data should be an object, got %v", fields["data"])
	assert.EqualValues(t, 0, data["count"])
}
