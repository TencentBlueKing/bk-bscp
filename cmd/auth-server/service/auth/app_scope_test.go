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
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	pbapp "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/app"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// fakeAppDS 提供服务归属、环境路径与默认归属查询，其余方法由嵌入的接口兜底。
type fakeAppDS struct {
	pbds.DataClient

	apps             map[uint32]*pbapp.App
	calls            int
	err              error
	environmentInfos map[uint32]*pbds.InstanceInfo
	environmentCalls int
	environmentErr   error

	// defaults 是业务 ID 到默认项目/环境的映射
	defaults    map[uint32]*pbds.EnsureDefaultProjectEnvResp
	ensureCalls int
}

func (f *fakeAppDS) GetAppByID(_ context.Context, in *pbds.GetAppByIDReq,
	_ ...grpc.CallOption) (*pbapp.App, error) {

	f.calls++
	if f.err != nil {
		return nil, f.err
	}

	app, ok := f.apps[in.AppId]
	if !ok {
		return nil, context.Canceled
	}

	return app, nil
}

func (f *fakeAppDS) EnsureDefaultProjectEnv(_ context.Context, in *pbds.EnsureDefaultProjectEnvReq,
	_ ...grpc.CallOption) (*pbds.EnsureDefaultProjectEnvResp, error) {

	f.ensureCalls++
	if resp, ok := f.defaults[in.BizId]; ok {
		return resp, nil
	}

	return &pbds.EnsureDefaultProjectEnvResp{}, nil
}

func (f *fakeAppDS) FetchInstanceInfo(_ context.Context, in *pbds.FetchInstanceInfoReq,
	_ ...grpc.CallOption) (*pbds.FetchInstanceInfoResp, error) {
	f.environmentCalls++
	if f.environmentErr != nil {
		return nil, f.environmentErr
	}

	resp := &pbds.FetchInstanceInfoResp{}
	for _, id := range in.GetIds() {
		envID, err := strconv.ParseUint(id, 10, 32)
		if err != nil {
			continue
		}
		if info, ok := f.environmentInfos[uint32(envID)]; ok {
			resp.Details = append(resp.Details, info)
		}
	}
	return resp, nil
}

func newAppScopeAuth(ds pbds.DataClient) *Auth {
	return &Auth{ds: ds, appScopes: newAppScopeCache(), defaultScopes: newDefaultScopeCache()}
}

func appResource(action meta.Action, appID uint32) *meta.ResourceAttribute {
	return &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.App, Action: action, ResourceID: appID},
		BizID: 100001,
	}
}

// 服务维度鉴权必须补齐项目与环境归属，路径才能拼成三级。
func TestFillAppScope(t *testing.T) {
	ds := &fakeAppDS{apps: map[uint32]*pbapp.App{
		30001: {Id: 30001, BizId: 100001, ProjectId: 40001, EnvId: 50001},
	}}
	a := newAppScopeAuth(ds)

	res := appResource(meta.View, 30001)
	// 调用方传入错误的业务 ID 时，以服务自身归属为准，避免用其他业务的授权访问该服务。
	res.BizID = 200002
	require.NoError(t, a.fillAppScope(testKit(), []*meta.ResourceAttribute{res}))
	require.Equal(t, uint32(100001), res.BizID)
	require.Equal(t, uint32(40001), res.ProjectID)
	require.Equal(t, uint32(50001), res.EnvironmentID)

	// 同一批内相同服务只回查一次
	require.Equal(t, 1, ds.calls)

	// 命中缓存后不再请求 data-service
	another := appResource(meta.Update, 30001)
	require.NoError(t, a.fillAppScope(testKit(), []*meta.ResourceAttribute{another}))
	require.Equal(t, uint32(50001), another.EnvironmentID)
	require.Equal(t, 1, ds.calls)
}

// 服务创建与列表按业务维度授权，不针对具体实例，不需要回查归属。
func TestFillAppScopeSkipsBizScopedActions(t *testing.T) {
	ds := &fakeAppDS{}
	a := newAppScopeAuth(ds)

	require.NoError(t, a.fillAppScope(testKit(), []*meta.ResourceAttribute{
		appResource(meta.Create, 0),
		appResource(meta.Find, 0),
	}))
	require.Zero(t, ds.calls)
}

