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

package sys

import "github.com/TencentBlueKing/bk-bscp/pkg/iam/client"

var (
	// 业务资源, 自动拉取 cmdb 业务列表
	businessResource = []client.RelateResourceType{
		{
			SystemID: SystemIDCMDB,
			ID:       Business,
			InstanceSelections: []client.RelatedInstanceSelection{
				{
					SystemID: SystemIDCMDB,
					ID:       BusinessSelection,
				},
			},
		},
	}

	// 项目资源，实例挂在业务下，用于项目自身的增删改查与环境创建
	projectResource = []client.RelateResourceType{
		{
			SystemID: SystemIDBSCP,
			ID:       Project,
			InstanceSelections: []client.RelatedInstanceSelection{
				{
					SystemID: SystemIDBSCP,
					ID:       ProjectSelection,
				},
			},
		},
	}

	// 环境资源，实例挂在项目下
	environmentResource = []client.RelateResourceType{
		{
			SystemID: SystemIDBSCP,
			ID:       Environment,
			InstanceSelections: []client.RelatedInstanceSelection{
				{
					SystemID: SystemIDBSCP,
					ID:       EnvironmentSelection,
				},
			},
		},
	}
)

// GenerateStaticActions return need to register action.
func GenerateStaticActions() []client.ResourceAction {
	resourceActionList := make([]client.ResourceAction, 0)

	resourceActionList = append(resourceActionList, genBusinessActions()...)
	resourceActionList = append(resourceActionList, genApplicationActions()...)
	resourceActionList = append(resourceActionList, genCredentialActions()...)
	resourceActionList = append(resourceActionList, genAuditActions()...)
	resourceActionList = append(resourceActionList, genProcConfigMgmtActions()...)
	resourceActionList = append(resourceActionList, genProjectActions()...)
	resourceActionList = append(resourceActionList, genEnvironmentActions()...)

	return resourceActionList
}

// genProjectActions 生成项目相关操作：项目按项目实例隔离，创建按业务授权
// （创建时项目实例还不存在），查看/编辑/删除都按项目实例授权。
func genProjectActions() []client.ResourceAction {
	return []client.ResourceAction{
		{
			ID:                   ProjectCreate,
			Name:                 ActionIDNameMap[ProjectCreate],
			NameEn:               "Create Project",
			Type:                 Create,
			RelatedResourceTypes: businessResource,
			RelatedActions:       []client.ActionID{BusinessViewResource},
			Version:              1,
		},
		{
			ID:                   ProjectView,
			Name:                 ActionIDNameMap[ProjectView],
			NameEn:               "View Project",
			Type:                 View,
			RelatedResourceTypes: projectResource,
			RelatedActions:       []client.ActionID{BusinessViewResource},
			Version:              1,
		},
		{
			ID:                   ProjectEdit,
			Name:                 ActionIDNameMap[ProjectEdit],
			NameEn:               "Edit Project",
			Type:                 Edit,
			RelatedResourceTypes: projectResource,
			RelatedActions:       []client.ActionID{BusinessViewResource, ProjectView},
			Version:              1,
		},
		{
			ID:                   ProjectDelete,
			Name:                 ActionIDNameMap[ProjectDelete],
			NameEn:               "Delete Project",
			Type:                 Delete,
			RelatedResourceTypes: projectResource,
			RelatedActions:       []client.ActionID{BusinessViewResource, ProjectView},
			Version:              1,
		},
	}
}

