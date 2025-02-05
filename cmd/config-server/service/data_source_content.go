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

// CreateTableContent implements pbcs.ConfigServer.
func (s *Service) CreateTableContent(ctx context.Context, req *pbcs.CreateTableContentReq) (
	*pbcs.CreateTableContentResp, error) {
	kit := kit.FromGrpcContext(ctx)
	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Credential, Action: meta.Manage}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(kit, res...)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.DS.CreateTableContent(kit.RpcCtx(), &pbds.CreateTableContentReq{
		BizId:               req.BizId,
		DataSourceMappingId: req.DataSourceMappingId,
		Content:             req.GetContent(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.CreateTableContentResp{Ids: resp.GetIds()}, nil
}

// ListTableContent implements pbcs.ConfigServer.
func (s *Service) ListTableContent(ctx context.Context, req *pbcs.ListTableContentReq) (
	*pbcs.ListTableContentResp, error) {
	kit := kit.FromGrpcContext(ctx)
	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Credential, Action: meta.Manage}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(kit, res...)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.DS.ListTableContent(kit.RpcCtx(), &pbds.ListTableContentReq{
		BizId:               req.BizId,
		DataSourceMappingId: req.DataSourceMappingId,
		FilterCondition:     req.FilterCondition,

		Start:        req.Start,
		Limit:        req.Limit,
		All:          req.All,
		FilterFields: req.FilterFields,
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ListTableContentResp{
		Details: resp.GetDetails(),
		Fields:  resp.GetFields(),
		Count:   resp.GetCount(),
	}, nil
}

// UpsertTableContent implements pbcs.ConfigServer.
func (s *Service) UpsertTableContent(ctx context.Context, req *pbcs.UpsertTableContentReq) (
	*pbcs.UpsertTableContentResp, error) {
	kit := kit.FromGrpcContext(ctx)
	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Credential, Action: meta.Manage}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(kit, res...)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.DS.UpsertTableContent(kit.RpcCtx(), &pbds.UpsertTableContentReq{
		BizId:               req.BizId,
		DataSourceMappingId: req.DataSourceMappingId,
		Contents:            req.GetContents(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.UpsertTableContentResp{Ids: resp.GetIds()}, nil
}

// CheckTableField implements pbcs.ConfigServer.
func (s *Service) CheckTableField(ctx context.Context, req *pbcs.CheckTableFieldReq) (*pbcs.CheckTableFieldResp, error) {
	kit := kit.FromGrpcContext(ctx)
	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Credential, Action: meta.Manage}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(kit, res...)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.DS.CheckTableField(kit.RpcCtx(), &pbds.CheckTableFieldReq{
		BizId:               req.BizId,
		DataSourceMappingId: req.DataSourceMappingId,
		FieldName:           req.FieldName,
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.CheckTableFieldResp{Exist: resp.Exist}, nil
}

// // UpdateTableContentV2 implements pbcs.ConfigServer.
// func (s *Service) UpdateTableContentV2(ctx context.Context, req *pbcs.UpdateTableContentV2Req) (
// 	*pbcs.UpdateTableContentResp, error) {
// 	kit := kit.FromGrpcContext(ctx)
// 	res := []*meta.ResourceAttribute{
// 		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
// 		{Basic: meta.Basic{Type: meta.Credential, Action: meta.Manage}, BizID: req.BizId},
// 	}

// 	if err := s.authorizer.Authorize(kit, res...); err != nil {
// 		return nil, err
// 	}

// 	resp, err := s.client.DS.UpdateTableContentV2(kit.RpcCtx(), &pbds.UpdateTableContentV2Req{
// 		BizId:               req.BizId,
// 		DataSourceMappingId: req.DataSourceMappingId,
// 		Content:             req.GetContent(),
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &pbcs.UpdateTableContentResp{Ids: resp.GetIds()}, nil
// }