// 调用方已带归属时不再回查，避免重复 RPC。
func TestFillAppScopeSkipsResolved(t *testing.T) {
	ds := &fakeAppDS{}
	a := newAppScopeAuth(ds)

	res := appResource(meta.View, 30001)
	res.ProjectID = 40001
	res.EnvironmentID = 50001
	require.NoError(t, a.fillAppScope(testKit(), []*meta.ResourceAttribute{res}))
	require.Zero(t, ds.calls)
}

// 回查失败必须让鉴权失败，不能带着不完整路径放行。
func TestFillAppScopePropagatesError(t *testing.T) {
	ds := &fakeAppDS{err: context.Canceled}
	a := newAppScopeAuth(ds)

	require.Error(t, a.fillAppScope(testKit(), []*meta.ResourceAttribute{appResource(meta.View, 30001)}))
}

// 服务归属缺失时保持为空，由鉴权路径缺失退化为无权限，不能兜底到默认项目/环境。
func TestFillAppScopeKeepsEmptyWhenNoScope(t *testing.T) {
	ds := &fakeAppDS{apps: map[uint32]*pbapp.App{
		30001: {Id: 30001, BizId: 100001},
	}}
	a := newAppScopeAuth(ds)

	res := appResource(meta.View, 30001)
	require.NoError(t, a.fillAppScope(testKit(), []*meta.ResourceAttribute{res}))
	require.Zero(t, res.ProjectID)
	require.Zero(t, res.EnvironmentID)

	// 不再回查默认项目/环境
	require.Zero(t, ds.ensureCalls)
}

// 已知环境时按真实父项目补齐，未指定目标实例时才使用默认项目/环境。
func TestFillResourceScopeFillsDefaultsForEnvironment(t *testing.T) {
	ds := &fakeAppDS{
		environmentInfos: map[uint32]*pbds.InstanceInfo{
			50001: {Id: "50001", Path: []string{"/biz,100001/project,40001/"}},
		},
		defaults: map[uint32]*pbds.EnsureDefaultProjectEnvResp{
			100001: {ProjectId: 40000, EnvId: 50000},
		},
	}
	a := newAppScopeAuth(ds)

	// 已知环境所属的项目不是业务默认项目。
	res := &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.Environment, Action: meta.View, ResourceID: 50001},
		BizID: 100001,
	}
	require.NoError(t, a.fillResourceScope(testKit(), []*meta.ResourceAttribute{res}))
	require.Equal(t, uint32(40001), res.ProjectID)
	require.Equal(t, uint32(50001), res.ResourceID)

	// 项目与环境都没传时取默认项目与默认环境
	empty := &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.Environment, Action: meta.Update},
		BizID: 100001,
	}
	require.NoError(t, a.fillResourceScope(testKit(), []*meta.ResourceAttribute{empty}))
	require.Equal(t, uint32(40000), empty.ProjectID)
	require.Equal(t, uint32(50000), empty.ResourceID)

	// 默认归属按业务缓存，同业务多次补齐只回查一次
	require.Equal(t, 1, ds.ensureCalls)
	require.Equal(t, 1, ds.environmentCalls)
}

func TestFillResourceScopeDoesNotDefaultParentForUnknownEnvironment(t *testing.T) {
	ds := &fakeAppDS{defaults: map[uint32]*pbds.EnsureDefaultProjectEnvResp{
		100001: {ProjectId: 40000, EnvId: 50000},
	}}
	a := newAppScopeAuth(ds)
	res := &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.Environment, Action: meta.View, ResourceID: 50001},
		BizID: 100001,
	}

	require.NoError(t, a.fillResourceScope(testKit(), []*meta.ResourceAttribute{res}))
	require.Zero(t, res.ProjectID)
	require.Equal(t, uint32(50001), res.ResourceID)
	require.Zero(t, ds.ensureCalls)
}

// 请求同时带了业务与项目也要回查：权限中心不反查拓扑，路径若取请求里的项目，
// 拥有项目 40001 权限的调用方可以拿项目 40002 的环境 ID 拼出项目 40001 的路径。
func TestFillResourceScopeOverridesRequestedParentForExistingEnvironment(t *testing.T) {
	ds := &fakeAppDS{
		environmentInfos: map[uint32]*pbds.InstanceInfo{
			50002: {Id: "50002", Path: []string{"/biz,100001/project,40002/"}},
		},
	}
	a := newAppScopeAuth(ds)

	res := &meta.ResourceAttribute{
		Basic:     meta.Basic{Type: meta.Environment, Action: meta.Delete, ResourceID: 50002},
		BizID:     100001,
		ProjectID: 40001,
	}
	require.NoError(t, a.fillResourceScope(testKit(), []*meta.ResourceAttribute{res}))

	require.Equal(t, 1, ds.environmentCalls, "已有环境实例必须按环境 ID 回查")
	require.Equal(t, uint32(40002), res.ProjectID, "父项目必须取自环境自身路径")
	require.Equal(t, uint32(100001), res.BizID)
	require.Equal(t, uint32(50002), res.ResourceID)
}

