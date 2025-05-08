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

package bkpaas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/TencentBlueKing/bk-bscp/internal/components"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

type userInfo struct {
	Username string `json:"username"`
}

// bkPaaSAuthClient 外部统一登入, 可使用主域名或者ESB查询
type bkPaaSAuthClient struct {
	conf *cc.LoginAuthSettings
}

// GetLoginCredentialFromCookies 从 cookie 获取 LoginCredential
func (b *bkPaaSAuthClient) GetLoginCredentialFromCookies(r *http.Request) (*LoginCredential, error) {
	token, err := r.Cookie("bk_token")
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return nil, fmt.Errorf("%s cookie not present", "bk_token")
		}
		return nil, err
	}

	return &LoginCredential{UID: "", Token: token.Value}, nil
}

// GetUserInfoByToken BK_PAAS 服务 bk_token 鉴权
func (b *bkPaaSAuthClient) GetUserInfoByToken(ctx context.Context, host, uid, token string) (string, error) {
	url := fmt.Sprintf("%s/login/accounts/is_login/", host)
	resp, err := components.GetClient().R().
		SetContext(ctx).
		SetQueryParam("bk_token", token).
		Get(url)

	if err != nil {
		return "", err
	}

	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("http code %d != 200, body: %s", resp.StatusCode(), resp.Body())
	}

	user := new(userInfo)
	if err := components.UnmarshalBKResult(resp, user); err != nil {
		return "", err
	}

	return user.Username, nil
}

// BuildLoginRedirectURL 登入跳转URL
func (b *bkPaaSAuthClient) BuildLoginRedirectURL(r *http.Request, webHost string) string {
	redirectURL := fmt.Sprintf("%s/login/?c_url=%s", b.conf.Host, url.QueryEscape(buildAbsoluteUri(webHost, r)))
	return redirectURL
}

// BuildLoginURL API未登入访问URL
func (b *bkPaaSAuthClient) BuildLoginURL(r *http.Request) (string, string) {
	loginURL := fmt.Sprintf("%s/login/?c_url=", b.conf.Host)
	loginPlainURL := fmt.Sprintf("%s/login/plain/?c_url=", b.conf.Host)
	return loginURL, loginPlainURL
}

// VerifyToken 校验token
func (b *bkPaaSAuthClient) GetTenantUserInfoByToken(ctx context.Context, uid, token string) (*TenantUserInfo, error) {
	u, err := url.Parse(b.conf.Host)
	if err != nil {
		return nil, fmt.Errorf("parse host: %w", err)
	}
	// 使用网关域名
	url := fmt.Sprintf("%s://bkapi.%s/api/bk-login/prod/login/api/v3/open/bk-tokens/verify/", u.Scheme, u.Host)

	authHeader := components.MakeBKAPIGWAuthHeader(cc.AuthServer().Esb.AppCode, cc.AuthServer().Esb.AppSecret)

	resp, err := components.GetClient().R().
		SetContext(ctx).
		SetQueryParam("bk_token", token).
		SetHeader("X-Bkapi-Authorization", authHeader).
		SetHeader("X-Bk-Tenant-Id", "default").
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
