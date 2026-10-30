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

func attr(t meta.ResourceType, a meta.Action, bizID, resourceID uint32) *meta.ResourceAttribute {
	return &meta.ResourceAttribute{
		Basic: meta.Basic{Type: t, Action: a, ResourceID: resourceID},
		BizID: bizID,
	}
}

// attrInProject 用于项目/环境维度的资源，额外携带所属项目 ID。
func attrInProject(t meta.ResourceType, a meta.Action, bizID, projectID, resourceID uint32) *meta.ResourceAttribute {
	return &meta.ResourceAttribute{
		Basic:     meta.Basic{Type: t, Action: a, ResourceID: resourceID},
		BizID:     bizID,
		ProjectID: projectID,
	}
}

// attrInEnv 用于服务维度的资源：服务挂在环境下，需要业务、项目、环境三级归属。
func attrInEnv(a meta.Action, bizID, projectID, envID, resourceID uint32) *meta.ResourceAttribute {
	return &meta.ResourceAttribute{
		Basic:         meta.Basic{Type: meta.App, Action: a, ResourceID: resourceID},
		BizID:         bizID,
		ProjectID:     projectID,
		EnvironmentID: envID,
	}
}

// 映射表逐项核对，覆盖方案 §4.3 的 17 个操作。
func TestAdaptActionMapping(t *testing.T) {
	cases := []struct {
		name    string
		attr    *meta.ResourceAttribute
		action  string
		resType string
		resID   string
		bizID   string
	}{
		{"业务访问", attr(meta.Biz, meta.FindBusinessResource, 100001, 0),
			model.ActionFindBusinessResource, model.ResourceTypeBiz, "100001", "100001"},
		{"服务创建", attrInProject(meta.App, meta.Create, 100001, 40001, 50001),
			model.ActionAppCreate, model.ResourceTypeEnvironment, "50001", "100001"},
		{"服务列表", attr(meta.App, meta.Find, 100001, 0),
			model.ActionFindBusinessResource, model.ResourceTypeBiz, "100001", "100001"},
		{"服务查看", attrInEnv(meta.View, 100001, 40001, 50001, 30001),
			model.ActionAppView, model.ResourceTypeApp, "30001", "100001"},
		{"服务编辑", attrInEnv(meta.Update, 100001, 40001, 50001, 30001),
			model.ActionAppEdit, model.ResourceTypeApp, "30001", "100001"},
		{"服务删除", attrInEnv(meta.Delete, 100001, 40001, 50001, 30001),
			model.ActionAppDelete, model.ResourceTypeApp, "30001", "100001"},
		{"生成版本", attrInEnv(meta.GenerateRelease, 100001, 40001, 50001, 30001),
			model.ActionReleaseGenerate, model.ResourceTypeApp, "30001", "100001"},
		{"上线版本", attrInEnv(meta.Publish, 100001, 40001, 50001, 30001),
			model.ActionReleasePublish, model.ResourceTypeApp, "30001", "100001"},
		{"密钥查看", attr(meta.Credential, meta.View, 100001, 0),
			model.ActionAppCredentialView, model.ResourceTypeBiz, "100001", "100001"},
		{"密钥管理", attr(meta.Credential, meta.Manage, 100001, 0),
			model.ActionAppCredentialManage, model.ResourceTypeBiz, "100001", "100001"},
		{"操作记录", attr(meta.Audit, meta.View, 100001, 0),
			model.ActionAuditView, model.ResourceTypeBiz, "100001", "100001"},
		{"进程配置查看", attr(meta.ProcConfigMgmt, meta.View, 100001, 0),
			model.ActionProcConfigMgmtView, model.ResourceTypeBiz, "100001", "100001"},
		{"进程操作", attr(meta.ProcConfigMgmt, meta.ProcessOperate, 100001, 0),
			model.ActionProcessOperate, model.ResourceTypeBiz, "100001", "100001"},
		{"模板创建", attr(meta.ProcConfigMgmt, meta.Create, 100001, 0),
			model.ActionConfigTemplateCreate, model.ResourceTypeBiz, "100001", "100001"},
		{"模板编辑", attr(meta.ProcConfigMgmt, meta.Update, 100001, 0),
			model.ActionConfigTemplateEdit, model.ResourceTypeBiz, "100001", "100001"},
		{"模板删除", attr(meta.ProcConfigMgmt, meta.Delete, 100001, 0),
			model.ActionConfigTemplateDelete, model.ResourceTypeBiz, "100001", "100001"},
		{"配置生成", attr(meta.ProcConfigMgmt, meta.GenerateConfig, 100001, 0),
			model.ActionConfigGenerate, model.ResourceTypeBiz, "100001", "100001"},
		{"配置下发", attr(meta.ProcConfigMgmt, meta.ReleaseConfig, 100001, 0),
			model.ActionConfigRelease, model.ResourceTypeBiz, "100001", "100001"},
		{"项目创建", attrInProject(meta.Project, meta.Create, 100001, 40001, 40001),
			model.ActionProjectCreate, model.ResourceTypeBiz, "100001", "100001"},
		{"项目查看", attrInProject(meta.Project, meta.View, 100001, 40001, 40001),
			model.ActionProjectView, model.ResourceTypeProject, "40001", "100001"},
		{"项目编辑", attrInProject(meta.Project, meta.Update, 100001, 40001, 40001),
			model.ActionProjectEdit, model.ResourceTypeProject, "40001", "100001"},
		{"项目删除", attrInProject(meta.Project, meta.Delete, 100001, 40001, 40001),
			model.ActionProjectDelete, model.ResourceTypeProject, "40001", "100001"},
		{"环境创建", attrInProject(meta.Environment, meta.Create, 100001, 40001, 40001),
			model.ActionEnvironmentCreate, model.ResourceTypeProject, "40001", "100001"},
		{"环境查看", attrInProject(meta.Environment, meta.View, 100001, 40001, 50001),
			model.ActionEnvironmentView, model.ResourceTypeEnvironment, "50001", "100001"},
		{"环境编辑", attrInProject(meta.Environment, meta.Update, 100001, 40001, 50001),
			model.ActionEnvironmentEdit, model.ResourceTypeEnvironment, "50001", "100001"},
		{"环境删除", attrInProject(meta.Environment, meta.Delete, 100001, 40001, 50001),
			model.ActionEnvironmentDelete, model.ResourceTypeEnvironment, "50001", "100001"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Adapt(c.attr)
			require.NoError(t, err)
			require.False(t, got.Skip)
			require.Equal(t, c.action, got.ActionID)
			require.Equal(t, c.resType, got.ResourceType)
			require.Equal(t, c.resID, got.ResourceID)
			require.Equal(t, c.bizID, got.BizID)
		})
	}
}

