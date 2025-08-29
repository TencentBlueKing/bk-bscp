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

	"github.com/TencentBlueKing/bk-bscp/internal/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

var (
	workflowPath = "/api/v1/workflows"
)

// ListWorkflow xxx
func ListWorkflow(ctx context.Context, req ListWorkflowReq) (map[string]string, error) {
	itsmConf := cc.DataService().ITSM
	// 默认使用网关访问，如果为外部版，则使用ESB访问
	host := itsmConf.GatewayHost
	if itsmConf.External {
		host = itsmConf.Host
	}
	reqURL := fmt.Sprintf("%s%s?workflow_keys=%s", host, workflowPath, req.WorkflowKeys)

	// 请求API
	body, err := ItsmRequest(ctx, http.MethodGet, reqURL, req)
	if err != nil {
		logs.Errorf("request itsm list workflows failed, %s", err.Error())
		return nil, fmt.Errorf("request itsm list workflows failed, %s", err.Error())
	}
	// 解析返回的body
	resp := &ListWorkflowResp{}
	if err := json.Unmarshal(body, resp); err != nil {
		logs.Errorf("parse itsm body error, body: %s", string(body))
		return nil, err
	}
	if !resp.Result {
		logs.Errorf("request itsm list workflows %v failed, msg: %s", req, resp.Message)
		return nil, errors.New(resp.Message)
	}
	result := make(map[string]string)
	// 遍历 items -> activities，找到 type == "APPROVE_TASK" 的 key
	for _, item := range resp.Data.Items {
		for _, v := range item.Activities {
			if v.Type == "APPROVE_TASK" && v.Name == "或签审批" {
				result[constant.ItsmApproveOrSignType] = v.Key
			}
			if v.Type == "APPROVE_TASK" && v.Name == "会签审批" {
				result[constant.ItsmApproveCountSignType] = v.Key
			}
		}
	}

	return result, nil
}