// 回查不到父级时，请求里声明的业务与项目必须丢掉，让鉴权路径缺失退化为无权限，
// 而不是按请求给出的项目拼出可以命中他人授权的路径。
func TestFillResourceScopeDropsRequestedParentWhenEnvironmentUnknown(t *testing.T) {
	ds := &fakeAppDS{defaults: map[uint32]*pbds.EnsureDefaultProjectEnvResp{
		100001: {ProjectId: 40000, EnvId: 50000},
	}}
	a := newAppScopeAuth(ds)

	res := &meta.ResourceAttribute{
		Basic:     meta.Basic{Type: meta.Environment, Action: meta.View, ResourceID: 50099},
		BizID:     100001,
		ProjectID: 40001,
	}
	require.NoError(t, a.fillResourceScope(testKit(), []*meta.ResourceAttribute{res}))

	require.Zero(t, res.BizID)
	require.Zero(t, res.ProjectID)
	require.Zero(t, ds.ensureCalls, "已知实例解析不出父级时不能兜底到默认项目")
}

// 服务创建按目标环境授权：祖先取目标环境的真实项目，而不是请求里的 project_id。
func TestFillResourceScopeAppCreateUsesTargetEnvironmentParent(t *testing.T) {
	ds := &fakeAppDS{
		environmentInfos: map[uint32]*pbds.InstanceInfo{
			50002: {Id: "50002", Path: []string{"/biz,100001/project,40002/"}},
		},
	}
	a := newAppScopeAuth(ds)

	res := &meta.ResourceAttribute{
		Basic:     meta.Basic{Type: meta.App, Action: meta.Create, ResourceID: 50002},
		BizID:     100001,
		ProjectID: 40001,
	}
	require.NoError(t, a.fillResourceScope(testKit(), []*meta.ResourceAttribute{res}))

	require.Equal(t, uint32(40002), res.ProjectID)
	require.Equal(t, uint32(100001), res.BizID)
	require.Equal(t, uint32(50002), res.EnvironmentID, "环境 ID 即资源实例 ID")
}

func TestFillResourceScopePropagatesEnvironmentLookupError(t *testing.T) {
	ds := &fakeAppDS{environmentErr: context.Canceled}
	a := newAppScopeAuth(ds)
	res := &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.Environment, Action: meta.View, ResourceID: 50001},
		BizID: 100001,
	}

	require.ErrorIs(t, a.fillResourceScope(testKit(), []*meta.ResourceAttribute{res}), context.Canceled)
	require.Zero(t, ds.ensureCalls)
}

// 创建环境时资源实例就是所属项目，project_id 与 resource_id 必须同时有值。
func TestFillResourceScopeEnvironmentCreateKeepsProjectAsInstance(t *testing.T) {
	ds := &fakeAppDS{defaults: map[uint32]*pbds.EnsureDefaultProjectEnvResp{
		100001: {ProjectId: 40000, EnvId: 50000},
	}}
	a := newAppScopeAuth(ds)

	res := &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.Environment, Action: meta.Create, ResourceID: 40001},
		BizID: 100001,
	}
	require.NoError(t, a.fillResourceScope(testKit(), []*meta.ResourceAttribute{res}))
	require.Equal(t, uint32(40001), res.ProjectID)
	require.Equal(t, uint32(40001), res.ResourceID)

	// 两侧都为空时取默认项目；资源实例仍是项目，不能退化成默认环境
	empty := &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.Environment, Action: meta.Create},
		BizID: 100001,
	}
	require.NoError(t, a.fillResourceScope(testKit(), []*meta.ResourceAttribute{empty}))
	require.Equal(t, uint32(40000), empty.ProjectID)
	require.Equal(t, uint32(40000), empty.ResourceID)
}

// 服务创建按目标环境授权：未传目标环境时按默认环境与默认项目补齐。
func TestFillResourceScopeAppCreateUsesDefaultEnvironment(t *testing.T) {
	ds := &fakeAppDS{defaults: map[uint32]*pbds.EnsureDefaultProjectEnvResp{
		100001: {ProjectId: 40000, EnvId: 50000},
	}}
	a := newAppScopeAuth(ds)

	res := appResource(meta.Create, 0)
	require.NoError(t, a.fillResourceScope(testKit(), []*meta.ResourceAttribute{res}))
	require.Equal(t, uint32(40000), res.ProjectID)
	require.Equal(t, uint32(50000), res.ResourceID)
	require.Equal(t, uint32(50000), res.EnvironmentID)
}

