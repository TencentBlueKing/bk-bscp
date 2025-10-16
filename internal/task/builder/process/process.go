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

package process

import (
	"fmt"

	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/task/builder/common"
	processExecutor "github.com/TencentBlueKing/bk-bscp/internal/task/executor/process"
	processStep "github.com/TencentBlueKing/bk-bscp/internal/task/step/process"
)

const (
	// TaskType 任务类型
	TaskType = "process_operate"
	// TaskIndexType 任务索引类型
	TaskIndexType = "task_batch"
)

// OperateTask task operate
type OperateTask struct {
	*common.Builder
	batchID           string
	processID         string
	processInstanceID string
	operateType       processExecutor.OperateType
	operatorUser      string
	needCompareCMDB   bool // 是否需要对比cmdb配置，适配页面强制更新的场景
}

// NewoperateTask 创建一个 operate 任务
func NewOperateTask(
	dao dao.Set,
	batchID string,
	processID string,
	processInstanceID string,
	operateType processExecutor.OperateType,
	operatorUser string,
	needCompareCMDB bool) types.TaskBuilder {
	return &OperateTask{
		Builder:           common.NewBuilder(dao),
		batchID:           batchID,
		processID:         processID,
		processInstanceID: processInstanceID,
		operateType:       operateType,
		operatorUser:      operatorUser,
		needCompareCMDB:   needCompareCMDB,
	}
}

// FinalizeTask implements types.TaskBuilder.
func (t *OperateTask) FinalizeTask(task *types.Task) error {
	t.CommonProcessFinalize(task, t.processInstanceID)

	return nil
}

// Steps implements types.TaskBuilder.
func (t *OperateTask) Steps() ([]*types.Step, error) {
	// 构建任务的步骤
	return []*types.Step{
		// 1、从 cmdb 获取最新的信息与DB主动对比是否一致，不一致则拒绝，TODO：这里可以增加时间间隔判断，比如cmdb这条数据更新时间再1min以内则不用判断

		// 2、获取gse管理的进程状态，判断是否跟db中存储一致

		// 3、通过GSE脚本执行获取gse托管的配置是否一致

		// 4、执行具体操作
		processStep.OperateProcess(t.processID, t.processInstanceID, t.operateType),
	}, nil
}

// TaskInfo implements types.TaskBuilder.
func (t *OperateTask) TaskInfo() types.TaskInfo {
	return types.TaskInfo{
		TaskName:      fmt.Sprintf("process_operate_%s_%s", t.operateType, t.processInstanceID),
		TaskType:      TaskType,
		TaskIndexType: TaskIndexType, // 任务一个索引类型，比如key，uuid等，
		TaskIndex:     t.batchID,     // 任务索引，代表一批任务
		Creator:       t.operatorUser,
	}
}