// 映射产出的操作必须都在模型定义里，否则会向权限中心发送不存在的 action。
func TestAdaptProducesRegisteredActions(t *testing.T) {
	registered := make(map[string]string)
	for _, a := range model.Actions() {
		registered[a.ID] = a.ResourceTypeID
	}

	attrs := []*meta.ResourceAttribute{
		attr(meta.Biz, meta.FindBusinessResource, 1, 0),
		attrInProject(meta.App, meta.Create, 1, 2, 3),
		attr(meta.App, meta.View, 1, 2),
		attr(meta.App, meta.Update, 1, 2),
		attr(meta.App, meta.Delete, 1, 2),
		attr(meta.App, meta.GenerateRelease, 1, 2),
		attr(meta.App, meta.Publish, 1, 2),
		attr(meta.Credential, meta.View, 1, 0),
		attr(meta.Credential, meta.Manage, 1, 0),
		attr(meta.Audit, meta.View, 1, 0),
		attr(meta.ProcConfigMgmt, meta.View, 1, 0),
		attr(meta.ProcConfigMgmt, meta.ProcessOperate, 1, 0),
		attr(meta.ProcConfigMgmt, meta.Create, 1, 0),
		attr(meta.ProcConfigMgmt, meta.Update, 1, 0),
		attr(meta.ProcConfigMgmt, meta.Delete, 1, 0),
		attr(meta.ProcConfigMgmt, meta.GenerateConfig, 1, 0),
		attr(meta.ProcConfigMgmt, meta.ReleaseConfig, 1, 0),
		attr(meta.Project, meta.Create, 1, 0),
		attr(meta.Project, meta.View, 1, 0),
		attr(meta.Project, meta.Update, 1, 0),
		attr(meta.Project, meta.Delete, 1, 0),
		attr(meta.Environment, meta.Create, 1, 2),
		attr(meta.Environment, meta.View, 1, 2),
		attr(meta.Environment, meta.Update, 1, 2),
		attr(meta.Environment, meta.Delete, 1, 2),
	}

	for _, a := range attrs {
		got, err := Adapt(a)
		require.NoError(t, err)

		ownType, ok := registered[got.ActionID]
		require.True(t, ok, "action %s is not registered in the model", got.ActionID)

		// 鉴权时使用的资源维度必须是该操作关联的资源类型本身。
		require.Equal(t, ownType, got.ResourceType,
			"action %s is registered on %s but mapped to %s", got.ActionID, ownType, got.ResourceType)
	}
}

