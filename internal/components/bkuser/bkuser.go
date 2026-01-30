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

package bkuser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/TencentBlueKing/bk-bscp/internal/components"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

var (
	// tenantAdminCache 缓存多租户下的 bk_admin 用户信息
	tenantAdminCache = map[string]*UserInfo{}
)

// UserInfo 用户信息
type UserInfo struct {
	BkUsername  string `json:"bk_username"`
	LoginName   string `json:"login_name"`
	DispalyName string `json:"display_name"`
}

// TenantInfo 租户信息
type TenantInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// GetVirtualUsers 根据用户名列表获取虚拟用户信息
func GetVirtualUsers(ctx context.Context, usernames []string) ([]UserInfo, error) {
	url := fmt.Sprintf("%s/api/bk-user/prod/api/v3/open/tenant/virtual-users/-/lookup/", cc.G().Esb.APIGWHost())

	kit := kit.MustGetKit(ctx)
	authHeader := components.MakeBKAPIGWAuthHeader(cc.G().Esb.AppCode, cc.G().Esb.AppSecret)
	resp, err := components.GetClient().R().
		SetContext(ctx).
		SetHeader("X-Bkapi-Authorization", authHeader).
		SetHeader("X-Bk-Tenant-Id", kit.TenantID).
		SetQueryParam("lookup_field", "login_name").
		SetQueryParam("lookups", strings.Join(usernames, ",")).
		Get(url)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("http code %d != 200, body: %s", resp.StatusCode(), resp.Body())
	}

	info := new([]UserInfo)
	bkResult := &components.BKResult{Data: info}
	if err := json.Unmarshal(resp.Body(), bkResult); err != nil {
		return nil, err
	}

	return *info, nil
}

// GetTenantBKAdmin 获取多租户下的bk_admin用户信息, 固定缓存
func GetTenantBKAdmin(ctx context.Context) (*UserInfo, error) {
	kit := kit.MustGetKit(ctx)
	if kit.TenantID == "" {
		return nil, fmt.Errorf("tenant ID is empty")
	}

	if admin, ok := tenantAdminCache[kit.TenantID]; ok {
		return admin, nil
	}
	users, err := GetVirtualUsers(ctx, []string{"bk_admin"}) // 名称固定
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("bk_admin user not found in tenant %s", kit.TenantID)
	}
	tenantAdminCache[kit.TenantID] = &users[0]

	return tenantAdminCache[kit.TenantID], nil
}

// ListEnabledTenants 获取所有启用状态的租户列表
func ListEnabledTenants(ctx context.Context) ([]TenantInfo, error) {
	url := fmt.Sprintf("%s/api/bk-user/prod/api/v3/open/tenants/", cc.G().Esb.APIGWHost())

	authHeader := components.MakeBKAPIGWAuthHeader(cc.G().Esb.AppCode, cc.G().Esb.AppSecret)
	resp, err := components.GetClient().R().
		SetContext(ctx).
		SetHeader("X-Bkapi-Authorization", authHeader).
		SetHeader("X-Bk-Tenant-Id", constant.DefaultTenantID). // 使用 system 租户调用
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("request list tenants failed: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("http code %d != 200, body: %s", resp.StatusCode(), resp.Body())
	}

	allTenants := new([]TenantInfo)
	bkResult := &components.BKResult{Data: allTenants}
	if err := json.Unmarshal(resp.Body(), bkResult); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	// 只返回 status=enabled 的租户
	enabledTenants := make([]TenantInfo, 0)
	for _, t := range *allTenants {
		if t.Status == "enabled" {
			enabledTenants = append(enabledTenants, t)
		}
	}

	return enabledTenants, nil
}
