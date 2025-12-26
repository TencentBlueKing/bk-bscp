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

package common

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/common/task"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	maxWait  = 10 * time.Second
	interval = 2 * time.Second
)

// Executor common executor
type Executor struct {
	GseService  *gse.Service   // GSE 服务
	CMDBService bkcmdb.Service // CMDB 服务
	Dao         dao.Set
	GseConf     cc.GSE // GSE 配置
}

// TaskPayload 公用的配置，作为任务快照，方便进行获取以及对比
type TaskPayload struct {
	// 进程相关
	ProcessPayload *ProcessPayload
	// 配置相关
	ConfigPayload *ConfigPayload
}

// ProcessPayload 进程相关
type ProcessPayload struct {
	SetName       string // 集群名
	ModuleName    string // 模块名
	ServiceName   string // 服务实例
	Environment   string // 环境
	Alias         string // 进程别名
	FuncName      string // 进程二进制文件名
	InnerIP       string // IP
	AgentID       string // agnet ID
	CloudID       int    // cloud ID
	CcProcessID   uint32 // CC 进程ID
	HostInstSeq   uint32 // HostInstSeq：主机级别的自增ID
	ModuleInstSeq uint32 // ModuleInstSeq：模块级别的自增ID
	ConfigData    string // 进程启动相关配置，比如启动脚本，优先级等
}

// ConfigPayload 配置相关
type ConfigPayload struct {
	ConfigTemplateID        uint32
	ConfigTemplateVersionID uint32
	ConfigTemplateName      string
	ConfigFileName          string
	ConfigFilePath          string
	ConfigFileOwner         string
	ConfigFileGroup         string
	ConfigFilePermission    string
	ConfigInstanceKey       string // 配置实例标识: {configTemplateID}-{ccProcessID}-{moduleInstSeq}
	ConfigContent           string
	ConfigContentSignature  string        // 配置内容的签名(sha256)
	CompareStatus           CompareStatus // 对比状态
}

// CompareStatus 对比状态
type CompareStatus string

const (
	// CompareResultSame 一致
	CompareResultSame CompareStatus = "SAME" // 一致
	// CompareResultDifferent 不一致
	CompareResultDifferent CompareStatus = "DIFFERENT" // 不一致
	// CompareResultNeverPublished 从未下发
	CompareResultNeverPublished CompareStatus = "NEVER_PUBLISHED" // 从未下发
	// CompareResultUnknown 未知
	CompareResultUnknown CompareStatus = "UNKNOWN" // 未知
)

// NewExecutor new executor
func NewExecutor(gseService *gse.Service, dao dao.Set) *Executor {
	return &Executor{
		GseService: gseService,
		Dao:        dao,
		GseConf:    cc.G().GSE,
	}
}

// WaitTaskFinish 等待进程操作任务执行结束
func (e *Executor) WaitProcOperateTaskFinish(
	ctx context.Context,
	gseTaskID string,
	bizID, hostInstSeq uint32,
	alias string,
	agentID string,
) (map[string]gse.ProcResult, error) {
	var (
		result  map[string]gse.ProcResult
		err     error
		gseResp *gse.GESResponse
	)

	err = task.LoopDoFunc(ctx, func() error {
		// 获取gse侧进程操作结果
		gseResp, err = e.GseService.GetProcOperateResultV2(ctx, &gse.QueryProcResultReq{
			TaskID: gseTaskID,
		})
		if err != nil {
			logs.Warnf("WaitTaskFinish get gse task state error, gseTaskID %s, err=%+v", gseTaskID, err)
			return nil
		}
		if gseResp.Code != 0 {
			logs.Errorf("WaitTaskFinish get gse task result failed, gseTaskID %s, code=%d, message=%s",
				gseTaskID, gseResp.Code, gseResp.Message)
			return fmt.Errorf("get gse task result failed, code=%d, message=%s", gseResp.Code, gseResp.Message)
		}

		err = gseResp.Decode(&result)
		if err != nil {
			return err
		}

		// 构建 GSE 结果查询 key
		key := gse.BuildResultKey(agentID, bizID, alias, hostInstSeq)
		logs.Infof("get gse task result, key: %s", key)

		// 该状态表示gse侧进程操作任务正在执行中，尚未完成
		if gse.IsInProgress(result[key].ErrorCode) {
			logs.Infof("WaitTaskFinish task %s is in progress, errorCode=%d", gseTaskID, result[key].ErrorCode)
			return nil
		}

		// 结束任务
		return task.ErrEndLoop
	}, task.LoopInterval(2*time.Second))

	if err != nil {
		logs.Errorf("WaitTaskFinish error, gseTaskID %s, err=%+v", gseTaskID, err)
		return nil, err
	}
	return result, nil
}

