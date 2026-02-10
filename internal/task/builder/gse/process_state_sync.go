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

package gse

import (
	"fmt"

	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/task/builder/common"
	"github.com/TencentBlueKing/bk-bscp/internal/task/step/gse"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

type processStateSyncTask struct {
	*common.Builder
	bizID            uint32
	process          *table.Process
	processInstances []*table.ProcessInstance
}

// NewProcessStateSyncTask 创建一个进程状态同步任务
func NewProcessStateSyncTask(dao dao.Set, bizID uint32, process *table.Process,
	processInstances []*table.ProcessInstance) types.TaskBuilder {
	return &processStateSyncTask{
		Builder:          common.NewBuilder(dao),
		bizID:            bizID,
		process:          process,
		processInstances: processInstances,
	}
}

// FinalizeTask implements [types.TaskBuilder].
func (p *processStateSyncTask) FinalizeTask(t *types.Task) error {
	return nil
}

// Steps implements [types.TaskBuilder].
func (p *processStateSyncTask) Steps() ([]*types.Step, error) {
	// 构建任务的步骤
	return []*types.Step{
		gse.ProcessStateSync(p.bizID, p.process, p.processInstances),
	}, nil
}

// TaskInfo implements [types.TaskBuilder].
func (p *processStateSyncTask) TaskInfo() types.TaskInfo {
	return types.TaskInfo{
		TaskName:      fmt.Sprintf("process-state-sync-%d", p.bizID),
		TaskType:      common.ProcessStateSyncType,
		TaskIndexType: common.BizIDTaskIndexType,
		TaskIndex:     fmt.Sprintf("%d", p.bizID),
	}
}
