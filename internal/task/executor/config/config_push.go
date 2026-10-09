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
	"path/filepath"
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
	// BackupConfigStepName 备份目标机器现有配置步骤名。
	// 仅 PushConfig（GSE 文件传输）下发前执行；ReleaseConfig 脚本内置备份，不走该步骤
	BackupConfigStepName istep.StepName = "BackupConfig"
	// ReleaseConfigStepName release config step name
	ReleaseConfigStepName istep.StepName = "ReleaseConfig"
)

const (
	// PushCacheNamespace 配置下发（data-service PushConfig）在 GSE 缓存目录下的独立命名空间。
	// 与 feed-server 异步下载源文件缓存分树：
	// 一方面避免 CacheCleaner 全树按 mtime 清理时误删传输中的下发源文件，
	// 另一方面隔离两边的缓存布局（异步下载为 {CacheDir}/{bizID}/{源文件名}，
	// 配置下发为 {CacheDir}/{PushCacheNamespace}/{bizID}/{内容签名}/{文件名}）
	PushCacheNamespace = "config-push"
)

// CallbackName push config callback name
const CallbackName istep.CallbackName = "Callback"

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
			TaskConf:    cc.G().TaskFramework,
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

	// 机器类型以目标机器 os_type（processes 表）为准，os_type 为空时回退配置模板 FileMode
	fileMode := FileModeFromOsType(commonPayload.ProcessPayload.OsType, commonPayload.ConfigPayload.ConfigFileMode)

	logs.Infof("[ReleaseConfig STEP]: start, biz_id=%d, batch_id=%d, file_mode=%s, "+
		"config_key=%s, agent_id=%s",
		payload.BizID, payload.BatchID, fileMode,
		commonPayload.ConfigPayload.ConfigInstanceKey,
		commonPayload.ProcessPayload.AgentID)

	// 渲染完整文件路径（路径+文件名）
	fullPath, err := renderFullPath(kt.Ctx, commonPayload)
	if err != nil {
		logs.Errorf("[ReleaseConfig STEP]: render full path failed: %v", err)
		return fmt.Errorf("render full path failed: %w", err)
	}

	builder := &ScriptBuilder{FileMode: fileMode, MaxBackups: e.GseConf.MaxBackups}
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

	executionUser := GetExecutionUser(fileMode)

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
			{Command: command, AtomicTaskID: 0, TimeoutSeconds: e.TaskConf.ScriptExecution.TimeoutSec},
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
	allCompleted, err := e.Dao.TaskBatch().IncrementCompletedCount(kt, payload.BatchID, isSuccess)
	if err != nil {
		return fmt.Errorf("increment completed count failed, batch: %d, err: %w", payload.BatchID, err)
	}

	// 统一推送事件，只在批次收尾的那次回调发出，避免异步通知重复推送
	if allCompleted {
		e.AfterCallbackNotify(kt.Ctx, common.CallbackNotify{
			TenantID: payload.TenantID,
			BizID:    payload.BizID,
			BatchID:  payload.BatchID,
			Operator: payload.OperatorUser,
			CbErr:    cbErr,
		})
	}

	// 仅配置下发成功才更新配置实例的状态
	if !isSuccess {
		return nil
	}

	if payload.Payload == nil || payload.Payload.ConfigPayload == nil || payload.Payload.ProcessPayload == nil {
		return fmt.Errorf("task payload not found, task_id=%s", c.GetTaskID())
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
	istep.Register(BackupConfigStepName, istep.StepExecutorFunc(e.BackupConfig))
	istep.Register(ReleaseConfigStepName, istep.StepExecutorFunc(e.ReleaseConfig))
	istep.Register(PushConfigStepName, istep.StepExecutorFunc(e.PushConfig))
	istep.RegisterCallback(CallbackName, istep.CallbackExecutorFunc(e.Callback))
}

