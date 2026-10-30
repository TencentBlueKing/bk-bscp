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

package iamv4

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/TencentBlueKing/bk-bscp/internal/space"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/model"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// fakeSpaces 是 space.Manager 的桩，避免单测依赖 CMDB。
type fakeSpaces struct {
	spaces  []*space.Space
	queried []string
	err     error
}

func (f *fakeSpaces) AllSpaces(_ context.Context) []*space.Space {
	return f.spaces
}

func (f *fakeSpaces) QuerySpace(_ context.Context, bizIDs []string) ([]*space.Space, error) {
	f.queried = bizIDs
	if f.err != nil {
		return nil, f.err
	}

	wanted := make(map[string]bool, len(bizIDs))
	for _, id := range bizIDs {
		wanted[id] = true
	}

	matched := make([]*space.Space, 0, len(bizIDs))
	for _, s := range f.spaces {
		if wanted[s.SpaceId] {
			matched = append(matched, s)
		}
	}

	return matched, nil
}

// fakeApps 是 data-service client 的桩。
type fakeApps struct {
	instances []*pbds.InstanceResource
	details   []*pbds.InstanceInfo
	// lastListReq 记录最后一次列表请求，用于断言分页与关键字是否正确下推
	lastListReq *pbds.ListInstancesReq
	listReqs    []*pbds.ListInstancesReq
	err         error
}

func (f *fakeApps) ListInstances(_ context.Context, in *pbds.ListInstancesReq,
	_ ...grpc.CallOption) (*pbds.ListInstancesResp, error) {

	f.lastListReq = in
	f.listReqs = append(f.listReqs, in)
	if f.err != nil {
		return nil, f.err
	}

	instances := f.instances
	if in.Keyword != "" {
		filtered := make([]*pbds.InstanceResource, 0, len(f.instances))
		for _, one := range f.instances {
			if !contains(one.Name, in.Keyword) && one.Id != in.Keyword {
				continue
			}
			filtered = append(filtered, one)
		}
		instances = filtered
	}

	// 模拟 data-service 先按 keyword 过滤再分页
	start, limit := int(in.Page.Start), int(in.Page.Limit)
	if start > len(instances) {
		start = len(instances)
	}

	end := start + limit
	if limit == 0 || end > len(instances) {
		end = len(instances)
	}

	return &pbds.ListInstancesResp{
		Count:   uint32(len(instances)),
		Details: instances[start:end],
	}, nil
}

func (f *fakeApps) FetchInstanceInfo(_ context.Context, _ *pbds.FetchInstanceInfoReq,
	_ ...grpc.CallOption) (*pbds.FetchInstanceInfoResp, error) {

	if f.err != nil {
		return nil, f.err
	}

	return &pbds.FetchInstanceInfoResp{Details: f.details}, nil
}

func testKit() *kit.Kit {
	return &kit.Kit{Ctx: context.Background(), TenantID: "default", User: "tester"}
}

func testSpaces() []*space.Space {
	return []*space.Space{
		{SpaceId: "100001", SpaceName: "测试业务"},
		{SpaceId: "200001", SpaceName: "电商业务"},
		{SpaceId: "200002", SpaceName: "游戏业务"},
	}
}

func newTestIAM(t *testing.T, apps appLister, bizs spaceLister) *IAM {
	t.Helper()

	i, err := NewIAM(apps, bizs)
	require.NoError(t, err)

	return i
}

func TestNewIAMRejectsNilDeps(t *testing.T) {
	_, err := NewIAM(nil, &fakeSpaces{})
	require.ErrorContains(t, err, "app lister")

	_, err = NewIAM(&fakeApps{}, nil)
	require.ErrorContains(t, err, "space lister")
}

func TestPullResourceRejectsUnknownMethod(t *testing.T) {
	i := newTestIAM(t, &fakeApps{}, &fakeSpaces{})

	_, err := i.PullResource(testKit(), &PullResourceReq{Type: model.ResourceTypeBiz,
		Method: "list_attr"})
	require.ErrorContains(t, err, "unsupported callback method")
}

