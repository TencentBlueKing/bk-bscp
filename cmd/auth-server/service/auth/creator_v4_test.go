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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	clientv4 "github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/model"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// fakeV4Gateway 记录授权请求，避免单测依赖真实网关。
type fakeV4Gateway struct {
	operator    string
	items       []clientv4.Authorization
	permissions []clientv4.ApplyPermission
	calls       int
	err         error
}

func (f *fakeV4Gateway) AddAuthorizations(_ *kit.Kit, operator string,
	items []clientv4.Authorization) error {

	f.calls++
	f.operator = operator
	f.items = items

	return f.err
}

func (f *fakeV4Gateway) GeneratePermApplyURL(_ *kit.Kit, permissions []clientv4.ApplyPermission) (
	string, error) {

	f.permissions = permissions
	return "http://iam.example.com/apply", nil
}

func testKit() *kit.Kit {
	return &kit.Kit{Ctx: context.Background(), TenantID: "default", User: "tester"}
}

func creatorOption(appID, bizID, creator string) *client.GrantResourceCreatorActionOption {
	opt := &client.GrantResourceCreatorActionOption{
		System:  sys.SystemIDBSCP,
		Type:    sys.Application,
		ID:      appID,
		Name:    "demo-app",
		Creator: creator,
	}

	if bizID != "" {
		opt.Ancestors = []client.GrantResourceCreatorActionAncestor{{
			System: sys.SystemIDCMDB,
			Type:   sys.Business,
			ID:     bizID,
		}}
	}

	return opt
}

func newV4Auth(gw v4Gateway) *Auth {
	return &Auth{v4Cli: gw}
}

// 跨维度角色必须拆成两条授权：一次调用只授予 related_resource_type_id 指定的那一个维度。
// 只授 app 维度会导致 find_business_resource 不生效，业务访问前置校验会拦住一切操作。
func TestGrantCreatorActionV4SplitsByScope(t *testing.T) {
	gw := new(fakeV4Gateway)
	a := newV4Auth(gw)

	err := a.grantResourceCreatorActionV4(testKit(), creatorOption("30001", "2", "alice"))
	require.NoError(t, err)

	// 两条授权在同一个请求里发出
	require.Equal(t, 1, gw.calls)
	require.Len(t, gw.items, 2)
	require.Equal(t, "alice", gw.operator)

	byScope := make(map[string]clientv4.Authorization, 2)
	for _, item := range gw.items {
		byScope[item.RelatedResourceTypeID] = item
	}

	appScoped, ok := byScope[model.ResourceTypeApp]
	require.True(t, ok, "app scoped authorization is missing")
	require.Equal(t, model.RoleAppOperator, appScoped.RoleID)
	require.Equal(t, "alice", appScoped.Subject.ID)
	require.Equal(t, clientv4.SubjectTypeUser, appScoped.Subject.Type)
	require.Equal(t, []clientv4.ResourceRef{
		{Type: model.ResourceTypeApp, ID: "30001"}}, appScoped.Resources)

	bizScoped, ok := byScope[model.ResourceTypeBiz]
	require.True(t, ok, "biz scoped authorization is missing")
	require.Equal(t, model.RoleAppOperator, bizScoped.RoleID)
	require.Equal(t, []clientv4.ResourceRef{
		{Type: model.ResourceTypeBiz, ID: "2"}}, bizScoped.Resources)
}

// V4 要求 expired_at 必填且不超过 365 天，两条授权的到期时间应一致。
func TestGrantCreatorActionV4SetsExpiry(t *testing.T) {
	gw := new(fakeV4Gateway)
	a := newV4Auth(gw)

	before := time.Now()
	require.NoError(t, a.grantResourceCreatorActionV4(testKit(), creatorOption("30001", "2", "alice")))

	require.Len(t, gw.items, 2)
	require.Equal(t, gw.items[0].ExpiredAt, gw.items[1].ExpiredAt)

	expiry := time.Unix(gw.items[0].ExpiredAt, 0)
	require.True(t, expiry.After(before), "expiry should be in the future")
	require.LessOrEqual(t, expiry.Sub(before), clientv4.MaxAuthorizationExpireDuration,
		"expiry must not exceed the 365-day limit enforced by IAM")
}

// 缺少业务 ID 时必须报错而不是只授 app 维度：那样授权看似成功，
// 实际创建者会因过不了业务访问校验而对新服务毫无权限，属静默失败。
func TestGrantCreatorActionV4RequiresBizID(t *testing.T) {
	gw := new(fakeV4Gateway)
	a := newV4Auth(gw)

	err := a.grantResourceCreatorActionV4(testKit(), creatorOption("30001", "", "alice"))
	require.ErrorContains(t, err, "biz id is missing")
	require.Zero(t, gw.calls, "must not authorize partially")
}

