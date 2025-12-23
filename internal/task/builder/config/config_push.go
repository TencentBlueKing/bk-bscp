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

package config

import (
	"fmt"

	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/task/builder/common"
	executorCommon "github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	configExecutor "github.com/TencentBlueKing/bk-bscp/internal/task/executor/config"
	configStep "github.com/TencentBlueKing/bk-bscp/internal/task/step/config"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

type PushConfigTask struct {
	*common.Builder
	bizID               uint32
	batchID             uint32
	operateType         table.ConfigOperateType
	operatorUser        string
	generateTaskID      string
	generateTaskPayload *executorCommon.TaskPayload
}

func NewPushConfigTask(
	dao dao.Set,
	bizID uint32,
	batchID uint32,
	operateType table.ConfigOperateType,
	operatorUser string,
	generateTaskID string,
	generateTaskPayload *executorCommon.TaskPayload,
) types.TaskBuilder {
	return &PushConfigTask{
		Builder:             common.NewBuilder(dao),
		bizID:               bizID,
		batchID:             batchID,
		operateType:         operateType,
		operatorUser:        operatorUser,
		generateTaskID:      generateTaskID,
		generateTaskPayload: generateTaskPayload,
	}
}

// FinalizeTask implements types.TaskBuilder.
func (t *PushConfigTask) FinalizeTask(task *types.Task) error {
	// 设置 CommonPayload，从配置生成任务继承
	if err := task.SetCommonPayload(t.generateTaskPayload); err != nil {
		return err
	}

	// 设置回调，用于任务完成后更新 TaskBatch 状态
	task.SetCallback(string(configExecutor.CallbackName))

	return nil
}

func (t *PushConfigTask) Steps() ([]*types.Step, error) {
	// 构建配置下发的步骤
	steps := []*types.Step{
		// 1. 验证步骤
		configStep.ValidatePushConfig(
			t.bizID,
			t.batchID,
			t.operateType,
			t.operatorUser,
			t.generateTaskID,
			t.generateTaskPayload,
		),
		configStep.ReleaseConfig(t.bizID,
			t.batchID,
			t.operateType,
			t.operatorUser,
			t.generateTaskID,
			t.generateTaskPayload,
		),
	}

	return steps, nil
}

func (t *PushConfigTask) TaskInfo() types.TaskInfo {
	// 使用配置实例 key 作为任务名称的一部分
	configKey := ""
	if t.generateTaskPayload != nil && t.generateTaskPayload.ConfigPayload != nil {
		configKey = t.generateTaskPayload.ConfigPayload.ConfigInstanceKey
	}

	taskName := fmt.Sprintf("%s_%s", t.operateType, configKey)
	return types.TaskInfo{
		TaskName:      taskName,
		TaskType:      common.ConfigPushTaskType,
		TaskIndexType: common.TaskIndexType,
		TaskIndex:     fmt.Sprintf("%d", t.batchID),
		Creator:       t.operatorUser,
	}
}
