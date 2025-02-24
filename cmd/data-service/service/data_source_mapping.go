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
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbbase "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
	pbdsm "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/data-source-mapping"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// ListDataSourceTable get data source tables
func (s *Service) ListDataSourceTable(ctx context.Context, req *pbds.ListDataSourceTableReq) (
	*pbds.ListDataSourceTableResp, error) {
	kit := kit.FromGrpcContext(ctx)

	items, count, err := s.dao.DataSourceMapping().List(kit, req.BizId, req.SearchValue, &types.BasePage{
		Start: req.Start,
		Limit: uint(req.Limit),
		All:   req.GetAll(),
	})
	if err != nil {
		return nil, err
	}

	citations := map[uint32]uint32{}
	// 查询被引用的服务
	for _, v := range items {
		_, count, err := s.dao.Kv().ListRelatedConfigItemsWithTableType(kit, v.ID, &types.BasePage{All: true})
		if err != nil {
			return nil, err
		}
		citations[v.ID] = uint32(count)
	}
	return &pbds.ListDataSourceTableResp{
		Count:   uint32(count),
		Details: pbdsm.PbDataSourceMappings(items, citations),
	}, nil
}

// CreateDataSourceTable create data source table
func (s *Service) CreateDataSourceTable(ctx context.Context, req *pbds.CreateDataSourceTableReq) (
	*pbds.CreateDataSourceTableResp, error) {
	kit := kit.FromGrpcContext(ctx)

	// 检测业务下是否存在该表名
	exist, err := s.dao.DataSourceMapping().
		GetDataSourceMappingByTableName(kit, req.BizId, req.Spec.TableName)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "get the data source table by table name failed, err: %v", err))
	}
	if exist != nil && exist.ID != 0 {
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "table name %s already exists", req.Spec.TableName))
	}

	id, err := s.dao.DataSourceMapping().Create(kit, &table.DataSourceMapping{
		Attachment: &table.DataSourceMappingAttachment{
			BizID:            req.GetBizId(),
			DataSourceInfoID: 0,
		},
		Spec: req.Spec.DataSourceMappingSpec(),
		Revision: &table.Revision{
			Creator:   kit.User,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	})
	if err != nil {
		return nil, err
	}

	return &pbds.CreateDataSourceTableResp{Id: id}, nil
}

// GetDataSourceTable 获取数据源表格
func (s *Service) GetDataSourceTable(ctx context.Context, req *pbds.GetDataSourceTableReq) (
	*pbds.GetDataSourceTableResp, error) {
	kit := kit.FromGrpcContext(ctx)

	item, err := s.dao.DataSourceMapping().GetDataSourceMappingByID(kit, req.DataSourceMappingId)
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "get the data source table failed, err: %v", err))
	}

	return &pbds.GetDataSourceTableResp{Details: pbdsm.PbDataSourceMapping(item, 0)}, nil
}

// UpdateDataSourceTable implements pbds.DataServer.
func (s *Service) UpdateDataSourceTable(ctx context.Context, req *pbds.UpdateDataSourceTableReq) (
	*pbbase.EmptyResp, error) {
	kit := kit.FromGrpcContext(ctx)

	err := s.dao.DataSourceMapping().Update(kit, &table.DataSourceMapping{
		ID: req.DataSourceMappingId,
		Attachment: &table.DataSourceMappingAttachment{
			BizID:            req.BizId,
			DataSourceInfoID: 0,
		},
		Spec: req.Spec.DataSourceMappingSpec(),
		Revision: &table.Revision{
			Reviser:   kit.User,
			UpdatedAt: time.Now(),
		},
	})
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "update data source table failed, err: %v", err))
	}

	return &pbbase.EmptyResp{}, nil
}

