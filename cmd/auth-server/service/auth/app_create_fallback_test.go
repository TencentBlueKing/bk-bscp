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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/cmd/auth-server/options"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	iamauth "github.com/TencentBlueKing/bk-bscp/pkg/iam/sdk/auth"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	pbas "github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server"
	pbapp "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/app"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// fakeV3Authorizer 记录发往权限中心 V3 的批量鉴权请求，按「操作|资源类型|实例」决定是否放行。
type fakeV3Authorizer struct {
	iamauth.Authorizer

	batches [][]*client.AuthBatch
	allow   map[string]bool
}

func (f *fakeV3Authorizer) AuthorizeBatch(_ context.Context, opts *client.AuthBatchOptions) (
	[]*client.Decision, error) {

	f.batches = append(f.batches, opts.Batch)

	decisions := make([]*client.Decision, len(opts.Batch))
	for i, b := range opts.Batch {
		res := b.Resources[0]
		decisions[i] = &client.Decision{Authorized: f.allow[b.Action.ID+"|"+string(res.Type)+"|"+res.ID]}
	}

	return decisions, nil
}

func newV3Auth(topology meta.ResourceTopology, authorizer *fakeV3Authorizer, ds *fakeAppDS) *Auth {
	return &Auth{
		auth:            authorizer,
		ds:              ds,
		projectEnvAuth:  false,
		topology:        topology,
		disableWriteOpt: &options.DisableWriteOption{},
		appScopes:       newAppScopeCache(),
		defaultScopes:   newDefaultScopeCache(),
	}
}

// 环境 50001 的真实归属是业务 100001 / 项目 40001。
func appCreateEnvDS() *fakeAppDS {
	return &fakeAppDS{
		environmentInfos: map[uint32]*pbds.InstanceInfo{
			50001: {Id: "50001", Path: []string{"/biz,100001/project,40001/"}},
		},
		apps: map[uint32]*pbapp.App{
			30001: {Id: 30001, BizId: 100001, ProjectId: 40001, EnvId: 50001},
		},
	}
}

func appCreateReq(bizID uint32, extra ...*meta.ResourceAttribute) *pbas.AuthorizeBatchReq {
	resources := append(extra, &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.App, Action: meta.Create, ResourceID: 50001},
		BizID: bizID,
	})

	return &pbas.AuthorizeBatchReq{
		User:      pbas.PbUserInfo(&meta.UserInfo{UserName: "tester"}),
		Resources: pbas.PbResourceAttributes(resources),
	}
}

// 存量授权都在业务维度的 app_create 上：按环境未授权时，回退按业务校验 app_create，
// 业务取目标环境的真实归属，请求里伪造的业务不参与。
func TestAuthorizeBatchAppCreateFallsBackToBusiness(t *testing.T) {
	authorizer := &fakeV3Authorizer{allow: map[string]bool{
		"app_create|biz|100001": true,
		"app_create|biz|200002": true,
	}}
	a := newV3Auth(meta.TopologyHierarchical, authorizer, appCreateEnvDS())

	resp, err := a.AuthorizeBatch(authBatchCtx("tester"), appCreateReq(200002))
	require.NoError(t, err)
	require.Len(t, resp.Decisions, 1)
	require.True(t, resp.Decisions[0].Authorized)

	require.Len(t, authorizer.batches, 2)
	require.Equal(t, string(sys.EnvironmentAppCreate), authorizer.batches[0][0].Action.ID)
	require.Equal(t, sys.Environment, authorizer.batches[0][0].Resources[0].Type)
	require.Equal(t, "50001", authorizer.batches[0][0].Resources[0].ID)

	require.Len(t, authorizer.batches[1], 1)
	fallback := authorizer.batches[1][0]
	require.Equal(t, string(sys.AppCreate), fallback.Action.ID)
	require.Equal(t, []client.Resource{{System: sys.SystemIDCMDB, Type: sys.Business, ID: "100001"}},
		fallback.Resources)
}

