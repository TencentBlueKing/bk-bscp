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

// Package enumor is enum of the audit
package enumor

/*
	audit.go store audit related enum values.
*/

// AuditResourceType audit resource type.
type AuditResourceType string

const (
	// App 应用模块资源
	App AuditResourceType = "app"
	// Config 配置资源
	Config AuditResourceType = "config"
	// Hook hook脚本资源
	Hook AuditResourceType = "hook"
	// Variable 变量
	Variable AuditResourceType = "variable"
	// Release 版本资源
	Release AuditResourceType = "release"
	// Group 分组资源
	Group AuditResourceType = "group"
	// Template 模版
	Template AuditResourceType = "template"
	// Credential 客户端秘钥
	Credential AuditResourceType = "credential"
	// Instance 客户端实例
	Instance AuditResourceType = "instance"
)

// AuditAction audit action type.
type AuditAction string

const (
	// Create 创建
	Create AuditAction = "create"
	// Update 更新
	Update AuditAction = "update"
	// Delete 删除
	Delete AuditAction = "delete"
	// Publish 发布
	Publish AuditAction = "publish"
)

// AuditStatus audit status.
type AuditStatus string

const (
	// Success audit status
	Success AuditStatus = "success"
	// Failure audit status
	Failure AuditStatus = "failure"
	// PendingApproval means this strategy audit status is pending.
	PendingApproval AuditStatus = "pending_approval"
	// PendingPublish means this strategy audit status is pending.
	PendingPublish AuditStatus = "pending_publish"
	// RevokedPublish means this strategy audit status is revoked.
	RevokedPublish AuditStatus = "revoked_publish"
	// RejectedApproval means this strategy audit status is rejected.
	RejectedApproval AuditStatus = "rejected_approval"
	// AlreadyPublish means this strategy audit status is already publish.
	AlreadyPublish AuditStatus = "already_publish"
)

// AuditOperateWay audit operate way.
type AuditOperateWay string

const (
	// WebUI audit operate way
	WebUI AuditOperateWay = "WebUI"
	// API audit operate way
	API AuditOperateWay = "API"
)
