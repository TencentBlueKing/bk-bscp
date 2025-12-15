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

package cmdbGse

import (
	"fmt"

	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	gseSvc "github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/task/builder/common"
	"github.com/TencentBlueKing/bk-bscp/internal/task/step/cmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/task/step/gse"
)

type syncCMDBGSETask struct {
	bizID        uint32
	opType       gseSvc.OpType
	operatorUser string
}

// NewSyncCMDBGSETask 创建一个 同步cmdb 任务
func NewSyncCMDBGSETask(bizID uint32, opType gseSvc.OpType, operatorUser string) types.TaskBuilder {
	return &syncCMDBGSETask{
		bizID:        bizID,
		opType:       opType,
		operatorUser: operatorUser,
	}
}

// FinalizeTask implements types.TaskBuilder.
func (s *syncCMDBGSETask) FinalizeTask(t *types.Task) error {
	// 设置一些通用的回调，比如执行结果回调
	return nil
}

// Steps implements types.TaskBuilder.
func (s *syncCMDBGSETask) Steps() ([]*types.Step, error) {
	// 构建任务的步骤
	return []*types.Step{
		cmdb.SyncCMDB(s.bizID),
		gse.SyncGseStatus(s.bizID, s.opType),
	}, nil
}

// TaskInfo implements types.TaskBuilder.
func (s *syncCMDBGSETask) TaskInfo() types.TaskInfo {
	return types.TaskInfo{
		TaskName:      BuildSyncCMDBGSETaskName(s.bizID),
		TaskType:      common.SyncCMDBGSETaskType,
		TaskIndexType: common.BizIDTaskIndexType,
		TaskIndex:     fmt.Sprintf("%d", s.bizID),
		Creator:       s.operatorUser,
	}
}

// BuildSyncCMDBTaskName 构造同步CMDB任务名
func BuildSyncCMDBGSETaskName(bizID uint32) string {
	return fmt.Sprintf("sync-cmdb-gse-%d", bizID)
}
