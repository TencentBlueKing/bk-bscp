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
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/pkg/errors"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/model"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// Gateway 同步模型所需的 IAM V4 权限中心能力
type Gateway interface {
	GetSystem(kt *kit.Kit) (*client.System, error)
	CreateSystem(kt *kit.Kit, req *client.CreateSystemReq) (string, error)
	UpdateSystem(kt *kit.Kit, req *client.UpdateSystemReq) error

	ListResourceTypes(kt *kit.Kit, page, pageSize int) ([]client.ResourceType, int, error)
	BatchCreateResourceTypes(kt *kit.Kit, types []client.ResourceType) ([]string, error)
	UpdateResourceType(kt *kit.Kit, id string, req *client.UpdateResourceTypeReq) error
	DeleteResourceType(kt *kit.Kit, id string) error

	ListActions(kt *kit.Kit, page, pageSize int) ([]client.Action, int, error)
	BatchCreateActions(kt *kit.Kit, actions []client.Action) ([]string, error)
	UpdateAction(kt *kit.Kit, id string, req *client.UpdateActionReq) error
	DeleteAction(kt *kit.Kit, id string) error

	ListRoles(kt *kit.Kit, page, pageSize int) ([]client.Role, int, error)
	BatchCreateRoles(kt *kit.Kit, roles []client.Role) ([]string, error)
	UpdateRole(kt *kit.Kit, id string, req *client.UpdateRoleReq) error
	DeleteRole(kt *kit.Kit, id string) error
	BatchCreateRoleActions(kt *kit.Kit, roleID string, actions []client.RoleAction) error
	BatchDeleteRoleActions(kt *kit.Kit, roleID string, actionIDs []string) error

	ListRoleDisplayResourceTypes(kt *kit.Kit, roleID string) ([]client.DisplayResourceType, error)
	UpdateRoleDisplayResourceTypes(kt *kit.Kit, roleID string,
		types []client.DisplayResourceType) error
}

var _ Gateway = (*client.Client)(nil)

// SystemSpec 系统本身的期望状态
type SystemSpec struct {
	ID   string
	Name string
	// Clients 允许调用本系统权限接口的蓝鲸应用列表，不在列表内的应用会被拒绝
	Clients []string
	// CallbackURL 权限中心拉取资源实例的完整地址
	CallbackURL string
	// Managers 系统管理员，为空时不改动线上取值
	Managers []string
}

func (s SystemSpec) validate() error {
	missing := make([]string, 0, 3)
	if s.ID == "" {
		missing = append(missing, "id")
	}
	if s.Name == "" {
		missing = append(missing, "name")
	}
	if s.CallbackURL == "" {
		missing = append(missing, "callback url")
	}
	if len(s.Clients) == 0 {
		missing = append(missing, "clients")
	}

	if len(missing) > 0 {
		return errors.Errorf("system spec 缺少 %s", strings.Join(missing, ", "))
	}

	return nil
}

// Syncer 把本地模型定义同步到权限中心。
type Syncer struct {
	gw     Gateway
	system SystemSpec
}

// NewSyncer 构造同步器，system 为系统本身的期望状态
func NewSyncer(gw Gateway, system SystemSpec) (*Syncer, error) {
	if gw == nil {
		return nil, errors.New("iam v4 gateway is nil")
	}

	if err := system.validate(); err != nil {
		return nil, err
	}

	return &Syncer{gw: gw, system: system}, nil
}

// ApplyOption 控制 Apply 的行为
type ApplyOption struct {
	// AllowDelete 为真时才删除线上多余的资源类型、操作与角色。
	//
	// 默认关闭：删除角色会使该角色下的全部授权失效，删除操作会使引用它的角色
	// 失去该权限，两者的影响均不可逆。日常同步只做新增与更新，
	// 清理由 prune 子命令显式发起。
	AllowDelete bool
	// Logf 输出每一步的进展，为空时静默执行
	Logf func(format string, args ...interface{})
}

func (o *ApplyOption) logf(format string, args ...interface{}) {
	if o.Logf == nil {
		return
	}

	o.Logf(format, args...)
}

