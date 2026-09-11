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
	"errors"
	"fmt"
	"reflect"
	"time"

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"

	"github.com/TencentBlueKing/bk-bscp/cmd/cache-service/service/cache/keys"
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
	// ValidateOperateStepName 验证操作
	ValidateOperateStepName istep.StepName = "ValidateOperateStep"
	// StopProcessStepName 停止进程
	StopProcessStepName istep.StepName = "StopProcessStep"
	// RegisterProcessStepName 托管进程
	RegisterProcessStepName istep.StepName = "RegisterProcessStep"
	// StartProcessStepName 启动进程
	StartProcessStepName istep.StepName = "StartProcessStep"
	// ProcessOperationCompletedStepName 进程操作完成
	OperationCompletedStepName istep.StepName = "OperationCompletedStep"
	// UpdateRegisterCallbackName 更新托管回调
	UpdateRegisterCallbackName istep.CallbackName = "UpdateRegisterCallback"
)

// UpdateRegisterExecutor update register executor
type UpdateRegisterExecutor struct {
	*common.Executor
}

// ErrRegisterProcessStepFailed 注册进程步骤失败
var ErrRegisterProcessStepFailed = errors.New("register process step failed")

// ErrStartProcessStepFailed 启动进程步骤失败（此时托管信息已更新）
var ErrStartProcessStepFailed = errors.New("start process step failed")

// ErrOperationCompletedStepFailed 操作完成步骤失败（此时托管信息已更新）
var ErrOperationCompletedStepFailed = errors.New("operation completed step failed")

// NewUpdateRegisterExecutor new update register executor
func NewUpdateRegisterExecutor(gseService *gse.Service, cmdbService bkcmdb.Service, dao dao.Set,
	redLock *lock.RedisLock) *UpdateRegisterExecutor {

	return &UpdateRegisterExecutor{
		Executor: &common.Executor{
			GseService:  gseService,
			CMDBService: cmdbService,
			Dao:         dao,
			RedLock:     redLock,
			TaskConf:    cc.G().TaskFramework,
		},
	}
}

// UpdateRegisterPayload 进程操作负载
type UpdateRegisterPayload struct {
	TenantID                  string
	BizID                     uint32
	BatchID                   uint32 // 任务批次ID，用于 Callback 更新批次状态
	OperateType               table.ProcessOperateType
	OperateUser               string
	ProcessID                 uint32
	ProcessInstanceID         uint32
	OriginalProcManagedStatus table.ProcessManagedStatus // 原进程托管状态，用于后续状态回滚
	OriginalProcStatus        table.ProcessStatus        // 原进程状态，用于后续状态回滚
	EnableProcessRestart      bool
	// CCSyncStatus 进程 CC 同步状态（下发时刻快照），供状态类校验使用
	CCSyncStatus table.CCSyncStatus
}

// ValidateOperateStep 校验操作是否合法（快照对比 + 校验合一，任务内零 CMDB 查询）
// 对比随任务下发的 DB 配置（ConfigData，即 DB source_data）与下发时刻 CMDB 最新快照（LatestConfigData）：
//   - 快照缺失（进程已在 CMDB 删除或下发时刷新降级）：无法比对直接报错，
//     进程删除的落库标记由 CMDB 同步主路径兜底
//   - 两者一致：无需执行任何 GSE 操作，后续 Stop / Register / Start 步骤自判断跳过，
//     由 OperationCompletedStep 收敛实例状态
//   - 两者不一致：Stop 用旧配置的命令停旧进程，Register / Start 用新配置托管并拉起
func (u *UpdateRegisterExecutor) ValidateOperateStep(c *istep.Context) error {
	logs.Infof("[ValidateOperateStep STEP]: starting validate operate")
	payload := &UpdateRegisterPayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return err
	}

	if commonPayload.ProcessPayload == nil {
		return fmt.Errorf("[ValidateOperateStep STEP]: common process payload is nil")
	}

	dbInfo, latestInfo, err := parseProcessConfigs(commonPayload)
	if err != nil {
		return fmt.Errorf("[ValidateOperateStep STEP]: %w", err)
	}

	// 检测是否拥有启停命令：停止旧进程依赖旧配置的 stop_cmd，启动新进程依赖新配置的 start_cmd
	if payload.EnableProcessRestart {
		if !pbproc.HasOperateCommand(table.StopProcessOperate, dbInfo) {
			return fmt.Errorf("the stop command does not exist")
		}
		if !pbproc.HasOperateCommand(table.StartProcessOperate, latestInfo) {
			return fmt.Errorf("the start command does not exist")
		}
	}

	// 验证更新托管操作（属性矩阵 + 状态类校验，针对将注册进 GSE 的 CMDB 最新配置；
	// 状态取下发时刻 payload 快照）
	canOperate, message, _ := pbproc.CanProcessOperateByAttrs(
		payload.OperateType,
		latestInfo,
		string(payload.OriginalProcStatus),
		string(payload.OriginalProcManagedStatus),
		payload.CCSyncStatus.String(),
	)
	if !canOperate {
		return fmt.Errorf("process cannot operate, reason: %s", message)
	}

	logs.Infof("[ValidateOperateStep STEP]: validate done, bizID: %d, processID: %d, "+
		"config changed: %t", payload.BizID, payload.ProcessID, processInfoChanged(dbInfo, latestInfo))

	return nil
}

