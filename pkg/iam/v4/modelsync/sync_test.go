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

package modelsync

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/model"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

var testSystemSpec = SystemSpec{
	ID:          "bk-bscp",
	Name:        model.SystemName,
	Clients:     []string{"bk-bscp"},
	CallbackURL: "http://bscp.example.com" + model.CallbackPath,
}

// fakeGateway 在内存里模拟权限中心的模型存储，既能作为 Plan 的数据源，
// 也能承接 Apply 的写入，从而验证"Apply 之后再 Plan 应为空"这个幂等性质。
type fakeGateway struct {
	system  *client.System
	types   []client.ResourceType
	actions []client.Action
	roles   []client.Role

	calls []string

	// ignoreUpdates 模拟权限中心忽略祖先链的更新（只接受改名）
	ignoreUpdates bool
}

func newFakeGateway() *fakeGateway {
	return &fakeGateway{}
}

// withSystem 模拟系统已注册且与期望一致。
func (f *fakeGateway) withSystem() *fakeGateway {
	f.system = &client.System{
		ID: testSystemSpec.ID, Name: testSystemSpec.Name,
		Clients: testSystemSpec.Clients, CallbackURL: testSystemSpec.CallbackURL,
	}

	return f
}

func (f *fakeGateway) GetSystem(_ *kit.Kit) (*client.System, error) {
	if f.system == nil {
		return nil, &client.Error{Layer: client.ErrorLayerApp,
			HTTPStatus: http.StatusNotFound, Code: "NOT_FOUND"}
	}

	return f.system, nil
}

func (f *fakeGateway) CreateSystem(_ *kit.Kit, req *client.CreateSystemReq) (string, error) {
	f.calls = append(f.calls, "create_system")
	f.system = &client.System{ID: req.ID, Name: req.Name, Managers: req.Managers,
		Clients: req.Clients, CallbackURL: req.CallbackURL}

	return req.ID, nil
}

func (f *fakeGateway) UpdateSystem(_ *kit.Kit, req *client.UpdateSystemReq) error {
	f.calls = append(f.calls, "update_system")
	if f.system == nil {
		return errors.New("system not found")
	}

	if req.Name != "" {
		f.system.Name = req.Name
	}
	if req.CallbackURL != "" {
		f.system.CallbackURL = req.CallbackURL
	}
	if len(req.Clients) > 0 {
		f.system.Clients = req.Clients
	}

	return nil
}

// withFullModel 用本地定义填充线上状态，模拟"已经同步过"的环境，含系统本身。
// 角色的操作按 ID 字母序返回，与真实网关一致。
func (f *fakeGateway) withFullModel() *fakeGateway {
	f.withSystem()
	f.types = append(f.types, model.ResourceTypes()...)
	f.actions = append(f.actions, model.Actions()...)

	for _, r := range model.Roles() {
		role := r
		role.Actions = append([]client.RoleAction(nil), r.Actions...)
		sortRoleActions(role.Actions)
		f.roles = append(f.roles, role)
	}

	return f
}

func (f *fakeGateway) ListResourceTypes(_ *kit.Kit, page, _ int) (
	[]client.ResourceType, int, error) {

	if page > 1 {
		return nil, len(f.types), nil
	}

	return f.types, len(f.types), nil
}

func (f *fakeGateway) BatchCreateResourceTypes(_ *kit.Kit, types []client.ResourceType) (
	[]string, error) {

	f.calls = append(f.calls, "create_resource_types")
	ids := make([]string, 0, len(types))
	for _, t := range types {
		f.types = append(f.types, t)
		ids = append(ids, t.ID)
	}

	return ids, nil
}

func (f *fakeGateway) UpdateResourceType(_ *kit.Kit, id string,
	req *client.UpdateResourceTypeReq) error {

	f.calls = append(f.calls, "update_resource_type:"+id)
	for i := range f.types {
		if f.types[i].ID != id {
			continue
		}
		if req.Name != "" {
			f.types[i].Name = req.Name
		}
		// ignoreUpdates 模拟权限中心只接受改名、忽略祖先链的情况。
		if len(req.Ancestors) > 0 && !f.ignoreUpdates {
			f.types[i].Ancestors = append([]string(nil), req.Ancestors...)
		}
	}

	return nil
}

