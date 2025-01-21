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

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbbase "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
	pbdsm "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/data-source-mapping"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
	"gorm.io/gorm"
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
