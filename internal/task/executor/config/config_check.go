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

package config

import (
	"errors"
	"fmt"
	"strings"

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"
	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	pushmanager "github.com/TencentBlueKing/bk-bscp/internal/components/push_manager"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	// CheckConfigCallbackName check config callback name
	CheckConfigCallbackName istep.CallbackName = "CheckConfigCallback"
	// CheckConfigStepName check config md5 step name
	CheckConfigMD5StepName istep.StepName = "CheckConfigMD5"
	// FetchConfigContentStepName fetch config content step name
	FetchConfigContentStepName istep.StepName = "FetchConfigConten"
)

// CheckConfigExecutor 配置检查执行器
type CheckConfigExecutor struct {
	*common.Executor
}

// NewCheckConfigExecutor new check config executor
func NewCheckConfigExecutor(dao dao.Set, gseService *gse.Service, cmdbService bkcmdb.Service,
	pm pushmanager.Service) *CheckConfigExecutor {
	return &CheckConfigExecutor{
		Executor: &common.Executor{
			Dao:         dao,
			GseService:  gseService,
			GseConf:     cc.G().GSE,
			CMDBService: cmdbService,
			PM:          pm,
		},
	}
}

// CheckConfigPayload check config step payload
type CheckConfigPayload struct {
	TenantID           string
	BizID              uint32
	BatchID            uint32
	ConfigTemplateID   uint32
	ConfigTemplateName string
	OperateType        table.ConfigOperateType
	OperatorUser       string
	Template           *table.Template
	TemplateRevision   *table.TemplateRevision
	Process            *table.Process
	ProcessInstance    *table.ProcessInstance
}

// CheckConfig implements istep.Step.
// nolint:funlen
func (e *CheckConfigExecutor) CheckConfigMD5(c *istep.Context) error {
	payload := &CheckConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		return err
	}

	kt := kit.NewWithTenant(payload.TenantID)
	kt.BizID = payload.BizID

	logs.Infof("[CheckConfigMD5 STEP]: start, biz_id=%d, batch_id=%d, template_id=%d, template_name=%s",
		payload.BizID, payload.BatchID,
		payload.ConfigTemplateID, payload.ConfigTemplateName)

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("get common payload failed: %w", err)
	}
	if commonPayload.ConfigPayload == nil {
		return fmt.Errorf("config payload is nil")
	}
	if commonPayload.ProcessPayload == nil {
		return fmt.Errorf("process payload is nil")
	}

	logs.Infof("[CheckConfigMD5 STEP]: file_mode=%s, config_key=%s, agent_id=%s",
		commonPayload.ConfigPayload.ConfigFileMode,
		commonPayload.ConfigPayload.ConfigInstanceKey,
		commonPayload.ProcessPayload.AgentID)

	fullPath, err := renderFullPath(commonPayload)
	if err != nil {
		return fmt.Errorf("render full path failed: %w", err)
	}

	fileMode := commonPayload.ConfigPayload.ConfigFileMode
	builder := &ScriptBuilder{FileMode: fileMode}

	script, err := builder.BuildFileMD5Script(fullPath)
	if err != nil {
		return err
	}

	scriptName := BuildScriptNameByFileMode("check_md5", commonPayload, fileMode)
	storeDir := ScriptStoreDirByFileMode(
		e.GseConf.ScriptStoreDir, e.GseConf.WindowsScriptStoreDir, fileMode)
	command := BuildScriptCommand(storeDir, scriptName, fileMode)

	logs.Infof("[CheckConfigMD5 STEP]: script prepared, batch_id=%d, command=%s, target=%s",
		payload.BatchID, command, fullPath)

	req := &gse.ExecuteScriptReq{
		Agents: []gse.Agent{
			{
				BkAgentID: payload.Process.Attachment.AgentID,
				User:      GetExecutionUser(fileMode, payload.TemplateRevision.Spec.Permission.User),
			},
		},
		Scripts: []gse.Script{
			{
				ScriptName:     scriptName,
				ScriptStoreDir: storeDir,
				ScriptContent:  script,
			},
		},
		AtomicTasks: []gse.AtomicTask{
			{
				Command:        command,
				AtomicTaskID:   0,
				TimeoutSeconds: scriptTimeoutSec,
			},
		},
		AtomicTasksRelations: []gse.AtomicTaskRelation{
			{AtomicTaskID: 0, AtomicTaskIDIdx: []int{}},
		},
	}

	resp, err := e.GseService.AsyncExtensionsExecuteScript(kt.Ctx, req)

	if err != nil {
		logs.Errorf("[CheckConfigMD5 STEP]: create execute script task failed: %v", err)
		return fmt.Errorf("create md5 execute script task failed: %w", err)
	}

	if resp == nil || resp.Result.TaskID == "" {
		logs.Errorf("[CheckConfigMD5 STEP]: gse execute script response is nil, batch_id=%d", payload.BatchID)
		return fmt.Errorf("gse execute script response is nil, batch_id=%d", payload.BatchID)
	}

	// 通过脚本任务ID获取脚本执行结果
	result, err := e.WaitExecuteScriptFinish(kt.Ctx, resp.Result.TaskID, payload.Process.Attachment.AgentID)
	if err != nil {
		return fmt.Errorf("wait script execution failed: %w", err)
	}

	if len(result.Result) == 0 {
		return fmt.Errorf("script execution result is empty, task_id=%s", resp.Result.TaskID)
	}

	logs.Infof("[CheckConfigMD5 STEP]: gse full result, batch_id: %d, result: %+v",
		payload.BatchID, result)

	r := result.Result[0]

	if r.ErrorCode != 0 || r.ScriptExitCode != 0 {
		logs.Errorf(
			"[CheckConfigMD5 STEP]: script execution failed, agent=%s, container=%s, "+
				"errorCode=%d, scriptExitCode=%d, msg=%s, screen=%s",
			r.BkAgentID, r.BkContainerID,
			r.ErrorCode, r.ScriptExitCode,
			r.ErrorMsg, r.Screen,
		)
		return fmt.Errorf(
			"script execution failed, agent=%s, container=%s, "+
				"errorCode=%d, scriptExitCode=%d, msg=%s, screen=%s",
			r.BkAgentID, r.BkContainerID,
			r.ErrorCode, r.ScriptExitCode,
			r.ErrorMsg, r.Screen,
		)
	}

	actualMD5 := strings.TrimSpace(r.Screen)

	// 2. 查询历史下发记录
	configInstance, err := e.Dao.ConfigInstance().GetConfigInstance(
		kt,
		payload.BizID,
		&dao.ConfigInstanceSearchCondition{
			ConfigTemplateId: commonPayload.ConfigPayload.ConfigTemplateID,
			CcProcessId:      commonPayload.ProcessPayload.CcProcessID,
			ModuleInstSeq:    commonPayload.ProcessPayload.ModuleInstSeq,
		},
	)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// 3. 推导状态
	var storedMD5 string
	switch {
	case configInstance == nil:
		commonPayload.ConfigPayload.CompareStatus = common.CompareResultNeverPublished
		storedMD5 = "<nil>"
	case configInstance.Attachment.Md5 == actualMD5:
		commonPayload.ConfigPayload.CompareStatus = common.CompareResultSame
		commonPayload.ConfigPayload.ConfigContent = configInstance.Attachment.Content
		storedMD5 = configInstance.Attachment.Md5
	default:
		commonPayload.ConfigPayload.CompareStatus = common.CompareResultDifferent
		storedMD5 = configInstance.Attachment.Md5
	}

	logs.Infof("[CheckConfigMD5 STEP]: compare result, batch_id: %d, actualMD5=%s, storedMD5=%s, status=%s",
		payload.BatchID, actualMD5, storedMD5, commonPayload.ConfigPayload.CompareStatus)

	commonPayload.ConfigPayload.ConfigContentSignature = actualMD5

	if err := c.SetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[Finalize STEP]: set common payload failed: %w", err)
	}

	return nil
}

