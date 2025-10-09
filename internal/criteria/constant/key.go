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

package constant

// Note:
// This scope is used to define all the constant keys which is used inside and outside
// the BSCP system except sidecar.
const (
	// KitKey
	KitKey = "X-BSCP-KIT"

	// RidKey is request id header key.
	RidKey = "X-Bkapi-Request-Id"
	// RidKeyGeneric for generic header key
	RidKeyGeneric = "X-Request-Id"

	// LangKey is language key
	LangKey = "X-Bkapi-Language"

	// UserKey is operator name header key.
	UserKey = "X-Bkapi-User-Name"

	// AppCodeKey is blueking application code header key.
	AppCodeKey = "X-Bkapi-App-Code"

	// OperateWayKey is approve operate way header key.
	OperateWayKey = "X-Bscp-Operate-Way"

	// Space
	SpaceIDKey     = "X-Bkapi-Space-Id"
	SpaceTypeIDKey = "X-Bkapi-Space-Type-Id"
	BizIDKey       = "X-Bkapi-Biz-Id"
	AppIDKey       = "X-Bkapi-App-Id"

	// LanguageKey the language key word.
	LanguageKey = "HTTP_BLUEKING_LANGUAGE"

	// BKGWJWTTokenKey is blueking api gateway jwt header key.
	BKGWJWTTokenKey = "X-Bkapi-JWT" //nolint

	// BKTokenForTest is a token for test
	BKTokenForTest = "bk-token-for-test" //nolint:gosec

	// BKUserForTestPrefix is a user prefix for test
	BKUserForTestPrefix = "bk-user-for-test-"

	// BKSystemUser can be saved for user field in db when some operations come from bscp system itself
	BKSystemUser = "system"

	// ContentIDHeaderKey is common content sha256 id.
	ContentIDHeaderKey = "X-Bkapi-File-Content-Id"
	// PartNumHeaderKey is multipart upload part num key.
	PartNumHeaderKey = "X-Bscp-Part-Num"
	// MultipartUploadID is multipart upload id key.
	UploadIDHeaderKey = "X-Bscp-Upload-Id"
	// AppIDHeaderKey is app id.
	AppIDHeaderKey = "X-Bscp-App-Id"
	// TmplSpaceIDHeaderKey is template space id.
	//nolint:gosec
	TmplSpaceIDHeaderKey = "X-Bscp-Template-Space-Id"

	// TemplateVariablePrefix is the prefix for template variable name
	TemplateVariablePrefix = "bk_bscp_"

	// MaxRenderBytes is the max bytes to render for template config which is 2MB
	MaxRenderBytes = 2 * 1024 * 1024
)

// default resource
const (
	// DefaultTmplSpaceName is default template space name
	DefaultTmplSpaceName = "default_space"
	// DefaultTmplSpaceCNName is default template space chinese name
	DefaultTmplSpaceCNName = "默认空间"
	// DefaultTmplSpaceMemo is default template space memo
	DefaultTmplSpaceMemo = "this is default space"
	// DefaultTmplSetName is default template set name
	DefaultTmplSetName = "默认套餐"
	// DefaultTmplSetMemo is default template set memo
	DefaultTmplSetMemo = "当前空间下的所有模版"

	// DefaultLanguage is default language
	DefaultLanguage = "zh-cn"
)

// Note:
// 1. This scope defines keys which is used only by sidecar and feed server.
// 2. All the defined key should be prefixed with 'Side'.
const (
	// SidecarMetaKey defines the key to store the sidecar's metadata info.
	SidecarMetaKey = "sidecar-meta"
	// SideRidKey defines the incoming request id between sidecar and feed server.
	SideRidKey = "side-rid"
	// SideWorkspaceDir sidecar workspace dir name.
	SideWorkspaceDir = "bk-bscp"
)

const (
	// AuthLoginProviderKey is auth login provider
	AuthLoginProviderKey = "auth-login-provider"
	// AuthLoginUID is auth login uid
	AuthLoginUID = "auth-login-uid"
	// AuthLoginToken is auth login token
	AuthLoginToken = "auth-login-token" //nolint
)

var (
	// RidKeys support request_id keys
	RidKeys = []string{
		RidKey,
		RidKeyGeneric,
	}
)

// 文件状态，未命名版本服务配置项相比上一个版本的变化
const (
	// FileStateAdd 增加
	FileStateAdd = "ADD"
	// FileStateDelete 删除
	FileStateDelete = "DELETE"
	// FileStateRevise 修改
	FileStateRevise = "REVISE"
	// FileStateUnchange 不变
	FileStateUnchange = "UNCHANGE"
)

const (
	// MaxUploadTextFileSize 最大上传文件大小
	MaxUploadTextFileSize = 5 * 1024 * 1024
	// MaxConcurrentUpload 限制上传文件并发数
	MaxConcurrentUpload = 10
	// UploadBatchSize 上传时分批检测文件路冲突
	UploadBatchSize = 50
	// UploadTemporaryDirectory 上传的临时目录
	UploadTemporaryDirectory = "upload/files"
	// MB 字节
	MB = 1 << 20 // 1MB = 2^20 bytes
)

const (
	// LabelKeyAgentID is the key of agent id in bcs node labels.
	LabelKeyAgentID = "bkcmdb.tencent.com/bk-agent-id"
)