// parseProcessConfigs 解析随任务下发的 DB 配置（ConfigData）与下发时刻 CMDB 最新快照（LatestConfigData）。
// LatestConfigData 为空表示进程已在 CMDB 删除或下发时快照刷新降级：更新托管无法与最新配置比对，
// 直接报错（对齐普通进程操作「除停止外快照缺失即报错」的语义）
func parseProcessConfigs(commonPayload *common.TaskPayload) (dbInfo, latestInfo table.ProcessInfo, err error) {
	proc := commonPayload.ProcessPayload
	if err = json.Unmarshal([]byte(proc.ConfigData), &dbInfo); err != nil {
		return dbInfo, latestInfo, fmt.Errorf("failed to unmarshal db process info: %w", err)
	}
	if proc.LatestConfigData == "" {
		return dbInfo, latestInfo, fmt.Errorf("process not found in cmdb, ccProcessID: %d", proc.CcProcessID)
	}
	if err = json.Unmarshal([]byte(proc.LatestConfigData), &latestInfo); err != nil {
		return dbInfo, latestInfo, fmt.Errorf("failed to unmarshal cmdb process info: %w", err)
	}
	return dbInfo, latestInfo, nil
}

// processInfoChanged 对比 DB 配置与 CMDB 最新快照是否不一致（不一致才需要执行托管更新）
func processInfoChanged(dbInfo, latestInfo table.ProcessInfo) bool {
	return !reflect.DeepEqual(dbInfo, latestInfo)
}

// StopProcessStep 用旧配置的停止命令停止旧进程（仅在 DB 配置与 CMDB 快照不一致时执行）
func (u *UpdateRegisterExecutor) StopProcessStep(c *istep.Context) error {
	logs.Infof("[StopProcessStep STEP]: starting stop process")

	payload := &UpdateRegisterPayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("[StopProcessStep STEP]: get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[StopProcessStep STEP]: get common payload failed: %w", err)
	}

	dbInfo, latestInfo, err := parseProcessConfigs(commonPayload)
	if err != nil {
		return fmt.Errorf("[StopProcessStep STEP]: %w", err)
	}

	// 配置一致时无需更新托管，跳过停止（由 OperationCompletedStep 收敛实例状态）
	if !processInfoChanged(dbInfo, latestInfo) {
		logs.Infof("[StopProcessStep STEP]: process config not changed, skip stop")
		return nil
	}

	// 1. 查询gse
	kt := kit.NewWithTenant(payload.TenantID)
	status, err := u.queryGSEProcessStatus(kt.Ctx, payload, commonPayload, dbInfo)
	if err != nil {
		return err
	}

	if !needStopProcess(status) {
		logs.Infof("[StopProcessStep STEP]: process not running, skip stop")
		return nil
	}

	// 2. 用旧配置的停止命令停止旧进程
	if err = u.executeGSEOperate(kt.Ctx, payload, commonPayload, table.StopProcessOperate, dbInfo); err != nil {
		return fmt.Errorf(
			"[StopProcessStep STEP]: execute process operate %s failed: %w",
			table.StopProcessOperate,
			err,
		)
	}

	return nil
}

