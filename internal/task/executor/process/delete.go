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
	"context"
	"encoding/json"
	"fmt"
	"time"

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	gesprocessor "github.com/TencentBlueKing/bk-bscp/internal/processor/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbproc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
)

const (
	// DeleteCompareWithCMDBStepName 清除实例对比 CMDB 快照步骤
	DeleteCompareWithCMDBStepName istep.StepName = "DeleteCompareWithCMDBStep"
	// DeleteValidateOperateStepName 清除实例校验步骤
	DeleteValidateOperateStepName istep.StepName = "DeleteValidateOperateStep"
	// DeleteOperateStepName 清除实例执行步骤
	DeleteOperateStepName istep.StepName = "DeleteOperateStep"
	// DeleteFinalizeOperateStepName 清除实例收尾步骤
	DeleteFinalizeOperateStepName istep.StepName = "DeleteFinalizeOperateStep"
	// DeleteCallbackName 清除实例回调
	DeleteCallbackName istep.CallbackName = "ProcessOperateDeleteCallback"
)

// DeleteExecutor 清除进程实例任务执行器。
// 清除链路完全独立于通用进程操作（start/stop/...）：拥有独立的 Payload、步骤实现与回调，
// 便于独立维护与排查——清除操作以 DB 配置为准，不需要与 CMDB 最新快照保持一致。
type DeleteExecutor struct {
	*common.Executor
}

// NewDeleteExecutor new delete executor
func NewDeleteExecutor(gseService *gse.Service, cmdbService bkcmdb.Service, dao dao.Set,
	redLock *lock.RedisLock) *DeleteExecutor {

	return &DeleteExecutor{
		Executor: &common.Executor{
			GseService:  gseService,
			CMDBService: cmdbService,
			Dao:         dao,
			RedLock:     redLock,
			TaskConf:    cc.G().TaskFramework,
		},
	}
}

// DeletePayload 清除进程实例任务负载
type DeletePayload struct {
	TenantID                  string
	BizID                     uint32
	BatchID                   uint32                   // 任务批次ID，用于 Callback 更新批次状态
	OperateType               table.ProcessOperateType // 拆解后的实际操作（停止 / 取消托管）
	OperateUser               string
	ProcessID                 uint32
	ProcessInstanceID         uint32
	OriginalProcManagedStatus table.ProcessManagedStatus // 原进程托管状态，用于失败回滚
	OriginalProcStatus        table.ProcessStatus        // 原进程状态，用于失败回滚
	// CCSyncStatus 进程 CC 同步状态（下发时刻快照），供状态类校验使用
	CCSyncStatus table.CCSyncStatus
}

// CompareWithCMDBDeleteStep 清除实例的 CMDB 快照对比（清除专属逻辑）。
// 与通用进程操作不同：清除操作以 DB 配置为准，目的只是停掉 / 取消托管进程，
// 因此 CMDB 快照缺失（进程已删除或刷新降级）或配置不一致均放行，仅记录差异供排查。
func (d *DeleteExecutor) CompareWithCMDBDeleteStep(c *istep.Context) error {
	logs.Infof("[DeleteCompareWithCMDBStep STEP]: starting compare with cmdb")
	payload := &DeletePayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("[DeleteCompareWithCMDBStep STEP]: get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[DeleteCompareWithCMDBStep STEP]: get common payload failed: %w", err)
	}

	if commonPayload.ProcessPayload == nil {
		return fmt.Errorf("[DeleteCompareWithCMDBStep STEP]: common process payload is nil")
	}

	var dbInfo table.ProcessInfo
	if err := json.Unmarshal([]byte(commonPayload.ProcessPayload.ConfigData), &dbInfo); err != nil {
		return fmt.Errorf("[DeleteCompareWithCMDBStep STEP]: failed to unmarshal db process info: %w", err)
	}

	// 清除专属：快照缺失放行（进程已在 CMDB 删除或下发时刷新降级），用 DB 配置执行清除
	if commonPayload.ProcessPayload.LatestConfigData == "" {
		logs.Infof("[DeleteCompareWithCMDBStep STEP]: latest cmdb config missing, "+
			"proceed with db config, bizID: %d, processID: %d, ccProcessID: %d",
			payload.BizID, payload.ProcessID, commonPayload.ProcessPayload.CcProcessID)
		return nil
	}

	var latestInfo table.ProcessInfo
	if err := json.Unmarshal([]byte(commonPayload.ProcessPayload.LatestConfigData), &latestInfo); err != nil {
		return fmt.Errorf("[DeleteCompareWithCMDBStep STEP]: failed to unmarshal cmdb process info: %w", err)
	}

	// 清除专属：配置不一致也放行（清除不依赖最新配置），仅记录日志
	if !processInfoChanged(dbInfo, latestInfo) {
		logs.Infof("[DeleteCompareWithCMDBStep STEP]: process config not changed, bizID: %d, processID: %d",
			payload.BizID, payload.ProcessID)
	} else {
		logs.Infof("[DeleteCompareWithCMDBStep STEP]: process config changed, proceed with db config, "+
			"bizID: %d, processID: %d", payload.BizID, payload.ProcessID)
	}

	return nil
}

