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
	"fmt"
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

	apps  map[uint32]*pbapp.App
	calls int
	err   error

	environmentInfos map[uint32]*pbds.InstanceInfo
	environmentCalls int
	environmentErr   error

	// defaults 是业务 ID 到默认项目/环境的映射
	defaults     map[uint32]*pbds.GetDefaultProjectEnvResp
	defaultCalls int
}

func (f *fakeAppDS) GetDefaultProjectEnv(_ context.Context, in *pbds.GetDefaultProjectEnvReq,
	_ ...grpc.CallOption) (*pbds.GetDefaultProjectEnvResp, error) {

	f.defaultCalls++
	if resp, ok := f.defaults[in.BizId]; ok {
		return resp, nil
	}

	return &pbds.GetDefaultProjectEnvResp{}, nil
}

func (f *fakeAppDS) FetchInstanceInfo(_ context.Context, in *pbds.FetchInstanceInfoReq,
	_ ...grpc.CallOption) (*pbds.FetchInstanceInfoResp, error) {

	if in.GetResourceType() == string(meta.App) {
		return f.fetchAppInfo(in)
	}

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

// fetchAppInfo 按服务自身归属拼出三级 path；服务不存在时不出现在返回结果里。
func (f *fakeAppDS) fetchAppInfo(in *pbds.FetchInstanceInfoReq) (*pbds.FetchInstanceInfoResp, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}

	resp := &pbds.FetchInstanceInfoResp{}
	for _, id := range in.GetIds() {
		appID, err := strconv.ParseUint(id, 10, 32)
		if err != nil {
			continue
		}
		app, ok := f.apps[uint32(appID)]
		if !ok {
			continue
		}

		resp.Details = append(resp.Details, &pbds.InstanceInfo{
			Id: id,
			Path: []string{fmt.Sprintf("/biz,%d/project,%d/environment,%d/",
				app.GetBizId(), app.GetProjectId(), app.GetEnvId())},
		})
	}

	return resp, nil
}

