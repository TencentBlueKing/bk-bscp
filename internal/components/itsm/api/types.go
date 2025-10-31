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

// Package itsm xxx
package api

import (
	"encoding/json"
	"time"
)

// CreateTicketReq xxx
type CreateTicketReq struct {
	// WorkFlowKey 流程pk
	WorkFlowKey string `json:"workflow_key"`
	// ServiceID 服务pk
	ServiceID string `json:"service_id"`
	// Fields schema的实例化数据
	Fields []map[string]any `json:"fields"`
	// CallbackUrl 回调url，post请求
	CallbackUrl string `json:"callback_url"`
	// CallbackToken 回调时作为post参数传入，由业务系统自己控制鉴权
	CallbackToken string `json:"callback_token"`
	// Options xxx
	Options Options `json:"options"`
	// SystemID 如果传入system_id，需要在请求头加入SYSTEM-TOKEN
	SystemID string `json:"system_id"`
	// Operator 实际提单人
	Operator string `json:"operator"`
	// ActivityKey [v4]activity_key
	ActivityKey string         `json:"activity_key"`
	Meta        map[string]any `json:"meta"`
}

// Options options
type Options struct{}

// CreateTicketResp xxx
type CreateTicketResp struct {
	CommonResp
	Data CreateTicketData `json:"data"`
}

// CreateTicketData xxx
type CreateTicketData struct {
	SN        string `json:"sn"`
	ID        int    `json:"id"`
	TicketURL string `json:"ticket_url"`
	StateID   string `json:"state_id"`
}

// CommonResp xxx
type CommonResp struct {
	Code    string `json:"code"`
	Result  bool   `json:"result"`
	Message string `json:"message"`
}

// ApprovalTicketReq xxx
type ApprovalTicketReq struct {
	// 兼容v2版本的sn字段
	TicketID     string `json:"ticket_id"`
	TaskID       string `json:"task_id"`
	Operator     string `json:"operator"`
	OperatorType string `json:"operator_type"`
	SystemID     string `json:"system_id"`
	Action       string `json:"action"`
	// 兼容v2版本的remark字段
	Desc string `json:"desc"`
	// v2版本中的字段
	ActionMessage string `json:"action_message"`
	ActionType    string `json:"action_type"`
	Approver      string `json:"approver"`
	StateId       string `json:"state_id"`
}

// RevokedTicketReq xxx
type RevokedTicketReq struct {
	// SystemID 系统标识
	SystemID string `json:"system_id"`
	// TicketID 工单标识
	TicketID string `json:"ticket_id"`
	// v2版本中的字段
	ActionMessage string `json:"action_message"`
	ActionType    string `json:"action_type"`
	Operator      string `json:"operator"`
}

// GetTicketStatusReq xxx
type GetTicketStatusReq struct {
	// TicketID 工单标识 对应v2版本的sn
	TicketID string `json:"ticket_id"`
}

// GetTicketStatusDetail ticket status detail
type GetTicketStatusDetail struct {
	CurrentStatus string           `json:"current_status"`
	CurrentSteps  []map[string]any `json:"current_steps"`
}

// GetTicketLogsReq xxx
type GetTicketLogsReq struct {
	// TicketID 工单标识 对应v2版本的sn
	TicketID string `json:"ticket_id"`
}

// GetApproveResultReq xxx
type GetApproveResultReq struct {
	TicketID string `json:"ticket_id"`
	// v2 使用state_id 匹配结果
	StateID string `json:"state_id"`
	// v4使用activity_key 匹配结果
	ActivityKey string `json:"activity_key"`
}

// TicketDetailReq xxx
type TicketDetailReq struct {
	// ID 工单id
	ID string `json:"id"`
}

