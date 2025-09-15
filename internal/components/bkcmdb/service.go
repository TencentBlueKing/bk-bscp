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
	"github.com/prometheus/client_golang/prometheus"

	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/bklogin"
	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/client"
	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/cmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

// Service xxx
type Service interface {
	Cmdb() cmdb.Client
	BKLogin() bklogin.Client
}

// NewBkClient xxx
func NewBkClient(cfg *cc.CMDBConfig, esbCfg *cc.Esb, reg prometheus.Registerer) (Service, error) {
	if cfg.UseEsb {
		return client.NewClient(esbCfg, reg)
	}

	return newBkCmdbClient(cfg.AppCode, cfg.AppSecret, cfg.Host)
}
