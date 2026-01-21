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
	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"github.com/samber/lo"

	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/process"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// ValidateOperateStep 验证更新托管进程操作步骤
func ValidateOperateStep(
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
	operateType table.ProcessOperateType,
	enableProcessRestart bool,
	ccSyncStatus table.CCSyncStatus,
) *types.Step {

	logs.V(3).Infof("ValidateOperateStep: bizID: %d, processID: %d, processInstanceID: %d",
		bizID, processID, processInstanceID)

	setp := types.NewStep(process.ValidateOperateStepName.String(),
		process.ValidateOperateStepName.String()).
		SetAlias("validate_operate").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(MaxTries)

	lo.Must0(setp.SetPayload(process.UpdateRegisterPayload{
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		OperateType:               operateType,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
		EnableProcessRestart:      enableProcessRestart,
		CCSyncStatus:              ccSyncStatus,
	}))

	return setp
}

// StopProcessStep xxx
func StopProcessStep(
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
	ccSyncStatus table.CCSyncStatus,
) *types.Step {

	setp := types.NewStep(process.StopProcessStepName.String(),
		process.StopProcessStepName.String()).
		SetAlias("stop_process").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(MaxTries)

	lo.Must0(setp.SetPayload(process.UpdateRegisterPayload{
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
		CCSyncStatus:              ccSyncStatus,
	}))

	return setp
}

// RegisterProcessStep xxx
func RegisterProcessStep(
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
	ccSyncStatus table.CCSyncStatus,
) *types.Step {

	setp := types.NewStep(process.RegisterProcessStepName.String(),
		process.RegisterProcessStepName.String()).
		SetAlias("register_process").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(MaxTries)

	lo.Must0(setp.SetPayload(process.UpdateRegisterPayload{
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
		CCSyncStatus:              ccSyncStatus,
	}))

	return setp
}

// StartProcessStep xxx
func StartProcessStep(
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
	ccSyncStatus table.CCSyncStatus,
) *types.Step {

	setp := types.NewStep(process.StartProcessStepName.String(),
		process.StartProcessStepName.String()).
		SetAlias("start_process").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(MaxTries)

	lo.Must0(setp.SetPayload(process.UpdateRegisterPayload{
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
		CCSyncStatus:              ccSyncStatus,
	}))

	return setp
}

// OperationCompletedStep xxx
func OperationCompletedStep(
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
	ccSyncStatus table.CCSyncStatus,
) *types.Step {

	setp := types.NewStep(process.OperationCompletedStepName.String(),
		process.OperationCompletedStepName.String()).
		SetAlias("operation_completed").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(MaxTries)

	lo.Must0(setp.SetPayload(process.UpdateRegisterPayload{
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
		CCSyncStatus:              ccSyncStatus,
	}))

	return setp
}
