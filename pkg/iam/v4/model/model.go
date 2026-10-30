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

import "github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/client"

// ResourceTypes 返回 BSCP 在 IAM V4 中注册的资源类型。
// biz 必须由 BSCP 自注册：IAM V4 的 ancestors 没有 system 维度，祖先链只能引用本系统内注册的
// 资源类型，实例 ID 仍沿用 CMDB 的 bk_biz_id，实例数据由 BSCP 通过资源回调代理提供。
func ResourceTypes() []client.ResourceType {
	// 顺序按祖先链从根到叶：新增时父资源必须先创建成功。
	return []client.ResourceType{
		{ID: ResourceTypeBiz, Name: "业务"},
		{ID: ResourceTypeProject, Name: "项目", Ancestors: []string{ResourceTypeBiz}},
		// 环境按项目隔离：拥有项目权限即拥有该项目下全部环境的权限，
		// 也可以只把某个环境单独授权给用户。
		// ancestors 是从根到父级的全链而非直接父级：鉴权与资源回调给出的环境路径都是
		// /biz,{bizID}/project,{projectID}/，少写 biz 会让注册的拓扑与路径对不上。
		{ID: ResourceTypeEnvironment, Name: "环境", Ancestors: []string{ResourceTypeBiz, ResourceTypeProject}},
		// 服务挂在环境下：授权路径为 业务 → 项目 → 环境 → 服务，
		// 项目或环境维度的授权据此覆盖其下全部服务。
		{ID: ResourceTypeApp, Name: "服务",
			Ancestors: []string{ResourceTypeBiz, ResourceTypeProject, ResourceTypeEnvironment}},
	}
}

// Actions 返回 BSCP 在 IAM V4 中注册的操作
func Actions() []client.Action {
	return []client.Action{
		{ID: ActionFindBusinessResource, Name: "业务访问", ResourceTypeID: ResourceTypeBiz},
		// 服务创建按环境授权：创建时尚无服务实例，授权维度只能落在服务将要归属的环境上，
		// 授权页面因此需要先选项目、再选环境；业务/项目维度的授权经拓扑路径继承覆盖。
		{ID: ActionAppCreate, Name: "服务创建", ResourceTypeID: ResourceTypeEnvironment},
		{ID: ActionAppView, Name: "服务查看", ResourceTypeID: ResourceTypeApp},
		{ID: ActionAppEdit, Name: "服务编辑", ResourceTypeID: ResourceTypeApp},
		{ID: ActionAppDelete, Name: "服务删除", ResourceTypeID: ResourceTypeApp},
		{ID: ActionReleaseGenerate, Name: "生成版本", ResourceTypeID: ResourceTypeApp},
		{ID: ActionReleasePublish, Name: "上线版本", ResourceTypeID: ResourceTypeApp},
		{ID: ActionAppCredentialView, Name: "服务密钥查看", ResourceTypeID: ResourceTypeBiz},
		{ID: ActionAppCredentialManage, Name: "服务密钥管理", ResourceTypeID: ResourceTypeBiz},
		{ID: ActionAuditView, Name: "操作记录查看", ResourceTypeID: ResourceTypeBiz},
		{ID: ActionProcConfigMgmtView, Name: "进程配置管理查看", ResourceTypeID: ResourceTypeBiz},
		{ID: ActionProcessOperate, Name: "进程操作", ResourceTypeID: ResourceTypeBiz},
		{ID: ActionConfigTemplateCreate, Name: "配置模板创建", ResourceTypeID: ResourceTypeBiz},
		{ID: ActionConfigTemplateEdit, Name: "配置模板编辑", ResourceTypeID: ResourceTypeBiz},
		{ID: ActionConfigTemplateDelete, Name: "配置模板删除", ResourceTypeID: ResourceTypeBiz},
		{ID: ActionConfigGenerate, Name: "配置生成", ResourceTypeID: ResourceTypeBiz},
		{ID: ActionConfigRelease, Name: "配置下发", ResourceTypeID: ResourceTypeBiz},
		{ID: ActionProjectCreate, Name: "项目创建", ResourceTypeID: ResourceTypeBiz},
		{ID: ActionProjectView, Name: "项目查看", ResourceTypeID: ResourceTypeProject},
		{ID: ActionProjectEdit, Name: "项目编辑", ResourceTypeID: ResourceTypeProject},
		{ID: ActionProjectDelete, Name: "项目删除", ResourceTypeID: ResourceTypeProject},
		{ID: ActionEnvironmentCreate, Name: "环境创建", ResourceTypeID: ResourceTypeProject},
		{ID: ActionEnvironmentView, Name: "环境查看", ResourceTypeID: ResourceTypeEnvironment},
		{ID: ActionEnvironmentEdit, Name: "环境编辑", ResourceTypeID: ResourceTypeEnvironment},
		{ID: ActionEnvironmentDelete, Name: "环境删除", ResourceTypeID: ResourceTypeEnvironment},
	}
}

