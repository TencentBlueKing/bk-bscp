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

// disabled 是关闭项目/环境维度校验时的映射选项。
var disabled = Options{ProjectEnvironmentAuth: false}

// 关闭项目/环境维度校验后，这两类资源不进入权限中心，直接放行。
func TestAdaptDisabledSkipsProjectAndEnvironment(t *testing.T) {
	cases := []*meta.ResourceAttribute{
		attrInProject(meta.Project, meta.Create, 100001, 40001, 40001),
		attrInProject(meta.Project, meta.View, 100001, 40001, 40001),
		attrInProject(meta.Project, meta.Update, 100001, 40001, 40001),
		attrInProject(meta.Project, meta.Delete, 100001, 40001, 40001),
		attrInProject(meta.Environment, meta.Create, 100001, 40001, 40001),
		attrInProject(meta.Environment, meta.View, 100001, 40001, 50001),
		attrInProject(meta.Environment, meta.Update, 100001, 40001, 50001),
		attrInProject(meta.Environment, meta.Delete, 100001, 40001, 50001),
	}

	for _, a := range cases {
		got, err := AdaptWithOptions(a, disabled)
		require.NoError(t, err, "type %s/%s", a.Type, a.Action)
		require.True(t, got.Skip, "type %s/%s should be skipped", a.Type, a.Action)
		require.Nil(t, got.AuthResource())
		require.Nil(t, got.ApplyResource())
	}
}

// 关闭项目/环境维度校验后，服务创建退回按业务授权，与改造前一致。
func TestAdaptDisabledAppCreateKeepsBizDimension(t *testing.T) {
	got, err := AdaptWithOptions(attrInEnv(meta.Create, 100001, 40001, 50001, 50001), disabled)
	require.NoError(t, err)

	// 关闭项目/环境维度后仍校验服务创建：资源实例仍是目标环境（模型约束），
	// 但路径只声明业务一层。
	require.False(t, got.Skip)
	require.Equal(t, model.ActionAppCreate, got.ActionID)
	require.Equal(t, model.ResourceTypeEnvironment, got.ResourceType)
	require.Equal(t, "50001", got.ResourceID)
	require.Empty(t, got.ProjectID)
	require.Equal(t, "/biz,100001/", got.AuthResource().Attributes[client.IAMPathAttrKey])

	applied := got.ApplyResource()
	require.Len(t, applied, 1)
	require.Equal(t, []client.ResourceRef{
		{Type: model.ResourceTypeBiz, ID: "100001"},
	}, applied[0].Ancestors)
}

// 没有目标环境实例时无法构造合法的申请项，鉴权仍按业务维度发起。
func TestAdaptDisabledAppCreateWithoutEnv(t *testing.T) {
	a := &meta.ResourceAttribute{Basic: meta.Basic{Type: meta.App, Action: meta.Create}, BizID: 100001}

	got, err := AdaptWithOptions(a, disabled)
	require.NoError(t, err)
	require.False(t, got.Skip)
	require.Nil(t, got.ApplyResource())
	require.Equal(t, "/biz,100001/", got.AuthResource().Attributes[client.IAMPathAttrKey])
}

// 关闭项目/环境维度校验后，服务鉴权仍针对服务实例，但顶层路径只声明业务一层。
func TestAdaptDisabledAppUsesBizOnlyPath(t *testing.T) {
	cases := []struct {
		action meta.Action
		want   string
	}{
		{meta.View, model.ActionAppView},
		{meta.Update, model.ActionAppEdit},
		{meta.Delete, model.ActionAppDelete},
		{meta.GenerateRelease, model.ActionReleaseGenerate},
		{meta.Publish, model.ActionReleasePublish},
	}

	for _, c := range cases {
		got, err := AdaptWithOptions(attrInEnv(c.action, 100001, 40001, 50001, 30001), disabled)
		require.NoError(t, err, "action %s", c.action)
		require.False(t, got.Skip)
		require.Equal(t, c.want, got.ActionID)
		require.Equal(t, model.ResourceTypeApp, got.ResourceType)
		require.Equal(t, "30001", got.ResourceID)

		// 项目/环境不参与鉴权，映射结果里也不应保留它们的 ID。
		require.Empty(t, got.ProjectID)
		require.Empty(t, got.EnvironmentID)

		res := got.AuthResource()
		require.NotNil(t, res)
		require.Equal(t, "/biz,100001/", res.Attributes[client.IAMPathAttrKey])
	}
}

// 关闭项目/环境维度校验后，申请链接同样只回显业务一层祖先。
func TestAdaptDisabledApplyResourceBizOnlyAncestors(t *testing.T) {
	got, err := AdaptWithOptions(attrInEnv(meta.Update, 100001, 40001, 50001, 30001), disabled)
	require.NoError(t, err)

	applied := got.ApplyResource()
	require.Len(t, applied, 1)
	require.Equal(t, "30001", applied[0].ID)
	require.Equal(t, model.ResourceTypeApp, applied[0].Type)
	require.Equal(t, []client.ResourceRef{
		{Type: model.ResourceTypeBiz, ID: "100001"},
	}, applied[0].Ancestors)
}

// 开关只影响项目/环境维度，业务与其他资源的映射保持不变。
func TestAdaptDisabledKeepsOtherResources(t *testing.T) {
	got, err := AdaptWithOptions(attr(meta.Biz, meta.FindBusinessResource, 100001, 0), disabled)
	require.NoError(t, err)
	require.False(t, got.Skip)
	require.Equal(t, model.ActionFindBusinessResource, got.ActionID)
	require.Equal(t, model.ResourceTypeBiz, got.ResourceType)

	cred, err := AdaptWithOptions(attr(meta.Credential, meta.Manage, 100001, 0), disabled)
	require.NoError(t, err)
	require.False(t, cred.Skip)
	require.Equal(t, model.ActionAppCredentialManage, cred.ActionID)

	// 显式跳过与细粒度资源的既有行为不受影响。
	skip, err := AdaptWithOptions(attr(meta.App, meta.SkipAction, 1, 2), disabled)
	require.NoError(t, err)
	require.True(t, skip.Skip)

	ci, err := AdaptWithOptions(attr(meta.ConfigItem, meta.View, 1, 2), disabled)
	require.NoError(t, err)
	require.True(t, ci.Skip)
}

// 开启项目/环境维度校验时保持三级拓扑，确保开关不会改变默认行为。
func TestAdaptEnabledKeepsThreeLevelPath(t *testing.T) {
	got, err := AdaptWithOptions(attrInEnv(meta.View, 100001, 40001, 50001, 30001),
		Options{ProjectEnvironmentAuth: true})
	require.NoError(t, err)
	require.Equal(t,
		"/biz,100001/project,40001/environment,50001/",
		got.AuthResource().Attributes[client.IAMPathAttrKey])
}