// Plan 拉取线上模型并与本地定义求差异。只读，不产生任何变更。
func (s *Syncer) Plan(kt *kit.Kit) (*Plan, error) {
	plan := new(Plan)

	systemExists, err := s.diffSystem(kt, plan)
	if err != nil {
		return nil, err
	}

	// 系统尚未注册时，其下的模型元素必然全都不存在，再去 list 只会拿到 404。
	// 直接按"全部新建"出计划，Apply 会先建系统再建模型。
	if !systemExists {
		s.planFullModel(plan)

		return plan, nil
	}

	if err := s.diffModel(kt, plan); err != nil {
		return nil, err
	}

	return plan, nil
}

// PlanDeleteAll 把线上全部模型元素列为删除，不与本地定义比较。
//
// Plan 只找出本地已移除的元素，而资源类型的祖先链、操作关联的资源类型不支持更新，
// 这类字段要变更只能先整体清空再重建，因此需要这个不比较差异的计划。
// 系统本身不删：回调地址与 clients 白名单必须保留，否则重建期间鉴权全部不可用。
func (s *Syncer) PlanDeleteAll(kt *kit.Kit) (*Plan, error) {
	if _, err := s.gw.GetSystem(kt); err != nil {
		if client.IsNotFound(err) {
			return nil, errors.Errorf("权限中心未注册 %s 系统，无需清理", s.system.ID)
		}

		return nil, errors.Wrap(err, "get online system")
	}

	roles, err := s.listAllRoles(kt)
	if err != nil {
		return nil, errors.Wrap(err, "list online roles")
	}

	actions, err := s.listAllActions(kt)
	if err != nil {
		return nil, errors.Wrap(err, "list online actions")
	}

	types, err := s.listAllResourceTypes(kt)
	if err != nil {
		return nil, errors.Wrap(err, "list online resource types")
	}

	plan := new(Plan)

	roleIDs := make([]string, 0, len(roles))
	for _, r := range roles {
		roleIDs = append(roleIDs, r.ID)
	}
	plan.DeleteRoles = uniqueIDs(roleIDs)

	actionIDs := make([]string, 0, len(actions))
	for _, a := range actions {
		actionIDs = append(actionIDs, a.ID)
	}
	plan.DeleteActions = uniqueIDs(actionIDs)

	typeIDs := make([]string, 0, len(types))
	for _, t := range types {
		typeIDs = append(typeIDs, t.ID)
	}
	plan.DeleteResourceTypes = uniqueIDs(typeIDs)

	return plan, nil
}

// uniqueIDs 去重并排序。权限中心可能对同一 ID 返回多条记录（重复注册残留），
// 重复的 ID 会让删除请求失败。
func uniqueIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}

	sort.Strings(result)

	return result
}

// diffSystem 比较系统本身，返回系统当前是否已注册。
func (s *Syncer) diffSystem(kt *kit.Kit, plan *Plan) (bool, error) {
	online, err := s.gw.GetSystem(kt)
	if err != nil {
		if !client.IsNotFound(err) {
			return false, errors.Wrap(err, "get online system")
		}

		plan.CreateSystem = &client.CreateSystemReq{
			ID: s.system.ID, Name: s.system.Name, Managers: s.system.Managers,
			Clients: s.system.Clients, CallbackURL: s.system.CallbackURL,
		}

		return false, nil
	}

	var parts []string
	if online.Name != s.system.Name {
		parts = append(parts, fmt.Sprintf("名称 %q -> %q", online.Name, s.system.Name))
	}
	if online.CallbackURL != s.system.CallbackURL {
		parts = append(parts, fmt.Sprintf("回调地址 %q -> %q",
			online.CallbackURL, s.system.CallbackURL))
	}
	// clients 是白名单，缺了会导致 BSCP 调不通鉴权接口，因此按集合补齐而非覆盖，
	// 避免把权限中心侧另行添加的调用方冲掉。
	merged, added := mergeClients(online.Clients, s.system.Clients)
	if len(added) > 0 {
		parts = append(parts, fmt.Sprintf("调用方新增 %v", added))
	}

	if len(parts) == 0 {
		return true, nil
	}

	plan.UpdateSystem = &SystemUpdate{
		Req: &client.UpdateSystemReq{
			Name: s.system.Name, Clients: merged, CallbackURL: s.system.CallbackURL,
		},
		Detail: strings.Join(parts, "；"),
	}

	return true, nil
}

