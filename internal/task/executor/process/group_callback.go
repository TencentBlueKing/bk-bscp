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

package process

import (
	"errors"
	"fmt"

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"

	"github.com/TencentBlueKing/bk-bscp/internal/processor/cmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// ProcessOperateGroupCallbackName 进程操作任务组的回调名称
const ProcessOperateGroupCallbackName istep.GroupCallbackName = "ProcessOperateGroupCallback"

// GroupPayload 进程操作任务组的负载，供任务组级回调收敛批次状态
type GroupPayload struct {
	TenantID    string   `json:"tenantID"`
	BizID       uint32   `json:"bizID"`
	BatchID     uint32   `json:"batchID"`
	OperateUser string   `json:"operateUser"`
	ProcessIDs  []uint32 `json:"processIDs"`
}

// GroupCallback 进程操作任务组回调。
//
// 阶段推进、级联阻断与计数由任务框架负责，这里只做两件业务收尾：
// 被阻断任务占用的实例中间态回滚，以及任务批次的终态收敛与通知。
type GroupCallback struct {
	*ProcessExecutor
}

// NewGroupCallback new process operate group callback
func NewGroupCallback(e *ProcessExecutor) *GroupCallback {
	return &GroupCallback{ProcessExecutor: e}
}

// OnStageBlocked 前序优先级失败导致后续任务被直接终结时触发。
//
// 这些任务一步都没执行过，也不会走各自的 Callback，对应实例会一直停在下发时写入的
// 中间态（如 starting），而中间态会让该实例后续所有操作都被判为非法，只能在这里收尾。
func (g *GroupCallback) OnStageBlocked(c *istep.GroupContext, blockedTaskIDs []string) error {
	payload := &GroupPayload{}
	if err := c.GetCommonPayload(payload); err != nil {
		return fmt.Errorf("get group payload failed: %w", err)
	}
	kt := kit.NewWithTenant(payload.TenantID)

	logs.Infof("[ProcessOperateGroupCallback]: rolling back %d blocked tasks, batchID: %d, groupID: %s",
		len(blockedTaskIDs), payload.BatchID, c.GetGroupID())

	var firstErr error
	for _, taskID := range blockedTaskIDs {
		t, err := c.GetTask(taskID)
		if err != nil {
			logs.Errorf("[ProcessOperateGroupCallback]: get blocked task %s failed: %v", taskID, err)
			firstErr = errors.Join(firstErr, err)
			continue
		}
		if err = RollbackPendingInstance(kt, g.Dao, t); err != nil {
			logs.Errorf("[ProcessOperateGroupCallback]: rollback blocked task %s failed: %v", taskID, err)
			firstErr = errors.Join(firstErr, err)
		}
	}
	return firstErr
}

// OnGroupComplete 任务组到达终态时收敛任务批次。
//
// 批次的最终计数直接取自任务组，任务框架在单个事务内维护这些计数，
// 因此不会与逐个任务累加出来的进度产生偏差。
func (g *GroupCallback) OnGroupComplete(c *istep.GroupContext) error {
	payload := &GroupPayload{}
	if err := c.GetCommonPayload(payload); err != nil {
		return fmt.Errorf("get group payload failed: %w", err)
	}
	kt := kit.NewWithTenant(payload.TenantID)

	group := c.GetGroup()
	// 被跳过的任务同样属于未成功，与失败一并计入
	failed := uint32(group.FailureCount + group.SkippedCount)
	// 幂等忽略（IGNORED）在任务组中独立计数、不计入 SuccessCount，
	// FinishBatch 会以这里的计数覆盖 task_batches，须把 IgnoredCount 视同成功并入，
	// 否则 completed_count = success + failed 会少于 total_count 且 success_count 少计 IGNORED 任务
	success := uint32(group.SuccessCount + group.IgnoredCount)

	if err := g.Dao.TaskBatch().FinishBatch(kt, payload.BatchID, success, failed); err != nil {
		logs.Errorf("[ProcessOperateGroupCallback]: finish task batch %d failed: %v", payload.BatchID, err)
		return fmt.Errorf("finish task batch %d failed: %w", payload.BatchID, err)
	}

	var cbErr error
	if failed > 0 {
		cbErr = fmt.Errorf("%d of %d process operate tasks failed", failed, group.TotalCount)
	}
	g.AfterCallbackNotify(kt.Ctx, common.CallbackNotify{
		TenantID: payload.TenantID,
		BizID:    payload.BizID,
		BatchID:  payload.BatchID,
		Operator: payload.OperateUser,
		CbErr:    cbErr,
	})

	// 批次结束后触发一次 CMDB 模块实例序列更新，确保模块实例序列的正确性
	for _, processID := range payload.ProcessIDs {
		if err := cmdb.ComputeModuleInstSeqUpdates(kt, g.Dao, payload.BizID, processID); err != nil {
			logs.Errorf("[ProcessOperateGroupCallback]: reorder module instance sequence failed, "+
				"bizID: %d, processID: %d, err: %v", payload.BizID, processID, err)
		}
	}
	return nil
}