// genEnvironmentActions 生成环境相关操作：环境按环境实例隔离，创建按所属项目授权
// （创建时环境实例还不存在）。
func genEnvironmentActions() []client.ResourceAction {
	return []client.ResourceAction{
		{
			ID:                   EnvironmentCreate,
			Name:                 ActionIDNameMap[EnvironmentCreate],
			NameEn:               "Create Environment",
			Type:                 Create,
			RelatedResourceTypes: projectResource,
			RelatedActions:       []client.ActionID{BusinessViewResource, ProjectView},
			Version:              1,
		},
		{
			ID:                   EnvironmentView,
			Name:                 ActionIDNameMap[EnvironmentView],
			NameEn:               "View Environment",
			Type:                 View,
			RelatedResourceTypes: environmentResource,
			RelatedActions:       []client.ActionID{BusinessViewResource},
			Version:              1,
		},
		{
			ID:                   EnvironmentEdit,
			Name:                 ActionIDNameMap[EnvironmentEdit],
			NameEn:               "Edit Environment",
			Type:                 Edit,
			RelatedResourceTypes: environmentResource,
			RelatedActions:       []client.ActionID{BusinessViewResource, EnvironmentView},
			Version:              1,
		},
		{
			ID:                   EnvironmentDelete,
			Name:                 ActionIDNameMap[EnvironmentDelete],
			NameEn:               "Delete Environment",
			Type:                 Delete,
			RelatedResourceTypes: environmentResource,
			RelatedActions:       []client.ActionID{BusinessViewResource, EnvironmentView},
			Version:              1,
		},
	}
}

func genApplicationActions() []client.ResourceAction {
	actions := make([]client.ResourceAction, 0)

	// 服务创建按环境授权：创建时尚无服务实例，授权维度落在服务将要归属的环境上，
	// 授权页面因此展开「业务 → 项目 → 环境」；业务/项目维度的授权经路径继承覆盖。
	actions = append(actions, client.ResourceAction{
		ID:                   AppCreate,
		Name:                 ActionIDNameMap[AppCreate],
		NameEn:               "Create APP",
		Type:                 Create,
		RelatedResourceTypes: environmentResource,
		RelatedActions:       []client.ActionID{BusinessViewResource},
		Version:              1,
	})

	relatedResource := []client.RelateResourceType{{
		SystemID:    SystemIDBSCP,
		ID:          Application,
		NameAlias:   "",
		NameAliasEn: "",
		Scope:       nil,
		InstanceSelections: []client.RelatedInstanceSelection{{
			SystemID: SystemIDBSCP,
			ID:       ApplicationSelection,
		}},
	}}

	actions = append(actions, client.ResourceAction{
		ID:                   AppView,
		Name:                 ActionIDNameMap[AppView],
		NameEn:               "View APP",
		Type:                 View,
		RelatedResourceTypes: relatedResource,
		RelatedActions:       []client.ActionID{BusinessViewResource},
		Version:              1,
	})

	actions = append(actions, client.ResourceAction{
		ID:                   AppEdit,
		Name:                 ActionIDNameMap[AppEdit],
		NameEn:               "Edit APP",
		Type:                 Edit,
		RelatedResourceTypes: relatedResource,
		RelatedActions:       []client.ActionID{BusinessViewResource, AppView},
		Version:              1,
	})

	actions = append(actions, client.ResourceAction{
		ID:                   AppDelete,
		Name:                 ActionIDNameMap[AppDelete],
		NameEn:               "Delete APP",
		Type:                 Delete,
		RelatedResourceTypes: relatedResource,
		RelatedActions:       []client.ActionID{BusinessViewResource, AppView},
		Version:              1,
	})

	actions = append(actions, client.ResourceAction{
		ID:                   ReleaseGenerate,
		Name:                 ActionIDNameMap[ReleaseGenerate],
		NameEn:               "Generate Release",
		Type:                 Edit,
		RelatedResourceTypes: relatedResource,
		RelatedActions:       []client.ActionID{BusinessViewResource, AppView},
		Version:              1,
	})

	actions = append(actions, client.ResourceAction{
		ID:                   ReleasePublish,
		Name:                 ActionIDNameMap[ReleasePublish],
		NameEn:               "Publish Release",
		Type:                 Edit,
		RelatedResourceTypes: relatedResource,
		RelatedActions:       []client.ActionID{BusinessViewResource, AppView},
		Version:              1,
	})

	return actions
}

