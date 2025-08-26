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
package v4

import (
	"fmt"
	"strings"

	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
)

// convertCreateTicketReq 将统一的 CreateTicketReq 转为 v2 的请求格式
func convertCreateTicketReq(req api.CreateTicketReq) CreateTicketReq {
	// 处理 fields
	formData := make(map[string]any)
	for _, field := range req.Fields {
		k := fmt.Sprintf("%v", field["key"])
		v := field["value"]

		if k == "title" {
			formData["ticket__title"] = v
		} else {
			formData[strings.ToLower(k)] = v
		}
	}
	return CreateTicketReq{
		WorkFlowKey:   req.WorkFlowKey,
		ServiceID:     req.ServiceID,
		FormData:      formData,
		CallbackUrl:   req.CallbackUrl,
		CallbackToken: req.CallbackToken,
		Options:       Options{},
		SystemID:      systemCode,
		Operator:      req.Operator,
	}
}

// convertCreateTicketResp 将 v2 的响应转为统一的 CreateTicketResp
func convertCreateTicketResp(resp *CreateTicketResp) *api.CreateTicketData {
	return &api.CreateTicketData{
		SN: resp.Data.ID,
	}
}

func convertApprovalTicketReq(req api.ApprovalTicketReq) ApprovalTicketReq {

	return ApprovalTicketReq{
		TicketID:     req.TicketID,
		TaskID:       req.TaskID,
		Operator:     req.Operator,
		OperatorType: req.OperatorType,
		SystemID:     req.SystemID,
		Action:       req.Action,
		Desc:         req.Desc,
	}
}

func convertRevokedTicketReq(req api.ApprovalTicketReq) RevokedTicketReq {
	return RevokedTicketReq{
		SystemID: req.SystemID,
		TicketID: req.TicketID,
	}
}

func convertRevokedTicketResp(resp *RevokedTicketResp) *api.RevokedTicketResp {
	return &api.RevokedTicketResp{
		Result: resp.Data.Result,
	}
}

func convertListTicketsReq(req api.ListTicketsReq) ListTicketsReq {
	return ListTicketsReq{
		ViewType:            req.ViewType,
		Page:                req.Page,
		PageSize:            req.PageSize,
		WorkflowKeyIn:       req.WorkflowKeyIn,
		CurrentProcessorsIn: req.CurrentProcessorsIn,
		SnContains:          req.SnContains,
		TitleContains:       req.TitleContains,
		CreatorIn:           req.CreatorIn,
		StatusDisplayIn:     req.StatusDisplayIn,
		CreatedAtRange:      req.CreatedAtRange,
		SystemIdIn:          req.SystemIdIn,
		IdIn:                strings.Join(req.Sns, ","),
	}
}
