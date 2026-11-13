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
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"
	"golang.org/x/text/encoding/simplifiedchinese"
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
			TaskConf:    cc.G().TaskFramework,
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

	fullPath, err := renderFullPath(kt.Ctx, commonPayload)
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

	executionUser := GetExecutionUser(fileMode)

	logs.Infof("[CheckConfigMD5 STEP]: script prepared, batch_id=%d, command=%s, user=%s, target=%s",
		payload.BatchID, command, executionUser, fullPath)

	req := &gse.ExecuteScriptReq{
		Agents: []gse.Agent{
			{
				BkAgentID: payload.Process.Attachment.AgentID,
				User:      executionUser,
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
				TimeoutSeconds: e.TaskConf.ScriptExecution.TimeoutSec,
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

	// 快照「最后一次下发版本」：config_instances 永远保存最新下发记录，
	// 检查时刻读取即为上一次发版的内容，连同 md5、下发时间写入 payload 作为不可变快照，
	// 供 GetConfigDiff 对比，避免后续下发覆盖配置实例后历史任务的 diff 失真。
	// 从未下发（configInstance == nil）时不快照，lastDispatched 保持为空
	if configInstance != nil {
		commonPayload.ConfigPayload.LastDispatchedContent = configInstance.Attachment.Content
		commonPayload.ConfigPayload.LastDispatchedSignature = configInstance.Attachment.Md5
		commonPayload.ConfigPayload.LastDispatchedAt = configInstance.Revision.UpdatedAt
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

	fullPath, err := renderFullPath(kt.Ctx, commonPayload)
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

	executionUser := GetExecutionUser(fileMode)

	logs.Infof("[FetchConfigContent STEP]: script prepared, batch_id=%d, command=%s, user=%s, target=%s",
		payload.BatchID, command, executionUser, fullPath)

	req := &gse.ExecuteScriptReq{
		Agents: []gse.Agent{
			{
				BkAgentID: payload.Process.Attachment.AgentID,
				User:      executionUser,
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
				TimeoutSeconds: e.TaskConf.ScriptExecution.TimeoutSec,
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
				"errorCode=%d, scriptExitCode=%d, msg=%s",
			r.BkAgentID, r.BkContainerID,
			r.ErrorCode, r.ScriptExitCode,
			r.ErrorMsg,
		)
		return fmt.Errorf(
			"cat script execution failed, agent=%s, container=%s, "+
				"errorCode=%d, scriptExitCode=%d, msg=%s, screen=%s",
			r.BkAgentID, r.BkContainerID,
			r.ErrorCode, r.ScriptExitCode,
			r.ErrorMsg, r.Screen,
		)
	}

	content, err := catOutputToConfigContent(r.Screen, fileMode, commonPayload.ConfigPayload.Charset)
	if err != nil {
		logs.Errorf("[FetchConfigContent STEP]: decode cat output failed, batch_id=%d, err=%v",
			payload.BatchID, err)
		return fmt.Errorf("[FetchConfigContent STEP]: read current config content failed: %w", err)
	}
	commonPayload.ConfigPayload.ConfigContent = content

	if err := c.SetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[FetchConfigContent STEP]: set common payload failed: %w", err)
	}

	// 这是 配置内容不一致 的业务错误
	return fmt.Errorf("config content inconsistent")
}

// catOutputToConfigContent 把读取脚本上报的 screen 转成可展示的配置内容。
// Windows 脚本只上报文件字节的 Base64，避免 Windows 脚本输出采集链路中的代码页转换影响：
// 脚本输出若被按系统 ANSI 代码页解释，UTF-8 的韩文、日文、俄文可能被二次解码成乱码，
// 而 Base64 是纯 ASCII，不受代码页影响。
// 解码失败说明输出被截断、夹杂了其它信息或采集异常，此时必须报错：退回原始 screen 会把
// Base64 或 PowerShell 报错当成现网配置展示，既制造虚假差异，也掩盖了采集本身失败。
// Linux 脚本直接 cat，Agent 不做转码，保持原样。
func catOutputToConfigContent(screen string, fileMode table.FileMode,
	charset table.FileCharset) (string, error) {
	if fileMode != table.Windows {
		return screen, nil
	}

	raw, err := decodeBase64Screen(screen)
	if err != nil {
		return "", fmt.Errorf("decode cat output as base64 failed: %w, screen=%s",
			err, truncateForScreen(screen, 128))
	}
	return toDisplayConfigContent(raw, charset), nil
}

// decodeBase64Screen 解析脚本上报的「Base64 + 原始字节数」输出。GSE 的 screen 可能带换行或
// 首尾空白，按空白切分后最后一段是脚本写入的字节数。
// 字节数同时是格式校验与完整性校验：screen 在采集链路里被截断时末尾字节数会一起丢失，
// 或与解码结果长度不一致。原文长度是 3 的倍数时 Base64 不带 padding，截断到 4 字符组边界
// 仍能解码成功，只检查解码错误是发现不了这种截断的。
func decodeBase64Screen(screen string) ([]byte, error) {
	fields := strings.Fields(screen)
	if len(fields) == 0 {
		return nil, errors.New("cat output is empty")
	}

	lengthToken := fields[len(fields)-1]
	want, err := strconv.Atoi(lengthToken)
	if err != nil {
		return nil, fmt.Errorf("cat output missing byte length trailer, got %q", lengthToken)
	}

	raw, err := base64.StdEncoding.DecodeString(strings.Join(fields[:len(fields)-1], ""))
	if err != nil {
		return nil, err
	}

	if len(raw) != want {
		return nil, fmt.Errorf("cat output is incomplete, got %d bytes, want %d", len(raw), want)
	}
	return raw, nil
}

// truncateForScreen 截断要写进错误信息的 screen，避免把整份 Base64 塞进任务错误里。
// 截断后按合法 UTF-8 修正，防止错误信息里出现半个多字节字符。
func truncateForScreen(screen string, max int) string {
	screen = strings.TrimSpace(screen)
	if len(screen) <= max {
		return screen
	}
	return strings.ToValidUTF8(screen[:max], "") + "..."
}

// toDisplayConfigContent 把文件原始字节转成可展示的 UTF-8 文本：
//   - 去掉下发时补的 UTF-8 BOM，避免与「最后一次下发」内容对比时多出看不见的首行差异；
//   - 合法 UTF-8 原样返回，韩文、日文、俄文等无需额外处理；
//   - GBK 字符集按 GBK 解码，否则 GBK 配置在对比页会显示成乱码；
//   - 其余非法字节替换为 U+FFFD，保证返回值一定是合法 UTF-8，protobuf/JSON 不会因此报错。
func toDisplayConfigContent(raw []byte, charset table.FileCharset) string {
	content := raw
	// 只有「去掉 BOM 后仍是合法 UTF-8」才认为是下发补的 BOM：GBK 文件的首字节行里
	// 也可能出现 EF BB BF，按 GBK 解释时它们属于正文，不能剥掉。
	if rest := bytes.TrimPrefix(content, []byte(utf8BOM)); len(rest) != len(content) && utf8.Valid(rest) {
		content = rest
	}
	if utf8.Valid(content) {
		return string(content)
	}
	if charset == table.GBK {
		if decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(content); err == nil {
			return string(decoded)
		}
	}
	return strings.ToValidUTF8(string(content), "\ufffd")
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
	allCompleted, err := e.Dao.TaskBatch().IncrementCompletedCount(kt, payload.BatchID, isSuccess)
	if err != nil {
		return fmt.Errorf("increment completed count failed, batchID: %d, err: %w",
			payload.BatchID, err)
	}

	// 只在批次收尾的那次回调发出通知，避免异步通知重复推送
	if allCompleted {
		e.AfterCallbackNotify(kt.Ctx, common.CallbackNotify{
			TenantID: payload.TenantID,
			BizID:    payload.BizID,
			BatchID:  payload.BatchID,
			Operator: payload.OperatorUser,
			CbErr:    cbErr,
		})
	}

	return nil
}

// RegisterCheckConfigExecutor 注册执行器
func RegisterCheckConfigExecutor(e *CheckConfigExecutor) {
	istep.Register(CheckConfigMD5StepName, istep.StepExecutorFunc(e.CheckConfigMD5))
	istep.Register(FetchConfigContentStepName, istep.StepExecutorFunc(e.FetchConfigContent))
	istep.RegisterCallback(CheckConfigCallbackName, istep.CallbackExecutorFunc(e.Callback))
}
