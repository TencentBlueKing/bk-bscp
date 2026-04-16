// * Tencent is pleased to support the open source community by making Blueking Container Service available.
//  * Copyright (C) 20\d\d THL A29 Limited, a Tencent company. All rights reserved.
//  * Licensed under the MIT License (the "License"); you may not use this file except
//  * in compliance with the License. You may obtain a copy of the License at
//  * http://opensource.org/licenses/MIT
//  * Unless required by applicable law or agreed to in writing, software distributed under
//  * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  * either express or implied. See the License for the specific language governing permissions and
//  * limitations under the License.

package asyncdownload

import (
	"context"
	"time"

	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
)

func (s *v2Scheduler) finalizeCompletedBatch(ctx context.Context, batch *types.AsyncDownloadV2Batch) error {
	taskIDs, err := s.store.listBatchTasks(ctx, batch.BatchID)
	if err != nil {
		return err
	}
	for _, taskID := range taskIDs {
		task, err := s.store.getTask(ctx, taskID)
		if err != nil {
			return err
		}
		if !isFinalTaskState(task.State) {
			continue
		}
		if err := s.store.clearInflightTaskID(ctx, buildFileVersionKey(task.BizID, task.AppID, task.FilePath, task.FileName,
			task.FileSignature), buildInflightTargetKey(task.TargetID, task.TargetUser, task.TargetFileDir)); err != nil {
			return err
		}
	}
	_ = s.store.removeDueBatchID(ctx, batch.BatchID)
	_ = s.store.clearOpenBatchID(ctx, buildBatchScopeKey(
		buildFileVersionKey(batch.BizID, batch.AppID, batch.FilePath, batch.FileName, batch.FileSignature),
		batch.TargetUser, batch.TargetFileDir))
	return nil
}

func (s *v2Scheduler) repairTerminalBatch(ctx context.Context, batchID, batchState string) error {
	batch, err := s.store.getBatch(ctx, batchID)
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
	if err := s.finalizeBatchTasks(ctx, batchID, batchState); err != nil {
		return err
	}
	successCount, failedCount, timeoutCount, _, _, err := s.countBatchTaskStates(ctx, batchID)
	if err != nil {
		return err
	}
	batch.SuccessCount = successCount
	batch.FailedCount = failedCount
	batch.TimeoutCount = timeoutCount
	if err := s.store.saveBatch(ctx, batch); err != nil {
		return err
	}
	s.metric.observeV2BatchTransition(batch, oldState)
	return s.finalizeCompletedBatch(ctx, batch)
}

func (s *v2Scheduler) finalizeBatchTasks(ctx context.Context, batchID, batchState string) error {
	taskIDs, err := s.store.listBatchTasks(ctx, batchID)
	if err != nil {
		return err
	}
	for _, taskID := range taskIDs {
		task, err := s.store.getTask(ctx, taskID)
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
		if s.metric != nil && s.metric.taskRepairCounter != nil {
			s.metric.taskRepairCounter.WithLabelValues(task.ErrMsg).Inc()
		}
		task.UpdatedAt = time.Now()
		if err := s.store.saveTask(ctx, task); err != nil {
			return err
		}
		s.metric.observeV2TaskTransition(task, oldState, oldUpdatedAt)
	}
	return nil
}

func (s *v2Scheduler) countBatchTaskStates(ctx context.Context, batchID string) (int, int, int, int, int, error) {
	taskIDs, err := s.store.listBatchTasks(ctx, batchID)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	var successCount, failedCount, timeoutCount, runningCount, pendingCount int
	for _, taskID := range taskIDs {
		task, err := s.store.getTask(ctx, taskID)
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

func (s *v2Scheduler) updateTaskStateByTarget(ctx context.Context, batchID, targetID, state, errMsg string) error {
	taskID, err := s.store.getBatchTaskID(ctx, batchID, targetID)
	if err != nil {
		return err
	}
	if taskID == "" {
		return nil
	}
	task, err := s.store.getTask(ctx, taskID)
	if err != nil {
		return err
	}
	oldState := task.State
	oldUpdatedAt := task.UpdatedAt
	task.State = state
	task.ErrMsg = errMsg
	task.UpdatedAt = time.Now()
	if err := s.store.saveTask(ctx, task); err != nil {
		return err
	}
	s.metric.observeV2TaskTransition(task, oldState, oldUpdatedAt)
	return nil
}
