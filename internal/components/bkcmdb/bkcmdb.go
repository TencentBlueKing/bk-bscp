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

// Package bkcmdb provides bkcmdb client.
package bkcmdb

import (
	"context"
	"fmt"

	"github.com/TencentBlueKing/bk-bscp/internal/components"
	"github.com/TencentBlueKing/bk-bscp/internal/components/bkuser"
	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/cmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/types"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// Biz is cmdb biz info.
type Biz struct {
	BizID         int64  `json:"bk_biz_id"`
	BizName       string `json:"bk_biz_name"`
	BizMaintainer string `json:"bk_biz_maintainer"`
}

// SearchBusiness 组件化的函数
func SearchBusiness(ctx context.Context, params *cmdb.SearchBizParams) (*cmdb.SearchBizResult, error) {
	// bk_supplier_account 是无效参数, 占位用
	url := fmt.Sprintf("%s/api/bk-cmdb/prod/api/v3/biz/search/bk_supplier_account", cc.G().Esb.APIGWHost())

	// SearchBizParams is esb search cmdb business parameter.
	type esbSearchBizParams struct {
		*types.CommParams
		*cmdb.SearchBizParams
	}

	admin, err := bkuser.GetTenantBKAdmin(ctx)
	if err != nil {
		return nil, fmt.Errorf("get tenant admin failed: %w", err)
	}

	kit := kit.MustGetKit(ctx)
	req := &esbSearchBizParams{
		SearchBizParams: params,
	}

	authHeader := components.MakeBKAPIGWAuthHeader(
		cc.G().Esb.AppCode,
		cc.G().Esb.AppSecret,
		components.WithBkUsername(admin.BkUsername),
	)
	resp, err := components.GetClient().R().
		SetContext(ctx).
		SetHeader("X-Bkapi-Authorization", authHeader).
		SetHeader("X-Bk-Tenant-Id", kit.TenantID).
		SetBody(req).
		Post(url)

	if err != nil {
		return nil, err
	}

	bizRes := new(cmdb.SearchBizResult)
	if err := components.UnmarshalBKResult(resp, bizRes); err != nil {
		return nil, err
	}

	return bizRes, nil
}

// ListAllBusiness 获取所有业务列表
func ListAllBusiness(ctx context.Context) ([]cmdb.Biz, error) {
	params := &cmdb.SearchBizParams{}
	bizRes, err := SearchBusiness(ctx, params)
	if err != nil {
		return nil, err
	}

	return bizRes.Info, nil
}