// planFullModel 在系统尚未注册时，直接把本地模型全量列为新建。
func (s *Syncer) planFullModel(plan *Plan) {
	plan.CreateResourceTypes = model.ResourceTypes()
	plan.CreateActions = model.Actions()
	plan.CreateRoles = model.Roles()

	for _, role := range model.Roles() {
		want, ok := model.DisplayResourceTypes()[role.ID]
		if !ok {
			continue
		}

		plan.SetRoleDisplays = append(plan.SetRoleDisplays, RoleDisplayChange{
			RoleID: role.ID, Types: want,
			Detail: formatDisplayTypes(want) + "（随新角色配置）",
		})
	}
}

func (s *Syncer) diffModel(kt *kit.Kit, plan *Plan) error {
	onlineTypes, err := s.listAllResourceTypes(kt)
	if err != nil {
		return errors.Wrap(err, "list online resource types")
	}

	onlineActions, err := s.listAllActions(kt)
	if err != nil {
		return errors.Wrap(err, "list online actions")
	}

	onlineRoles, err := s.listAllRoles(kt)
	if err != nil {
		return errors.Wrap(err, "list online roles")
	}

	s.diffResourceTypes(onlineTypes, plan)
	s.diffActions(onlineActions, plan)

	return s.diffRoles(kt, onlineRoles, plan)
}

// mergeClients 把期望的调用方并入线上列表，返回合并结果与新增项。
func mergeClients(online, want []string) (merged, added []string) {
	seen := make(map[string]struct{}, len(online))
	merged = make([]string, 0, len(online)+len(want))
	for _, c := range online {
		seen[c] = struct{}{}
		merged = append(merged, c)
	}

	for _, c := range want {
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		merged = append(merged, c)
		added = append(added, c)
	}

	return merged, added
}

func (s *Syncer) diffResourceTypes(online []client.ResourceType, plan *Plan) {
	onlineMap := make(map[string]client.ResourceType, len(online))
	for _, rt := range online {
		onlineMap[rt.ID] = rt
	}

	localMap := make(map[string]struct{})
	for _, local := range model.ResourceTypes() {
		localMap[local.ID] = struct{}{}

		cur, ok := onlineMap[local.ID]
		if !ok {
			plan.CreateResourceTypes = append(plan.CreateResourceTypes, local)

			continue
		}

		// 祖先链不可更新，不一致只能报冲突。空切片与 nil 视为相同。
		if !sameStrings(cur.Ancestors, local.Ancestors) {
			plan.Conflicts = append(plan.Conflicts, Conflict{
				Kind: "资源类型", ID: local.ID, Field: "ancestors",
				Online: fmt.Sprint(cur.Ancestors), Local: fmt.Sprint(local.Ancestors),
			})
		}

		if cur.Name != local.Name {
			plan.RenameResourceTypes = append(plan.RenameResourceTypes,
				Rename{ID: local.ID, From: cur.Name, To: local.Name})
		}
	}

	for _, rt := range online {
		if _, ok := localMap[rt.ID]; !ok {
			plan.DeleteResourceTypes = append(plan.DeleteResourceTypes, rt.ID)
		}
	}
}

func (s *Syncer) diffActions(online []client.Action, plan *Plan) {
	onlineMap := make(map[string]client.Action, len(online))
	for _, a := range online {
		onlineMap[a.ID] = a
	}

	localMap := make(map[string]struct{})
	for _, local := range model.Actions() {
		localMap[local.ID] = struct{}{}

		cur, ok := onlineMap[local.ID]
		if !ok {
			plan.CreateActions = append(plan.CreateActions, local)

			continue
		}

		// 关联资源类型不可更新，同样只能报冲突。
		if cur.ResourceTypeID != local.ResourceTypeID {
			plan.Conflicts = append(plan.Conflicts, Conflict{
				Kind: "操作", ID: local.ID, Field: "resource_type_id",
				Online: cur.ResourceTypeID, Local: local.ResourceTypeID,
			})
		}

		if cur.Name != local.Name {
			plan.RenameActions = append(plan.RenameActions,
				Rename{ID: local.ID, From: cur.Name, To: local.Name})
		}
	}

	for _, a := range online {
		if _, ok := localMap[a.ID]; !ok {
			plan.DeleteActions = append(plan.DeleteActions, a.ID)
		}
	}
}

