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

package dao

import (
	"fmt"
	"reflect"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// 设置不需要 TenantID 的表
var excludedTables = map[string]struct{}{
	"clients":       {},
	"client_events": {},
}

// 注册回调
func registerCallbacks(db *gorm.DB) {
	_ = db.Callback().Create().Before("gorm:create").Register("set_tenant_id", beforeAnyOp)
	_ = db.Callback().Update().Before("gorm:update").Register("set_tenant_id", beforeAnyOp)
	_ = db.Callback().Delete().Before("gorm:delete").Register("set_tenant_id", beforeQuery)
	_ = db.Callback().Query().Before("gorm:query").Register("set_tenant_id", beforeQuery)
}

// 查询前置操作，有 TenantID 字段，就自动加条件
func beforeQuery(db *gorm.DB) {
	if _, excluded := excludedTables[db.Statement.Table]; excluded {
		return
	}

	kit := kit.FromGrpcContext(db.Statement.Context)
	if kit.TenantID == "" || db.Statement.Schema == nil {
		return
	}
	// 查找 TenantID 字段
	field := db.Statement.Schema.LookUpField("TenantID")
	if field == nil {
		return
	}

	var oldExprs []clause.Expression

	// 获取原来的 WHERE 表达式
	if c, ok := db.Statement.Clauses["WHERE"]; ok {
		if where, ok := c.Expression.(clause.Where); ok {
			oldExprs = where.Exprs
		}
	}

	// 加上主表表名限定（防止歧义）
	// 如 "apps" 或表别名
	tableName := db.Statement.Table
	qualifiedTenantCol := fmt.Sprintf("%s.tenant_id", tableName)

	// 构建新的 WHERE 表达式
	newWhere := clause.Where{
		Exprs: append([]clause.Expression{
			clause.Eq{Column: qualifiedTenantCol, Value: kit.TenantID},
		}, oldExprs...),
	}

	// 设置新的 WHERE 子句
	db.Statement.Clauses["WHERE"] = clause.Clause{
		Name:       "WHERE",
		Expression: newWhere,
	}
}

// 新增和编辑前置操作
func beforeAnyOp(db *gorm.DB) {
	if _, excluded := excludedTables[db.Statement.Table]; excluded {
		return
	}
	kit := kit.FromGrpcContext(db.Statement.Context)
	if kit.TenantID == "" || db.Statement.Schema == nil {
		return
	}
	rv := db.Statement.ReflectValue
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		for i := range rv.Len() {
			item := rv.Index(i)
			if item.Kind() == reflect.Ptr {
				item = item.Elem()
			}
			applyKitFields(db, item, kit.TenantID)
		}
	case reflect.Ptr:
		applyKitFields(db, rv.Elem(), kit.TenantID)
	case reflect.Struct:
		applyKitFields(db, rv, kit.TenantID)
	}
}

func applyKitFields(db *gorm.DB, rv reflect.Value, tenantId string) {
	schema := db.Statement.Schema
	if field := schema.LookUpField("TenantID"); field != nil {
		err := field.Set(db.Statement.Context, rv, tenantId)
		if err != nil {
			logs.Errorf("set tenant id failed, err: %v", err)
			return
		}
	}
}
