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
package v2

import (
	"fmt"
	"strconv"

	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
)

// convertCreateTicketReq 将统一的 api.CreateTicketReq 转为 v2 的请求格式
func convertCreateTicketReq(req api.CreateTicketReq) map[string]any {
	return map[string]any{
		"creator":    req.Operator,
		"service_id": req.ServiceID,
		"fields":     req.Fields,
		"meta":       req.Meta,
	}
}

// convertCreateTicketResp 将 v2 的响应转为统一的 api.CreateTicketData
func convertCreateTicketResp(resp *CreateTicketData) *api.CreateTicketData {
	return &api.CreateTicketData{
		SN:        resp.SN,
		ID:        resp.ID,
		TicketURL: resp.TicketURL,
		StateID:   resp.StateID,
	}
}

func convertApprovalTicketReq(req api.ApprovalTicketReq) map[string]any {
	return map[string]any{
		"sn":             req.TicketID,
		"state_id":       req.StateId,
		"operator":       req.Operator,
		"action":         req.Action,
		"action_type":    req.ActionType,
		"action_message": req.ActionMessage,
		"approver":       req.Approver,
		"remark":         req.Desc,
	}
}

func convertGetTicketLogsResp(resp *GetTicketLogsDetail) *api.TicketLogsData {
	if resp == nil || len(resp.Logs) == 0 {
		return nil
	}

	data := make([]*api.TicketLogsDataItems, 0)

	for _, v := range resp.Logs {
		data = append(data, &api.TicketLogsDataItems{
			Operator: v.Operator,
			Message:  v.Message,
		})
	}

	return &api.TicketLogsData{Items: data}
}

func convertListTicketResp(resp *ListTicketsData) *api.ListTicketsData {

	if resp == nil || resp.Count == 0 {
		return nil
	}

	result := make([]*api.Ticket, 0)
	for _, v := range resp.Items {
		result = append(result, &api.Ticket{
			ID:          fmt.Sprintf("%d", v.ID),
			SN:          v.SN,
			Title:       v.Title,
			CreatedAt:   v.CreateAt,
			UpdatedAt:   v.UpdateAt,
			EndAt:       v.EndAt,
			Status:      v.CurrentStatus,
			ServiceID:   fmt.Sprintf("%d", v.ServiceID),
			CatalogID:   v.CatalogID,
			ServiceType: v.ServiceType,
			FlowID:      v.FlowID,
			CommentID:   v.CommentID,
			IsCommented: v.IsCommented,
			BkBizID:     v.BkBizID,
			TicketURL:   v.TicketURL,
		})
	}

	return &api.ListTicketsData{
		Results:  result,
		Page:     resp.Page,
		PageSize: resp.TotalPage,
		Count:    resp.Count,
	}
}

func convertListWorkflowReq(req api.ListWorkflowReq) int {
	id, _ := strconv.Atoi(req.WorkflowKeys)
	return id
}

func convertListWorkflowResp(resp map[string]int) map[string]string {

	dst := make(map[string]string, len(resp))
	for k, v := range resp {
		dst[k] = strconv.Itoa(v)
	}

	return dst
}