func (s *Syncer) diffRoles(kt *kit.Kit, online []client.Role, plan *Plan) error {
	onlineMap := make(map[string]client.Role, len(online))
	for _, r := range online {
		onlineMap[r.ID] = r
	}

	displays := model.DisplayResourceTypes()

	localMap := make(map[string]struct{})
	for _, local := range model.Roles() {
		localMap[local.ID] = struct{}{}

		cur, ok := onlineMap[local.ID]
		if !ok {
			plan.CreateRoles = append(plan.CreateRoles, local)
			// 新角色的展示层级在创建后一并配置
			if want, has := displays[local.ID]; has {
				plan.SetRoleDisplays = append(plan.SetRoleDisplays, RoleDisplayChange{
					RoleID: local.ID, Types: want,
					Detail: formatDisplayTypes(want) + "（随新角色配置）",
				})
			}

			continue
		}

		if detail := roleMetaDiff(cur, local); detail != "" {
			plan.UpdateRoleMetas = append(plan.UpdateRoleMetas, RoleMetaUpdate{
				ID: local.ID, Name: local.Name, Description: local.Description, Detail: detail,
			})
		}

		// 线上返回的操作按 ID 字母序排列，与本地定义顺序无关，只能按集合比较。
		add, del := diffRoleActions(cur.Actions, local.Actions)
		if len(add) > 0 {
			plan.AddRoleActions = append(plan.AddRoleActions,
				RoleActionsChange{RoleID: local.ID, Actions: add})
		}
		if len(del) > 0 {
			plan.DelRoleActions = append(plan.DelRoleActions,
				RoleActionsChange{RoleID: local.ID, Actions: del})
		}

		if err := s.diffRoleDisplay(kt, local.ID, displays[local.ID], plan); err != nil {
			return err
		}
	}

	for _, r := range online {
		if _, ok := localMap[r.ID]; !ok {
			plan.DeleteRoles = append(plan.DeleteRoles, r.ID)
		}
	}

	return nil
}

// diffRoleDisplay 比较角色的展示资源层级。
// 线上会把未显式配置的关联资源类型按默认值一并返回，因此只比对本地声明的那些键。
func (s *Syncer) diffRoleDisplay(kt *kit.Kit, roleID string,
	want []client.DisplayResourceType, plan *Plan) error {

	if len(want) == 0 {
		return nil
	}

	cur, err := s.gw.ListRoleDisplayResourceTypes(kt, roleID)
	if err != nil {
		return errors.Wrapf(err, "list display resource types of role %s", roleID)
	}

	curMap := make(map[string]string, len(cur))
	for _, t := range cur {
		curMap[t.RelatedResourceTypeID] = t.DisplayResourceTypeID
	}

	for _, t := range want {
		if curMap[t.RelatedResourceTypeID] != t.DisplayResourceTypeID {
			plan.SetRoleDisplays = append(plan.SetRoleDisplays, RoleDisplayChange{
				RoleID: roleID, Types: want,
				Detail: fmt.Sprintf("%s（线上 %s）", formatDisplayTypes(want),
					formatDisplayTypes(cur)),
			})

			return nil
		}
	}

	return nil
}

// Apply 按依赖顺序施加变更：新增与更新自上而下（资源类型→操作→角色），
// 删除自下而上（角色→操作→资源类型），因为被引用的对象不能先删。
func (s *Syncer) Apply(kt *kit.Kit, plan *Plan, opt ApplyOption) error {
	if len(plan.Conflicts) > 0 {
		return errors.Errorf("模型存在 %d 处不可通过接口消除的差异，需人工处理后重试",
			len(plan.Conflicts))
	}

	// 系统必须最先处理：资源类型、操作、角色都挂在系统下，系统不存在时它们无处可建。
	if err := s.applySystem(kt, plan, &opt); err != nil {
		return err
	}

	if err := s.applyResourceTypes(kt, plan, &opt); err != nil {
		return err
	}

	if err := s.applyActions(kt, plan, &opt); err != nil {
		return err
	}

	if err := s.applyRoles(kt, plan, &opt); err != nil {
		return err
	}

	return s.applyDeletions(kt, plan, &opt)
}

