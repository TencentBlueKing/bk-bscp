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
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"
	taskTypes "github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	pushmanager "github.com/TencentBlueKing/bk-bscp/internal/components/push_manager"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	gesprocessor "github.com/TencentBlueKing/bk-bscp/internal/processor/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbproc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
)

const (
	// CompareWithCMDBProcessInfoStepName 对比 DB 配置与 CMDB 最新配置步骤名
	CompareWithCMDBProcessInfoStepName istep.StepName = "CompareWithCMDBProcessInfo"

	// ValidateOperateProcessStepName validate operate process step name
	ValidateOperateProcessStepName istep.StepName = "ValidateOperateProcess"

	// OperateProcessStepName operate process step name
	OperateProcessStepName istep.StepName = "OperateProcess"
	// FinalizeOperateProcessStepName finalize operate process step name
	FinalizeOperateProcessStepName istep.StepName = "FinalizeOperateProcess"
	// RollbackProcessStepName rollback process step name
	RollbackProcessStepName istep.StepName = "RollbackProcess"
	// ProcessOperateCallbackName 进程操作回调名称
	ProcessOperateCallbackName istep.CallbackName = "ProcessOperateCallback"
)

// ProcessExecutor process step executor
// nolint: revive
type ProcessExecutor struct {
	*common.Executor
}

// NewProcessExecutor new process executor
func NewProcessExecutor(gseService *gse.Service, cmdbService bkcmdb.Service, pm pushmanager.Service,
	dao dao.Set) *ProcessExecutor {
	return &ProcessExecutor{
		Executor: &common.Executor{
			GseService:  gseService,
			CMDBService: cmdbService,
			Dao:         dao,
			PM:          pm,
			TaskConf:    cc.G().TaskFramework,
		},
	}
}

// OperatePayload 进程操作负载
type OperatePayload struct {
	TenantID string
	BizID    uint32
	// 任务批次ID，用于 Callback 更新批次状态
	BatchID           uint32
	OperateType       table.ProcessOperateType
	OperateUser       string
	ProcessID         uint32
	ProcessInstanceID uint32
	// OriginalProcManagedStatus 原进程托管状态，用于后续状态回滚
	OriginalProcManagedStatus table.ProcessManagedStatus
	// OriginalProcStatus 原进程状态，用于后续状态回滚
	OriginalProcStatus table.ProcessStatus
	// CCSyncStatus 进程 CC 同步状态（下发时刻快照），供状态类校验使用
	CCSyncStatus table.CCSyncStatus
}

// CompareWithCMDBProcessInfo 对比随任务下发的 DB 配置与 CMDB 最新配置快照
// （等价原 CompareWithCMDBProcessInfo 步骤，但免 CMDB 查询——两个配置已由下发 / 重试侧批量拉取后随任务带入）。
// 对比规则：
//   - 两者一致：直接放行，以当前配置执行操作
//   - 停止操作且 CMDB 最新快照缺失（进程已在 CMDB 删除或快照刷新降级）：忽略缺失，以 DB 配置执行停止操作
//   - 其他操作快照缺失、或两者配置不一致：对比失败直接报错
func (e *ProcessExecutor) CompareWithCMDBProcessInfo(c *istep.Context) error {
	logs.Infof("[CompareWithCMDBProcessInfo STEP]: starting compare")
	payload := &OperatePayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return err
	}

	if commonPayload.ProcessPayload == nil {
		return fmt.Errorf("[CompareWithCMDBProcessInfo STEP]: common process payload is nil")
	}
	proc := commonPayload.ProcessPayload

	var dbInfo, latestInfo table.ProcessInfo
	if err := json.Unmarshal([]byte(proc.ConfigData), &dbInfo); err != nil {
		return fmt.Errorf("[CompareWithCMDBProcessInfo STEP]: failed to unmarshal db process info: %w", err)
	}
	hasLatest := proc.LatestConfigData != ""
	if hasLatest {
		if err := json.Unmarshal([]byte(proc.LatestConfigData), &latestInfo); err != nil {
			return fmt.Errorf("[CompareWithCMDBProcessInfo STEP]: failed to unmarshal cmdb process info: %w", err)
		}
	}

	if err := compareExecConfig(payload.OperateType, dbInfo, latestInfo, hasLatest); err != nil {
		return fmt.Errorf("[CompareWithCMDBProcessInfo STEP]: %w", err)
	}
	return nil
}

