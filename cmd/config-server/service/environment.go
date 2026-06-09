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

// CreateEnvironment implements [pbcs.ConfigServer].
func (s *Service) CreateEnvironment(ctx context.Context, req *pbcs.CreateEnvironmentReq) (*pbcs.CreateEnvironmentResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.CreateEnvironment(grpcKit.RpcCtx(), &pbds.CreateEnvironmentReq{
		BizId:     req.GetBizId(),
		ProjectId: req.GetProjectId(),
		Name:      req.GetName(),
		Type:      req.GetType(),
		Memo:      req.GetMemo(),
		Protected: req.GetProtected(),
	})

	if err != nil {
		return nil, err
	}

	return &pbcs.CreateEnvironmentResp{
		Id: resp.GetId(),
	}, nil
}

// DeleteEnvironment implements [pbcs.ConfigServer].
func (s *Service) DeleteEnvironment(ctx context.Context, req *pbcs.DeleteEnvironmentReq) (*pbcs.DeleteEnvironmentResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	_, err := s.client.DS.DeleteEnvironment(grpcKit.RpcCtx(), &pbds.DeleteEnvironmentReq{
		BizId:     req.GetBizId(),
		ProjectId: req.GetProjectId(),
		EnvId:     req.GetEnvId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.DeleteEnvironmentResp{}, nil
}

// GetEnvironment implements [pbcs.ConfigServer].
func (s *Service) GetEnvironment(ctx context.Context, req *pbcs.GetEnvironmentReq) (*pbcs.GetEnvironmentResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.GetEnvironment(grpcKit.RpcCtx(), &pbds.GetEnvironmentReq{
		BizId:     req.GetBizId(),
		ProjectId: req.GetProjectId(),
		EnvId:     req.GetEnvId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.GetEnvironmentResp{
		Id:         resp.GetId(),
		Spec:       resp.GetSpec(),
		Attachment: resp.GetAttachment(),
	}, nil
}

// ListEnvironments implements [pbcs.ConfigServer].
func (s *Service) ListEnvironments(ctx context.Context, req *pbcs.ListEnvironmentsReq) (*pbcs.ListEnvironmentsResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.ListEnvironments(grpcKit.RpcCtx(), &pbds.ListEnvironmentsReq{
		BizId:           req.GetBizId(),
		ProjectId:       req.GetProjectId(),
		Start:           req.GetStart(),
		Limit:           req.GetLimit(),
		All:             req.GetAll(),
		SearchCondition: req.GetSearchCondition(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ListEnvironmentsResp{
		Environments: resp.GetEnvironments(),
		Count:        resp.GetCount(),
	}, nil
}

// UpdateEnvironment implements [pbcs.ConfigServer].
func (s *Service) UpdateEnvironment(ctx context.Context, req *pbcs.UpdateEnvironmentReq) (*pbcs.UpdateEnvironmentResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	_, err := s.client.DS.UpdateEnvironment(grpcKit.RpcCtx(), &pbds.UpdateEnvironmentReq{
		BizId:     req.GetBizId(),
		ProjectId: req.GetProjectId(),
		EnvId:     req.GetEnvId(),
		Memo:      req.GetMemo(),
		Protected: req.GetProtected(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.UpdateEnvironmentResp{}, nil
}
