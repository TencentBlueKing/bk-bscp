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

// OperateTask task operate
type OperateTask struct {
	*common.Builder
	tenantID                  string
	bizID                     uint32
	batchID                   uint32
	processID                 uint32
	processInstanceID         uint32
	operateType               table.ProcessOperateType
	operatorUser              string
	originalProcManagedStatus table.ProcessManagedStatus // 原进程托管状态，用于后续状态回滚
	originalProcStatus        table.ProcessStatus        // 原进程状态，用于后续状态回滚
	ccSyncStatus              table.CCSyncStatus         // 进程 CC 同步状态（下发时刻快照），供状态类校验使用
	taskType                  string                     // 任务批次的操作类型
}

// NewoperateTask 创建一个 operate 任务
func NewOperateTask(
	dao dao.Set,
	tenantID string,
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	operateType table.ProcessOperateType,
	operatorUser string,
	originalProcManagedStatus table.ProcessManagedStatus, // 原进程托管状态，用于后续状态回滚
	originalProcStatus table.ProcessStatus, // 原进程状态，用于后续状态回滚
	ccSyncStatus table.CCSyncStatus, // 进程 CC 同步状态（下发时刻快照），供状态类校验使用
	taskType string, // 任务批次的操作类型
) types.TaskBuilder {
	return &OperateTask{
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
func (t *OperateTask) FinalizeTask(task *types.Task) error {
	// 设置通用进程信息（包括原始状态）
	if err := t.CommonProcessFinalize(task, t.tenantID, t.bizID, t.processID, t.processInstanceID); err != nil {
		return err
	}

	// 设置回调用于失败回滚
	task.SetCallback(string(processExecutor.ProcessOperateCallbackName))

	return nil
}

// Steps implements types.TaskBuilder.
func (t *OperateTask) Steps() ([]*types.Step, error) {
	// 构建任务的步骤
	return []*types.Step{
		// 对比 DB 配置与 CMDB 最新配置，选定执行配置（已删除进程的停止操作回退 DB 配置，其余报错）
		processStep.CompareWithCMDBProcessInfo(
			t.tenantID,
			t.bizID,
			t.batchID,
			t.processID,
			t.processInstanceID,
			t.operateType,
			t.operatorUser,
			t.originalProcManagedStatus,
			t.originalProcStatus,
		),

		// 校验操作是否合法（对 Compare 步骤选定的执行配置做属性矩阵 + 状态类校验）
		processStep.ValidateOperateProcess(
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

		// 执行进程操作
		processStep.OperateProcess(
			t.tenantID,
			t.bizID,
			t.batchID,
			t.processID,
			t.processInstanceID,
			t.operateType,
			t.originalProcManagedStatus,
			t.originalProcStatus,
		),

		// 进程操作完成，更新进程实例状态
		processStep.FinalizeOperateProcess(
			t.tenantID,
			t.bizID,
			t.batchID,
			t.processID,
			t.processInstanceID,
			t.operateType,
			t.originalProcManagedStatus,
			t.originalProcStatus,
		),
	}, nil
}

// TaskInfo implements types.TaskBuilder.
func (t *OperateTask) TaskInfo() types.TaskInfo {
	return types.TaskInfo{
		TaskName:      fmt.Sprintf("process_operate_%s_%d", t.operateType, t.processInstanceID),
		TaskType:      t.taskType,                   // 存具体的操作类型，防止任务详情拿到其他的任务
		TaskIndexType: common.TaskIndexType,         // 任务一个索引类型，比如key，uuid等，
		TaskIndex:     fmt.Sprintf("%d", t.batchID), // 任务索引，代表一批任务
		Creator:       t.operatorUser,
	}
}
