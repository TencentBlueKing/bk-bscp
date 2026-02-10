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
	"time"

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"

	"github.com/TencentBlueKing/bk-bscp/cmd/cache-service/service/cache/keys"
	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/processor/cmdb"
	gesprocessor "github.com/TencentBlueKing/bk-bscp/internal/processor/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
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

// NewUpdateRegisterExecutor new update register executor
func NewUpdateRegisterExecutor(gseService *gse.Service, cmdbService bkcmdb.Service, dao dao.Set,
	redLock *lock.RedisLock) *UpdateRegisterExecutor {

	return &UpdateRegisterExecutor{
		Executor: &common.Executor{
			GseService:  gseService,
			CMDBService: cmdbService,
			Dao:         dao,
			RedLock:     redLock,
		},
	}
}

// UpdateRegisterPayload 进程操作负载
type UpdateRegisterPayload struct {
	BizID                     uint32
	BatchID                   uint32 // 任务批次ID，用于 Callback 更新批次状态
	OperateType               table.ProcessOperateType
	OperateUser               string
	ProcessID                 uint32
	ProcessInstanceID         uint32
	OriginalProcManagedStatus table.ProcessManagedStatus // 原进程托管状态，用于后续状态回滚
	OriginalProcStatus        table.ProcessStatus        // 原进程状态，用于后续状态回滚
	EnableProcessRestart      bool
	CCSyncStatus              table.CCSyncStatus
}

// ValidateOperateStep 校验操作是否合法
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

	// 获取cmdb侧最新进程详情
	processInfo, err := u.CMDBService.ListProcessDetailByIds(c.Context(), bkcmdb.ProcessReq{
		BkBizID:      int(payload.BizID),
		BkProcessIDs: []int{int(commonPayload.ProcessPayload.CcProcessID)},
	})
	if err != nil {
		return fmt.Errorf("[ValidateOperateStep STEP]: failed to get process from CMDB, bizID: %d, "+
			"ccProcessID: %d, err: %v", payload.BizID, commonPayload.ProcessPayload.CcProcessID, err)
	}

	if len(processInfo) == 0 {
		tx := u.Dao.GenQuery().Begin()
		err = cmdb.DeleteInstanceStoppedUnmanaged(kit.New(), u.Dao, tx, payload.BizID, []uint32{payload.ProcessID})
		if err != nil {
			logs.Errorf("[CompareWithCMDBProcessInfo STEP]: delete stopped/unmanaged failed for bizID=%d, processIDs=%v: %v",
				payload.BizID, payload.ProcessID, err)
			if rbErr := tx.Rollback(); rbErr != nil {
				logs.Errorf("[CompareWithCMDBProcessInfo STEP]: rollback failed for bizID=%d: %v", payload.BizID, rbErr)
				return rbErr
			}
			return err
		}
		if errT := tx.Commit(); errT != nil {
			logs.Errorf("[CompareWithCMDBProcessInfo STEP]: commit failed for biz %d: %v", payload.BizID, errT)
			return errT
		}
		return fmt.Errorf("process not found in CMDB, bizID: %d, ccProcessID: %d",
			payload.BizID, commonPayload.ProcessPayload.CcProcessID)
	}

	cmdbProcessInfo := processInfo[0]
	latestCMDBInfo := table.ProcessInfo{
		BkStartParamRegex: cmdbProcessInfo.BkStartParamRegex,
		WorkPath:          cmdbProcessInfo.WorkPath,
		PidFile:           cmdbProcessInfo.PidFile,
		User:              cmdbProcessInfo.User,
		ReloadCmd:         cmdbProcessInfo.ReloadCmd,
		RestartCmd:        cmdbProcessInfo.RestartCmd,
		StartCmd:          cmdbProcessInfo.StartCmd,
		StopCmd:           cmdbProcessInfo.StopCmd,
		FaceStopCmd:       cmdbProcessInfo.FaceStopCmd,
		Timeout:           cmdbProcessInfo.Timeout,
		StartCheckSecs:    cmdbProcessInfo.BkStartCheckSecs,
	}

	// 检测是否拥有启停命令
	if payload.EnableProcessRestart {
		if !pbproc.HasOperateCommand(table.StopProcessOperate, latestCMDBInfo) {
			return fmt.Errorf("the stop command does not exist")
		}
		if !pbproc.HasOperateCommand(table.StartProcessOperate, latestCMDBInfo) {
			return fmt.Errorf("the start command does not exist")
		}
	}

	// 验证更新托管操作
	canOperate, message, _ := pbproc.CanProcessOperate(
		payload.OperateType,
		latestCMDBInfo,
		string(payload.OriginalProcStatus),
		string(payload.OriginalProcManagedStatus),
		payload.CCSyncStatus.String(),
	)
	if !canOperate {
		return fmt.Errorf("process cannot operate, reason: %s", message)
	}

	configData, err := json.Marshal(latestCMDBInfo)
	if err != nil {
		logs.Errorf("[ValidateOperateStep STEP]: json marshal prev_data and source_data failed to %s, processID=%s, err=%v",
			payload.ProcessID, err)
	}
	commonPayload.ProcessPayload.ConfigData = string(configData)
	if err = c.SetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[ValidateOperateStep STEP]: set common payload failed: %w", err)
	}

	return nil
}