// ValidateOperate 校验操作是否合法
// 属性矩阵对齐 gsekit 校验矩阵（必备命令 + 通用 3 项），并叠加状态类校验（状态未知 / ing 中间态 /
// syncStatus = Abnormal / Updated 特殊规则；状态取下发时刻 payload 快照）；
// 重复操作 / 状态漂移仍由 Operate 阶段的 GSE 幂等语义兜底（828/829 视为幂等成功）。
func (e *ProcessExecutor) ValidateOperate(c *istep.Context) error {
	logs.Infof("[ValidateOperate STEP]: starting validation")
	payload := &OperatePayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return err
	}

	if commonPayload.ProcessPayload == nil {
		return fmt.Errorf("[ValidateOperate STEP]: common process payload is nil")
	}

	// 解析执行配置（Compare 步骤放行后的 DB 配置）
	var processInfo table.ProcessInfo
	if err := json.Unmarshal([]byte(commonPayload.ProcessPayload.ConfigData), &processInfo); err != nil {
		return fmt.Errorf("[ValidateOperate STEP]: failed to unmarshal process info: %w", err)
	}

	// 校验操作是否合法（属性矩阵 + 状态类校验；状态取下发时刻 payload 快照，
	// 重复操作 / 状态漂移由 Operate 阶段 GSE 幂等兜底）
	if canOperate, message, _ := pbproc.CanProcessOperateByAttrs(
		payload.OperateType, processInfo,
		payload.OriginalProcStatus.String(),
		payload.OriginalProcManagedStatus.String(),
		payload.CCSyncStatus.String(),
	); !canOperate {
		return fmt.Errorf("process cannot operate, reason: %s", message)
	}

	return nil
}

// compareExecConfig 对比 DB 配置与 CMDB 最新配置，判定能否执行操作：
//   - 两者一致：放行（执行配置即 DB 配置）
//   - 停止操作且 CMDB 最新快照缺失：忽略缺失放行，以 DB 配置直接强停
//   - 其他操作快照缺失、或两者配置不一致：报错
func compareExecConfig(operateType table.ProcessOperateType, dbInfo, latestInfo table.ProcessInfo,
	hasLatest bool) error {

	if !hasLatest {
		if operateType == table.StopProcessOperate {
			return nil
		}
		return fmt.Errorf("process not found in CMDB, operateType: %s", operateType)
	}
	if !reflect.DeepEqual(dbInfo, latestInfo) {
		return fmt.Errorf("process config differs between db and cmdb, operateType: %s", operateType)
	}
	return nil
}

