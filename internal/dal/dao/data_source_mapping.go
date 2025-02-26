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
	"fmt"

	"github.com/TencentBlueKing/bk-bscp/internal/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/enumor"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// DataSourceMapping xxx
type DataSourceMapping interface {
	// List get data source mapping
	List(kit *kit.Kit, bizID uint32, searchValue string, opt *types.BasePage) ([]*table.DataSourceMapping, int64, error)
	// Create one data source mapping
	Create(kit *kit.Kit, data *table.DataSourceMapping) (uint32, error)
	// GetDataSourceMappingByTableName 通过表名获取数据源表格
	GetDataSourceMappingByTableName(kit *kit.Kit, bizID uint32, tableName string) (*table.DataSourceMapping, error)
	// GetDataSourceMappingByID 通过主键获取数据源表格
	GetDataSourceMappingByID(kit *kit.Kit, id uint32) (*table.DataSourceMapping, error)
	// Update one data source mapping
	Update(kit *kit.Kit, data *table.DataSourceMapping) error
	// Delete one data source mapping
	Delete(kit *kit.Kit, data *table.DataSourceMapping) error
	// GetTableStructByMultipleFieldNames 通过多个字段名获取表结构
	GetTableStructByMultipleFieldNames(kit *kit.Kit, id uint32, fieldNames []string) (map[string]*table.Columns_, error)
	// ListByDataSourceInfoId 按数据源信息 ID 列出
	ListByDataSourceInfoId(kit *kit.Kit, bizID, dataSourceInfoID uint32) ([]*table.DataSourceMapping, error)
}

var _ DataSourceMapping = new(dataSourceMappingDao)

type dataSourceMappingDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
}

// ListByDataSourceInfoId 按数据源信息 ID 列出
func (dao *dataSourceMappingDao) ListByDataSourceInfoId(kit *kit.Kit, bizID uint32,
	dataSourceInfoID uint32) ([]*table.DataSourceMapping, error) {
	m := dao.genQ.DataSourceMapping

	return dao.genQ.DataSourceMapping.WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID), m.DataSourceInfoID.In(dataSourceInfoID)).
		Find()
}

// GetTableStructByMultipleFieldNames 通过多个字段名获取表结构
func (dao *dataSourceMappingDao) GetTableStructByMultipleFieldNames(kit *kit.Kit, id uint32, fieldNames []string) (
	map[string]*table.Columns_, error) {
	m := dao.genQ.DataSourceMapping
	q := dao.genQ.DataSourceMapping.WithContext(kit.Ctx)

	columns := make(map[string]*table.Columns_, 0)
	data, err := q.Where(m.ID.Eq(id)).Take()
	if err != nil {
		return nil, err
	}
	for _, fieldName := range fieldNames {
		for _, column := range data.Spec.Columns_ {
			if fieldName == column.Name {
				columns[fieldName] = column
			}
		}
	}

	return columns, nil
}

// Delete implements DataSourceMapping.
func (dao *dataSourceMappingDao) Delete(kit *kit.Kit, data *table.DataSourceMapping) error {
	// 参数校验
	if err := data.ValidateDelete(kit); err != nil {
		return err
	}

	// 删除操作, 获取当前记录做审计
	m := dao.genQ.DataSourceMapping
	q := dao.genQ.DataSourceMapping.WithContext(kit.Ctx)
	oldOne, err := q.Where(m.ID.Eq(data.ID), m.BizID.Eq(data.Attachment.BizID)).Take()
	if err != nil {
		return err
	}
	ad := dao.auditDao.Decorator(kit, data.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.DataSourceName, oldOne.Spec.TableName_),
		Status:           enumor.Success,
	}).PrepareDelete(oldOne)

	// 多个使用事务处理
	deleteTx := func(tx *gen.Query) error {
		q = tx.DataSourceMapping.WithContext(kit.Ctx)
		if _, err := q.Where(m.BizID.Eq(data.Attachment.BizID)).Delete(data); err != nil {
			return err
		}

		if err := ad.Do(tx); err != nil {
			return err
		}
		return nil
	}
	if err := dao.genQ.Transaction(deleteTx); err != nil {
		return err
	}

	return nil
}