// BackupConfig implements istep.Step.
// 在目标机器上备份现有配置文件并清理旧备份，供 PushConfig（GSE 文件传输）下发前执行：
// 文件传输会直接覆盖目标文件，覆盖前先落一份备份，避免误下发后无法回退。
// 备份为尽力而为：任何失败（渲染、脚本构建、GSE 执行、退出码非 0）只记录日志，
// 不阻断后续下发步骤；目标文件不存在（首次下发）时脚本以 0 退出。
func (e *PushConfigExecutor) BackupConfig(c *istep.Context) error {
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

	kt := kit.NewWithTenant(payload.TenantID)
	kt.BizID = payload.BizID

	// 机器类型与下发步骤保持一致：目标机器 os_type 优先，为空时回退配置模板 FileMode
	fileMode := FileModeFromOsType(commonPayload.ProcessPayload.OsType, commonPayload.ConfigPayload.ConfigFileMode)

	fullPath, err := renderFullPath(kt.Ctx, commonPayload)
	if err != nil {
		logs.Errorf("[BackupConfig STEP]: render full path failed, skip backup, batch_id=%d, err: %v",
			payload.BatchID, err)
		return nil
	}

	builder := &ScriptBuilder{FileMode: fileMode}
	script, err := builder.BuildBackupScript(fullPath, e.GseConf.MaxBackups)
	if err != nil {
		logs.Errorf("[BackupConfig STEP]: build backup script failed, skip backup, batch_id=%d, err: %v",
			payload.BatchID, err)
		return nil
	}

	scriptName := BuildScriptNameByFileMode("backup", commonPayload, fileMode)
	storeDir := ScriptStoreDirByFileMode(
		e.GseConf.ScriptStoreDir, e.GseConf.WindowsScriptStoreDir, fileMode)
	command := BuildScriptCommand(storeDir, scriptName, fileMode)
	executionUser := GetExecutionUser(fileMode)

	logs.Infof("[BackupConfig STEP]: script prepared, batch_id=%d, command=%s, user=%s, target=%s",
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
			{Command: command, AtomicTaskID: 0, TimeoutSeconds: e.TaskConf.ScriptExecution.TimeoutSec},
		},
		AtomicTasksRelations: []gse.AtomicTaskRelation{
			{AtomicTaskID: 0, AtomicTaskIDIdx: []int{}},
		},
	}

	resp, err := e.GseService.AsyncExtensionsExecuteScript(kt.Ctx, req)
	if err != nil {
		logs.Errorf("[BackupConfig STEP]: create execute script task failed, skip backup, batch_id=%d, err: %v",
			payload.BatchID, err)
		return nil
	}
	if resp == nil || resp.Result.TaskID == "" {
		logs.Errorf("[BackupConfig STEP]: gse execute script response is nil, skip backup, batch_id=%d",
			payload.BatchID)
		return nil
	}

	// 等待备份脚本执行结束并校验结果
	result, err := e.WaitExecuteScriptFinish(kt.Ctx, resp.Result.TaskID, commonPayload.ProcessPayload.AgentID)
	if err != nil {
		logs.Errorf("[BackupConfig STEP]: wait script execution failed, skip backup, batch_id=%d, err: %v",
			payload.BatchID, err)
		return nil
	}
	if len(result.Result) == 0 {
		logs.Errorf("[BackupConfig STEP]: script execution result is empty, skip backup, task_id=%s",
			resp.Result.TaskID)
		return nil
	}

	r := result.Result[0]
	if r.ErrorCode != 0 || r.ScriptExitCode != 0 {
		logs.Errorf(
			"[BackupConfig STEP]: backup script failed, skip backup, agent=%s, errorCode=%d, "+
				"scriptExitCode=%d, msg=%s, screen=%s",
			r.BkAgentID, r.ErrorCode, r.ScriptExitCode, r.ErrorMsg, r.Screen,
		)
		return nil
	}

	logs.Infof("[BackupConfig STEP]: backup success, batch_id: %d, task_id: %s", payload.BatchID, resp.Result.TaskID)
	return nil
}

