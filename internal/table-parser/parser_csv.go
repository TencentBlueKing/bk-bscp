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

package tableparser

import (
	"encoding/csv"
	"io"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// NewCsvImport 解析csv
func NewCsvImport() *csvImport {
	return &csvImport{}
}

type csvImport struct {
}

// Import 解析csv
func (c *csvImport) Import(kit *kit.Kit, r io.Reader) ([]*types.TableImportResp, error) {
	resp := make([]*types.TableImportResp, 0)
	// 创建 CSV 解析器
	reader := csv.NewReader(r)

	// 读取表头
	headers, err := reader.Read()
	if err != nil {
		return resp, err
	}
	// 初始化列信息
	columns := c.initializeColumns(headers)
	// 追踪唯一性
	uniqueCheck := make([]map[string]bool, len(headers))
	for i := range uniqueCheck {
		uniqueCheck[i] = make(map[string]bool)
	}

	// 解析行数据
	rowsData := make([]map[string]interface{}, 0)
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return resp, err
		}

		// 解析每一行数据
		rowData := make(map[string]interface{})
		for i, cell := range row {
			if i < len(headers) {
				rowData[headers[i]] = cell
				// 更新列信息
				updateColumnInfo(columns[i].Columns_, cell, uniqueCheck[i])
			}
		}
		rowsData = append(rowsData, rowData)
	}
	resp = append(resp, &types.TableImportResp{TableName: "", Columns: columns, Rows: rowsData})

	return resp, nil
}

func (e *csvImport) initializeColumns(headers []string) []*types.Columns_ {
	columns := make([]*types.Columns_, len(headers))
	for i, header := range headers {
		columns[i] = &types.Columns_{
			Columns_: &table.Columns_{
				Name:       header,
				Alias:      header,
				Length:     0,
				ColumnType: table.StringColumn,
				Primary:    false,
				NotNull:    false,
				Unique:     false,
			},
			Status: table.KvStateAdd.String(),
		}
	}
	return columns
}