// StopProcessStep 停止旧的进程
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

	// 解析进程配置信息
	var processInfo table.ProcessInfo
	err := json.Unmarshal([]byte(commonPayload.ProcessPayload.ConfigData), &processInfo)
	if err != nil {
		return fmt.Errorf("[StopProcessStep STEP]: unmarshal process info failed: %w", err)
	}

	// 1. 查询gse
	status, err := u.queryGSEProcessStatus(c.Context(), payload, commonPayload, processInfo)
	if err != nil {
		return err
	}

	if !needStopProcess(status) {
		logs.Infof("[StopProcessStep STEP]: process not running, skip stop")
		return nil
	}

	if err = u.executeGSEOperate(c.Context(), payload, commonPayload, table.StopProcessOperate); err != nil {
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

// RegisterProcessStep 托管进程
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

	if err := u.executeGSEOperate(
		c.Context(),
		payload,
		commonPayload,
		table.RegisterProcessOperate,
	); err != nil {
		return fmt.Errorf("%w: %v", ErrRegisterProcessStepFailed, err)
	}

	return nil
}

// StartProcessStep 启动进程
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

	if err := u.executeGSEOperate(c.Context(), payload, commonPayload, table.StartProcessOperate); err != nil {
		return fmt.Errorf(
			"[StartProcessStep STEP]: execute process operate %s failed: %w",
			table.StartProcessOperate,
			err,
		)
	}

	return nil
}

