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

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	sdkauth "github.com/TencentBlueKing/bk-bscp/pkg/iam/sdk/auth"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	authv4 "github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/auth"
)

// newCreatorGrantAuth 构造走 V4 分发、开关与拓扑取值可控的 Auth。
func newCreatorGrantAuth(t *testing.T, projectEnvAuth bool, topology meta.ResourceTopology, gw v4Gateway) *Auth {
	t.Helper()

	authorizer, err := authv4.NewAuthorizer(&fakeAuthzV4Gateway{}, authv4.Config{})
	require.NoError(t, err)

	return &Auth{
		v4:             authorizer,
		v4Cli:          gw,
		projectEnvAuth: projectEnvAuth,
		topology:       topology,
	}
}

// fakeV3CreatorAuthorizer 只记录 V3 创建者授权调用，其余方法由内嵌接口兜底。
type fakeV3CreatorAuthorizer struct {
	sdkauth.Authorizer
	calls int
	opted *client.GrantResourceCreatorActionOption
}

func (f *fakeV3CreatorAuthorizer) GrantResourceCreatorAction(_ context.Context,
	opts *client.GrantResourceCreatorActionOption) error {

	f.calls++
	f.opted = opts

	return nil
}

// creatorContext 构造带操作人与租户的入站上下文，供 kit.FromGrpcContext 解析。
func creatorContext(user string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		strings.ToLower(constant.UserKey), user,
		strings.ToLower(constant.BkTenantID), "default",
	))
}

func creatorGrantOption(resourceType client.TypeID, id string) *client.GrantResourceCreatorActionOption {
	return &client.GrantResourceCreatorActionOption{
		System:  sys.SystemIDBSCP,
		Type:    resourceType,
		ID:      id,
		Name:    "Default",
		Creator: "alice",
		Ancestors: []client.GrantResourceCreatorActionAncestor{
			{
				System: sys.SystemIDCMDB,
				Type:   sys.Business,
				ID:     "2",
			},
			{
				System: sys.SystemIDBSCP,
				Type:   sys.Project,
				ID:     "40001",
			},
		},
	}
}

// 旧拓扑下权限中心没有项目/环境资源类型，也没有对应的创建者角色，授权请求一定失败，
// 因此必须跳过，且判断只取拓扑，与开关取值无关。V3、V4 表现一致。
func TestGrantCreatorActionSkipsProjectAndEnvironmentInLegacyTopology(t *testing.T) {
	ctx := creatorContext("alice")

	for _, projectEnvAuth := range []bool{false, true} {
		for _, resourceType := range []client.TypeID{sys.Project, sys.Environment} {
			t.Run(string(resourceType), func(t *testing.T) {
				gw := new(fakeV4Gateway)
				v4Auth := newCreatorGrantAuth(t, projectEnvAuth, meta.TopologyLegacy, gw)

				require.NoError(t, v4Auth.GrantResourceCreatorAction(
					ctx, creatorGrantOption(resourceType, "40001")))
				require.Zero(t, gw.calls, "v4 should skip %s creator grant", resourceType)

				v3 := new(fakeV3CreatorAuthorizer)
				v3Auth := &Auth{auth: v3, projectEnvAuth: projectEnvAuth, topology: meta.TopologyLegacy}

				require.NoError(t, v3Auth.GrantResourceCreatorAction(
					ctx, creatorGrantOption(resourceType, "40001")))
				require.Zero(t, v3.calls, "v3 should skip %s creator grant", resourceType)
			})
		}
	}
}

// 新拓扑下模型已注册，即使开关关闭也要写创建者策略：否则关闭期间创建的项目和环境，
// 在开关打开后创建者对自己的资源全部没有权限。V3、V4 表现一致。
func TestGrantCreatorActionWritesProjectAndEnvironmentInHierarchicalTopology(t *testing.T) {
	ctx := creatorContext("alice")

	for _, resourceType := range []client.TypeID{sys.Project, sys.Environment} {
		t.Run(string(resourceType), func(t *testing.T) {
			gw := new(fakeV4Gateway)
			v4Auth := newCreatorGrantAuth(t, false, meta.TopologyHierarchical, gw)

			require.NoError(t, v4Auth.GrantResourceCreatorAction(
				ctx, creatorGrantOption(resourceType, "40001")))
			require.Equal(t, 1, gw.calls, "v4 should grant %s creator action", resourceType)
			require.Len(t, gw.items, 2)

			v3 := new(fakeV3CreatorAuthorizer)
			v3Auth := &Auth{auth: v3, projectEnvAuth: false, topology: meta.TopologyHierarchical}

			require.NoError(t, v3Auth.GrantResourceCreatorAction(
				ctx, creatorGrantOption(resourceType, "40001")))
			require.Equal(t, 1, v3.calls, "v3 should grant %s creator action", resourceType)
		})
	}
}

// 开关开启（必然要求新拓扑）时项目/环境仍需下发创建者授权：项目创建者要拿到项目权限，
// 默认环境还必须单独拿到环境实例上的权限。
func TestGrantCreatorActionKeepsProjectAndEnvironmentWhenEnabled(t *testing.T) {
	ctx := creatorContext("alice")

	gw := new(fakeV4Gateway)
	v4Auth := newCreatorGrantAuth(t, true, meta.TopologyHierarchical, gw)

	require.NoError(t, v4Auth.GrantResourceCreatorAction(
		ctx, creatorGrantOption(sys.Environment, "50001")))
	require.Equal(t, 1, gw.calls)
	// 环境维度 + 业务维度两条，缺业务维度时业务访问前置校验会拦住后续操作。
	require.Len(t, gw.items, 2)
	require.Equal(t, "alice", gw.operator)

	scopes := make(map[string]string, len(gw.items))
	for _, item := range gw.items {
		scopes[item.RelatedResourceTypeID] = item.Resources[0].ID
	}
	require.Equal(t, "50001", scopes["environment"])
	require.Equal(t, "2", scopes["biz"])
}

// 服务创建者授权只按拓扑判断，与开关无关：新拓扑下 app_operator 里的
// project_view / environment_view 必须按服务所属项目、环境各授一次，
// 否则创建者对自己新建的服务改不了配置项与 KV。
func TestGrantCreatorActionKeepsApplicationScopesInHierarchicalTopology(t *testing.T) {
	gw := new(fakeV4Gateway)
	a := newCreatorGrantAuth(t, false, meta.TopologyHierarchical, gw)

	require.NoError(t, a.GrantResourceCreatorAction(
		creatorContext("alice"), creatorOption("30001", "2", "alice")))
	require.Equal(t, 1, gw.calls)
	require.NotEmpty(t, gw.items)

	scopes := make(map[string]struct{}, len(gw.items))
	for _, item := range gw.items {
		scopes[item.RelatedResourceTypeID] = struct{}{}
	}
	require.Contains(t, scopes, "project")
	require.Contains(t, scopes, "environment")
}