// ValidateOperateDeleteStep 校验清除操作是否合法（属性矩阵 + 状态类校验，用下发时刻快照状态）
func (d *DeleteExecutor) ValidateOperateDeleteStep(c *istep.Context) error {
	logs.Infof("[DeleteValidateOperateStep STEP]: starting validate delete operate")
	payload := &DeletePayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("[DeleteValidateOperateStep STEP]: get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[DeleteValidateOperateStep STEP]: get common payload failed: %w", err)
	}

	if commonPayload.ProcessPayload == nil {
		return fmt.Errorf("[DeleteValidateOperateStep STEP]: common process payload is nil")
	}

	var dbInfo table.ProcessInfo
	if err := json.Unmarshal([]byte(commonPayload.ProcessPayload.ConfigData), &dbInfo); err != nil {
		return fmt.Errorf("[DeleteValidateOperateStep STEP]: failed to unmarshal db process info: %w", err)
	}

	// 验证清除操作（属性矩阵 + 状态类校验，针对 DB 配置；状态取下发时刻 payload 快照）
	canOperate, message, _ := pbproc.CanProcessOperateByAttrs(
		payload.OperateType,
		dbInfo,
		string(payload.OriginalProcStatus),
		string(payload.OriginalProcManagedStatus),
		payload.CCSyncStatus.String(),
	)
	if !canOperate {
		return fmt.Errorf("process cannot operate, reason: %s", message)
	}

	logs.Infof("[DeleteValidateOperateStep STEP]: validate done, bizID: %d, processID: %d",
		payload.BizID, payload.ProcessID)

	return nil
}

// OperateDeleteStep 执行清除操作：运行中则停止，已托管则取消托管（均用 DB 配置）
func (d *DeleteExecutor) OperateDeleteStep(c *istep.Context) error {
	logs.Infof("[DeleteOperateStep STEP]: starting delete operate")

	payload := &DeletePayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("[DeleteOperateStep STEP]: get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[DeleteOperateStep STEP]: get common payload failed: %w", err)
	}

	var dbInfo table.ProcessInfo
	if err := json.Unmarshal([]byte(commonPayload.ProcessPayload.ConfigData), &dbInfo); err != nil {
		return fmt.Errorf("[DeleteOperateStep STEP]: failed to unmarshal db process info: %w", err)
	}

	kt := kit.NewWithTenant(payload.TenantID)

	switch payload.OperateType {
	// 停止运行中的进程
	case table.StopProcessOperate:
		status, err := d.queryGSEProcessStatus(kt.Ctx, payload, commonPayload, dbInfo)
		if err != nil {
			return err
		}
		if !needStopProcess(status) {
			logs.Infof("[DeleteOperateStep STEP]: process not running, skip stop")
			return nil
		}
		if err = d.executeGSEOperate(kt.Ctx, payload, commonPayload, table.StopProcessOperate, dbInfo); err != nil {
			return fmt.Errorf("[DeleteOperateStep STEP]: execute process operate %s failed: %w",
				table.StopProcessOperate, err)
		}

	// 取消已托管的进程
	case table.UnregisterProcessOperate:
		status, err := d.queryGSEProcessStatus(kt.Ctx, payload, commonPayload, dbInfo)
		if err != nil {
			return err
		}
		if !needUnregisterProcess(status) {
			logs.Infof("[DeleteOperateStep STEP]: process not managed, skip unregister")
			return nil
		}
		if err = d.executeGSEOperate(kt.Ctx, payload, commonPayload, table.UnregisterProcessOperate, dbInfo); err != nil {
			return fmt.Errorf("[DeleteOperateStep STEP]: execute process operate %s failed: %w",
				table.UnregisterProcessOperate, err)
		}

	default:
		return fmt.Errorf("[DeleteOperateStep STEP]: unsupported operate type for delete: %s", payload.OperateType)
	}

	return nil
}

