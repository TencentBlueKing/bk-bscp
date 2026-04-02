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
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bcs"
	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	pushmanager "github.com/TencentBlueKing/bk-bscp/internal/components/push_manager"
	"github.com/TencentBlueKing/bk-bscp/internal/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/repository"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
	"github.com/TencentBlueKing/bk-bscp/render"
)

const (
	// validate push config step name
	ValidatePushConfigStepName istep.StepName = "ValidatePushConfig"
	// download config step name
	DownloadConfigStepName istep.StepName = "DownloadConfig"
	// PushConfigStepName push config step name
	PushConfigStepName istep.StepName = "PushConfig"
	// ReleaseConfigStepName release config step name
	ReleaseConfigStepName istep.StepName = "ReleaseConfig"
)

// CallbackName push config callback name
const CallbackName istep.CallbackName = "Callback"

const scriptTimeoutSec = 3600

// PushConfigExecutor 配置下发执行器
type PushConfigExecutor struct {
	*common.Executor
	Repo     repository.Provider // 仓库服务
	fileLock *lock.FileLock      // 文件锁
}

// NewPushConfigExecutor new push config executor
func NewPushConfigExecutor(dao dao.Set, gseService *gse.Service, cmdbService bkcmdb.Service, repo repository.Provider,
	pm pushmanager.Service) *PushConfigExecutor {
	return &PushConfigExecutor{
		Executor: &common.Executor{
			Dao:         dao,
			GseService:  gseService,
			GseConf:     cc.G().GSE,
			CMDBService: cmdbService,
			PM:          pm,
		},
		Repo:     repo,
		fileLock: lock.NewFileLock(),
	}
}

// PushConfigPayload 配置下发 payload
type PushConfigPayload struct {
	TenantID     string
	BizID        uint32
	BatchID      uint32
	OperateType  table.ConfigOperateType
	OperatorUser string
	Payload      *common.TaskPayload // 配置生成任务 payload
}

// ValidatePushConfig implements istep.Step.
func (e *PushConfigExecutor) ValidatePushConfig(c *istep.Context) error {
	payload := &PushConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		return err
	}

	return nil
}

