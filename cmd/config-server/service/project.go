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

// CreateProject implements [pbcs.ConfigServer].
func (s *Service) CreateProject(ctx context.Context, req *pbcs.CreateProjectReq) (*pbcs.CreateProjectResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.CreateProject(grpcKit.RpcCtx(), &pbds.CreateProjectReq{
		BizId: req.GetBizId(),
		Name:  req.GetName(),
		Memo:  req.GetMemo(),
	})

	if err != nil {
		return nil, err
	}

	return &pbcs.CreateProjectResp{
		Id: resp.GetId(),
	}, nil
}

// DeleteProject implements [pbcs.ConfigServer].
func (s *Service) DeleteProject(ctx context.Context, req *pbcs.DeleteProjectReq) (*pbcs.DeleteProjectResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	_, err := s.client.DS.DeleteProject(grpcKit.RpcCtx(), &pbds.DeleteProjectReq{
		BizId:     req.GetBizId(),
		ProjectId: req.GetProjectId(),
	})

	if err != nil {
		return nil, err
	}

	return &pbcs.DeleteProjectResp{}, nil
}

// GetProject implements [pbcs.ConfigServer].
func (s *Service) GetProject(ctx context.Context, req *pbcs.GetProjectReq) (*pbcs.GetProjectResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.GetProject(grpcKit.RpcCtx(), &pbds.GetProjectReq{
		BizId:     req.GetBizId(),
		ProjectId: req.GetProjectId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.GetProjectResp{
		Id:         resp.GetId(),
		Spec:       resp.GetSpec(),
		Attachment: resp.GetAttachment(),
	}, nil
}

// ListProjects implements [pbcs.ConfigServer].
func (s *Service) ListProjects(ctx context.Context, req *pbcs.ListProjectsReq) (*pbcs.ListProjectsResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.ListProjects(grpcKit.RpcCtx(), &pbds.ListProjectsReq{
		BizId:           req.GetBizId(),
		Start:           req.GetStart(),
		Limit:           req.GetLimit(),
		All:             req.GetAll(),
		SearchCondition: req.GetSearchCondition(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ListProjectsResp{
		Count:    resp.GetCount(),
		Projects: resp.GetProjects(),
	}, nil
}

// UpdateProject implements [pbcs.ConfigServer].
func (s *Service) UpdateProject(ctx context.Context, req *pbcs.UpdateProjectReq) (*pbcs.UpdateProjectResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	_, err := s.client.DS.UpdateProject(grpcKit.RpcCtx(), &pbds.UpdateProjectReq{
		BizId:     req.GetBizId(),
		ProjectId: req.GetProjectId(),
		Memo:      req.GetMemo(),
		Name:      req.GetName(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.UpdateProjectResp{}, nil
}