func TestGrantCreatorActionV4ValidatesInput(t *testing.T) {
	gw := new(fakeV4Gateway)
	a := newV4Auth(gw)

	err := a.grantResourceCreatorActionV4(testKit(), nil)
	require.ErrorContains(t, err, "option is nil")

	err = a.grantResourceCreatorActionV4(testKit(), creatorOption("30001", "2", ""))
	require.ErrorContains(t, err, "creator is not set")

	// 目前只有服务需要创建者授权，与 V3 注册的范围一致
	opt := creatorOption("30001", "2", "alice")
	opt.Type = sys.Business
	err = a.grantResourceCreatorActionV4(testKit(), opt)
	require.ErrorContains(t, err, "unsupported resource type")

	require.Zero(t, gw.calls)
}

func TestGrantCreatorActionV4PropagatesError(t *testing.T) {
	gw := &fakeV4Gateway{err: errors.New("gateway rejected")}
	a := newV4Auth(gw)

	err := a.grantResourceCreatorActionV4(testKit(), creatorOption("30001", "2", "alice"))
	require.ErrorContains(t, err, "gateway rejected")
}

// 业务 ID 只按资源类型匹配，忽略 Ancestor.System——V4 的资源类型不带 system 维度，
// 而调用方传的仍是 V3 形态（System 为 bk_cmdb）。
func TestBizIDFromAncestors(t *testing.T) {
	require.Equal(t, "2", bizIDFromAncestors([]client.GrantResourceCreatorActionAncestor{
		{System: sys.SystemIDCMDB, Type: sys.Business, ID: "2"},
	}))

	// system 换成 BSCP 也应能取到
	require.Equal(t, "3", bizIDFromAncestors([]client.GrantResourceCreatorActionAncestor{
		{System: sys.SystemIDBSCP, Type: sys.Business, ID: "3"},
	}))

	// 多个祖先时取业务那一个
	require.Equal(t, "4", bizIDFromAncestors([]client.GrantResourceCreatorActionAncestor{
		{Type: sys.Application, ID: "999"},
		{Type: sys.Business, ID: "4"},
	}))

	require.Empty(t, bizIDFromAncestors(nil))
	require.Empty(t, bizIDFromAncestors([]client.GrantResourceCreatorActionAncestor{
		{Type: sys.Application, ID: "999"},
	}))
}

// 未启用 V4 时 useV4 必须为假，确保 V3 行为不受影响。
func TestUseV4Gating(t *testing.T) {
	require.False(t, new(Auth).useV4())
}

// 申请链接的祖先链必须与鉴权路径同源：旧拓扑下服务只声明业务一层，
// 漏传拓扑会让链接带上权限中心并不存在的 project/environment 节点而被拒绝。
func TestGenApplyURLV4FollowsTopology(t *testing.T) {
	cases := []struct {
		name     string
		topology meta.ResourceTopology
		want     []clientv4.ResourceRef
	}{
		{"legacy", meta.TopologyLegacy, []clientv4.ResourceRef{
			{Type: model.ResourceTypeBiz, ID: "100001"},
		}},
		{"hierarchical", meta.TopologyHierarchical, []clientv4.ResourceRef{
			{Type: model.ResourceTypeBiz, ID: "100001"},
			{Type: model.ResourceTypeProject, ID: "40001"},
			{Type: model.ResourceTypeEnvironment, ID: "50001"},
		}},
	}

	for _, c := range cases {
		gw := new(fakeV4Gateway)
		a := &Auth{v4Cli: gw, topology: c.topology}

		url, err := a.genApplyURLV4(testKit(), []*meta.ResourceAttribute{{
			Basic:         meta.Basic{Type: meta.App, Action: meta.View, ResourceID: 30001},
			BizID:         100001,
			ProjectID:     40001,
			EnvironmentID: 50001,
		}})
		require.NoError(t, err, c.name)
		require.NotEmpty(t, url, c.name)
		require.Len(t, gw.permissions, 1, c.name)
		require.Equal(t, model.ActionAppView, gw.permissions[0].ActionID, c.name)
		require.Len(t, gw.permissions[0].Resources, 1, c.name)
		require.Equal(t, c.want, gw.permissions[0].Resources[0].Ancestors, c.name)
	}
}
