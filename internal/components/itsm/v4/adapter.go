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
	"context"
	"fmt"

	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
)

// ITSMV4Adapter xxx
type ITSMV4Adapter struct{}

// ListWorkflow implements itsm.Service.
func (a *ITSMV4Adapter) ListWorkflow(ctx context.Context, req api.ListWorkflowReq) (map[string]string, error) {
	return ListWorkflow(ctx, ListWorkflowReq{
		WorkflowKeys: req.WorkflowKeys,
	})
}

// ApprovalTasks implements itsm.Service.
func (a *ITSMV4Adapter) ApprovalTasks(ctx context.Context, req api.ApprovalTasksReq) (*api.TasksData, error) {
	return ApprovalTasks(ctx, ApprovalTasksReq{
		TicketID:    req.TicketID,
		ActivityKey: req.ActivityKey,
	})
}

// CreateTicket implements itsm.ITSMService.
func (a *ITSMV4Adapter) CreateTicket(ctx context.Context, req api.CreateTicketReq) (*api.CreateTicketData, error) {
	v4Resp, err := CreateTicket(ctx, convertCreateTicketReq(req))
	if err != nil {
		return &api.CreateTicketData{}, err
	}
	return convertCreateTicketResp(v4Resp), nil
}

// GetTicketLogs implements itsm.ITSMService.
func (a *ITSMV4Adapter) GetTicketLogs(ctx context.Context, req api.GetTicketLogsReq) (*api.TicketLogsData, error) {
	return GetTicketLogs(ctx, TicketDetailReq{req.TicketID})
}

// TicketDetail implements itsm.ITSMService.
func (a *ITSMV4Adapter) TicketDetail(ctx context.Context, req api.TicketDetailReq) (*api.Ticket, error) {
	return TicketDetail(ctx, TicketDetailReq{ID: req.ID})
}

// ApprovalTicket implements itsm.ITSMService.
func (a *ITSMV4Adapter) ApprovalTicket(ctx context.Context, req api.ApprovalTicketReq) error {
	return ApprovalTicket(ctx, convertApprovalTicketReq(req))
}

// RevokedTicket implements itsm.ITSMService.
func (a *ITSMV4Adapter) RevokedTicket(ctx context.Context, req api.ApprovalTicketReq) (*api.RevokedTicketResp, error) {
	v4Resp, err := RevokedTicket(ctx, convertRevokedTicketReq(req))
	if err != nil {
		return &api.RevokedTicketResp{}, err
	}
	return convertRevokedTicketResp(v4Resp), nil
}

// ListTickets implements itsm.ITSMService.
func (a *ITSMV4Adapter) ListTickets(ctx context.Context, req api.ListTicketsReq) (*api.ListTicketsData, error) {
	return ListTickets(ctx, convertListTicketsReq(req))
}

// GetTicketStatus implements itsm.ITSMService.
// v4版本中该方法就是 TicketDetail
func (a *ITSMV4Adapter) GetTicketStatus(ctx context.Context, req api.GetTicketStatusReq) (*api.GetTicketStatusDetail, error) {
	return nil, fmt.Errorf("GetTicketStatus is not supported in v4 adapter")
}

// GetApproveNodeResult implements itsm.ITSMService.
// v4版本中该方法就是 TicketDetail
func (a *ITSMV4Adapter) GetApproveNodeResult(ctx context.Context, req api.GetApproveNodeResultReq) (
	*api.GetApproveNodeResultDetail, error) {
	return nil, fmt.Errorf("GetApproveNodeResult is not supported in v4 adapter")
}
