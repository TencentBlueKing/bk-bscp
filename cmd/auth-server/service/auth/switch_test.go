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
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/TencentBlueKing/bk-bscp/cmd/auth-server/options"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	authv4 "github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/auth"
	clientv4 "github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/model"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbas "github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server"
	pbapp "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/app"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// fakeAuthzV4Gateway 记录 V4 鉴权请求，便于断言开关是否真正改变了发往权限中心的资源。
type fakeAuthzV4Gateway struct {
	calls     int
	actionIDs []string
	resources [][]clientv4.AuthResource
	allow     map[string]bool
}

func (f *fakeAuthzV4Gateway) AuthByResources(_ *kit.Kit, _ clientv4.Subject, actionID string,
	resources []clientv4.AuthResource) (map[string]bool, error) {

	f.calls++
	f.actionIDs = append(f.actionIDs, actionID)
	f.resources = append(f.resources, resources)

	got := make(map[string]bool, len(resources))
	for _, r := range resources {
		got[r.ID] = f.allow[actionID+"|"+r.ID]
	}

	return got, nil
}

// newSwitchAuth 构造一个走 V4 鉴权、开关取值可控的 Auth。
func newSwitchAuth(t *testing.T, projectEnvAuth bool, gw *fakeAuthzV4Gateway, ds *fakeAppDS) *Auth {
	t.Helper()

	a := &Auth{
		ds:              ds,
		projectEnvAuth:  projectEnvAuth,
		disableWriteOpt: &options.DisableWriteOption{},
		appScopes:       newAppScopeCache(),
		defaultScopes:   newDefaultScopeCache(),
	}

	authorizer, err := authv4.NewAuthorizer(gw, authv4.Config{})
	require.NoError(t, err)
	a.v4 = authorizer

	return a
}

// authBatchCtx 构造带登录用户信息、可被 kit.FromGrpcContext 解析的上下文。
func authBatchCtx(user string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		strings.ToLower(constant.UserKey), user,
	))
}

func appViewAndProjectViewReq() *pbas.AuthorizeBatchReq {
	return &pbas.AuthorizeBatchReq{
		User: pbas.PbUserInfo(&meta.UserInfo{UserName: "tester"}),
		Resources: pbas.PbResourceAttributes([]*meta.ResourceAttribute{
			{Basic: meta.Basic{Type: meta.Project, Action: meta.View, ResourceID: 40001}, BizID: 100001},
			{Basic: meta.Basic{Type: meta.App, Action: meta.View, ResourceID: 30001}, BizID: 100001},
		}),
	}
}

// V4 不读取项目/环境维度开关：即使配置关闭，项目仍按实例鉴权，服务路径仍为三级。
func TestAuthorizeBatchV4IgnoresDisabledSwitch(t *testing.T) {
	gw := &fakeAuthzV4Gateway{allow: map[string]bool{
		"app_view|30001":     true,
		"project_view|40001": true,
	}}
	ds := &fakeAppDS{apps: map[uint32]*pbapp.App{
		30001: {Id: 30001, BizId: 100001, ProjectId: 40001, EnvId: 50001},
	}, projectInfos: map[uint32]*pbds.InstanceInfo{
		40001: {Id: "40001", Path: []string{"/biz,100001/"}},
	}}
	a := newSwitchAuth(t, false, gw, ds)

	resp, err := a.AuthorizeBatch(authBatchCtx("tester"), appViewAndProjectViewReq())
	require.NoError(t, err)
	require.Len(t, resp.Decisions, 2)
	require.True(t, resp.Decisions[0].Authorized)
	require.True(t, resp.Decisions[1].Authorized)

	// 项目与服务各自向权限中心发问，项目没有被跳过。
	require.Equal(t, 2, gw.calls)
	require.Contains(t, gw.actionIDs, model.ActionProjectView)

	for i, actionID := range gw.actionIDs {
		if actionID == model.ActionAppView {
			require.Equal(t, "/biz,100001/project,40001/environment,50001/",
				gw.resources[i][0].Attributes[clientv4.IAMPathAttrKey])
		}
	}
}

// V4 只有环境级的服务创建：即使配置关闭开关，也按目标环境实例发问，
// 项目祖先取自目标环境的真实路径。
func TestAuthorizeBatchV4AppCreateAlwaysOnEnvironment(t *testing.T) {
	gw := &fakeAuthzV4Gateway{allow: map[string]bool{"app_create|50002": true}}
	ds := &fakeAppDS{environmentInfos: map[uint32]*pbds.InstanceInfo{
		50002: {Id: "50002", Path: []string{"/biz,100001/project,40002/"}},
	}}
	a := newSwitchAuth(t, false, gw, ds)

	resp, err := a.AuthorizeBatch(authBatchCtx("tester"), &pbas.AuthorizeBatchReq{
		User: pbas.PbUserInfo(&meta.UserInfo{UserName: "tester"}),
		Resources: pbas.PbResourceAttributes([]*meta.ResourceAttribute{
			{
				Basic:     meta.Basic{Type: meta.App, Action: meta.Create, ResourceID: 50002},
				BizID:     100001,
				ProjectID: 40001,
			},
		}),
	})
	require.NoError(t, err)
	require.Len(t, resp.Decisions, 1)
	require.True(t, resp.Decisions[0].Authorized)

	require.Equal(t, 1, ds.environmentCalls, "目标环境的父级必须回查")
	require.Equal(t, 1, gw.calls)
	require.Equal(t, model.ActionAppCreate, gw.actionIDs[0])
	require.Equal(t, "50002", gw.resources[0][0].ID)
	require.Equal(t, "/biz,100001/project,40002/", gw.resources[0][0].Attributes[clientv4.IAMPathAttrKey])
}

// 资源项缺少 basic 时按参数错误返回，而不是落到 nil pointer panic。
func TestAuthorizeBatchRejectsResourceWithoutBasic(t *testing.T) {
	a := &Auth{disableWriteOpt: &options.DisableWriteOption{}}

	_, err := a.AuthorizeBatch(authBatchCtx("tester"), &pbas.AuthorizeBatchReq{
		User:      pbas.PbUserInfo(&meta.UserInfo{UserName: "tester"}),
		Resources: []*pbas.ResourceAttribute{{BizId: 2}},
	})
	require.ErrorContains(t, err, "basic is required")
}

// 开启项目/环境维度后，项目与服务各自按实例维度校验，路径为完整三级拓扑。
func TestAuthorizeBatchEnabledAuthorizesProjectAndEnvironment(t *testing.T) {
	gw := &fakeAuthzV4Gateway{allow: map[string]bool{
		"app_view|30001":     true,
		"project_view|40001": true,
	}}
	ds := &fakeAppDS{apps: map[uint32]*pbapp.App{
		30001: {Id: 30001, BizId: 100001, ProjectId: 40001, EnvId: 50001},
	}}
	a := newSwitchAuth(t, true, gw, ds)

	resp, err := a.AuthorizeBatch(authBatchCtx("tester"), appViewAndProjectViewReq())
	require.NoError(t, err)
	require.Len(t, resp.Decisions, 2)
	require.True(t, resp.Decisions[0].Authorized)
	require.True(t, resp.Decisions[1].Authorized)

	// 项目与服务分别发问。
	require.Equal(t, 2, gw.calls)

	var appResources []clientv4.AuthResource
	for i, actionID := range gw.actionIDs {
		if actionID == model.ActionAppView {
			appResources = gw.resources[i]
		}
	}
	require.Len(t, appResources, 1)
	require.Equal(t,
		"/biz,100001/project,40001/environment,50001/",
		appResources[0].Attributes[clientv4.IAMPathAttrKey])
}
