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

// Package itsmv4 xxx
package v4

import (
	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
)

// CreateTicketReq xxx
type CreateTicketReq struct {
	// WorkFlowKey 流程pk
	WorkFlowKey string `json:"workflow_key"`
	// ServiceID 服务pk
	ServiceID string `json:"service_id"`
	// FormData schema的实例化数据
	FormData map[string]any `json:"form_data"`
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
}

// Options options
type Options struct {
}

// CreateTicketResp xxx
type CreateTicketResp struct {
	api.CommonResp
	Data api.Ticket `json:"data"`
}

// ApprovalTicketReq
type ApprovalTicketReq struct {
	TicketID     string `json:"ticket_id"`
	TaskID       string `json:"task_id"`
	Operator     string `json:"operator"`
	OperatorType string `json:"operator_type"`
	SystemID     string `json:"system_id"`
	Action       string `json:"action"`
	Desc         string `json:"desc"`
}

type ApprovalTicketResp struct {
	api.CommonResp
	Data struct {
		Detail string `json:"detail"`
	} `json:"data"`
}

// RevokedTicketReq 撤销工单请求参数
type RevokedTicketReq struct {
	// SystemID 系统标识
	SystemID string `json:"system_id"`
	// TicketID 工单标识
	TicketID string `json:"ticket_id"`
}

// RevokedTicketResp 撤销工单返回参数
type RevokedTicketResp struct {
	api.CommonResp
	Data struct {
		Result bool `json:"result"`
	} `json:"data"`
}

// TicketDetailReq xxx
type TicketDetailReq struct {
	// ID 工单id
	ID string `json:"id"`
}

// TicketDetailResp xxx
type TicketDetailResp struct {
	api.CommonResp
	Data *api.Ticket `json:"data"`
}

// GetTicketLogsResp xxx
type GetTicketLogsResp struct {
	api.CommonResp
	Data *api.TicketLogsData `json:"data"`
}

// ListTicketReq 工单列表请求参数
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
}

// ListTicketResp xxx
type ListTicketResp struct {
	api.CommonResp
	Data *api.ListTicketsData `json:"data"`
}