func TestPullResourceRejectsUnknownType(t *testing.T) {
	i := newTestIAM(t, &fakeApps{}, &fakeSpaces{})

	_, err := i.PullResource(testKit(), &PullResourceReq{Type: "credential",
		Method: MethodListInstance})
	require.ErrorContains(t, err, "unsupported resource type")
}

func TestListBizInstances(t *testing.T) {
	i := newTestIAM(t, &fakeApps{}, &fakeSpaces{spaces: testSpaces()})

	got, err := i.PullResource(testKit(), &PullResourceReq{
		Type:   model.ResourceTypeBiz,
		Method: MethodListInstance,
		Page:   Page{Page: 1, PageSize: 10},
	})
	require.NoError(t, err)

	result := got.(*ListInstanceResult)
	require.Equal(t, 3, result.Count)
	require.Len(t, result.Results, 3)
	require.Equal(t, InstanceBrief{ID: "100001", DisplayName: "测试业务"}, result.Results[0])
}

// count 是过滤后的总数，与当前页的条目数无关。
func TestListBizInstancesPaging(t *testing.T) {
	i := newTestIAM(t, &fakeApps{}, &fakeSpaces{spaces: testSpaces()})

	second, err := i.listBizInstances(testKit(), Filter{}, Page{Page: 2, PageSize: 2})
	require.NoError(t, err)
	require.Equal(t, 3, second.Count)
	require.Len(t, second.Results, 1)
	require.Equal(t, "200002", second.Results[0].ID)

	// 超出范围的页返回空列表而非报错
	fourth, err := i.listBizInstances(testKit(), Filter{}, Page{Page: 4, PageSize: 2})
	require.NoError(t, err)
	require.Equal(t, 3, fourth.Count)
	require.Empty(t, fourth.Results)
}

func TestListBizInstancesKeyword(t *testing.T) {
	i := newTestIAM(t, &fakeApps{}, &fakeSpaces{spaces: testSpaces()})

	byName, err := i.listBizInstances(testKit(), Filter{Keyword: "业务"}, Page{})
	require.NoError(t, err)
	require.Equal(t, 3, byName.Count)

	narrowed, err := i.listBizInstances(testKit(), Filter{Keyword: "电商"}, Page{})
	require.NoError(t, err)
	require.Equal(t, 1, narrowed.Count)
	require.Equal(t, "200001", narrowed.Results[0].ID)

	// 关键字也匹配实例 ID，方便用户直接输入业务 ID 搜索
	byID, err := i.listBizInstances(testKit(), Filter{Keyword: "100001"}, Page{})
	require.NoError(t, err)
	require.Equal(t, 1, byID.Count)

	none, err := i.listBizInstances(testKit(), Filter{Keyword: "不存在的业务"}, Page{})
	require.NoError(t, err)
	require.Zero(t, none.Count)
	require.Empty(t, none.Results)
}

// biz 是拓扑根节点，带了 parent 也应忽略而不是报错。
func TestListBizInstancesIgnoresParent(t *testing.T) {
	i := newTestIAM(t, &fakeApps{}, &fakeSpaces{spaces: testSpaces()})

	got, err := i.listBizInstances(testKit(),
		Filter{Parent: &Parent{Type: "unknown", ID: "x"}}, Page{})
	require.NoError(t, err)
	require.Equal(t, 3, got.Count)
}

