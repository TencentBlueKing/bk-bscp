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

// UpdateRegisterTask 更新托管任务
type UpdateRegisterTask struct {
	*common.Builder
	bizID                     uint32
	batchID                   uint32
	processID                 uint32
	processInstanceID         uint32
	operateType               table.ProcessOperateType
	operatorUser              string
	originalProcManagedStatus table.ProcessManagedStatus // 原进程托管状态，用于后续状态回滚
	originalProcStatus        table.ProcessStatus        // 原进程状态，用于后续状态回滚
	enableProcessRestart      bool                       // 是否启停进程
	ccSyncStatus              table.CCSyncStatus         // 进程的cc同步状态
}

// NewUpdateRegisterTask 创建一个更新托管任务
func NewUpdateRegisterTask(
	dao dao.Set,
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	operatorUser string,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
	ccSyncStatus table.CCSyncStatus, // 进程的cc同步状态
	enableProcessRestart bool,
) types.TaskBuilder {
	return &UpdateRegisterTask{
		Builder:                   common.NewBuilder(dao),
		bizID:                     bizID,
		batchID:                   batchID,
		processID:                 processID,
		processInstanceID:         processInstanceID,
		operatorUser:              operatorUser,
		operateType:               table.UpdateRegisterProcessOperate, // 直接定义成更新托管信息
		originalProcManagedStatus: originalProcManagedStatus,
		originalProcStatus:        originalProcStatus,
		ccSyncStatus:              ccSyncStatus,
		enableProcessRestart:      enableProcessRestart,
	}
}

// FinalizeTask implements types.TaskBuilder.
func (t *UpdateRegisterTask) FinalizeTask(task *types.Task) error {
	// 设置通用进程信息（包括原始状态）
	if err := t.CommonProcessFinalize(task, t.bizID, t.processID, t.processInstanceID); err != nil {
		return err
	}

	// 设置回调用于失败回滚
	task.SetCallback(string(processExecutor.UpdateRegisterCallbackName))

	return nil
}

// Steps implements types.TaskBuilder.
func (t *UpdateRegisterTask) Steps() ([]*types.Step, error) {
	steps := make([]*types.Step, 0, 4)
	// 1. 校验操作（必选）
	steps = append(steps,
		processStep.ValidateOperateStep(
			t.bizID,
			t.batchID,
			t.processID,
			t.processInstanceID,
			t.originalProcManagedStatus,
			t.originalProcStatus,
			t.operateType,
			t.enableProcessRestart,
			t.ccSyncStatus,
		),
	)

	// 2. 更新托管信息（必选）
	steps = append(steps,
		processStep.RegisterProcessStep(
			t.bizID,
			t.batchID,
			t.processID,
			t.processInstanceID,
			t.originalProcManagedStatus,
			t.originalProcStatus,
			t.ccSyncStatus,
		),
	)

	// 3. 是否需要重启进程
	if t.enableProcessRestart {
		// Stop 旧进程
		steps = append(steps,
			processStep.StopProcessStep(
				t.bizID,
				t.batchID,
				t.processID,
				t.processInstanceID,
				t.originalProcManagedStatus,
				t.originalProcStatus,
				t.ccSyncStatus,
			),
		)
	}

	// 4. 是否需要启动进程
	if t.enableProcessRestart {
		steps = append(steps,
			processStep.StartProcessStep(
				t.bizID,
				t.batchID,
				t.processID,
				t.processInstanceID,
				t.originalProcManagedStatus,
				t.originalProcStatus,
				t.ccSyncStatus,
			),
		)
	}

	// 3. 进程操作完成（必选）
	steps = append(steps,
		processStep.OperationCompletedStep(
			t.bizID,
			t.batchID,
			t.processID,
			t.processInstanceID,
			t.originalProcManagedStatus,
			t.originalProcStatus,
			t.ccSyncStatus,
		),
	)

	return steps, nil
}

// TaskInfo implements types.TaskBuilder.
func (t *UpdateRegisterTask) TaskInfo() types.TaskInfo {
	return types.TaskInfo{
		TaskName:      fmt.Sprintf("process_operate_%s_%d", t.operateType, t.processInstanceID),
		TaskType:      string(t.operateType),        // 更新托管操作
		TaskIndexType: common.TaskIndexType,         // 任务一个索引类型，比如key，uuid等，
		TaskIndex:     fmt.Sprintf("%d", t.batchID), // 任务索引，代表一批任务
		Creator:       t.operatorUser,
	}
}