// 按环境与按业务都没有授权时，服务创建保持无权限。
func TestAuthorizeBatchAppCreateDeniedWithoutAnyGrant(t *testing.T) {
	authorizer := &fakeV3Authorizer{}
	a := newV3Auth(meta.TopologyHierarchical, authorizer, appCreateEnvDS())

	resp, err := a.AuthorizeBatch(authBatchCtx("tester"), appCreateReq(100001))
	require.NoError(t, err)
	require.False(t, resp.Decisions[0].Authorized)
	require.Len(t, authorizer.batches, 2)
}

// 按环境已授权时不再发起业务维度的校验。
func TestAuthorizeBatchAppCreateGrantedByEnvironmentSkipsFallback(t *testing.T) {
	authorizer := &fakeV3Authorizer{allow: map[string]bool{
		"environment_app_create|environment|50001": true,
	}}
	a := newV3Auth(meta.TopologyHierarchical, authorizer, appCreateEnvDS())

	resp, err := a.AuthorizeBatch(authBatchCtx("tester"), appCreateReq(100001))
	require.NoError(t, err)
	require.True(t, resp.Decisions[0].Authorized)
	require.Len(t, authorizer.batches, 1)
}

// 目标环境回查不到归属时业务被清空，不能回退到任何业务的授权上。
func TestAuthorizeBatchAppCreateUnknownEnvironmentSkipsFallback(t *testing.T) {
	authorizer := &fakeV3Authorizer{allow: map[string]bool{"app_create|biz|100001": true}}
	a := newV3Auth(meta.TopologyHierarchical, authorizer, &fakeAppDS{})

	resp, err := a.AuthorizeBatch(authBatchCtx("tester"), appCreateReq(100001))
	require.NoError(t, err)
	require.False(t, resp.Decisions[0].Authorized)
	require.Len(t, authorizer.batches, 1)
}

// 旧拓扑下服务创建本身就按业务校验 app_create，只发起一次。
func TestAuthorizeBatchAppCreateLegacyChecksBusinessOnce(t *testing.T) {
	authorizer := &fakeV3Authorizer{}
	a := newV3Auth(meta.TopologyLegacy, authorizer, appCreateEnvDS())

	resp, err := a.AuthorizeBatch(authBatchCtx("tester"), appCreateReq(100001))
	require.NoError(t, err)
	require.False(t, resp.Decisions[0].Authorized)
	require.Len(t, authorizer.batches, 1)
	require.Equal(t, string(sys.AppCreate), authorizer.batches[0][0].Action.ID)
	require.Equal(t, sys.Business, authorizer.batches[0][0].Resources[0].Type)
}

// 混合批次里只有服务创建参与回退，其他资源的判定与顺序保持不变。
func TestAuthorizeBatchAppCreateFallbackKeepsOtherDecisions(t *testing.T) {
	authorizer := &fakeV3Authorizer{allow: map[string]bool{
		"find_business_resource|biz|100001": true,
		"app_create|biz|100001":             true,
	}}
	a := newV3Auth(meta.TopologyHierarchical, authorizer, appCreateEnvDS())

	req := appCreateReq(100001,
		&meta.ResourceAttribute{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: 100001},
		&meta.ResourceAttribute{Basic: meta.Basic{Type: meta.App, Action: meta.View, ResourceID: 30001}, BizID: 100001},
	)

	resp, err := a.AuthorizeBatch(authBatchCtx("tester"), req)
	require.NoError(t, err)
	require.Len(t, resp.Decisions, 3)
	require.True(t, resp.Decisions[0].Authorized, "business view")
	require.False(t, resp.Decisions[1].Authorized, "app view")
	require.True(t, resp.Decisions[2].Authorized, "app create")

	require.Len(t, authorizer.batches, 2)
	require.Len(t, authorizer.batches[1], 1)
	require.Equal(t, string(sys.AppCreate), authorizer.batches[1][0].Action.ID)
}
