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
	system   *client.System
	types    []client.ResourceType
	actions  []client.Action
	roles    []client.Role
	displays map[string][]client.DisplayResourceType

	calls []string
}

func newFakeGateway() *fakeGateway {
	return &fakeGateway{displays: make(map[string][]client.DisplayResourceType)}
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

	for id, types := range model.DisplayResourceTypes() {
		f.displays[id] = append([]client.DisplayResourceType(nil), types...)
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
		if f.types[i].ID == id {
			f.types[i].Name = req.Name
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
		f.actions = append(f.actions, a)
		ids = append(ids, a.ID)
	}

	return ids, nil
}

func (f *fakeGateway) UpdateAction(_ *kit.Kit, id string, req *client.UpdateActionReq) error {
	f.calls = append(f.calls, "update_action:"+id)
	for i := range f.actions {
		if f.actions[i].ID == id {
			f.actions[i].Name = req.Name
		}
	}

	return nil
}

func (f *fakeGateway) DeleteAction(_ *kit.Kit, id string) error {
	f.calls = append(f.calls, "delete_action:"+id)
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
		f.roles[i].Actions = append(f.roles[i].Actions, actions...)
		sortRoleActions(f.roles[i].Actions)
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

func (f *fakeGateway) ListRoleDisplayResourceTypes(_ *kit.Kit, roleID string) (
	[]client.DisplayResourceType, error) {

	return f.displays[roleID], nil
}

func (f *fakeGateway) UpdateRoleDisplayResourceTypes(_ *kit.Kit, roleID string,
	types []client.DisplayResourceType) error {

	f.calls = append(f.calls, "set_display:"+roleID)
	f.displays[roleID] = append([]client.DisplayResourceType(nil), types...)

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
	require.Len(t, plan.SetRoleDisplays, len(model.DisplayResourceTypes()))

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

// 祖先链与操作关联的资源类型都不可更新，差异必须报冲突而不是静默忽略。
func TestPlanReportsUnchangeableFieldsAsConflicts(t *testing.T) {
	gw := newFakeGateway().withFullModel()
	for i := range gw.types {
		if gw.types[i].ID == model.ResourceTypeApp {
			gw.types[i].Ancestors = nil
		}
	}
	for i := range gw.actions {
		if gw.actions[i].ID == model.ActionAppView {
			gw.actions[i].ResourceTypeID = model.ResourceTypeBiz
		}
	}

	s := mustSyncer(t, gw)
	plan, err := s.Plan(testKit())
	require.NoError(t, err)

	require.Len(t, plan.Conflicts, 2)

	// 有冲突时 Apply 必须拒绝执行，避免只改一半
	err = s.Apply(testKit(), plan, ApplyOption{})
	require.ErrorContains(t, err, "人工处理")
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

// 展示层级不一致要能识别；线上多返回本地未声明的键不算差异。
func TestPlanDiffsRoleDisplay(t *testing.T) {
	gw := newFakeGateway().withFullModel()
	gw.displays[model.RoleAppOperator] = []client.DisplayResourceType{
		{RelatedResourceTypeID: model.ResourceTypeBiz,
			DisplayResourceTypeID: model.ResourceTypeBiz},
		{RelatedResourceTypeID: model.ResourceTypeApp,
			DisplayResourceTypeID: model.ResourceTypeApp}, // 本地期望是 biz
	}

	s := mustSyncer(t, gw)
	plan, err := s.Plan(testKit())
	require.NoError(t, err)

	require.Len(t, plan.SetRoleDisplays, 1)
	require.Equal(t, model.RoleAppOperator, plan.SetRoleDisplays[0].RoleID)
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

// 新增角色时展示层级要随之配置，否则申请页的资源层级会是默认值。
func TestApplyConfiguresDisplayForNewRoles(t *testing.T) {
	gw := newFakeGateway()
	s := mustSyncer(t, gw)
	kt := testKit()

	plan, err := s.Plan(kt)
	require.NoError(t, err)
	require.NoError(t, s.Apply(kt, plan, ApplyOption{}))

	for id, want := range model.DisplayResourceTypes() {
		require.Equal(t, want, gw.displays[id], "role %s display not configured", id)
	}
}

func TestNewSyncerRejectsNilGateway(t *testing.T) {
	_, err := NewSyncer(nil, testSystemSpec)
	require.ErrorContains(t, err, "nil")
}