func needStopProcess(status *gse.ProcessStatusContent) bool {
	if status == nil {
		return false
	}

	for _, proc := range status.Process {
		for _, inst := range proc.Instance {
			if inst.PID > 0 {
				return true
			}
		}
	}
	return false
}

// RegisterProcessStep 用 CMDB 最新配置重新托管进程（仅在配置不一致时执行）
func (u *UpdateRegisterExecutor) RegisterProcessStep(c *istep.Context) error {
	logs.Infof("[RegisterProcessStep STEP]: starting register process")

	payload := &UpdateRegisterPayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("[RegisterProcessStep STEP]: get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[RegisterProcessStep STEP]: get common payload failed: %w", err)
	}

	dbInfo, latestInfo, err := parseProcessConfigs(commonPayload)
	if err != nil {
		return fmt.Errorf("[RegisterProcessStep STEP]: %w", err)
	}

	// 配置一致时无需更新托管，跳过注册（由 OperationCompletedStep 收敛实例状态）
	if !processInfoChanged(dbInfo, latestInfo) {
		logs.Infof("[RegisterProcessStep STEP]: process config not changed, skip register")
		return nil
	}

	// 用 CMDB 最新快照配置注册托管信息
	if err := u.executeGSEOperate(
		kit.NewWithTenant(payload.TenantID).Ctx,
		payload,
		commonPayload,
		table.RegisterProcessOperate,
		latestInfo,
	); err != nil {
		return fmt.Errorf("%w: %v", ErrRegisterProcessStepFailed, err)
	}

	return nil
}

// StartProcessStep 用新配置的启动命令启动进程（仅在配置不一致时执行）
func (u *UpdateRegisterExecutor) StartProcessStep(c *istep.Context) error {
	logs.Infof("[StartProcessStep STEP]: starting start process")

	payload := &UpdateRegisterPayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("[StartProcessStep STEP]: get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[StartProcessStep STEP]: get common payload failed: %w", err)
	}

	dbInfo, latestInfo, err := parseProcessConfigs(commonPayload)
	if err != nil {
		return fmt.Errorf("[StartProcessStep STEP]: %w", err)
	}

	// 配置一致时无需更新托管，跳过启动（由 OperationCompletedStep 收敛实例状态）
	if !processInfoChanged(dbInfo, latestInfo) {
		logs.Infof("[StartProcessStep STEP]: process config not changed, skip start")
		return nil
	}

	// 用 CMDB 最新快照配置启动新进程
	if err := u.executeGSEOperate(
		kit.NewWithTenant(payload.TenantID).Ctx,
		payload,
		commonPayload,
		table.StartProcessOperate,
		latestInfo,
	); err != nil {
		return fmt.Errorf("%w: %v", ErrStartProcessStepFailed, err)
	}

	return nil
}

