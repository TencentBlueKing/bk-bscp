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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-resty/resty/v2"

	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	migrateItsm = "/bk-bscp/etc/itsm/system_bk_bscp.json"
	migratePath = "/api/v1/system/migrate/"

	fileName = "system_bk_bscp.json"
)

// MigrateResp resp
type MigrateResp struct {
	Result  bool        `json:"result"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    MigrateData `json:"data"`
}

// MigrateData data
type MigrateData struct {
	Message string `json:"message"`
}

// MigrateSystem xxx
func MigrateSystem(ctx context.Context, content []byte) error {
	kit := kit.FromGrpcContext(ctx)

	tenantID := ctx.Value(constant.BkTenantID)
	if tenantID != nil {
		kit.TenantID = tenantID.(string)
	}

	itsmConf := cc.DataService().ITSM

	// 默认使用网关访问，如果为外部版，则使用ESB访问
	host := itsmConf.GatewayHost
	if itsmConf.External {
		host = itsmConf.Host
	}

	reqURL := fmt.Sprintf("%s%s", host, migratePath)
	request := resty.New().SetDebug(true).R()
	request.SetHeaders(GetAuthHeader(ctx))
	request.SetMultipartFormData(map[string]string{
		"tenant_id": kit.TenantID,
	})

	request.SetMultipartField("file", fileName, "text/plain", bytes.NewReader(content))

	resp, err := request.Post(reqURL)

	if err != nil {
		return err
	}

	if resp.RawResponse.StatusCode < 200 || resp.RawResponse.StatusCode >= 300 {
		return fmt.Errorf("MigrateSystem api failed return statusCode: %d", resp.StatusCode())
	}

	// 解析返回的body
	migrateData := &MigrateResp{}
	if err := json.Unmarshal(resp.Body(), migrateData); err != nil {
		logs.Errorf("parse itsm body error, body: %v", string(resp.Body()))
		return err
	}

	if !migrateData.Result {
		logs.Errorf("request migrate itsm system %v failed, msg: %s", migrateData.Code, migrateData.Message)
		return errors.New(migrateData.Message)
	}

	return nil
}