func TestFetchBizInstanceInfo(t *testing.T) {
	spaces := &fakeSpaces{spaces: testSpaces()}
	i := newTestIAM(t, &fakeApps{}, spaces)

	got, err := i.PullResource(testKit(), &PullResourceReq{
		Type:     model.ResourceTypeBiz,
		Method:   MethodFetchInstanceInfo,
		Filter:   Filter{IDs: []string{"200001", "100001"}},
		Requires: []string{AttrDisplayName},
	})
	require.NoError(t, err)

	infos := got.([]instanceInfo)
	require.Len(t, infos, 2)
	// 按请求顺序返回
	require.Equal(t, "200001", infos[0]["id"])
	require.Equal(t, "电商业务", infos[0][AttrDisplayName])
	require.Equal(t, "100001", infos[1]["id"])
	// 只 require 了 display_name，不应带其他属性
	require.NotContains(t, infos[0], AttrIAMApprovers)
	// biz 是根节点，任何情况下都不该有拓扑路径
	require.NotContains(t, infos[0], AttrIAMPath)
}

// 查不到的实例跳过而不报错，业务可能已在 CMDB 侧归档。
func TestFetchBizInstanceInfoSkipsMissing(t *testing.T) {
	i := newTestIAM(t, &fakeApps{}, &fakeSpaces{spaces: testSpaces()})

	infos, err := i.fetchBizInstanceInfo(testKit(), []string{"100001", "999999"},
		newAttrSelector(nil))
	require.NoError(t, err)
	require.Len(t, infos, 1)
	require.Equal(t, "100001", infos[0]["id"])
}

func TestFetchBizInstanceInfoPropagatesError(t *testing.T) {
	i := newTestIAM(t, &fakeApps{}, &fakeSpaces{err: errors.New("cmdb down")})

	_, err := i.fetchBizInstanceInfo(testKit(), []string{"100001"}, newAttrSelector(nil))
	require.ErrorContains(t, err, "cmdb down")
}

// requires 为空表示返回全部属性。
func TestFetchInstanceInfoAllAttrs(t *testing.T) {
	i := newTestIAM(t, &fakeApps{}, &fakeSpaces{spaces: testSpaces()})

	infos, err := i.fetchBizInstanceInfo(testKit(), []string{"100001"}, newAttrSelector(nil))
	require.NoError(t, err)
	require.Contains(t, infos[0], AttrDisplayName)
	require.Contains(t, infos[0], AttrIAMApprovers)
}

func TestListAppInstancesRequiresParent(t *testing.T) {
	i := newTestIAM(t, &fakeApps{}, &fakeSpaces{})

	_, err := i.listAppInstances(testKit(), Filter{}, Page{})
	require.ErrorContains(t, err, "requires a parent")

	_, err = i.listAppInstances(testKit(),
		Filter{Parent: &Parent{Type: model.ResourceTypeApp, ID: "1"}}, Page{})
	require.ErrorContains(t, err, "unexpected parent type")
}

// 展示层级可能配置在业务或项目上，权限中心会按对应 parent 回调，不能报错。
func TestListAppInstancesAcceptsUpperParents(t *testing.T) {
	for _, parentType := range []string{model.ResourceTypeBiz, model.ResourceTypeProject,
		model.ResourceTypeEnvironment} {
		apps := &fakeApps{}
		i := newTestIAM(t, apps, &fakeSpaces{})

		_, err := i.listAppInstances(testKit(),
			Filter{Parent: &Parent{Type: parentType, ID: "1"}}, Page{})
		require.NoError(t, err, "parent type %s", parentType)
		require.Equal(t, parentType, apps.lastListReq.ParentType)
	}
}

// 无关键字时分页应下推给 data-service，不在内存里做。
func TestListAppInstancesPushesDownPaging(t *testing.T) {
	apps := &fakeApps{instances: []*pbds.InstanceResource{
		{Id: "30001", Name: "主服务"},
		{Id: "30002", Name: "网关"},
		{Id: "30003", Name: "任务调度"},
	}}
	i := newTestIAM(t, apps, &fakeSpaces{})

	got, err := i.listAppInstances(testKit(),
		Filter{Parent: &Parent{Type: model.ResourceTypeEnvironment, ID: "50001"}},
		Page{Page: 2, PageSize: 2})
	require.NoError(t, err)

	require.Equal(t, uint32(2), apps.lastListReq.Page.Start)
	require.Equal(t, uint32(2), apps.lastListReq.Page.Limit)
	require.Equal(t, model.ResourceTypeApp, apps.lastListReq.ResourceType)
	require.Equal(t, model.ResourceTypeEnvironment, apps.lastListReq.ParentType)
	require.Equal(t, "50001", apps.lastListReq.ParentId)
	require.Equal(t, 3, got.Count)
	require.Len(t, got.Results, 1)
}

