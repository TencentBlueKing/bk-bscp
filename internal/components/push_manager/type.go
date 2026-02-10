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

// Package pushmanager provides bcs push manager api client.
package pushmanager

// BaseResponse 所有接口通用响应结构
type BaseResponse struct {
	// Code 返回码
	// 0: 成功
	// 400: 请求参数无效
	// 404: 资源未找到
	// 500: 服务器内部错误
	Code int `json:"code"`
	// Message 返回信息
	Message string `json:"message"`
}

// Pagination 通用分页参数
type Pagination struct {
	// Page 页码，默认 1
	Page int `json:"page,omitempty"`
	// PageSize 每页数量，默认 100
	PageSize int `json:"page_size,omitempty"`
}

const (
	DefaultPage     = 1
	DefaultPageSize = 100
)

// EventStatus 推送事件状态
type EventStatus int

const (
	EventStatusPending     EventStatus = 0 // 待处理
	EventStatusSuccess     EventStatus = 1 // 推送成功
	EventStatusFailed      EventStatus = 2 // 推送失败
	EventStatusWhitelisted EventStatus = 3 // 已被白名单屏蔽
)

// PushLevel 告警级别
type PushLevel string

const (
	PushLevelFatal    PushLevel = "fatal"    // 致命告警
	PushLevelWarning  PushLevel = "warning"  // 警告告警（默认）
	PushLevelReminder PushLevel = "reminder" // 提醒告警
)

// PushType 推送类型（可组合，逗号分隔）
type PushType string

const (
	PushTypeRTX  PushType = "rtx"  // 企业微信 RTX
	PushTypeMail PushType = "mail" // 邮件
	PushTypeMsg  PushType = "msg"  // bkchat 消息
)

// WhitelistStatus 白名单状态
type WhitelistStatus int

const (
	WhitelistStatusNone    WhitelistStatus = 0 // 未加白
	WhitelistStatusActive  WhitelistStatus = 1 // 已加白
	WhitelistStatusExpired WhitelistStatus = 2 // 已过期
)

// ApprovalStatus 审批状态
type ApprovalStatus int

const (
	ApprovalStatusPending  ApprovalStatus = 0 // 待审批
	ApprovalStatusApproved ApprovalStatus = 1 // 已批准
	ApprovalStatusRejected ApprovalStatus = 2 // 已拒绝
)

// Dimension 维度信息
type Dimension struct {
	// Fields 维度字段（不能为空）
	// 示例：{"cluster_id":"xxx","namespace":"default"}
	Fields map[string]string `json:"fields"`
}

// MetricData 指标数据
type MetricData struct {
	// Timestamp 指标时间戳（RFC3339）
	Timestamp string `json:"timestamp,omitempty"`
	// MetricValue 指标值
	MetricValue float64 `json:"metric_value,omitempty"`
}

// PushEvent 推送事件
type PushEvent struct {
	EventID             string               `json:"event_id,omitempty"`
	Domain              string               `json:"domain"`
	RuleID              string               `json:"rule_id,omitempty"`
	EventDetail         PushEventDetail      `json:"event_detail"`
	PushLevel           PushLevel            `json:"push_level,omitempty"`
	Status              EventStatus          `json:"status,omitempty"`
	NotificationResults *NotificationResults `json:"notification_results,omitempty"`
	Dimension           *Dimension           `json:"dimension,omitempty"`
	BkBizName           string               `json:"bk_biz_name,omitempty"`
	MetricData          *MetricData          `json:"metric_data,omitempty"`
	CreatedAt           string               `json:"created_at,omitempty"`
	UpdatedAt           string               `json:"updated_at,omitempty"`
}

// PushEventDetail 推送事件详情
type PushEventDetail struct {
	Fields PushEventFields `json:"fields"`
}

// PushEventFields 推送内容字段
type PushEventFields struct {
	Types string `json:"types"`
	// RTX
	RTXReceivers string `json:"rtx_receivers,omitempty"`
	RTXTitle     string `json:"rtx_title,omitempty"`
	RTXContent   string `json:"rtx_content,omitempty"`
	// Mail
	MailReceivers string `json:"mail_receivers,omitempty"`
	MailTitle     string `json:"mail_title,omitempty"`
	MailContent   string `json:"mail_content,omitempty"`
	// Msg
	MsgReceivers string `json:"msg_receivers,omitempty"`
	MsgContent   string `json:"msg_content,omitempty"`
}

// NotificationResults 推送结果
type NotificationResults struct {
	// Fields 推送渠道返回结果，如 rtx_status
	Fields map[string]string `json:"fields"`
}

// CreatePushEventResponse 创建推送事件请求
type CreatePushEventRequest struct {
	Event PushEvent `json:"event"`
}

