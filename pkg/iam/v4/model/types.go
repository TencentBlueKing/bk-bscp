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

// Package model defines the BSCP permission model for BK-IAM V4.
package model

// 资源类型 ID
const (
	// ResourceTypeBiz 业务，拓扑根节点
	ResourceTypeBiz = "biz"
	// ResourceTypeApp 服务，父级为业务
	ResourceTypeApp = "app"
	// ResourceTypeProject 项目，父级为业务
	ResourceTypeProject = "project"
	// ResourceTypeEnvironment 环境，父级为项目
	ResourceTypeEnvironment = "environment"
)

// 操作 ID
const (
	// ActionFindBusinessResource 业务访问，BSCP 所有业务接口的前置校验
	ActionFindBusinessResource = "find_business_resource"
	// ActionAppCreate 服务创建，按环境授权即"允许在哪个环境下创建服务"；
	// 创建时尚无服务实例，项目/业务维度的授权经拓扑路径继承覆盖到该环境。
	ActionAppCreate = "app_create"
	// ActionAppView 服务查看
	ActionAppView = "app_view"
	// ActionAppEdit 服务编辑
	ActionAppEdit = "app_edit"
	// ActionAppDelete 服务删除
	ActionAppDelete = "app_delete"
	// ActionReleaseGenerate 生成版本
	ActionReleaseGenerate = "release_generate"
	// ActionReleasePublish 上线版本
	ActionReleasePublish = "release_publish"
	// ActionAppCredentialView 服务密钥查看
	ActionAppCredentialView = "app_credential_view"
	// ActionAppCredentialManage 服务密钥管理
	ActionAppCredentialManage = "app_credential_manage"
	// ActionAuditView 操作记录查看
	ActionAuditView = "audit_view"
	// ActionProcConfigMgmtView 进程配置管理查看
	ActionProcConfigMgmtView = "proc_config_mgmt_view"
	// ActionProcessOperate 进程操作
	ActionProcessOperate = "process_operate"
	// ActionConfigTemplateCreate 配置模板创建
	ActionConfigTemplateCreate = "config_template_create"
	// ActionConfigTemplateEdit 配置模板编辑
	ActionConfigTemplateEdit = "config_template_edit"
	// ActionConfigTemplateDelete 配置模板删除
	ActionConfigTemplateDelete = "config_template_delete"
	// ActionConfigGenerate 配置生成
	ActionConfigGenerate = "config_generate"
	// ActionConfigRelease 配置下发
	ActionConfigRelease = "config_release"
	// ActionProjectCreate 项目创建，按业务授权（此时项目实例尚未存在）
	ActionProjectCreate = "project_create"
	// ActionProjectView 项目查看，按项目实例授权
	ActionProjectView = "project_view"
	// ActionProjectEdit 项目编辑，按项目实例授权
	ActionProjectEdit = "project_edit"
	// ActionProjectDelete 项目删除，按项目实例授权
	ActionProjectDelete = "project_delete"
	// ActionEnvironmentCreate 环境创建，按项目授权（此时环境实例尚未存在）
	ActionEnvironmentCreate = "environment_create"
	// ActionEnvironmentView 环境查看，按环境实例授权
	ActionEnvironmentView = "environment_view"
	// ActionEnvironmentEdit 环境编辑，按环境实例授权
	ActionEnvironmentEdit = "environment_edit"
	// ActionEnvironmentDelete 环境删除，按环境实例授权
	ActionEnvironmentDelete = "environment_delete"
)

const (
	// SystemName BSCP 在权限中心展示的系统名
	SystemName = "服务配置中心"

	// CallbackPath 权限中心回调 BSCP 拉取资源实例的路径
	CallbackPath = "/api/v1/auth/iam/find/resource"
)

// 角色 ID。RoleBizViewer 至 RoleAuditViewer 为业务维度角色，
// RoleAppOperator 至 RoleAppViewer 为服务维度角色
const (
	// RoleBizViewer 业务只读
	RoleBizViewer = "biz_viewer"
	// RoleBizOperator 业务运维
	RoleBizOperator = "biz_operator"
	// RoleAppCreator 服务创建者：只能在被授权的环境下新建服务，配合创建者自动授权获得自己所建服务的管理权，对他人的服务无权限
	RoleAppCreator = "app_creator"
	// RoleCredentialManager 服务密钥管理员
	RoleCredentialManager = "credential_manager"
	// RoleProcConfigManager 进程配置管理员
	RoleProcConfigManager = "proc_config_manager"
	// RoleAuditViewer 操作记录查看者
	RoleAuditViewer = "audit_viewer"
	// RoleAppOperator 服务运维，也是创建者自动授权所授予的角色
	RoleAppOperator = "app_operator"
	// RoleAppPublisher 服务发布者，分离配置编辑与发布权限
	RoleAppPublisher = "app_publisher"
	// RoleAppViewer 服务查看者，最小粒度只读
	RoleAppViewer = "app_viewer"
	// RoleProjectOperator 项目运维，管理指定项目及其全部环境
	RoleProjectOperator = "project_operator"
	// RoleProjectViewer 项目查看者，查看指定项目及其全部环境
	RoleProjectViewer = "project_viewer"
	// RoleEnvironmentOperator 环境运维，管理指定环境，也是环境创建者自动授权所授予的角色
	RoleEnvironmentOperator = "environment_operator"
	// RoleEnvironmentViewer 环境查看者，查看指定环境
	RoleEnvironmentViewer = "environment_viewer"
)