// genCredentialActions 服务密钥
func genCredentialActions() []client.ResourceAction {
	actions := make([]client.ResourceAction, 0)

	// actions = append(actions, client.ResourceAction{
	// 	ID:                   CredentialCreate,
	// 	Name:                 ActionIDNameMap[CredentialCreate],
	// 	NameEn:               "Create App Credential",
	// 	Type:                 Create,
	// 	RelatedResourceTypes: businessResource,
	// 	RelatedActions:       []client.ActionID{BusinessViewResource},
	// 	Version:              1,
	// })

	// relatedResource := []client.RelateResourceType{{
	// 	SystemID:    SystemIDBSCP,
	// 	ID:          Application,
	// 	NameAlias:   "",
	// 	NameAliasEn: "",
	// 	Scope:       nil,
	// 	InstanceSelections: []client.RelatedInstanceSelection{{
	// 		SystemID: SystemIDCMDB,
	// 		ID:       BusinessSelection,
	// 	}},
	// }}

	// actions = append(actions, client.ResourceAction{
	// 	ID:                   CredentialView,
	// 	Name:                 ActionIDNameMap[CredentialView],
	// 	NameEn:               "View App Credential",
	// 	Type:                 View,
	// 	RelatedResourceTypes: relatedResource,
	// 	RelatedActions:       []client.ActionID{BusinessViewResource},
	// 	Version:              1,
	// })

	// actions = append(actions, client.ResourceAction{
	// 	ID:                   CredentialEdit,
	// 	Name:                 ActionIDNameMap[CredentialEdit],
	// 	NameEn:               "Edit APP Credential",
	// 	Type:                 Edit,
	// 	RelatedResourceTypes: relatedResource,
	// 	RelatedActions:       []client.ActionID{BusinessViewResource, CredentialView},
	// 	Version:              1,
	// })

	// actions = append(actions, client.ResourceAction{
	// 	ID:                   CredentialDelete,
	// 	Name:                 ActionIDNameMap[CredentialDelete],
	// 	NameEn:               "Delete App Credential",
	// 	Type:                 Delete,
	// 	RelatedResourceTypes: relatedResource,
	// 	RelatedActions:       []client.ActionID{BusinessViewResource, CredentialView},
	// 	Version:              1,
	// })

	actions = append(actions, client.ResourceAction{
		ID:                   CredentialView,
		Name:                 ActionIDNameMap[CredentialView],
		NameEn:               "View APP Credential",
		Type:                 View,
		RelatedResourceTypes: businessResource,
		RelatedActions:       []client.ActionID{BusinessViewResource},
		Version:              1,
	})

	actions = append(actions, client.ResourceAction{
		ID:                   CredentialManage,
		Name:                 ActionIDNameMap[CredentialManage],
		NameEn:               "Manage APP Credential",
		Type:                 Manage,
		RelatedResourceTypes: businessResource,
		RelatedActions:       []client.ActionID{BusinessViewResource, CredentialView},
		Version:              1,
	})

	return actions

}

// genGroupActions 应用分组
//
//nolint:unused
func genGroupActions() []client.ResourceAction {
	actions := make([]client.ResourceAction, 0)

	actions = append(actions, client.ResourceAction{
		ID:     GroupCreate,
		Name:   ActionIDNameMap[GroupCreate],
		NameEn: "Create Group",
		Type:   Create,
		RelatedResourceTypes: []client.RelateResourceType{{
			SystemID:    SystemIDBSCP,
			ID:          Application,
			NameAlias:   "",
			NameAliasEn: "",
			Scope:       nil,
			InstanceSelections: []client.RelatedInstanceSelection{{
				SystemID: SystemIDBSCP,
				ID:       ApplicationSelection,
			}},
		}},
		RelatedActions: []client.ActionID{BusinessViewResource},
		Version:        1,
	})

	relatedResource := []client.RelateResourceType{{
		SystemID:    SystemIDBSCP,
		ID:          Application,
		NameAlias:   "",
		NameAliasEn: "",
		Scope:       nil,
		InstanceSelections: []client.RelatedInstanceSelection{{
			SystemID: SystemIDBSCP,
			ID:       ApplicationSelection,
		}},
	}}

	actions = append(actions, client.ResourceAction{
		ID:                   GroupEdit,
		Name:                 ActionIDNameMap[GroupEdit],
		NameEn:               "Edit Group",
		Type:                 Edit,
		RelatedResourceTypes: relatedResource,
		RelatedActions:       nil,
		Version:              1,
	})

	actions = append(actions, client.ResourceAction{
		ID:                   GroupDelete,
		Name:                 ActionIDNameMap[GroupDelete],
		NameEn:               "Delete Group",
		Type:                 Delete,
		RelatedResourceTypes: relatedResource,
		RelatedActions:       nil,
		Version:              1,
	})

	return actions
}

