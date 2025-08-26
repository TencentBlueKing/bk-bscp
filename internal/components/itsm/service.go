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
package itsm

import (
	"context"

	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
	v2 "github.com/TencentBlueKing/bk-bscp/internal/components/itsm/v2"
	v4 "github.com/TencentBlueKing/bk-bscp/internal/components/itsm/v4"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

// NewITSMService new itsm service
func NewITSMService() Service {
	if cc.DataService().ITSM.EnableV4 {
		return &v4.ITSMV4Adapter{}
	}
	return &v2.ITSMV2Adapter{}
}

// Service xxx
type Service interface {
	CreateTicket(ctx context.Context, req api.CreateTicketReq) (*api.CreateTicketData, error)
	ApprovalTicket(ctx context.Context, req api.ApprovalTicketReq) error
	RevokedTicket(ctx context.Context, req api.ApprovalTicketReq) (*api.RevokedTicketResp, error)
	GetTicketStatus(ctx context.Context, req api.GetTicketStatusReq) (*api.GetTicketStatusDetail, error)
	GetTicketLogs(ctx context.Context, req api.GetTicketLogsReq) (*api.TicketLogsData, error)
	TicketDetail(ctx context.Context, req api.TicketDetailReq) (*api.Ticket, error)
	GetApproveNodeResult(ctx context.Context, req api.GetApproveNodeResultReq) (*api.GetApproveNodeResultDetail, error)
	ListTickets(ctx context.Context, req api.ListTicketsReq) (*api.ListTicketsData, error)
	ApprovalTasks(ctx context.Context, req api.ApprovalTasksReq) (*api.TasksData, error)
	ListWorkflow(ctx context.Context, req api.ListWorkflowReq) (map[string]string, error)
}
