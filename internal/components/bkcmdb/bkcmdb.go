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
	"encoding/json"
	"fmt"

	"github.com/go-resty/resty/v2"

	"github.com/TencentBlueKing/bk-bscp/internal/components"
	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/bklogin"
	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/client"
	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/cmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/types"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

var (
	searchBusiness = "/api/c/compapi/v2/cc/search_business/"
)

// newBkCmdbClient 返回一个 bkcmdb 的 client。
// 由于当前实现不会出错，error 始终为 nil，函数签名仅用于兼容其他版本。
// nolint: unparam
func newBkCmdbClient(appCode, appSecret, host string) (client.Client, error) {
	return &bkCli{
		cc:         newClient(appCode, appSecret, host),
		bkloginCli: nil,
	}, nil
}

type bkCli struct {
	cc         cmdb.Client
	bkloginCli bklogin.Client
}

// Cmdb NOTES
func (e *bkCli) Cmdb() cmdb.Client {
	return e.cc
}

// BKLogin NOTES
func (e *bkCli) BKLogin() bklogin.Client {
	return e.bkloginCli
}

// NewClient new bk cmdb client.
func newClient(appCode, appSecret, host string) cmdb.Client {
	return &cmdbCli{
		appCode:   appCode,
		appSecret: appSecret,
		host:      host,
		client:    components.GetClient(),
	}
}

type cmdbCli struct {
	appCode   string
	appSecret string
	host      string
	client    *resty.Client
}

// GeBusinessByID 读取单个biz
func (b *cmdbCli) GeBusinessByID(ctx context.Context, bizID uint32) (*cmdb.Biz, error) {
	return nil, fmt.Errorf("GeBusinessByID is not implemented")
}

// ListAllBusiness implements BKClient.
func (b *cmdbCli) ListAllBusiness(ctx context.Context) (*cmdb.SearchBizResult, error) {
	params := &cmdb.SearchBizParams{}
	bizRes, err := b.SearchBusiness(ctx, params)
	if err != nil {
		return nil, err
	}

	return &bizRes.SearchBizResult, nil
}

// SearchBusiness implements BKClient.
func (b *cmdbCli) SearchBusiness(ctx context.Context, params *cmdb.SearchBizParams) (*cmdb.SearchBizResp, error) {
	type searchBizParams struct {
		*types.CommParams
		*cmdb.SearchBizParams
	}

	req := &searchBizParams{
		CommParams: &types.CommParams{
			AppCode:   b.appCode,
			AppSecret: b.appSecret,
			UserName:  "admin",
		},
		SearchBizParams: params,
	}
	url := fmt.Sprintf("%s%s", b.host, searchBusiness)

	kit := kit.FromGrpcContext(ctx)
	requtst := b.client.R()
	if len(kit.TenantID) != 0 {
		requtst = requtst.SetHeader(constant.BkTenantID, kit.TenantID)
	}
	resp, err := requtst.SetBody(req).Post(url)
	if err != nil {
		return nil, err
	}

	bizList := &cmdb.SearchBizResp{}
	if err := json.Unmarshal(resp.Body(), bizList); err != nil {
		return nil, err
	}

	return bizList, nil
}