func (f *fakeGateway) DeleteResourceType(_ *kit.Kit, id string) error {
	f.calls = append(f.calls, "delete_resource_type:"+id)
	kept := f.types[:0]
	for _, t := range f.types {
		if t.ID != id {
			kept = append(kept, t)
		}
	}
	f.types = kept

	return nil
}

func (f *fakeGateway) ListActions(_ *kit.Kit, page, _ int) ([]client.Action, int, error) {
	if page > 1 {
		return nil, len(f.actions), nil
	}

	return f.actions, len(f.actions), nil
}

func (f *fakeGateway) BatchCreateActions(_ *kit.Kit, actions []client.Action) ([]string, error) {
	f.calls = append(f.calls, "create_actions")
	ids := make([]string, 0, len(actions))
	for _, a := range actions {
		for _, cur := range f.actions {
			if cur.ID == a.ID {
				return nil, &client.Error{Layer: client.ErrorLayerApp, HTTPStatus: http.StatusBadRequest,
					Code: "INVALID_ARGUMENT", Message: "action id `" + a.ID + "` already used"}
			}
		}
		f.actions = append(f.actions, a)
		ids = append(ids, a.ID)
	}

	return ids, nil
}

func (f *fakeGateway) UpdateAction(_ *kit.Kit, id string, req *client.UpdateActionReq) error {
	f.calls = append(f.calls, "update_action:"+id)
	for i := range f.actions {
		if f.actions[i].ID != id {
			continue
		}
		if req.Name != "" {
			f.actions[i].Name = req.Name
		}
	}

	return nil
}

func (f *fakeGateway) DeleteAction(_ *kit.Kit, id string) error {
	f.calls = append(f.calls, "delete_action:"+id)
	for _, r := range f.roles {
		for _, a := range r.Actions {
			if a.ID == id {
				return &client.Error{Layer: client.ErrorLayerApp, HTTPStatus: http.StatusBadRequest,
					Code: "INVALID_ARGUMENT", Message: "action `" + id + "` is used by role"}
			}
		}
	}

	kept := f.actions[:0]
	for _, a := range f.actions {
		if a.ID != id {
			kept = append(kept, a)
		}
	}
	f.actions = kept

	return nil
}

func (f *fakeGateway) ListRoles(_ *kit.Kit, page, _ int) ([]client.Role, int, error) {
	if page > 1 {
		return nil, len(f.roles), nil
	}

	return f.roles, len(f.roles), nil
}

func (f *fakeGateway) BatchCreateRoles(_ *kit.Kit, roles []client.Role) ([]string, error) {
	f.calls = append(f.calls, "create_roles")
	ids := make([]string, 0, len(roles))
	for _, r := range roles {
		if err := f.validateRoleActions(r.ID, r.Actions); err != nil {
			return nil, err
		}

		role := r
		role.Actions = append([]client.RoleAction(nil), r.Actions...)
		sortRoleActions(role.Actions)
		f.roles = append(f.roles, role)
		ids = append(ids, r.ID)
	}

	return ids, nil
}

func (f *fakeGateway) UpdateRole(_ *kit.Kit, id string, req *client.UpdateRoleReq) error {
	f.calls = append(f.calls, "update_role:"+id)
	for i := range f.roles {
		if f.roles[i].ID == id {
			f.roles[i].Name = req.Name
			f.roles[i].Description = req.Description
		}
	}

	return nil
}

func (f *fakeGateway) DeleteRole(_ *kit.Kit, id string) error {
	f.calls = append(f.calls, "delete_role:"+id)
	kept := f.roles[:0]
	for _, r := range f.roles {
		if r.ID != id {
			kept = append(kept, r)
		}
	}
	f.roles = kept

	return nil
}