// DeleteDataSourceTable implements pbds.DataServer.
func (s *Service) DeleteDataSourceTable(ctx context.Context, req *pbds.DeleteDataSourceTableReq) (
	*pbbase.EmptyResp, error) {
	kit := kit.FromGrpcContext(ctx)

	err := s.dao.DataSourceMapping().Delete(kit, &table.DataSourceMapping{
		ID: req.DataSourceMappingId,
		Attachment: &table.DataSourceMappingAttachment{
			BizID:            req.BizId,
			DataSourceInfoID: 0,
		},
	})
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "delete data source table failed, err: %v", err))
	}

	return &pbbase.EmptyResp{}, nil
}

// CreateTableStructAndContent 同时创建表结构和表数据
// 1. 检测创建表结构的条件
// 2. 创建表结构
// 3. 检测创建表数据的条件
// 4. 创建表数据
func (s *Service) CreateTableStructAndContent(ctx context.Context, req *pbds.CreateTableStructAndContentReq) (
	*pbds.CreateTableStructAndContentResp, error) {
	kit := kit.FromGrpcContext(ctx)
	// 检测业务下是否存在该表名
	exist, err := s.dao.DataSourceMapping().
		GetDataSourceMappingByTableName(kit, req.BizId, req.Spec.TableName)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "get the data source table by table name failed, err: %v", err))
	}
	if exist != nil && exist.ID != 0 {
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "table name %s already exists", req.Spec.TableName))
	}

	// 开启事务
	tx := s.dao.GenQuery().Begin()

	id, err := s.dao.DataSourceMapping().CreateWithTx(kit, tx, &table.DataSourceMapping{
		Attachment: &table.DataSourceMappingAttachment{
			BizID:            req.GetBizId(),
			DataSourceInfoID: 0,
		},
		Spec: req.Spec.DataSourceMappingSpec(),
		Revision: &table.Revision{
			Creator:   kit.User,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	})
	if err != nil {
		logs.Errorf("create table struct failed, err: %v, rid: %s", err, kit.Rid)
		if rErr := tx.Rollback(); rErr != nil {
			logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kit.Rid)
		}
		return nil, err
	}

	tableStruct, err := s.dao.DataSourceMapping().GetTableStructWithTx(kit, tx, id)
	if err != nil {
		logs.Errorf("get table struct failed, err: %v, rid: %s", err, kit.Rid)
		if rErr := tx.Rollback(); rErr != nil {
			logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kit.Rid)
		}
		return nil, err
	}

	createData := make([]*table.DataSourceContent, 0)
	for _, v := range req.GetContents() {
		createData = append(createData, &table.DataSourceContent{
			Attachment: &table.DataSourceContentAttachment{DataSourceMappingID: id},
			Spec: &table.DataSourceContentSpec{
				Content: v.AsMap(),
				Status:  table.KvStateAdd.String(),
			},
			Revision: &table.Revision{
				Creator:   kit.User,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			},
		})
	}

	if err = handleAutoIncrement(kit, tableStruct.Spec.Columns_, createData); err != nil {
		return nil, err
	}

	if err = s.verifyTableData(kit, tableStruct.Spec.Columns_, createData); err != nil {
		return nil, err
	}

	if err := s.dao.DataSourceContent().BatchCreateWithTx(kit, tx, createData); err != nil {
		logs.Errorf("create table content failed, err: %v, rid: %s", err, kit.Rid)
		if rErr := tx.Rollback(); rErr != nil {
			logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kit.Rid)
		}
		return nil, err
	}

	if e := tx.Commit(); e != nil {
		logs.Errorf("commit transaction failed, err: %v, rid: %s", e, kit.Rid)
		return nil, e
	}

	return &pbds.CreateTableStructAndContentResp{DataSourceMappingId: id}, nil
}

