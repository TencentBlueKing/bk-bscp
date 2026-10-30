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

package model

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// idPattern 是 IAM V4 对资源类型、操作与角色 ID 的格式要求。
var idPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,30}[a-z0-9]$`)

// validScopes 是每个资源类型的合法授权维度集合：自身加上全部祖先。
var validScopes = map[string]map[string]bool{
	ResourceTypeBiz: {ResourceTypeBiz: true},
	ResourceTypeApp: {
		ResourceTypeApp: true, ResourceTypeEnvironment: true,
		ResourceTypeProject: true, ResourceTypeBiz: true,
	},
	ResourceTypeProject:     {ResourceTypeProject: true, ResourceTypeBiz: true},
	ResourceTypeEnvironment: {ResourceTypeEnvironment: true, ResourceTypeProject: true, ResourceTypeBiz: true},
}

func TestIDsMatchIAMPattern(t *testing.T) {
	for _, rt := range ResourceTypes() {
		require.Regexp(t, idPattern, rt.ID)
		require.LessOrEqual(t, len(rt.ID), 32)
	}

	for _, a := range Actions() {
		require.Regexp(t, idPattern, a.ID)
		require.LessOrEqual(t, len(a.ID), 32)
	}

	for _, r := range Roles() {
		require.Regexp(t, idPattern, r.ID)
		require.LessOrEqual(t, len(r.ID), 32)
	}
}

func TestModelCounts(t *testing.T) {
	require.Len(t, ResourceTypes(), 4)
	// 项目 4 个 + 环境 4 个 + 服务与业务 17 个
	require.Len(t, Actions(), 25)
	require.Len(t, Roles(), 13)
	require.Len(t, DisplayResourceTypes(), 13)
}

func TestIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, a := range Actions() {
		require.False(t, seen[a.ID], "duplicated action id %s", a.ID)
		seen[a.ID] = true
	}

	seen = make(map[string]bool)
	for _, r := range Roles() {
		require.False(t, seen[r.ID], "duplicated role id %s", r.ID)
		seen[r.ID] = true
	}
}

// 每个操作关联的资源类型必须已注册。
func TestActionResourceTypesAreRegistered(t *testing.T) {
	registered := make(map[string]bool)
	for _, rt := range ResourceTypes() {
		registered[rt.ID] = true
	}

	for _, a := range Actions() {
		require.True(t, registered[a.ResourceTypeID],
			"action %s references unregistered resource type %s", a.ID, a.ResourceTypeID)
	}
}

// 角色中引用的操作必须都已定义，且其授权维度必须是该操作关联的资源类型本身或其祖先。
func TestRoleActionsAreValid(t *testing.T) {
	actionScope := make(map[string]string)
	for _, a := range Actions() {
		actionScope[a.ID] = a.ResourceTypeID
	}

	for _, role := range Roles() {
		require.NotEmpty(t, role.Actions, "role %s has no action", role.ID)

		for _, ra := range role.Actions {
			ownType, ok := actionScope[ra.ID]
			require.True(t, ok, "role %s references undefined action %s", role.ID, ra.ID)
			require.True(t, validScopes[ownType][ra.ResourceTypeID],
				"role %s action %s has invalid scope %s", role.ID, ra.ID, ra.ResourceTypeID)
		}
	}
}

// 每个角色的展示层级配置必须覆盖它实际用到的全部授权维度，
// 且展示粒度必须是授权粒度自身或其祖先。
func TestDisplayResourceTypesCoverRoleScopes(t *testing.T) {
	display := DisplayResourceTypes()

	for _, role := range Roles() {
		configured := make(map[string]string)
		for _, d := range display[role.ID] {
			configured[d.RelatedResourceTypeID] = d.DisplayResourceTypeID
		}

		for _, ra := range role.Actions {
			displayType, ok := configured[ra.ResourceTypeID]
			require.True(t, ok, "role %s scope %s has no display config", role.ID, ra.ResourceTypeID)
			require.True(t, validScopes[ra.ResourceTypeID][displayType],
				"role %s display %s is not an ancestor of scope %s",
				role.ID, displayType, ra.ResourceTypeID)
		}
	}
}

// 服务挂在环境下：只授服务的角色（服务运维/发布者/查看者）展示层级必须在环境，
// 否则授权页面只会在业务层展开，看不到项目与环境。
func TestAppRolesDisplayAtEnvironment(t *testing.T) {
	display := DisplayResourceTypes()

	for _, roleID := range []string{RoleAppOperator, RoleAppPublisher, RoleAppViewer} {
		var found bool
		for _, d := range display[roleID] {
			if d.RelatedResourceTypeID != ResourceTypeApp {
				continue
			}

			found = true
			require.Equal(t, ResourceTypeEnvironment, d.DisplayResourceTypeID,
				"role %s should display app at environment", roleID)
		}

		require.True(t, found, "role %s has no app display config", roleID)
	}
}

// 所有业务接口都有 find_business_resource 前置校验，任何角色缺了它都会被第一道校验拦住。
func TestEveryRoleGrantsBusinessAccess(t *testing.T) {
	for _, role := range Roles() {
		var found bool
		for _, ra := range role.Actions {
			if ra.ID == ActionFindBusinessResource {
				found = true
				require.Equal(t, ResourceTypeBiz, ra.ResourceTypeID)

				break
			}
		}
		require.True(t, found, "role %s does not grant %s", role.ID, ActionFindBusinessResource)
	}
}

// biz_operator 是「业务运维」，按业务维度声明全部操作：
// 项目、环境的授权维度都是 biz 的后代，声明在 biz 上合法，
// 语义是"该业务下的项目、环境、服务全部可操作"。
func TestBizOperatorCoversAllActions(t *testing.T) {
	all := make(map[string]bool, len(Actions()))
	for _, a := range Actions() {
		all[a.ID] = true
	}

	for _, role := range Roles() {
		if role.ID != RoleBizOperator {
			continue
		}

		require.Len(t, role.Actions, len(all))
		for _, ra := range role.Actions {
			require.True(t, all[ra.ID], "biz_operator references unknown action %s", ra.ID)
			require.Equal(t, ResourceTypeBiz, ra.ResourceTypeID,
				"biz_operator must grant every action on biz, got %s scoped action %s",
				ra.ResourceTypeID, ra.ID)
			delete(all, ra.ID)
		}

		require.Empty(t, all, "biz_operator misses actions: %v", all)

		return
	}

	t.Fatalf("role %s not found", RoleBizOperator)
}

// ancestors 是从根到父级的全链，不是直接父级：
// 子类型的祖先链必须等于父类型祖先链再追加父类型本身。
// 只写直接父级会让注册的拓扑比鉴权路径少一层（环境漏掉 biz 就是这个错）。
func TestResourceTypeAncestorsAreFullChain(t *testing.T) {
	ancestors := make(map[string][]string, len(ResourceTypes()))
	parents := make(map[string]string, len(ResourceTypes()))
	for _, rt := range ResourceTypes() {
		ancestors[rt.ID] = rt.Ancestors
		if len(rt.Ancestors) > 0 {
			parents[rt.ID] = rt.Ancestors[len(rt.Ancestors)-1]
		}
	}

	require.Equal(t, ResourceTypeEnvironment, parents[ResourceTypeApp], "app 必须直接挂在环境上")

	for id, parent := range parents {
		want := append(append([]string{}, ancestors[parent]...), parent)
		require.Equal(t, want, ancestors[id], "%s 的祖先链必须是从根到父级的全链", id)
	}
}

// 环境是独立授权资源：类型挂在项目下，查看/编辑/删除按环境实例授权，
// 创建按所属项目授权（创建时尚无环境实例）。
func TestEnvironmentResourceAndActions(t *testing.T) {
	var envType bool
	for _, rt := range ResourceTypes() {
		if rt.ID != ResourceTypeEnvironment {
			continue
		}
		envType = true
		// 祖先链是从根到父级的全链，鉴权路径 /biz,{bizID}/project,{projectID}/ 依赖它包含 biz。
		require.Equal(t, []string{ResourceTypeBiz, ResourceTypeProject}, rt.Ancestors,
			"environment should be under biz → project")
	}
	require.True(t, envType, "resource type %s is not registered", ResourceTypeEnvironment)

	want := map[string]string{
		ActionEnvironmentCreate: ResourceTypeProject,
		ActionEnvironmentView:   ResourceTypeEnvironment,
		ActionEnvironmentEdit:   ResourceTypeEnvironment,
		ActionEnvironmentDelete: ResourceTypeEnvironment,
	}

	for _, a := range Actions() {
		scope, ok := want[a.ID]
		if !ok {
			continue
		}
		require.Equal(t, scope, a.ResourceTypeID,
			"environment action %s should be authorized on %s", a.ID, scope)
		delete(want, a.ID)
	}

	require.Empty(t, want, "missing environment actions: %v", want)
}

// 项目隔离：除创建按业务授权外，查看/编辑/删除都必须是项目实例维度。
func TestProjectActionsAreScopedPerProject(t *testing.T) {
	want := map[string]string{
		ActionProjectCreate: ResourceTypeBiz,
		ActionProjectView:   ResourceTypeProject,
		ActionProjectEdit:   ResourceTypeProject,
		ActionProjectDelete: ResourceTypeProject,
	}

	for _, a := range Actions() {
		scope, ok := want[a.ID]
		if !ok {
			continue
		}
		require.Equal(t, scope, a.ResourceTypeID,
			"project action %s should be authorized on %s", a.ID, scope)
		delete(want, a.ID)
	}

	require.Empty(t, want, "missing project actions: %v", want)
}

// 项目、环境维度角色只在业务上保留业务访问，其余操作不能退回业务维度，
// 否则"只授某个项目/环境"会被业务授权整体放宽。
func TestProjectAndEnvironmentRolesKeepBizScopeOnlyForBusinessAccess(t *testing.T) {
	roles := map[string]bool{
		RoleProjectOperator: true, RoleProjectViewer: true,
		RoleEnvironmentOperator: true, RoleEnvironmentViewer: true,
	}

	for _, role := range Roles() {
		if !roles[role.ID] {
			continue
		}

		for _, ra := range role.Actions {
			if ra.ID == ActionFindBusinessResource {
				require.Equal(t, ResourceTypeBiz, ra.ResourceTypeID)
				continue
			}
			require.NotEqual(t, ResourceTypeBiz, ra.ResourceTypeID,
				"role %s must not grant %s on biz, it would cover the whole business", role.ID, ra.ID)
		}
	}
}

// 服务挂在环境下：项目/环境维度角色只继承「查看」，不得继承写操作。
//
// 写操作一旦声明在项目或环境维度，语义就是"该项目/环境下全部服务都可写"，
// 与"只授权特定服务"的诉求冲突（列表仍可看，未授权的服务靠逐项权限置灰并引导申请）。
func TestProjectAndEnvironmentOperatorsOnlyInheritAppView(t *testing.T) {
	wantScope := map[string]string{
		RoleProjectOperator:     ResourceTypeProject,
		RoleEnvironmentOperator: ResourceTypeEnvironment,
	}
	found := make(map[string]bool, len(wantScope))

	for _, role := range Roles() {
		scope, ok := wantScope[role.ID]
		if !ok {
			continue
		}

		for _, ra := range role.Actions {
			switch ra.ID {
			case ActionAppView:
				require.Equal(t, scope, ra.ResourceTypeID,
					"role %s should inherit app_view on %s", role.ID, scope)
				found[role.ID] = true
			case ActionAppEdit, ActionAppDelete, ActionReleaseGenerate, ActionReleasePublish:
				t.Fatalf("role %s must not inherit %s on %s: 服务写操作只能在服务维度授予",
					role.ID, ra.ID, scope)
			}
		}
	}

	for roleID := range wantScope {
		require.True(t, found[roleID], "role %s should inherit app_view", roleID)
	}
}

// 服务挂在环境下：app 的祖先链必须是「业务 → 项目 → 环境」，
// 项目/环境维度的授权才能通过路径继承覆盖到服务。
func TestAppAncestorsFollowEnvironmentChain(t *testing.T) {
	for _, rt := range ResourceTypes() {
		if rt.ID != ResourceTypeApp {
			continue
		}

		require.Equal(t, []string{ResourceTypeBiz, ResourceTypeProject, ResourceTypeEnvironment},
			rt.Ancestors, "app should be under biz → project → environment")

		return
	}

	t.Fatalf("resource type %s not found", ResourceTypeApp)
}

// app_creator 是唯一"能创建服务但不能读写他人服务"的角色，它的存在让创建者自动授权
// 产生真实的权限增量。三个断言各自锁住一条设计决策，改动前请先读注释。
func TestAppCreatorScopesCreationOnly(t *testing.T) {
	var actions map[string]string
	for _, role := range Roles() {
		if role.ID != RoleAppCreator {
			continue
		}

		actions = make(map[string]string, len(role.Actions))
		for _, ra := range role.Actions {
			actions[ra.ID] = ra.ResourceTypeID
		}
	}
	require.NotNil(t, actions, "role %s not found", RoleAppCreator)

	// 一、能建服务，且必须按环境授权——app_create 的语义是"允许在哪个环境下创建"。
	require.Equal(t, ResourceTypeEnvironment, actions[ActionAppCreate],
		"app_create must be granted per environment")

	// 二、放行业务访问前置校验，否则持有者连业务都进不去。
	require.Equal(t, ResourceTypeBiz, actions[ActionFindBusinessResource])

	// 三、刻意不含 app_view。这不是遗漏：app_view 在纯 biz 维度角色里只能按 biz 授权，
	// 一给就放开了业务下所有服务的详情访问，与本角色"他人服务需另行申请"的语义冲突。
	// 自己创建的服务由创建者自动授权（app_operator on 该 app）覆盖。
	require.NotContains(t, actions, ActionAppView,
		"app_view would leak read access to every app under the business")

	require.Len(t, actions, 2, "keep this role minimal")
}

// 服务创建者角色的展示层级必须下移到环境，授权页面才会展开「业务 → 项目 → 环境」，
// 让用户先选项目、再选环境。
func TestAppCreatorDisplaysEnvironmentChain(t *testing.T) {
	var envDisplay string
	for _, scope := range DisplayResourceTypes()[RoleAppCreator] {
		if scope.RelatedResourceTypeID == ResourceTypeEnvironment {
			envDisplay = scope.DisplayResourceTypeID
		}
	}

	require.Equal(t, ResourceTypeEnvironment, envDisplay,
		"app_create should be displayed on environment so the apply page drills biz -> project -> environment")
}

// 服务维度角色必须同时含 biz 与 app 两种授权维度，这是 IAM V4 允许角色跨维度的直接体现，
// 也是 dev 环境需要实测确认的关键假设。
func TestAppScopedRolesAreCrossDimension(t *testing.T) {
	appRoles := map[string]bool{
		RoleAppOperator: true, RoleAppPublisher: true, RoleAppViewer: true,
	}

	for _, role := range Roles() {
		if !appRoles[role.ID] {
			continue
		}

		scopes := make(map[string]bool)
		for _, ra := range role.Actions {
			scopes[ra.ResourceTypeID] = true
		}
		require.True(t, scopes[ResourceTypeBiz] && scopes[ResourceTypeApp],
			"role %s should span both biz and app scopes, got %v", role.ID, scopes)
	}
}