// OperationCompletedStep 进程操作完成，收敛进程实例状态
func (u *UpdateRegisterExecutor) OperationCompletedStep(c *istep.Context) error {
	logs.Infof("[OperationCompletedStep STEP]: starting process operation completed")
	payload := &UpdateRegisterPayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("[OperationCompletedStep STEP]: get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[OperationCompletedStep STEP]: get common payload failed: %w", err)
	}

	dbInfo, latestInfo, err := parseProcessConfigs(commonPayload)
	if err != nil {
		return fmt.Errorf("[OperationCompletedStep STEP]: %w", err)
	}

	var processStatus table.ProcessStatus
	var managedStatus table.ProcessManagedStatus
	if processInfoChanged(dbInfo, latestInfo) {
		// 执行过托管更新，以 gse 侧真实状态为准
		processStatus, managedStatus, err = u.getGSEProcessStatus(c, payload.BizID)
		if err != nil {
			// 走到这里说明注册步骤已成功执行、托管信息已在 gse 侧生效，
			// 收尾失败也必须按「注册后失败」分类：registerProcessSuccessDelta
			// 依赖该错误码计入成功数，触发回调收敛 prev_data / source_data / cc_sync_status，
			// 否则进程会一直被 Updated 阻断正常操作，直到某次重试收尾成功
			return fmt.Errorf("[OperationCompletedStep STEP]: %w: get gse process status failed: %v",
				ErrOperationCompletedStepFailed, err)
		}
	} else {
		// 配置一致未执行任何 GSE 操作，实例恢复为操作前状态即可（也规避未注册进程查 GSE 报错）
		processStatus = payload.OriginalProcStatus
		managedStatus = payload.OriginalProcManagedStatus
	}

	// 更新进程实例状态字段
	m := u.Dao.GenQuery().ProcessInstance
	if err = u.Dao.ProcessInstance().UpdateSelectedFields(kit.NewWithTenant(payload.TenantID), payload.BizID, map[string]any{
		"status":            processStatus,
		"managed_status":    managedStatus,
		"status_updated_at": time.Now(),
	}, m.ID.Eq(payload.ProcessInstanceID)); err != nil {
		return fmt.Errorf("%w: %v", ErrOperationCompletedStepFailed, err)
	}

	return nil
}

