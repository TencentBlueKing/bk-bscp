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
	"path"
	"strings"
	"time"

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"
	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	pushmanager "github.com/TencentBlueKing/bk-bscp/internal/components/push_manager"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
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
	md5ScriptTmpl              string         = "bk_ges_check_config_md5_%d.sh"
	catScriptTmpl              string         = "bk_ges_cat_config_%d.sh"
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
			CMDBService: cmdbService,
			PM:          pm,
		},
	}
}

// CheckConfigPayload check config step payload
type CheckConfigPayload struct {
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

	kt := kit.New()
	kt.BizID = payload.BizID

	script, err := buildFileMD5Script(
		path.Join(
			payload.TemplateRevision.Spec.Path,
			payload.TemplateRevision.Spec.Name,
		),
	)
	if err != nil {
		return err
	}

	scriptName := fmt.Sprintf(md5ScriptTmpl, time.Now().Unix())
	scriptStoreDir := e.GseConf.ScriptStoreDir

	resp, err := e.GseService.AsyncExtensionsExecuteScript(kt.Ctx, &gse.ExecuteScriptReq{
		Agents: []gse.Agent{
			{
				BkAgentID: payload.Process.Attachment.AgentID,
				User:      payload.TemplateRevision.Spec.Permission.User,
			},
		},
		Scripts: []gse.Script{
			{
				ScriptName:     scriptName,
				ScriptStoreDir: scriptStoreDir,
				ScriptContent:  script,
			},
		},
		AtomicTasks: []gse.AtomicTask{
			{
				Command:        path.Join(scriptStoreDir, scriptName),
				AtomicTaskID:   0,
				TimeoutSeconds: scriptTimeoutSec,
			},
		},
		AtomicTasksRelations: []gse.AtomicTaskRelation{
			{AtomicTaskID: 0, AtomicTaskIDIdx: []int{}},
		},
	})

	if err != nil {
		logs.Errorf("[CheckConfigMD5 STEP]: create execute script task failed: %v", err)
		return fmt.Errorf("create md5 execute script task failed: %w", err)
	}

	if resp == nil || resp.Result.TaskID == "" {
		logs.Errorf("[CheckConfigMD5 STEP]: gse execute script response is nil, batch_id=%d", payload.BatchID)
		return fmt.Errorf("gse execute script response is nil, batch_id=%d", payload.BatchID)
	}

	logs.Infof("[CheckConfigMD5 STEP]: gse task created, batch_id: %d, task_id: %s, target: %s",
		payload.BatchID, resp.Result.TaskID, path.Join(payload.TemplateRevision.Spec.Path,
			payload.TemplateRevision.Spec.Name))

	// 通过脚本任务ID获取脚本执行结果
	result, err := e.WaitExecuteScriptFinish(kt.Ctx, resp.Result.TaskID, payload.Process.Attachment.AgentID)
	if err != nil {
		return fmt.Errorf("wait script execution failed: %w", err)
	}

	if len(result.Result) == 0 {
		return fmt.Errorf("script execution result is empty, task_id=%s", resp.Result.TaskID)
	}

	r := result.Result[0]
	if r.ErrorCode != 0 {
		logs.Errorf(
			"[checkConfigMD5 Callback]: script execution failed, agent=%s, container=%s, code=%d, msg=%s",
			r.BkAgentID,
			r.BkContainerID,
			r.ErrorCode,
			r.ErrorMsg,
		)
		return fmt.Errorf(
			"script execution failed, agent=%s, container=%s, code=%d, msg=%s",
			r.BkAgentID,
			r.BkContainerID,
			r.ErrorCode,
			r.ErrorMsg,
		)
	}

	actualMD5 := strings.TrimSpace(r.Screen)

	// 2. 查询历史下发记录
	commonPayload := &common.TaskPayload{}
	if errC := c.GetCommonPayload(commonPayload); errC != nil {
		return errC
	}

	if commonPayload.ConfigPayload == nil {
		return fmt.Errorf("script execution failed, config payload nil")
	}
	if commonPayload.ProcessPayload == nil {
		return fmt.Errorf("script execution failed, process payload nil")
	}

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
	switch {
	case configInstance == nil:
		commonPayload.ConfigPayload.CompareStatus = common.CompareResultNeverPublished
	case configInstance.Attachment.Md5 == actualMD5:
		commonPayload.ConfigPayload.CompareStatus = common.CompareResultSame
	default:
		commonPayload.ConfigPayload.CompareStatus = common.CompareResultDifferent
	}

	commonPayload.ConfigPayload.ConfigContentSignature = actualMD5

	if err := c.SetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[Finalize STEP]: set common payload failed: %w", err)
	}

	return nil
}

