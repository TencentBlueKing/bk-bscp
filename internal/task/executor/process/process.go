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

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	gesprocessor "github.com/TencentBlueKing/bk-bscp/internal/processor/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbproc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
)

const (
	// ValidateOperateProcessStepName validate operate process step name
	ValidateOperateProcessStepName istep.StepName = "ValidateOperateProcess"
	// CompareWithCMDBProcessInfoStepName compare with cmdb process info step name
	CompareWithCMDBProcessInfoStepName istep.StepName = "CompareWithCMDBProcessInfo"
	// CompareWithGSEProcessStatusStepName compare with gse process status step name
	CompareWithGSEProcessStatusStepName istep.StepName = "CompareWithGSEProcessStatus"
	// CompareWithGSEProcessConfigStepName compare with gse process config step name
	CompareWithGSEProcessConfigStepName istep.StepName = "CompareWithGSEProcessConfig"
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
func NewProcessExecutor(gseService *gse.Service, cmdbService bkcmdb.Service, dao dao.Set) *ProcessExecutor {
	return &ProcessExecutor{
		Executor: &common.Executor{
			GseService:  gseService,
			CMDBService: cmdbService,
			Dao:         dao,
		},
	}
}

// OperatePayload 进程操作负载
type OperatePayload struct {
	BizID                     uint32
	BatchID                   uint32 // 任务批次ID，用于 Callback 更新批次状态
	OperateType               table.ProcessOperateType
	ProcessID                 uint32
	ProcessInstanceID         uint32
	NeedCompareCMDB           bool                       // 是否需要对比CMDB配置，适配页面强制更新的场景
	OriginalProcManagedStatus table.ProcessManagedStatus // 原进程托管状态，用于后续状态回滚
	OriginalProcStatus        table.ProcessStatus        // 原进程状态，用于后续状态回滚
}

// ValidateOperate 校验操作是否合法
func (e *ProcessExecutor) ValidateOperate(c *istep.Context) error {
	logs.Infof("[ValidateOperate STEP]: starting validation")
	payload := &OperatePayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("get payload failed: %w", err)
	}
	// 获取原进程状态和托管状态
	originalProcStatus := payload.OriginalProcStatus
	originalProcManagedStatus := payload.OriginalProcManagedStatus

	// 获取进程信息
	process, err := e.Dao.Process().GetByID(kit.New(), payload.BizID, payload.ProcessID)
	if err != nil {
		return fmt.Errorf("failed to get process: %w", err)
	}
	//  解析 SourceData 获取运行时配置
	var processInfo table.ProcessInfo
	if err := json.Unmarshal([]byte(process.Spec.SourceData), &processInfo); err != nil {
		return fmt.Errorf("unmarshal process source data failed: %v", err)
	}
	canOperate, message, _ := pbproc.CanProcessOperate(
		payload.OperateType,
		processInfo,
		originalProcStatus.String(),
		originalProcManagedStatus.String(),
		process.Spec.CcSyncStatus.String(),
	)
	if !canOperate {
		return fmt.Errorf("process cannot operate, reason: %s", message)
	}
	return nil
}

