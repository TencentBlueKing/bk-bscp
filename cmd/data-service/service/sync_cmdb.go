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
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/common/task/stores/iface"
	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/internal/task"
	cmdbGse "github.com/TencentBlueKing/bk-bscp/internal/task/builder/cmdb_gse"
	"github.com/TencentBlueKing/bk-bscp/internal/task/builder/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// SyncCmdbGseStatus implements pbds.DataServer.
func (s *Service) SyncCmdbGseStatus(ctx context.Context, req *pbds.SyncCmdbGseStatusReq) (*pbds.SyncCmdbGseStatusResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	processOperateTask, err := task.NewByTaskBuilder(
		cmdbGse.NewSyncCMDBGSETask(req.GetBizId(), gse.OpTypeQuery, grpcKit.User),
	)
	if err != nil {
		logs.Errorf("create sync cmdb task failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	// 启动任务
	s.taskManager.Dispatch(processOperateTask)

	return &pbds.SyncCmdbGseStatusResp{
		TaskId: processOperateTask.TaskID,
	}, nil

}

// SynchronizeCmdbData 同步cmdb数据
func (s *Service) SynchronizeCmdbData(ctx context.Context, bizIDs []int) error {
	grpcKit := kit.FromGrpcContext(ctx)
	// 不指定业务同步，表示同步所有业务
	if len(bizIDs) == 0 {
		business, err := s.cmdb.SearchBusinessByAccount(ctx, bkcmdb.SearchSetReq{
			BkSupplierAccount: "0",
			Fields:            []string{"bk_biz_id", "bk_biz_name"},
		})
		if err != nil {
			return fmt.Errorf("get business data failed: %v", err)
		}

		for _, item := range business.Info {
			bizIDs = append(bizIDs, item.BkBizID)
		}
	}

	for _, id := range bizIDs {
		processOperateTask, err := task.NewByTaskBuilder(
			cmdbGse.NewSyncCMDBGSETask(uint32(id), gse.OpTypeQuery, grpcKit.User),
		)
		if err != nil {
			logs.Errorf("create sync cmdb task failed, err: %v, rid: %s", err, grpcKit.Rid)
			return err
		}
		// 启动任务
		s.taskManager.Dispatch(processOperateTask)
	}

	logs.Infof("[syncBiz][group][success] all biz sync completed")
	return nil
}

// CmdbGseStatus implements pbds.DataServer.
func (s *Service) CmdbGseStatus(ctx context.Context, req *pbds.CmdbGseStatusReq) (*pbds.CmdbGseStatusResp, error) {

	// 获取通过业务查询是否有同步任务
	task, err := s.taskManager.ListTask(ctx, &iface.ListOption{
		TaskType:      common.SyncCMDBGSETaskType,
		TaskName:      cmdbGse.BuildSyncCMDBGSETaskName(req.GetBizId()),
		TaskIndex:     fmt.Sprintf("%d", req.GetBizId()),
		TaskIndexType: common.BizIDTaskIndexType,
		Offset:        0,
		Limit:         1,
	})
	if err != nil {
		return nil, err
	}

	var (
		rawStatus    string
		lastSyncTime time.Time
	)

	if task.Count == 0 || len(task.Items) == 0 {
		return &pbds.CmdbGseStatusResp{
			LastSyncTime: nil,
			Status:       constant.StatusNeverSynced,
		}, nil
	}

	item := task.Items[0]
	rawStatus = item.GetStatus()
	lastSyncTime = item.GetEndTime()

	return &pbds.CmdbGseStatusResp{
		LastSyncTime: timestamppb.New(lastSyncTime),
		Status:       simplifyTaskStatus(rawStatus),
	}, nil
}

// simplifyTaskStatus 简化任务状态
func simplifyTaskStatus(status string) string {
	switch status {
	case types.TaskStatusInit, types.TaskStatusRunning, types.TaskStatusNotStarted:
		return constant.StatusRunning
	case types.TaskStatusSuccess:
		return constant.StatusSuccess
	case types.TaskStatusFailure,
		types.TaskStatusTimeout,
		types.TaskStatusRevoked:
		return constant.StatusFailure
	}

	return constant.StatusNeverSynced
}
