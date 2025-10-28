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
	"strconv"
	"strings"

	"k8s.io/utils/pointer"

	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
	"github.com/TencentBlueKing/bk-bscp/internal/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// ITSMV2Adapter xxx
type ITSMV2Adapter struct{}

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
	stateID, _ := strconv.Atoi(req.StateID)
	return GetApproveNodeResult(ctx, req.TicketID, stateID)
}

// TicketDetail implements itsm.ITSMService.
// v2版本对应的方法是 GetTicketStatus
func (a *ITSMV2Adapter) TicketDetail(ctx context.Context, req api.TicketDetailReq) (*api.Ticket, error) {
	return nil, fmt.Errorf("TicketDetail is not supported in v2 adapter")
}

// GetApproveResult 获取审批结果
func (a *ITSMV2Adapter) GetApproveResult(ctx context.Context, req api.GetApproveResultReq) (*api.ApproveResultData, error) {
	v2Resp, err := GetTicketLogs(ctx, req.TicketID)
	if err != nil {
		logs.Errorf("GetApproveResult failed, err: %v", err)
		return nil, err
	}

	res := &api.ApproveResultData{
		Result:      nil,
		Items:       []*api.ApproveResultDataItem{},
		PassUsers:   []string{},
		RejectUsers: []string{},
	}
	// 提取审批信息
	for _, v := range v2Resp.Logs {
		// 审批拒绝
		if strings.Contains(v.Message, constant.ItsmRejectedApproveResult) {
			// 审批拒绝，提取审批拒绝原因
			stateID, err := strconv.Atoi(req.StateID)
			if err != nil {
				logs.Errorf("GetApproveResult failed, err: %v", err)
				return nil, err
			}
			approveNodeResult, err := GetApproveNodeResult(ctx, req.TicketID, stateID)
			if err != nil {
				logs.Errorf("GetApproveResult failed, err: %v", err)
				return nil, err
			}
			res.Result = pointer.Bool(false)
			res.RejectUsers = append(res.RejectUsers, v.Operator)
			res.Reasons = append(res.Reasons, approveNodeResult.ApproveRemark)
			res.Items = append(res.Items, &api.ApproveResultDataItem{
				Result: pointer.Bool(false),
				Reason: approveNodeResult.ApproveRemark,
			})
		}
		// 审批通过
		if strings.Contains(v.Message, constant.ItsmPassedApproveResult) {
			// 审批通过，提取审批人
			res.Items = append(res.Items, &api.ApproveResultDataItem{
				Result:   pointer.Bool(true),
				Operator: v.Operator,
			})
			res.PassUsers = append(res.PassUsers, v.Operator)
		}
	}

	// 有记录，并且最终result没有记录，说明审批通过
	if len(res.Items) > 0 && res.Result == nil {
		res.Result = pointer.Bool(true)
	}

	return res, nil
}
