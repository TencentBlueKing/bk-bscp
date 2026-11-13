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

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
)

// projectEnvAuthDisabled 是关闭项目/环境维度校验时的映射选项。
var projectEnvAuthDisabled = PermissionOptions{ProjectEnvironmentAuth: false}

// 关闭项目/环境维度校验后，这两类资源不进入权限中心，映射结果为跳过。
func TestAdaptAuthOptionsDisabledSkipsProjectAndEnvironment(t *testing.T) {
	cases := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Project, Action: meta.Create}, BizID: 100001},
		{Basic: meta.Basic{Type: meta.Project, Action: meta.Find}, BizID: 100001},
		{Basic: meta.Basic{Type: meta.Project, Action: meta.View, ResourceID: 40001}, BizID: 100001},
		{Basic: meta.Basic{Type: meta.Project, Action: meta.Update, ResourceID: 40001}, BizID: 100001},
		{Basic: meta.Basic{Type: meta.Project, Action: meta.Delete, ResourceID: 40001}, BizID: 100001},
		{
			Basic:     meta.Basic{Type: meta.Environment, Action: meta.Create, ResourceID: 40001},
			BizID:     100001,
			ProjectID: 40001,
		},
		{
			Basic:     meta.Basic{Type: meta.Environment, Action: meta.View, ResourceID: 50001},
			BizID:     100001,
			ProjectID: 40001,
		},
		{
			Basic:     meta.Basic{Type: meta.Environment, Action: meta.Update, ResourceID: 50001},
			BizID:     100001,
			ProjectID: 40001,
		},
		{
			Basic:     meta.Basic{Type: meta.Environment, Action: meta.Delete, ResourceID: 50001},
			BizID:     100001,
			ProjectID: 40001,
		},
	}

	for _, a := range cases {
		action, resources, err := AdaptAuthOptionsWithOptions(a, projectEnvAuthDisabled)
		require.NoError(t, err, "type %s/%s", a.Type, a.Action)
		require.Equal(t, sys.Skip, action, "type %s/%s should be skipped", a.Type, a.Action)
		require.Empty(t, resources)
	}
}

// 关闭项目/环境维度校验后，服务创建回到业务维度：app_create 按业务实例发起，不再依赖环境实例。
func TestAdaptAuthOptionsDisabledAppCreateUsesBusiness(t *testing.T) {
	a := &meta.ResourceAttribute{
		Basic:         meta.Basic{Type: meta.App, Action: meta.Create, ResourceID: 50001},
		BizID:         100001,
		ProjectID:     40001,
		EnvironmentID: 50001,
	}

	action, resources, err := AdaptAuthOptionsWithOptions(a, projectEnvAuthDisabled)
	require.NoError(t, err)
	require.Equal(t, sys.AppCreate, action)
	require.Len(t, resources, 1)
	require.Equal(t, sys.Business, resources[0].Type)
	require.Equal(t, "100001", resources[0].ID)

	// 申请链接与鉴权同源，同样只到业务一层。
	application, err := AdaptIAMApplicationOptions([]*meta.ResourceAttribute{a}, projectEnvAuthDisabled)
	require.NoError(t, err)
	require.NotNil(t, application)
	require.Len(t, application.Actions, 1)
	require.Equal(t, string(sys.AppCreate), application.Actions[0].ID)
	require.Len(t, application.Actions[0].RelatedResourceTypes, 1)
	require.Equal(t, string(sys.Business), application.Actions[0].RelatedResourceTypes[0].Type)

	nodes := application.Actions[0].RelatedResourceTypes[0].Instances[0]
	require.Len(t, nodes, 1)
	require.Equal(t, string(sys.Business), nodes[0].Type)
	require.Equal(t, "100001", nodes[0].ID)
}

