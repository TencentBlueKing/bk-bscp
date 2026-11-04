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

package auth

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/model"
)

// 关闭项目/环境维度后，这两类资源直接放行且不请求权限中心。
func TestAuthorizeBatchIgnoresProjectEnvironmentWhenDisabled(t *testing.T) {
	gw := &fakeGateway{allow: map[string]bool{}}
	a := newTestAuthorizer(t, gw, Config{IgnoreProjectEnvironment: true})

	decisions, err := a.AuthorizeBatch(testKit(), "tester",
		&meta.ResourceAttribute{
			Basic: meta.Basic{Type: meta.Project, Action: meta.View, ResourceID: 40001}, BizID: 100001},
		&meta.ResourceAttribute{
			Basic: meta.Basic{Type: meta.Environment, Action: meta.Delete, ResourceID: 50001},
			BizID: 100001, ProjectID: 40001},
	)
	require.NoError(t, err)
	require.Len(t, decisions, 2)
	require.True(t, decisions[0].Authorized)
	require.True(t, decisions[1].Authorized)

	// 没有真正需要问权限中心的资源。
	require.Zero(t, gw.callCount())
}

// 关闭项目/环境维度后，服务创建仍然校验，只是按业务维度判定。
func TestAuthorizeBatchAppCreateKeepsBizDimensionWhenDisabled(t *testing.T) {
	gw := &fakeGateway{allow: map[string]bool{
		model.ActionAppCreate + "|50001": false,
	}}
	a := newTestAuthorizer(t, gw, Config{IgnoreProjectEnvironment: true})

	decisions, err := a.AuthorizeBatch(testKit(), "tester",
		&meta.ResourceAttribute{
			Basic:     meta.Basic{Type: meta.App, Action: meta.Create, ResourceID: 50001},
			BizID:     100001,
			ProjectID: 40001,
		},
	)
	require.NoError(t, err)
	require.Len(t, decisions, 1)
	require.False(t, decisions[0].Authorized)

	// 仍然向权限中心发问，动作与实例不变，由路径决定只认业务维度。
	require.Equal(t, 1, gw.callCount())
	require.Equal(t, model.ActionAppCreate, gw.calls[0].actionID)
	require.Equal(t, []string{"50001"}, gw.calls[0].resourceIDs)
}

// 关闭项目/环境维度后，服务鉴权仍按服务实例发问，只是路径退化为业务单级。
func TestAuthorizeBatchAppKeepsResourceWhenDisabled(t *testing.T) {
	gw := &fakeGateway{allow: map[string]bool{
		model.ActionAppView + "|30001": true,
	}}
	a := newTestAuthorizer(t, gw, Config{IgnoreProjectEnvironment: true})

	decisions, err := a.AuthorizeBatch(testKit(), "tester",
		&meta.ResourceAttribute{
			Basic:         meta.Basic{Type: meta.App, Action: meta.View, ResourceID: 30001},
			BizID:         100001,
			ProjectID:     40001,
			EnvironmentID: 50001,
		},
	)
	require.NoError(t, err)
	require.Len(t, decisions, 1)
	require.True(t, decisions[0].Authorized)
	require.Equal(t, 1, gw.callCount())
	require.Equal(t, model.ActionAppView, gw.calls[0].actionID)
	require.Equal(t, []string{"30001"}, gw.calls[0].resourceIDs)
}

// 默认配置（不设置 IgnoreProjectEnvironment）仍按项目/环境维度校验。
func TestAuthorizeBatchEnforcesProjectEnvironmentByDefault(t *testing.T) {
	gw := &fakeGateway{allow: map[string]bool{}}
	a := newTestAuthorizer(t, gw, Config{})

	decisions, err := a.AuthorizeBatch(testKit(), "tester",
		&meta.ResourceAttribute{
			Basic: meta.Basic{Type: meta.Environment, Action: meta.View, ResourceID: 50001},
			BizID: 100001, ProjectID: 40001},
	)
	require.NoError(t, err)
	require.Len(t, decisions, 1)
	require.False(t, decisions[0].Authorized)
	require.Equal(t, 1, gw.callCount())
	require.Equal(t, model.ActionEnvironmentView, gw.calls[0].actionID)
}
