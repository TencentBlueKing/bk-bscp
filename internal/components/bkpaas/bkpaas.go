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

// Package bkpaas provides bkpaas auth client.
package bkpaas

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/TencentBlueKing/bk-bscp/internal/components"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

const (
	// BKLoginProvider 蓝鲸内部统一登入
	BKLoginProvider = "BK_LOGIN"
	// BKPaaSProvider 外部统一登入, 可使用主域名或者ESB查询
	BKPaaSProvider = "BK_PAAS"
)

// LoginCredential uid/token for grpc auth
type LoginCredential struct {
	UID   string
	Token string
}

// TenantUserInfo 用户信息
type TenantUserInfo struct {
	BkUsername string `json:"bk_username"`
	TenantID   string `json:"tenant_id"`
}

// AuthLoginClient 登入鉴权
type AuthLoginClient interface {
	GetLoginCredentialFromCookies(r *http.Request) (*LoginCredential, error)
	GetUserInfoByToken(ctx context.Context, host, uid, token string) (string, error)
	GetTenantUserInfoByToken(ctx context.Context, token string) (*TenantUserInfo, error)
	BuildLoginRedirectURL(r *http.Request, webHost string) string
	BuildLoginURL(r *http.Request) (string, string)
}

// NewAuthLoginClient init client
func NewAuthLoginClient(conf *cc.LoginAuthSettings) AuthLoginClient {
	if conf.Provider == BKLoginProvider {
		return &bkLoginAuthClient{conf: conf}
	}
	return &bkPaaSAuthClient{conf: conf}
}

// BuildAbsoluteUri
func buildAbsoluteUri(webHost string, r *http.Request) string {
	// fallback use request host
	if webHost == "" {
		webHost = "http://" + r.Host
	}

	return fmt.Sprintf("%s%s", webHost, r.RequestURI)
}

// getTenantUserInfoByToken 获取租户用户信息
func getTenantUserInfoByToken(ctx context.Context, token string) (*TenantUserInfo, error) {
	// 使用网关域名
	url := fmt.Sprintf("%s/api/bk-login/prod/login/api/v3/open/bk-tokens/verify/", cc.G().Esb.APIGWHost())

	authHeader := components.MakeBKAPIGWAuthHeader(cc.G().Esb.AppCode, cc.G().Esb.AppSecret)
	resp, err := components.GetClient().R().
		SetContext(ctx).
		SetQueryParam("bk_token", token).
		SetHeader("X-Bkapi-Authorization", authHeader).
		SetHeader("X-Bk-Tenant-Id", "default"). // 鉴权是没有租户信息, 使用默认租户
		Get(url)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("http code %d != 200, body: %s", resp.StatusCode(), resp.Body())
	}

	info := new(TenantUserInfo)
	bkResult := &components.BKResult{Data: info}
	if err := json.Unmarshal(resp.Body(), bkResult); err != nil {
		return nil, err
	}

	if info.BkUsername == "" {
		return nil, fmt.Errorf("bk_username not found in response: %s", resp.Body())
	}

	return info, nil
}