// getServerInfo 获取本服务侧（源端）服务器 AgentID 和 ContainerID
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
// PushConfig 通过 GSE 传输文件到目标机器。
// 参考异步下载任务的处理方式：先将配置内容保存到本地临时目录，
// 再通过 GSE 文件传输下发到目标机器。
// 流程拆分：splitTargetPath 解析目标路径 -> writeSourceFileIfNeeded 准备本地源文件
// -> buildTransferFileReq 构建传输请求 -> executeTransferFile 执行传输并校验结果。
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

	fullPath, err := renderFullPath(kt.Ctx, commonPayload)
	if err != nil {
		logs.Errorf("[PushConfig STEP]: render full path failed: %v", err)
		return fmt.Errorf("render full path failed: %w", err)
	}

	renderedFilePath, renderedFileName, err := splitTargetPath(fullPath)
	if err != nil {
		return err
	}

	// 本机临时目录使用独立命名空间（{CacheDir}/config-push/{bizID}/{内容签名}），
	// 避免共享 CacheDir 时被 CacheCleaner 按 mtime 清理误删传输中的源文件；
	// 目录以 sha256 内容签名为文件夹名，文件以原文件名保存在其中。
	// GSE 文件传输为原样复制（落地名与源文件名一致），源文件用原名传输即可保证目标机器上的文件名正确
	srcDir := path.Join(e.GseConf.CacheDir, PushCacheNamespace,
		strconv.Itoa(int(payload.BizID)), cfg.ConfigContentSignature)
	srcPath := path.Join(srcDir, renderedFileName)

	// 文件锁避免并发写入；锁贯穿写入与传输全程，防止并发同内容任务在传输读取期间删除源文件
	e.fileLock.Acquire(srcPath)
	defer e.fileLock.Release(srcPath)

	// 目标文件权限由源（临时）文件决定：临时文件按配置权限写入，GSE 原样复制落地
	filePerm, err := resolveSourceFilePermission(proc.OsType, cfg)
	if err != nil {
		return err
	}

	if err = e.writeSourceFileIfNeeded(srcPath, cfg.ConfigContent, filePerm); err != nil {
		return err
	}

	// 获取源服务器信息
	srcAgentID, srcContainerID, err := getServerInfo()
	if err != nil {
		logs.Errorf("[PushConfig STEP]: get server info failed: %v", err)
		return fmt.Errorf("get server info failed: %w", err)
	}

	req := buildTransferFileReq(transferFileParams{
		SrcAgentID:     srcAgentID,
		SrcContainerID: srcContainerID,
		SrcDir:         srcDir,
		FileName:       renderedFileName,
		TargetDir:      renderedFilePath,
		Owner:          cfg.ConfigFileOwner,
		DestAgentID:    proc.AgentID,
	})

	if err := e.executeTransferFile(kt.Ctx, payload.BatchID, req, fullPath); err != nil {
		return err
	}

	// 传输完成后清理本地临时文件，任务即完成；清理失败不影响下发结果
	if err := os.Remove(srcPath); err != nil {
		logs.Warnf("[PushConfig STEP]: remove temp source file failed, path=%s, err=%v", srcPath, err)
	} else {
		logs.Infof("[PushConfig STEP]: temp source file removed: %s", srcPath)
	}

	// 清理空目录，避免 {内容签名}/{bizID} 层级只增不减（目录非空时 Remove 自然失败，静默忽略）
	cleanupEmptyDir(srcDir)
	cleanupEmptyDir(filepath.Dir(srcDir))

	return nil
}

// cleanupEmptyDir 尝试删除空目录，非空或删除失败静默忽略
func cleanupEmptyDir(dir string) {
	if err := os.Remove(dir); err != nil {
		return
	}
	logs.Infof("[PushConfig STEP]: empty cache dir removed: %s", dir)
}