// 项目按实例授权：project_id 与 resource_id 互补，都为空时取默认项目。
func TestFillResourceScopeProjectSyncsInstanceID(t *testing.T) {
	ds := &fakeAppDS{defaults: map[uint32]*pbds.EnsureDefaultProjectEnvResp{
		100001: {ProjectId: 40000, EnvId: 50000},
	}}
	a := newAppScopeAuth(ds)

	res := &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.Project, Action: meta.View, ResourceID: 40001},
		BizID: 100001,
	}
	require.NoError(t, a.fillResourceScope(testKit(), []*meta.ResourceAttribute{res}))
	require.Equal(t, uint32(40001), res.ProjectID)
	require.Equal(t, uint32(40001), res.ResourceID)

	empty := &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.Project, Action: meta.Delete},
		BizID: 100001,
	}
	require.NoError(t, a.fillResourceScope(testKit(), []*meta.ResourceAttribute{empty}))
	require.Equal(t, uint32(40000), empty.ProjectID)
	require.Equal(t, uint32(40000), empty.ResourceID)
}

// 服务归属缺失时不写缓存：历史服务还没绑定项目/环境时若把空归属固化下来，
// 补上归属后也要等 TTL 过期才恢复鉴权，期间会持续按空路径拒绝。
func TestAppScopeOf_SkipsCachingEmptyScope(t *testing.T) {
	ds := &fakeAppDS{apps: map[uint32]*pbapp.App{
		30001: {Id: 30001, BizId: 100001},
	}}
	a := newAppScopeAuth(ds)

	for i := 0; i < 2; i++ {
		scope, err := a.appScopeOf(testKit(), 30001)
		require.NoError(t, err)
		require.Zero(t, scope.projectID)
		require.Zero(t, scope.envID)
	}

	require.Equal(t, 2, ds.calls, "empty scope must not be cached")
}

// 归属完整的服务写缓存，同一服务只回查一次。
func TestAppScopeOf_CachesResolvedScope(t *testing.T) {
	ds := &fakeAppDS{apps: map[uint32]*pbapp.App{
		30001: {Id: 30001, BizId: 100001, ProjectId: 40001, EnvId: 50001},
	}}
	a := newAppScopeAuth(ds)

	for i := 0; i < 2; i++ {
		scope, err := a.appScopeOf(testKit(), 30001)
		require.NoError(t, err)
		require.Equal(t, uint32(40001), scope.projectID)
		require.Equal(t, uint32(50001), scope.envID)
	}

	require.Equal(t, 1, ds.calls, "resolved scope should be cached")
}

// TestDefaultScopeOf_SkipsCachingEmptyScope 默认归属缺失时不写缓存，
// 否则一次异常会在整段 TTL 内把该业务的鉴权固化为无权限。
func TestDefaultScopeOf_SkipsCachingEmptyScope(t *testing.T) {
	ds := &fakeAppDS{defaults: map[uint32]*pbds.EnsureDefaultProjectEnvResp{}}
	a := newAppScopeAuth(ds)

	for i := 0; i < 2; i++ {
		scope, err := a.defaultScopeOf(testKit(), 100001)
		require.NoError(t, err)
		require.Zero(t, scope.projectID)
		require.Zero(t, scope.envID)
	}

	require.Equal(t, 2, ds.ensureCalls, "empty scope must not be cached")
}

// TestDefaultScopeOf_CachesResolvedScope 默认归属完整时写缓存，同一业务只回查一次。
func TestDefaultScopeOf_CachesResolvedScope(t *testing.T) {
	ds := &fakeAppDS{defaults: map[uint32]*pbds.EnsureDefaultProjectEnvResp{
		100001: {ProjectId: 40000, EnvId: 50000},
	}}
	a := newAppScopeAuth(ds)

	for i := 0; i < 2; i++ {
		scope, err := a.defaultScopeOf(testKit(), 100001)
		require.NoError(t, err)
		require.Equal(t, uint32(40000), scope.projectID)
		require.Equal(t, uint32(50000), scope.envID)
	}

	require.Equal(t, 1, ds.ensureCalls, "resolved scope should be cached")
}