// Roles 返回 BSCP 在 IAM V4 中注册的角色
// 业务维度角色的全部操作按 biz 授权；服务维度角色的 find_business_resource 按 biz 授权
// 以放行业务访问前置校验，其余操作按 app 授权。
// 项目、环境维度角色只在 biz 上保留业务访问，其余操作按项目或环境实例授权，
// 这样"只授某个项目/环境"才成立。
func Roles() []client.Role {
	return []client.Role{
		{
			ID:          RoleBizViewer,
			Name:        "业务只读",
			Description: "查看业务下的项目、环境、服务、密钥、审计与进程配置",
			Actions: bizScoped(ActionFindBusinessResource, ActionAppView,
				ActionAppCredentialView, ActionAuditView, ActionProcConfigMgmtView, ActionProjectView,
				ActionEnvironmentView),
		},
		{
			ID:          RoleBizOperator,
			Name:        "业务运维",
			Description: "业务下的全部操作权限（含项目、环境、服务）",
			// 全部操作都以 biz 为授权维度：biz 是项目与环境的祖先，
			// 一次授权即覆盖该业务下的项目、环境与服务。
			Actions: bizScoped(allActionIDs()...),
		},
		{
			ID:   RoleAppCreator,
			Name: "服务创建者",
			// 服务创建按环境授权：授权时选到项目下的具体环境，项目/业务维度的授权经路径继承覆盖。
			Description: "在指定环境下创建服务；创建后自动获得该服务的运维权限，" +
				"访问他人创建的服务需另行申请",
			Actions: append(bizScoped(ActionFindBusinessResource),
				environmentScoped(ActionAppCreate)...),
		},
		{
			ID:          RoleCredentialManager,
			Name:        "服务密钥管理员",
			Description: "管理业务下的服务密钥",
			Actions: bizScoped(ActionFindBusinessResource, ActionAppCredentialView,
				ActionAppCredentialManage),
		},
		{
			ID:          RoleProcConfigManager,
			Name:        "进程配置管理员",
			Description: "管理业务下的进程配置与配置模板",
			Actions: bizScoped(ActionFindBusinessResource, ActionProcConfigMgmtView,
				ActionProcessOperate, ActionConfigTemplateCreate, ActionConfigTemplateEdit,
				ActionConfigTemplateDelete, ActionConfigGenerate, ActionConfigRelease),
		},
		{
			ID:          RoleAuditViewer,
			Name:        "操作记录查看者",
			Description: "查看业务下的操作记录",
			Actions:     bizScoped(ActionFindBusinessResource, ActionAuditView),
		},
		{
			ID:          RoleAppOperator,
			Name:        "服务运维",
			Description: "管理指定服务的配置与版本",
			Actions: append(bizScoped(ActionFindBusinessResource),
				appScoped(ActionAppView, ActionAppEdit, ActionAppDelete,
					ActionReleaseGenerate, ActionReleasePublish)...),
		},
		{
			ID:          RoleAppPublisher,
			Name:        "服务发布者",
			Description: "生成并上线指定服务的版本，不含配置编辑",
			Actions: append(bizScoped(ActionFindBusinessResource),
				appScoped(ActionAppView, ActionReleaseGenerate, ActionReleasePublish)...),
		},
		{
			ID:          RoleAppViewer,
			Name:        "服务查看者",
			Description: "查看指定服务",
			Actions:     append(bizScoped(ActionFindBusinessResource), appScoped(ActionAppView)...),
		},
		{
			ID:   RoleProjectOperator,
			Name: "项目运维",
			// 服务写操作（app_edit/app_delete/release_*）只能在服务维度授予：
			// 一旦放到项目维度，就是"该项目下全部服务都可写"，无法做到只授权特定服务。
			// 列表与详情仍按 app_view 继承放行，保证能看见服务、点了引导申请。
			Description: "管理指定项目及其全部环境；服务可查看，写操作需在服务维度单独授权",
			Actions: append(bizScoped(ActionFindBusinessResource),
				projectScoped(ActionProjectView, ActionProjectEdit, ActionProjectDelete,
					ActionEnvironmentView, ActionEnvironmentEdit, ActionEnvironmentDelete,
					ActionAppView)...),
		},
		{
			ID:          RoleProjectViewer,
			Name:        "项目查看者",
			Description: "查看指定项目及其全部环境、服务",
			Actions: append(bizScoped(ActionFindBusinessResource),
				projectScoped(ActionProjectView, ActionEnvironmentView, ActionAppView)...),
		},
		{
			ID:   RoleEnvironmentOperator,
			Name: "环境运维",
			// 同项目运维：服务写操作只在服务维度授予，环境维度只继承 app_view，
			// 避免"授了环境运维就能改该环境下所有服务"。
			Description: "管理指定环境；服务可查看，写操作需在服务维度单独授权",
			Actions: append(bizScoped(ActionFindBusinessResource),
				environmentScoped(ActionEnvironmentView, ActionEnvironmentEdit,
					ActionEnvironmentDelete, ActionAppView)...),
		},
		{
			ID:          RoleEnvironmentViewer,
			Name:        "环境查看者",
			Description: "查看指定环境及其全部服务",
			Actions: append(bizScoped(ActionFindBusinessResource),
				environmentScoped(ActionEnvironmentView, ActionAppView)...),
		},
	}
}