// OperationCompletedStep 进程操作完成
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

	// 解析进程配置信息
	var processInfo table.ProcessInfo
	err := json.Unmarshal([]byte(commonPayload.ProcessPayload.ConfigData), &processInfo)
	if err != nil {
		return fmt.Errorf("[OperationCompletedStep STEP]: unmarshal process info failed: %w", err)
	}

	// 获取gse侧进程状态
	processStatus, managedStatus, err := u.getGSEProcessStatus(c, payload.BizID)
	if err != nil {
		return fmt.Errorf("[OperationCompletedStep STEP]: failed to get gse process status: %w", err)
	}

	// 更新进程实例状态字段
	m := u.Dao.GenQuery().ProcessInstance
	if err = u.Dao.ProcessInstance().UpdateSelectedFields(kit.New(), payload.BizID, map[string]any{
		"status":            processStatus,
		"managed_status":    managedStatus,
		"status_updated_at": time.Now(),
	}, m.ID.Eq(payload.ProcessInstanceID)); err != nil {
		return fmt.Errorf("[OperationCompletedStep STEP]: failed to update process instance: %w", err)
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
	process, err := u.Dao.Process().GetByID(kit.New(), bizID, payload.ProcessID)
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
	resp, err := u.GseService.OperateProcMulti(c.Context(), req)
	if err != nil {
		return "", "",
			fmt.Errorf("[getGSEProcessStatus STEP]: failed to query process status via gseService.OperateProcMulti: %w", err)
	}
	result, err := u.WaitProcOperateTaskFinish(
		c.Context(),
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

	// 更新 TaskBatch 的完成计数
	isSuccess := cbErr == nil
	if payload.BatchID > 0 {
		if err := u.Dao.TaskBatch().IncrementCompletedCount(kit.New(), payload.BatchID, isSuccess); err != nil {
			logs.Errorf("[UpdateRegisterCallback CALLBACK]: failed to increment completed count, "+
				"batchID: %d, err: %v", payload.BatchID, err)
		}

		snapshot, err := u.updateBatchExtraDataWithLock(payload.BatchID, registerProcessSuccessDelta(cbErr))
		if err != nil {
			logs.Errorf("update batch extra data failed, batchID=%d, err=%v",
				payload.BatchID, err)
		}

		if snapshot != nil {
			// 是否更新进程配置：仅由数量一致性决定
			allRegisterSucceeded := snapshot.RegisterProcessSuccessCount == snapshot.TotalCount
			if allRegisterSucceeded {
				updateFields := map[string]any{
					"cc_sync_status": table.Synced,
					"prev_data":      commonPayload.ProcessPayload.ConfigData,
					"source_data":    commonPayload.ProcessPayload.ConfigData,
				}
				if errU := u.Dao.Process().UpdateSelectedFields(
					kit.New(),
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

				logs.Infof("[UpdateRegisterCallback CALLBACK]: successfully rolled back process instance status, "+
					"bizID: %d, processInstanceID: %d", payload.BizID, payload.ProcessInstanceID)
			}
		}

		// 统一推送事件
		u.AfterCallbackNotify(c.Context(), common.CallbackNotify{
			BizID:    payload.BizID,
			BatchID:  payload.BatchID,
			Operator: payload.OperateUser,
			CbErr:    cbErr,
		})
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
	if err = u.Dao.ProcessInstance().UpdateSelectedFields(kit.New(), payload.BizID, map[string]any{
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
func (u *UpdateRegisterExecutor) updateBatchExtraDataWithLock(batchID uint32, delta uint32) (
	*BatchConfigDecisionSnapshot, error) {

	if delta == 0 {
		return nil, nil
	}

	u.RedLock.Acquire(keys.ResKind.BatchID(batchID))

	defer u.RedLock.Release(keys.ResKind.BatchID(batchID))

	// 1. 重新从 DB 读
	task, err := u.Dao.TaskBatch().GetByID(kit.New(), batchID)
	if err != nil {
		return nil, err
	}

	// 2. 解析 ExtraData
	extra, err := parseTaskBatchExtraData(task.Spec.ExtraData)
	if err != nil {
		return nil, err
	}

	// RegisterProcessExtra 可能不存在，需兼容旧数据或首次写入场景
	if extra.RegisterProcess == nil {
		extra.RegisterProcess = &RegisterProcessExtra{}
	}

	// 3. 累加
	extra.RegisterProcess.SuccessCount += delta

	raw, err := json.Marshal(extra)
	if err != nil {
		return nil, err
	}

	// 4. 写回 DB
	if err := u.Dao.TaskBatch().UpdateExtraData(kit.New(), batchID, string(raw)); err != nil {
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
	commonPayload *common.TaskPayload, op table.ProcessOperateType) error {

	var processInfo table.ProcessInfo
	if err := json.Unmarshal([]byte(commonPayload.ProcessPayload.ConfigData), &processInfo); err != nil {
		return fmt.Errorf("unmarshal process info failed: %w", err)
	}

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

// registerProcessSuccessDelta 根据 RegisterProcessStep 的执行结果，返回成功数增量
func registerProcessSuccessDelta(err error) uint32 {
	if errors.Is(err, ErrRegisterProcessStepFailed) {
		return 0
	}
	return 1
}

// TaskBatchExtraData 扩展参数
type TaskBatchExtraData struct {
	RegisterProcess *RegisterProcessExtra `json:"register_process,omitempty"`
}

// RegisterProcessExtra 更新托管扩展参数
type RegisterProcessExtra struct {
	SuccessCount uint32 `json:"success_count"`
}

// parseTaskBatchExtraData 解析扩展参数
func parseTaskBatchExtraData(raw string) (*TaskBatchExtraData, error) {
	if raw == "" {
		return &TaskBatchExtraData{}, nil
	}

	var extra TaskBatchExtraData
	if err := json.Unmarshal([]byte(raw), &extra); err != nil {
		return nil, err
	}

	return &extra, nil
}