// CreatePushEventResponse 创建推送事件响应
type CreatePushEventResponse struct {
	BaseResponse
	EventID string `json:"event_id"`
}

// GetPushEventResponse 获取推送事件响应
type GetPushEventResponse struct {
	BaseResponse
	Event PushEvent `json:"event"`
}

// UpdatePushEventRequest 更新推送事件请求
type UpdatePushEventRequest struct {
	Event PushEvent `json:"event"`
}

// ListPushEventsResponse 列出推送事件请求
type ListPushEventsRequest struct {
	Pagination
	// RuleID 按规则 ID 过滤
	RuleID string `json:"rule_id,omitempty"`
	// Status 按事件状态过滤
	// 枚举值：0 / 1 / 2 / 3
	Status EventStatus `json:"status,omitempty"`
	// PushLevel 按告警级别过滤
	// fatal / warning / reminder
	PushLevel *PushLevel `json:"push_level,omitempty"`
	// StartTime 开始时间（RFC3339），需与 EndTime 同时使用
	StartTime string `json:"start_time,omitempty"`
	// EndTime 结束时间（RFC3339），需与 StartTime 同时使用
	EndTime string `json:"end_time,omitempty"`
}

// ListPushEventsResponse 列出推送事件响应
type ListPushEventsResponse struct {
	BaseResponse
	Events []PushEvent `json:"events"`
	Total  string      `json:"total"`
}

// PushWhitelist 推送白名单
type PushWhitelist struct {
	WhitelistID     string          `json:"whitelist_id"`
	Domain          string          `json:"domain"`
	Dimension       Dimension       `json:"dimension"`
	Reason          string          `json:"reason"`
	Applicant       string          `json:"applicant"`
	Approver        string          `json:"approver,omitempty"`
	WhitelistStatus WhitelistStatus `json:"whitelist_status"`
	ApprovalStatus  ApprovalStatus  `json:"approval_status"`
	StartTime       string          `json:"start_time"`
	EndTime         string          `json:"end_time"`
	ApprovedAt      string          `json:"approved_at,omitempty"`
	CreatedAt       string          `json:"created_at,omitempty"`
	UpdatedAt       string          `json:"updated_at,omitempty"`
}

type CreatePushWhitelistRequest struct {
	Whitelist PushWhitelist `json:"whitelist"`
}

type GetPushWhitelistResponse struct {
	BaseResponse
	Whitelist PushWhitelist `json:"whitelist"`
}

// ListPushWhitelistsRequest 推送白名单列表请求
type ListPushWhitelistsRequest struct {
	Pagination
	// Applicant 按申请人过滤
	Applicant string `json:"applicant,omitempty"`
	// WhitelistStatus 按白名单状态过滤
	// 0: 未加白 / 1: 已加白 / 2: 已过期
	WhitelistStatus *WhitelistStatus `json:"whitelist_status,omitempty"`
	// ApprovalStatus 按审批状态过滤
	// 0: 待审批 / 1: 已批准 / 2: 已拒绝
	ApprovalStatus *ApprovalStatus `json:"approval_status,omitempty"`
}

// ListPushWhitelistsResponse 推送白名单列表响应
type ListPushWhitelistsResponse struct {
	BaseResponse
	Whitelists []PushWhitelist `json:"whitelists"`
	Total      string          `json:"total"`
}

type UpdatePushWhitelistRequest struct {
	Whitelist PushWhitelist `json:"whitelist"`
}

// PushTemplate 推送模板
type PushTemplate struct {
	TemplateID   string          `json:"template_id"`
	Domain       string          `json:"domain"`
	TemplateType string          `json:"template_type"`
	Content      TemplateContent `json:"content"`
	Creator      string          `json:"creator,omitempty"`
	CreatedAt    string          `json:"created_at,omitempty"`
}

// TemplateContent 模板内容
type TemplateContent struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Variables []string `json:"variables,omitempty"`
}
type CreatePushTemplateRequest struct {
	Template PushTemplate `json:"template"`
}

type GetPushTemplateResponse struct {
	BaseResponse
	Template PushTemplate `json:"template"`
}

// ListPushTemplatesRequest 推送模板列表请求
type ListPushTemplatesRequest struct {
	Pagination
	// TemplateType 模板类型过滤
	// rtx / mail / msg
	TemplateType string `json:"template_type,omitempty"`
	// Creator 按创建者过滤
	Creator string `json:"creator,omitempty"`
}

// ListPushTemplatesResponse 推送模板列表响应
type ListPushTemplatesResponse struct {
	BaseResponse
	Templates []PushTemplate `json:"templates"`
	Total     string         `json:"total"`
}

type UpdatePushTemplateRequest struct {
	Template PushTemplate `json:"template"`
}