// Operate 进程操作
func (e *ProcessExecutor) Operate(c *istep.Context) error {
	payload := &OperatePayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("[Operate STEP]: get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[Operate STEP]: get common payload failed: %w", err)
	}

	proc := commonPayload.ProcessPayload
	logs.Infof("[Operate STEP]: start, biz_id=%d, batch_id=%d, operate_type=%s, "+
		"alias=%s, func_name=%s, agent_id=%s",
		payload.BizID, payload.BatchID, payload.OperateType,
		proc.Alias, proc.FuncName, proc.AgentID)

	// 解析进程配置信息
	var processInfo table.ProcessInfo
	err := json.Unmarshal([]byte(proc.ConfigData), &processInfo)
	if err != nil {
		return fmt.Errorf("[Operate STEP]: unmarshal process info failed: %w", err)
	}

	// 转换操作类型
	gseOpType, err := gse.ConvertProcessOperateTypeToOpType(payload.OperateType)
	if err != nil {
		return fmt.Errorf("[Operate STEP]: failed to convert operate type: %w", err)
	}

	// 构建进程操作接口请求参数
	params := gesprocessor.BuildProcessOperateParams{
		BizID:         payload.BizID,
		Alias:         proc.Alias,
		FuncName:      proc.FuncName,
		AgentID:       []string{proc.AgentID},
		GseOpType:     gseOpType,
		HostInstSeq:   proc.HostInstSeq,
		ModuleInstSeq: proc.ModuleInstSeq,
		SetName:       proc.SetName,
		ModuleName:    proc.ModuleName,
		ProcessInfo:   processInfo,
	}
	processOperate, err := gesprocessor.BuildProcessOperate(params)
	if err != nil {
		return fmt.Errorf("[Operate STEP]: failed to build process operate: %w", err)
	}

	req := &gse.MultiProcOperateReq{
		ProcOperateReq: []gse.ProcessOperate{*processOperate},
	}

	kt := kit.NewWithTenant(payload.TenantID)
	resp, err := e.GseService.OperateProcMulti(kt.Ctx, req)
	if err != nil {
		return fmt.Errorf("[Operate STEP]: OperateProcMulti failed: %w", err)
	}

	logs.Infof("[Operate STEP]: gse task created, task_id=%s", resp.TaskID)

	result, err := e.WaitProcOperateTaskFinish(kt.Ctx, resp.TaskID,
		payload.BizID, proc.HostInstSeq, proc.Alias, proc.AgentID)
	if err != nil {
		return fmt.Errorf("[Operate STEP]: wait task finish failed: %w", err)
	}

	// 构建 GSE 返回结果的 key
	key := gse.BuildResultKey(proc.AgentID, payload.BizID, proc.Alias, proc.HostInstSeq)
	procResult, ok := result[key]
	if !ok {
		return fmt.Errorf("[Operate STEP]: result not found for key: %s", key)
	}

	if !gse.IsSuccess(procResult.ErrorCode) {
		// 记录 GSE 执行结果，供查询侧转译展示状态
		commonPayload.GsePayload = &common.GsePayload{ErrorCode: procResult.ErrorCode, ErrorMsg: procResult.ErrorMsg}

		// 重复操作幂等成功：重复启动（828）/ 无需停止（829）即目标态已达成，
		// 任务按成功返回（跳过失败回滚），Finalize 正常把 GSE 真实状态写回实例，
		// 查询侧按 GsePayload.ErrorCode 转译为 IGNORED
		if IsIdempotentOperateError(procResult.ErrorCode) {
			if err = c.SetCommonPayload(commonPayload); err != nil {
				logs.Errorf("[Operate STEP]: failed to set common payload: %v", err)
				return fmt.Errorf("[Operate STEP]: failed to set common payload: %w", err)
			}
			logs.Infof("[Operate STEP]: duplicate operate treated as idempotent success, "+
				"errorCode=%d, errorMsg=%s", procResult.ErrorCode, procResult.ErrorMsg)
			return nil
		}

		// 其余错误码照旧失败
		if err = c.SetCommonPayload(commonPayload); err != nil {
			logs.Errorf("[Operate STEP]: failed to set common payload: %v", err)
		}
		return fmt.Errorf("[Operate STEP]: process operate failed, errorCode=%d, errorMsg=%s",
			procResult.ErrorCode, procResult.ErrorMsg)
	}

	logs.Infof("[Operate STEP]: success")
	return nil
}

// Finalize 进程操作完成
func (e *ProcessExecutor) Finalize(c *istep.Context) error {
	logs.Infof("Finalize: starting finalize")
	payload := &OperatePayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("[Finalize STEP]: get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[Finalize STEP]: get common payload failed: %w", err)
	}

	// 解析进程配置信息
	var processInfo table.ProcessInfo
	err := json.Unmarshal([]byte(commonPayload.ProcessPayload.ConfigData), &processInfo)
	if err != nil {
		return fmt.Errorf("[Finalize STEP]: unmarshal process info failed: %w", err)
	}

	// 获取gse侧进程状态
	processStatus, managedStatus, err := e.getGSEProcessStatus(c, payload.BizID)
	if err != nil {
		return fmt.Errorf("[Finalize STEP]: failed to get gse process status: %w", err)
	}

	// 更新进程实例状态字段
	m := e.Dao.GenQuery().ProcessInstance
	if err = e.Dao.ProcessInstance().UpdateSelectedFields(kit.NewWithTenant(payload.TenantID), payload.BizID, map[string]any{
		"status":            processStatus,
		"managed_status":    managedStatus,
		"status_updated_at": time.Now(),
	}, m.ID.Eq(payload.ProcessInstanceID)); err != nil {
		return fmt.Errorf("[Finalize STEP]: failed to update process instance: %w", err)
	}

	return nil
}

