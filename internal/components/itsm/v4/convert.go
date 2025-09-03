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
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
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
func convertCreateTicketResp(ctx context.Context, activityKey string, resp *CreateTicketResp) (*api.CreateTicketData, error) {
	// v4 需要获取 任务ID和前端的url
	taskIDs, ticketURL, err := getTaskIDsAndURL(ctx, resp.Data.ID, activityKey)
	if err != nil {
		return nil, err
	}
	return &api.CreateTicketData{
		SN:        resp.Data.ID,
		StateID:   taskIDs,
		TicketURL: ticketURL,
	}, nil
}

func getTaskIDsAndURL(ctx context.Context, ticketSN, activityKey string) (string, string, error) {
	ticker := time.NewTicker(1 * time.Second) // 优化为1秒重试
	defer ticker.Stop()
	// 加上超时时间30s
	timeout := time.After(30 * time.Second)
	for {
		select {
		case <-ticker.C:
			detail, err := TicketDetail(ctx, TicketDetailReq{
				ID: ticketSN,
			})
			if err != nil {
				logs.Errorf("get ticket detail failed, err: %v", err)
				continue
			}
			taskIDs := map[string]bool{}
			// 判断是否存在 activeKey
			for _, step := range detail.CurrentSteps {
				if step.ActivityKey == activityKey {
					taskIDs[step.TaskID] = true
				}
			}
			// 如果taskIDs中都没有值 则继续轮询，有可能没有走到这一步
			if len(taskIDs) == 0 {
				logs.Warnf("getTaskIDsAndURL no activeKey found, will retry, ticket id: %s", ticketSN)
				continue
			}
			// 任务ID+审批人形式保存
			res := map[string]string{}

			for _, processor := range detail.CurrentProcessors {
				if _, ok := taskIDs[processor.TaskID]; ok {
					// 处理人-> 任务ID
					res[processor.Processor] = processor.TaskID
				}
			}
			data, err := json.Marshal(res)
			if err != nil {
				logs.Errorf("marshal approval tasks failed, err: %v, will retry", err)
				continue
			}
			return string(data), detail.FrontendURL, nil
		case <-timeout:
			return "", "", fmt.Errorf("getTaskIDsAndURL timeout after 30s")
		}
	}
}

func convertApprovalTicketReq(req api.ApprovalTicketReq) ApprovalTicketReq {
	action := "refuse"
	if req.Action == "true" {
		action = "approve"
	}
	return ApprovalTicketReq{
		TicketID:     req.TicketID,
		TaskID:       req.TaskID,
		Operator:     req.Operator,
		OperatorType: "user",
		SystemID:     req.SystemID,
		Action:       action,
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
