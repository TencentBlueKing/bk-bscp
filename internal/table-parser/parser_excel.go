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
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// NewExcelImport 解析excel
func NewExcelImport() *excelImport {
	return &excelImport{}
}

type excelImport struct {
}

// Import 解析excel
func (e *excelImport) Import(kit *kit.Kit, r io.Reader) ([]*types.TableImportResp, error) {

	resp := make([]*types.TableImportResp, 0)

	f, err := excelize.OpenReader(r)
	if err != nil {
		return resp, errors.New(i18n.T(kit, "open Excel file failed %v", err))
	}
	defer f.Close()

	sheetList := f.GetSheetList()

	for _, sheetName := range sheetList {
		rowsData := make([]map[string]interface{}, 0)
		rows, err := f.Rows(sheetName)
		if err != nil {
			return resp, err
		}
		// 解析表头
		var headers []string
		if rows.Next() {
			headers, err = rows.Columns()
			if err != nil {
				return resp, err
			}
		}

		// 初始化列
		columns := initializeColumns(headers)

		rowIndex := 0
		uniqueCheck := make([]map[string]bool, len(headers))
		for i := range uniqueCheck {
			uniqueCheck[i] = make(map[string]bool)
		}

		// 逐行解析数据
		for rows.Next() {
			rowIndex++
			cells, err := rows.Columns()
			if err != nil {
				return resp, err
			}

			// 解析行数据
			rowData := make(map[string]interface{})
			for i, cell := range cells {
				if i < len(headers) {
					rowData[headers[i]] = cell
					// 更新列信息
					updateColumnInfo(columns[i].Columns_, cell, uniqueCheck[i])
				}
			}
			rowsData = append(rowsData, rowData)
		}

		resp = append(resp, &types.TableImportResp{TableName: sheetName, Columns: columns, Rows: rowsData})

	}

	return resp, nil
}

func initializeColumns(headers []string) []*types.Columns_ {
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

func updateColumnInfo(column *table.Columns_, value interface{}, uniqueMap map[string]bool) {
	// 更新 NotNull
	if value == "" {
		column.NotNull = false
		return
	}

	// 检查唯一性
	strValue := fmt.Sprintf("%v", value) // 转成字符串方便比较
	if uniqueMap[strValue] {
		column.Unique = false // 一旦重复，不再唯一
	} else {
		uniqueMap[strValue] = true
	}

	// 检测数据类型
	column.ColumnType = detectColumnType(value)
}

func detectColumnType(value interface{}) table.ColumnType {
	// 检查是否为 nil 或空值
	if value == nil {
		return "string" // 默认为字符串
	}

	// 转为字符串以便进一步解析
	strValue, ok := value.(string)
	if !ok {
		strValue = fmt.Sprintf("%v", value)
	}

	// 尝试检测数据类型
	if _, err := strconv.Atoi(strValue); err == nil {
		return table.NumberColumn // 整数类型
	}
	if _, err := strconv.ParseFloat(strValue, 64); err == nil {
		return table.StringColumn // 浮点类型
	}
	if strings.ToLower(strValue) == "true" || strings.ToLower(strValue) == "false" {
		return table.StringColumn // 布尔类型
	}

	// 默认返回字符串类型
	return table.StringColumn
}
