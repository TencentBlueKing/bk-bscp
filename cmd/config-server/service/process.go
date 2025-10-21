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

// OperateProcess implements pbcs.ConfigServer.
func (s *Service) OperateProcess(ctx context.Context, req *pbcs.OperateProcessReq) (*pbcs.OperateProcessResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.OperateProcess(grpcKit.RpcCtx(), &pbds.OperateProcessReq{
		BizId:       req.GetBizId(),
		ProcessId:   req.GetProcessId(),
		OperateType: req.GetOperateType(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.OperateProcessResp{
		BatchID: resp.GetBatchID(),
	}, nil
}

// ListProcess implements pbcs.ConfigServer.
func (s *Service) ListProcess(ctx context.Context, req *pbcs.ListProcessReq) (*pbcs.ListProcessResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.ListProcess(grpcKit.RpcCtx(), &pbds.ListProcessReq{
		BizId: req.GetBizId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ListProcessResp{
		Count:   resp.Count,
		Process: resp.GetProcess(),
	}, nil
}