// 带关键字时把 keyword 与分页一起下推给 data-service，由下游过滤。
func TestListAppInstancesKeyword(t *testing.T) {
	apps := &fakeApps{instances: []*pbds.InstanceResource{
		{Id: "30001", Name: "配置中心-主服务"},
		{Id: "30002", Name: "配置中心-网关"},
		{Id: "30003", Name: "任务调度"},
	}}
	i := newTestIAM(t, apps, &fakeSpaces{})

	got, err := i.listAppInstances(testKit(),
		Filter{Parent: &Parent{Type: model.ResourceTypeEnvironment, ID: "50001"}, Keyword: "配置中心"},
		Page{Page: 1, PageSize: 10})
	require.NoError(t, err)

	require.Equal(t, "配置中心", apps.lastListReq.Keyword)
	require.Equal(t, uint32(0), apps.lastListReq.Page.Start)
	require.Equal(t, uint32(10), apps.lastListReq.Page.Limit)
	require.Len(t, apps.listReqs, 1)
	require.Equal(t, 2, got.Count)
	require.Equal(t, "30001", got.Results[0].ID)
	require.Equal(t, "30002", got.Results[1].ID)
}

// 关键字过滤后的分页也下推，Count 是过滤后的总数。
func TestListAppInstancesKeywordPushesDownPaging(t *testing.T) {
	apps := &fakeApps{instances: []*pbds.InstanceResource{
		{Id: "30001", Name: "配置中心-主服务"},
		{Id: "30002", Name: "任务调度"},
		{Id: "30003", Name: "配置中心-网关"},
		{Id: "30004", Name: "配置中心-隐藏服务"},
	}}
	i := newTestIAM(t, apps, &fakeSpaces{})

	got, err := i.listAppInstances(testKit(),
		Filter{Parent: &Parent{Type: model.ResourceTypeEnvironment, ID: "50001"}, Keyword: "配置中心"},
		Page{Page: 2, PageSize: 2})
	require.NoError(t, err)

	require.Equal(t, "配置中心", apps.lastListReq.Keyword)
	require.Equal(t, uint32(2), apps.lastListReq.Page.Start)
	require.Equal(t, uint32(2), apps.lastListReq.Page.Limit)
	require.Len(t, apps.listReqs, 1)
	require.Equal(t, 3, got.Count)
	require.Len(t, got.Results, 1)
	require.Equal(t, "30004", got.Results[0].ID)
}

// 关键字是完整服务 ID 时也应命中，方便在权限中心直接粘贴 ID 搜索。
func TestListAppInstancesKeywordByID(t *testing.T) {
	apps := &fakeApps{instances: []*pbds.InstanceResource{
		{Id: "30001", Name: "主服务"},
		{Id: "30002", Name: "网关"},
	}}
	i := newTestIAM(t, apps, &fakeSpaces{})

	got, err := i.listAppInstances(testKit(),
		Filter{Parent: &Parent{Type: model.ResourceTypeEnvironment, ID: "50001"}, Keyword: "30002"},
		Page{Page: 1, PageSize: 10})
	require.NoError(t, err)

	require.Equal(t, "30002", apps.lastListReq.Keyword)
	require.Equal(t, 1, got.Count)
	require.Equal(t, "30002", got.Results[0].ID)
}

