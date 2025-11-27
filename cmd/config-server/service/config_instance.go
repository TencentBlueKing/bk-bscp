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
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// ListConfigInstances implements pbcs.ConfigServer.
func (s *Service) ListConfigInstances(ctx context.Context, req *pbcs.ListConfigInstancesReq) (*pbcs.ListConfigInstancesResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	// 调用 data-service 获取配置实例列表
	dsResp, err := s.client.DS.ListConfigInstances(grpcKit.RpcCtx(), &pbds.ListConfigInstancesReq{
		BizId:                    req.GetBizId(),
		ConfigTemplateId:         req.GetConfigTemplateId(),
		ConfigTemplateVersionIds: req.GetConfigTemplateVersionIds(),
		Search:                   req.GetSearch(),
		Start:                    req.GetStart(),
		Limit:                    req.GetLimit(),
	})
	if err != nil {
		logs.Errorf("list config instances from data-service failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	return &pbcs.ListConfigInstancesResp{
		Count:           dsResp.GetCount(),
		ConfigInstances: dsResp.GetConfigInstances(),
		FilterOptions:   dsResp.GetFilterOptions(),
	}, nil
}

// CompareConfig implements pbcs.ConfigServer.
func (s *Service) CompareConfig(ctx context.Context, req *pbcs.CompareConfigReq) (*pbcs.CompareConfigResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	return &pbcs.CompareConfigResp{}, nil
}

// PushConfig implements pbcs.ConfigServer.
func (s *Service) PushConfig(ctx context.Context, req *pbcs.PushConfigReq) (*pbcs.PushConfigResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	return &pbcs.PushConfigResp{}, nil
}

// GetConfigRenderResult implements pbcs.ConfigServer.
func (s *Service) GetConfigRenderResult(ctx context.Context, req *pbcs.GetConfigRenderResultReq) (*pbcs.GetConfigRenderResultResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	return &pbcs.GetConfigRenderResultResp{}, nil
}