// UpdateTableStructAndContent implements pbds.DataServer.
func (s *Service) UpdateTableStructAndContent(ctx context.Context, req *pbds.UpdateTableStructAndContentReq) (
	*pbbase.EmptyResp, error) {
	kit := kit.FromGrpcContext(ctx)

	// 开启事务
	tx := s.dao.GenQuery().Begin()
	err := s.dao.DataSourceMapping().UpdateWithTx(kit, tx, &table.DataSourceMapping{
		ID: req.DataSourceMappingId,
		Attachment: &table.DataSourceMappingAttachment{
			BizID:            req.BizId,
			DataSourceInfoID: 0,
		},
		Spec: req.Spec.DataSourceMappingSpec(),
		Revision: &table.Revision{
			Reviser:   kit.User,
			UpdatedAt: time.Now(),
		},
	})
	if err != nil {
		logs.Errorf("update data source table failed, err: %v, rid: %s", err, kit.Rid)
		if rErr := tx.Rollback(); rErr != nil {
			logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kit.Rid)
		}
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "update data source table failed, err: %v", err))
	}

	// 获取更改过后的表结构
	tableStruct, err := s.dao.DataSourceMapping().GetTableStructWithTx(kit, tx, req.GetDataSourceMappingId())
	if err != nil {
		logs.Errorf("get table struct failed, err: %v, rid: %s", err, kit.Rid)
		if rErr := tx.Rollback(); rErr != nil {
			logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kit.Rid)
		}
		return nil, err
	}

	// 获取表数据
	contents, _, err := s.dao.DataSourceContent().ListByDataSourceMappingID(kit, req.GetDataSourceMappingId(),
		&types.BasePage{
			All: true,
		})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logs.Errorf("get the data list according to the data dource mapping id failed, err: %v, rid: %s", err, kit.Rid)
		if rErr := tx.Rollback(); rErr != nil {
			logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kit.Rid)
		}
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "get the data list according to the data dource mapping id failed, err: %v", err))
	}

	// 判断是否清空表数据
	if req.ReplaceAll {
		reallyDeleteIds, fakeDeleteIds := []uint32{}, []uint32{}
		// 获取需要真删还是假删的数据
		for _, v := range contents {
			if v.Spec.Status == table.KvStateAdd.String() {
				reallyDeleteIds = append(reallyDeleteIds, v.ID)
			} else {
				fakeDeleteIds = append(fakeDeleteIds, v.ID)
			}
		}
		err := s.dao.DataSourceContent().BatchFakeDeleteWithTx(kit, tx,
			req.DataSourceMappingId, fakeDeleteIds)
		if err != nil {
			logs.Errorf("batch fake delete table content failed, err: %v, rid: %s", err, kit.Rid)
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kit.Rid)
			}
			return nil, errf.Errorf(errf.DBOpFailed,
				i18n.T(kit, "batch fake delete table content failed, err: %v", err))
		}

		err = s.dao.DataSourceContent().BatchReallyDeleteWithTx(kit, tx,
			req.DataSourceMappingId, reallyDeleteIds)
		if err != nil {
			logs.Errorf("batch really delete table content failed, err: %v, rid: %s", err, kit.Rid)
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kit.Rid)
			}
			return nil, errf.Errorf(errf.DBOpFailed,
				i18n.T(kit, "batch really delete table content failed, err: %v", err))
		}
	}

	toCreate, toUpdate, _, err := s.handleTableContent(kit, tableStruct, contents, req.GetContents())
	if err != nil {
		logs.Errorf("batch really delete table content failed, err: %v, rid: %s", err, kit.Rid)
		if rErr := tx.Rollback(); rErr != nil {
			logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kit.Rid)
		}
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "batch really delete table content failed, err: %v", err))
	}

	if len(toCreate) != 0 {
		if err := s.dao.DataSourceContent().BatchCreateWithTx(kit, tx, toCreate); err != nil {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kit.Rid)
			}
			return nil, err
		}
	}

	if len(toUpdate) != 0 {
		if err := s.dao.DataSourceContent().BatchUpdateWithTx(kit, tx, toUpdate); err != nil {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kit.Rid)
			}
			return nil, err
		}
	}

	if e := tx.Commit(); e != nil {
		logs.Errorf("commit transaction failed, err: %v, rid: %s", e, kit.Rid)
		return nil, e
	}

	return &pbbase.EmptyResp{}, nil
}