// splitTargetPath 分离渲染后完整路径中的目录与文件名，并校验文件名合法性。
// 保留原始路径分隔符（Windows 为 `\`，Linux 为 `/`），
// 不能用 path.Dir/path.Base（只认 `/`，会把 Windows 路径切错），
// 确保下发到目标机器的路径和文件名与原配置一致
func splitTargetPath(fullPath string) (dir, name string, err error) {
	sepIdx := strings.LastIndexAny(fullPath, `/\`)
	if sepIdx < 0 {
		return "", "", fmt.Errorf("invalid rendered path: %s", fullPath)
	}
	dir, name = fullPath[:sepIdx], fullPath[sepIdx+1:]

	// 目标文件名必须是纯文件名，且拒绝空串与 "."、".." 相对路径分量：
	// 否则 srcPath 会逃逸出临时缓存目录（配合传输后的 Remove 可误删目录外文件），
	// 或空文件名进入 GSE 传输请求
	if name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return "", "", fmt.Errorf("invalid rendered file name: %q", name)
	}

	return dir, name, nil
}

// writeSourceFileIfNeeded 将配置内容写入本地临时缓存目录（调用方需持有 srcPath 的文件锁）。
// 文件按 perm 指定的权限写入（GSE 原样复制，源文件权限即目标落地权限）；
// 已存在且大小一致的文件直接复用（权限不一致时 chmod 对齐）；
// 否则先写临时文件再原子重命名，避免传输读取到写入一半的内容
func (e *PushConfigExecutor) writeSourceFileIfNeeded(srcPath, content string, perm os.FileMode) error {
	needWrite := true
	if info, statErr := os.Stat(srcPath); statErr == nil {
		// 已存在文件必须校验完整性：上次任务可能在写入中途失败留下半截文件，
		// 直接复用会把损坏内容下发到目标机器
		if info.Size() == int64(len(content)) {
			needWrite = false
			// 复用历史文件时对齐权限：可能是旧版本以其他权限写入的文件
			if info.Mode().Perm() != perm {
				if errC := os.Chmod(srcPath, perm); errC != nil {
					return fmt.Errorf("chmod source file failed: %w", errC)
				}
			}
			logs.Infof("[PushConfig STEP]: config file exists, skip writing: %s", srcPath)
		} else {
			logs.Warnf("[PushConfig STEP]: config file size mismatch, rewrite: %s, expect=%d, actual=%d",
				srcPath, len(content), info.Size())
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("stat source file failed: %w", statErr)
	}

	if !needWrite {
		return nil
	}

	// 目录保持 os.ModePerm，与异步下载缓存链路一致：
	// 源端 GSE Agent 以 AgentUser 身份读取本目录下的源文件做传输，
	// AgentUser 可能与 data-service 运行用户不同，目录需可穿越
	if errM := os.MkdirAll(path.Dir(srcPath), os.ModePerm); errM != nil {
		return fmt.Errorf("create source dir failed: %w", errM)
	}
	// 先写临时文件再原子重命名，避免传输读取到写入一半的内容
	tmpPath := srcPath + ".tmp"
	// 对齐异步下载的写盘方式：OpenFile + Sync，确保 GSE 传输前内容已持久化
	file, errO := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if errO != nil {
		return fmt.Errorf("open source temp file failed: %w", errO)
	}
	if _, errF := file.WriteString(content); errF != nil {
		file.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write source file failed: %w", errF)
	}
	if errS := file.Sync(); errS != nil {
		file.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync source file failed: %w", errS)
	}
	if errC := file.Close(); errC != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close source file failed: %w", errC)
	}
	if errR := os.Rename(tmpPath, srcPath); errR != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename source file failed: %w", errR)
	}
	logs.Infof("[PushConfig STEP]: config file saved to temp dir: %s, size: %d", srcPath, len(content))
	return nil
}

// resolveSourceFilePermission 计算本地临时源文件的写入权限。
// 模板侧 Privilege 为 "644" 形式的八进制字符串，需按八进制解析；
// GSE 文件传输为原样复制，源文件权限即目标落地权限。
// Windows 机器无权限概念，保持 os.ModePerm 交由默认处理；
// 机器类型以目标机器 os_type（processes 表）为准，os_type 为空时回退配置模板 FileMode
func resolveSourceFilePermission(osType string, cfg *common.ConfigPayload) (os.FileMode, error) {
	fileMode := FileModeFromOsType(osType, cfg.ConfigFileMode)
	if fileMode == table.Windows {
		return os.ModePerm, nil
	}

	perm, err := strconv.ParseUint(cfg.ConfigFilePermission, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("parse source file permission %q failed: %w", cfg.ConfigFilePermission, err)
	}
	return os.FileMode(perm), nil
}

// transferFileParams GSE 文件传输请求构建参数
type transferFileParams struct {
	SrcAgentID     string // 源端 agent ID
	SrcContainerID string // 源端容器 ID（容器部署时使用）
	SrcDir         string // 源端文件所在目录（本机临时缓存目录）
	FileName       string // 传输文件名（源与目标一致）
	TargetDir      string // 目标机器目录
	Owner          string // 目标文件属主
	DestAgentID    string // 目标机器 agent ID
}

// buildTransferFileReq 构建 GSE 文件传输请求
func buildTransferFileReq(p transferFileParams) *gse.TransferFileReq {
	return &gse.TransferFileReq{
		TimeOutSeconds: 3600,
		AutoMkdir:      true,
		Tasks: []gse.TransferFileTask{
			{
				Source: gse.TransferFileSource{
					FileName: p.FileName,
					StoreDir: p.SrcDir,
					Agent: gse.TransferFileAgent{
						BkAgentID:     p.SrcAgentID,
						BkContainerID: p.SrcContainerID,
						User:          cc.G().GSE.AgentUser,
					},
				},
				Target: gse.TransferFileTarget{
					// GSE 文件传输为原样复制（落地名与源文件名一致），
					// 源文件已用原文件名保存，目标名保持一致即可；
					// 不设置 Permission，落地权限跟随源（临时）文件
					Owner:    p.Owner,
					FileName: p.FileName,
					StoreDir: p.TargetDir,
					Agents: []gse.TransferFileAgent{
						{
							BkAgentID: p.DestAgentID,
							// 传输执行账号与脚本下发对齐：Linux 固定 root，Windows 固定 system。
							// 不能用配置属主（ConfigFileOwner）——属主可能是普通业务账号，
							// 无权写入目标目录或 chown；属主仅通过 Target.Owner 生效
						},
					},
				},
			},
		},
	}
}

// executeTransferFile 调用 GSE 传输文件并等待结束，校验所有目标的传输状态
func (e *PushConfigExecutor) executeTransferFile(ctx context.Context, batchID uint32,
	req *gse.TransferFileReq, target string) error {
	resp, err := e.GseService.AsyncExtensionsTransferFile(ctx, req)
	if err != nil {
		logs.Errorf("[PushConfig STEP]: create transfer task failed: %v", err)
		return fmt.Errorf("create transfer task failed: %w", err)
	}
	if resp == nil || resp.Result.TaskID == "" {
		logs.Errorf("[PushConfig STEP]: gse transfer file response is nil, batch_id=%d", batchID)
		return fmt.Errorf("gse transfer file response is nil, batch_id=%d", batchID)
	}

	logs.Infof("[PushConfig STEP]: gse task created, batch_id: %d, task_id: %s, target: %s",
		batchID, resp.Result.TaskID, target)

	// 等待传输完成
	result, err := e.WaitTransferFileTaskFinish(ctx, resp.Result.TaskID)
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

	logs.Infof("[PushConfig STEP]: transfer success, batch_id: %d, task_id: %s", batchID, resp.Result.TaskID)
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
func renderFullPath(ctx context.Context, commonPayload *common.TaskPayload) (string, error) {
	cfg := commonPayload.ConfigPayload
	proc := commonPayload.ProcessPayload

	// 机器类型以目标机器 os_type（processes 表）为准，os_type 为空时回退配置模板 FileMode
	fileMode := FileModeFromOsType(proc.OsType, cfg.ConfigFileMode)

	// 统一将反斜杠转为正斜杠，确保 POSIX path.Join 能正确处理
	filePath := strings.ReplaceAll(cfg.ConfigFilePath, `\`, "/")
	fileName := strings.ReplaceAll(cfg.ConfigFileName, `\`, "/")

	fullPath := path.Join(filePath, fileName)
	// 不包含变量则无需渲染
	if !strings.Contains(fullPath, "${") {
		if fileMode == table.Windows {
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

	renderedPath, err := render.Template(ctx, fullPath, contextParams)
	if err != nil {
		return "", fmt.Errorf("render full path failed: %w", err)
	}

	// Windows 机器转回反斜杠
	if fileMode == table.Windows {
		renderedPath = ToWindowsPath(renderedPath)
	}

	logs.Infof("render full path: %s -> %s", fullPath, renderedPath)
	return renderedPath, nil
}