// newAppScopeAuth 构造按新拓扑、开启项目/环境维度校验的 Auth，便于验证完整归属补齐。
func newAppScopeAuth(ds pbds.DataClient) *Auth {
	return &Auth{
		ds:             ds,
		projectEnvAuth: true,
		topology:       meta.TopologyHierarchical,
		appScopes:      newAppScopeCache(),
		defaultScopes:  newDefaultScopeCache(),
	}
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

// 调用方传入的归属不能作数：即使 project/env 都有值，也必须按服务 ID 回查并用真实值覆盖。
// 否则 CheckPermission 这类由前端提交 resources 的接口可以伪造归属，让服务 X 命中环境 E 的授权。
func TestFillAppScopeOverridesRequestedScope(t *testing.T) {
	ds := &fakeAppDS{apps: map[uint32]*pbapp.App{
		30001: {Id: 30001, BizId: 100001, ProjectId: 40001, EnvId: 50001},
	}}
	a := newAppScopeAuth(ds)

	res := appResource(meta.View, 30001)
	res.ProjectID = 40099
	res.EnvironmentID = 50099
	require.NoError(t, a.fillAppScope(testKit(), []*meta.ResourceAttribute{res}))

	require.Equal(t, 1, ds.calls, "带了归属也必须回查")
	require.Equal(t, uint32(100001), res.BizID)
	require.Equal(t, uint32(40001), res.ProjectID, "项目必须取自服务自身归属")
	require.Equal(t, uint32(50001), res.EnvironmentID, "环境必须取自服务自身归属")
}

// 服务不存在时清空归属，退化为无权限，而不是带着调用方给的值继续鉴权。
func TestFillAppScopeDropsScopeWhenAppUnknown(t *testing.T) {
	a := newAppScopeAuth(&fakeAppDS{})

	res := appResource(meta.View, 39999)
	res.ProjectID = 40001
	res.EnvironmentID = 50001
	require.NoError(t, a.fillAppScope(testKit(), []*meta.ResourceAttribute{res}))

	require.Zero(t, res.ProjectID)
	require.Zero(t, res.EnvironmentID)
}

// 同一批内的多个服务合并成一次批量回查，不逐个串行请求 data-service。
func TestFillAppScopeBatchLookup(t *testing.T) {
	ds := &fakeAppDS{apps: map[uint32]*pbapp.App{
		30001: {Id: 30001, BizId: 100001, ProjectId: 40001, EnvId: 50001},
		30002: {Id: 30002, BizId: 100001, ProjectId: 40002, EnvId: 50002},
	}}
	a := newAppScopeAuth(ds)

	res := []*meta.ResourceAttribute{
		appResource(meta.View, 30001),
		appResource(meta.Update, 30002),
		appResource(meta.Delete, 30001),
	}
	require.NoError(t, a.fillAppScope(testKit(), res))

	require.Equal(t, 1, ds.calls)
	require.Equal(t, uint32(50001), res[0].EnvironmentID)
	require.Equal(t, uint32(40002), res[1].ProjectID)
	require.Equal(t, uint32(50001), res[2].EnvironmentID)
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
	require.Zero(t, ds.defaultCalls)
}

// 已知环境时按真实父项目补齐，未指定目标实例时才使用默认项目/环境。
func TestFillResourceScopeFillsDefaultsForEnvironment(t *testing.T) {
	ds := &fakeAppDS{
		environmentInfos: map[uint32]*pbds.InstanceInfo{
			50001: {Id: "50001", Path: []string{"/biz,100001/project,40001/"}},
		},
		defaults: map[uint32]*pbds.GetDefaultProjectEnvResp{
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
	require.Equal(t, 1, ds.defaultCalls)
	require.Equal(t, 1, ds.environmentCalls)
}

func TestFillResourceScopeDoesNotDefaultParentForUnknownEnvironment(t *testing.T) {
	ds := &fakeAppDS{defaults: map[uint32]*pbds.GetDefaultProjectEnvResp{
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
	require.Zero(t, ds.defaultCalls)
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
	ds := &fakeAppDS{defaults: map[uint32]*pbds.GetDefaultProjectEnvResp{
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
	require.Zero(t, ds.defaultCalls, "已知实例解析不出父级时不能兜底到默认项目")
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

// 关闭项目/环境维度校验但权限中心已是新拓扑时，服务创建的鉴权路径仍带项目祖先：
// 项目必须回查目标环境的真实归属，不能采信请求里的 project_id。
func TestFillResourceScopeAppCreateRejectsForgedProjectWhenAuthDisabled(t *testing.T) {
	ds := &fakeAppDS{
		environmentInfos: map[uint32]*pbds.InstanceInfo{
			50002: {Id: "50002", Path: []string{"/biz,100001/project,40002/"}},
		},
	}
	a := newAppScopeAuth(ds)
	a.projectEnvAuth = false

	res := &meta.ResourceAttribute{
		Basic:     meta.Basic{Type: meta.App, Action: meta.Create, ResourceID: 50002},
		BizID:     100001,
		ProjectID: 40001,
	}
	require.NoError(t, a.fillResourceScope(testKit(), []*meta.ResourceAttribute{res}))

	require.Equal(t, 1, ds.environmentCalls, "开关关闭时服务创建同样要回查环境父级")
	require.Equal(t, uint32(40002), res.ProjectID, "项目祖先必须取自环境自身路径")
	require.Equal(t, uint32(50002), res.EnvironmentID)
}

// 开关关闭时 project_id 为 0 也要补齐，否则服务创建的申请链接会直接丢掉该申请项。
func TestFillResourceScopeAppCreateFillsProjectWhenAuthDisabled(t *testing.T) {
	ds := &fakeAppDS{
		environmentInfos: map[uint32]*pbds.InstanceInfo{
			50002: {Id: "50002", Path: []string{"/biz,100001/project,40002/"}},
		},
	}
	a := newAppScopeAuth(ds)
	a.projectEnvAuth = false

	res := &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.App, Action: meta.Create, ResourceID: 50002},
		BizID: 100001,
	}
	require.NoError(t, a.fillResourceScope(testKit(), []*meta.ResourceAttribute{res}))
	require.Equal(t, uint32(40002), res.ProjectID)
}

// 开关关闭时项目/环境资源本身不参与鉴权，不做无谓的回查与默认归属补齐。
func TestFillResourceScopeSkipsProjectEnvironmentLookupWhenAuthDisabled(t *testing.T) {
	ds := &fakeAppDS{}
	a := newAppScopeAuth(ds)
	a.projectEnvAuth = false

	resources := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Project, Action: meta.View, ResourceID: 40001}, BizID: 100001},
		{Basic: meta.Basic{Type: meta.Environment, Action: meta.View, ResourceID: 50001}, BizID: 100001},
		{Basic: meta.Basic{Type: meta.Environment, Action: meta.Create}, BizID: 100001},
	}
	require.NoError(t, a.fillResourceScope(testKit(), resources))

	require.Zero(t, ds.environmentCalls)
	require.Zero(t, ds.defaultCalls)
	require.Zero(t, resources[0].ProjectID)
}

// 旧拓扑下服务创建只按业务授权，没有项目祖先需要回查。
func TestFillResourceScopeAppCreateSkipsLookupInLegacyTopology(t *testing.T) {
	ds := &fakeAppDS{}
	a := newAppScopeAuth(ds)
	a.projectEnvAuth = false
	a.topology = meta.TopologyLegacy

	res := &meta.ResourceAttribute{
		Basic:     meta.Basic{Type: meta.App, Action: meta.Create, ResourceID: 50002},
		BizID:     100001,
		ProjectID: 40001,
	}
	require.NoError(t, a.fillResourceScope(testKit(), []*meta.ResourceAttribute{res}))
	require.Zero(t, ds.environmentCalls)
}

func TestFillResourceScopePropagatesEnvironmentLookupError(t *testing.T) {
	ds := &fakeAppDS{environmentErr: context.Canceled}
	a := newAppScopeAuth(ds)
	res := &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.Environment, Action: meta.View, ResourceID: 50001},
		BizID: 100001,
	}

	require.ErrorIs(t, a.fillResourceScope(testKit(), []*meta.ResourceAttribute{res}), context.Canceled)
	require.Zero(t, ds.defaultCalls)
}

// 创建环境时资源实例就是所属项目，project_id 与 resource_id 必须同时有值。
func TestFillResourceScopeEnvironmentCreateKeepsProjectAsInstance(t *testing.T) {
	ds := &fakeAppDS{defaults: map[uint32]*pbds.GetDefaultProjectEnvResp{
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
	ds := &fakeAppDS{defaults: map[uint32]*pbds.GetDefaultProjectEnvResp{
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
	ds := &fakeAppDS{defaults: map[uint32]*pbds.GetDefaultProjectEnvResp{
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
func TestLookupAppScopes_SkipsCachingEmptyScope(t *testing.T) {
	ds := &fakeAppDS{apps: map[uint32]*pbapp.App{
		30001: {Id: 30001, BizId: 100001},
	}}
	a := newAppScopeAuth(ds)

	for i := 0; i < 2; i++ {
		scopes, err := a.lookupAppScopes(testKit(), []uint32{30001})
		require.NoError(t, err)
		require.Zero(t, scopes[30001].projectID)
		require.Zero(t, scopes[30001].envID)
	}

	require.Equal(t, 2, ds.calls, "empty scope must not be cached")
}

// 归属完整的服务写缓存，同一服务只回查一次。
func TestLookupAppScopes_CachesResolvedScope(t *testing.T) {
	ds := &fakeAppDS{apps: map[uint32]*pbapp.App{
		30001: {Id: 30001, BizId: 100001, ProjectId: 40001, EnvId: 50001},
	}}
	a := newAppScopeAuth(ds)

	for i := 0; i < 2; i++ {
		scopes, err := a.lookupAppScopes(testKit(), []uint32{30001})
		require.NoError(t, err)
		require.Equal(t, uint32(40001), scopes[30001].projectID)
		require.Equal(t, uint32(50001), scopes[30001].envID)
	}

	require.Equal(t, 1, ds.calls, "resolved scope should be cached")
}

// TestDefaultScopeOf_SkipsCachingEmptyScope 默认归属缺失时不写缓存，
// 否则一次异常会在整段 TTL 内把该业务的鉴权固化为无权限。
func TestDefaultScopeOf_SkipsCachingEmptyScope(t *testing.T) {
	ds := &fakeAppDS{defaults: map[uint32]*pbds.GetDefaultProjectEnvResp{}}
	a := newAppScopeAuth(ds)

	for i := 0; i < 2; i++ {
		scope, err := a.defaultScopeOf(testKit(), 100001)
		require.NoError(t, err)
		require.Zero(t, scope.projectID)
		require.Zero(t, scope.envID)
	}

	require.Equal(t, 2, ds.defaultCalls, "empty scope must not be cached")
}

// TestDefaultScopeOf_CachesResolvedScope 默认归属完整时写缓存，同一业务只回查一次。
func TestDefaultScopeOf_CachesResolvedScope(t *testing.T) {
	ds := &fakeAppDS{defaults: map[uint32]*pbds.GetDefaultProjectEnvResp{
		100001: {ProjectId: 40000, EnvId: 50000},
	}}
	a := newAppScopeAuth(ds)

	for i := 0; i < 2; i++ {
		scope, err := a.defaultScopeOf(testKit(), 100001)
		require.NoError(t, err)
		require.Equal(t, uint32(40000), scope.projectID)
		require.Equal(t, uint32(50000), scope.envID)
	}

	require.Equal(t, 1, ds.defaultCalls, "resolved scope should be cached")
}