// FetchConfigContent implements istep.Step.
// nolint:funlen
func (e *CheckConfigExecutor) FetchConfigContent(c *istep.Context) error {
	payload := &CheckConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		logs.Errorf("[FetchConfigContent STEP]: get payload failed, err=%v", err)
		return err
	}

	logs.Infof("[FetchConfigContent STEP]: start, biz_id=%d, batch_id=%d",
		payload.BizID, payload.BatchID)

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[FetchConfigContent STEP]: get common payload failed: %w", err)
	}
	if commonPayload.ConfigPayload == nil {
		logs.Errorf("[FetchConfigContent STEP]: config payload is nil")
		return fmt.Errorf("config payload is nil")
	}

	logs.Infof("[FetchConfigContent STEP]: compare_status=%s, config_key=%s",
		commonPayload.ConfigPayload.CompareStatus,
		commonPayload.ConfigPayload.ConfigInstanceKey)

	// 没有差异直接跳过
	if commonPayload.ConfigPayload.CompareStatus != common.CompareResultDifferent {
		logs.Infof("[FetchConfigContent STEP]: status=%s, skip fetch", commonPayload.ConfigPayload.CompareStatus)
		return nil
	}

	kt := kit.NewWithTenant(payload.TenantID)
	kt.BizID = payload.BizID

	fullPath, err := renderFullPath(commonPayload)
	if err != nil {
		return fmt.Errorf("render full path failed: %w", err)
	}

	fileMode := commonPayload.ConfigPayload.ConfigFileMode
	builder := &ScriptBuilder{FileMode: fileMode}

	script, err := builder.BuildFileCatScript(fullPath)
	if err != nil {
		return err
	}

	scriptName := BuildScriptNameByFileMode("cat", commonPayload, fileMode)
	storeDir := ScriptStoreDirByFileMode(
		e.GseConf.ScriptStoreDir, e.GseConf.WindowsScriptStoreDir, fileMode)
	command := BuildScriptCommand(storeDir, scriptName, fileMode)

	logs.Infof("[FetchConfigContent STEP]: script prepared, batch_id=%d, command=%s, target=%s",
		payload.BatchID, command, fullPath)

	req := &gse.ExecuteScriptReq{
		Agents: []gse.Agent{
			{
				BkAgentID: payload.Process.Attachment.AgentID,
				User:      GetExecutionUser(fileMode, payload.TemplateRevision.Spec.Permission.User),
			},
		},
		Scripts: []gse.Script{
			{
				ScriptName:     scriptName,
				ScriptStoreDir: storeDir,
				ScriptContent:  script,
			},
		},
		AtomicTasks: []gse.AtomicTask{
			{
				Command:        command,
				AtomicTaskID:   0,
				TimeoutSeconds: scriptTimeoutSec,
			},
		},
		AtomicTasksRelations: []gse.AtomicTaskRelation{
			{AtomicTaskID: 0, AtomicTaskIDIdx: []int{}},
		},
	}

	resp, err := e.GseService.AsyncExtensionsExecuteScript(kt.Ctx, req)

	if err != nil {
		logs.Errorf("[FetchConfigContent STEP]: create execute script task failed: %v", err)
		return fmt.Errorf("create execute script task failed: %w", err)
	}

	if resp == nil || resp.Result.TaskID == "" {
		logs.Errorf("[FetchConfigContent STEP]: gse execute script response is nil, batch_id=%d", payload.BatchID)
		return fmt.Errorf("gse execute script response is nil, batch_id=%d", payload.BatchID)
	}

	logs.Infof("[FetchConfigContent STEP]: gse task created, batch_id: %d, task_id: %s",
		payload.BatchID, resp.Result.TaskID)

	result, err := e.WaitExecuteScriptFinish(kt.Ctx, resp.Result.TaskID, payload.Process.Attachment.AgentID)
	if err != nil {
		return fmt.Errorf("wait script execution failed: %w", err)
	}

	if len(result.Result) == 0 {
		return fmt.Errorf("cat script execution result is empty, task_id=%s", resp.Result.TaskID)
	}

	logs.Infof("[FetchConfigContent STEP]: gse full result, batch_id: %d, result: %+v",
		payload.BatchID, result)

	r := result.Result[0]

	if r.ErrorCode != 0 || r.ScriptExitCode != 0 {
		logs.Errorf(
			"[FetchConfigContent STEP]: cat script execution failed, agent=%s, container=%s, "+
				"errorCode=%d, scriptExitCode=%d, msg=%s, screen=%s",
			r.BkAgentID, r.BkContainerID,
			r.ErrorCode, r.ScriptExitCode,
			r.ErrorMsg, r.Screen,
		)
		return fmt.Errorf(
			"cat script execution failed, agent=%s, container=%s, "+
				"errorCode=%d, scriptExitCode=%d, msg=%s, screen=%s",
			r.BkAgentID, r.BkContainerID,
			r.ErrorCode, r.ScriptExitCode,
			r.ErrorMsg, r.Screen,
		)
	}

	commonPayload.ConfigPayload.ConfigContent = r.Screen

	if err := c.SetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[FetchConfigContent STEP]: set common payload failed: %w", err)
	}

	// 这是 配置内容不一致 的业务错误
	return fmt.Errorf("config content inconsistent")
}

