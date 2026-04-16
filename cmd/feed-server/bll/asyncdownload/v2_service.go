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
	"fmt"
	"time"

	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/uuid"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

type v2Service struct {
	store   *v2Store
	redLock *lock.RedisLock
	metric  *metric
	cfg     cc.AsyncDownloadV2
}

func newV2Service(bds bedis.Client, redLock *lock.RedisLock, mc *metric, cfg cc.AsyncDownloadV2) *v2Service {
	return &v2Service{
		store:   newV2Store(bds, cfg),
		redLock: redLock,
		metric:  mc,
		cfg:     cfg,
	}
}

func (s *v2Service) enabled() bool {
	return s != nil
}

func (s *v2Service) createTask(kt *kit.Kit, bizID, appID uint32, filePath, fileName,
	targetAgentID, targetContainerID, targetUser, targetDir, signature string) (string, error) {
	fileVersionKey := buildFileVersionKey(bizID, appID, filePath, fileName, signature)
	targetID := buildTargetID(targetAgentID, targetContainerID)
	inflightTargetKey := buildInflightTargetKey(targetID, targetUser, targetDir)

	if taskID, err := s.tryReuseInflightTask(kt.Ctx, fileVersionKey, inflightTargetKey); err == nil && taskID != "" {
		return taskID, nil
	}

	return s.createOrJoinBatch(kt, bizID, appID, filePath, fileName, signature, targetUser, targetDir,
		fileVersionKey, targetID, inflightTargetKey)
}

func (s *v2Service) tryReuseInflightTask(ctx context.Context, fileVersionKey, inflightTargetKey string) (string, error) {
	taskID, err := s.store.getInflightTaskID(ctx, fileVersionKey, inflightTargetKey)
	if err != nil || taskID == "" {
		return "", err
	}
	task, err := s.store.getTask(ctx, taskID)
	if err != nil {
		_ = s.store.clearInflightTaskID(ctx, fileVersionKey, inflightTargetKey)
		return "", nil
	}
	if isFinalTaskState(task.State) {
		_ = s.store.clearInflightTaskID(ctx, fileVersionKey, inflightTargetKey)
		return "", nil
	}
	return taskID, nil
}

func (s *v2Service) createOrJoinBatch(kt *kit.Kit, bizID, appID uint32, filePath, fileName, signature,
	targetUser, targetDir, fileVersionKey, targetID, inflightTargetKey string) (string, error) {
	lockKey := fmt.Sprintf("AsyncDownloadBatchCreateV2:%s", fileVersionKey)
	lockWaitStart := time.Now()
	s.redLock.Acquire(lockKey)
	defer s.redLock.Release(lockKey)
	logs.CtxInfof(kt.Ctx,
		"async download v2 batch create lock acquired, biz:%d, app:%d, file:%s, target_id:%s, target_user:%s, target_dir:%s, wait_ms:%d",
		bizID, appID, fmt.Sprintf("%s/%s", filePath, fileName), targetID, targetUser, targetDir,
		time.Since(lockWaitStart).Milliseconds())

	if taskID, err := s.tryReuseInflightTask(kt.Ctx, fileVersionKey, inflightTargetKey); err == nil && taskID != "" {
		return taskID, nil
	}

	now := time.Now()
	batchScopeKey := buildBatchScopeKey(fileVersionKey, targetUser, targetDir)
	openBatchID, err := s.store.getOpenBatchID(kt.Ctx, batchScopeKey)
	if err != nil {
		return "", err
	}
	if openBatchID != "" {
		batch, err := s.store.getBatch(kt.Ctx, openBatchID)
		if err == nil && batch.State == types.AsyncDownloadBatchStateCollecting && now.Before(batch.OpenUntil) {
			if taskID, err := s.store.getBatchTaskID(kt.Ctx, openBatchID, targetID); err == nil && taskID != "" {
				return taskID, nil
			}
			if s.cfg.MaxTargetsPerBatch <= 0 || batch.TargetCount < s.cfg.MaxTargetsPerBatch {
				task := newV2Task(kt.TenantID, bizID, appID, openBatchID, targetID, filePath, fileName, signature,
					targetUser, targetDir, now)
				oldOpenUntil := batch.OpenUntil
				batch.TargetCount++
				batch.OpenUntil = now.Add(time.Duration(s.cfg.CollectWindowSeconds) * time.Second)
				logs.CtxInfof(kt.Ctx,
					"async download v2 batch collect window extended, biz:%d, app:%d, batch_id:%s, file:%s/%s, target_id:%s, old_open_until:%s, new_open_until:%s, target_count:%d",
					bizID, appID, openBatchID, filePath, fileName, targetID,
					oldOpenUntil.Format(time.RFC3339Nano), batch.OpenUntil.Format(time.RFC3339Nano), batch.TargetCount)
				if err := s.store.saveBatch(kt.Ctx, batch); err != nil {
					return "", err
				}
				if err := s.store.addTaskToBatch(kt.Ctx, openBatchID, fileVersionKey, targetID, task.TaskID, task); err != nil {
					return "", err
				}
				s.metric.observeV2TaskCreated(task)
				if batch.TargetCount >= s.cfg.MaxTargetsPerBatch {
					_ = s.store.clearOpenBatchID(kt.Ctx, batchScopeKey)
				}
				return task.TaskID, nil
			}
			_ = s.store.clearOpenBatchID(kt.Ctx, batchScopeKey)
		}
	}

	batch := newV2Batch(kt.TenantID, bizID, appID, filePath, fileName, signature, targetUser, targetDir, now,
		time.Duration(s.cfg.CollectWindowSeconds)*time.Second)
	task := newV2Task(kt.TenantID, bizID, appID, batch.BatchID, targetID, filePath, fileName, signature,
		targetUser, targetDir, now)
	if err := s.store.createBatchAndTask(kt.Ctx, fileVersionKey, batch.BatchID, targetID, task.TaskID, batch, task); err != nil {
		return "", err
	}
	s.metric.observeV2BatchCreated(batch)
	s.metric.observeV2TaskCreated(task)
	return task.TaskID, nil
}