// CompareWithCMDBProcessInfo 对比CMDB进程信息
// 在进程操作前，对比数据库中存储的进程配置和 CMDB 最新的进程配置是否一致
func (e *ProcessExecutor) CompareWithCMDBProcessInfo(c *istep.Context) error {
	logs.Infof("[CompareWithCMDBProcessInfo STEP]: starting comparison")
	payload := &OperatePayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("get payload failed: %w", err)
	}

	// 判断是否需要对比CMDB，不需要则直接跳过
	if !payload.NeedCompareCMDB {
		logs.Infof("[CompareWithCMDBProcessInfo STEP]: skip comparison as needCompareCMDB=false, bizID: %d, "+
			"processID: %d, processInstanceID: %d", payload.BizID, payload.ProcessID, payload.ProcessInstanceID)
		return nil
	}

	// 获取bscp侧存储的进程配置
	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[CompareWithCMDBProcessInfo STEP]: get common payload failed: %w", err)
	}
	var dbProcessInfo table.ProcessInfo
	if err := json.Unmarshal([]byte(commonPayload.ProcessPayload.ConfigData), &dbProcessInfo); err != nil {
		return fmt.Errorf("[CompareWithCMDBProcessInfo STEP]: unmarshal database config data failed: %w", err)
	}

	// 调用 CMDB API 获取最新的进程配置
	// 查询进程记录获取 ServiceInstanceID 和 CcProcessID
	process, err := e.Dao.Process().GetByID(kit.New(), payload.BizID, payload.ProcessID)
	if err != nil {
		return fmt.Errorf("[CompareWithCMDBProcessInfo STEP]: get process from database failed: %w", err)
	}

	// 如果进程从cmdb侧删除，且本次操作是停止、强制停止、取消托管，则不进行对比
	if process.Spec.CcSyncStatus == table.Deleted &&
		(payload.OperateType == table.KillProcessOperate || payload.OperateType == table.StopProcessOperate ||
			payload.OperateType == table.UnregisterProcessOperate) {
		return nil
	}

	// 获取cmdb侧最新进程详情
	processInfo, err := e.CMDBService.ListProcessDetailByIds(c.Context(), bkcmdb.ProcessReq{
		BkBizID:      int(payload.BizID),
		BkProcessIDs: []int{int(process.Attachment.CcProcessID)},
	})
	if err != nil {
		return fmt.Errorf("[CompareWithCMDBProcessInfo STEP]: failed to get process from CMDB, bizID: %d, "+
			"ccProcessID: %d, err: %v", payload.BizID, process.Attachment.CcProcessID, err)
	}
	if len(processInfo) == 0 {
		return fmt.Errorf("process not found in CMDB, bizID: %d, ccProcessID: %d",
			payload.BizID, process.Attachment.CcProcessID)
	}
	cmdbProcessInfo := processInfo[0]

	// 用cmdb的ProcessInfo构建bscp侧的ProcessInfo方便后续对比
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

	// 对比数据库配置和 CMDB 最新配置
	if !reflect.DeepEqual(dbProcessInfo, latestCMDBInfo) {
		// 输出差异信息
		diffs := buildProcessInfoDiff(&dbProcessInfo, &latestCMDBInfo)
		logs.Errorf("CompareWithCMDBProcessInfo: process config mismatch, bizID: %d, processID: %d, "+
			"processInstanceID: %d, differences: %v", payload.BizID, payload.ProcessID, payload.ProcessInstanceID, diffs)

		return fmt.Errorf("process config mismatch with CMDB, please sync from CMDB first")
	}

	logs.Infof("CompareWithCMDBProcessInfo completed: bizID: %d, processID: %d, processInstanceID: %d, config matched",
		payload.BizID, payload.ProcessID, payload.ProcessInstanceID)
	return nil
}

// buildProcessInfoDiff 构建 ProcessInfo 差异详情
func buildProcessInfoDiff(dbInfo, cmdbInfo *table.ProcessInfo) []string {
	var diffs []string

	if dbInfo.BkStartParamRegex != cmdbInfo.BkStartParamRegex {
		diffs = append(diffs, fmt.Sprintf("BkStartParamRegex: db=%q, cmdb=%q",
			dbInfo.BkStartParamRegex, cmdbInfo.BkStartParamRegex))
	}
	if dbInfo.WorkPath != cmdbInfo.WorkPath {
		diffs = append(diffs, fmt.Sprintf("WorkPath: db=%q, cmdb=%q",
			dbInfo.WorkPath, cmdbInfo.WorkPath))
	}
	if dbInfo.PidFile != cmdbInfo.PidFile {
		diffs = append(diffs, fmt.Sprintf("PidFile: db=%q, cmdb=%q",
			dbInfo.PidFile, cmdbInfo.PidFile))
	}
	if dbInfo.User != cmdbInfo.User {
		diffs = append(diffs, fmt.Sprintf("User: db=%q, cmdb=%q",
			dbInfo.User, cmdbInfo.User))
	}
	if dbInfo.ReloadCmd != cmdbInfo.ReloadCmd {
		diffs = append(diffs, fmt.Sprintf("ReloadCmd: db=%q, cmdb=%q",
			dbInfo.ReloadCmd, cmdbInfo.ReloadCmd))
	}
	if dbInfo.RestartCmd != cmdbInfo.RestartCmd {
		diffs = append(diffs, fmt.Sprintf("RestartCmd: db=%q, cmdb=%q",
			dbInfo.RestartCmd, cmdbInfo.RestartCmd))
	}
	if dbInfo.StartCmd != cmdbInfo.StartCmd {
		diffs = append(diffs, fmt.Sprintf("StartCmd: db=%q, cmdb=%q",
			dbInfo.StartCmd, cmdbInfo.StartCmd))
	}
	if dbInfo.StopCmd != cmdbInfo.StopCmd {
		diffs = append(diffs, fmt.Sprintf("StopCmd: db=%q, cmdb=%q",
			dbInfo.StopCmd, cmdbInfo.StopCmd))
	}
	if dbInfo.FaceStopCmd != cmdbInfo.FaceStopCmd {
		diffs = append(diffs, fmt.Sprintf("FaceStopCmd: db=%q, cmdb=%q",
			dbInfo.FaceStopCmd, cmdbInfo.FaceStopCmd))
	}
	if dbInfo.Timeout != cmdbInfo.Timeout {
		diffs = append(diffs, fmt.Sprintf("Timeout: db=%d, cmdb=%d",
			dbInfo.Timeout, cmdbInfo.Timeout))
	}

	return diffs
}