// ReleaseConfig implements istep.Step.
// ReleaseConfig 通过脚本方式下发配置
func (e *PushConfigExecutor) ReleaseConfig(c *istep.Context) error {
	payload := &PushConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		return err
	}

	kt := kit.NewWithTenant(payload.TenantID)

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("get common payload failed: %w", err)
	}

	if commonPayload.ConfigPayload == nil {
		return fmt.Errorf("config payload not found")
	}

	kt.BizID = payload.BizID

	fileMode := commonPayload.ConfigPayload.ConfigFileMode

	logs.Infof("[ReleaseConfig STEP]: start, biz_id=%d, batch_id=%d, file_mode=%s, "+
		"config_key=%s, agent_id=%s",
		payload.BizID, payload.BatchID, fileMode,
		commonPayload.ConfigPayload.ConfigInstanceKey,
		commonPayload.ProcessPayload.AgentID)

	// 渲染完整文件路径（路径+文件名）
	fullPath, err := renderFullPath(commonPayload)
	if err != nil {
		logs.Errorf("[ReleaseConfig STEP]: render full path failed: %v", err)
		return fmt.Errorf("render full path failed: %w", err)
	}

	builder := &ScriptBuilder{FileMode: fileMode}
	script, err := builder.BuildConfigPushScript(
		base64.StdEncoding.EncodeToString([]byte(commonPayload.ConfigPayload.ConfigContent)),
		fullPath,
		commonPayload.ConfigPayload.ConfigFilePermission,
		commonPayload.ConfigPayload.ConfigFileOwner,
		commonPayload.ConfigPayload.ConfigFileGroup,
	)
	if err != nil {
		logs.Errorf("[ReleaseConfig STEP]: build config script failed: %v", err)
		return err
	}

	storeDir := ScriptStoreDirByFileMode(
		e.GseConf.ScriptStoreDir, e.GseConf.WindowsScriptStoreDir, fileMode)
	scriptName := BuildScriptNameByFileMode("release", commonPayload, fileMode)
	command := BuildScriptCommand(storeDir, scriptName, fileMode)

	executionUser := GetExecutionUser(fileMode, commonPayload.ConfigPayload.ConfigFileOwner)

	logs.Infof("[ReleaseConfig STEP]: script prepared, batch_id=%d, command=%s, user=%s, target=%s",
		payload.BatchID, command, executionUser, fullPath)

	req := &gse.ExecuteScriptReq{
		Agents: []gse.Agent{
			{
				BkAgentID: commonPayload.ProcessPayload.AgentID,
				User:      executionUser,
			},
		},
		Scripts: []gse.Script{
			{ScriptName: scriptName, ScriptStoreDir: storeDir, ScriptContent: script},
		},
		AtomicTasks: []gse.AtomicTask{
			{Command: command, AtomicTaskID: 0, TimeoutSeconds: scriptTimeoutSec},
		},
		AtomicTasksRelations: []gse.AtomicTaskRelation{
			{AtomicTaskID: 0, AtomicTaskIDIdx: []int{}},
		},
	}

	resp, err := e.GseService.AsyncExtensionsExecuteScript(kt.Ctx, req)
	if err != nil {
		logs.Errorf("[ReleaseConfig STEP]: create execute script task failed: %v", err)
		return fmt.Errorf("create execute script task failed: %w", err)
	}

	if resp == nil || resp.Result.TaskID == "" {
		logs.Errorf("[ReleaseConfig STEP]: gse execute script response is nil, batch_id=%d", payload.BatchID)
		return fmt.Errorf("gse execute script response is nil, batch_id=%d", payload.BatchID)
	}

	logs.Infof("[ReleaseConfig STEP]: gse task created, batch_id: %d, task_id: %s, target: %s",
		payload.BatchID, resp.Result.TaskID, fullPath)

	// 通过任务ID查询脚本执行结果
	result, err := e.WaitExecuteScriptFinish(kt.Ctx, resp.Result.TaskID, commonPayload.ProcessPayload.AgentID)
	if err != nil {
		return fmt.Errorf("wait script execution failed: %w", err)
	}
	if len(result.Result) == 0 {
		return fmt.Errorf("script execution result is empty, task_id=%s", resp.Result.TaskID)
	}

	r := result.Result[0]
	if r.ErrorCode != 0 || r.ScriptExitCode != 0 {
		logs.Errorf(
			"[ReleaseConfig STEP]: script execution failed, agent=%s, container=%s, "+
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

	logs.Infof("[ReleaseConfig STEP]: script execution success, batch_id: %d, task_id: %s", payload.BatchID,
		resp.Result.TaskID)

	return nil
}

// Callback implements istep.Callback.
func (e *PushConfigExecutor) Callback(c *istep.Context, cbErr error) error {
	logs.Infof("[PushConfig Callback]: taskID=%s, success=%v", c.GetTaskID(), cbErr == nil)
	if cbErr != nil {
		logs.Errorf("[PushConfig Callback]: taskID=%s, err=%v", c.GetTaskID(), cbErr)
	}

	payload := &PushConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("get payload failed: %w", err)
	}

	kt := kit.NewWithTenant(payload.TenantID)
	kt.BizID = payload.BizID
	kt.User = payload.OperatorUser

	isSuccess := cbErr == nil
	if _, err := e.Dao.TaskBatch().IncrementCompletedCount(kt, payload.BatchID, isSuccess); err != nil {
		return fmt.Errorf("increment completed count failed, batch: %d, err: %w", payload.BatchID, err)
	}

	// 统一推送事件
	e.AfterCallbackNotify(kt.Ctx, common.CallbackNotify{
		TenantID: payload.TenantID,
		BizID:    payload.BizID,
		BatchID:  payload.BatchID,
		Operator: payload.OperatorUser,
		CbErr:    cbErr,
	})

	// 仅配置下发成功才更新配置实例的状态
	if !isSuccess {
		return nil
	}

	cfg := payload.Payload.ConfigPayload
	proc := payload.Payload.ProcessPayload
	now := time.Now()

	instance := &table.ConfigInstance{
		Attachment: &table.ConfigInstanceAttachment{
			BizID:            payload.BizID,
			ConfigTemplateID: cfg.ConfigTemplateID,
			ConfigVersionID:  cfg.ConfigTemplateVersionID,
			CcProcessID:      proc.CcProcessID,
			ModuleInstSeq:    proc.ModuleInstSeq,
			GenerateTaskID:   c.GetTaskID(),
			Md5:              tools.ByteMD5([]byte(cfg.ConfigContent)),
			Content:          cfg.ConfigContent,
			TenantID:         payload.TenantID,
		},
		Revision: &table.Revision{
			Creator:   kt.User,
			Reviser:   kt.User,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := e.Dao.ConfigInstance().Upsert(kt, instance); err != nil {
		return fmt.Errorf("upsert config instance failed: %w", err)
	}

	return nil
}

// RegisterPushConfigExecutor 注册执行器
func RegisterPushConfigExecutor(e *PushConfigExecutor) {
	istep.Register(ValidatePushConfigStepName, istep.StepExecutorFunc(e.ValidatePushConfig))
	istep.Register(ReleaseConfigStepName, istep.StepExecutorFunc(e.ReleaseConfig))
	istep.RegisterCallback(CallbackName, istep.CallbackExecutorFunc(e.Callback))
}

// getServerInfo 获取服务器 AgentID 和 ContainerID(暂时没有用到)
func getServerInfo() (agentID string, containerID string, err error) {
	conf := cc.DataService().GSE

	// 主机部署，直接返回配置的 AgentID
	if conf.NodeAgentID != "" {
		return conf.NodeAgentID, "", nil
	}

	ctx := context.Background()
	retry := tools.NewRetryPolicy(5, [2]uint{3000, 5000})

	var lastErr error
	for retry.RetryCount() < 5 {
		// 查询 Pod
		pod, err := bcs.QueryPod(ctx, conf.ClusterID, conf.PodID)
		if err != nil {
			lastErr = fmt.Errorf("query pod failed: %w", err)
			logs.Warnf("get server info from k8s failed, retry: %d, err: %v", retry.RetryCount(), lastErr)
			retry.Sleep()
			continue
		}

		// 查找容器 ID
		for _, c := range pod.Status.ContainerStatuses {
			if c.Name == conf.ContainerName {
				containerID = tools.SplitContainerID(c.ContainerID)
				break
			}
		}
		if containerID == "" {
			lastErr = fmt.Errorf("container %s not found in pod %s/%s",
				conf.ContainerName, conf.ClusterID, conf.PodID)
			logs.Warnf("get server info from k8s failed, retry: %d, err: %v", retry.RetryCount(), lastErr)
			retry.Sleep()
			continue
		}

		// 查询 Node
		node, err := bcs.QueryNode(ctx, conf.ClusterID, pod.Spec.NodeName)
		if err != nil {
			lastErr = fmt.Errorf("query node failed: %w", err)
			logs.Warnf("get server info from k8s failed, retry: %d, err: %v", retry.RetryCount(), lastErr)
			retry.Sleep()
			continue
		}

		agentID = node.Labels[constant.LabelKeyAgentID]
		if agentID == "" {
			lastErr = fmt.Errorf("agent-id not found in node %s/%s", conf.ClusterID, pod.Spec.NodeName)
			logs.Warnf("get server info from k8s failed, retry: %d, err: %v", retry.RetryCount(), lastErr)
			retry.Sleep()
			continue
		}

		logs.Infof("get server info from k8s success, agent_id: %s, container_id: %s", agentID, containerID)
		return agentID, containerID, nil
	}

	return "", "", fmt.Errorf("get server info failed after 5 retries: %w", lastErr)
}

// PushConfig implements istep.Step.
// PushConfig 通过 GSE 传输文件到目标机器(预留接口，暂未使用)
func (e *PushConfigExecutor) PushConfig(c *istep.Context) error {
	payload := &PushConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		return err
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("get common payload failed: %w", err)
	}

	if commonPayload.ConfigPayload == nil {
		return fmt.Errorf("config payload not found")
	}

	cfg := commonPayload.ConfigPayload
	proc := commonPayload.ProcessPayload

	logs.Infof("[PushConfig STEP]: push config for batch %d, biz_id: %d, config_key: %s",
		payload.BatchID, payload.BizID, cfg.ConfigInstanceKey)

	kt := kit.NewWithTenant(payload.TenantID)
	kt.BizID = payload.BizID

	fullPath, err := renderFullPath(commonPayload)
	if err != nil {
		logs.Errorf("[PushConfig STEP]: render full path failed: %v", err)
		return fmt.Errorf("render full path failed: %w", err)
	}

	// 分离文件名和目录
	renderedFilePath := path.Dir(fullPath)
	renderedFileName := path.Base(fullPath)

	// 构建源文件路径
	cacheDir := cc.G().GSE.CacheDir
	srcDir := path.Join(cacheDir, strconv.Itoa(int(payload.BizID)))
	fileName := cfg.ConfigContentSignature

	// 获取源服务器信息
	srcAgentID, srcContainerID, err := getServerInfo()
	if err != nil {
		logs.Errorf("[PushConfig STEP]: get server info failed: %v", err)
		return fmt.Errorf("get server info failed: %w", err)
	}

	// 构建传输请求
	req := &gse.TransferFileReq{
		TimeOutSeconds: 3600,
		AutoMkdir:      true,
		Tasks: []gse.TransferFileTask{
			{
				Source: gse.TransferFileSource{
					FileName: fileName,
					StoreDir: srcDir,
					Agent: gse.TransferFileAgent{
						BkAgentID:     srcAgentID,
						BkContainerID: srcContainerID,
						User:          cc.G().GSE.AgentUser,
					},
				},
				Target: gse.TransferFileTarget{
					FileName: renderedFileName,
					StoreDir: renderedFilePath,
					Agents: []gse.TransferFileAgent{
						{
							BkAgentID: proc.AgentID,
							User:      cfg.ConfigFileOwner,
						},
					},
				},
			},
		},
	}

	// 调用 GSE 传输文件
	resp, err := e.GseService.AsyncExtensionsTransferFile(kt.Ctx, req)
	if err != nil {
		logs.Errorf("[PushConfig STEP]: create transfer task failed: %v", err)
		return fmt.Errorf("create transfer task failed: %w", err)
	}

	logs.Infof("[PushConfig STEP]: gse task created, batch_id: %d, task_id: %s, target: %s",
		payload.BatchID, resp.Result.TaskID, fullPath)

	// 等待传输完成
	result, err := e.WaitTransferFileTaskFinish(kt.Ctx, resp.Result.TaskID)
	if err != nil {
		return fmt.Errorf("wait transfer task failed: %w", err)
	}

	// 检查传输结果
	for _, r := range result.Result {
		if r.ErrorCode != 0 {
			logs.Errorf("[PushConfig STEP]: transfer failed, agent: %s, code: %d, msg: %s",
				r.Content.DestAgentID, r.ErrorCode, r.ErrorMsg)
			return fmt.Errorf("transfer failed, agent: %s, code: %d, msg: %s",
				r.Content.DestAgentID, r.ErrorCode, r.ErrorMsg)
		}
	}

	logs.Infof("[PushConfig STEP]: transfer success, batch_id: %d, task_id: %s", payload.BatchID, resp.Result.TaskID)
	return nil
}

// DownloadConfig implements istep.Step.
// DownloadConfig 下载配置文件到本地(暂时没有用到)
func (e *PushConfigExecutor) DownloadConfig(c *istep.Context) error {
	payload := &PushConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		return err
	}

	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("get common payload failed: %w", err)
	}

	if commonPayload.ConfigPayload == nil {
		return fmt.Errorf("config payload not found")
	}

	cfg := commonPayload.ConfigPayload
	content := cfg.ConfigContent
	signature := cfg.ConfigContentSignature

	logs.Infof("download config for batch %d, biz_id: %d, config_key: %s",
		payload.BatchID, payload.BizID, cfg.ConfigInstanceKey)

	cacheDir := cc.G().GSE.CacheDir
	dir := path.Join(cacheDir, strconv.Itoa(int(payload.BizID)))
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("create directory failed: %w", err)
	}

	filePath := path.Join(dir, signature)

	// 文件锁避免并发写入
	e.fileLock.Acquire(filePath)
	defer e.fileLock.Release(filePath)

	// 文件已存在则跳过
	if _, err := os.Stat(filePath); err == nil {
		logs.Infof("config file exists, skip writing: %s", filePath)
		return nil
	}

	if err := os.WriteFile(filePath, []byte(content), os.ModePerm); err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}

	logs.Infof("write config file success: %s", filePath)
	return nil
}