func genBusinessActions() []client.ResourceAction {
	actions := make([]client.ResourceAction, 0)

	actions = append(actions, client.ResourceAction{
		ID:                   BusinessViewResource,
		Name:                 ActionIDNameMap[BusinessViewResource],
		NameEn:               "View Business",
		Type:                 View,
		RelatedResourceTypes: businessResource,
		RelatedActions:       nil,
		Version:              1,
	})

	return actions
}

func genAuditActions() []client.ResourceAction {
	actions := make([]client.ResourceAction, 0)

	actions = append(actions, client.ResourceAction{
		ID:                   AuditView,
		Name:                 ActionIDNameMap[AuditView],
		NameEn:               "View Audit",
		RelatedResourceTypes: businessResource,
		RelatedActions:       []client.ActionID{BusinessViewResource},
		Version:              1,
	})

	return actions
}

// 生成进程管理按钮
func genProcConfigMgmtActions() []client.ResourceAction {
	actions := make([]client.ResourceAction, 0)

	actions = append(actions, client.ResourceAction{
		ID:                   ProcConfigMgmtView,
		Name:                 ActionIDNameMap[ProcConfigMgmtView],
		NameEn:               "View Process And Config Management",
		Type:                 View,
		RelatedResourceTypes: businessResource,
		RelatedActions:       []client.ActionID{BusinessViewResource},
		Version:              1,
	})

	actions = append(actions, client.ResourceAction{
		ID:                   ProcessOperate,
		Name:                 ActionIDNameMap[ProcessOperate],
		NameEn:               "Process Operate",
		Type:                 Edit,
		RelatedResourceTypes: businessResource,
		RelatedActions:       []client.ActionID{BusinessViewResource, ProcConfigMgmtView},
		Version:              1,
	})

	actions = append(actions, client.ResourceAction{
		ID:                   ConfigTemplateCreate,
		Name:                 ActionIDNameMap[ConfigTemplateCreate],
		NameEn:               "Create Config Template",
		Type:                 Create,
		RelatedResourceTypes: businessResource,
		RelatedActions:       []client.ActionID{BusinessViewResource, ProcConfigMgmtView},
		Version:              1,
	})

	actions = append(actions, client.ResourceAction{
		ID:                   ConfigTemplateEdit,
		Name:                 ActionIDNameMap[ConfigTemplateEdit],
		NameEn:               "Edit Config Template",
		Type:                 Edit,
		RelatedResourceTypes: businessResource,
		RelatedActions:       []client.ActionID{BusinessViewResource, ProcConfigMgmtView},
		Version:              1,
	})

	actions = append(actions, client.ResourceAction{
		ID:                   ConfigTemplateDelete,
		Name:                 ActionIDNameMap[ConfigTemplateDelete],
		NameEn:               "Delete Config Template",
		Type:                 Delete,
		RelatedResourceTypes: businessResource,
		RelatedActions:       []client.ActionID{BusinessViewResource, ProcConfigMgmtView},
		Version:              1,
	})

	actions = append(actions, client.ResourceAction{
		ID:                   ConfigGenerate,
		Name:                 ActionIDNameMap[ConfigGenerate],
		NameEn:               "Generate Config",
		Type:                 Edit,
		RelatedResourceTypes: businessResource,
		RelatedActions:       []client.ActionID{BusinessViewResource, ProcConfigMgmtView},
		Version:              1,
	})

	actions = append(actions, client.ResourceAction{
		ID:                   ConfigRelease,
		Name:                 ActionIDNameMap[ConfigRelease],
		NameEn:               "Release Config",
		Type:                 Edit,
		RelatedResourceTypes: businessResource,
		RelatedActions:       []client.ActionID{BusinessViewResource, ProcConfigMgmtView},
		Version:              1,
	})

	return actions
}
