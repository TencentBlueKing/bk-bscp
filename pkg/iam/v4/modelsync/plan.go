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

// Package modelsync 把 model 包里的 IAM V4 权限模型定义同步到权限中心，
// 做法是先与线上状态求差异得到 Plan，再按依赖顺序施加变更，因此可重复执行。
package modelsync

import (
	"fmt"
	"sort"
	"strings"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/client"
)

// Rename 是一次名称变更。IAM V4 的资源类型与操作只有名称可改
type Rename struct {
	ID   string
	From string
	To   string
}

// RoleMetaUpdate 是角色名称或描述的变更
type RoleMetaUpdate struct {
	ID          string
	Name        string
	Description string
	// Detail 描述具体改了哪些字段，仅用于输出
	Detail string
}

// RoleActionsChange 是角色内操作列表的增量变更
type RoleActionsChange struct {
	RoleID  string
	Actions []client.RoleAction
}

// RoleDisplayChange 是角色展示资源层级的变更
type RoleDisplayChange struct {
	RoleID string
	Types  []client.DisplayResourceType
	Detail string
}

// ResourceTypeAncestorsUpdate 是资源类型祖先链的变更。
// 拓扑调整只能原地更新：删除资源类型会连带删掉引用它的操作和既有授权，
// 因此优先走更新，更新后由同步流程回读校验是否真的生效。
type ResourceTypeAncestorsUpdate struct {
	ID string
	// Name 本地定义的名称，更新时一并提交，避免部分更新接口要求全量字段
	Name string
	From []string
	To   []string
}

// ActionResourceTypeUpdate 是操作关联资源类型的变更。
// 与祖先链同理走原地更新：例如服务创建从业务维度改为环境维度时，
// 不能删掉 app_create 重建，否则业务维度的授权会一起失效。
type ActionResourceTypeUpdate struct {
	ID string
	// Name 本地定义的名称，更新时一并提交
	Name string
	From string
	To   string
}

// Conflict 是同步无法自动处理的差异。
//
// 目前没有字段会落到这里：名称走改名、祖先链与操作关联资源类型走原地更新，
// 其余差异（新增/删除/角色内操作增减）都有对应的接口。保留该结构用于报告
// 需要人工介入的差异并终止命令，避免静默跳过。
type Conflict struct {
	Kind   string
	ID     string
	Field  string
	Online string
	Local  string
}

func (c Conflict) String() string {
	return fmt.Sprintf("%s %s 的 %s 不一致：线上 %q，本地 %q（该字段不支持更新）",
		c.Kind, c.ID, c.Field, c.Online, c.Local)
}

// SystemUpdate IAM V4 注册的系统变更信息
type SystemUpdate struct {
	Req *client.UpdateSystemReq
	// Detail 描述具体改了哪些字段
	Detail string
}

// Plan 一次同步需要施加的全部变更
type Plan struct {
	// CreateSystem 与 UpdateSystem 互斥，系统未注册时为前者
	CreateSystem *client.CreateSystemReq
	UpdateSystem *SystemUpdate

	CreateResourceTypes         []client.ResourceType
	RenameResourceTypes         []Rename
	UpdateResourceTypeAncestors []ResourceTypeAncestorsUpdate
	DeleteResourceTypes         []string

	CreateActions             []client.Action
	RenameActions             []Rename
	UpdateActionResourceTypes []ActionResourceTypeUpdate
	DeleteActions             []string

	CreateRoles     []client.Role
	UpdateRoleMetas []RoleMetaUpdate
	AddRoleActions  []RoleActionsChange
	DelRoleActions  []RoleActionsChange
	SetRoleDisplays []RoleDisplayChange
	DeleteRoles     []string

	Conflicts []Conflict
}

// HasChanges 报告 Plan 是否包含需要施加的变更
// 幂等的直接体现：模型未变时第二次执行的 Plan 必然返回 false
func (p *Plan) HasChanges() bool {
	return p.CreateSystem != nil || p.UpdateSystem != nil ||
		len(p.CreateResourceTypes) > 0 || len(p.RenameResourceTypes) > 0 ||
		len(p.UpdateResourceTypeAncestors) > 0 || len(p.DeleteResourceTypes) > 0 ||
		len(p.CreateActions) > 0 || len(p.RenameActions) > 0 ||
		len(p.UpdateActionResourceTypes) > 0 || len(p.DeleteActions) > 0 ||
		len(p.CreateRoles) > 0 || len(p.UpdateRoleMetas) > 0 ||
		len(p.AddRoleActions) > 0 || len(p.DelRoleActions) > 0 ||
		len(p.SetRoleDisplays) > 0 || len(p.DeleteRoles) > 0
}

