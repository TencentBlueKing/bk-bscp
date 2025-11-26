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

// BizTopo implements pbcs.ConfigServer.
func (s *Service) BizTopo(ctx context.Context, req *pbcs.BizTopoReq) (*pbcs.BizTopoResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.BizTopo(grpcKit.RpcCtx(), &pbds.BizTopoReq{
		BizId: req.GetBizId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.BizTopoResp{
		BizTopoNodes: resp.GetBizTopoNodes(),
	}, nil
}

// ServiceTemplate implements pbcs.ConfigServer.
func (s *Service) ServiceTemplate(ctx context.Context, req *pbcs.ServiceTemplateReq) (*pbcs.ServiceTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}
	resp, err := s.client.DS.ServiceTemplate(grpcKit.RpcCtx(), &pbds.ServiceTemplateReq{
		BizId: req.GetBizId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ServiceTemplateResp{
		ServiceTemplates: resp.GetServiceTemplates(),
	}, nil
}

// ProcessTemplate implements pbcs.ConfigServer.
func (s *Service) ProcessTemplate(ctx context.Context, req *pbcs.ProcessTemplateReq) (*pbcs.ProcessTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}
	resp, err := s.client.DS.ProcessTemplate(grpcKit.RpcCtx(), &pbds.ProcessTemplateReq{
		BizId:             req.GetBizId(),
		ServiceTemplateId: req.GetServiceTemplateId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ProcessTemplateResp{
		ProcessTemplates: resp.GetProcessTemplates(),
	}, nil
}
