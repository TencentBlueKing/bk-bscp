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
	"fmt"
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/common/task"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// Executor common executor
type Executor struct {
	GseService  *gse.Service
	CMDBService bkcmdb.Service
	Dao         dao.Set
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
	ConfigContentSignature  string // 配置内容的签名(sha256)
}

// NewExecutor new executor
func NewExecutor(gseService *gse.Service, dao dao.Set) *Executor {
	return &Executor{
		GseService: gseService,
		Dao:        dao,
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
	agents []gse.AgentList,
) (*gse.TransferFileResultData, error) {
	var (
		result *gse.TransferFileResultData
		err    error
	)

	err = task.LoopDoFunc(ctx, func() error {
		// 获取gse侧文件传输任务结果
		result, err = e.GseService.GetExtensionsTransferFileResult(ctx, &gse.GetTransferFileResultReq{
			TaskID: gseTaskID,
			Agents: agents,
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
