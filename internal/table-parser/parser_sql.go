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

	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/mysql"
	"github.com/pingcap/tidb/pkg/parser/test_driver"
	parserTypes "github.com/pingcap/tidb/pkg/parser/types"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// NewSqlImport 解析sql
func NewSqlImport() *sqlImport {
	return &sqlImport{}
}

type sqlImport struct {
}

// Import 解析mysql
func (s *sqlImport) Import(kit *kit.Kit, r io.Reader) ([]*types.TableImportResp, error) {
	resp := make([]*types.TableImportResp, 0)

	sql, err := io.ReadAll(r)
	if err != nil {
		return resp, errors.New(i18n.T(kit, "read file failed, err: %v", err))
	}

	p := parser.New()
	stmtNode, _, err := p.ParseSQL(string(sql))
	if err != nil {
		return resp, errors.New(i18n.T(kit, "parses a query string to raw ast.StmtNode failed, err: %v", err))
	}

	columns := make(map[string][]*types.Columns_)
	rows := make(map[string][]map[string]interface{})
	colNames := []string{}

	// 解析 SQL 语句
	for _, stmt := range stmtNode {
		switch stmt := stmt.(type) {
		case *ast.CreateTableStmt:
			// 处理创建表语句
			tableName := stmt.Table.Name.String()
			colNames = processCreateTableStmt(columns, colNames, stmt, tableName)

		case *ast.InsertStmt:
			// 处理插入语句
			insertTableName := processInsertStmt(stmt)
			processInsertValues(stmt, insertTableName, rows, colNames)
		}
	}

	// 生成返回结果
	for tableName, columnList := range columns {
		resp = append(resp, &types.TableImportResp{
			TableName: tableName,
			Columns:   columnList,
			Rows:      rows[tableName],
		})
	}

	return resp, nil
}

// 处理 CreateTableStmt
func processCreateTableStmt(columns map[string][]*types.Columns_, colNames []string, stmt *ast.CreateTableStmt, tableName string) []string {
	for _, col := range stmt.Cols {
		var unique, primaryKey bool
		primaryKey = isPrimaryKey(col, stmt)
		unique = isUnique(col.Name.String(), stmt)
		if primaryKey {
			unique = primaryKey
		}
		columns[tableName] = append(columns[tableName], &types.Columns_{
			Columns_: &table.Columns_{
				Name:          col.Name.String(),
				Length:        col.Tp.GetFlen(),
				Primary:       primaryKey,
				ColumnType:    parseColumnType(col.Tp),
				NotNull:       isNotNull(col),
				DefaultValue:  getDefaultValue(col),
				Unique:        unique,
				AutoIncrement: isAutoIncrement(col),
				EnumValue:     getEnumValues(col),
			},
			Status: table.KvStateAdd.String(),
		})
		colNames = append(colNames, col.Name.String())
	}
	return colNames
}

// 处理 InsertStmt
func processInsertStmt(stmt *ast.InsertStmt) string {
	var insertTableName string
	if stmt.Table != nil && stmt.Table.TableRefs != nil {
		if tableRef, ok := stmt.Table.TableRefs.Left.(*ast.TableSource); ok {
			if tableInf, ok := tableRef.Source.(*ast.TableName); ok {
				insertTableName = tableInf.Name.String()
			}
		}
	}
	return insertTableName
}

// 处理插入数据
func processInsertValues(stmt *ast.InsertStmt, insertTableName string, rows map[string][]map[string]interface{},
	colNames []string) {
	for _, value := range stmt.Lists {
		row := make(map[string]interface{})
		for i, val := range value {
			if v, ok := val.(*test_driver.ValueExpr); ok {
				row[colNames[i]] = v.GetValue()
			}
		}
		rows[insertTableName] = append(rows[insertTableName], row)
	}
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
func parseColumnType(colType *parserTypes.FieldType) table.ColumnType {
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
