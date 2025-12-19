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
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bcs"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/repository"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
)

const (
	// validate push config step name
	ValidatePushConfigStepName istep.StepName = "ValidatePushConfig"
	// download config step name
	DownloadConfigStepName istep.StepName = "DownloadConfig"
	// PushConfigStepName push config step name
	PushConfigStepName istep.StepName = "PushConfig"
	// CallbackName push config callback name
	CallbackName istep.CallbackName = "Callback"
)

// PushConfigExecutor 配置下发执行器
type PushConfigExecutor struct {
	*common.Executor
	GseService *gse.Service        // GSE 服务
	Repo       repository.Provider // 仓库服务
	fileLock   *lock.FileLock      // 文件锁
}

// NewPushConfigExecutor new push config executor
func NewPushConfigExecutor(dao dao.Set, gseService *gse.Service, repo repository.Provider) *PushConfigExecutor {
	return &PushConfigExecutor{
		Executor: &common.Executor{
			Dao:        dao,
			GseService: gseService,
		},
		GseService: gseService,
		Repo:       repo,
		fileLock:   lock.NewFileLock(),
	}
}

// PushConfigPayload 配置下发 payload
type PushConfigPayload struct {
	BizID               uint32
	BatchID             uint32
	OperateType         table.ConfigOperateType
	OperatorUser        string
	GenerateTaskID      string              // 关联的配置生成任务 ID
	GenerateTaskPayload *common.TaskPayload // 配置生成任务 payload
}

// ValidatePushConfig implements istep.Step.
func (e *PushConfigExecutor) ValidatePushConfig(c *istep.Context) error {
	payload := &PushConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		return err
	}

	return nil
}

// DownloadConfig implements istep.Step.
// DownloadConfig 下载配置文件到本地
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

// PushConfig implements istep.Step.
// PushConfig 通过 GSE 传输文件到目标机器
func (e *PushConfigExecutor) PushConfig(c *istep.Context) error {
	payload := &PushConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		return err
	}

	cfg := payload.GenerateTaskPayload.ConfigPayload
	proc := payload.GenerateTaskPayload.ProcessPayload

	logs.Infof("[PushConfig STEP]: push config for batch %d, biz_id: %d, config_key: %s",
		payload.BatchID, payload.BizID, cfg.ConfigInstanceKey)

	kt := kit.New()
	kt.BizID = payload.BizID

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
					FileName: cfg.ConfigFileName,
					StoreDir: cfg.ConfigFilePath,
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

	logs.Infof("[PushConfig STEP]: gse task created, batch_id: %d, task_id: %s, target: %s/%s",
		payload.BatchID, resp.Result.TaskID, cfg.ConfigFilePath, cfg.ConfigFileName)

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

// Callback implements istep.Callback.
func (e *PushConfigExecutor) Callback(c *istep.Context, cbErr error) error {
	payload := &PushConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("get payload failed: %w", err)
	}

	kt := kit.New()
	kt.BizID = payload.BizID
	kt.User = payload.OperatorUser

	isSuccess := cbErr == nil
	// 更新批次状态
	if err := e.Dao.TaskBatch().IncrementCompletedCount(kt, payload.BatchID, isSuccess); err != nil {
		return fmt.Errorf("increment completed count failed, batch: %d, err: %w", payload.BatchID, err)
	}

	// 仅配置下发成功才更新配置实例的状态
	if !isSuccess {
		return nil
	}

	cfg := payload.GenerateTaskPayload.ConfigPayload
	proc := payload.GenerateTaskPayload.ProcessPayload
	now := time.Now()

	instance := &table.ConfigInstance{
		Attachment: &table.ConfigInstanceAttachment{
			BizID:            payload.BizID,
			ConfigTemplateID: cfg.ConfigTemplateID,
			ConfigVersionID:  cfg.ConfigTemplateVersionID,
			CcProcessID:      proc.CcProcessID,
			ModuleInstSeq:    proc.ModuleInstSeq,
			GenerateTaskID:   payload.GenerateTaskID,
			TenantID:         "",
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

// getServerInfo 获取服务器 AgentID 和 ContainerID
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

// RegisterPushConfigExecutor 注册执行器
func RegisterPushConfigExecutor(e *PushConfigExecutor) {
	istep.Register(ValidatePushConfigStepName, istep.StepExecutorFunc(e.ValidatePushConfig))
	istep.Register(DownloadConfigStepName, istep.StepExecutorFunc(e.DownloadConfig))
	istep.Register(PushConfigStepName, istep.StepExecutorFunc(e.PushConfig))
	istep.RegisterCallback(CallbackName, istep.CallbackExecutorFunc(e.Callback))
}