func (f *fakeGateway) BatchCreateRoleActions(_ *kit.Kit, roleID string,
	actions []client.RoleAction) error {

	f.calls = append(f.calls, "add_role_actions:"+roleID)
	for i := range f.roles {
		if f.roles[i].ID != roleID {
			continue
		}

		merged := append(append([]client.RoleAction(nil), f.roles[i].Actions...), actions...)
		if err := f.validateRoleActions(roleID, merged); err != nil {
			return err
		}

		f.roles[i].Actions = append(f.roles[i].Actions, actions...)
		sortRoleActions(f.roles[i].Actions)
	}

	return nil
}

// validateRoleActions 模拟权限中心对角色内操作的两条校验：
// 按操作 ID 判重（授权维度不同也算重复）；关联资源类型相同的操作，授权维度必须一致。
func (f *fakeGateway) validateRoleActions(roleID string, actions []client.RoleAction) error {
	actionTypes := make(map[string]string, len(f.actions))
	for _, a := range f.actions {
		actionTypes[a.ID] = a.ResourceTypeID
	}

	seen := make(map[string]struct{}, len(actions))
	scopeByType := make(map[string]string)
	for _, a := range actions {
		if _, ok := seen[a.ID]; ok {
			return &client.Error{Layer: client.ErrorLayerApp, HTTPStatus: http.StatusBadRequest,
				Code:    "INVALID_ARGUMENT",
				Message: "The action `" + a.ID + "` is repeated in the role `" + roleID + "`"}
		}
		seen[a.ID] = struct{}{}

		ownType := actionTypes[a.ID]
		if scope, ok := scopeByType[ownType]; ok && scope != a.ResourceTypeID {
			return &client.Error{Layer: client.ErrorLayerApp, HTTPStatus: http.StatusBadRequest,
				Code: "INVALID_ARGUMENT",
				Message: "The resource type id `" + a.ResourceTypeID + "` of the action `" + a.ID +
					"` is different from other actions in the same resource type in role `" + roleID + "`"}
		}
		scopeByType[ownType] = a.ResourceTypeID
	}

	return nil
}

func (f *fakeGateway) BatchDeleteRoleActions(_ *kit.Kit, roleID string, actionIDs []string) error {
	f.calls = append(f.calls, "del_role_actions:"+roleID)
	drop := make(map[string]struct{}, len(actionIDs))
	for _, id := range actionIDs {
		drop[id] = struct{}{}
	}

	for i := range f.roles {
		if f.roles[i].ID != roleID {
			continue
		}
		kept := make([]client.RoleAction, 0, len(f.roles[i].Actions))
		for _, a := range f.roles[i].Actions {
			if _, ok := drop[a.ID]; !ok {
				kept = append(kept, a)
			}
		}
		f.roles[i].Actions = kept
	}

	return nil
}

func testKit() *kit.Kit {
	return &kit.Kit{Ctx: context.Background(), TenantID: "default", User: "tester"}
}

func mustSyncer(t *testing.T, gw Gateway) *Syncer {
	s, err := NewSyncer(gw, testSystemSpec)
	require.NoError(t, err)

	return s
}

// 系统尚未注册时，应连系统一起建，且模型全量新建。
func TestPlanRegistersSystemWhenAbsent(t *testing.T) {
	s := mustSyncer(t, newFakeGateway())

	plan, err := s.Plan(testKit())
	require.NoError(t, err)

	require.NotNil(t, plan.CreateSystem)
	require.Equal(t, testSystemSpec.ID, plan.CreateSystem.ID)
	require.Equal(t, testSystemSpec.CallbackURL, plan.CreateSystem.CallbackURL)
	require.Equal(t, testSystemSpec.Clients, plan.CreateSystem.Clients)
	require.Nil(t, plan.UpdateSystem)

	require.Len(t, plan.CreateResourceTypes, len(model.ResourceTypes()))
	require.Len(t, plan.CreateActions, len(model.Actions()))
	require.Len(t, plan.CreateRoles, len(model.Roles()))
}

// 系统必须先建再建模型，否则模型元素无处可挂。
func TestApplyCreatesSystemBeforeModel(t *testing.T) {
	gw := newFakeGateway()
	s := mustSyncer(t, gw)
	kt := testKit()

	plan, err := s.Plan(kt)
	require.NoError(t, err)
	require.NoError(t, s.Apply(kt, plan, ApplyOption{}))

	require.NotEmpty(t, gw.calls)
	require.Equal(t, "create_system", gw.calls[0], "system must be created first")
	require.Equal(t, "create_resource_types", gw.calls[1])
	require.Equal(t, "create_actions", gw.calls[2])
	require.Equal(t, "create_roles", gw.calls[3])
}

