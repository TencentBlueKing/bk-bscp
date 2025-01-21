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

package service

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/TencentBlueKing/bk-bscp/internal/iam/auth"
	tableparser "github.com/TencentBlueKing/bk-bscp/internal/table-parser"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	"github.com/TencentBlueKing/bk-bscp/pkg/rest"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

type tableService struct {
	authorizer auth.Authorizer
	cfgClient  pbcs.ConfigClient
}

func newTableService(authorizer auth.Authorizer,
	cfgClient pbcs.ConfigClient) *tableService {
	s := &tableService{
		authorizer: authorizer,
		cfgClient:  cfgClient,
	}
	return s
}

func (m *tableService) Import(w http.ResponseWriter, r *http.Request) {
	kit := kit.MustGetKit(r.Context())
	// Ensure r.Body is closed after reading
	defer r.Body.Close()

	dataSourceMappingIdStr := chi.URLParam(r, "data_source_mapping_id")
	dataSourceMappingId, _ := strconv.Atoi(dataSourceMappingIdStr)

	format := chi.URLParam(r, "format")
	format, err := url.PathUnescape(format)
	if err != nil {
		_ = render.Render(w, r, rest.BadRequest(errors.New(i18n.T(kit, "invalid file name"))))
		return
	}

	var columns []table.Columns_
	var rows []map[string]interface{}
	var tableName string

	switch format {
	case "sql", "SQL":
		tableName, columns, rows, err = tableparser.NewSqlImport().Import(kit, r.Body)
		if err != nil {
			_ = render.Render(w, r, rest.BadRequest(errors.New(i18n.T(kit, "failed to parse SQL: %v", err))))
			return
		}
	case "xls", "xlsx":
		tableName, columns, rows, err = tableparser.NewExcelImport().Import(kit, r.Body)
		if err != nil {
			_ = render.Render(w, r, rest.BadRequest(errors.New(i18n.T(kit, "failed to parse SQL: %v", err))))
			return
		}
	case "csv":
	default:
		return
	}

	cols := make([]types.Columns_, 0)
	// 不是0表示已存在表结构和表数据，需要对比结构和数据
	if dataSourceMappingId == 0 {
		for _, v := range columns {
			cols = append(cols, types.Columns_{
				Columns_: v,
				Status:   table.KvStateAdd.String(),
			})
		}
	}

	_ = render.Render(w, r, rest.OKRender(&types.TableImportResp{
		TableName: tableName,
		Columns:   cols,
		Rows:      rows,
	}))
}
