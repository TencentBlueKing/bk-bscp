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

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// ListDataSourceTable get data source tables
func (s *Service) ListDataSourceTable(ctx context.Context, req *pbcs.ListDataSourceTableReq) (
	*pbcs.ListDataSourceTableResp, error) {
	kit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Credential, Action: meta.Manage}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(kit, res...)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.DS.ListDataSourceTable(kit.RpcCtx(), &pbds.ListDataSourceTableReq{
		BizId:          req.BizId,
		SearchValue:    req.SearchValue,
		Start:          req.Start,
		Limit:          req.Limit,
		All:            req.All,
		DataSourceType: req.DataSourceType,
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ListDataSourceTableResp{
		Count:   resp.GetCount(),
		Details: resp.GetDetails(),
	}, nil
}

// CreateDataSourceTable create data source table
func (s *Service) CreateDataSourceTable(ctx context.Context, req *pbcs.CreateDataSourceTableReq) (
	*pbcs.CreateDataSourceTableResp, error) {
	kit := kit.FromGrpcContext(ctx)
	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Credential, Action: meta.Manage}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(kit, res...)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.DS.CreateDataSourceTable(kit.RpcCtx(), &pbds.CreateDataSourceTableReq{
		BizId: req.GetBizId(),
		Spec:  req.GetSpec(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.CreateDataSourceTableResp{Id: resp.Id}, nil
}

func (s *Service) GetDataSourceTable(ctx context.Context, req *pbcs.GetDataSourceTableReq) (
	*pbcs.GetDataSourceTableResp, error) {
	kit := kit.FromGrpcContext(ctx)
	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Credential, Action: meta.Manage}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(kit, res...)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.DS.GetDataSourceTable(kit.RpcCtx(), &pbds.GetDataSourceTableReq{
		BizId:               req.BizId,
		DataSourceMappingId: req.DataSourceMappingId,
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.GetDataSourceTableResp{Details: resp.Details}, nil
}

// UpdateDataSourceTable 编辑托管表格数据源
func (s *Service) UpdateDataSourceTable(ctx context.Context, req *pbcs.UpdateDataSourceTableReq) (
	*pbcs.UpdateDataSourceTableResp, error) {
	kit := kit.FromGrpcContext(ctx)
	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Credential, Action: meta.Manage}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(kit, res...)
	if err != nil {
		return nil, err
	}

	_, err = s.client.DS.UpdateDataSourceTable(kit.RpcCtx(), &pbds.UpdateDataSourceTableReq{
		BizId:               req.GetBizId(),
		Spec:                req.GetSpec(),
		DataSourceMappingId: req.GetDataSourceMappingId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.UpdateDataSourceTableResp{}, nil
}

// DeleteDataSourceTable 删除托管表格数据源
func (s *Service) DeleteDataSourceTable(ctx context.Context, req *pbcs.DeleteDataSourceTableReq) (
	*pbcs.DeleteDataSourceTableResp, error) {
	kit := kit.FromGrpcContext(ctx)
	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Credential, Action: meta.Manage}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(kit, res...)
	if err != nil {
		return nil, err
	}

	_, err = s.client.DS.DeleteDataSourceTable(kit.RpcCtx(), &pbds.DeleteDataSourceTableReq{
		BizId:               req.GetBizId(),
		DataSourceMappingId: req.GetDataSourceMappingId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.DeleteDataSourceTableResp{}, nil
}