// HasDeletions 报告 Plan 是否要删除线上已有的资源类型、操作或角色。
func (p *Plan) HasDeletions() bool {
	return len(p.DeleteResourceTypes) > 0 || len(p.DeleteActions) > 0 ||
		len(p.DeleteRoles) > 0
}

// String 把 Plan 格式化为逐条列出的变更清单，用于命令行输出。
func (p *Plan) String() string {
	var b strings.Builder

	for _, c := range p.Conflicts {
		fmt.Fprintf(&b, "  [冲突] %s\n", c)
	}

	if p.CreateSystem != nil {
		fmt.Fprintf(&b, "  [新增] 系统 %s（%s）回调地址 %s，调用方 %v\n",
			p.CreateSystem.ID, p.CreateSystem.Name, p.CreateSystem.CallbackURL,
			p.CreateSystem.Clients)
	}
	if p.UpdateSystem != nil {
		fmt.Fprintf(&b, "  [更新] 系统：%s\n", p.UpdateSystem.Detail)
	}

	for _, rt := range p.CreateResourceTypes {
		fmt.Fprintf(&b, "  [新增] 资源类型 %s（%s）祖先 %v\n", rt.ID, rt.Name, rt.Ancestors)
	}
	for _, r := range p.RenameResourceTypes {
		fmt.Fprintf(&b, "  [改名] 资源类型 %s：%q -> %q\n", r.ID, r.From, r.To)
	}
	for _, u := range p.UpdateResourceTypeAncestors {
		fmt.Fprintf(&b, "  [更新] 资源类型 %s 的祖先链：%v -> %v\n", u.ID, u.From, u.To)
	}

	for _, a := range p.CreateActions {
		fmt.Fprintf(&b, "  [新增] 操作 %s（%s）关联 %s\n", a.ID, a.Name, orNone(a.ResourceTypeID))
	}
	for _, r := range p.RenameActions {
		fmt.Fprintf(&b, "  [改名] 操作 %s：%q -> %q\n", r.ID, r.From, r.To)
	}
	for _, u := range p.UpdateActionResourceTypes {
		fmt.Fprintf(&b, "  [更新] 操作 %s 的关联资源类型：%s -> %s\n", u.ID, orNone(u.From), orNone(u.To))
	}

	for _, r := range p.CreateRoles {
		fmt.Fprintf(&b, "  [新增] 角色 %s（%s）含 %d 个操作\n", r.ID, r.Name, len(r.Actions))
	}
	for _, u := range p.UpdateRoleMetas {
		fmt.Fprintf(&b, "  [更新] 角色 %s：%s\n", u.ID, u.Detail)
	}
	for _, c := range p.AddRoleActions {
		fmt.Fprintf(&b, "  [新增] 角色 %s 的操作 %s\n", c.RoleID, formatRoleActions(c.Actions))
	}
	for _, c := range p.DelRoleActions {
		fmt.Fprintf(&b, "  [移除] 角色 %s 的操作 %s\n", c.RoleID, formatRoleActions(c.Actions))
	}
	for _, c := range p.SetRoleDisplays {
		fmt.Fprintf(&b, "  [更新] 角色 %s 的展示层级：%s\n", c.RoleID, c.Detail)
	}

	// 删除放在最后，与实际施加顺序一致
	for _, id := range p.DeleteRoles {
		fmt.Fprintf(&b, "  [删除] 角色 %s\n", id)
	}
	for _, id := range p.DeleteActions {
		fmt.Fprintf(&b, "  [删除] 操作 %s\n", id)
	}
	for _, id := range p.DeleteResourceTypes {
		fmt.Fprintf(&b, "  [删除] 资源类型 %s\n", id)
	}

	if b.Len() == 0 {
		return "  线上模型与本地定义一致，无需变更\n"
	}

	return b.String()
}

func orNone(s string) string {
	if s == "" {
		return "（无关资源类型）"
	}

	return s
}

// formatRoleActions 格式化角色操作列表
// 返回格式为：操作ID@资源类型ID，多个操作用逗号分隔
func formatRoleActions(actions []client.RoleAction) string {
	parts := make([]string, 0, len(actions))
	for _, a := range actions {
		parts = append(parts, fmt.Sprintf("%s@%s", a.ID, orNone(a.ResourceTypeID)))
	}
	sort.Strings(parts)

	return strings.Join(parts, ", ")
}

// formatDisplayTypes 格式化角色展示资源层级列表
// 返回格式为：资源类型ID->展示资源类型ID，多个资源类型用逗号分隔
func formatDisplayTypes(types []client.DisplayResourceType) string {
	parts := make([]string, 0, len(types))
	for _, t := range types {
		parts = append(parts, fmt.Sprintf("%s->%s", t.RelatedResourceTypeID, t.DisplayResourceTypeID))
	}
	sort.Strings(parts)

	return strings.Join(parts, ", ")
}
