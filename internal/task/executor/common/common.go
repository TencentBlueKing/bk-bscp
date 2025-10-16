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
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/common/task"

	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// Executor common executor
type Executor struct {
	gseService *gse.Service
}

// ProcessPayload 公用的配置，作为任务快照，方便进行获取以及对比
type ProcessPayload struct {
	SetName     string // 集群名
	ModuleName  string // 模块名
	ServiceName string // 服务实例
	Environment string // 环境
	Alias       string // 进程别名
	InnerIP     string // IP
	AgentID     string // agnet ID
	CcProcessID string // CC 进程ID
	LocalInstID string // LocalInstID
	InstID      string // InstID
	ConfigData  string // 进程启动相关配置，比如启动脚本，优先级等
}

// NewExecutor new executor
func NewExecutor(gseService *gse.Service) *Executor {
	return &Executor{gseService}
}

// WaitTaskFinish 等待任务执行结束
func (e *Executor) WaitTaskFinish(
	ctx context.Context,
	gseTaskID string,
	agentIDs []string) (*gse.TaskOperateResult, error) {
	var (
		taskResult *gse.TaskOperateResult
		err        error
	)

	err = task.LoopDoFunc(ctx, func() error {
		req := &gse.TaskReq{
			TaskID:      gseTaskID,
			AgentIDList: agentIDs,
		}
		taskResult, err = e.gseService.GetTaskState(ctx, req)
		if err != nil {
			logs.Warnf("WaitTaskFinish get gse task state error, gseTaskID %s, err=%+v ", gseTaskID, err)
			return nil
		}

		// 任务处理进行中需要继续
		if taskResult.Result.State == gse.ExecutingState || taskResult.Result.State == gse.PendingState {
			logs.Warnf("WaitTaskFinish task %s is in progress, state=%s", gseTaskID, taskResult.Result.State)
			return nil
		}

		// 结束任务
		return task.ErrEndLoop
	}, task.LoopInterval(2*time.Second))
	if err != nil {
		logs.Errorf("WaitTaskFinish error, gseTaskID %s, err=%+v", gseTaskID, err)
		return nil, err
	}
	return taskResult, nil
}
