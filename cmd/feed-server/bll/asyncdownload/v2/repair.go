// * Tencent is pleased to support the open source community by making Blueking Container Service available.
//  * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
//  * Licensed under the MIT License (the "License"); you may not use this file except
//  * in compliance with the License. You may obtain a copy of the License at
//  * http://opensource.org/licenses/MIT
//  * Unless required by applicable law or agreed to in writing, software distributed under
//  * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  * either express or implied. See the License for the specific language governing permissions and
//  * limitations under the License.

package v2

import (
	"context"
	"time"

	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	batchFinalReasonFailed          = "batch_failed"
	batchFinalReasonDispatchTimeout = "dispatch_timeout"
	batchFinalReasonDispatchCutoff  = "orphan_after_dispatch_cutoff"
)

func (s *Scheduler) finalizeCompletedBatch(ctx context.Context, batch *types.AsyncDownloadV2Batch) error {
	taskIDs, err := s.store.ListBatchTasks(ctx, batch.BatchID)
	if err != nil {
		return err
	}
	for _, taskID := range taskIDs {
		task, err := s.store.GetTask(ctx, taskID)
		if err != nil {
			return err
		}
		if !isFinalTaskState(task.State) {
			continue
		}
		if err := s.store.ClearInflightTaskID(ctx, BuildFileVersionKey(task.BizID, task.AppID, task.FilePath, task.FileName,
			task.FileSignature), BuildInflightTargetKey(task.TargetID, task.TargetUser, task.TargetFileDir)); err != nil {
			return err
		}
	}
	_ = s.store.RemoveDueBatchID(ctx, batch.BatchID)
	_ = s.store.ClearOpenBatchID(ctx, BuildBatchScopeKey(
		BuildFileVersionKey(batch.BizID, batch.AppID, batch.FilePath, batch.FileName, batch.FileSignature),
		batch.TargetUser, batch.TargetFileDir))
	return nil
}

func (s *Scheduler) RepairTerminalBatch(ctx context.Context, batchID, batchState string) error {
	batch, err := s.store.GetBatch(ctx, batchID)
	if err != nil {
		return err
	}
	oldState := batch.State
	batch.State = batchState
	if batchState == types.AsyncDownloadBatchStateFailed {
		batch.FinalReason = batchFinalReasonFailed
	} else {
		batch.FinalReason = batchFinalReasonDispatchCutoff
	}
	if finalizeErr := s.FinalizeBatchTasks(ctx, batchID, batchState); finalizeErr != nil {
		return finalizeErr
	}
	successCount, failedCount, timeoutCount, _, _, err := s.countBatchTaskStates(ctx, batchID)
	if err != nil {
		return err
	}
	batch.SuccessCount = successCount
	batch.FailedCount = failedCount
	batch.TimeoutCount = timeoutCount
	if err := s.store.SaveBatch(ctx, batch); err != nil {
		return err
	}
	s.metric.ObserveV2BatchTransition(batch, oldState)
	return s.finalizeCompletedBatch(ctx, batch)
}

func (s *Scheduler) RepairTimeoutBatch(ctx context.Context, batchID string) error {
	batch, err := s.store.GetBatch(ctx, batchID)
	if err != nil {
		return err
	}
	oldState := batch.State
	s.terminateBatchDispatches(batch)
	if finalizeErr := s.FinalizeTimeoutBatchTasks(ctx, batchID); finalizeErr != nil {
		return finalizeErr
	}
	successCount, failedCount, timeoutCount, _, _, err := s.countBatchTaskStates(ctx, batchID)
	if err != nil {
		return err
	}
	batch.State = deriveTerminalBatchState(successCount, failedCount, timeoutCount)
	batch.FinalReason = batchFinalReasonDispatchTimeout
	batch.SuccessCount = successCount
	batch.FailedCount = failedCount
	batch.TimeoutCount = timeoutCount
	if err := s.store.SaveBatch(ctx, batch); err != nil {
		return err
	}
	s.metric.ObserveV2BatchTransition(batch, oldState)
	return s.finalizeCompletedBatch(ctx, batch)
}

func (s *Scheduler) FinalizeBatchTasks(ctx context.Context, batchID, batchState string) error {
	switch batchState {
	case types.AsyncDownloadBatchStateFailed:
		return s.finalizeBatchTasks(ctx, batchID, types.AsyncDownloadJobStatusFailed, batchFinalReasonFailed)
	default:
		return s.finalizeBatchTasks(ctx, batchID, types.AsyncDownloadJobStatusFailed, batchFinalReasonDispatchCutoff)
	}
}

func (s *Scheduler) FinalizeTimeoutBatchTasks(ctx context.Context, batchID string) error {
	return s.finalizeBatchTasks(ctx, batchID, types.AsyncDownloadJobStatusTimeout, batchFinalReasonDispatchTimeout)
}