// FinalizeOperateDeleteStep 清除操作完成，查 GSE 真实状态收敛进程实例状态
func (d *DeleteExecutor) FinalizeOperateDeleteStep(c *istep.Context) error {
	logs.Infof("[DeleteFinalizeOperateStep STEP]: starting finalize delete operate")
	payload := &DeletePayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("[DeleteFinalizeOperateStep STEP]: get payload failed: %w", err)
	}

	// 获取gse侧进程状态
	processStatus, managedStatus, err := d.getGSEProcessStatus(c, payload.BizID)
	if err != nil {
		return fmt.Errorf("[DeleteFinalizeOperateStep STEP]: failed to get gse process status: %w", err)
	}

	// 更新进程实例状态字段
	m := d.Dao.GenQuery().ProcessInstance
	if err = d.Dao.ProcessInstance().UpdateSelectedFields(kit.NewWithTenant(payload.TenantID), payload.BizID,
		map[string]any{
			"status":            processStatus,
			"managed_status":    managedStatus,
			"status_updated_at": time.Now(),
		}, m.ID.Eq(payload.ProcessInstanceID)); err != nil {
		return fmt.Errorf("[DeleteFinalizeOperateStep STEP]: failed to update process instance: %w", err)
	}

	return nil
}

// Callback 清除实例任务回调方法，在任务完成时被调用
// cbErr: 如果为 nil 表示任务成功，否则表示任务失败
func (d *DeleteExecutor) Callback(c *istep.Context, cbErr error) error {
	logs.Infof("[DeleteCallback CALLBACK]: starting callback")
	var payload DeletePayload
	if err := c.GetPayload(&payload); err != nil {
		logs.Errorf("[DeleteCallback CALLBACK]: failed to get payload: %v", err)
		return fmt.Errorf("failed to get payload: %w", err)
	}

	kt := kit.NewWithTenant(payload.TenantID)

	// 只累加批次进度用于展示，批次终态由任务组回调统一收敛
	isSuccess := cbErr == nil
	if payload.BatchID > 0 {
		if _, err := d.Dao.TaskBatch().IncrementCompletedCount(kt, payload.BatchID, isSuccess); err != nil {
			logs.Errorf("[DeleteCallback CALLBACK]: failed to increment completed count, "+
				"batchID: %d, err: %v", payload.BatchID, err)
		}
	}

	if isSuccess {
		logs.Infof("[DeleteCallback CALLBACK]: task %s completed successfully, no rollback needed", c.GetTaskID())

		// 清除成功后删除缩容实例记录；任务执行期间 CMDB 可能扩容导致 proc_num 变化，
		// 删除前重新校验缩容状态，非缩容实例保留记录（由 CMDB 同步管理）
		if err := d.deleteScaledDownInstance(kt, &payload); err != nil {
			logs.Errorf("[DeleteCallback CALLBACK]: failed to delete scaled-down instance, "+
				"bizID: %d, processInstanceID: %d, err: %v", payload.BizID, payload.ProcessInstanceID, err)
			return err
		}
		return nil
	}

	// 清除失败时优先使用 gse 侧真实状态（操作可能已部分生效），获取失败则回滚到原始状态
	processStatus, managedStatus, err := d.getGSEProcessStatus(c, payload.BizID)
	if err != nil {
		logs.Errorf("[DeleteCallback CALLBACK]: failed to get gse process status: %v, "+
			"falling back to original status", err)
		processStatus = payload.OriginalProcStatus
		managedStatus = payload.OriginalProcManagedStatus
		logs.Infof("[DeleteCallback CALLBACK]: rolling back to original status, bizID: %d, "+
			"processInstanceID: %d, status: %s, managedStatus: %s",
			payload.BizID, payload.ProcessInstanceID, processStatus, managedStatus)
	} else {
		logs.Infof("[DeleteCallback CALLBACK]: using gse process status, bizID: %d, "+
			"processInstanceID: %d, status: %s, managedStatus: %s",
			payload.BizID, payload.ProcessInstanceID, processStatus, managedStatus)
	}

	// 更新进程实例状态
	m := d.Dao.GenQuery().ProcessInstance
	if err = d.Dao.ProcessInstance().UpdateSelectedFields(kit.NewWithTenant(payload.TenantID), payload.BizID,
		map[string]any{
			"status":            processStatus,
			"managed_status":    managedStatus,
			"status_updated_at": time.Now(),
		}, m.ID.Eq(payload.ProcessInstanceID)); err != nil {
		logs.Errorf("[DeleteCallback CALLBACK]: failed to update process instance: %v", err)
		return fmt.Errorf("failed to update process instance during rollback: %w", err)
	}

	logs.Infof("[DeleteCallback CALLBACK]: successfully rolled back process instance status, "+
		"bizID: %d, processInstanceID: %d", payload.BizID, payload.ProcessInstanceID)

	return nil
}

