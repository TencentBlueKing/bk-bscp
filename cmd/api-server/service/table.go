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
	"reflect"
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

// Import 表格文件导入
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

	var tables []*types.TableImportResp

	switch format {
	case "sql", "SQL":
		tables, err = tableparser.NewSqlImport().Import(kit, r.Body)
		if err != nil {
			_ = render.Render(w, r, rest.BadRequest(errors.New(i18n.T(kit, "failed to parse SQL: %v", err))))
			return
		}
	case "xls", "xlsx":
		tables, err = tableparser.NewExcelImport().Import(kit, r.Body)
		if err != nil {
			_ = render.Render(w, r, rest.BadRequest(errors.New(i18n.T(kit, "failed to parse SQL: %v", err))))
			return
		}
	case "csv":
		tables, err = tableparser.NewCsvImport().Import(kit, r.Body)
		if err != nil {
			_ = render.Render(w, r, rest.BadRequest(errors.New(i18n.T(kit, "failed to parse SQL: %v", err))))
			return
		}
	default:
		_ = render.Render(w, r, rest.BadRequest(errors.New(i18n.T(kit, "the imported file type is not currently supported"))))
		return
	}

	if dataSourceMappingId != 0 {
		resp, err := m.cfgClient.GetDataSourceTable(kit.RpcCtx(), &pbcs.GetDataSourceTableReq{
			BizId:               kit.BizID,
			DataSourceMappingId: uint32(dataSourceMappingId),
		})
		if err != nil {
			_ = render.Render(w, r, rest.BadRequest(err))
			return
		}

		// 由于proto生成pb文件时默认在json字段加上 omitempty，导致字段不一致，不能做对比
		// 所以统一转成某个结构体
		existsMap := map[string]*table.Columns_{}
		for _, column := range resp.GetDetails().GetSpec().GetColumns() {
			existsMap[column.Name] = column.DataSourceMappingColumn()
		}

		for _, tab := range tables {
			if tab.TableName != resp.GetDetails().Spec.TableName {
				continue
			}
			tab.IsChange = len(tab.Columns) != len(existsMap)
			for _, column := range tab.Columns {
				data, exists := existsMap[column.Name]
				if !exists {
					tab.IsChange = true
					column.Status = table.KvStateAdd.String()
					continue
				}
				col := &table.Columns_{
					Name:          column.Name,
					Alias:         column.Alias,
					Length:        column.Length,
					Primary:       column.Primary,
					ColumnType:    column.ColumnType,
					NotNull:       column.NotNull,
					DefaultValue:  column.DefaultValue,
					Unique:        column.Unique,
					ReadOnly:      column.ReadOnly,
					AutoIncrement: column.AutoIncrement,
					EnumValue:     column.EnumValue,
					Selected:      column.Selected,
				}
				if reflect.DeepEqual(data, col) {
					column.Status = table.KvStateUnchange.String()
				} else {
					tab.IsChange = true
					column.Status = table.KvStateRevise.String()
				}
				// 移除existsMap中已匹配的字段，剩下的就是删除字段
				delete(existsMap, column.Name)
			}
			// 3. 处理删除字段
			for _, deletedCol := range existsMap {
				tab.IsChange = true
				tab.Columns = append(tab.Columns, &types.Columns_{
					Columns_: deletedCol,
					Status:   table.KvStateDelete.String(),
				})
			}
		}
	}

	_ = render.Render(w, r, rest.OKRender(tables))
}
