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

package adaptor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/model"
)

// V4 固定按项目/环境模型鉴权：项目与环境资源始终进入权限中心，不会被跳过。
func TestAdaptAlwaysAuthorizesProjectAndEnvironment(t *testing.T) {
	cases := []*meta.ResourceAttribute{
		attrInProject(meta.Project, meta.Create, 100001, 40001, 40001),
		attrInProject(meta.Project, meta.View, 100001, 40001, 40001),
		attrInProject(meta.Environment, meta.Create, 100001, 40001, 40001),
		attrInProject(meta.Environment, meta.View, 100001, 40001, 50001),
	}

	for _, a := range cases {
		got, err := Adapt(a)
		require.NoError(t, err, "type %s/%s", a.Type, a.Action)
		require.False(t, got.Skip, "type %s/%s must be authorized", a.Type, a.Action)
	}
}

// V4 只有环境级的服务创建：实例是目标环境，路径为「业务 → 项目」两级，不存在业务级的服务创建。
func TestAdaptAppCreateOnlyOnEnvironment(t *testing.T) {
	got, err := Adapt(attrInEnv(meta.Create, 100001, 40001, 50001, 50001))
	require.NoError(t, err)

	require.Equal(t, model.ActionAppCreate, got.ActionID)
	require.Equal(t, model.ResourceTypeEnvironment, got.ResourceType)
	require.Equal(t, "50001", got.ResourceID)
	require.Equal(t, "/biz,100001/project,40001/", got.AuthResource().Attributes[client.IAMPathAttrKey])
}

// 服务的鉴权路径固定为三级，与权限中心注册的祖先链一致。
func TestAdaptAppKeepsThreeLevelPath(t *testing.T) {
	got, err := Adapt(attrInEnv(meta.View, 100001, 40001, 50001, 30001))
	require.NoError(t, err)
	require.Equal(t,
		"/biz,100001/project,40001/environment,50001/",
		got.AuthResource().Attributes[client.IAMPathAttrKey])
}