func (s *Scheduler) finalizeBatchTasks(ctx context.Context, batchID, taskState, errMsg string) error {
	taskIDs, err := s.store.ListBatchTasks(ctx, batchID)
	if err != nil {
		return err
	}
	for _, taskID := range taskIDs {
		task, err := s.store.GetTask(ctx, taskID)
		if err != nil || isFinalTaskState(task.State) {
			continue
		}
		oldState := task.State
		oldUpdatedAt := task.UpdatedAt
		task.State = taskState
		task.ErrMsg = errMsg
		if s.metric != nil {
			s.metric.IncV2TaskRepair(task.ErrMsg)
		}
		task.UpdatedAt = time.Now()
		if err := s.store.SaveTask(ctx, task); err != nil {
			return err
		}
		s.metric.ObserveV2TaskTransition(task, oldState, oldUpdatedAt)
	}
	return nil
}

func (s *Scheduler) terminateBatchDispatches(batch *types.AsyncDownloadV2Batch) {
	if s.gseService == nil || batch == nil {
		return
	}
	kt := kit.NewWithTenant(batch.TenantID)
	dispatchState, err := s.store.ListBatchDispatchState(kt.Ctx, batch.BatchID)
	if err != nil {
		logs.Errorf("list batch dispatch state for timeout batch %s failed, err: %v", batch.BatchID, err)
		return
	}
	targetTasks, err := s.store.ListBatchTargetTasks(kt.Ctx, batch.BatchID)
	if err != nil {
		logs.Errorf("list batch target tasks for timeout batch %s failed, err: %v", batch.BatchID, err)
		return
	}
	groupedAgents := make(map[string][]gse.TransferFileAgent)
	for targetID, gseTaskID := range dispatchState {
		if gseTaskID == "" || gseTaskID == "local" {
			continue
		}
		taskID := targetTasks[targetID]
		if taskID == "" {
			continue
		}
		task, err := s.store.GetTask(kt.Ctx, taskID)
		if err != nil {
			logs.Errorf("get task %s for timeout batch %s failed, err: %v", taskID, batch.BatchID, err)
			continue
		}
		if isFinalTaskState(task.State) {
			continue
		}
		agentID, containerID := ParseTargetID(targetID)
		groupedAgents[gseTaskID] = append(groupedAgents[gseTaskID], gse.TransferFileAgent{
			User:          task.TargetUser,
			BkAgentID:     agentID,
			BkContainerID: containerID,
		})
	}
	for gseTaskID, agents := range groupedAgents {
		if len(agents) == 0 {
			continue
		}
		if _, err := s.gseService.AsyncTerminateTransferFile(kt.Ctx, &gse.TerminateTransferFileTaskReq{
			TaskID: gseTaskID,
			Agents: agents,
		}); err != nil {
			logs.Errorf("terminate timeout transfer file task %s for batch %s failed, err: %v",
				gseTaskID, batch.BatchID, err)
		}
	}
}

func (s *Scheduler) countBatchTaskStates(ctx context.Context, batchID string) (int, int, int, int, int, error) {
	taskIDs, err := s.store.ListBatchTasks(ctx, batchID)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	var successCount, failedCount, timeoutCount, runningCount, pendingCount int
	for _, taskID := range taskIDs {
		task, err := s.store.GetTask(ctx, taskID)
		if err != nil {
			return 0, 0, 0, 0, 0, err
		}
		switch task.State {
		case types.AsyncDownloadJobStatusSuccess:
			successCount++
		case types.AsyncDownloadJobStatusFailed:
			failedCount++
		case types.AsyncDownloadJobStatusTimeout:
			timeoutCount++
		case types.AsyncDownloadJobStatusRunning:
			runningCount++
		default:
			pendingCount++
		}
	}
	return successCount, failedCount, timeoutCount, runningCount, pendingCount, nil
}

func deriveTerminalBatchState(successCount, failedCount, timeoutCount int) string {
	switch {
	case failedCount == 0 && timeoutCount == 0:
		return types.AsyncDownloadBatchStateDone
	case successCount > 0:
		return types.AsyncDownloadBatchStatePartial
	default:
		return types.AsyncDownloadBatchStateFailed
	}
}

func (s *Scheduler) updateTaskStateByTarget(ctx context.Context, batchID, targetID, state, errMsg string) (bool, error) {
	taskID, err := s.store.GetBatchTaskID(ctx, batchID, targetID)
	if err != nil {
		return false, err
	}
	if taskID == "" {
		return false, nil
	}
	task, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return false, err
	}
	oldState := task.State
	oldUpdatedAt := task.UpdatedAt
	if task.State == state && task.ErrMsg == errMsg {
		return false, nil
	}
	task.State = state
	task.ErrMsg = errMsg
	task.UpdatedAt = time.Now()
	if err := s.store.SaveTask(ctx, task); err != nil {
		return false, err
	}
	s.metric.ObserveV2TaskTransition(task, oldState, oldUpdatedAt)
	return oldState != state, nil
}