func TestAdaptSkip(t *testing.T) {
	// 显式跳过
	got, err := Adapt(attr(meta.App, meta.SkipAction, 1, 2))
	require.NoError(t, err)
	require.True(t, got.Skip)

	// 细粒度资源不单独鉴权
	for _, resType := range []meta.ResourceType{
		meta.Commit, meta.ConfigItem, meta.Content, meta.CRInstance, meta.Release,
		meta.ReleasedCI, meta.Strategy, meta.StrategySet, meta.PSH, meta.Repo, meta.Sidecar,
	} {
		got, err := Adapt(attr(resType, meta.View, 1, 2))
		require.NoError(t, err, "type %s", resType)
		require.True(t, got.Skip, "type %s should be skipped", resType)
	}
}

func TestAdaptRejectsInvalidInput(t *testing.T) {
	_, err := Adapt(nil)
	require.ErrorContains(t, err, "not set")

	_, err = Adapt(attr("unknown_type", meta.View, 1, 2))
	require.ErrorContains(t, err, "unsupported bscp auth type")

	_, err = Adapt(attr(meta.Biz, meta.Publish, 1, 0))
	require.ErrorContains(t, err, "unsupported bscp action for biz")

	_, err = Adapt(attr(meta.App, meta.Manage, 1, 2))
	require.ErrorContains(t, err, "unsupported bscp action for app")

	_, err = Adapt(attr(meta.Credential, meta.Delete, 1, 0))
	require.ErrorContains(t, err, "unsupported bscp action for credential")

	_, err = Adapt(attr(meta.Audit, meta.Delete, 1, 0))
	require.ErrorContains(t, err, "unsupported bscp action for audit")

	_, err = Adapt(attr(meta.ProcConfigMgmt, meta.Publish, 1, 0))
	require.ErrorContains(t, err, "unsupported bscp action for process config management")

	_, err = Adapt(attr(meta.Project, meta.Publish, 1, 0))
	require.ErrorContains(t, err, "unsupported bscp action for project")

	_, err = Adapt(attr(meta.Environment, meta.Publish, 1, 2))
	require.ErrorContains(t, err, "unsupported bscp action for environment")
}

// 服务创建按目标环境授权（此时还没有服务实例），路径只到项目一级：
// 业务与项目维度的授权据此覆盖到该环境。
func TestAppCreateIsAuthorizedOnTargetEnvironment(t *testing.T) {
	got, err := Adapt(attrInProject(meta.App, meta.Create, 100001, 40001, 50001))
	require.NoError(t, err)
	require.Equal(t, model.ActionAppCreate, got.ActionID)
	require.Equal(t, model.ResourceTypeEnvironment, got.ResourceType)
	require.Equal(t, "50001", got.ResourceID)

	res := got.AuthResource()
	require.NotNil(t, res)
	require.Equal(t, "50001", res.ID)
	require.Equal(t, "/biz,100001/project,40001/", res.Attributes[client.IAMPathAttrKey])

	applyRes := got.ApplyResource()
	require.Len(t, applyRes, 1)
	require.Equal(t, model.ResourceTypeEnvironment, applyRes[0].Type)
}

// 服务挂在环境下，鉴权路径必须是 业务 → 项目 → 环境 三级，
// 项目/环境维度的授权才能覆盖到该服务。
func TestAuthResourceCarriesTopologyForApp(t *testing.T) {
	got, err := Adapt(attrInEnv(meta.View, 100001, 40001, 50001, 30001))
	require.NoError(t, err)

	res := got.AuthResource()
	require.NotNil(t, res)
	require.Equal(t, "30001", res.ID)
	require.Equal(t, "/biz,100001/project,40001/environment,50001/", res.Attributes[client.IAMPathAttrKey])
}

// 服务缺少项目或环境归属时三级路径不完整，必须退化为无 attributes（即无权限）。
func TestAppWithoutEnvironmentHasNoPath(t *testing.T) {
	got, err := Adapt(attr(meta.App, meta.View, 100001, 30001))
	require.NoError(t, err)

	res := got.AuthResource()
	require.NotNil(t, res)
	require.Nil(t, res.Attributes, "incomplete path must be dropped")
}

// biz 是拓扑根节点，不带 attributes。
func TestAuthResourceHasNoTopologyForBiz(t *testing.T) {
	got, err := Adapt(attr(meta.Biz, meta.FindBusinessResource, 100001, 0))
	require.NoError(t, err)

	res := got.AuthResource()
	require.NotNil(t, res)
	require.Equal(t, "100001", res.ID)
	require.Nil(t, res.Attributes)
}