// 回调地址变更要能识别——这是切换部署环境时最常见的差异。
func TestPlanDetectsCallbackURLChange(t *testing.T) {
	gw := newFakeGateway().withFullModel()
	gw.system.CallbackURL = "http://old-host" + model.CallbackPath

	s := mustSyncer(t, gw)
	plan, err := s.Plan(testKit())
	require.NoError(t, err)

	require.NotNil(t, plan.UpdateSystem)
	require.Contains(t, plan.UpdateSystem.Detail, "回调地址")
	require.Equal(t, testSystemSpec.CallbackURL, plan.UpdateSystem.Req.CallbackURL)

	require.NoError(t, s.Apply(testKit(), plan, ApplyOption{}))
	require.Equal(t, testSystemSpec.CallbackURL, gw.system.CallbackURL)
}

// clients 是白名单，按集合补齐而不是覆盖，避免冲掉权限中心侧另行添加的调用方。
func TestPlanMergesClientsInsteadOfOverwriting(t *testing.T) {
	gw := newFakeGateway().withFullModel()
	gw.system.Clients = []string{"other-app"}

	s := mustSyncer(t, gw)
	kt := testKit()

	plan, err := s.Plan(kt)
	require.NoError(t, err)
	require.NotNil(t, plan.UpdateSystem)
	require.Contains(t, plan.UpdateSystem.Detail, "调用方新增")

	require.NoError(t, s.Apply(kt, plan, ApplyOption{}))
	require.ElementsMatch(t, []string{"other-app", "bk-bscp"}, gw.system.Clients,
		"existing client must be preserved")
}

func TestNewSyncerValidatesSystemSpec(t *testing.T) {
	_, err := NewSyncer(newFakeGateway(), SystemSpec{})
	require.ErrorContains(t, err, "id")

	_, err = NewSyncer(newFakeGateway(), SystemSpec{ID: "bk-bscp", Name: "x",
		Clients: []string{"c"}})
	require.ErrorContains(t, err, "callback url")
}

// 空系统上首次同步应创建全部模型元素，且不产生任何删除。
func TestPlanOnEmptySystemCreatesEverything(t *testing.T) {
	s := mustSyncer(t, newFakeGateway())

	plan, err := s.Plan(testKit())
	require.NoError(t, err)

	require.Len(t, plan.CreateResourceTypes, len(model.ResourceTypes()))
	require.Len(t, plan.CreateActions, len(model.Actions()))
	require.Len(t, plan.CreateRoles, len(model.Roles()))

	require.False(t, plan.HasDeletions())
	require.Empty(t, plan.Conflicts)
	require.True(t, plan.HasChanges())
}

// 幂等的核心断言：Apply 之后重新 Plan 必须为空。
func TestApplyIsIdempotent(t *testing.T) {
	gw := newFakeGateway()
	s := mustSyncer(t, gw)
	kt := testKit()

	first, err := s.Plan(kt)
	require.NoError(t, err)
	require.NoError(t, s.Apply(kt, first, ApplyOption{}))

	second, err := s.Plan(kt)
	require.NoError(t, err)
	require.False(t, second.HasChanges(), "second plan should be empty, got:\n%s", second)

	// 再 Apply 一次也不应产生任何写调用
	gw.calls = nil
	require.NoError(t, s.Apply(kt, second, ApplyOption{}))
	require.Empty(t, gw.calls)
}

// 线上已是最新状态时，Plan 为空——即便线上返回的操作顺序与本地定义不同。
func TestPlanOnSyncedSystemIsEmpty(t *testing.T) {
	s := mustSyncer(t, newFakeGateway().withFullModel())

	plan, err := s.Plan(testKit())
	require.NoError(t, err)
	require.False(t, plan.HasChanges(), "expected no changes, got:\n%s", plan)
}