// pendingInstanceSnapshot 从任务的校验步骤中取出进程实例及其操作前状态。
// 普通操作任务与更新托管任务的校验步骤名和 payload 结构不同，需分别识别。
type pendingInstanceSnapshot struct {
	bizID           uint32
	instanceID      uint32
	originalStatus  table.ProcessStatus
	originalManaged table.ProcessManagedStatus
}

func getPendingInstanceSnapshot(t *taskTypes.Task) (*pendingInstanceSnapshot, error) {
	if step, ok := t.GetStep(ValidateOperateProcessStepName.String()); ok {
		payload := &OperatePayload{}
		if err := step.GetPayload(payload); err != nil {
			return nil, fmt.Errorf("get payload of task %s failed: %w", t.TaskID, err)
		}
		return &pendingInstanceSnapshot{
			bizID:           payload.BizID,
			instanceID:      payload.ProcessInstanceID,
			originalStatus:  payload.OriginalProcStatus,
			originalManaged: payload.OriginalProcManagedStatus,
		}, nil
	}

	if step, ok := t.GetStep(ValidateOperateStepName.String()); ok {
		payload := &UpdateRegisterPayload{}
		if err := step.GetPayload(payload); err != nil {
			return nil, fmt.Errorf("get payload of task %s failed: %w", t.TaskID, err)
		}
		return &pendingInstanceSnapshot{
			bizID:           payload.BizID,
			instanceID:      payload.ProcessInstanceID,
			originalStatus:  payload.OriginalProcStatus,
			originalManaged: payload.OriginalProcManagedStatus,
		}, nil
	}

	return nil, fmt.Errorf("task %s has no validate step", t.TaskID)
}

// RollbackPendingInstance 把尚未执行就被判失败的任务对应的进程实例恢复为操作前状态。
// 优先级级联阻断与批次下发失败补偿都会走到这里：这类任务一步都没执行过，也不会走 Callback，
// 实例会一直停在下发时写入的中间态（如 starting），而中间态会让该实例后续所有操作都被判为非法。
func RollbackPendingInstance(kt *kit.Kit, daoSet dao.Set, t *taskTypes.Task) error {
	if t == nil || daoSet == nil {
		return nil
	}

	snapshot, err := getPendingInstanceSnapshot(t)
	if err != nil {
		return err
	}
	if snapshot.instanceID == 0 {
		return nil
	}

	m := daoSet.GenQuery().ProcessInstance
	if err := daoSet.ProcessInstance().UpdateSelectedFields(kt, snapshot.bizID, map[string]any{
		"status":            snapshot.originalStatus,
		"managed_status":    snapshot.originalManaged,
		"status_updated_at": time.Now(),
	}, m.ID.Eq(snapshot.instanceID)); err != nil {
		return fmt.Errorf("restore process instance %d failed: %w", snapshot.instanceID, err)
	}

	logs.Infof("[PendingRollback]: restored process instance %d to status=%s managed=%s, taskID: %s",
		snapshot.instanceID, snapshot.originalStatus, snapshot.originalManaged, t.TaskID)
	return nil
}

