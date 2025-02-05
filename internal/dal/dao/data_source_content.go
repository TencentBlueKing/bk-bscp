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

// Package dao NOTES
package dao

import (
	"encoding/json"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/utils"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/runtime/selector"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
	"gorm.io/datatypes"
	rawgen "gorm.io/gen"
	"gorm.io/gen/field"
)

// DataSourceContent xxx
type DataSourceContent interface {
	List(kit *kit.Kit, dataSourceMappingID uint32, filterCondition *selector.Selector,
		filterFields []string, opt *types.BasePage) ([]*table.DataSourceContent, int64, error)
	// Create one data source content
	Create(kit *kit.Kit, data *table.DataSourceContent) (uint32, error)
	// BatchCreate multiple data source content
	BatchCreate(kit *kit.Kit, data []*table.DataSourceContent) error
	// BatchCreateWithTx multiple data source content with transaction.
	BatchCreateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.DataSourceContent) error
	// BatchCreateWithTx multiple data source content with transaction.
	BatchUpdateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.DataSourceContent) error
	// BatchDeleteWithTx batch configItem instances with transaction.
	BatchDeleteWithTx(kit *kit.Kit, tx *gen.QueryTx, dataSourceMappingID uint32, ids []uint32) error
	// ListByDataSourceMappingID 根据表结构ID获取数据列表
	ListByDataSourceMappingID(kit *kit.Kit, dataSourceMappingID uint32, opt *types.BasePage) (
		[]*table.DataSourceContent, int64, error)
	// ListByJsonField 根据某个json字段获取数据列表
	ListByJsonField(kit *kit.Kit, dataSourceMappingID uint32, fieldName string,
		fieldType table.ColumnType, opt *types.BasePage) ([]*table.DataSourceContent, int64, error)
}

var _ DataSourceContent = new(dataSourceContentDao)

type dataSourceContentDao struct {
	genQ                 *gen.Query
	dataSourceMappingDao DataSourceMapping
	idGen                IDGenInterface
	auditDao             AuditDao
}

// filterContentFields filters out specified fields from the Content.
func filterContentFields(result []*table.DataSourceContent, filterFields []string) {
	for _, item := range result {
		for _, field := range filterFields {
			delete(item.Spec.Content, field)
		}
	}
}

// List implements DataSourceContent.
func (dao *dataSourceContentDao) List(kit *kit.Kit, dataSourceMappingID uint32, filterCondition *selector.Selector,
	filterFields []string, opt *types.BasePage) ([]*table.DataSourceContent, int64, error) {
	m := dao.genQ.DataSourceContent
	q := dao.genQ.DataSourceContent.WithContext(kit.Ctx)
	q = q.Where(m.DataSourceMappingID.Eq(dataSourceMappingID))
	// 处理搜索条件
	var conds []rawgen.Condition
	var err error
	if filterCondition != nil {
		conds, err = dao.handlefilterCondition(kit, dataSourceMappingID, filterCondition)
		if err != nil {
			return nil, 0, err
		}
	}
	var result []*table.DataSourceContent
	var count int64

	if opt.All {
		result, err = q.Where(conds...).Find()
		if err != nil {
			return nil, 0, err
		}
		count = int64(len(result))
	} else {
		result, count, err = q.Where(conds...).FindByPage(opt.Offset(), opt.LimitInt())
		if err != nil {
			return nil, 0, err
		}
	}

	if len(filterFields) > 0 {
		filterContentFields(result, filterFields)
	}

	return result, count, nil
}

