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
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/mysql"
	"github.com/pingcap/tidb/pkg/parser/test_driver"
	"github.com/pingcap/tidb/pkg/parser/types"
)

type sqlImport struct {
}

func NewSqlImport() *sqlImport {
	return &sqlImport{}
}

// 解析mysql
func (s *sqlImport) Import(kit *kit.Kit, r io.Reader) (string, []table.Columns_, []map[string]interface{}, error) {
	sql, err := io.ReadAll(r)
	if err != nil {
		return "", nil, nil, errors.New(i18n.T(kit, "read file failed, err: %v", err))
	}

	p := parser.New()
	stmtNode, _, err := p.ParseSQL(string(sql))
	if err != nil {
		return "", nil, nil, errors.New(i18n.T(kit, "parses a query string to raw ast.StmtNode failed, err: %v", err))
	}

	tableName := ""
	// 字段和行
	columns := make([]table.Columns_, 0)
	rows := make([]map[string]interface{}, 0)
	colNames := []string{}
	for _, stmt := range stmtNode {
		switch stmt := stmt.(type) {
		// 解析列名、数据类型等
		case *ast.CreateTableStmt:
			tableName = stmt.Table.Name.String()
			for _, col := range stmt.Cols {
				var unique, primaryKey bool
				primaryKey = isPrimaryKey(col, stmt)
				unique = isUnique(col.Name.String(), stmt)
				if primaryKey {
					unique = primaryKey
				}
				columns = append(columns, table.Columns_{
					Name:          col.Name.String(),
					Length:        col.Tp.GetFlen(),
					Primary:       primaryKey,
					ColumnType:    parseColumnType(col.Tp),
					NotNull:       isNotNull(col),
					DefaultValue:  getDefaultValue(col),
					Unique:        unique,
					AutoIncrement: isAutoIncrement(col),
					EnumValue:     getEnumValues(col),
				})
				colNames = append(colNames, col.Name.String())
			}
		// 解析 Insert 语法
		case *ast.InsertStmt:
			for _, value := range stmt.Lists {
				row := make(map[string]interface{})
				for i, val := range value {
					switch v := val.(type) {
					case *test_driver.ValueExpr:
						row[colNames[i]] = v.GetValue()
					}
				}
				rows = append(rows, row)
			}
		}
	}

	return tableName, columns, rows, nil
}

// isPrimaryKey checks if the column is a primary key.
func isPrimaryKey(col *ast.ColumnDef, stmt *ast.CreateTableStmt) bool {
	for _, constraint := range stmt.Constraints {
		if constraint.Tp == ast.ConstraintPrimaryKey {
			for _, column := range constraint.Keys {
				if column.Column.Name.L == col.Name.String() {
					return true
				}
			}
		}
	}
	return false
}

// isAutoIncrement checks if the column is auto-increment.
func isAutoIncrement(col *ast.ColumnDef) bool {
	for _, opt := range col.Options {
		if opt.Tp == ast.ColumnOptionAutoIncrement {
			return true
		}
	}
	return false
}

// getDefaultValue returns the default value of the column.
func getDefaultValue(col *ast.ColumnDef) string {
	for _, opt := range col.Options {
		if opt.Tp == ast.ColumnOptionDefaultValue {
			return toString(opt.Expr.(*test_driver.ValueExpr).GetValue())
		}
	}

	return ""
}

// isNotNull checks if the column is not null.
func isNotNull(col *ast.ColumnDef) bool {
	for _, opt := range col.Options {
		if opt.Tp == ast.ColumnOptionNotNull {
			return true
		}
	}

	return false
}

// isUnique checks if the column has a unique constraint.
func isUnique(colName string, stmt *ast.CreateTableStmt) bool {
	for _, constraint := range stmt.Constraints {
		if constraint.Tp == ast.ConstraintUniq {
			for _, column := range constraint.Keys {
				if column.Column.Name.L == colName {
					return true
				}
			}
		}
	}

	return false
}

// getEnumValues extracts the enum values for a column if it's an enum type.
func getEnumValues(col *ast.ColumnDef) string {
	if col.Tp.GetElems() == nil {
		return ""
	}
	enumValues, err := json.Marshal(col.Tp.GetElems())
	if err != nil {
		return ""
	}

	return string(enumValues)
}

// parseColumnType maps the column type based on the expression.
func parseColumnType(colType *types.FieldType) table.ColumnType {
	switch colType.GetType() {
	case mysql.TypeBit, mysql.TypeTiny, mysql.TypeShort, mysql.TypeLong, mysql.TypeFloat,
		mysql.TypeDouble, mysql.TypeLonglong, mysql.TypeInt24:
		return table.NumberColumn
	case mysql.TypeEnum:
		return table.EnumColumn
	default:
		return table.StringColumn
	}
}

func toString(value interface{}) string {
	if value == nil {
		return ""
	}

	return fmt.Sprintf("%v", value)
}