// 获取gse侧进程状态
func (u *UpdateRegisterExecutor) getGSEProcessStatus(
	c *istep.Context,
	bizID uint32,
) (table.ProcessStatus, table.ProcessManagedStatus, error) {
	payload := &UpdateRegisterPayload{}
	if err := c.GetPayload(payload); err != nil {
		return "", "", fmt.Errorf("[getGSEProcessStatus STEP]: get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return "", "", fmt.Errorf("[getGSEProcessStatus STEP]: get common payload failed: %w", err)
	}
	// 查询进程信息
	process, err := u.Dao.Process().GetByID(kit.NewWithTenant(payload.TenantID), bizID, payload.ProcessID)
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
	resp, err := u.GseService.OperateProcMulti(ktCtx, req)
	if err != nil {
		return "", "",
			fmt.Errorf("[getGSEProcessStatus STEP]: failed to query process status via gseService.OperateProcMulti: %w", err)
	}
	result, err := u.WaitProcOperateTaskFinish(
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

// Callback 进程操作回调方法，在任务完成时被调用
// cbErr: 如果为 nil 表示任务成功，否则表示任务失败
func (u *UpdateRegisterExecutor) Callback(c *istep.Context, cbErr error) error {
	logs.Infof("[UpdateRegisterCallback CALLBACK]: starting callback")
	var payload UpdateRegisterPayload
	if err := c.GetPayload(&payload); err != nil {
		logs.Errorf("[UpdateRegisterCallback CALLBACK]: failed to get payload: %v", err)
		return fmt.Errorf("failed to get payload: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[UpdateRegisterCallback CALLBACK]: get common payload failed: %w", err)
	}

	kt := kit.NewWithTenant(payload.TenantID)

	// 只累加批次进度用于展示，批次终态由任务组回调统一收敛
	isSuccess := cbErr == nil
	if payload.BatchID > 0 {
		if _, err := u.Dao.TaskBatch().IncrementCompletedCount(kt, payload.BatchID, isSuccess); err != nil {
			logs.Errorf("[UpdateRegisterCallback CALLBACK]: failed to increment completed count, "+
				"batchID: %d, err: %v", payload.BatchID, err)
		}

		snapshot, err := u.updateBatchExtraDataWithLock(payload.TenantID, payload.BizID, payload.BatchID, registerProcessSuccessDelta(cbErr))
		if err != nil {
			logs.Errorf("update batch extra data failed, batchID=%d, err=%v",
				payload.BatchID, err)
		}

		if snapshot != nil {
			// 是否更新进程配置：仅由数量一致性决定
			allRegisterSucceeded := snapshot.RegisterProcessSuccessCount == snapshot.TotalCount
			if allRegisterSucceeded {
				// prev_data 保留操作前 DB 配置；source_data 收敛为 CMDB 最新快照
				// （配置一致时两者本就相同，等价于刷新同步状态）
				sourceData := commonPayload.ProcessPayload.LatestConfigData
				if sourceData == "" {
					sourceData = commonPayload.ProcessPayload.ConfigData
				}
				updateFields := map[string]any{
					"cc_sync_status": table.Synced,
					"prev_data":      commonPayload.ProcessPayload.ConfigData,
					"source_data":    sourceData,
				}
				if errU := u.Dao.Process().UpdateSelectedFields(
					kit.NewWithTenant(payload.TenantID),
					payload.BizID,
					updateFields,
					u.Dao.GenQuery().Process.ID.Eq(payload.ProcessID),
				); errU != nil {
					logs.Errorf(
						"[UpdateRegisterCallback CALLBACK]: update process config failed, processID=%s, err=%v",
						payload.ProcessID,
						errU,
					)
				}

				logs.Infof("[UpdateRegisterCallback CALLBACK]: successfully synced process config, "+
					"bizID: %d, processID: %d, prev_data: %s, source_data: %s",
					payload.BizID, payload.ProcessID,
					commonPayload.ProcessPayload.ConfigData, sourceData)
			}
		}

	}

	if isSuccess {
		logs.Infof("[UpdateRegisterCallback CALLBACK]: task %s completed successfully, no rollback needed",
			c.GetTaskID())
		return nil
	}

	// 进程操作失败，但是进程的部分状态可能在gse侧已经生效（如启动进程失败，但是进程实际上也会托管）
	// 优先使用gse侧进程状态，如果获取失败则回滚到原始状态
	processStatus, managedStatus, err := u.getGSEProcessStatus(c, payload.BizID)
	if err != nil {
		logs.Errorf("[UpdateRegisterCallback CALLBACK]: failed to get gse process status: %v, "+
			"falling back to original status", err)
		// PASS
		processStatus = payload.OriginalProcStatus
		managedStatus = payload.OriginalProcManagedStatus
		logs.Infof("[UpdateRegisterCallback CALLBACK]: rolling back to original status, bizID: %d, "+
			"processInstanceID: %d, status: %s, managedStatus: %s",
			payload.BizID, payload.ProcessInstanceID, processStatus, managedStatus)
	} else {
		logs.Infof("[UpdateRegisterCallback CALLBACK]: using gse process status, bizID: %d, "+
			"processInstanceID: %d, status: %s, managedStatus: %s",
			payload.BizID, payload.ProcessInstanceID, processStatus, managedStatus)
	}

	// 更新进程实例状态
	m := u.Dao.GenQuery().ProcessInstance
	if err = u.Dao.ProcessInstance().UpdateSelectedFields(kit.NewWithTenant(payload.TenantID), payload.BizID, map[string]any{
		"status":            processStatus,
		"managed_status":    managedStatus,
		"status_updated_at": time.Now(),
	}, m.ID.Eq(payload.ProcessInstanceID)); err != nil {
		logs.Errorf("[UpdateRegisterCallback CALLBACK]: failed to update process instance: %v", err)
		return fmt.Errorf("failed to update process instance during rollback: %w", err)
	}

	logs.Infof("[UpdateRegisterCallback CALLBACK]: successfully rolled back process instance status, "+
		"bizID: %d, processInstanceID: %d", payload.BizID, payload.ProcessInstanceID)

	return nil
}

// BatchConfigDecisionSnapshot 用于判断是否需要更新进程配置的最小状态快照
// 该结构不等同于 TaskBatch 的完整状态，仅包含配置更新判断所需的字段：
//   - Status：批次最终状态（由 IncrementCompletedCount 推进）
//   - TotalCount：批次内任务总数
//   - RegisterProcessSuccessCount：RegisterProcessStep 成功次数（来自 ExtraData）
type BatchConfigDecisionSnapshot struct {
	Status                      table.TaskBatchStatus
	TotalCount                  uint32
	RegisterProcessSuccessCount uint32
}

// updateBatchExtraDataWithLock 更新任务批次的 ExtraData（RegisterProcess.SuccessCount），并发安全
func (u *UpdateRegisterExecutor) updateBatchExtraDataWithLock(tenantID string, bizID, batchID uint32, delta uint32) (
	*BatchConfigDecisionSnapshot, error) {

	if delta == 0 {
		return nil, nil
	}

	u.RedLock.Acquire(keys.ResKind.BatchID(batchID))

	defer u.RedLock.Release(keys.ResKind.BatchID(batchID))

	kt := kit.NewWithTenant(tenantID)

	task, err := u.Dao.TaskBatch().GetByID(kt, bizID, batchID)
	if err != nil {
		return nil, err
	}

	// 2. 解析 ExtraData，与优先级编排共用同一 JSON，避免互相覆盖
	extra, err := task.Spec.GetExtraData()
	if err != nil {
		return nil, err
	}

	// RegisterProcessExtra 可能不存在，需兼容旧数据或首次写入场景
	if extra.RegisterProcess == nil {
		extra.RegisterProcess = &table.RegisterProcessExtra{}
	}

	// 3. 累加
	extra.RegisterProcess.SuccessCount += delta

	raw, err := json.Marshal(extra)
	if err != nil {
		return nil, err
	}

	// 4. 写回 DB
	if err := u.Dao.TaskBatch().UpdateExtraData(kt, batchID, string(raw)); err != nil {
		return nil, err
	}

	return &BatchConfigDecisionSnapshot{
		Status:                      task.Spec.Status,
		TotalCount:                  task.Spec.TotalCount,
		RegisterProcessSuccessCount: extra.RegisterProcess.SuccessCount,
	}, nil
}

// queryGSEProcessStatus 查询 GSE 状态
func (u *UpdateRegisterExecutor) queryGSEProcessStatus(ctx context.Context, payload *UpdateRegisterPayload,
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

	resp, err := u.GseService.OperateProcMulti(ctx, &gse.MultiProcOperateReq{
		ProcOperateReq: []gse.ProcessOperate{*operate},
	})
	if err != nil {
		return nil, err
	}

	result, err := u.WaitProcOperateTaskFinish(
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

// executeGSEOperate 执行gse操作
func (u *UpdateRegisterExecutor) executeGSEOperate(ctx context.Context, payload *UpdateRegisterPayload,
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

	resp, err := u.GseService.OperateProcMulti(ctx, &gse.MultiProcOperateReq{
		ProcOperateReq: []gse.ProcessOperate{*operate},
	})
	if err != nil {
		return err
	}

	result, err := u.WaitProcOperateTaskFinish(
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

// RegisterUpdateRegisterExecutor register executor
func RegisterUpdateRegisterExecutor(e *UpdateRegisterExecutor) {
	istep.Register(ValidateOperateStepName, istep.StepExecutorFunc(e.ValidateOperateStep))
	istep.Register(RegisterProcessStepName, istep.StepExecutorFunc(e.RegisterProcessStep))
	istep.Register(StartProcessStepName, istep.StepExecutorFunc(e.StartProcessStep))
	istep.Register(StopProcessStepName, istep.StepExecutorFunc(e.StopProcessStep))
	istep.Register(OperationCompletedStepName, istep.StepExecutorFunc(e.OperationCompletedStep))
	// 注册回调，用于任务失败时的状态回滚
	istep.RegisterCallback(UpdateRegisterCallbackName, istep.CallbackExecutorFunc(e.Callback))
}

// registerProcessSuccessDelta 根据任务失败发生的阶段，返回「托管信息是否已更新」的成功数增量：
//   - 任务成功，或失败发生在托管注册之后（启动 / 收尾）：托管信息已按 CMDB 最新快照更新，增量 1
//   - 失败发生在托管注册之前或注册本身（校验 / 停止 / 注册）：托管信息未更新，增量 0，
//     避免批次收尾误把 prev_data / source_data / cc_sync_status 刷成已同步
func registerProcessSuccessDelta(cbErr error) uint32 {
	if cbErr == nil {
		return 1
	}
	if errors.Is(cbErr, ErrStartProcessStepFailed) || errors.Is(cbErr, ErrOperationCompletedStepFailed) {
		return 1
	}
	return 0
}