// CompareWithGSEProcessStatus 对比GSE进程状态
func (e *ProcessExecutor) CompareWithGSEProcessStatus(c *istep.Context) error {
	logs.Infof("[CompareWithGSEProcessStatus STEP]: starting comparison")
	payload := &OperatePayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("[CompareWithGSEProcessStatus STEP]: get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return err
	}

	// 查询进程信息
	process, err := e.Dao.Process().GetByID(kit.New(), payload.BizID, payload.ProcessID)
	if err != nil {
		return fmt.Errorf("[CompareWithGSEProcessStatus STEP]: failed to get process: %w", err)
	}
	// 获取进程配置信息
	var processInfo table.ProcessInfo
	err = json.Unmarshal([]byte(process.Spec.SourceData), &processInfo)
	if err != nil {
		return fmt.Errorf("[CompareWithGSEProcessStatus STEP]: failed to marshal process info: %w", err)
	}

	// 使用 OperateProcMulti 接口查询进程状态，操作码为 2（OpTypeQuery）
	params := gesprocessor.BuildProcessOperateParams{
		BizID:         payload.BizID,
		Alias:         commonPayload.ProcessPayload.Alias,
		FuncName:      commonPayload.ProcessPayload.FuncName,
		AgentID:       []string{commonPayload.ProcessPayload.AgentID},
		HostInstSeq:   commonPayload.ProcessPayload.HostInstSeq,
		ModuleInstSeq: commonPayload.ProcessPayload.ModuleInstSeq,
		SetName:       commonPayload.ProcessPayload.SetName,
		ModuleName:    commonPayload.ProcessPayload.ModuleName,
		GseOpType:     gse.OpTypeQuery,
		ProcessInfo:   processInfo,
	}
	processOperate, err := gesprocessor.BuildProcessOperate(params)
	if err != nil {
		return fmt.Errorf("[CompareWithGSEProcessStatus STEP]: failed to build process operate: %w", err)
	}
	req := &gse.MultiProcOperateReq{
		ProcOperateReq: []gse.ProcessOperate{*processOperate},
	}

	resp, err := e.GseService.OperateProcMulti(c.Context(), req)
	if err != nil {
		// nolint: goerr113
		return fmt.Errorf("[CompareWithGSEProcessStatus STEP]: failed to query process status via gseService.OperateProcMulti: %w", err)
	}
	// 等待查询任务完成
	result, err := e.WaitProcOperateTaskFinish(c.Context(),
		resp.TaskID, payload.BizID,
		commonPayload.ProcessPayload.HostInstSeq,
		commonPayload.ProcessPayload.Alias,
		commonPayload.ProcessPayload.AgentID)
	if err != nil {
		return fmt.Errorf("[CompareWithGSEProcessStatus STEP]: failed to wait for query task finish: %w", err)
	}

	// 构建 GSE 接口响应的 key
	key := gse.BuildResultKey(commonPayload.ProcessPayload.AgentID,
		payload.BizID,
		commonPayload.ProcessPayload.Alias,
		commonPayload.ProcessPayload.HostInstSeq)
	logs.Infof("[CompareWithGSEProcessStatus STEP]: Finalize key: %s", key)
	procResult, ok := result[key]
	if !ok {
		return fmt.Errorf("[CompareWithGSEProcessStatus STEP]: process result not found for key: %s", key)
	}

	// 检查查询操作是否成功
	if !gse.IsSuccess(procResult.ErrorCode) {
		return fmt.Errorf("[CompareWithGSEProcessStatus STEP]: failed to query process status, errorCode=%d, errorMsg=%s",
			procResult.ErrorCode, procResult.ErrorMsg)
	}

	// 解析 content 获取进程状态
	var statusContent gse.ProcessStatusContent
	if err = json.Unmarshal([]byte(procResult.Content), &statusContent); err != nil {
		return fmt.Errorf("[CompareWithGSEProcessStatus STEP]: failed to unmarshal process status content: %w", err)
	}

	// 根据操作类型判断是否需要继续操作进程
	isValid, message := isOperationValid(
		payload.OperateType, &statusContent, payload.OriginalProcStatus, payload.OriginalProcManagedStatus)
	if !isValid {
		return fmt.Errorf("[CompareWithGSEProcessStatus STEP]: operation is not valid: %s", message)
	}
	return nil
}

