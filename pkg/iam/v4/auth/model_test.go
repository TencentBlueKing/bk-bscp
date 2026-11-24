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

// V4 固定按项目/环境维度校验，环境资源必须向权限中心发问。
func TestAuthorizeBatchEnforcesProjectEnvironment(t *testing.T) {
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

// V4 只有环境级的服务创建：按目标环境实例发问，不会退回业务实例。
func TestAuthorizeBatchAppCreateUsesEnvironment(t *testing.T) {
	gw := &fakeGateway{allow: map[string]bool{
		model.ActionAppCreate + "|50001": true,
	}}
	a := newTestAuthorizer(t, gw, Config{})

	decisions, err := a.AuthorizeBatch(testKit(), "tester",
		&meta.ResourceAttribute{
			Basic:     meta.Basic{Type: meta.App, Action: meta.Create, ResourceID: 50001},
			BizID:     100001,
			ProjectID: 40001,
		},
	)
	require.NoError(t, err)
	require.Len(t, decisions, 1)
	require.True(t, decisions[0].Authorized)
	require.Equal(t, 1, gw.callCount())
	require.Equal(t, model.ActionAppCreate, gw.calls[0].actionID)
	require.Equal(t, []string{"50001"}, gw.calls[0].resourceIDs)
}