// 名称变更走 update，不应触发重建。
func TestPlanDetectsRenames(t *testing.T) {
	gw := newFakeGateway().withFullModel()
	gw.types[0].Name = "旧业务名"
	gw.actions[0].Name = "旧操作名"
	gw.roles[0].Name = "旧角色名"
	gw.roles[0].Description = "旧描述"

	s := mustSyncer(t, gw)
	plan, err := s.Plan(testKit())
	require.NoError(t, err)

	require.Len(t, plan.RenameResourceTypes, 1)
	require.Equal(t, "旧业务名", plan.RenameResourceTypes[0].From)
	require.Len(t, plan.RenameActions, 1)
	require.Len(t, plan.UpdateRoleMetas, 1)
	require.Contains(t, plan.UpdateRoleMetas[0].Detail, "旧角色名")

	require.Empty(t, plan.CreateResourceTypes)
	require.Empty(t, plan.CreateActions)
	require.Empty(t, plan.CreateRoles)
}

// withLegacyAppCreate 把线上 app_create 还原为业务维度，appCreatorBinding 为 false 时
// 模拟上一次同步已把它从 app_creator 中移除、但未能加回的中间态。
func (f *fakeGateway) withLegacyAppCreate(appCreatorBinding bool) *fakeGateway {
	for i := range f.actions {
		if f.actions[i].ID == model.ActionAppCreate {
			f.actions[i].ResourceTypeID = model.ResourceTypeBiz
		}
	}

	for i := range f.roles {
		if f.roles[i].ID != model.RoleAppCreator {
			continue
		}
		f.roles[i].Actions = []client.RoleAction{
			{ID: model.ActionFindBusinessResource, ResourceTypeID: model.ResourceTypeBiz},
		}
		if appCreatorBinding {
			f.roles[i].Actions = append(f.roles[i].Actions,
				client.RoleAction{ID: model.ActionAppCreate, ResourceTypeID: model.ResourceTypeBiz})
			sortRoleActions(f.roles[i].Actions)
		}
	}

	return f
}

// 祖先链走原地更新；操作关联资源类型的更新会被权限中心忽略，只能重建该操作。
func TestPlanUpdatesTopologyInPlace(t *testing.T) {
	gw := newFakeGateway().withFullModel().withLegacyAppCreate(true)
	for i := range gw.types {
		if gw.types[i].ID == model.ResourceTypeApp {
			gw.types[i].Ancestors = []string{model.ResourceTypeBiz}
			gw.types[i].Name = "旧服务名"
		}
	}

	s := mustSyncer(t, gw)
	plan, err := s.Plan(testKit())
	require.NoError(t, err)

	require.Empty(t, plan.Conflicts)
	require.Empty(t, plan.DeleteResourceTypes)
	require.Empty(t, plan.DeleteActions)
	require.Len(t, plan.UpdateResourceTypeAncestors, 1)
	require.Equal(t, model.ResourceTypeApp, plan.UpdateResourceTypeAncestors[0].ID)
	require.Len(t, plan.RecreateActions, 1)
	require.Equal(t, model.ActionAppCreate, plan.RecreateActions[0].Action.ID)
	require.Equal(t, model.ResourceTypeEnvironment, plan.RecreateActions[0].Action.ResourceTypeID)
	require.Equal(t, []string{model.RoleAppCreator, model.RoleBizOperator}, plan.RecreateActions[0].Roles)

	require.NoError(t, s.Apply(testKit(), plan, ApplyOption{}))

	// 祖先链更新已带上名称，同一次变更只发一个请求
	require.Equal(t, 1, countCalls(gw.calls, "update_resource_type:"+model.ResourceTypeApp))
	require.Zero(t, countCalls(gw.calls, "update_action:"+model.ActionAppCreate))

	// 施加后再次 Plan 应该没有变更，说明更新确实生效
	again, err := s.Plan(testKit())
	require.NoError(t, err)
	require.False(t, again.HasChanges(), "expected no changes, got:\n%s", again)
}

