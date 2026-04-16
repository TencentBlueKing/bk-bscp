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
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// ValidateOperateProcess 校验操作是否合法
func ValidateOperateProcess(
	tenantID string,
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	operateType table.ProcessOperateType,
	operateUser string,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
	ccSyncStatus table.CCSyncStatus,
) *types.Step {
	logs.V(3).Infof("validate operate process: bizID: %d, processID: %d, processInstanceID: %d, opType: %s",
		bizID, processID, processInstanceID, operateType)
	vtf := cc.G().TaskFramework.ProcessOperate.ValidateOperateProcess
	validate := types.NewStep(process.ValidateOperateProcessStepName.String(),
		process.ValidateOperateProcessStepName.String()).
		SetAlias("validate_operate_process").
		SetMaxExecution(vtf.MaxExecution).
		SetMaxTries(vtf.MaxRetries)
	lo.Must0(validate.SetPayload(process.OperatePayload{
		TenantID:                  tenantID,
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		OperateType:               operateType,
		OperateUser:               operateUser,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
		CCSyncStatus:              ccSyncStatus,
	}))
	return validate
}

// CompareWithCMDBProcessInfo 对比CMDB进程信息
func CompareWithCMDBProcessInfo(
	tenantID string,
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	needCompareCMDB bool,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
	ccSyncStatus table.CCSyncStatus,
) *types.Step {
	logs.V(3).Infof("compare with cmdb process info: bizID: %d, processID: %d, processInstanceID: %d, needCompareCMDB: %t",
		bizID, processID, processInstanceID, needCompareCMDB)

	ctf := cc.G().TaskFramework.ProcessOperate.CompareWithCMDBProcessInfo
	compare := types.NewStep(process.CompareWithCMDBProcessInfoStepName.String(),
		process.CompareWithCMDBProcessInfoStepName.String()).
		SetAlias("compare_with_cmdb_process_info").
		SetMaxExecution(ctf.MaxExecution).
		SetMaxTries(ctf.MaxRetries)

	lo.Must0(compare.SetPayload(process.OperatePayload{
		TenantID:                  tenantID,
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		NeedCompareCMDB:           needCompareCMDB,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
		CCSyncStatus:              ccSyncStatus,
	}))

	return compare
}

// CompareWithGSEProcessStatus 对比GSE进程状态
func CompareWithGSEProcessStatus(
	tenantID string,
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
	ccSyncStatus table.CCSyncStatus,
) *types.Step {
	logs.V(3).Infof("compare with gse process status: bizID: %d, processID: %d, processInstanceID: %d",
		bizID, processID, processInstanceID)

	stf := cc.G().TaskFramework.ProcessOperate.CompareWithGSEProcessStatus
	compare := types.NewStep(process.CompareWithGSEProcessStatusStepName.String(),
		process.CompareWithGSEProcessStatusStepName.String()).
		SetAlias("compare_with_gse_process_status").
		SetMaxExecution(stf.MaxExecution).
		SetMaxTries(stf.MaxRetries)

	lo.Must0(compare.SetPayload(process.OperatePayload{
		TenantID:                  tenantID,
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
		CCSyncStatus:              ccSyncStatus,
	}))

	return compare
}

// CompareWithGSEProcessConfig 对比GSE进程配置
func CompareWithGSEProcessConfig(
	tenantID string,
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
	ccSyncStatus table.CCSyncStatus,
) *types.Step {
	logs.V(3).Infof("compare with gse process config: bizID: %d, processID: %d, processInstanceID: %d",
		bizID, processID, processInstanceID)

	gcf := cc.G().TaskFramework.ProcessOperate.CompareWithGSEProcessConfig
	compare := types.NewStep(process.CompareWithGSEProcessConfigStepName.String(),
		process.CompareWithGSEProcessConfigStepName.String()).
		SetAlias("compare_with_gse_process_config").
		SetMaxExecution(gcf.MaxExecution).
		SetMaxTries(gcf.MaxRetries)

	lo.Must0(compare.SetPayload(process.OperatePayload{
		TenantID:                  tenantID,
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
		CCSyncStatus:              ccSyncStatus,
	}))

	return compare
}

// OperateProcess 进程操作
func OperateProcess(
	tenantID string,
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	operateType table.ProcessOperateType,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
	ccSyncStatus table.CCSyncStatus,
) *types.Step {
	logs.V(3).Infof("operate process: bizID: %d, processID: %d, processInstanceID: %d, opType: %s",
		bizID, processID, processInstanceID, operateType)

	otf := cc.G().TaskFramework.ProcessOperate.OperateProcess
	operate := types.NewStep(process.OperateProcessStepName.String(), process.OperateProcessStepName.String()).
		SetAlias("operate_process").
		SetMaxExecution(otf.MaxExecution).
		SetMaxTries(otf.MaxRetries)

	lo.Must0(operate.SetPayload(process.OperatePayload{
		TenantID:                  tenantID,
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		OperateType:               operateType,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
		CCSyncStatus:              ccSyncStatus,
	}))
	return operate
}

// FinalizeOperateProcess 进程操作完成
func FinalizeOperateProcess(
	tenantID string,
	bizID uint32,
	batchID uint32,
	processID uint32,
	processInstanceID uint32,
	operateType table.ProcessOperateType,
	originalProcManagedStatus table.ProcessManagedStatus,
	originalProcStatus table.ProcessStatus,
	ccSyncStatus table.CCSyncStatus,
) *types.Step {
	logs.V(3).Infof("finalize process: bizID: %d, processID: %d, processInstanceID: %d, opType: %s",
		bizID, processID, processInstanceID, operateType)

	ftf := cc.G().TaskFramework.ProcessOperate.FinalizeOperateProcess
	finalize := types.NewStep(process.FinalizeOperateProcessStepName.String(), process.FinalizeOperateProcessStepName.String()).
		SetAlias("finalize_operate_process").
		SetMaxExecution(ftf.MaxExecution).
		SetMaxTries(ftf.MaxRetries)

	lo.Must0(finalize.SetPayload(process.OperatePayload{
		TenantID:                  tenantID,
		BizID:                     bizID,
		BatchID:                   batchID,
		ProcessID:                 processID,
		ProcessInstanceID:         processInstanceID,
		OperateType:               operateType,
		OriginalProcManagedStatus: originalProcManagedStatus,
		OriginalProcStatus:        originalProcStatus,
		CCSyncStatus:              ccSyncStatus,
	}))
	return finalize
}