// FetchConfigContent implements istep.Step.
func (e *CheckConfigExecutor) FetchConfigContent(c *istep.Context) error {
	payload := &CheckConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		logs.Errorf("[FetchConfigContent Execute]: fetch config conten payload nil")
		return err
	}
	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[Finalize STEP]: get common payload failed: %w", err)
	}
	if commonPayload.ConfigPayload == nil {
		logs.Errorf("[FetchConfigContent Execute]: fetch config conten config payload nil")
		return fmt.Errorf("fetch config conten config payload nil")
	}

	// 没有差异直接跳过
	if commonPayload.ConfigPayload.CompareStatus != common.CompareResultDifferent {
		logs.Infof("[FetchConfigContent]: md5 matched, skip")
		return nil
	}

	kt := kit.New()
	kt.BizID = payload.BizID

	script, err := buildFileCatScript(
		path.Join(
			payload.TemplateRevision.Spec.Path,
			payload.TemplateRevision.Spec.Name,
		),
	)
	if err != nil {
		return err
	}

	scriptName := fmt.Sprintf(catScriptTmpl, time.Now().Unix())
	scriptStoreDir := e.GseConf.ScriptStoreDir

	resp, err := e.GseService.AsyncExtensionsExecuteScript(kt.Ctx, &gse.ExecuteScriptReq{
		Agents: []gse.Agent{
			{
				BkAgentID: payload.Process.Attachment.AgentID,
				User:      payload.TemplateRevision.Spec.Permission.User,
			},
		},
		Scripts: []gse.Script{
			{
				ScriptName:     scriptName,
				ScriptStoreDir: scriptStoreDir,
				ScriptContent:  script,
			},
		},
		AtomicTasks: []gse.AtomicTask{
			{
				Command:        path.Join(scriptStoreDir, scriptName),
				AtomicTaskID:   0,
				TimeoutSeconds: scriptTimeoutSec,
			},
		},
		AtomicTasksRelations: []gse.AtomicTaskRelation{
			{AtomicTaskID: 0, AtomicTaskIDIdx: []int{}},
		},
	})

	if err != nil {
		logs.Errorf("[FetchConfigContent STEP]: create execute script task failed: %v", err)
		return fmt.Errorf("create execute script task failed: %w", err)
	}

	if resp == nil || resp.Result.TaskID == "" {
		logs.Errorf("[FetchConfigContent STEP]: gse execute script response is nil, batch_id=%d", payload.BatchID)
		return fmt.Errorf("gse execute script response is nil, batch_id=%d", payload.BatchID)
	}

	// 存在差异化
	result, err := e.WaitExecuteScriptFinish(kt.Ctx, resp.Result.TaskID, payload.Process.Attachment.AgentID)
	if err != nil {
		return fmt.Errorf("wait script execution failed: %w", err)
	}

	if len(result.Result) == 0 {
		return fmt.Errorf("cat script execution result is empty, task_id=%s", resp.Result.TaskID)
	}

	r := result.Result[0]
	if r.ErrorCode != 0 {
		logs.Errorf(
			"[FetchConfigContent STEP]: cat script execution failed, agent=%s, container=%s, code=%d, msg=%s",
			r.BkAgentID,
			r.BkContainerID,
			r.ErrorCode,
			r.ErrorMsg,
		)
		return fmt.Errorf(
			"cat script execution failed, agent=%s, container=%s, code=%d, msg=%s",
			r.BkAgentID,
			r.BkContainerID,
			r.ErrorCode,
			r.ErrorMsg,
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
	logs.Infof("[CheckConfig Callback]: start callback processing")
	payload := &CheckConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("get payload failed: %w", err)
	}

	// 更新 TaskBatch 状态
	isSuccess := cbErr == nil
	if err := e.Dao.TaskBatch().IncrementCompletedCount(kit.New(), payload.BatchID, isSuccess); err != nil {
		return fmt.Errorf("increment completed count failed, batchID: %d, err: %w",
			payload.BatchID, err)
	}

	// 统一推送事件
	e.AfterCallbackNotify(c.Context(), common.CallbackNotify{
		BizID:    payload.BizID,
		BatchID:  payload.BatchID,
		Operator: payload.OperatorUser,
		CbErr:    cbErr,
	})

	logs.Infof(
		"[CheckConfig Callback] finished, taskID=%s, batchID=%d",
		c.GetTaskID(), payload.BatchID,
	)

	return nil
}

// RegisterCheckConfigExecutor 注册执行器
func RegisterCheckConfigExecutor(e *CheckConfigExecutor) {
	istep.Register(CheckConfigMD5StepName, istep.StepExecutorFunc(e.CheckConfigMD5))
	istep.Register(FetchConfigContentStepName, istep.StepExecutorFunc(e.FetchConfigContent))
	istep.RegisterCallback(CheckConfigCallbackName, istep.CallbackExecutorFunc(e.Callback))
}

// buildFileMD5Script 构建计算文件MD5的脚本
func buildFileMD5Script(absPath string) (string, error) {
	if !strings.HasPrefix(absPath, "/") {
		return "", fmt.Errorf("absPath must be absolute")
	}

	return fmt.Sprintf(`#!/bin/bash
set -euo pipefail

TARGET_PATH=%s

md5sum "$TARGET_PATH" | awk '{print $1}'
`,
		shellQuote(absPath),
	), nil
}

// buildFileCatScript 构建cat文件内容的脚本
func buildFileCatScript(absPath string) (string, error) {
	if !strings.HasPrefix(absPath, "/") {
		return "", fmt.Errorf("absPath must be absolute")
	}

	return fmt.Sprintf(`#!/bin/bash
set -euo pipefail

TARGET_PATH=%s

cat "$TARGET_PATH"
`,
		shellQuote(absPath),
	), nil
}