// shouldSkipOperation 判断操作是否合法
// false表示操作不合法，true表示操作合法
// message表示操作不合法的原因
func isOperationValid(
	operateType table.ProcessOperateType,
	statusContent *gse.ProcessStatusContent,
	originalProcStatus table.ProcessStatus,
	originalProcManagedStatus table.ProcessManagedStatus,
) (bool, string) {
	// 只要操作成功，即使进程未托管及未启动也会返回查询的进程的信息
	if len(statusContent.Process) == 0 {
		return false, "process not found in gse"
	}
	procDetail := statusContent.Process[0]
	// 只要操作成功，即使进程未托管及未启动也会返回查询的进程实例的信息
	if len(procDetail.Instance) == 0 {
		return false, "process instance not found in gse"
	}
	// 获取gse侧存储的进程实例信息
	instance := procDetail.Instance[0]

	gseStatus := table.ProcessStatusStopped
	if instance.PID > 0 {
		gseStatus = table.ProcessStatusRunning
	}
	gseManagedStatus := table.ProcessManagedStatusUnmanaged
	if instance.IsAuto {
		gseManagedStatus = table.ProcessManagedStatusManaged
	}

	if originalProcStatus != gseStatus {
		return false, fmt.Sprintf("process status is %s in bscp, but %s in gse", originalProcStatus, gseStatus)
	}
	if originalProcManagedStatus != gseManagedStatus {
		return false, fmt.Sprintf("process managed status is %s in bscp, but %s in gse", originalProcManagedStatus, gseManagedStatus)
	}

	switch operateType {
	case table.StartProcessOperate:
		// 启动操作：如果进程已经在运行，跳过
		if gseStatus == table.ProcessStatusRunning {
			return false, "process status is running in gse"
		}

	case table.StopProcessOperate, table.KillProcessOperate:
		// 停止/杀死操作：如果进程已经停止，跳过
		if gseStatus == table.ProcessStatusStopped {
			return false, "process already stopped in gse"
		}

	case table.RegisterProcessOperate:
		// 托管操作：如果进程已经被托管
		if gseManagedStatus == table.ProcessManagedStatusManaged {
			return false, "process already managed in gse"
		}

	case table.UnregisterProcessOperate:
		// 取消托管操作：如果进程已经取消托管，跳过
		if gseManagedStatus == table.ProcessManagedStatusUnmanaged {
			return false, "process already unmanaged in gse"
		}

	case table.RestartProcessOperate, table.ReloadProcessOperate:
		// 重启操作/重载操作：总是执行，不跳过
		return true, ""
	}

	return true, ""
}

// CompareWithGSEProcessConfig 对比GSE进程配置（TODO: 待实现）
func (e *ProcessExecutor) CompareWithGSEProcessConfig(c *istep.Context) error {
	// TODO: 通过gse进程配置文件获取接口获取gse托管的进程配置，与db中存储的配置进行对比
	logs.Infof("[CompareWithGSEProcessConfig STEP]: skip for now (TODO)")
	return nil
}

// Operate 进程操作
func (e *ProcessExecutor) Operate(c *istep.Context) error {
	logs.Infof("[Operate STEP]: starting operation")
	payload := &OperatePayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("[Operate STEP]: get payload failed: %w", err)
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[Operate STEP]: get common payload failed: %w", err)
	}

	// 解析进程配置信息
	var processInfo table.ProcessInfo
	err := json.Unmarshal([]byte(commonPayload.ProcessPayload.ConfigData), &processInfo)
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
	processOperate, err := gesprocessor.BuildProcessOperate(params)
	if err != nil {
		return fmt.Errorf("[Operate STEP]: failed to build process operate: %w", err)
	}

	items := []gse.ProcessOperate{*processOperate}

	req := &gse.MultiProcOperateReq{
		ProcOperateReq: items,
	}

	resp, err := e.GseService.OperateProcMulti(c.Context(), req)
	if err != nil {
		return fmt.Errorf("[Operate STEP]: failed to operate process via gseService.OperateProcMulti: %w", err)
	}

	result, err := e.WaitProcOperateTaskFinish(c.Context(), resp.TaskID,
		payload.BizID,
		commonPayload.ProcessPayload.HostInstSeq,
		commonPayload.ProcessPayload.Alias,
		commonPayload.ProcessPayload.AgentID)
	if err != nil {
		return fmt.Errorf("[Operate STEP]: failed to wait for task finish: %w", err)
	}
	// 构建 GSE 返回结果的 key
	key := gse.BuildResultKey(commonPayload.ProcessPayload.AgentID,
		payload.BizID,
		commonPayload.ProcessPayload.Alias,
		commonPayload.ProcessPayload.HostInstSeq)
	logs.Infof("[Operate STEP]: Finalize key: %s", key)
	procResult, ok := result[key]
	if !ok {
		return fmt.Errorf("[Operate STEP]: process result not found for key: %s", key)
	}

	// 检查进程操作是否成功，若操作成功则进入Finalize步骤，由Finalize步骤更新进程实例状态，否则在回调中回滚
	if !gse.IsSuccess(procResult.ErrorCode) {
		return fmt.Errorf("[Operate STEP]: process operate failed, errorCode=%d, errorMsg=%s",
			procResult.ErrorCode, procResult.ErrorMsg)
	}
	return nil
}