// 重现线上：app_create 仍关联业务，且上一次同步已把它从 app_creator 中移除。
// 直接给 app_creator 加 app_create@environment 会被拒，必须先重建操作；
// 重建前要从 biz_operator 中移除，重建后再按本地定义把两个角色的绑定加回。
func TestApplyRecreatesActionWhenResourceTypeChanges(t *testing.T) {
	gw := newFakeGateway().withFullModel().withLegacyAppCreate(false)
	s := mustSyncer(t, gw)
	kt := testKit()

	err := gw.BatchCreateRoleActions(kt, model.RoleAppCreator, []client.RoleAction{
		{ID: model.ActionAppCreate, ResourceTypeID: model.ResourceTypeEnvironment}})
	require.ErrorContains(t, err, "is different from other actions in the same resource type")

	plan, err := s.Plan(kt)
	require.NoError(t, err)
	require.Len(t, plan.RecreateActions, 1)
	require.Equal(t, []string{model.RoleBizOperator}, plan.RecreateActions[0].Roles)
	require.Empty(t, plan.DelRoleActions, "recreation removes the bindings itself")

	gw.calls = nil
	require.NoError(t, s.Apply(kt, plan, ApplyOption{}))
	require.Equal(t, []string{
		"del_role_actions:" + model.RoleBizOperator,
		"delete_action:" + model.ActionAppCreate,
		"create_actions",
		"add_role_actions:" + model.RoleBizOperator,
		"add_role_actions:" + model.RoleAppCreator,
	}, gw.calls[:5])

	again, err := s.Plan(kt)
	require.NoError(t, err)
	require.False(t, again.HasChanges(), "expected no changes, got:\n%s", again)
}

// 重建中途失败（操作已删、未建）后重跑，应按新建处理并补回全部角色绑定。
func TestApplyResumesAfterActionDeleted(t *testing.T) {
	gw := newFakeGateway().withFullModel().withLegacyAppCreate(false)
	require.NoError(t, gw.BatchDeleteRoleActions(testKit(), model.RoleBizOperator,
		[]string{model.ActionAppCreate}))
	require.NoError(t, gw.DeleteAction(testKit(), model.ActionAppCreate))

	s := mustSyncer(t, gw)
	kt := testKit()

	plan, err := s.Plan(kt)
	require.NoError(t, err)
	require.Empty(t, plan.RecreateActions)
	require.Len(t, plan.CreateActions, 1)

	require.NoError(t, s.Apply(kt, plan, ApplyOption{}))

	again, err := s.Plan(kt)
	require.NoError(t, err)
	require.False(t, again.HasChanges(), "expected no changes, got:\n%s", again)
}

func countCalls(calls []string, target string) int {
	var count int
	for _, c := range calls {
		if c == target {
			count++
		}
	}

	return count
}

// 权限中心若忽略祖先链，更新请求也会成功；必须回读校验并让命令失败，
// 否则会出现「模型显示已对齐、实际按旧拓扑鉴权」的静默错误。
func TestApplyFailsWhenGatewayIgnoresTopologyUpdates(t *testing.T) {
	gw := newFakeGateway().withFullModel()
	gw.ignoreUpdates = true
	for i := range gw.types {
		if gw.types[i].ID == model.ResourceTypeApp {
			gw.types[i].Ancestors = []string{model.ResourceTypeBiz}
		}
	}

	s := mustSyncer(t, gw)
	plan, err := s.Plan(testKit())
	require.NoError(t, err)
	require.Len(t, plan.UpdateResourceTypeAncestors, 1)

	err = s.Apply(testKit(), plan, ApplyOption{})
	require.ErrorContains(t, err, "delete-iam-v4")
	require.ErrorContains(t, err, model.ResourceTypeApp)
}