// 关闭项目/环境维度校验后按旧模型注册，服务申请链接只提交「业务 → 服务」两级。
func TestAdaptIAMApplicationOptionsDisabledAppKeepsBizChain(t *testing.T) {
	a := &meta.ResourceAttribute{
		Basic:         meta.Basic{Type: meta.App, Action: meta.View, ResourceID: 30001},
		BizID:         100001,
		ProjectID:     40001,
		EnvironmentID: 50001,
	}

	application, err := AdaptIAMApplicationOptions([]*meta.ResourceAttribute{a}, projectEnvAuthDisabled)
	require.NoError(t, err)
	require.NotNil(t, application)
	require.Len(t, application.Actions, 1)
	require.Equal(t, string(sys.AppView), application.Actions[0].ID)
	require.Len(t, application.Actions[0].RelatedResourceTypes, 1)
	require.Equal(t, string(sys.Application), application.Actions[0].RelatedResourceTypes[0].Type)

	nodes := application.Actions[0].RelatedResourceTypes[0].Instances[0]
	require.Len(t, nodes, 2)
	require.Equal(t, string(sys.Business), nodes[0].Type)
	require.Equal(t, "100001", nodes[0].ID)
	require.Equal(t, string(sys.Application), nodes[1].Type)
	require.Equal(t, "30001", nodes[1].ID)
}

// 关闭开关后服务申请链接只到业务一层，不要求项目/环境归属。
func TestAdaptIAMApplicationOptionsDisabledAppWithoutProjectEnv(t *testing.T) {
	a := &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.App, Action: meta.View, ResourceID: 30001},
		BizID: 100001,
	}

	application, err := AdaptIAMApplicationOptions([]*meta.ResourceAttribute{a}, projectEnvAuthDisabled)
	require.NoError(t, err)
	require.NotNil(t, application)
	require.Len(t, application.Actions, 1)
	require.Equal(t, string(sys.AppView), application.Actions[0].ID)

	nodes := application.Actions[0].RelatedResourceTypes[0].Instances[0]
	require.Len(t, nodes, 2)
	require.Equal(t, string(sys.Business), nodes[0].Type)
	require.Equal(t, string(sys.Application), nodes[1].Type)
}

// 没有服务实例时（如克隆只按业务维度校验 app_edit）不生成实例级申请项。
func TestAdaptIAMApplicationOptionsWithoutAppInstance(t *testing.T) {
	a := &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.App, Action: meta.Update},
		BizID: 100001,
	}

	application, err := AdaptIAMApplicationOptions([]*meta.ResourceAttribute{a}, projectEnvAuthDisabled)
	require.NoError(t, err)
	require.Nil(t, application)

	// 鉴权本身仍然发起，路径保持完整形态（归属缺失时对应层级为 0）。
	action, resources, err := AdaptAuthOptionsWithOptions(a, projectEnvAuthDisabled)
	require.NoError(t, err)
	require.Equal(t, sys.AppEdit, action)
	require.Len(t, resources, 1)
	require.Equal(t, []string{"/biz,100001/"}, resources[0].Attribute[client.IamPathKey])
}

// 关闭项目/环境维度校验后，服务创建只依赖业务，没有目标环境也能生成申请项。
func TestAdaptIAMApplicationOptionsDisabledAppCreateWithoutEnv(t *testing.T) {
	a := &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.App, Action: meta.Create},
		BizID: 100001,
	}

	application, err := AdaptIAMApplicationOptions([]*meta.ResourceAttribute{a}, projectEnvAuthDisabled)
	require.NoError(t, err)
	require.NotNil(t, application)
	require.Len(t, application.Actions, 1)
	require.Equal(t, string(sys.AppCreate), application.Actions[0].ID)
}