// Ticket xxx
type Ticket struct {
	// ID 工单ID
	ID string `json:"id"`
	// SN 工单单号
	SN string `json:"sn"`
	// Title 工单标题
	Title string `json:"title"`
	// CreatedAt 提单时间
	CreatedAt string `json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt string `json:"updated_at"`
	// EndAt 结束时间
	EndAt string `json:"end_at"`
	// Status 状态标识
	Status string `json:"status"`
	// StatusDisplay 状态展示名
	StatusDisplay string `json:"status_display"`
	// WorkflowID 流程ID
	WorkflowID string `json:"workflow_id"`
	// ServiceID 服务ID
	ServiceID string `json:"service_id"`
	// PortalID 门户ID
	PortalID string `json:"portal_id"`
	// CurrentProcessors 当前处理人列表
	CurrentProcessors []Processor `json:"current_processors"`
	// CurrentSteps 当前步骤列表
	CurrentSteps []Step `json:"current_steps"`
	// FrontendURL 工单前端访问地址
	FrontendURL string `json:"frontend_url"`
	// FormData 工单表单实例化数据
	FormData json.RawMessage `json:"form_data"`
	// ApproveResult 审批结果
	ApproveResult bool `json:"approve_result"`
	// CallbackResult 回调结果
	CallbackResult CallbackResult `json:"callback_result"`
	// v2版本中的字段
	CatalogID   int    `json:"catalog_id"`
	ServiceType string `json:"service_type"`
	FlowID      int    `json:"flow_id"`
	CommentID   string `json:"comment_id"`
	IsCommented bool   `json:"is_commented"`
	BkBizID     int    `json:"bk_biz_id"`
	TicketURL   string `json:"ticket_url"`
	// 提单人
	Creator string `json:"creator"`
}

// Step 步骤信息
type Step struct {
	TicketID string `json:"ticket_id"`
	// Name 步骤名称
	Name string `json:"name"`
	// ActivityKey [v4]activity_key
	ActivityKey string `json:"activity_key"`
	// TaskID: [v4] task id
	TaskID string `json:"task_id"`
}

// CallbackResult 回调结果
type CallbackResult struct {
	// Result 回调接口最外层的result信息
	Result bool `json:"result"`
	// Message 回调报错信息或者回调接口最外层的message信息
	Message string `json:"message"`
}

// Processor 处理人信息
type Processor struct {
	TicketID string `json:"ticket_id"`
	// Processor 类型处理人标识列表字符串
	Processor string `json:"processor"`
	// ProcessorType 处理人类型: user/group/organization
	ProcessorType string `json:"processor_type"`
	// TaskID 任务ID
	TaskID string `json:"task_id"`
}

type RevokedTicketResp struct {
	// Result 	是否撤销成功
	Result bool `json:"result"`
}

// TicketLogsData xxx
type TicketLogsData struct {
	Items []*TicketLogsDataItems `json:"items"`
}

// ApproveResultData 审批结果数据
type ApproveResultData struct {
	Result      *bool                    `json:"result"` // nil 还没有审批，true 已经审批通过，false 已经审批驳回
	RejectUsers []string                 `json:"reject_users"`
	PassUsers   []string                 `json:"pass_users"`
	Reasons     []string                 `json:"reason"`
	Items       []*ApproveResultDataItem `json:"items"`
}

// ApproveResultDataItem 单项审批结果
type ApproveResultDataItem struct {
	Result   *bool  `json:"result"` // nil 还没有审批，true 已经审批通过，false 已经审批驳回
	Reason   string `json:"reason"`
	Operator string `json:"operator"` // 操作人
}

// TicketLogsDataItems xxx
type TicketLogsDataItems struct {
	TicketID        string       `json:"ticket_id"`
	ActivityKey     string       `json:"activity_key"`
	ActivityName    string       `json:"activity_name"`
	ActivityType    string       `json:"activity_type"`
	Operator        string       `json:"operator"`
	OperatorType    string       `json:"operator_type"`
	Agent           string       `json:"agent"`
	AgentType       string       `json:"agent_type"`
	OperateAt       time.Time    `json:"operate_at"`
	Action          string       `json:"action"`
	ActionDisplay   string       `json:"action_display"`
	Extra           []any        `json:"extra"`
	Translations    Translations `json:"translations"`
	AgentDisplay    string       `json:"agent_display"`
	OperatorDisplay string       `json:"operator_display"`
	Message         string       `json:"message"`
}

// Translations xxx
type Translations struct {
	ActivityName    string `json:"activity_name"`
	ActivityNameEn  string `json:"activity_name_en"`
	ActionDisplay   string `json:"action_display"`
	ActionDisplayEn string `json:"action_display_en"`
}

// ListTicketsReq xxx
type ListTicketsReq struct {
	// ViewType 视图类型，默认"all"
	ViewType string `json:"view_type"`
	// Page 页码，默认1
	Page int `json:"page"`
	// PageSize 页大小，默认10，最大50
	PageSize int `json:"page_size"`
	// WorkflowKeyIn 逗号隔开的多个流程id
	WorkflowKeyIn string `json:"workflow_key__in"`
	// CurrentProcessorsIn 逗号隔开的多个用户对象
	CurrentProcessorsIn string `json:"current_processors__in"`
	// SnContains 单号模糊查询
	SnContains string `json:"sn__contains"`
	// TitleContains 标题模糊查询
	TitleContains string `json:"title__contains"`
	// CreatorIn 逗号隔开的多个username
	CreatorIn string `json:"creator__in"`
	// StatusDisplayIn 逗号隔开的多个状态名
	StatusDisplayIn string `json:"status_display__in"`
	// CreatedAtRange 提单时间范围
	CreatedAtRange string `json:"created_at__range"`
	// SystemIdIn 逗号隔开的多个系统标识
	SystemIdIn string `json:"system_id__in"`
	// IdIn 逗号隔开的多个工单id
	IdIn string `json:"id__in"`
	// v2版本中的字段
	Sns []string `json:"sns"`
}

// ListTicketsData 工单列表响应数据
type ListTicketsData struct {
	// Results 工单详情数据
	Results  []*Ticket `json:"results"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
	// Count 总数
	Count int `json:"count"`
}

// GetApproveNodeResultReq xxx
type GetApproveNodeResultReq struct {
	// TicketID 工单标识 对应v2版本的sn
	TicketID string `json:"ticket_id"`
	// v2 版本中的字段
	StateID string `json:"state_id"`
}

// GetApproveNodeResultResp xxx
type GetApproveNodeResultResp struct {
	CommonResp
	Data *GetApproveNodeResultDetail `json:"data"`
}

// GetApproveNodeResultDetail xxx
type GetApproveNodeResultDetail struct {
	Name          string `json:"name"`
	Processeduser string `json:"processed_user"`
	ApproveResult bool   `json:"approve_result"`
	ApproveRemark string `json:"approve_remark"`
}

// ApprovalTicketReq xxx
type ApprovalTasksReq struct {
	TicketID    string `json:"ticket_id"`
	ActivityKey string `json:"activity_key"`
}

// TasksData xxx
type TasksData struct {
	Items []*Tasks `json:"items"`
}

// Tasks xxx
type Tasks struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	ActivityKey       string    `json:"activity_key"`
	Desc              string    `json:"desc"`
	Type              string    `json:"type"`
	Status            string    `json:"status"`
	StatusDisplay     string    `json:"status_display"`
	Operator          string    `json:"operator"`
	OperatorType      string    `json:"operator_type"`
	OperatorAt        time.Time `json:"operator_at"`
	CurrentProcessors []any     `json:"current_processors"`
}

// ListWorkflowReq xxx
type ListWorkflowReq struct {
	WorkflowKeys string `json:"workflow_keys"`
}