// 处理搜索条件
func (dao *dataSourceContentDao) handlefilterCondition(kit *kit.Kit, dataSourceMappingID uint32,
	filterCondition *selector.Selector) ([]rawgen.Condition, error) {
	var conds []rawgen.Condition
	m := dao.genQ.DataSourceContent

	fieldNames := []string{}
	for _, v := range filterCondition.LabelsAnd {
		fieldNames = append(fieldNames, v.Key)
	}

	columns, err := dao.dataSourceMappingDao.GetTableStructByMultipleFieldNames(kit, dataSourceMappingID, fieldNames)
	if err != nil {
		return nil, err
	}

	// 根据字段获取类型
	for _, v := range filterCondition.LabelsAnd {
		if columns[v.Key] != nil {

			field := m.Content
			switch columns[v.Key].ColumnType {
			case table.NumberColumn:
				// Add condition for number or string columns
				addCondition(&conds, field, v.Key, v.Value, v.Op)
			case table.StringColumn:
				// Add condition for Enum columns
				addCondition(&conds, field, v.Key, v.Value, v.Op)
			case table.EnumColumn:
				if strVal, ok := v.Value.(string); ok {
					var stringArray []string
					_ = json.Unmarshal([]byte(strVal), &stringArray)
					for _, value := range stringArray {
						conds = append(conds, rawgen.Cond(datatypes.JSONArrayQuery("content").Contains(value, v.Key))...)
					}
				}
			}

		}
	}

	return conds, nil
}

// Define a function to append the condition based on the operator and column type
func addCondition(conds *[]rawgen.Condition, field field.Expr, key string, value interface{}, op selector.Operator) {
	var cond string
	var args []interface{}

	// Build the condition based on operator
	switch op {
	case &selector.InOperator:
		cond = "JSON_UNQUOTE(JSON_EXTRACT(?, ?)) IN ?"
		args = append(args, utils.Field{Field: field}, "$."+key, value)
	case &selector.NotEqualOperator:
		cond = "JSON_UNQUOTE(JSON_EXTRACT(?, ?)) NOT IN ?"
		args = append(args, utils.Field{Field: field}, "$."+key, value)
	case &selector.EqualOperator:
		cond = "JSON_UNQUOTE(JSON_EXTRACT(?, ?)) = ?"
		args = append(args, utils.Field{Field: field}, "$."+key, value)
	case &selector.NotEqualOperator:
		cond = "JSON_UNQUOTE(JSON_EXTRACT(?, ?)) != ?"
		args = append(args, utils.Field{Field: field}, "$."+key, value)
	case &selector.GreaterThanOperator:
		cond = "JSON_UNQUOTE(JSON_EXTRACT(?, ?)) > ?"
		args = append(args, utils.Field{Field: field}, "$."+key, value)
	case &selector.GreaterThanEqualOperator:
		cond = "JSON_UNQUOTE(JSON_EXTRACT(?, ?)) >= ?"
		args = append(args, utils.Field{Field: field}, "$."+key, value)
	case &selector.LessThanOperator:
		cond = "JSON_UNQUOTE(JSON_EXTRACT(?, ?)) < ?"
		args = append(args, utils.Field{Field: field}, "$."+key, value)
	case &selector.LessThanEqualOperator:
		cond = "JSON_UNQUOTE(JSON_EXTRACT(?, ?)) <= ?"
		args = append(args, utils.Field{Field: field}, "$."+key, value)
	case &selector.RegexOperator:
		cond = "JSON_UNQUOTE(JSON_EXTRACT(?, ?)) REGEXP ?"
		args = append(args, utils.Field{Field: field}, "$."+key, value)
	case &selector.NotRegexOperator:
		cond = "JSON_UNQUOTE(JSON_EXTRACT(?, ?)) NOT REGEXP ?"
		args = append(args, utils.Field{Field: field}, "$."+key, value)
	default:
		return // Could handle additional operators in the future
	}

	// Append the condition to the list
	*conds = append(*conds, utils.RawCond(cond, args...))
}

// BatchDeleteWithTx implements DataSourceContent.
func (dao *dataSourceContentDao) BatchDeleteWithTx(kit *kit.Kit, tx *gen.QueryTx, dataSourceMappingID uint32, ids []uint32) error {
	m := dao.genQ.DataSourceContent
	q := tx.DataSourceContent.WithContext(kit.Ctx)
	_, err := q.Where(m.DataSourceMappingID.Eq(dataSourceMappingID), m.ID.In(ids...)).
		Update(m.Status, table.KvStateDelete)
	if err != nil {
		return err
	}

	return nil
}

