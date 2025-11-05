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
	"fmt"

	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// SyncCmdbGseStatus implements pbcs.ConfigServer.
func (s *Service) SyncCmdbGseStatus(ctx context.Context, req *pbcs.SyncCmdbGseStatusReq) (
	*pbcs.SyncCmdbGseStatusResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	// 判断是否已经存在同步，存在直接返回任务ID或者其他信息
	status, err := s.CmdbGseStatus(grpcKit.RpcCtx(), &pbcs.CmdbGseStatusReq{
		BizId: req.GetBizId(),
	})
	if err != nil {
		return nil, err
	}

	if status.GetStatus() == types.TaskStatusInit ||
		status.GetStatus() == types.TaskStatusRunning ||
		status.GetStatus() == types.TaskStatusRevoked {
		return nil, fmt.Errorf("the task already exists. please try again later. current status: %s",
			status.GetStatus())
	}

	resp, err := s.client.DS.SyncCmdbGseStatus(grpcKit.RpcCtx(), &pbds.SyncCmdbGseStatusReq{
		BizId: req.GetBizId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.SyncCmdbGseStatusResp{
		TaskId: resp.GetTaskId(),
	}, nil
}

// CmdbGseStatus implements pbcs.ConfigServer.
func (s *Service) CmdbGseStatus(ctx context.Context, req *pbcs.CmdbGseStatusReq) (*pbcs.CmdbGseStatusResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.CmdbGseStatus(grpcKit.RpcCtx(), &pbds.CmdbGseStatusReq{
		BizId: req.GetBizId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.CmdbGseStatusResp{
		LastSyncTime: resp.GetLastSyncTime(),
		Status:       resp.GetStatus(),
	}, nil
}