// deleteScaledDownInstance 清除成功后删除缩容实例记录：
// 进程实例按 host_inst_seq 升序排列后，序位超过 proc_num 的实例为缩容实例（与前端"一键清除缩容实例"口径一致）；
// proc_num 与实例数量一致时无缩容，实例不能被删除。目标实例已被并发清除时视为完成。
func (d *DeleteExecutor) deleteScaledDownInstance(kt *kit.Kit, payload *DeletePayload) error {
	proc, err := d.Dao.Process().GetByID(kt, payload.BizID, payload.ProcessID)
	if err != nil {
		return fmt.Errorf("failed to get process: %w", err)
	}

	// GetByProcessIDs 已按 host_inst_seq 升序返回
	insts, err := d.Dao.ProcessInstance().GetByProcessIDs(kt, payload.BizID, []uint32{payload.ProcessID})
	if err != nil {
		return fmt.Errorf("failed to list process instances: %w", err)
	}

	targetIdx := -1
	for idx, inst := range insts {
		if inst.ID == payload.ProcessInstanceID {
			targetIdx = idx
			break
		}
	}
	if targetIdx < 0 {
		logs.Infof("[DeleteCallback CALLBACK]: process instance already deleted, "+
			"bizID: %d, processInstanceID: %d", payload.BizID, payload.ProcessInstanceID)
		return nil
	}

	// 序位（下标+1）未超过 proc_num 时为非缩容实例，保留记录
	if uint32(targetIdx+1) <= uint32(proc.Spec.ProcNum) {
		logs.Infof("[DeleteCallback CALLBACK]: process instance is not a scaled-down instance, keep it, "+
			"bizID: %d, processID: %d, processInstanceID: %d, procNum: %d",
			payload.BizID, payload.ProcessID, payload.ProcessInstanceID, proc.Spec.ProcNum)
		return nil
	}

	if err := d.Dao.ProcessInstance().Delete(kt, payload.BizID, payload.ProcessInstanceID); err != nil {
		return fmt.Errorf("failed to delete scaled-down process instance %d: %w", payload.ProcessInstanceID, err)
	}

	logs.Infof("[DeleteCallback CALLBACK]: scaled-down process instance deleted, bizID: %d, "+
		"processID: %d, processInstanceID: %d", payload.BizID, payload.ProcessID, payload.ProcessInstanceID)
	return nil
}

// needUnregisterProcess 判断进程是否已托管（需要取消托管）
func needUnregisterProcess(status *gse.ProcessStatusContent) bool {
	if status == nil {
		return false
	}

	for _, proc := range status.Process {
		for _, inst := range proc.Instance {
			if inst.IsAuto {
				return true
			}
		}
	}
	return false
}

