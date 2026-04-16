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
		batch.FinalReason = "batch_failed"
	} else {
		batch.FinalReason = "orphan_after_dispatch_cutoff"
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

func (s *Scheduler) FinalizeBatchTasks(ctx context.Context, batchID, batchState string) error {
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
		switch batchState {
		case types.AsyncDownloadBatchStateFailed:
			task.State = types.AsyncDownloadJobStatusFailed
			task.ErrMsg = "batch_failed"
		default:
			task.State = types.AsyncDownloadJobStatusFailed
			task.ErrMsg = "orphan_after_dispatch_cutoff"
		}
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

func (s *Scheduler) updateTaskStateByTarget(ctx context.Context, batchID, targetID, state, errMsg string) error {
	taskID, err := s.store.GetBatchTaskID(ctx, batchID, targetID)
	if err != nil {
		return err
	}
	if taskID == "" {
		return nil
	}
	task, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	oldState := task.State
	oldUpdatedAt := task.UpdatedAt
	task.State = state
	task.ErrMsg = errMsg
	task.UpdatedAt = time.Now()
	if err := s.store.SaveTask(ctx, task); err != nil {
		return err
	}
	s.metric.ObserveV2TaskTransition(task, oldState, oldUpdatedAt)
	return nil
}