// renderFullPath 渲染完整文件路径
// 支持 Windows 和 Linux 两种路径格式：
//   - 内部统一使用 `/` 进行 path.Join，避免 POSIX path 库无法处理 `\` 的问题
//   - Windows 平台最终输出转回 `\`
func renderFullPath(commonPayload *common.TaskPayload) (string, error) {
	cfg := commonPayload.ConfigPayload
	proc := commonPayload.ProcessPayload

	// 统一将反斜杠转为正斜杠，确保 POSIX path.Join 能正确处理
	filePath := strings.ReplaceAll(cfg.ConfigFilePath, `\`, "/")
	fileName := strings.ReplaceAll(cfg.ConfigFileName, `\`, "/")

	fullPath := path.Join(filePath, fileName)
	// 不包含变量则无需渲染
	if !strings.Contains(fullPath, "${") {
		if cfg.ConfigFileMode == table.Windows {
			return ToWindowsPath(fullPath), nil
		}
		return fullPath, nil
	}

	// 构建渲染上下文参数
	contextParams := render.ProcessContextParams{
		SetName:       proc.SetName,
		ModuleName:    proc.ModuleName,
		ServiceName:   proc.ServiceName,
		ProcessName:   proc.Alias,
		ProcessID:     int(proc.CcProcessID),
		FuncName:      proc.FuncName,
		HostInnerIP:   proc.InnerIP,
		HostInstSeq:   int(proc.HostInstSeq),
		ModuleInstSeq: int(proc.ModuleInstSeq),
		CloudID:       proc.CloudID,
		// 不需要 HELP，因为文件名/路径中不应该包含 HELP
		WithHelp: false,
	}

	renderedPath, err := render.Template(fullPath, contextParams)
	if err != nil {
		return "", fmt.Errorf("render full path failed: %w", err)
	}

	// Windows 平台转回反斜杠
	if cfg.ConfigFileMode == table.Windows {
		renderedPath = ToWindowsPath(renderedPath)
	}

	logs.Infof("render full path: %s -> %s", fullPath, renderedPath)
	return renderedPath, nil
}