// itsm相关
const (
	// CreateApproveItsmServiceID used to create an itsm ticket
	// when creating an approve in a shared cluster
	CreateApproveItsmServiceID = "create_approve_itsm_service_id"
	// CreateApproveItsmWorkflowID used to create an itsm ticket
	// when creating an or sign approve in a shared cluster
	CreateApproveItsmWorkflowID = "create_approve_itsm_workflow_id"
	// CreateOrSignApproveItsmStateID used to create an itsm ticket
	// when creating an or sign approve in a shared cluster
	CreateOrSignApproveItsmStateID = "create_or_sign_approve_itsm_state_id"
	// CreateApproveItsmWorkflowID used to create an itsm ticket
	// when creating an count sign approve in a shared cluster
	CreateCountSignApproveItsmStateID = "create_count_sign_approve_itsm_state_id"

	// ItsmTicketStatusCreated enum string for created status
	ItsmTicketStatusCreated = "created"
	// ItsmTicketStatusRevoked enum string for revoked status
	ItsmTicketStatusRevoked = "revoked"
	// ItsmTicketStatusRejected enum string for rejected status
	ItsmTicketStatusRejected = "rejected"
	// ItsmTicketStatusPassed enum string for passed status
	ItsmTicketStatusPassed = "passed"

	// ItsmTicketTypeCreate enum string for itsm ticket type create
	ItsmTicketTypeCreate = "create"
	// ItsmTicketTypeUpdate enum string for itsm ticket type update
	ItsmTicketTypeUpdate = "update"
	// ItsmTicketTypeDelete enum string for itsm ticket type delete
	ItsmTicketTypeDelete = "delete"

	// ItsmApproveCountSingType 会签审批
	ItsmApproveCountSignType = "会签审批"
	// ItsmApproveOrSignType 或签审批
	ItsmApproveOrSignType = "或签审批"
	// 负责人审批类型
	ItsmApproveType = "APPROVAL"
	// ItsmApproveServiceName 服务名称
	ItsmApproveServiceName = "创建上线审批"
	// ItsmPassApproveResult itsm已处理人的结果
	ItsmPassedApproveResult = "通过" // nolint: gosec
	// ItsmRejectApproveResult itsm已处理人的结果
	ItsmRejectedApproveResult = "拒绝"

	// itsm 审批任务状态：approve(通过) refuse(拒绝) revoked(撤单)
	ItsmApproveAction = "approve"
	ItsmRefuseAction  = "refuse"
	ItsmRevokedAction = "revoked"

	// 单据状态:

	// TicketRunningStatus 处理中
	TicketRunningStatus = "RUNNING"
	// TicketFinishedStatus 已结束
	TicketFinishedStatus = "FINISHED"
	// TicketTerminatedStatus 被终止
	TicketTerminatedStatus = "TERMINATED"
	// TicketSuspendedStatus 被挂起
	TicketSuspendedStatus = "SUSPENDED"
	// TicketRevokedStatus 被撤销
	TicketRevokedStatus = "REVOKED"
)

// 操作记录资源实例相关
const (
	// ResSeparator 不同资源名称叠加时分隔符
	ResSeparator = "\n"
	// NameSeparator 相同资源名称叠加时分隔符
	NameSeparator = ", "
	// AppName 服务名称
	AppName = "app_name: %s"
	// ConfigFileAbsolutePath 配置文件绝对路径
	ConfigFileAbsolutePath = "config_file_absolute_path: %s"
	// ConfigItemName 配置项名称
	ConfigItemName = "config_item_name: %s"
	// HookName 脚本名称
	HookName = "hook_name: %s"
	// VariableName 变量名称
	VariableName = "variable_name: %s"
	// SetVariableName 设置变量名称
	SetVariableName = "set_variable_name: %s"
	// ConfigReleaseName 配置版本名称
	ConfigReleaseName = "config_release_name: %s"
	// ConfigReleaseScope 配置上线范围
	ConfigReleaseScope = "config_release_scope: %s"
	// GroupName 分组名称
	GroupName = "group_name: %s"
	// HookRevisionName 脚本版本名称
	HookRevisionName = "hook_revision_name: %s"
	// TemplateSpaceName 模版空间名称
	TemplateSpaceName = "template_space_name: %s"
	// TemplateSetName 模版套餐名称
	TemplateSetName = "template_set_name: %s"
	// TemplateAbsolutePath 模版文件绝对路径
	TemplateAbsolutePath = "template_absolute_path: %s"
	// TemplateRevision 模版版本号
	TemplateRevision = "template_revision: %s"
	// CredentialName 密钥名称
	CredentialName = "credential_name: %s" // nolint
	// ReferenceHookName 引用脚本名称
	ReferenceHookName = "reference_%s_name: %s"
	// ReplaceHookName 更换脚本名称
	ReplaceHookName = "replace_%s_name: %s"
	// CancelPreHookName 取消脚本名称
	CancelHookName = "cancel_%s_name: %s"
	// ObsoleteConfigReleaseName 废弃配置版本名称
	ObsoleteConfigReleaseName = "obsolete_config_release_name: %s"
	// RestoreConfigReleaseName 恢复配置版本名称
	RestoreConfigReleaseName = "restore_config_release_name: %s"
	// DeleteConfigReleaseName 删除配置版本名称
	DeleteConfigReleaseName = "delete_config_release_name: %s"
	// CredentialEnableName 启用密钥名称
	CredentialEnableName = "credential_enable_name: %s" // nolint
	// CredentialUnableName 禁用密钥名称
	CredentialUnableName = "credential_unable_name: %s" // nolint
	// AssociatedAppConfigCredentialName 关联服务配置密钥名称
	AssociatedAppConfigCredentialName = "associated_app_config_credential_name: %s" // nolint
	// ConfigRetryClientUID 配置重新拉取客户端UID
	ConfigRetryClientUID = "config_retry_client_uid: %s"
	// ConfigRetryClientIp 配置重新拉取客户端IP
	ConfigRetryClientIp = "config_retry_client_ip: %s"
	// OperateObject 等 xx 个对象进行操作
	OperateObject = "operate_objects: %d" // nolint
)
