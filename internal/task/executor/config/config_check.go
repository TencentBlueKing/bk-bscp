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

	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
)

const (
	// CheckConfigStepName check config step name
	CheckConfigStepName istep.StepName = "CheckConfig"
	scriptTmpl          string         = "bk_ges_check_config_%d.sh"
	// CheckConfigCallbackName check config callback name
	CheckConfigCallbackName istep.CallbackName = "CheckConfigCallback"
)

// CheckConfigExecutor 配置检查执行器
type CheckConfigExecutor struct {
	*common.Executor
}

// NewCheckConfigExecutor new check config executor
func NewCheckConfigExecutor(dao dao.Set, gseService *gse.Service) *CheckConfigExecutor {
	return &CheckConfigExecutor{
		Executor: &common.Executor{
			Dao:        dao,
			GseService: gseService,
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
	TaskID             string
}

// CheckConfig implements istep.Step.
func (e *CheckConfigExecutor) CheckConfig(c *istep.Context) error {
	kt := kit.New()
	logs.Infof("[CheckConfig STEP]: start check config")
	payload := &CheckConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		logs.Errorf("[CheckConfig STEP]: get payload failed: %v", err)
		return err
	}

	kt.BizID = payload.BizID

	script, err := buildFileCatScript(path.Join(payload.TemplateRevision.Spec.Path, payload.TemplateRevision.Spec.Name))
	if err != nil {
		logs.Errorf("[CheckConfig STEP]: build read file script failed: %v", err)
		return fmt.Errorf("build read file script failed: %w", err)
	}

	scriptStoreDir := e.GseConf.ScriptStoreDir
	scriptName := fmt.Sprintf(scriptTmpl, time.Now().Unix())

	resp, err := e.GseService.AsyncExtensionsExecuteScript(kt.Ctx, &gse.ExecuteScriptReq{
		Agents: []gse.Agent{
			{
				BkAgentID: payload.Process.Attachment.AgentID,
				User:      payload.TemplateRevision.Spec.Permission.User,
			},
		},
		Scripts: []gse.Script{
			{ScriptName: scriptName, ScriptStoreDir: scriptStoreDir, ScriptContent: script},
		},
		AtomicTasks: []gse.AtomicTask{
			{Command: path.Join(scriptStoreDir, scriptName), AtomicTaskID: 0, TimeoutSeconds: scriptTimeoutSec},
		},
		AtomicTasksRelations: []gse.AtomicTaskRelation{
			{AtomicTaskID: 0, AtomicTaskIDIdx: []int{}},
		},
	})

	if err != nil {
		logs.Errorf("[CheckConfig STEP]: create execute script task failed: %v", err)
		return fmt.Errorf("create execute script task failed: %w", err)
	}

	if resp == nil || resp.Result.TaskID == "" {
		logs.Errorf("[CheckConfig STEP]: gse execute script response is nil, batch_id=%d", payload.BatchID)
		return fmt.Errorf("gse execute script response is nil, batch_id=%d", payload.BatchID)
	}

	logs.Infof("[CheckConfig STEP]: gse task created, batch_id: %d, task_id: %s, target: %s",
		payload.BatchID, resp.Result.TaskID, path.Join(payload.TemplateRevision.Spec.Path,
			payload.TemplateRevision.Spec.Name))

	payload.TaskID = resp.Result.TaskID

	err = c.SetPayload(payload)
	if err != nil {
		logs.Errorf("[CheckConfig STEP]: set common payload failed: %v", err)
		return err
	}

	logs.Infof("[CheckConfig STEP]: script execution success, batch_id: %d, task_id: %s", payload.BatchID,
		resp.Result.TaskID)

	return nil
}

// Callback implements istep.Callback.
func (e *CheckConfigExecutor) Callback(c *istep.Context, cbErr error) error {
	logs.Infof("[CheckConfig Callback]: start callback processing")
	payload := &CheckConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("get payload failed: %w", err)
	}

	kit := kit.FromGrpcContext(c.Context())
	kit.BizID = payload.BizID
	kit.User = payload.OperatorUser

	// 通过脚本任务ID获取脚本执行结果
	result, err := e.WaitExecuteScriptFinish(kit.Ctx, payload.TaskID, payload.Process.Attachment.AgentID)
	if err != nil {
		return fmt.Errorf("wait script execution failed: %w", err)
	}

	if len(result.Result) == 0 {
		return fmt.Errorf("script execution result is empty, task_id=%s", payload.TaskID)
	}

	r := result.Result[0]
	if r.ErrorCode != 0 {
		logs.Errorf(
			"[CheckConfig STEP]: script execution failed, agent=%s, container=%s, code=%d, msg=%s",
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

	commonPayload := &common.TaskPayload{}
	if errP := c.GetCommonPayload(commonPayload); errP != nil {
		return fmt.Errorf("[CheckConfig STEP]: get common payload failed: %w", errP)
	}

	cfg := commonPayload.ConfigPayload
	if cfg == nil {
		return fmt.Errorf("[CheckConfig STEP]: config payload is nil")
	}

	proc := commonPayload.ProcessPayload
	if proc == nil {
		return fmt.Errorf("[CheckConfig STEP]: process payload is nil")
	}

	// 获取最后一次下发的md5
	configInstance, err := e.Dao.ConfigInstance().GetConfigInstance(kit, payload.BizID, &dao.ConfigInstanceSearchCondition{
		ConfigTemplateId: cfg.ConfigTemplateID,
		CcProcessId:      proc.CcProcessID,
		ModuleInstSeq:    proc.ModuleInstSeq,
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// 计算内容 hash
	contentHash := tools.SHA256(r.Screen)

	// 默认：一致 + 成功
	cfg.CompareStatus = common.CompareResultSame
	cfg.ConfigContentSignature = contentHash

	var taskErr error

	switch {
	case configInstance == nil:
		// 从未下发过
		cfg.CompareStatus = common.CompareResultNeverPublished
	case configInstance.Attachment.Md5 == contentHash:
		// 已下发且一致, 什么都不做
	default:
		// 已下发但内容不一致, 失败
		cfg.CompareStatus = common.CompareResultDifferent
		cfg.ConfigContent = r.Screen

		taskErr = fmt.Errorf("config content inconsistent with last release")
	}

	if err := c.SetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[CheckConfig STEP]: set common payload failed: %w", err)
	}

	isSuccess := taskErr == nil
	if err := e.Dao.TaskBatch().IncrementCompletedCount(kit, payload.BatchID, isSuccess); err != nil {
		logs.Errorf(
			"[CheckConfig Callback]: increment completed count failed, batch_id=%d, success=%v, err=%v",
			payload.BatchID,
			isSuccess,
			err,
		)
		return fmt.Errorf("increment completed count failed: %w", err)
	}

	if taskErr != nil {
		logs.Warnf(
			"[CheckConfig Callback]: config check failed, batch_id=%d, reason=%v",
			payload.BatchID,
			taskErr,
		)
		return taskErr
	}

	return nil
}

// RegisterCheckConfigExecutor 注册执行器
func RegisterCheckConfigExecutor(e *CheckConfigExecutor) {
	istep.Register(CheckConfigStepName, istep.StepExecutorFunc(e.CheckConfig))
	istep.RegisterCallback(CheckConfigCallbackName, istep.CallbackExecutorFunc(e.Callback))
}

// buildFileMD5Script 构建计算文件MD5的脚本
// nolint:unused
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
