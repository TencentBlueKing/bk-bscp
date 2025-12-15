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

// ValidateOperateProcess 校验操作是否合法
func ValidateOperateProcess(
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	operateType table.ProcessOperateType,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
) *types.Step {
	logs.V(3).Infof("validate operate process: bizID: %d, processID: %d, processInstanceID: %d, opType: %s",
		bizID, processID, processInstanceID, operateType)
	validate := types.NewStep(process.ValidateOperateProcessStepName.String(),
		process.ValidateOperateProcessStepName.String()).
		SetAlias("validate_operate_process").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(0) // 校验操作是否合法，不需要重试
	lo.Must0(validate.SetPayload(process.OperatePayload{
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		OperateType:               operateType,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
	}))
	return validate
}

// CompareWithCMDBProcessInfo 对比CMDB进程信息
func CompareWithCMDBProcessInfo(
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	needCompareCMDB bool,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
) *types.Step {
	logs.V(3).Infof("compare with cmdb process info: bizID: %d, processID: %d, processInstanceID: %d, needCompareCMDB: %t",
		bizID, processID, processInstanceID, needCompareCMDB)

	compare := types.NewStep(process.CompareWithCMDBProcessInfoStepName.String(),
		process.CompareWithCMDBProcessInfoStepName.String()).
		SetAlias("compare_with_cmdb_process_info").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(MaxTries)

	lo.Must0(compare.SetPayload(process.OperatePayload{
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		NeedCompareCMDB:           needCompareCMDB,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
	}))

	return compare
}

// CompareWithGSEProcessStatus 对比GSE进程状态
func CompareWithGSEProcessStatus(
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
) *types.Step {
	logs.V(3).Infof("compare with gse process status: bizID: %d, processID: %d, processInstanceID: %d",
		bizID, processID, processInstanceID)

	compare := types.NewStep(process.CompareWithGSEProcessStatusStepName.String(),
		process.CompareWithGSEProcessStatusStepName.String()).
		SetAlias("compare_with_gse_process_status").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(MaxTries)

	lo.Must0(compare.SetPayload(process.OperatePayload{
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
	}))

	return compare
}

// CompareWithGSEProcessConfig 对比GSE进程配置
func CompareWithGSEProcessConfig(
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
) *types.Step {
	logs.V(3).Infof("compare with gse process config: bizID: %d, processID: %d, processInstanceID: %d",
		bizID, processID, processInstanceID)

	compare := types.NewStep(process.CompareWithGSEProcessConfigStepName.String(),
		process.CompareWithGSEProcessConfigStepName.String()).
		SetAlias("compare_with_gse_process_config").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(MaxTries)

	lo.Must0(compare.SetPayload(process.OperatePayload{
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
	}))

	return compare
}

// OperateProcess 进程操作
func OperateProcess(
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	operateType table.ProcessOperateType,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
) *types.Step {
	logs.V(3).Infof("operate process: bizID: %d, processID: %d, processInstanceID: %d, opType: %s",
		bizID, processID, processInstanceID, operateType)

	operate := types.NewStep(process.OperateProcessStepName.String(), process.OperateProcessStepName.String()).
		SetAlias("operate_process").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(0) // 进程操作只操作一次，避免重复操作

	lo.Must0(operate.SetPayload(process.OperatePayload{
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		OperateType:               operateType,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
	}))
	return operate
}

// FinalizeOperateProcess 进程操作完成
func FinalizeOperateProcess(
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	operateType table.ProcessOperateType,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
) *types.Step {
	logs.V(3).Infof("finalize process: bizID: %d, processID: %d, processInstanceID: %d, opType: %s",
		bizID, processID, processInstanceID, operateType)

	finalize := types.NewStep(process.FinalizeOperateProcessStepName.String(), process.FinalizeOperateProcessStepName.String()).
		SetAlias("finalize_operate_process").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(MaxTries)

	lo.Must0(finalize.SetPayload(process.OperatePayload{
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		OperateType:               operateType,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
	}))
	return finalize
}