// getGSEProcessStatus 查询 gse 侧进程运行状态与托管状态
func (d *DeleteExecutor) getGSEProcessStatus(
	c *istep.Context,
	bizID uint32,
) (table.ProcessStatus, table.ProcessManagedStatus, error) {
	payload := &DeletePayload{}
	if err := c.GetPayload(payload); err != nil {
		return "", "", fmt.Errorf("[getGSEProcessStatus STEP]: get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return "", "", fmt.Errorf("[getGSEProcessStatus STEP]: get common payload failed: %w", err)
	}
	// 查询进程信息
	process, err := d.Dao.Process().GetByID(kit.NewWithTenant(payload.TenantID), bizID, payload.ProcessID)
	if err != nil {
		return "", "", fmt.Errorf("[getGSEProcessStatus STEP]: failed to get process: %w", err)
	}
	// 获取进程配置信息
	var processInfo table.ProcessInfo
	err = json.Unmarshal([]byte(process.Spec.SourceData), &processInfo)
	if err != nil {
		return "", "", fmt.Errorf("[getGSEProcessStatus STEP]: failed to marshal process info: %w", err)
	}
	params := gesprocessor.BuildProcessOperateParams{
		BizID:         bizID,
		Alias:         commonPayload.ProcessPayload.Alias,
		FuncName:      commonPayload.ProcessPayload.FuncName,
		HostInstSeq:   commonPayload.ProcessPayload.HostInstSeq,
		ModuleInstSeq: commonPayload.ProcessPayload.ModuleInstSeq,
		SetName:       commonPayload.ProcessPayload.SetName,
		ModuleName:    commonPayload.ProcessPayload.ModuleName,
		AgentID:       []string{commonPayload.ProcessPayload.AgentID},
		GseOpType:     gse.OpTypeQuery,
		ProcessInfo:   processInfo,
	}
	processOperate, err := gesprocessor.BuildProcessOperate(params)
	if err != nil {
		return "", "", fmt.Errorf("[getGSEProcessStatus STEP]: failed to build process operate: %w", err)
	}
	req := &gse.MultiProcOperateReq{
		ProcOperateReq: []gse.ProcessOperate{*processOperate},
	}
	ktCtx := kit.NewWithTenant(payload.TenantID).Ctx
	resp, err := d.GseService.OperateProcMulti(ktCtx, req)
	if err != nil {
		return "", "",
			fmt.Errorf("[getGSEProcessStatus STEP]: failed to query process status via gseService.OperateProcMulti: %w", err)
	}
	result, err := d.WaitProcOperateTaskFinish(
		ktCtx,
		resp.TaskID,
		bizID,
		commonPayload.ProcessPayload.HostInstSeq,
		commonPayload.ProcessPayload.Alias,
		commonPayload.ProcessPayload.AgentID,
	)
	if err != nil {
		return "", "", fmt.Errorf("[getGSEProcessStatus STEP]: failed to wait for query task finish: %w", err)
	}
	key := gse.BuildResultKey(
		commonPayload.ProcessPayload.AgentID,
		bizID,
		commonPayload.ProcessPayload.Alias,
		commonPayload.ProcessPayload.HostInstSeq,
	)
	procResult, ok := result[key]
	if !ok {
		return "", "", fmt.Errorf("[getGSEProcessStatus STEP]: process result not found for key: %s", key)
	}
	if !gse.IsSuccess(procResult.ErrorCode) {
		return "", "", fmt.Errorf("[getGSEProcessStatus STEP]: failed to query process status, errorCode=%d, errorMsg=%s",
			procResult.ErrorCode, procResult.ErrorMsg)
	}
	var statusContent gse.ProcessStatusContent
	if err = json.Unmarshal([]byte(procResult.Content), &statusContent); err != nil {
		return "", "", fmt.Errorf("[getGSEProcessStatus STEP]: failed to unmarshal process status content: %w", err)
	}
	if len(statusContent.Process) == 0 || len(statusContent.Process[0].Instance) == 0 {
		return "", "", fmt.Errorf("[getGSEProcessStatus STEP]: process not found in gse")
	}
	instance := statusContent.Process[0].Instance[0]
	processStatus := table.ProcessStatusStopped
	managedStatus := table.ProcessManagedStatusUnmanaged
	if instance.IsAuto {
		managedStatus = table.ProcessManagedStatusManaged
	}
	if instance.PID > 0 {
		processStatus = table.ProcessStatusRunning
	}
	return processStatus, managedStatus, nil
}

// queryGSEProcessStatus 查询 GSE 状态
func (d *DeleteExecutor) queryGSEProcessStatus(ctx context.Context, payload *DeletePayload,
	commonPayload *common.TaskPayload, processInfo table.ProcessInfo) (*gse.ProcessStatusContent, error) {

	params := gesprocessor.BuildProcessOperateParams{
		BizID:         payload.BizID,
		Alias:         commonPayload.ProcessPayload.Alias,
		FuncName:      commonPayload.ProcessPayload.FuncName,
		AgentID:       []string{commonPayload.ProcessPayload.AgentID},
		GseOpType:     gse.OpTypeQuery,
		HostInstSeq:   commonPayload.ProcessPayload.HostInstSeq,
		ModuleInstSeq: commonPayload.ProcessPayload.ModuleInstSeq,
		SetName:       commonPayload.ProcessPayload.SetName,
		ModuleName:    commonPayload.ProcessPayload.ModuleName,
		ProcessInfo:   processInfo,
	}

	operate, err := gesprocessor.BuildProcessOperate(params)
	if err != nil {
		return nil, err
	}

	resp, err := d.GseService.OperateProcMulti(ctx, &gse.MultiProcOperateReq{
		ProcOperateReq: []gse.ProcessOperate{*operate},
	})
	if err != nil {
		return nil, err
	}

	result, err := d.WaitProcOperateTaskFinish(
		ctx,
		resp.TaskID,
		payload.BizID,
		commonPayload.ProcessPayload.HostInstSeq,
		commonPayload.ProcessPayload.Alias,
		commonPayload.ProcessPayload.AgentID,
	)
	if err != nil {
		return nil, err
	}

	key := gse.BuildResultKey(
		commonPayload.ProcessPayload.AgentID,
		payload.BizID,
		commonPayload.ProcessPayload.Alias,
		commonPayload.ProcessPayload.HostInstSeq,
	)

	procResult, ok := result[key]
	if !ok {
		return nil, fmt.Errorf("query result not found, key=%s", key)
	}

	if !gse.IsSuccess(procResult.ErrorCode) {
		return nil, fmt.Errorf("query gse failed, code=%d, msg=%s",
			procResult.ErrorCode, procResult.ErrorMsg)
	}

	var status gse.ProcessStatusContent
	if err := json.Unmarshal([]byte(procResult.Content), &status); err != nil {
		return nil, err
	}

	return &status, nil
}

// executeGSEOperate 执行 gse 操作
func (d *DeleteExecutor) executeGSEOperate(ctx context.Context, payload *DeletePayload,
	commonPayload *common.TaskPayload, op table.ProcessOperateType, processInfo table.ProcessInfo) error {

	gseOpType, err := gse.ConvertProcessOperateTypeToOpType(op)
	if err != nil {
		return err
	}

	params := gesprocessor.BuildProcessOperateParams{
		BizID:         payload.BizID,
		Alias:         commonPayload.ProcessPayload.Alias,
		FuncName:      commonPayload.ProcessPayload.FuncName,
		AgentID:       []string{commonPayload.ProcessPayload.AgentID},
		GseOpType:     gseOpType,
		HostInstSeq:   commonPayload.ProcessPayload.HostInstSeq,
		ModuleInstSeq: commonPayload.ProcessPayload.ModuleInstSeq,
		SetName:       commonPayload.ProcessPayload.SetName,
		ModuleName:    commonPayload.ProcessPayload.ModuleName,
		ProcessInfo:   processInfo,
	}

	operate, err := gesprocessor.BuildProcessOperate(params)
	if err != nil {
		return err
	}

	resp, err := d.GseService.OperateProcMulti(ctx, &gse.MultiProcOperateReq{
		ProcOperateReq: []gse.ProcessOperate{*operate},
	})
	if err != nil {
		return err
	}

	result, err := d.WaitProcOperateTaskFinish(
		ctx,
		resp.TaskID,
		payload.BizID,
		commonPayload.ProcessPayload.HostInstSeq,
		commonPayload.ProcessPayload.Alias,
		commonPayload.ProcessPayload.AgentID,
	)
	if err != nil {
		return err
	}

	key := gse.BuildResultKey(
		commonPayload.ProcessPayload.AgentID,
		payload.BizID,
		commonPayload.ProcessPayload.Alias,
		commonPayload.ProcessPayload.HostInstSeq,
	)

	procResult, ok := result[key]
	if !ok {
		return fmt.Errorf("process result not found, key=%s", key)
	}

	if !gse.IsSuccess(procResult.ErrorCode) {
		return fmt.Errorf("gse operate failed, code=%d, msg=%s", procResult.ErrorCode, procResult.ErrorMsg)
	}

	return nil
}

// RegisterDeleteExecutor register delete executor
func RegisterDeleteExecutor(e *DeleteExecutor) {
	istep.Register(DeleteCompareWithCMDBStepName, istep.StepExecutorFunc(e.CompareWithCMDBDeleteStep))
	istep.Register(DeleteValidateOperateStepName, istep.StepExecutorFunc(e.ValidateOperateDeleteStep))
	istep.Register(DeleteOperateStepName, istep.StepExecutorFunc(e.OperateDeleteStep))
	istep.Register(DeleteFinalizeOperateStepName, istep.StepExecutorFunc(e.FinalizeOperateDeleteStep))
	// 注册清除实例任务回调，用于任务失败时的状态回滚
	istep.RegisterCallback(DeleteCallbackName, istep.CallbackExecutorFunc(e.Callback))
}
