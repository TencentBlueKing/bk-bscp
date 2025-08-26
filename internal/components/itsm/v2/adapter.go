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
	"context"
	"fmt"

	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
)

// ITSMV2Adapter xxx
type ITSMV2Adapter struct{}

// ListWorkflow implements itsm.Service.
func (a *ITSMV2Adapter) ListWorkflow(ctx context.Context, req api.ListWorkflowReq) (map[string]string, error) {
	v2Resp, err := GetStateApproveByWorkfolw(ctx, convertListWorkflowReq(req))
	if err != nil {
		return nil, err
	}
	return convertListWorkflowResp(v2Resp), nil
}

// ApprovalTasks implements itsm.Service.
func (a *ITSMV2Adapter) ApprovalTasks(ctx context.Context, req api.ApprovalTasksReq) (*api.TasksData, error) {
	return nil, fmt.Errorf("ApprovalTasks is not supported in v2 adapter")
}

// GetTicketLogs implements itsm.ITSMService.
func (a *ITSMV2Adapter) GetTicketLogs(ctx context.Context, req api.GetTicketLogsReq) (*api.TicketLogsData, error) {
	v2Resp, err := GetTicketLogs(ctx, req.TicketID)
	if err != nil {
		return nil, err
	}

	return convertGetTicketLogsResp(v2Resp), nil
}

// GetTicketStatus implements itsm.ITSMService.
func (a *ITSMV2Adapter) GetTicketStatus(ctx context.Context, req api.GetTicketStatusReq) (*api.GetTicketStatusDetail, error) {
	return GetTicketStatus(ctx, req.TicketID)
}

// CreateTicket implements itsm.ITSMService.
func (a *ITSMV2Adapter) CreateTicket(ctx context.Context, req api.CreateTicketReq) (*api.CreateTicketData, error) {
	v2Resp, err := CreateTicket(ctx, convertCreateTicketReq(req))
	if err != nil {
		return &api.CreateTicketData{}, err
	}
	return convertCreateTicketResp(v2Resp), nil
}

// ApprovalTicket implements itsm.ITSMService.
func (a *ITSMV2Adapter) ApprovalTicket(ctx context.Context, req api.ApprovalTicketReq) error {
	return UpdateTicketByApporver(ctx, convertApprovalTicketReq(req))
}

// RevokedTicket implements itsm.ITSMService.
func (a *ITSMV2Adapter) RevokedTicket(ctx context.Context, req api.ApprovalTicketReq) (*api.RevokedTicketResp, error) {
	err := WithdrawTicket(ctx, convertApprovalTicketReq(req))
	if err != nil {
		return &api.RevokedTicketResp{Result: false}, err
	}
	return &api.RevokedTicketResp{Result: true}, nil
}

// ListTickets implements itsm.ITSMService.
func (a *ITSMV2Adapter) ListTickets(ctx context.Context, req api.ListTicketsReq) (*api.ListTicketsData, error) {
	v2Resp, err := ListTickets(ctx, ListTicketsReq{
		Sns:      req.Sns,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}

	return convertListTicketResp(v2Resp), nil
}

// GetApproveNodeResult implements itsm.ITSMService.
func (a *ITSMV2Adapter) GetApproveNodeResult(ctx context.Context, req api.GetApproveNodeResultReq) (
	*api.GetApproveNodeResultDetail, error) {
	return GetApproveNodeResult(ctx, req.TicketID, req.StateID)
}

// TicketDetail implements itsm.ITSMService.
// v2版本对应的方法是 GetTicketStatus
func (a *ITSMV2Adapter) TicketDetail(ctx context.Context, req api.TicketDetailReq) (*api.Ticket, error) {
	return nil, fmt.Errorf("TicketDetail is not supported in v2 adapter")
}