// Callback 进程操作回调方法，在任务完成时被调用
// cbErr: 如果为 nil 表示任务成功，否则表示任务失败
func (e *ProcessExecutor) Callback(c *istep.Context, cbErr error) error {
	logs.Infof("[ProcessOperateCallback CALLBACK]: starting callback")
	var payload OperatePayload
	if err := c.GetPayload(&payload); err != nil {
		logs.Errorf("[ProcessOperateCallback CALLBACK]: failed to get payload: %v", err)
		return fmt.Errorf("failed to get payload: %w", err)
	}

	kt := kit.NewWithTenant(payload.TenantID)

	// 只累加批次进度用于展示。阶段推进、级联阻断以及批次终态的收敛
	// 都由任务框架的任务组编排负责，见 ProcessOperateGroupCallback。
	isSuccess := cbErr == nil
	if payload.BatchID > 0 {
		if _, err := e.Dao.TaskBatch().IncrementCompletedCount(kt, payload.BatchID, isSuccess); err != nil {
			logs.Errorf("[ProcessOperateCallback CALLBACK]: failed to increment completed count, "+
				"batchID: %d, err: %v", payload.BatchID, err)
			// PASS 继续执行，不影响回滚逻辑
		}
	}

	// 如果任务成功，不需要回滚
	if isSuccess {
		if payload.OperateType == table.UnregisterProcessOperate || payload.OperateType == table.StopProcessOperate {
			process, errP := e.Dao.Process().GetByID(kt, payload.BizID, payload.ProcessID)
			if errP != nil {
				return fmt.Errorf("[Finalize STEP]: failed to get process: %w", errP)
			}

			allInsts, errI := e.Dao.ProcessInstance().GetByProcessIDs(kt, payload.BizID, []uint32{payload.ProcessID})
			if errI != nil {
				return fmt.Errorf("[Finalize STEP]: failed to get process instance: %w", errI)
			}

			// 若进程数量被缩容，则删除对应的实例
			if process.Spec.ProcNum < uint(len(allInsts)) {
				if errD := e.Dao.ProcessInstance().Delete(kt, payload.BizID, payload.ProcessInstanceID); errD != nil {
					return fmt.Errorf("[Finalize STEP]: failed to delete process instance: %w", errD)
				}
			}
		}

		logs.Infof("[ProcessOperateCallback CALLBACK]: task %s completed successfully, no rollback needed",
			c.GetTaskID())

		return nil
	}

	// 任务失败，执行回滚逻辑
	logs.Infof("[ProcessOperateCallback CALLBACK]: task %s failed with error: %v, starting rollback",
		c.GetTaskID(), cbErr)

	// 进程操作失败，但是进程的部分状态可能在gse侧已经生效（如启动进程失败，但是进程实际上也会托管）
	// 优先使用gse侧进程状态，如果获取失败则回滚到原始状态
	processStatus, managedStatus, err := e.getGSEProcessStatus(c, payload.BizID)
	if err != nil {
		logs.Errorf("[ProcessOperateCallback CALLBACK]: failed to get gse process status: %v, "+
			"falling back to original status", err)
		// PASS
		processStatus = payload.OriginalProcStatus
		managedStatus = payload.OriginalProcManagedStatus
		logs.Infof("[ProcessOperateCallback CALLBACK]: rolling back to original status, bizID: %d, "+
			"processInstanceID: %d, status: %s, managedStatus: %s",
			payload.BizID, payload.ProcessInstanceID, processStatus, managedStatus)
	} else {
		logs.Infof("[ProcessOperateCallback CALLBACK]: using gse process status, bizID: %d, "+
			"processInstanceID: %d, status: %s, managedStatus: %s",
			payload.BizID, payload.ProcessInstanceID, processStatus, managedStatus)
	}

	// 更新进程实例状态
	m := e.Dao.GenQuery().ProcessInstance
	if err = e.Dao.ProcessInstance().UpdateSelectedFields(kt, payload.BizID, map[string]any{
		"status":            processStatus,
		"managed_status":    managedStatus,
		"status_updated_at": time.Now(),
	}, m.ID.Eq(payload.ProcessInstanceID)); err != nil {
		logs.Errorf("[ProcessOperateCallback CALLBACK]: failed to update process instance: %v", err)
		return fmt.Errorf("failed to update process instance during rollback: %w", err)
	}

	logs.Infof("[ProcessOperateCallback CALLBACK]: successfully rolled back process instance status, "+
		"bizID: %d, processInstanceID: %d", payload.BizID, payload.ProcessInstanceID)
	return nil
}