func (s *Syncer) applySystem(kt *kit.Kit, plan *Plan, opt *ApplyOption) error {
	if plan.CreateSystem != nil {
		if _, err := s.gw.CreateSystem(kt, plan.CreateSystem); err != nil {
			return errors.Wrap(err, "create system")
		}
		opt.logf("已注册系统 %s，回调地址 %s", plan.CreateSystem.ID, plan.CreateSystem.CallbackURL)

		return nil
	}

	if plan.UpdateSystem != nil {
		if err := s.gw.UpdateSystem(kt, plan.UpdateSystem.Req); err != nil {
			return errors.Wrap(err, "update system")
		}
		opt.logf("已更新系统：%s", plan.UpdateSystem.Detail)
	}

	return nil
}

func (s *Syncer) applyResourceTypes(kt *kit.Kit, plan *Plan, opt *ApplyOption) error {
	if len(plan.CreateResourceTypes) > 0 {
		if _, err := s.gw.BatchCreateResourceTypes(kt, plan.CreateResourceTypes); err != nil {
			return errors.Wrap(err, "create resource types")
		}
		opt.logf("已新增 %d 个资源类型", len(plan.CreateResourceTypes))
	}

	for _, r := range plan.RenameResourceTypes {
		if err := s.gw.UpdateResourceType(kt, r.ID,
			&client.UpdateResourceTypeReq{Name: r.To}); err != nil {
			return errors.Wrapf(err, "rename resource type %s", r.ID)
		}
		opt.logf("已重命名资源类型 %s 为 %q", r.ID, r.To)
	}

	return nil
}

func (s *Syncer) applyActions(kt *kit.Kit, plan *Plan, opt *ApplyOption) error {
	if len(plan.CreateActions) > 0 {
		if _, err := s.gw.BatchCreateActions(kt, plan.CreateActions); err != nil {
			return errors.Wrap(err, "create actions")
		}
		opt.logf("已新增 %d 个操作", len(plan.CreateActions))
	}

	for _, r := range plan.RenameActions {
		if err := s.gw.UpdateAction(kt, r.ID, &client.UpdateActionReq{Name: r.To}); err != nil {
			return errors.Wrapf(err, "rename action %s", r.ID)
		}
		opt.logf("已重命名操作 %s 为 %q", r.ID, r.To)
	}

	return nil
}

func (s *Syncer) applyRoles(kt *kit.Kit, plan *Plan, opt *ApplyOption) error {
	if len(plan.CreateRoles) > 0 {
		if _, err := s.gw.BatchCreateRoles(kt, plan.CreateRoles); err != nil {
			return errors.Wrap(err, "create roles")
		}
		opt.logf("已新增 %d 个角色", len(plan.CreateRoles))
	}

	for _, u := range plan.UpdateRoleMetas {
		if err := s.gw.UpdateRole(kt, u.ID, &client.UpdateRoleReq{
			Name: u.Name, Description: u.Description,
		}); err != nil {
			return errors.Wrapf(err, "update role %s", u.ID)
		}
		opt.logf("已更新角色 %s：%s", u.ID, u.Detail)
	}

	// 先加后删，避免中途出现角色没有任何操作的空窗
	for _, c := range plan.AddRoleActions {
		if err := s.gw.BatchCreateRoleActions(kt, c.RoleID, c.Actions); err != nil {
			return errors.Wrapf(err, "add actions to role %s", c.RoleID)
		}
		opt.logf("已为角色 %s 新增操作 %s", c.RoleID, formatRoleActions(c.Actions))
	}

	for _, c := range plan.DelRoleActions {
		ids := make([]string, 0, len(c.Actions))
		for _, a := range c.Actions {
			ids = append(ids, a.ID)
		}

		if err := s.gw.BatchDeleteRoleActions(kt, c.RoleID, ids); err != nil {
			return errors.Wrapf(err, "remove actions from role %s", c.RoleID)
		}
		opt.logf("已移除角色 %s 的操作 %v", c.RoleID, ids)
	}

	for _, c := range plan.SetRoleDisplays {
		if err := s.gw.UpdateRoleDisplayResourceTypes(kt, c.RoleID, c.Types); err != nil {
			return errors.Wrapf(err, "set display resource types of role %s", c.RoleID)
		}
		opt.logf("已配置角色 %s 的展示层级 %s", c.RoleID, formatDisplayTypes(c.Types))
	}

	return nil
}

