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

package cmdb

import (
	"fmt"

	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/internal/task/step/cmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

const (
	// TaskType 任务类型
	TaskType = "cmdb_sync"
	// TaskIndexType 任务索引类型
	TaskIndexType = "biz_id"
)

type syncCMDBTask struct {
	bizID        uint32
	operatorUser string
	operateType  table.CCSyncStatus
}

// NewSyncCMDBTask 创建一个 同步cmdb 任务
func NewSyncCMDBTask(bizID uint32, operateType table.CCSyncStatus,
	operatorUser string) types.TaskBuilder {
	return &syncCMDBTask{
		bizID:        bizID,
		operateType:  operateType,
		operatorUser: operatorUser,
	}
}

// FinalizeTask implements types.TaskBuilder.
func (s *syncCMDBTask) FinalizeTask(t *types.Task) error {
	// 设置一些通用的回调，比如执行结果回调
	return nil
}

// Steps implements types.TaskBuilder.
func (s *syncCMDBTask) Steps() ([]*types.Step, error) {
	// 构建任务的步骤
	return []*types.Step{
		cmdb.SyncCMDB(s.bizID, s.operateType),
	}, nil
}

// TaskInfo implements types.TaskBuilder.
func (s *syncCMDBTask) TaskInfo() types.TaskInfo {
	return types.TaskInfo{
		TaskName:      BuildSyncCMDBTaskName(s.operateType.String(), s.bizID),
		TaskType:      TaskType,
		TaskIndexType: TaskIndexType,
		TaskIndex:     fmt.Sprintf("%d", s.bizID),
		Creator:       s.operatorUser,
	}
}

// BuildSyncCMDBTaskName 构造同步CMDB任务名
func BuildSyncCMDBTaskName(operateType string, bizID uint32) string {
	return fmt.Sprintf("sync-cmdb-%s-%d", operateType, bizID)
}