// 获取gse侧进程状态
func (e *ProcessExecutor) getGSEProcessStatus(
	c *istep.Context,
	bizID uint32,
) (table.ProcessStatus, table.ProcessManagedStatus, error) {
	payload := &OperatePayload{}
	if err := c.GetPayload(payload); err != nil {
		return "", "", fmt.Errorf("[getGSEProcessStatus STEP]: get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return "", "", fmt.Errorf("get common payload failed: %w", err)
	}
	// 查询进程信息
	process, err := e.Dao.Process().GetByID(kit.NewWithTenant(payload.TenantID), bizID, payload.ProcessID)
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
		return "", "", fmt.Errorf("failed to build process operate: %w", err)
	}
	req := &gse.MultiProcOperateReq{
		ProcOperateReq: []gse.ProcessOperate{*processOperate},
	}
	ktCtx := kit.NewWithTenant(payload.TenantID).Ctx
	resp, err := e.GseService.OperateProcMulti(ktCtx, req)
	if err != nil {
		return "", "", fmt.Errorf("failed to query process status via gseService.OperateProcMulti: %w", err)
	}
	result, err := e.WaitProcOperateTaskFinish(
		ktCtx,
		resp.TaskID,
		bizID,
		commonPayload.ProcessPayload.HostInstSeq,
		commonPayload.ProcessPayload.Alias,
		commonPayload.ProcessPayload.AgentID,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to wait for query task finish: %w", err)
	}
	key := gse.BuildResultKey(
		commonPayload.ProcessPayload.AgentID,
		bizID,
		commonPayload.ProcessPayload.Alias,
		commonPayload.ProcessPayload.HostInstSeq,
	)
	procResult, ok := result[key]
	if !ok {
		return "", "", fmt.Errorf("process result not found for key: %s", key)
	}
	if !gse.IsSuccess(procResult.ErrorCode) {
		return "", "", fmt.Errorf("failed to query process status, errorCode=%d, errorMsg=%s",
			procResult.ErrorCode, procResult.ErrorMsg)
	}
	var statusContent gse.ProcessStatusContent
	if err = json.Unmarshal([]byte(procResult.Content), &statusContent); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal process status content: %w", err)
	}
	if len(statusContent.Process) == 0 || len(statusContent.Process[0].Instance) == 0 {
		return "", "", fmt.Errorf("process not found in gse")
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

// IsIdempotentOperateError 判断 GSE 错误码是否为「进程已处于目标态」的幂等错误：
// 重复启动 -> 828（ErrCodeAlreadyRunning），无需停止 -> 829（ErrCodeNoNeedStop）
func IsIdempotentOperateError(errorCode int) bool {
	return gse.IsAlreadyRunning(errorCode) || gse.IsNoNeedStop(errorCode)
}

// RegisterExecutor register executor
func RegisterExecutor(e *ProcessExecutor) {
	// 对比 DB 配置与 CMDB 最新配置，选定执行配置
	istep.Register(CompareWithCMDBProcessInfoStepName, istep.StepExecutorFunc(e.CompareWithCMDBProcessInfo))
	// 校验操作是否合法
	istep.Register(ValidateOperateProcessStepName, istep.StepExecutorFunc(e.ValidateOperate))
	// 注册主要执行步骤
	istep.Register(OperateProcessStepName, istep.StepExecutorFunc(e.Operate))
	// 注册进程操作完成后的状态更新步骤
	istep.Register(FinalizeOperateProcessStepName, istep.StepExecutorFunc(e.Finalize))
	// 注册回调，用于任务失败时的状态回滚
	istep.RegisterCallback(ProcessOperateCallbackName, istep.CallbackExecutorFunc(e.Callback))
	// 任务组级回调：级联阻断后的实例状态回收与批次终态收敛
	istep.RegisterGroupCallback(ProcessOperateGroupCallbackName, NewGroupCallback(e))
}
