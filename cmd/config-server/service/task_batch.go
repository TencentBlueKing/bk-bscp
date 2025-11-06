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

// ListTaskBatch implements pbcs.ConfigServer.
func (s *Service) ListTaskBatch(ctx context.Context, req *pbcs.ListTaskBatchReq) (*pbcs.ListTaskBatchResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	var sortRule *pbds.SortRule
	if req.GetSort() != nil {
		sortRule = &pbds.SortRule{
			Field: req.GetSort().GetField(),
			Order: req.GetSort().GetOrder(),
		}
	}

	resp, err := s.client.DS.ListTaskBatch(grpcKit.RpcCtx(), &pbds.ListTaskBatchReq{
		BizId:          req.GetBizId(),
		TaskObject:     req.GetTaskObject(),
		Start:          req.GetStart(),
		Limit:          req.GetLimit(),
		TaskAction:     req.GetTaskAction(),
		Status:         req.GetStatus(),
		Executor:       req.GetExecutor(),
		Sort:           sortRule,
		TimeRangeStart: req.GetTimeRangeStart(),
		TimeRangeEnd:   req.GetTimeRangeEnd(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ListTaskBatchResp{
		Count:         resp.Count,
		List:          resp.GetList(),
		FilterOptions: resp.GetFilterOptions(),
	}, nil
}

// GetTaskBatchDetail implements pbcs.ConfigServer.
func (s *Service) GetTaskBatchDetail(
	ctx context.Context,
	req *pbcs.GetTaskBatchDetailReq,
) (*pbcs.GetTaskBatchDetailResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.GetTaskBatchDetail(grpcKit.RpcCtx(), &pbds.GetTaskBatchDetailReq{
		BizId:   req.GetBizId(),
		BatchId: req.GetBatchId(),
		Start:   req.GetStart(),
		Limit:   req.GetLimit(),
		Status:  req.GetStatus(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.GetTaskBatchDetailResp{
		Tasks:         resp.GetTasks(),
		Count:         resp.Count,
		Statistics:    resp.GetStatistics(),
		FilterOptions: resp.GetFilterOptions(),
	}, nil
}