// Finalize 进程操作完成
// nolint: funlen
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

	if payload.OperateType == table.UnregisterProcessOperate {
		// 判断是否存在缩容
		process, errP := e.Dao.Process().GetByID(kit.New(), payload.BizID, payload.ProcessID)
		if errP != nil {
			return fmt.Errorf("[Finalize STEP]: failed to get process: %w", errP)
		}

		procInst, errI := e.Dao.ProcessInstance().GetByProcessIDs(kit.New(), payload.BizID, []uint32{payload.ProcessID})
		if errI != nil {
			return fmt.Errorf("[Finalize STEP]: failed to get process instance: %w", errI)
		}
		// 若进程数量被缩容，则删除对应的实例
		if process.Spec.ProcNum < uint(len(procInst)) {
			if errD := e.Dao.ProcessInstance().Delete(kit.New(), payload.BizID, payload.ProcessInstanceID); errD != nil {
				return fmt.Errorf("[Finalize STEP]: failed to delete process instance: %w", errD)
			}
			return nil // 删除后直接返回，无需执行后续更新逻辑
		}
	}

	// 更新进程实例状态字段
	m := e.Dao.GenQuery().ProcessInstance
	if err = e.Dao.ProcessInstance().UpdateSelectedFields(kit.New(), payload.BizID, map[string]any{
		"status":            processStatus,
		"managed_status":    managedStatus,
		"status_updated_at": time.Now(),
	}, m.ID.Eq(payload.ProcessInstanceID)); err != nil {
		return fmt.Errorf("[Finalize STEP]: failed to update process instance: %w", err)
	}

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

	// 更新 TaskBatch 的完成计数
	isSuccess := cbErr == nil
	if payload.BatchID > 0 {
		if err := e.Dao.TaskBatch().IncrementCompletedCount(kit.New(), payload.BatchID, isSuccess); err != nil {
			logs.Errorf("[ProcessOperateCallback CALLBACK]: failed to increment completed count, "+
				"batchID: %d, err: %v", payload.BatchID, err)
			// PASS 继续执行，不影响回滚逻辑
		}
	}

	// 如果任务成功，不需要回滚
	if isSuccess {
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
	if err = e.Dao.ProcessInstance().UpdateSelectedFields(kit.New(), payload.BizID, map[string]any{
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
	process, err := e.Dao.Process().GetByID(kit.New(), bizID, payload.ProcessID)
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
	resp, err := e.GseService.OperateProcMulti(c.Context(), req)
	if err != nil {
		return "", "", fmt.Errorf("failed to query process status via gseService.OperateProcMulti: %w", err)
	}
	result, err := e.WaitProcOperateTaskFinish(
		c.Context(),
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

// RegisterExecutor register executor
func RegisterExecutor(e *ProcessExecutor) {
	// 校验操作是否合法
	istep.Register(ValidateOperateProcessStepName, istep.StepExecutorFunc(e.ValidateOperate))
	istep.Register(CompareWithCMDBProcessInfoStepName, istep.StepExecutorFunc(e.CompareWithCMDBProcessInfo))
	istep.Register(CompareWithGSEProcessStatusStepName, istep.StepExecutorFunc(e.CompareWithGSEProcessStatus))
	istep.Register(CompareWithGSEProcessConfigStepName, istep.StepExecutorFunc(e.CompareWithGSEProcessConfig))
	// 注册主要执行步骤
	istep.Register(OperateProcessStepName, istep.StepExecutorFunc(e.Operate))
	// 注册进程操作完成后的状态更新步骤
	istep.Register(FinalizeOperateProcessStepName, istep.StepExecutorFunc(e.Finalize))
	// 注册回调，用于任务失败时的状态回滚
	istep.RegisterCallback(ProcessOperateCallbackName, istep.CallbackExecutorFunc(e.Callback))
}