// 角色内操作的增删要能识别，且换了授权维度视为重建。
func TestPlanDiffsRoleActions(t *testing.T) {
	gw := newFakeGateway().withFullModel()

	var target int
	for i := range gw.roles {
		if gw.roles[i].ID == model.RoleAppOperator {
			target = i

			break
		}
	}

	// 线上少一个操作，多一个本地没有的操作，另有一个操作的授权维度不同
	gw.roles[target].Actions = []client.RoleAction{
		{ID: model.ActionFindBusinessResource, ResourceTypeID: model.ResourceTypeBiz},
		{ID: model.ActionAppView, ResourceTypeID: model.ResourceTypeBiz},   // 维度本应是 app
		{ID: model.ActionAuditView, ResourceTypeID: model.ResourceTypeBiz}, // 本地没有
	}

	s := mustSyncer(t, gw)
	plan, err := s.Plan(testKit())
	require.NoError(t, err)

	require.Len(t, plan.AddRoleActions, 1)
	require.Len(t, plan.DelRoleActions, 1)

	added := make([]string, 0)
	for _, a := range plan.AddRoleActions[0].Actions {
		added = append(added, a.ID)
	}
	sort.Strings(added)
	// app_view 换回 app 维度也要重新添加
	require.Contains(t, added, model.ActionAppView)
	require.Contains(t, added, model.ActionAppEdit)

	removed := make([]string, 0)
	for _, a := range plan.DelRoleActions[0].Actions {
		removed = append(removed, a.ID)
	}
	require.Contains(t, removed, model.ActionAuditView)
	require.Contains(t, removed, model.ActionAppView)
}

// 权限中心按操作 ID 判重，同一操作换授权维度时必须先移除旧维度再添加新维度，
// 否则会报 The action `app_create` is repeated in the role `app_creator`。
func TestApplyRescopesRoleActionByRemovingFirst(t *testing.T) {
	gw := newFakeGateway().withFullModel()
	for i := range gw.roles {
		if gw.roles[i].ID != model.RoleAppCreator {
			continue
		}
		gw.roles[i].Actions = []client.RoleAction{
			{ID: model.ActionAppCreate, ResourceTypeID: model.ResourceTypeBiz},
			{ID: model.ActionFindBusinessResource, ResourceTypeID: model.ResourceTypeBiz},
		}
	}

	s := mustSyncer(t, gw)
	kt := testKit()

	plan, err := s.Plan(kt)
	require.NoError(t, err)
	require.Len(t, plan.AddRoleActions, 1)
	require.Len(t, plan.DelRoleActions, 1)

	gw.calls = nil
	require.NoError(t, s.Apply(kt, plan, ApplyOption{}))
	require.Equal(t, []string{
		"del_role_actions:" + model.RoleAppCreator,
		"add_role_actions:" + model.RoleAppCreator,
	}, gw.calls, "rescoped action must be removed once before being re-added")

	again, err := s.Plan(kt)
	require.NoError(t, err)
	require.False(t, again.HasChanges(), "expected no changes, got:\n%s", again)
}

// 换维度的操作先删，其余新增仍先于移除，避免角色中途出现空窗。
func TestApplyKeepsAddBeforeRemoveForOtherActions(t *testing.T) {
	gw := newFakeGateway().withFullModel()
	for i := range gw.roles {
		if gw.roles[i].ID != model.RoleAppCreator {
			continue
		}
		gw.roles[i].Actions = []client.RoleAction{
			{ID: model.ActionAppCreate, ResourceTypeID: model.ResourceTypeBiz},
			{ID: model.ActionAuditView, ResourceTypeID: model.ResourceTypeBiz},
		}
	}

	s := mustSyncer(t, gw)
	kt := testKit()

	plan, err := s.Plan(kt)
	require.NoError(t, err)

	gw.calls = nil
	require.NoError(t, s.Apply(kt, plan, ApplyOption{}))
	require.Equal(t, []string{
		"del_role_actions:" + model.RoleAppCreator,
		"add_role_actions:" + model.RoleAppCreator,
		"del_role_actions:" + model.RoleAppCreator,
	}, gw.calls)

	again, err := s.Plan(kt)
	require.NoError(t, err)
	require.False(t, again.HasChanges(), "expected no changes, got:\n%s", again)
}

