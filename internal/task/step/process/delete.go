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

// DeleteCompareWithCMDBStep 清除实例对比 CMDB 快照步骤（清除专属：缺失/不一致均放行）
func DeleteCompareWithCMDBStep(
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

	logs.V(3).Infof("DeleteCompareWithCMDBStep: bizID: %d, processID: %d, processInstanceID: %d",
		bizID, processID, processInstanceID)

	ctf := cc.G().TaskFramework.ProcessDelete.CompareWithCMDBProcessInfo
	setp := types.NewStep(process.DeleteCompareWithCMDBStepName.String(),
		process.DeleteCompareWithCMDBStepName.String()).
		SetAlias("delete_compare_with_cmdb").
		SetMaxExecution(ctf.MaxExecution).
		SetMaxTries(ctf.MaxRetries)

	lo.Must0(setp.SetPayload(process.DeletePayload{
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

	return setp
}

// DeleteValidateOperateStep 校验清除进程实例操作步骤
func DeleteValidateOperateStep(
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

	logs.V(3).Infof("DeleteValidateOperateStep: bizID: %d, processID: %d, processInstanceID: %d",
		bizID, processID, processInstanceID)

	vtf := cc.G().TaskFramework.ProcessDelete.ValidateOperateProcess
	setp := types.NewStep(process.DeleteValidateOperateStepName.String(),
		process.DeleteValidateOperateStepName.String()).
		SetAlias("delete_validate_operate").
		SetMaxExecution(vtf.MaxExecution).
		SetMaxTries(vtf.MaxRetries)

	lo.Must0(setp.SetPayload(process.DeletePayload{
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

	return setp
}

// DeleteOperateStep 执行清除进程实例操作步骤
func DeleteOperateStep(
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

	otf := cc.G().TaskFramework.ProcessDelete.OperateProcess
	setp := types.NewStep(process.DeleteOperateStepName.String(),
		process.DeleteOperateStepName.String()).
		SetAlias("delete_operate_process").
		SetMaxExecution(otf.MaxExecution).
		SetMaxTries(otf.MaxRetries)

	lo.Must0(setp.SetPayload(process.DeletePayload{
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

	return setp
}

// DeleteFinalizeOperateStep 清除进程实例操作收尾步骤
func DeleteFinalizeOperateStep(
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

	ftf := cc.G().TaskFramework.ProcessDelete.FinalizeOperateProcess
	setp := types.NewStep(process.DeleteFinalizeOperateStepName.String(),
		process.DeleteFinalizeOperateStepName.String()).
		SetAlias("delete_finalize_operate").
		SetMaxExecution(ftf.MaxExecution).
		SetMaxTries(ftf.MaxRetries)

	lo.Must0(setp.SetPayload(process.DeletePayload{
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

	return setp
}
