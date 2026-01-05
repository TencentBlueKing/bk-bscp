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
	bizID                     uint32
	batchID                   uint32
	processID                 uint32
	processInstanceID         uint32
	operateType               table.ProcessOperateType
	operatorUser              string
	originalProcManagedStatus table.ProcessManagedStatus // 原进程托管状态，用于后续状态回滚
	originalProcStatus        table.ProcessStatus        // 原进程状态，用于后续状态回滚
	needCompareCMDB           bool                       // 是否需要对比cmdb配置，适配页面强制更新的场景
}

// NewoperateTask 创建一个 operate 任务
func NewOperateTask(
	dao dao.Set,
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	operateType table.ProcessOperateType,
	operatorUser string,
	needCompareCMDB bool, // 是否需要对比cmdb配置，适配页面强制更新的场景
	originalProcManagedStatus table.ProcessManagedStatus, // 原进程托管状态，用于后续状态回滚
	originalProcStatus table.ProcessStatus, // 原进程状态，用于后续状态回滚
) types.TaskBuilder {
	return &OperateTask{
		Builder:                   common.NewBuilder(dao),
		bizID:                     bizID,
		batchID:                   batchID,
		processID:                 processID,
		processInstanceID:         processInstanceID,
		operateType:               operateType,
		operatorUser:              operatorUser,
		originalProcManagedStatus: originalProcManagedStatus,
		originalProcStatus:        originalProcStatus,
		needCompareCMDB:           needCompareCMDB,
	}
}

// FinalizeTask implements types.TaskBuilder.
func (t *OperateTask) FinalizeTask(task *types.Task) error {
	// 设置通用进程信息（包括原始状态）
	if err := t.CommonProcessFinalize(task, t.bizID, t.processID, t.processInstanceID); err != nil {
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
		// TODO：这里可以增加时间间隔判断，比如cmdb这条数据更新时间再1min以内则不用判断
		// 校验操作是否合法
		processStep.ValidateOperateProcess(
			t.bizID,
			t.batchID,
			t.processID,
			t.processInstanceID,
			t.operateType,
			t.originalProcManagedStatus,
			t.originalProcStatus,
		),
		// 对比CMDB进程配置
		processStep.CompareWithCMDBProcessInfo(
			t.bizID,
			t.batchID,
			t.processID,
			t.processInstanceID,
			t.needCompareCMDB,
			t.originalProcManagedStatus,
			t.originalProcStatus,
		),

		// 对比GSE进程状态
		processStep.CompareWithGSEProcessStatus(
			t.bizID,
			t.batchID,
			t.processID,
			t.processInstanceID,
			t.originalProcManagedStatus,
			t.originalProcStatus,
		),

		// 对比GSE进程配置
		processStep.CompareWithGSEProcessConfig(
			t.bizID,
			t.batchID,
			t.processID,
			t.processInstanceID,
			t.originalProcManagedStatus,
			t.originalProcStatus,
		),

		// 执行进程操作
		processStep.OperateProcess(
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
		TaskType:      string(t.operateType),        // 存具体的操作类型，防止任务详情拿到其他的任务
		TaskIndexType: common.TaskIndexType,         // 任务一个索引类型，比如key，uuid等，
		TaskIndex:     fmt.Sprintf("%d", t.batchID), // 任务索引，代表一批任务
		Creator:       t.operatorUser,
	}
}
