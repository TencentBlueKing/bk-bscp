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
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

// DeleteTask 清除进程实例任务
// 清除语义在下发前已按实例状态拆解为停止 / 取消托管（已停止且未托管的实例直接删库，不建任务），
// 任务内以拆解后的实际操作类型执行，批次动作保持 delete 以区分任务类型。
type DeleteTask struct {
	*common.Builder
	tenantID                  string
	bizID                     uint32
	batchID                   uint32
	processID                 uint32
	processInstanceID         uint32
	operateType               table.ProcessOperateType // 拆解后的实际操作（停止 / 取消托管）
	operatorUser              string
	originalProcManagedStatus table.ProcessManagedStatus // 原进程托管状态，用于后续状态回滚
	originalProcStatus        table.ProcessStatus        // 原进程状态，用于后续状态回滚
	ccSyncStatus              table.CCSyncStatus         // 进程 CC 同步状态（下发时刻快照），供状态类校验使用
	taskType                  string                     // 任务批次的操作类型（delete）
}

// NewDeleteTask 创建一个清除进程实例任务
func NewDeleteTask(
	dao dao.Set,
	tenantID string,
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	operateType table.ProcessOperateType,
	operatorUser string,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
	ccSyncStatus table.CCSyncStatus,
	taskType string,
) types.TaskBuilder {
	return &DeleteTask{
		Builder:                   common.NewBuilder(dao),
		tenantID:                  tenantID,
		bizID:                     bizID,
		batchID:                   batchID,
		processID:                 processID,
		processInstanceID:         processInstanceID,
		operateType:               operateType,
		operatorUser:              operatorUser,
		originalProcManagedStatus: originalProcManagedStatus,
		originalProcStatus:        originalProcStatus,
		ccSyncStatus:              ccSyncStatus,
		taskType:                  taskType,
	}
}

// FinalizeTask implements types.TaskBuilder.
func (t *DeleteTask) FinalizeTask(task *types.Task) error {
	// 设置通用进程信息（包括原始状态）
	if _, err := t.CommonProcessFinalize(task, t.tenantID, t.bizID, t.processID, t.processInstanceID); err != nil {
		return err
	}

	// 设置回调用于失败回滚
	task.SetCallback(string(processExecutor.DeleteCallbackName))

	return nil
}

// Steps implements types.TaskBuilder.
func (t *DeleteTask) Steps() ([]*types.Step, error) {
	// 构建任务的步骤（清除链路独立步骤集，不复用通用进程操作步骤）
	return []*types.Step{
		// 对比 DB 配置与 CMDB 最新配置（清除专属：缺失/不一致均放行，以 DB 配置清除）
		processStep.DeleteCompareWithCMDBStep(
			t.tenantID,
			t.bizID,
			t.batchID,
			t.processID,
			t.processInstanceID,
			t.operateType,
			t.operatorUser,
			t.originalProcManagedStatus,
			t.originalProcStatus,
			t.ccSyncStatus,
		),

		// 校验操作是否合法（属性矩阵 + 状态类校验）
		processStep.DeleteValidateOperateStep(
			t.tenantID,
			t.bizID,
			t.batchID,
			t.processID,
			t.processInstanceID,
			t.operateType,
			t.operatorUser,
			t.originalProcManagedStatus,
			t.originalProcStatus,
			t.ccSyncStatus,
		),

		// 执行清除操作（停止 / 取消托管，用 DB 配置）
		processStep.DeleteOperateStep(
			t.tenantID,
			t.bizID,
			t.batchID,
			t.processID,
			t.processInstanceID,
			t.operateType,
			t.operatorUser,
			t.originalProcManagedStatus,
			t.originalProcStatus,
			t.ccSyncStatus,
		),

		// 清除操作完成，更新进程实例状态
		processStep.DeleteFinalizeOperateStep(
			t.tenantID,
			t.bizID,
			t.batchID,
			t.processID,
			t.processInstanceID,
			t.operateType,
			t.operatorUser,
			t.originalProcManagedStatus,
			t.originalProcStatus,
			t.ccSyncStatus,
		),
	}, nil
}

// TaskInfo implements types.TaskBuilder.
func (t *DeleteTask) TaskInfo() types.TaskInfo {
	return types.TaskInfo{
		TaskName:      fmt.Sprintf("process_operate_%s_%d", t.taskType, t.processInstanceID),
		TaskType:      t.taskType,                   // 存具体的操作类型，防止任务详情拿到其他的任务
		TaskIndexType: common.TaskIndexType,         // 任务一个索引类型，比如key，uuid等，
		TaskIndex:     fmt.Sprintf("%d", t.batchID), // 任务索引，代表一批任务
		Creator:       t.operatorUser,
	}
}