// Update one data source mapping
func (dao *dataSourceMappingDao) Update(kit *kit.Kit, data *table.DataSourceMapping) error {
	if err := data.ValidateUpdate(kit); err != nil {
		return err
	}

	// 更新操作, 获取当前记录做审计
	m := dao.genQ.DataSourceMapping
	q := dao.genQ.DataSourceMapping.WithContext(kit.Ctx)
	oldOne, err := q.Where(m.ID.Eq(data.ID), m.BizID.Eq(data.Attachment.BizID)).Take()
	if err != nil {
		return err
	}
	ad := dao.auditDao.Decorator(kit, data.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.DataSourceName, oldOne.Spec.TableName_),
		Status:           enumor.Success,
	}).PrepareUpdate(data)

	// 多个使用事务处理
	updateTx := func(tx *gen.Query) error {
		q = tx.DataSourceMapping.WithContext(kit.Ctx)
		if _, err := q.Where(m.BizID.Eq(data.Attachment.BizID), m.ID.Eq(data.ID)).
			Updates(data); err != nil {
			return err
		}

		if err := ad.Do(tx); err != nil {
			return err
		}
		return nil
	}
	if err := dao.genQ.Transaction(updateTx); err != nil {
		return err
	}

	return nil
}

// GetDataSourceMappingByID implements DataSourceMapping.
func (dao *dataSourceMappingDao) GetDataSourceMappingByID(kit *kit.Kit, id uint32) (
	*table.DataSourceMapping, error) {
	m := dao.genQ.DataSourceMapping

	return dao.genQ.DataSourceMapping.
		WithContext(kit.Ctx).
		Where(m.ID.Eq(id)).
		Take()
}

// GetDataSourceMappingByTableName 通过表名获取数据源表格
func (dao *dataSourceMappingDao) GetDataSourceMappingByTableName(kit *kit.Kit, bizID uint32,
	tableName string) (*table.DataSourceMapping, error) {
	m := dao.genQ.DataSourceMapping

	return dao.genQ.DataSourceMapping.
		WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID), m.TableName_.Eq(tableName)).
		Take()
}

// Create one data source mapping
func (dao *dataSourceMappingDao) Create(kit *kit.Kit, data *table.DataSourceMapping) (uint32, error) {
	if err := data.ValidateCreate(kit); err != nil {
		return 0, err
	}

	id, err := dao.idGen.One(kit, table.Name(data.TableName()))
	if err != nil {
		return 0, errf.ErrDBOpsFailedF(kit).WithCause(err)
	}
	data.ID = id

	ad := dao.auditDao.Decorator(kit, data.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.DataSourceName, data.Spec.TableName_),
		Status:           enumor.Success,
	}).PrepareCreate(data)

	// 多个使用事务处理
	createTx := func(tx *gen.Query) error {
		if err := tx.DataSourceMapping.WithContext(kit.Ctx).Create(data); err != nil {
			return err
		}

		if err := ad.Do(tx); err != nil {
			return err
		}
		return nil
	}
	if err := dao.genQ.Transaction(createTx); err != nil {
		return 0, errf.ErrDBOpsFailedF(kit).WithCause(err)
	}

	return data.ID, nil
}

// List get data source mapping
func (dao *dataSourceMappingDao) List(kit *kit.Kit, bizID uint32, searchValue string, opt *types.BasePage) (
	[]*table.DataSourceMapping, int64, error) {
	m := dao.genQ.DataSourceMapping
	q := dao.genQ.DataSourceMapping.WithContext(kit.Ctx)
	// if searchValue != "" {

	// }
	q = q.Where(m.BizID.Eq(bizID))
	if opt.All {
		result, err := q.Find()
		if err != nil {
			return nil, 0, err
		}
		return result, int64(len(result)), err
	}

	return q.FindByPage(opt.Offset(), opt.LimitInt())
}
