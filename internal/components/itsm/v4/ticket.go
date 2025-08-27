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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

var (
	createTicketPath   = "/api/v1/ticket/create"
	listTicketPath     = "/api/v1/ticket/list"
	ticketDetailPath   = "/api/v1/ticket/detail"
	revokedTicketPath  = "/api/v1/tickets/revoked"
	approvalTicketPath = "/api/v1/handle_approval_node"
	ticketLogsPath     = "/api/v1/ticket/logs"
	approvalTasksPath  = "/api/v1/approval_tasks"

	systemCode = "bk_bscp"
)

// CreateTicket create itsm ticket
func CreateTicket(ctx context.Context, req CreateTicketReq) (*CreateTicketResp, error) {
	itsmConf := cc.DataService().ITSM
	// 默认使用网关访问，如果为外部版，则使用ESB访问
	host := itsmConf.GatewayHost
	if itsmConf.External {
		host = itsmConf.Host
	}
	reqURL := fmt.Sprintf("%s%s", host, createTicketPath)

	// 请求API
	body, err := ItsmRequest(ctx, http.MethodPost, reqURL, req)
	if err != nil {
		logs.Errorf("request itsm create ticket failed, %s", err.Error())
		return nil, fmt.Errorf("request itsm create ticket failed, %s", err.Error())
	}
	// 解析返回的body
	resp := &CreateTicketResp{}
	if err := json.Unmarshal(body, resp); err != nil {
		logs.Errorf("parse itsm body error, body: %v", body)
		return nil, err
	}
	if !resp.Result {
		logs.Errorf("request create itsm ticket %v failed, msg: %s", req, resp.Message)
		return nil, errors.New(resp.Message)
	}
	return resp, nil
}

// ApprovalTicket approval itsm ticket
func ApprovalTicket(ctx context.Context, req ApprovalTicketReq) error {
	itsmConf := cc.DataService().ITSM
	// 默认使用网关访问，如果为外部版，则使用ESB访问
	host := itsmConf.GatewayHost
	if itsmConf.External {
		host = itsmConf.Host
	}
	reqURL := fmt.Sprintf("%s%s", host, approvalTicketPath)

	// 请求API
	body, err := ItsmRequest(ctx, http.MethodPost, reqURL, req)
	if err != nil {
		logs.Errorf("request itsm approval ticket failed, %s", err.Error())
		return fmt.Errorf("request itsm approval ticket failed, %s", err.Error())
	}
	// 解析返回的body
	resp := &ApprovalTicketResp{}
	if err := json.Unmarshal(body, resp); err != nil {
		logs.Errorf("parse itsm body error, body: %v", body)
		return err
	}
	if !resp.Result {
		logs.Errorf("request create itsm ticket %v failed, msg: %s", req, resp.Message)
		return errors.New(resp.Message)
	}
	return nil
}

// RevokedTicket revoked itsm ticket
func RevokedTicket(ctx context.Context, req RevokedTicketReq) (*RevokedTicketResp, error) {
	itsmConf := cc.DataService().ITSM
	// 默认使用网关访问，如果为外部版，则使用ESB访问
	host := itsmConf.GatewayHost
	if itsmConf.External {
		host = itsmConf.Host
	}
	reqURL := fmt.Sprintf("%s%s", host, revokedTicketPath)

	// 请求API
	body, err := ItsmRequest(ctx, http.MethodPost, reqURL, req)
	if err != nil {
		logs.Errorf("request itsm revoke ticket failed, %s", err.Error())
		return nil, fmt.Errorf("request itsm revoke ticket failed, %s", err.Error())
	}
	// 解析返回的body
	resp := &RevokedTicketResp{}
	if err := json.Unmarshal(body, resp); err != nil {
		logs.Errorf("parse itsm body error, body: %v", body)
		return nil, err
	}
	if !resp.Result {
		logs.Errorf("request revoke itsm ticket %v failed, msg: %s", req, resp.Message)
		return nil, errors.New(resp.Message)
	}
	return resp, nil
}

