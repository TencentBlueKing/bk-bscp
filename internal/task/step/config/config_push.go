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
	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"github.com/samber/lo"

	executorCommon "github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/config"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// ValidatePushConfig 验证配置下发步骤
func ValidatePushConfig(
	bizID uint32,
	batchID uint32,
	operateType table.ConfigOperateType,
	operatorUser string,
	generateTaskID string,
	generateTaskPayload *executorCommon.TaskPayload,
) *types.Step {
	logs.V(3).Infof("validate push config: bizID: %d, batchID: %d, operateType: %s", bizID, batchID, operateType)

	validate := types.NewStep(config.ValidatePushConfigStepName.String(), config.ValidatePushConfigStepName.String()).
		SetAlias("validate_push_config").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(0)

	lo.Must0(validate.SetPayload(config.PushConfigPayload{
		BizID:               bizID,
		BatchID:             batchID,
		OperateType:         operateType,
		OperatorUser:        operatorUser,
		GenerateTaskID:      generateTaskID,
		GenerateTaskPayload: generateTaskPayload,
	}))
	return validate
}

// DownloadConfig 下载配置到本地步骤
func DownloadConfig(
	bizID uint32,
	batchID uint32,
	operateType table.ConfigOperateType,
	operatorUser string,
	generateTaskID string,
	generateTaskPayload *executorCommon.TaskPayload,
) *types.Step {
	logs.V(3).Infof("download config: bizID: %d, batchID: %d, operateType: %s", bizID, batchID, operateType)

	download := types.NewStep(config.DownloadConfigStepName.String(), config.DownloadConfigStepName.String()).
		SetAlias("download_config").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(0)

	lo.Must0(download.SetPayload(config.PushConfigPayload{
		BizID:               bizID,
		BatchID:             batchID,
		OperateType:         operateType,
		OperatorUser:        operatorUser,
		GenerateTaskID:      generateTaskID,
		GenerateTaskPayload: generateTaskPayload,
	}))
	return download
}

// PushConfigToTarget 推送配置到目标机器步骤
func PushConfigToTarget(
	bizID uint32,
	batchID uint32,
	operateType table.ConfigOperateType,
	operatorUser string,
	generateTaskID string,
	generateTaskPayload *executorCommon.TaskPayload,
) *types.Step {
	logs.V(3).Infof("push config to target: bizID: %d, batchID: %d, operateType: %s", bizID, batchID, operateType)

	push := types.NewStep(config.PushConfigStepName.String(), config.PushConfigStepName.String()).
		SetAlias("push_config_to_target").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(0)

	lo.Must0(push.SetPayload(config.PushConfigPayload{
		BizID:               bizID,
		BatchID:             batchID,
		OperateType:         operateType,
		OperatorUser:        operatorUser,
		GenerateTaskID:      generateTaskID,
		GenerateTaskPayload: generateTaskPayload,
	}))
	return push
}

// ReleaseConfig 通过脚本方式下发配置步骤
func ReleaseConfig(
	bizID uint32,
	batchID uint32,
	operateType table.ConfigOperateType,
	operatorUser string,
	generateTaskID string,
	generateTaskPayload *executorCommon.TaskPayload,
) *types.Step {
	logs.V(3).Infof("release config: bizID: %d, batchID: %d, operateType: %s", bizID, batchID, operateType)

	push := types.NewStep(config.ReleaseConfigStepName.String(), config.ReleaseConfigStepName.String()).
		SetAlias("release_config").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(0)

	lo.Must0(push.SetPayload(config.PushConfigPayload{
		BizID:               bizID,
		BatchID:             batchID,
		OperateType:         operateType,
		OperatorUser:        operatorUser,
		GenerateTaskID:      generateTaskID,
		GenerateTaskPayload: generateTaskPayload,
	}))
	return push
}
