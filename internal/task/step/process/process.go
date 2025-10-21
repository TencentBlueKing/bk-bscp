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
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"github.com/samber/lo"

	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/process"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	// MaxExecutionTime 最大执行时间
	MaxExecutionTime = 30 * time.Second
	// MaxTries 最大重试次数
	MaxTries = 3
)

// OperatePrOperateProcessocess 进程操作
func OperateProcess(processID, processInstanceID uint32, operateType table.ProcessOperateType) *types.Step {
	logs.V(3).Infof("operate process: %s, process instance id: %s, op type: %s", processID, processInstanceID, operateType)

	operate := types.NewStep(process.OperateStepName.String(), process.OperateStepName.String()).
		SetAlias("operate_process").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(MaxTries)

	lo.Must0(operate.SetPayload(process.OperatePayload{
		ProcessID:         processID,
		ProcessInstanceID: processInstanceID,
		OperateType:       operateType,
	}))
	return operate
}
