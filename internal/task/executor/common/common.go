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

	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// Executor common executor
type Executor struct {
	GseService *gse.Service
	Dao        dao.Set
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
	CloudID     int    // cloud ID
	CcProcessID string // CC 进程ID
	LocalInstID string // LocalInstID
	InstID      string // InstID
	ConfigData  string // 进程启动相关配置，比如启动脚本，优先级等
}

// NewExecutor new executor
func NewExecutor(gseService *gse.Service, dao dao.Set) *Executor {
	return &Executor{
		GseService: gseService,
		Dao:        dao,
	}
}

// WaitTaskFinish 等待任务执行结束
func (e *Executor) WaitTaskFinish(
	ctx context.Context,
	gseTaskID string,
	bizID, processInstanceID uint32,
	processName string,
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
			logs.Warnf("WaitTaskFinish get gse task state error, gseTaskID %s, err=%+v ", gseTaskID, err)
			return nil
		}

		err = gseResp.Decode(&result)
		if err != nil {
			return err
		}

		// key 为 bk_agent_id:namespace:name
		key := fmt.Sprintf("%s:GSEKIT_BIZ_%d:%s_%d", agentID, bizID, processName, processInstanceID)
		logs.Infof("get gse task result, key: %s", key)
		// 该状态表示gse侧进程操作任务正在执行中，尚未完成
		if result[key].ErrorCode == 115 {
			logs.Infof("WaitTaskFinish task %s is in progress, state=%d", gseTaskID, result[key].ErrorCode)
			return nil
		}

		if result[key].ErrorCode != 0 {
			logs.Errorf("WaitTaskFinish task %s failed, errorCode=%d, errorMsg=%s", gseTaskID, result[key].ErrorCode, result[key].ErrorMsg)
		} else {
			logs.Infof("WaitTaskFinish task %s success, errorCode=%d, errorMsg=%s", gseTaskID, result[key].ErrorCode, result[key].ErrorMsg)
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
