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

// Package gse provides gse api client.
package gse

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-resty/resty/v2"

	"github.com/TencentBlueKing/bk-bscp/internal/components"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

type HTTPMethod string

const (
	GET  HTTPMethod = "GET"
	POST HTTPMethod = "POST"
)

// Service xxx
type Service struct {
	appCode   string
	appSecret string
	host      string
}

// NewService new service
func NewService(appCode, appSecret, host string) *Service {
	return &Service{
		appCode:   appCode,
		appSecret: appSecret,
		host:      host,
	}
}

// nolint: unparam
func (gse *Service) doRequest(ctx context.Context, method HTTPMethod, url string, body any, result any) error {
	authHeader := components.MakeBKAPIGWAuthHeader(
		gse.appCode,
		gse.appSecret,
		components.WithBkUsername("admin"),
	)

	// 构造请求
	request := components.GetClient().R().
		SetContext(ctx).
		SetHeader("X-Bkapi-Authorization", authHeader).
		SetBody(body)

	// 执行请求
	var resp *resty.Response
	var err error

	switch method {
	case GET:
		resp, err = request.Get(url)
	case POST:
		resp, err = request.Post(url)
	default:
		return fmt.Errorf("%s method not defined", method)
	}

	if err != nil {
		return err
	}

	if err := json.Unmarshal(resp.Body(), result); err != nil {
		logs.Errorf("unmarshal bk result failed, err: %v", err)
		return err
	}

	return nil
}
