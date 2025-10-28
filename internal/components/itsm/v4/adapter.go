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
	"strings"

	"k8s.io/utils/pointer"

	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
)

// ITSMV4Adapter xxx
type ITSMV4Adapter struct{}

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
	return convertCreateTicketResp(ctx, req.ActivityKey, v4Resp)
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
func (a *ITSMV4Adapter) GetTicketStatus(ctx context.Context, req api.GetTicketStatusReq) (*api.GetTicketStatusDetail, error) {
	// 从 TicketDetail 获取工单详情
	ticket, err := TicketDetail(ctx, TicketDetailReq{ID: req.TicketID})
	if err != nil {
		return nil, err
	}

	// 组装返回的状态信息
	statusDetail := &api.GetTicketStatusDetail{
		// 将 ticket.Status 映射为 CurrentStatus
		CurrentStatus: strings.ToUpper(ticket.Status),
		// 当前步骤信息可以根据需要添加
		CurrentSteps: []map[string]any{},
	}

	return statusDetail, nil
}

// GetApproveResult implements itsm.ITSMService.
func (a *ITSMV4Adapter) GetApproveResult(ctx context.Context, req api.GetApproveResultReq) (*api.ApproveResultData, error) {
	// 获取工单日志
	logs, err := GetTicketLogs(ctx, TicketDetailReq{ID: req.TicketID})
	if err != nil {
		return nil, err
	}
	res := &api.ApproveResultData{
		Result:      nil,
		Items:       []*api.ApproveResultDataItem{},
		RejectUsers: []string{},
		PassUsers:   []string{},
	}

	// 在日志中查找匹配  activity_key 的记录
	for _, item := range logs.Items {
		if item.ActivityKey == req.ActivityKey {
			// 找到匹配的记录，提取审批信息
			itemRes := &api.ApproveResultDataItem{
				Result:   pointer.Bool(item.Action == "approve"),
				Operator: item.Operator,
			}
			// 没有审批通过
			if !(*itemRes.Result) {
				res.Result = pointer.Bool(false)
				res.RejectUsers = append(res.RejectUsers, item.Operator)
			} else {
				res.PassUsers = append(res.PassUsers, item.Operator)
			}

			// 提取审批意见
			remark := extractRemarkFromExtra(item.Extra)
			if remark != "" {
				itemRes.Reason = remark
				res.Reasons = append(res.Reasons, remark)
			}
			res.Items = append(res.Items, itemRes)
		}
	}

	// 有记录，并且最终result没有记录，说明审批通过
	if len(res.Items) > 0 && res.Result == nil {
		res.Result = pointer.Bool(true)
	}

	return res, nil

}

// GetApproveNodeResult implements itsm.ITSMService.
// 从工单日志中提取指定节点的审批意见
func (a *ITSMV4Adapter) GetApproveNodeResult(ctx context.Context, req api.GetApproveNodeResultReq) (
	*api.GetApproveNodeResultDetail, error) {

	// 获取工单日志
	logs, err := GetTicketLogs(ctx, TicketDetailReq{ID: req.TicketID})
	if err != nil {
		return nil, err
	}

	// 在日志中查找匹配 stateID（即 activity_key）的记录
	for _, item := range logs.Items {
		if item.ActivityKey == req.StateID {
			// 找到匹配的记录，提取审批信息
			detail := &api.GetApproveNodeResultDetail{
				Name:          item.ActivityName,
				Processeduser: item.OperatorDisplay,
				// 根据 action 判断审批结果
				ApproveResult: item.Action == "approve",
			}

			// 提取审批意见，处理各种可能的格式
			remark := extractRemarkFromExtra(item.Extra)
			if remark != "" {
				detail.ApproveRemark = remark
			}

			return detail, nil
		}
	}

	// 没有找到匹配的记录
	return nil, fmt.Errorf("no approval record found for state ID: %s", req.StateID)
}

// extractRemarkFromExtra 从Extra字段中提取审批意见
func extractRemarkFromExtra(extras []any) string {
	for _, extraAny := range extras {
		// 尝试将 any 类型转换为 map[string]interface{}
		extraMap, ok := extraAny.(map[string]interface{})
		if !ok {
			continue
		}

		// 处理格式: {name: "审批意见", type: "name_value", value: "拒绝原因"}
		if extraType, _ := extraMap["type"].(string); extraType == "name_value" {
			if extraName, _ := extraMap["name"].(string); extraName == "审批意见" {
				if value, ok := extraMap["value"].(string); ok {
					return value
				}
			}
		}
	}
	return ""
}