// 关闭项目/环境维度校验后按旧模型注册，服务鉴权仍针对服务实例，路径只声明业务一层。
func TestAdaptAuthOptionsDisabledAppUsesBizPath(t *testing.T) {
	cases := []struct {
		action meta.Action
		want   client.ActionID
	}{
		{meta.View, sys.AppView},
		{meta.Update, sys.AppEdit},
		{meta.Delete, sys.AppDelete},
		{meta.Publish, sys.ReleasePublish},
		{meta.GenerateRelease, sys.ReleaseGenerate},
	}

	for _, c := range cases {
		a := &meta.ResourceAttribute{
			Basic:         meta.Basic{Type: meta.App, Action: c.action, ResourceID: 30001},
			BizID:         100001,
			ProjectID:     40001,
			EnvironmentID: 50001,
		}

		action, resources, err := AdaptAuthOptionsWithOptions(a, projectEnvAuthDisabled)
		require.NoError(t, err, "action %s", c.action)
		require.Equal(t, c.want, action)
		require.Len(t, resources, 1)
		require.Equal(t, sys.Application, resources[0].Type)
		require.Equal(t, "30001", resources[0].ID)
		require.Equal(t, []string{"/biz,100001/"}, resources[0].Attribute[client.IamPathKey])
	}
}

// 开启项目/环境维度校验时保持三级拓扑与按环境创建，确保开关不改变默认行为。
func TestAdaptAuthOptionsEnabledKeepsProjectEnvironmentModel(t *testing.T) {
	view := &meta.ResourceAttribute{
		Basic:         meta.Basic{Type: meta.App, Action: meta.View, ResourceID: 30001},
		BizID:         100001,
		ProjectID:     40001,
		EnvironmentID: 50001,
	}
	action, resources, err := AdaptAuthOptionsWithOptions(view, PermissionOptions{ProjectEnvironmentAuth: true})
	require.NoError(t, err)
	require.Equal(t, sys.AppView, action)
	require.Equal(t, []string{"/biz,100001/project,40001/environment,50001/"},
		resources[0].Attribute[client.IamPathKey])

	create := &meta.ResourceAttribute{
		Basic:     meta.Basic{Type: meta.App, Action: meta.Create, ResourceID: 50001},
		BizID:     100001,
		ProjectID: 40001,
	}
	action, resources, err = AdaptAuthOptionsWithOptions(create, PermissionOptions{ProjectEnvironmentAuth: true})
	require.NoError(t, err)
	require.Equal(t, sys.EnvironmentAppCreate, action)
	require.Len(t, resources, 1)
	require.Equal(t, sys.Environment, resources[0].Type)
	require.Equal(t, "50001", resources[0].ID)
}

// 关闭项目/环境维度校验后，申请链接不再包含这两类资源。
func TestAdaptIAMApplicationOptionsDisabledSkipsProjectAndEnvironment(t *testing.T) {
	resources := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: 100001},
		{Basic: meta.Basic{Type: meta.Project, Action: meta.View, ResourceID: 40001}, BizID: 100001},
		{
			Basic:     meta.Basic{Type: meta.Environment, Action: meta.View, ResourceID: 50001},
			BizID:     100001,
			ProjectID: 40001,
		},
	}

	application, err := AdaptIAMApplicationOptions(resources, projectEnvAuthDisabled)
	require.NoError(t, err)
	require.NotNil(t, application)
	require.Len(t, application.Actions, 1)
	require.Equal(t, string(sys.BusinessViewResource), application.Actions[0].ID)
}

// 只有项目/环境资源时没有可申请的权限，返回空应用由调用方生成空链接。
func TestAdaptIAMApplicationOptionsDisabledAllSkipped(t *testing.T) {
	resources := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Project, Action: meta.View, ResourceID: 40001}, BizID: 100001},
		{
			Basic:     meta.Basic{Type: meta.Environment, Action: meta.Delete, ResourceID: 50001},
			BizID:     100001,
			ProjectID: 40001,
		},
	}

	application, err := AdaptIAMApplicationOptions(resources, projectEnvAuthDisabled)
	require.NoError(t, err)
	require.Nil(t, application)
}