// WaitTransferFileTaskFinish 等待文件传输任务执行结束
func (e *Executor) WaitTransferFileTaskFinish(
	ctx context.Context,
	gseTaskID string,
) (*gse.TransferFileResultData, error) {
	var (
		result *gse.TransferFileResultData
		err    error
	)

	err = task.LoopDoFunc(ctx, func() error {
		// 获取gse侧文件传输任务结果
		result, err = e.GseService.GetExtensionsTransferFileResult(ctx, &gse.GetTransferFileResultReq{
			TaskID: gseTaskID,
		})
		if err != nil {
			logs.Warnf("WaitTransferFileTaskFinish get gse task state error, gseTaskID %s, err=%+v", gseTaskID, err)
			return nil
		}

		// 检查所有目标的传输状态
		allFinished := true
		for _, r := range result.Result {
			// ErrorCode 115 表示任务正在执行中
			if gse.IsInProgress(r.ErrorCode) {
				allFinished = false
				logs.Infof("WaitTransferFileTaskFinish task %s is in progress, agentID: %s, errorCode=%d",
					gseTaskID, r.Content.DestAgentID, r.ErrorCode)
				break
			}
		}

		// 如果所有任务都已完成（成功或失败），结束循环
		if allFinished {
			logs.Infof("WaitTransferFileTaskFinish task %s finished", gseTaskID)
			return task.ErrEndLoop
		}

		// 继续等待
		return nil
	}, task.LoopInterval(2*time.Second))

	if err != nil {
		logs.Errorf("WaitTransferFileTaskFinish error, gseTaskID %s, err=%+v", gseTaskID, err)
		return nil, err
	}
	return result, nil
}

// WaitExecuteScriptFinish 等待脚本执行任务完成
func (e *Executor) WaitExecuteScriptFinish(ctx context.Context, gseTaskID, bkAgentID string) (*gse.ExecuteScriptResult, error) {
	var result *gse.ExecuteScriptResult

	err := wait.PollUntilContextTimeout(
		ctx,
		interval,
		maxWait,
		true,
		func(ctx context.Context) (bool, error) {
			resp, err := e.GseService.GetExecuteScriptResult(ctx, &gse.GetExecuteScriptResultReq{
				TaskID: gseTaskID,
				AgentTasks: []gse.AgentTaskQuery{
					{
						BkAgentID: bkAgentID,
						AtomicTasks: []gse.AtomicTaskQuery{
							{Offset: 0},
						},
					},
				},
			})
			if err != nil {
				return false, fmt.Errorf("get execute script result failed, taskID=%s, err=%v", gseTaskID, err)
			}

			result = resp

			// 是否仍存在执行中的任务
			for _, r := range result.Result {
				// 正在执行：status=1
				if r.ErrorCode == 0 && r.Status == 1 {
					logs.Infof("script executing, task=%s, agentID=%s, containerID=%s", gseTaskID, r.BkAgentID, r.BkContainerID)
					return false, nil
				}

				// 兼容 GSE 其他 executing 状态
				if gse.IsInProgress(r.ErrorCode) {
					logs.Infof(
						"script in progress, task=%s, agentID=%s, containerID=%s, errorCode=%d",
						gseTaskID, r.BkAgentID, r.BkContainerID, r.ErrorCode,
					)
					return false, nil
				}
			}

			// 全部非执行中
			return true, nil
		},
	)

	if err != nil {
		if wait.Interrupted(err) || errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("wait execute script timeout, taskID=%s, maxWait=%s", gseTaskID, maxWait)
		}
		return nil, err
	}

	return result, nil
}

func BuildConfigTaskPayload(
	process *table.Process,
	processInstance *table.ProcessInstance,
	templateRevision *table.TemplateRevision,
	configTemplateID uint32,
	configTemplateName string,
) *TaskPayload {

	key := fmt.Sprintf(
		"%d-%d-%d",
		configTemplateID,
		process.Attachment.CcProcessID,
		processInstance.Spec.ModuleInstSeq,
	)

	return &TaskPayload{
		ProcessPayload: &ProcessPayload{
			SetName:       process.Spec.SetName,
			ModuleName:    process.Spec.ModuleName,
			ServiceName:   process.Spec.ServiceName,
			Environment:   process.Spec.Environment,
			Alias:         process.Spec.Alias,
			FuncName:      process.Spec.FuncName,
			InnerIP:       process.Spec.InnerIP,
			AgentID:       process.Attachment.AgentID,
			CcProcessID:   process.Attachment.CcProcessID,
			HostInstSeq:   processInstance.Spec.HostInstSeq,
			ModuleInstSeq: processInstance.Spec.ModuleInstSeq,
			ConfigData:    process.Spec.SourceData,
			CloudID:       int(process.Attachment.CloudID),
		},
		ConfigPayload: &ConfigPayload{
			ConfigTemplateID:        configTemplateID,
			ConfigTemplateVersionID: templateRevision.ID,
			ConfigTemplateName:      configTemplateName,
			ConfigFileName:          templateRevision.Spec.Name,
			ConfigFilePath:          templateRevision.Spec.Path,
			ConfigFileOwner:         templateRevision.Spec.Permission.User,
			ConfigFileGroup:         templateRevision.Spec.Permission.UserGroup,
			ConfigFilePermission:    templateRevision.Spec.Permission.Privilege,
			ConfigInstanceKey:       key,
			ConfigContent:           "",
		},
	}
}