// 线上多余的元素默认只报告不删除，避免误伤既有授权。
func TestApplySkipsDeletionsByDefault(t *testing.T) {
	gw := newFakeGateway().withFullModel()
	gw.roles = append(gw.roles, client.Role{ID: "legacy_role", Name: "历史角色"})
	gw.actions = append(gw.actions, client.Action{ID: "legacy_action", Name: "历史操作"})

	s := mustSyncer(t, gw)
	kt := testKit()

	plan, err := s.Plan(kt)
	require.NoError(t, err)
	require.True(t, plan.HasDeletions())
	require.Equal(t, []string{"legacy_role"}, plan.DeleteRoles)
	require.Equal(t, []string{"legacy_action"}, plan.DeleteActions)

	gw.calls = nil
	require.NoError(t, s.Apply(kt, plan, ApplyOption{}))
	require.Empty(t, gw.calls, "deletions must not happen without AllowDelete")
	require.Len(t, gw.roles, len(model.Roles())+1)
}

// 显式开启后才删除，且顺序为角色→操作→资源类型，因为被引用者不能先删。
func TestApplyDeletesInDependencyOrder(t *testing.T) {
	gw := newFakeGateway().withFullModel()
	gw.roles = append(gw.roles, client.Role{ID: "legacy_role", Name: "历史角色"})
	gw.actions = append(gw.actions, client.Action{ID: "legacy_action", Name: "历史操作"})
	gw.types = append(gw.types, client.ResourceType{ID: "legacy_type", Name: "历史资源"})

	s := mustSyncer(t, gw)
	kt := testKit()

	plan, err := s.Plan(kt)
	require.NoError(t, err)

	gw.calls = nil
	require.NoError(t, s.Apply(kt, plan, ApplyOption{AllowDelete: true}))

	require.Equal(t, []string{
		"delete_role:legacy_role",
		"delete_action:legacy_action",
		"delete_resource_type:legacy_type",
	}, gw.calls)

	// 删完之后再 Plan 应为空
	after, err := s.Plan(kt)
	require.NoError(t, err)
	require.False(t, after.HasChanges())
}

// delete-iam-v4 走的是不比较差异的计划：本地定义仍然存在，但线上元素必须全部列为删除。
func TestPlanDeleteAllListsEveryOnlineElement(t *testing.T) {
	gw := newFakeGateway().withFullModel()
	s := mustSyncer(t, gw)

	plan, err := s.PlanDeleteAll(testKit())
	require.NoError(t, err)

	require.Empty(t, plan.CreateRoles, "cleanup must not create anything")
	require.Empty(t, plan.CreateActions)
	require.Empty(t, plan.CreateResourceTypes)
	require.Len(t, plan.DeleteRoles, len(model.Roles()))
	require.Len(t, plan.DeleteActions, len(model.Actions()))
	require.Len(t, plan.DeleteResourceTypes, len(model.ResourceTypes()))
	require.True(t, plan.HasDeletions())
}

// 清空后线上不应剩下任何角色、操作与资源类型，系统本身保留。
func TestApplyDeleteAllEmptiesModel(t *testing.T) {
	gw := newFakeGateway().withFullModel()
	s := mustSyncer(t, gw)
	kt := testKit()

	plan, err := s.PlanDeleteAll(kt)
	require.NoError(t, err)

	require.NoError(t, s.Apply(kt, plan, ApplyOption{AllowDelete: true}))

	require.Empty(t, gw.roles)
	require.Empty(t, gw.actions)
	require.Empty(t, gw.types)
	require.NotNil(t, gw.system, "system itself must be kept")
	require.Equal(t, testSystemSpec.CallbackURL, gw.system.CallbackURL)
}

// 系统未注册时没有可清空的模型，直接报错而不是静默成功。
func TestPlanDeleteAllRejectsUnregisteredSystem(t *testing.T) {
	s := mustSyncer(t, newFakeGateway())

	_, err := s.PlanDeleteAll(testKit())
	require.ErrorContains(t, err, "无需清理")
}

// 同一 ID 在线上可能重复出现，删除前要去重，否则删除请求会因重复项失败。
func TestUniqueIDs(t *testing.T) {
	require.Equal(t, []string{"a", "b"}, uniqueIDs([]string{"b", "a", "b"}))
	require.Empty(t, uniqueIDs(nil))
}

func TestNewSyncerRejectsNilGateway(t *testing.T) {
	_, err := NewSyncer(nil, testSystemSpec)
	require.ErrorContains(t, err, "nil")
}