// Callback implements istep.Callback.
func (e *CheckConfigExecutor) Callback(c *istep.Context, cbErr error) error {
	logs.Infof("[CheckConfig Callback]: taskID=%s, success=%v", c.GetTaskID(), cbErr == nil)
	if cbErr != nil {
		logs.Errorf("[CheckConfig Callback]: taskID=%s, err=%v", c.GetTaskID(), cbErr)
	}
	payload := &CheckConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("get payload failed: %w", err)
	}

	kt := kit.NewWithTenant(payload.TenantID)

	isSuccess := cbErr == nil
	if _, err := e.Dao.TaskBatch().IncrementCompletedCount(kt, payload.BatchID, isSuccess); err != nil {
		return fmt.Errorf("increment completed count failed, batchID: %d, err: %w",
			payload.BatchID, err)
	}

	e.AfterCallbackNotify(kt.Ctx, common.CallbackNotify{
		TenantID: payload.TenantID,
		BizID:    payload.BizID,
		BatchID:  payload.BatchID,
		Operator: payload.OperatorUser,
		CbErr:    cbErr,
	})

	return nil
}

// RegisterCheckConfigExecutor 注册执行器
func RegisterCheckConfigExecutor(e *CheckConfigExecutor) {
	istep.Register(CheckConfigMD5StepName, istep.StepExecutorFunc(e.CheckConfigMD5))
	istep.Register(FetchConfigContentStepName, istep.StepExecutorFunc(e.FetchConfigContent))
	istep.RegisterCallback(CheckConfigCallbackName, istep.CallbackExecutorFunc(e.Callback))
}