func TestAuthResourceNilWhenSkipped(t *testing.T) {
	got, err := Adapt(attr(meta.ConfigItem, meta.View, 1, 2))
	require.NoError(t, err)
	require.Nil(t, got.AuthResource())
	require.Nil(t, got.ApplyResource())
}

// 申请链接用 ancestors 数组表达拓扑，而不是 attributes 里的路径字符串。
func TestApplyResourceUsesAncestors(t *testing.T) {
	appMapped, err := Adapt(attrInEnv(meta.Update, 100001, 40001, 50001, 30001))
	require.NoError(t, err)

	appRes := appMapped.ApplyResource()
	require.Len(t, appRes, 1)
	require.Equal(t, "30001", appRes[0].ID)
	require.Equal(t, model.ResourceTypeApp, appRes[0].Type)
	require.Equal(t, []client.ResourceRef{
		{Type: model.ResourceTypeBiz, ID: "100001"},
		{Type: model.ResourceTypeProject, ID: "40001"},
		{Type: model.ResourceTypeEnvironment, ID: "50001"},
	}, appRes[0].Ancestors)

	bizMapped, err := Adapt(attr(meta.Credential, meta.Manage, 100001, 0))
	require.NoError(t, err)

	bizRes := bizMapped.ApplyResource()
	require.Len(t, bizRes, 1)
	require.Equal(t, "100001", bizRes[0].ID)
	require.Equal(t, model.ResourceTypeBiz, bizRes[0].Type)
	require.Empty(t, bizRes[0].Ancestors)
}

// 环境按环境实例鉴权，路径带上业务与项目两级，项目/业务维度的授权才能覆盖到环境。
func TestAuthResourceForEnvironment(t *testing.T) {
	got, err := Adapt(attrInProject(meta.Environment, meta.Update, 100001, 40001, 50001))
	require.NoError(t, err)
	require.False(t, got.Skip)
	require.Equal(t, model.ActionEnvironmentEdit, got.ActionID)
	require.Equal(t, model.ResourceTypeEnvironment, got.ResourceType)

	res := got.AuthResource()
	require.NotNil(t, res)
	require.Equal(t, "50001", res.ID)
	require.Equal(t, "/biz,100001/project,40001/", res.Attributes[client.IAMPathAttrKey])

	applied := got.ApplyResource()
	require.Len(t, applied, 1)
	require.Equal(t, "50001", applied[0].ID)
	require.Equal(t, model.ResourceTypeEnvironment, applied[0].Type)
	require.Equal(t, []client.ResourceRef{
		{Type: model.ResourceTypeBiz, ID: "100001"},
		{Type: model.ResourceTypeProject, ID: "40001"},
	}, applied[0].Ancestors)
}

// 项目按项目实例鉴权，路径只到业务一层；申请链接只回显业务这一级祖先。
func TestAuthResourceForProject(t *testing.T) {
	got, err := Adapt(attrInProject(meta.Project, meta.View, 100001, 40001, 40001))
	require.NoError(t, err)
	require.Equal(t, model.ActionProjectView, got.ActionID)
	require.Equal(t, model.ResourceTypeProject, got.ResourceType)

	res := got.AuthResource()
	require.NotNil(t, res)
	require.Equal(t, "40001", res.ID)
	require.Equal(t, "/biz,100001/", res.Attributes[client.IAMPathAttrKey])

	applied := got.ApplyResource()
	require.Equal(t, []client.ResourceRef{
		{Type: model.ResourceTypeBiz, ID: "100001"},
	}, applied[0].Ancestors)
}

// 环境创建时还没有环境实例，按所属项目授权。
func TestEnvironmentCreateIsProjectScoped(t *testing.T) {
	got, err := Adapt(attrInProject(meta.Environment, meta.Create, 100001, 40001, 40001))
	require.NoError(t, err)
	require.Equal(t, model.ActionEnvironmentCreate, got.ActionID)
	require.Equal(t, model.ResourceTypeProject, got.ResourceType)
	require.Equal(t, "40001", got.ResourceID)
	require.Equal(t, "/biz,100001/", got.AuthResource().Attributes[client.IAMPathAttrKey])
}

// 缺少项目 ID 时环境路径不完整，不能带着残缺路径去鉴权，必须退化为无 attributes（即无权限）。
func TestEnvironmentWithoutProjectIDHasNoPath(t *testing.T) {
	got, err := Adapt(attr(meta.Environment, meta.View, 100001, 50001))
	require.NoError(t, err)
	require.Nil(t, got.AuthResource().Attributes)
}
