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

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbtb "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/task_batch"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// ListTaskBatch implements pbds.DataServer.
func (s *Service) ListTaskBatch(ctx context.Context, req *pbds.ListTaskBatchReq) (*pbds.ListTaskBatchResp, error) {
	kt := kit.FromGrpcContext(ctx)

	opt := &types.BasePage{
		Start: req.Start,
		Limit: uint(req.Limit),
	}

	filter := &dao.TaskBatchListFilter{
		TaskObject: table.TaskObject(req.TaskObject),
		TaskAction: table.TaskAction(req.TaskAction),
		Status:     table.TaskBatchStatus(req.Status),
		Executor:   req.Executor,
	}
	res, count, err := s.dao.TaskBatch().List(kt, req.BizId, filter, opt)
	if err != nil {
		logs.Errorf("list task batch failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return &pbds.ListTaskBatchResp{
		Count: uint32(count),
		// 转换为 protobuf 格式
		List: pbtb.PbTaskBatches(res),
	}, nil
}