func (s *v2Service) getTaskStatus(ctx context.Context, taskID string) (string, error) {
	task, err := s.store.getTask(ctx, taskID)
	if err != nil {
		return "", err
	}
	return task.State, nil
}

func (s *v2Service) getAsyncDownloadTask(ctx context.Context, taskID string) (*types.AsyncDownloadTask, error) {
	task, err := s.store.getTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	agentID, containerID := parseTargetID(task.TargetID)
	return &types.AsyncDownloadTask{
		BizID:             task.BizID,
		AppID:             task.AppID,
		JobID:             task.BatchID,
		TargetAgentID:     agentID,
		TargetContainerID: containerID,
		FilePath:          task.FilePath,
		FileName:          task.FileName,
		FileSignature:     task.FileSignature,
		Status:            task.State,
		CreateTime:        task.CreatedAt,
	}, nil
}

func newV2Batch(tenantID string, bizID, appID uint32, filePath, fileName, signature, targetUser, targetDir string,
	now time.Time, collectWindow time.Duration) *types.AsyncDownloadV2Batch {
	return &types.AsyncDownloadV2Batch{
		BatchID:       fmt.Sprintf("AsyncDownloadBatchV2:%s", uuid.UUID()),
		TenantID:      tenantID,
		BizID:         bizID,
		AppID:         appID,
		FilePath:      filePath,
		FileName:      fileName,
		FileSignature: signature,
		TargetUser:    targetUser,
		TargetFileDir: targetDir,
		State:         types.AsyncDownloadBatchStateCollecting,
		OpenUntil:     now.Add(collectWindow),
		CreatedAt:     now,
		TargetCount:   1,
	}
}

func newV2Task(tenantID string, bizID, appID uint32, batchID, targetID, filePath, fileName, signature,
	targetUser, targetDir string, now time.Time) *types.AsyncDownloadV2Task {
	return &types.AsyncDownloadV2Task{
		TaskID:        fmt.Sprintf("AsyncDownloadTaskV2:%s", uuid.UUID()),
		BatchID:       batchID,
		TargetID:      targetID,
		BizID:         bizID,
		AppID:         appID,
		TenantID:      tenantID,
		FilePath:      filePath,
		FileName:      fileName,
		FileSignature: signature,
		TargetUser:    targetUser,
		TargetFileDir: targetDir,
		State:         types.AsyncDownloadJobStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func isFinalTaskState(state string) bool {
	switch state {
	case types.AsyncDownloadJobStatusSuccess, types.AsyncDownloadJobStatusFailed, types.AsyncDownloadJobStatusTimeout:
		return true
	default:
		return false
	}
}