// TicketDetail itsm ticket detail
func TicketDetail(ctx context.Context, req TicketDetailReq) (*api.Ticket, error) {
	itsmConf := cc.DataService().ITSM
	// 默认使用网关访问，如果为外部版，则使用ESB访问
	host := itsmConf.GatewayHost
	if itsmConf.External {
		host = itsmConf.Host
	}
	reqURL := fmt.Sprintf("%s%s?id=%s", host, ticketDetailPath, req.ID)

	// 请求API
	body, err := ItsmRequest(ctx, http.MethodGet, reqURL, req)
	if err != nil {
		logs.Errorf("request get itsm ticket %v detail failed, error: %s", req.ID, err.Error())
		return nil, fmt.Errorf("request get itsm ticket detail failed, %s", err.Error())
	}
	// 解析返回的body
	resp := &TicketDetailResp{}
	if err := json.Unmarshal(body, resp); err != nil {
		logs.Errorf("parse itsm body error, body: %v", body)
		return nil, err
	}
	if !resp.Result {
		logs.Errorf("request get itsm ticket %v detail failed, msg: %s", req.ID, resp.Message)
		return nil, errors.New(resp.Message)
	}
	return resp.Data, nil
}

// GetTicketLogs xxx
func GetTicketLogs(ctx context.Context, req TicketDetailReq) (*api.TicketLogsData, error) {
	itsmConf := cc.DataService().ITSM
	// 默认使用网关访问，如果为外部版，则使用ESB访问
	host := itsmConf.GatewayHost
	if itsmConf.External {
		host = itsmConf.Host
	}
	reqURL := fmt.Sprintf("%s%s?ticket_id=%s", host, ticketLogsPath, req.ID)

	// 请求API
	body, err := ItsmRequest(ctx, http.MethodGet, reqURL, req)
	if err != nil {
		logs.Errorf("request itsm get ticket logs failed, %s", err.Error())
		return nil, fmt.Errorf("request itsm get ticket logs failed, %s", err.Error())
	}
	// 解析返回的body
	resp := &GetTicketLogsResp{}
	if err := json.Unmarshal(body, resp); err != nil {
		logs.Errorf("parse itsm body error, body: %v", body)
		return nil, err
	}
	if !resp.Result {
		logs.Errorf("request get ticket logs %v failed, msg: %s", req, resp.Message)
		return nil, errors.New(resp.Message)
	}
	return resp.Data, nil
}

// ListTickets xxx
func ListTickets(ctx context.Context, req ListTicketsReq) (*api.ListTicketsData, error) {
	itsmConf := cc.DataService().ITSM
	// 默认使用网关访问，如果为外部版，则使用ESB访问
	host := itsmConf.GatewayHost
	if itsmConf.External {
		host = itsmConf.Host
	}
	reqURL := fmt.Sprintf("%s%s?page=%d&page_size=%d&id__in=%s", host, listTicketPath,
		req.Page, req.PageSize, req.IdIn)

	// 请求API
	body, err := ItsmRequest(ctx, http.MethodGet, reqURL, req)
	if err != nil {
		logs.Errorf("request itsm list tickets failed, %s", err.Error())
		return nil, fmt.Errorf("request itsm create ticket failed, %s", err.Error())
	}
	// 解析返回的body
	resp := &ListTicketResp{}
	if err := json.Unmarshal(body, resp); err != nil {
		logs.Errorf("parse itsm body error, body: %v", body)
		return nil, err
	}
	if !resp.Result {
		logs.Errorf("request itsm list tickets %v failed, msg: %s", req, resp.Message)
		return nil, errors.New(resp.Message)
	}

	return resp.Data, nil
}

// ApprovalTasks approval itsm tasks
func ApprovalTasks(ctx context.Context, req ApprovalTasksReq) (*api.TasksData, error) {
	itsmConf := cc.DataService().ITSM
	// 默认使用网关访问，如果为外部版，则使用ESB访问
	host := itsmConf.GatewayHost
	if itsmConf.External {
		host = itsmConf.Host
	}
	reqURL := fmt.Sprintf("%s%s", host, approvalTasksPath)

	// 请求API
	body, err := ItsmRequest(ctx, http.MethodPost, reqURL, req)
	if err != nil {
		logs.Errorf("request itsm approval tasks failed, %s", err.Error())
		return nil, fmt.Errorf("request itsm approval tasks failed, %s", err.Error())
	}
	// 解析返回的body
	resp := &ApprovalTasksResp{}
	if err := json.Unmarshal(body, resp); err != nil {
		logs.Errorf("parse itsm body error, body: %v", body)
		return nil, err
	}
	if !resp.Result {
		logs.Errorf("request create itsm tasks %v failed, msg: %s", req, resp.Message)
		return nil, errors.New(resp.Message)
	}
	return resp.Data, nil
}