func TestFetchAppInstanceInfo(t *testing.T) {
	apps := &fakeApps{details: []*pbds.InstanceInfo{
		{
			Id:          "30001",
			DisplayName: "主服务",
			Path:        []string{"/biz,100001/project,40001/environment,50001/"},
			Approver:    []string{"alice"},
		},
	}}
	i := newTestIAM(t, apps, &fakeSpaces{})

	got, err := i.PullResource(testKit(), &PullResourceReq{
		Type:     model.ResourceTypeApp,
		Method:   MethodFetchInstanceInfo,
		Filter:   Filter{IDs: []string{"30001"}},
		Requires: []string{AttrDisplayName, AttrIAMPath},
	})
	require.NoError(t, err)

	infos := got.([]instanceInfo)
	require.Len(t, infos, 1)
	require.Equal(t, "30001", infos[0]["id"])
	require.Equal(t, "主服务", infos[0][AttrDisplayName])
	// V3 返回数组，V4 只接受单个字符串
	require.Equal(t, "/biz,100001/project,40001/environment,50001/", infos[0][AttrIAMPath])
	require.NotContains(t, infos[0], AttrIAMApprovers)
}

func TestFetchInstanceInfoEmptyIDs(t *testing.T) {
	i := newTestIAM(t, &fakeApps{}, &fakeSpaces{})

	got, err := i.PullResource(testKit(), &PullResourceReq{
		Type:   model.ResourceTypeApp,
		Method: MethodFetchInstanceInfo,
		Filter: Filter{IDs: []string{}},
	})
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestPageNormalize(t *testing.T) {
	require.Equal(t, Page{Page: 1, PageSize: MaxPageSize}, Page{}.normalize())
	require.Equal(t, Page{Page: 1, PageSize: MaxPageSize},
		Page{Page: -1, PageSize: MaxPageSize + 1}.normalize())
	require.Equal(t, Page{Page: 3, PageSize: 20}, Page{Page: 3, PageSize: 20}.normalize())
	require.Equal(t, 40, Page{Page: 3, PageSize: 20}.offset())
}

// 项目是环境的授权维度，权限中心必须按业务维度拉取项目，缺父级或父级不是业务都视为非法请求。
func TestListProjectInstancesRequiresBizParent(t *testing.T) {
	i := newTestIAM(t, &fakeApps{}, &fakeSpaces{})

	_, err := i.listProjectInstances(testKit(), Filter{}, Page{})
	require.ErrorContains(t, err, "requires a biz parent")

	_, err = i.listProjectInstances(testKit(),
		Filter{Parent: &Parent{Type: model.ResourceTypeApp, ID: "1"}}, Page{})
	require.ErrorContains(t, err, "unexpected parent type")
}

func TestListProjectInstances(t *testing.T) {
	apps := &fakeApps{instances: []*pbds.InstanceResource{
		{Id: "40001", Name: "默认项目"},
		{Id: "40002", Name: "电商项目"},
	}}
	i := newTestIAM(t, apps, &fakeSpaces{})

	got, err := i.PullResource(testKit(), &PullResourceReq{
		Type:   model.ResourceTypeProject,
		Method: MethodListInstance,
		Filter: Filter{Parent: &Parent{Type: model.ResourceTypeBiz, ID: "100001"}},
		Page:   Page{Page: 1, PageSize: 10},
	})
	require.NoError(t, err)

	require.Equal(t, model.ResourceTypeProject, apps.lastListReq.ResourceType)
	require.Equal(t, model.ResourceTypeBiz, apps.lastListReq.ParentType)
	require.Equal(t, "100001", apps.lastListReq.ParentId)

	result := got.(*ListInstanceResult)
	require.Equal(t, 2, result.Count)
	require.Equal(t, InstanceBrief{ID: "40001", DisplayName: "默认项目"}, result.Results[0])
}

// 环境挂在项目下，权限中心必须按项目维度拉取环境，缺父级或父级不是项目都视为非法请求。
func TestListEnvironmentInstancesRequiresProjectParent(t *testing.T) {
	i := newTestIAM(t, &fakeApps{}, &fakeSpaces{})

	_, err := i.listEnvironmentInstances(testKit(), Filter{}, Page{})
	require.ErrorContains(t, err, "requires a project parent")

	_, err = i.listEnvironmentInstances(testKit(),
		Filter{Parent: &Parent{Type: model.ResourceTypeBiz, ID: "100001"}}, Page{})
	require.ErrorContains(t, err, "unexpected parent type")
}

func TestListEnvironmentInstances(t *testing.T) {
	apps := &fakeApps{instances: []*pbds.InstanceResource{
		{Id: "50001", Name: "生产环境"},
		{Id: "50002", Name: "测试环境"},
	}}
	i := newTestIAM(t, apps, &fakeSpaces{})

	got, err := i.PullResource(testKit(), &PullResourceReq{
		Type:   model.ResourceTypeEnvironment,
		Method: MethodListInstance,
		Filter: Filter{Parent: &Parent{Type: model.ResourceTypeProject, ID: "40001"}},
		Page:   Page{Page: 1, PageSize: 10},
	})
	require.NoError(t, err)

	require.Equal(t, model.ResourceTypeEnvironment, apps.lastListReq.ResourceType)
	require.Equal(t, model.ResourceTypeProject, apps.lastListReq.ParentType)
	require.Equal(t, "40001", apps.lastListReq.ParentId)

	result := got.(*ListInstanceResult)
	require.Equal(t, 2, result.Count)
	require.Equal(t, InstanceBrief{ID: "50001", DisplayName: "生产环境"}, result.Results[0])
}

// 环境的拓扑路径是 /biz,{bizID}/project,{projectID}/，
// 按业务或项目授权的用户据此覆盖到项目下所有环境。
func TestFetchEnvironmentInstanceInfoCarriesProjectPath(t *testing.T) {
	apps := &fakeApps{details: []*pbds.InstanceInfo{
		{
			Id:          "50001",
			DisplayName: "生产环境",
			Path:        []string{"/biz,100001/project,40001/"},
			Approver:    []string{"alice"},
		},
	}}
	i := newTestIAM(t, apps, &fakeSpaces{})

	got, err := i.PullResource(testKit(), &PullResourceReq{
		Type:     model.ResourceTypeEnvironment,
		Method:   MethodFetchInstanceInfo,
		Filter:   Filter{IDs: []string{"50001"}},
		Requires: []string{AttrDisplayName, AttrIAMPath, AttrIAMApprovers},
	})
	require.NoError(t, err)

	infos := got.([]instanceInfo)
	require.Len(t, infos, 1)
	require.Equal(t, "50001", infos[0]["id"])
	require.Equal(t, "生产环境", infos[0][AttrDisplayName])
	require.Equal(t, "/biz,100001/project,40001/", infos[0][AttrIAMPath])
	require.Equal(t, []string{"alice"}, infos[0][AttrIAMApprovers])
}

// 项目的拓扑路径是 /biz,{bizID}/，按业务授权的用户据此覆盖到业务下所有项目及其环境。
func TestFetchProjectInstanceInfoCarriesBizPath(t *testing.T) {
	apps := &fakeApps{details: []*pbds.InstanceInfo{
		{Id: "40001", DisplayName: "默认项目", Path: []string{"/biz,100001/"}, Approver: []string{}},
	}}
	i := newTestIAM(t, apps, &fakeSpaces{})

	got, err := i.PullResource(testKit(), &PullResourceReq{
		Type:     model.ResourceTypeProject,
		Method:   MethodFetchInstanceInfo,
		Filter:   Filter{IDs: []string{"40001"}},
		Requires: []string{AttrDisplayName, AttrIAMPath},
	})
	require.NoError(t, err)

	infos := got.([]instanceInfo)
	require.Len(t, infos, 1)
	require.Equal(t, "40001", infos[0]["id"])
	require.Equal(t, "默认项目", infos[0][AttrDisplayName])
	require.Equal(t, "/biz,100001/", infos[0][AttrIAMPath])
}