// BatchCreateWithTx implements DataSourceContent.
func (dao *dataSourceContentDao) BatchCreateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.DataSourceContent) error {
	if len(data) == 0 {
		return nil
	}
	ids, err := dao.idGen.Batch(kit, table.DataSourceContentTable, len(data))
	if err != nil {
		return err
	}
	for i, item := range data {
		if err := item.ValidateCreate(kit); err != nil {
			return err
		}
		item.ID = ids[i]
	}
	if err := tx.DataSourceContent.WithContext(kit.Ctx).CreateInBatches(data, 500); err != nil {
		return err
	}

	return nil
}

// BatchUpdateWithTx implements DataSourceContent.
func (dao *dataSourceContentDao) BatchUpdateWithTx(kit *kit.Kit, tx *gen.QueryTx,
	data []*table.DataSourceContent) error {
	if len(data) == 0 {
		return nil
	}

	return tx.DataSourceContent.WithContext(kit.Ctx).Save(data...)
}

// ListByJsonField implements DataSourceContent.
func (dao *dataSourceContentDao) ListByJsonField(kit *kit.Kit, dataSourceMappingID uint32, fieldName string,
	fieldType table.ColumnType, opt *types.BasePage) ([]*table.DataSourceContent, int64, error) {
	m := dao.genQ.DataSourceContent
	q := dao.genQ.DataSourceContent.WithContext(kit.Ctx)
	q = q.Where(m.DataSourceMappingID.Eq(dataSourceMappingID))

	var conds []rawgen.Condition
	if len(fieldName) != 0 {
		switch fieldType {
		case table.StringColumn:

			conds = append(conds, rawgen.Cond(datatypes.JSONQuery(m.Content.ColumnName().String()).HasKey(fieldName))...)
			conds = append(conds, utils.RawCond("JSON_EXTRACT(?,?) != ''", utils.Field{
				Field: m.Content,
			}, "$."+fieldName))

		case table.EnumColumn:

		}
	}

	if opt.All {
		result, err := q.Where(conds...).Find()
		if err != nil {
			return nil, 0, err
		}
		return result, int64(len(result)), err
	}

	return q.Where(conds...).FindByPage(opt.Offset(), opt.LimitInt())
}

// BatchCreate implements DataSourceContent.
func (dao *dataSourceContentDao) BatchCreate(kit *kit.Kit, data []*table.DataSourceContent) error {
	if len(data) == 0 {
		return nil
	}

	ids, err := dao.idGen.Batch(kit, table.DataSourceContentTable, len(data))
	if err != nil {
		return err
	}
	for i, item := range data {
		if err := item.ValidateCreate(kit); err != nil {
			return err
		}
		item.ID = ids[i]
	}
	// 分批插入
	if err := dao.genQ.DataSourceContent.WithContext(kit.Ctx).
		CreateInBatches(data, 500); err != nil {
		return err
	}

	return nil
}

// ListByDataSourceMappingID implements DataSourceContent.
func (dao *dataSourceContentDao) ListByDataSourceMappingID(kit *kit.Kit, dataSourceMappingID uint32,
	opt *types.BasePage) ([]*table.DataSourceContent, int64, error) {
	m := dao.genQ.DataSourceContent
	q := dao.genQ.DataSourceContent.WithContext(kit.Ctx)

	q = q.Where(m.DataSourceMappingID.Eq(dataSourceMappingID))
	if opt.All {
		result, err := q.Find()
		if err != nil {
			return nil, 0, err
		}
		return result, int64(len(result)), err
	}

	return q.FindByPage(opt.Offset(), opt.LimitInt())
}

// Create one data source content
func (dao *dataSourceContentDao) Create(kit *kit.Kit, data *table.DataSourceContent) (uint32, error) {
	if err := data.ValidateCreate(kit); err != nil {
		return 0, err
	}

	id, err := dao.idGen.One(kit, table.Name(data.TableName()))
	if err != nil {
		return 0, errf.ErrDBOpsFailedF(kit).WithCause(err)
	}
	data.ID = id

	if err = dao.genQ.DataSourceContent.
		WithContext(kit.Ctx).Create(data); err != nil {
		return 0, err
	}

	return data.ID, nil
}
