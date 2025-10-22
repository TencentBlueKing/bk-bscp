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

	// 转换为 protobuf 格式
	list := make([]*pbtb.TaskBatch, 0, len(res))
	for _, item := range res {
		detail := &pbtb.TaskBatch{
			Id:         item.ID,
			TaskObject: string(item.Spec.TaskObject),
			TaskAction: string(item.Spec.TaskAction),
			TaskData:   item.Spec.TaskData,
			Status:     string(item.Spec.Status),
		}

		if item.Spec.StartAt != nil {
			detail.StartAt = item.Spec.StartAt.Format("2006-01-02 15:04:05")
		}
		if item.Spec.EndAt != nil {
			detail.EndAt = item.Spec.EndAt.Format("2006-01-02 15:04:05")
		}
		if item.Revision != nil {
			detail.CreatedAt = item.Revision.CreatedAt.Format("2006-01-02 15:04:05")
			detail.UpdatedAt = item.Revision.UpdatedAt.Format("2006-01-02 15:04:05")
		}

		list = append(list, detail)
	}

	return &pbds.ListTaskBatchResp{
		Count: uint32(count),
		List:  list,
	}, nil
}