func (s *Syncer) applyDeletions(kt *kit.Kit, plan *Plan, opt *ApplyOption) error {
	if !plan.HasDeletions() {
		return nil
	}

	if !opt.AllowDelete {
		opt.logf("线上有 %d 个角色、%d 个操作、%d 个资源类型不在本地定义中，"+
			"未删除（删除会失效相关授权，需用 prune 子命令显式执行）",
			len(plan.DeleteRoles), len(plan.DeleteActions), len(plan.DeleteResourceTypes))

		return nil
	}

	for _, id := range plan.DeleteRoles {
		if err := s.gw.DeleteRole(kt, id); err != nil {
			return errors.Wrapf(err, "delete role %s", id)
		}
		opt.logf("已删除角色 %s", id)
	}

	for _, id := range plan.DeleteActions {
		if err := s.gw.DeleteAction(kt, id); err != nil {
			return errors.Wrapf(err, "delete action %s", id)
		}
		opt.logf("已删除操作 %s", id)
	}

	for _, id := range plan.DeleteResourceTypes {
		if err := s.gw.DeleteResourceType(kt, id); err != nil {
			return errors.Wrapf(err, "delete resource type %s", id)
		}
		opt.logf("已删除资源类型 %s", id)
	}

	return nil
}

func (s *Syncer) listAllResourceTypes(kt *kit.Kit) ([]client.ResourceType, error) {
	var all []client.ResourceType

	for page := 1; ; page++ {
		items, total, err := s.gw.ListResourceTypes(kt, page, client.MaxPageSize)
		if err != nil {
			return nil, err
		}

		all = append(all, items...)
		if len(items) == 0 || len(all) >= total {
			return all, nil
		}
	}
}

func (s *Syncer) listAllActions(kt *kit.Kit) ([]client.Action, error) {
	var all []client.Action

	for page := 1; ; page++ {
		items, total, err := s.gw.ListActions(kt, page, client.MaxPageSize)
		if err != nil {
			return nil, err
		}

		all = append(all, items...)
		if len(items) == 0 || len(all) >= total {
			return all, nil
		}
	}
}

func (s *Syncer) listAllRoles(kt *kit.Kit) ([]client.Role, error) {
	var all []client.Role

	for page := 1; ; page++ {
		items, total, err := s.gw.ListRoles(kt, page, client.MaxPageSize)
		if err != nil {
			return nil, err
		}

		all = append(all, items...)
		if len(items) == 0 || len(all) >= total {
			return all, nil
		}
	}
}

func roleMetaDiff(online, local client.Role) string {
	var parts []string
	if online.Name != local.Name {
		parts = append(parts, fmt.Sprintf("名称 %q -> %q", online.Name, local.Name))
	}
	if online.Description != local.Description {
		parts = append(parts, fmt.Sprintf("描述 %q -> %q", online.Description, local.Description))
	}

	if len(parts) == 0 {
		return ""
	}

	return parts[0] + joinRest(parts[1:])
}

func joinRest(rest []string) string {
	out := ""
	for _, r := range rest {
		out += "；" + r
	}

	return out
}

// diffRoleActions 求角色操作列表的双向差集。
// 键取 ID 与授权维度的组合：同一个操作换了授权维度也必须重建，
// 否则会留下一条维度错误的授权关系。
func diffRoleActions(online, local []client.RoleAction) (add, del []client.RoleAction) {
	onlineMap := make(map[client.RoleAction]struct{}, len(online))
	for _, a := range online {
		onlineMap[a] = struct{}{}
	}

	localMap := make(map[client.RoleAction]struct{}, len(local))
	for _, a := range local {
		localMap[a] = struct{}{}
	}

	for _, a := range local {
		if _, ok := onlineMap[a]; !ok {
			add = append(add, a)
		}
	}

	for _, a := range online {
		if _, ok := localMap[a]; !ok {
			del = append(del, a)
		}
	}

	sortRoleActions(add)
	sortRoleActions(del)

	return add, del
}

func sortRoleActions(actions []client.RoleAction) {
	sort.Slice(actions, func(i, j int) bool {
		if actions[i].ID != actions[j].ID {
			return actions[i].ID < actions[j].ID
		}

		return actions[i].ResourceTypeID < actions[j].ResourceTypeID
	})
}

func sameStrings(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}

	return reflect.DeepEqual(a, b)
}