// DisplayResourceTypes 返回每个角色的默认展示资源层级配置，键为角色 ID
func DisplayResourceTypes() map[string][]client.DisplayResourceType {
	// 只有 biz 一个授权维度的角色。
	bizScopeOnly := []client.DisplayResourceType{
		{RelatedResourceTypeID: ResourceTypeBiz, DisplayResourceTypeID: ResourceTypeBiz},
	}
	// 同时有 biz 与 app 两个授权维度的角色（服务运维/发布者/查看者）：
	// 业务访问按 biz 展示；服务挂在环境下，展示粒度下移到环境，
	// 授权页面才会展开「业务 → 项目 → 环境」再落到服务。
	appScopes := []client.DisplayResourceType{
		{RelatedResourceTypeID: ResourceTypeBiz, DisplayResourceTypeID: ResourceTypeBiz},
		{RelatedResourceTypeID: ResourceTypeApp, DisplayResourceTypeID: ResourceTypeEnvironment},
	}
	// 项目维度角色按项目展示：授权时选到项目，即覆盖该项目下全部环境与服务。
	projectScopes := []client.DisplayResourceType{
		{RelatedResourceTypeID: ResourceTypeBiz, DisplayResourceTypeID: ResourceTypeBiz},
		{RelatedResourceTypeID: ResourceTypeProject, DisplayResourceTypeID: ResourceTypeProject},
		{RelatedResourceTypeID: ResourceTypeApp, DisplayResourceTypeID: ResourceTypeProject},
	}
	// 环境维度角色按环境展示：授权粒度止于单个环境，环境下的服务随环境一并被覆盖。
	environmentScopes := []client.DisplayResourceType{
		{RelatedResourceTypeID: ResourceTypeBiz, DisplayResourceTypeID: ResourceTypeBiz},
		{RelatedResourceTypeID: ResourceTypeEnvironment, DisplayResourceTypeID: ResourceTypeEnvironment},
		{RelatedResourceTypeID: ResourceTypeApp, DisplayResourceTypeID: ResourceTypeEnvironment},
	}
	// 服务创建者：业务访问按业务展示，app_create 按环境展示，
	// 授权页面因此展开「业务 → 项目 → 环境」让用户选到具体环境。
	appCreatorScopes := []client.DisplayResourceType{
		{RelatedResourceTypeID: ResourceTypeBiz, DisplayResourceTypeID: ResourceTypeBiz},
		{RelatedResourceTypeID: ResourceTypeEnvironment, DisplayResourceTypeID: ResourceTypeEnvironment},
	}
	return map[string][]client.DisplayResourceType{
		RoleBizViewer:           bizScopeOnly,
		RoleBizOperator:         bizScopeOnly,
		RoleAppCreator:          appCreatorScopes,
		RoleCredentialManager:   bizScopeOnly,
		RoleProcConfigManager:   bizScopeOnly,
		RoleAuditViewer:         bizScopeOnly,
		RoleAppOperator:         appScopes,
		RoleAppPublisher:        appScopes,
		RoleAppViewer:           appScopes,
		RoleProjectOperator:     projectScopes,
		RoleProjectViewer:       projectScopes,
		RoleEnvironmentOperator: environmentScopes,
		RoleEnvironmentViewer:   environmentScopes,
	}
}

// bizScoped 把一组操作全部声明为按业务维度授权。
func bizScoped(actionIDs ...string) []client.RoleAction {
	return scoped(ResourceTypeBiz, actionIDs)
}

// projectScoped 把一组操作声明为按项目维度授权。
func projectScoped(actionIDs ...string) []client.RoleAction {
	return scoped(ResourceTypeProject, actionIDs)
}

// environmentScoped 把一组操作声明为按环境维度授权。
func environmentScoped(actionIDs ...string) []client.RoleAction {
	return scoped(ResourceTypeEnvironment, actionIDs)
}

// appScoped 把一组操作全部声明为按服务维度授权。
func appScoped(actionIDs ...string) []client.RoleAction {
	return scoped(ResourceTypeApp, actionIDs)
}

func scoped(resourceTypeID string, actionIDs []string) []client.RoleAction {
	actions := make([]client.RoleAction, 0, len(actionIDs))
	for _, id := range actionIDs {
		actions = append(actions, client.RoleAction{ID: id, ResourceTypeID: resourceTypeID})
	}

	return actions
}

// allActionIDs 返回全部操作 ID。
// 「业务运维」按业务维度声明全部操作：项目、环境的授权维度是 biz 的后代，
// 声明在 biz 上合法且语义就是"该业务下的所有资源全部可操作"。
func allActionIDs() []string {
	actions := Actions()
	ids := make([]string, 0, len(actions))
	for _, a := range actions {
		ids = append(ids, a.ID)
	}

	return ids
}
