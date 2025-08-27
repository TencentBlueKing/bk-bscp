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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

var (
	listTicketsPath          = "/itsm/get_tickets/"
	getTicketStatusPath      = "/itsm/get_ticket_status/"
	getApproveNodeResultPath = "/itsm/get_approve_node_result/"
	getTicketLogsPath        = "/itsm/get_ticket_logs/"
)

// ListTicketsReq xxx
type ListTicketsReq struct {
	Sns      []string `json:"sns"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
}

// ListTicketsResp itsm list tickets resp
type ListTicketsResp struct {
	CommonResp
	Data *ListTicketsData `json:"data"`
}

// ListTicketsData list tickets data
type ListTicketsData struct {
	Page      int            `json:"page"`
	TotalPage int            `json:"total_page"`
	Count     int            `json:"count"`
	Next      string         `json:"next"`
	Previous  string         `json:"previous"`
	Items     []*TicketsItem `json:"items"`
}

// TicketsItem ITSM list tickets item
type TicketsItem struct {
	ID            int    `json:"id"`
	SN            string `json:"sn"`
	Title         string `json:"title"`
	CatalogID     int    `json:"catalog_id"`
	ServiceID     int    `json:"service_id"`
	ServiceType   string `json:"service_type"`
	FlowID        int    `json:"flow_id"`
	CurrentStatus string `json:"current_status"`
	CommentID     string `json:"comment_id"`
	IsCommented   bool   `json:"is_commented"`
	UpdatedBy     string `json:"updated_by"`
	UpdateAt      string `json:"update_at"`
	EndAt         string `json:"ent_at"`
	Creator       string `json:"creator"`
	CreateAt      string `json:"creat_at"`
	BkBizID       int    `json:"bk_biz_id"`
	TicketURL     string `json:"ticket_url"`
}

// ListTickets list itsm tickets by sn list
func ListTickets(ctx context.Context, req ListTicketsReq) (*ListTicketsData, error) {
	itsmConf := cc.DataService().ITSM
	// 默认使用网关访问，如果为外部版，则使用ESB访问
	host := itsmConf.GatewayHost
	if itsmConf.External {
		host = itsmConf.Host
	}

	reqURL := fmt.Sprintf("%s%s?page=%d&page_size=%d", host, listTicketsPath, req.Page, req.PageSize)

	reqData := map[string]any{
		"sns": req.Sns,
	}

	body, err := ItsmRequest(ctx, http.MethodPost, reqURL, reqData)
	if err != nil {
		logs.Errorf("request list itsm tickets %v failed, %s", req.Sns, err.Error())
		return nil, fmt.Errorf("request list itsm tickets %v failed, %s", req.Sns, err.Error())
	}
	// 解析返回的body
	resp := &ListTicketsResp{}
	if err := json.Unmarshal(body, resp); err != nil {
		logs.Errorf("parse itsm body error, body: %v", body)
		return nil, err
	}
	if resp.Code != 0 {
		logs.Errorf("list itsm tickets %v failed, msg: %s", req.Sns, resp.Message)
		return nil, errors.New(resp.Message)
	}

	return resp.Data, nil
}

// GetTicketStatusData get ticket status
type GetTicketStatusData struct {
	CommonResp
	Data *api.GetTicketStatusDetail `json:"data"`
}

// GetTicketStatus get itsm ticket status by sn
func GetTicketStatus(ctx context.Context, sn string) (*api.GetTicketStatusDetail, error) {
	itsmConf := cc.DataService().ITSM
	// 默认使用网关访问，如果为外部版，则使用ESB访问
	host := itsmConf.GatewayHost
	if itsmConf.External {
		host = itsmConf.Host
	}
	reqURL := fmt.Sprintf("%s%s?sn=%s", host, getTicketStatusPath, sn)

	body, err := ItsmRequest(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		logs.Errorf("request get itsm ticket status %v failed, %s", sn, err.Error())
		return nil, fmt.Errorf("request get itsm ticket status %v failed, %s", sn, err.Error())
	}
	// 解析返回的body
	resp := GetTicketStatusData{}
	if err := json.Unmarshal(body, &resp); err != nil {
		logs.Errorf("parse itsm body error, body: %v", body)
		return nil, err
	}
	if resp.Code != 0 {
		logs.Errorf("get itsm ticket status %v failed, msg: %s", sn, resp.Message)
		return nil, errors.New(resp.Message)
	}

	return resp.Data, nil
}

// GetApproveNodeResultData get ticket approve node result
type GetApproveNodeResultData struct {
	CommonResp
	Data *api.GetApproveNodeResultDetail `json:"data"`
}

// GetApproveNodeResult get itsm ticket approve node by sn
func GetApproveNodeResult(ctx context.Context, sn string, stateID int) (*api.GetApproveNodeResultDetail, error) {
	itsmConf := cc.DataService().ITSM
	// 默认使用网关访问，如果为外部版，则使用ESB访问
	host := itsmConf.GatewayHost
	if itsmConf.External {
		host = itsmConf.Host
	}
	reqURL := fmt.Sprintf("%s%s?sn=%s&state_id=%d", host, getApproveNodeResultPath, sn, stateID)

	body, err := ItsmRequest(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		logs.Errorf("request get approve node result %v failed, %s", sn, err.Error())
		return nil, fmt.Errorf("request get approve node result %v failed, %s", sn, err.Error())
	}
	// 解析返回的body
	resp := GetApproveNodeResultData{}
	if err := json.Unmarshal(body, &resp); err != nil {
		logs.Errorf("parse itsm body error, body: %v", body)
		return nil, err
	}
	if resp.Code != 0 {
		logs.Errorf("get approve node result %v failed, msg: %s", sn, resp.Message)
		return nil, errors.New(resp.Message)
	}

	return resp.Data, nil
}

// GetTicketLogsData get ticket logs result
type GetTicketLogsData struct {
	CommonResp
	Data *GetTicketLogsDetail `json:"data"`
}

// GetTicketLogsDetail get ticket logs
type GetTicketLogsDetail struct {
	Logs []*TicketLogs `json:"logs"`
}

// TicketLogs ticket log
type TicketLogs struct {
	Operator string `json:"operator"`
	Message  string `json:"message"`
}

// GetTicketLogs get itsm ticket logs by sn
func GetTicketLogs(ctx context.Context, sn string) (*GetTicketLogsDetail, error) {
	itsmConf := cc.DataService().ITSM
	// 默认使用网关访问，如果为外部版，则使用ESB访问
	host := itsmConf.GatewayHost
	if itsmConf.External {
		host = itsmConf.Host
	}
	reqURL := fmt.Sprintf("%s%s?sn=%s", host, getTicketLogsPath, sn)

	body, err := ItsmRequest(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		logs.Errorf("request get ticket logs by pass %v failed, %s", sn, err.Error())
		return nil, fmt.Errorf("request get ticket logs by pass %v failed, %s", sn, err.Error())
	}
	// 解析返回的body
	resp := &GetTicketLogsData{}
	if err := json.Unmarshal(body, &resp); err != nil {
		logs.Errorf("parse itsm body error, body: %v", body)
		return nil, err
	}
	if resp.Code != 0 {
		logs.Errorf("get ticket logs %v failed, msg: %s", sn, resp.Message)
		return nil, errors.New(resp.Message)
	}

	return resp.Data, nil
}
