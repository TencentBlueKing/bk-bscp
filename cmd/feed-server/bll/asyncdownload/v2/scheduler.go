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
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"time"

	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

type Scheduler struct {
	store             *Store
	gseService        TransferFileClient
	provider          SourceDownloader
	redLock           *lock.RedisLock
	fileLock          *lock.FileLock
	metric            Metrics
	instance          string
	serverAgentID     string
	serverContainerID string
	agentUser         string
	cacheDir          string
	cfg               cc.AsyncDownloadV2
}

func NewScheduler(store *Store, gseService TransferFileClient, provider SourceDownloader, redLock *lock.RedisLock,
	fileLock *lock.FileLock, mc Metrics, serverAgentID, serverContainerID, agentUser, cacheDir string,
	cfg cc.AsyncDownloadV2) *Scheduler {
	return &Scheduler{
		store:             store,
		gseService:        gseService,
		provider:          provider,
		redLock:           redLock,
		fileLock:          fileLock,
		metric:            mc,
		instance:          BuildTargetID(serverAgentID, serverContainerID),
		serverAgentID:     serverAgentID,
		serverContainerID: serverContainerID,
		agentUser:         agentUser,
		cacheDir:          cacheDir,
		cfg:               cfg,
	}
}

func (s *Scheduler) Enabled() bool {
	return s != nil
}

func (s *Scheduler) ProcessDueBatches(ctx context.Context) (int, error) {
	batchIDs, err := s.store.ListDueBatchIDs(ctx, time.Now(), s.cfg.MaxDueBatchesPerTick)
	if err != nil {
		return 0, err
	}
	if s.metric != nil {
		s.metric.SetV2DueBacklog(len(batchIDs))
		if len(batchIDs) == 0 {
			s.metric.SetV2OldestDueAgeSeconds(0)
		} else if batch, batchErr := s.store.GetBatch(ctx, batchIDs[0]); batchErr == nil {
			s.metric.SetV2OldestDueAgeSeconds(time.Since(batch.OpenUntil).Seconds())
		}
	}
	for _, batchID := range batchIDs {
		if err := s.ProcessBatch(ctx, batchID); err != nil {
			logs.Errorf("process v2 batch %s failed, err: %v", batchID, err)
		}
	}
	if err := s.refreshDispatchingBatches(ctx); err != nil {
		return len(batchIDs), err
	}
	return len(batchIDs), nil
}

func splitTargets(targets []string, shardSize int) [][]string {
	if shardSize <= 0 {
		shardSize = len(targets)
	}
	var shards [][]string
	for len(targets) > 0 {
		n := shardSize
		if len(targets) < n {
			n = len(targets)
		}
		shards = append(shards, append([]string(nil), targets[:n]...))
		targets = targets[n:]
	}
	return shards
}

func (s *Scheduler) ProcessBatch(ctx context.Context, batchID string) error {
	lockKey := fmt.Sprintf("AsyncDownloadBatchDispatchV2:%s", batchID)
	if !s.redLock.TryAcquire(lockKey) {
		return nil
	}
	defer s.redLock.Release(lockKey)

	batch, err := s.store.GetBatch(ctx, batchID)
	if err != nil {
		return err
	}
	if batch.State != types.AsyncDownloadBatchStateCollecting {
		return nil
	}

	now := time.Now()
	oldState := batch.State
	batch.State = types.AsyncDownloadBatchStateDispatching
	batch.DispatchStartedAt = now
	batch.DispatchOwner = s.instance
	batch.DispatchHeartbeatAt = now
	batch.DispatchLeaseUntil = now.Add(time.Duration(s.cfg.DispatchLeaseSeconds) * time.Second)
	batch.DispatchAttempt++
	batch.OpenUntil = time.Time{}
	if saveErr := s.store.SaveBatch(ctx, batch); saveErr != nil {
		return saveErr
	}
	s.metric.ObserveV2BatchTransition(batch, oldState)
	_ = s.store.ClearOpenBatchID(ctx, BuildBatchScopeKey(
		BuildFileVersionKey(batch.BizID, batch.AppID, batch.FilePath, batch.FileName, batch.FileSignature),
		batch.TargetUser, batch.TargetFileDir))

	targets, err := s.store.ListBatchTargets(ctx, batchID)
	if err != nil {
		return err
	}
	shards := splitTargets(targets, s.cfg.ShardSize)
	batch.ShardCount = len(shards)
	if err := s.store.SaveBatch(ctx, batch); err != nil {
		return err
	}
	logs.Infof(
		"v2 batch dispatch, biz_id=%d app_id=%d batch_id=%s file=%s/%s shard_count=%d "+
			"target_user=%s target_dir=%s target_count=%d dispatch_attempt=%d dispatch_lease_until=%s",
		batch.BizID, batch.AppID, batch.BatchID, batch.FilePath, batch.FileName, batch.ShardCount,
		batch.TargetUser, batch.TargetFileDir, batch.TargetCount, batch.DispatchAttempt,
		batch.DispatchLeaseUntil.Format(time.RFC3339Nano))

	for _, shard := range shards {
		mapping, err := s.dispatchShard(ctx, batch, shard)
		if err != nil {
			logs.Errorf("dispatch batch %s shard failed, err: %v", batchID, err)
		}
		if err := s.store.RecordBatchDispatch(ctx, batchID, mapping); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) dispatchShard(ctx context.Context, batch *types.AsyncDownloadV2Batch,
	targetIDs []string) (map[string]string, error) {
	start := time.Now()
	mapping := make(map[string]string, len(targetIDs))
	if len(targetIDs) == 0 {
		return mapping, nil
	}

	if s.gseService == nil || s.provider == nil || s.cacheDir == "" {
		for _, targetID := range targetIDs {
			if err := s.updateTaskStateByTarget(ctx, batch.BatchID, targetID, types.AsyncDownloadJobStatusRunning, ""); err != nil {
				return nil, err
			}
			mapping[targetID] = "local"
		}
		s.observeShardDispatch("success", start)
		return mapping, nil
	}

	sourceDir := path.Join(s.cacheDir, fmt.Sprintf("%d", batch.BizID))
	if err := os.MkdirAll(sourceDir, os.ModePerm); err != nil {
		return nil, err
	}
	serverFilePath := path.Join(sourceDir, batch.FileSignature)
	kt := kit.NewWithTenant(batch.TenantID)
	kt.BizID = batch.BizID
	kt.AppID = batch.AppID
	if err := s.checkAndDownloadFile(kt, serverFilePath, batch.FileSignature); err != nil {
		return nil, err
	}

	targetAgents := make([]gse.TransferFileAgent, 0, len(targetIDs))
	for _, targetID := range targetIDs {
		agentID, containerID := ParseTargetID(targetID)
		targetAgents = append(targetAgents, gse.TransferFileAgent{
			BkAgentID:     agentID,
			BkContainerID: containerID,
			User:          batch.TargetUser,
		})
	}
	resp, err := s.gseService.AsyncExtensionsTransferFile(kt.Ctx, &gse.TransferFileReq{
		TimeOutSeconds: 600,
		AutoMkdir:      true,
		UploadSpeed:    0,
		DownloadSpeed:  0,
		Tasks: []gse.TransferFileTask{{
			Source: gse.TransferFileSource{
				FileName: batch.FileSignature,
				StoreDir: sourceDir,
				Agent: gse.TransferFileAgent{
					BkAgentID:     s.serverAgentID,
					BkContainerID: s.serverContainerID,
					User:          s.agentUser,
				},
			},
			Target: gse.TransferFileTarget{
				FileName: batch.FileSignature,
				StoreDir: batch.TargetFileDir,
				Agents:   targetAgents,
			},
		}},
	})
	if err != nil {
		for _, targetID := range targetIDs {
			if updateErr := s.updateTaskStateByTarget(ctx, batch.BatchID, targetID, types.AsyncDownloadJobStatusFailed,
				err.Error()); updateErr != nil {
				return nil, updateErr
			}
		}
		s.observeShardDispatch("failed", start)
		return mapping, err
	}

	for _, targetID := range targetIDs {
		if err := s.updateTaskStateByTarget(ctx, batch.BatchID, targetID, types.AsyncDownloadJobStatusRunning, ""); err != nil {
			return nil, err
		}
		mapping[targetID] = resp.Result.TaskID
	}
	s.observeShardDispatch("success", start)
	return mapping, nil
}

func (s *Scheduler) refreshDispatchingBatches(ctx context.Context) error {
	batchIDs, err := s.store.ListDispatchingBatchIDs(ctx)
	if err != nil {
		return err
	}
	for _, batchID := range batchIDs {
		if err := s.refreshDispatchingBatch(ctx, batchID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) refreshDispatchingBatch(ctx context.Context, batchID string) error {
	batch, err := s.store.GetBatch(ctx, batchID)
	if err != nil {
		return err
	}
	if batch.State != types.AsyncDownloadBatchStateDispatching {
		return nil
	}

	dispatchState, err := s.store.ListBatchDispatchState(ctx, batchID)
	if err != nil {
		return err
	}
	if s.gseService != nil {
		kt := kit.NewWithTenant(batch.TenantID)
		taskIDs := make([]string, 0)
		seen := make(map[string]struct{})
		for _, gseTaskID := range dispatchState {
			if gseTaskID == "" || gseTaskID == "local" {
				continue
			}
			if _, ok := seen[gseTaskID]; ok {
				continue
			}
			seen[gseTaskID] = struct{}{}
			taskIDs = append(taskIDs, gseTaskID)
		}
		sort.Strings(taskIDs)
		for _, gseTaskID := range taskIDs {
			resp, resultErr := s.gseService.GetExtensionsTransferFileResult(kt.Ctx,
				&gse.GetTransferFileResultReq{TaskID: gseTaskID})
			if resultErr != nil {
				continue
			}
			for _, result := range resp.Result {
				if result.Content.Type == "upload" {
					if result.ErrorCode == 0 || result.ErrorCode == 115 {
						continue
					}
					for targetID, mappedTaskID := range dispatchState {
						if mappedTaskID != gseTaskID {
							continue
						}
						if updateErr := s.updateTaskStateByTarget(ctx, batchID, targetID,
							types.AsyncDownloadJobStatusFailed, result.ErrorMsg); updateErr != nil {
							return updateErr
						}
					}
					continue
				}
				targetID := BuildTargetID(result.Content.DestAgentID, result.Content.DestContainerID)
				if dispatchState[targetID] != gseTaskID {
					continue
				}
				switch result.ErrorCode {
				case 0:
					err = s.updateTaskStateByTarget(ctx, batchID, targetID, types.AsyncDownloadJobStatusSuccess, "")
				case 115:
					err = s.updateTaskStateByTarget(ctx, batchID, targetID, types.AsyncDownloadJobStatusRunning, "")
				default:
					err = s.updateTaskStateByTarget(ctx, batchID, targetID, types.AsyncDownloadJobStatusFailed, result.ErrorMsg)
				}
				if err != nil {
					return err
				}
			}
		}
	}

	successCount, failedCount, timeoutCount, runningCount, pendingCount, err := s.countBatchTaskStates(ctx, batchID)
	if err != nil {
		return err
	}
	oldLeaseUntil := batch.DispatchLeaseUntil
	batch.SuccessCount = successCount
	batch.FailedCount = failedCount
	batch.TimeoutCount = timeoutCount

	if runningCount == 0 && pendingCount == 0 {
		oldState := batch.State
		batch.State = deriveTerminalBatchState(successCount, failedCount, timeoutCount)
		if err := s.store.SaveBatch(ctx, batch); err != nil {
			return err
		}
		s.metric.ObserveV2BatchTransition(batch, oldState)
		return s.finalizeCompletedBatch(ctx, batch)
	}

	if !oldLeaseUntil.IsZero() && time.Now().After(oldLeaseUntil) {
		return s.RepairTerminalBatch(ctx, batchID, types.AsyncDownloadBatchStatePartial)
	}
	batch.DispatchHeartbeatAt = time.Now()
	batch.DispatchLeaseUntil = batch.DispatchHeartbeatAt.Add(time.Duration(s.cfg.DispatchLeaseSeconds) * time.Second)
	return s.store.SaveBatch(ctx, batch)
}

func (s *Scheduler) checkAndDownloadFile(kt *kit.Kit, filePath, signature string) error {
	if s.provider == nil {
		return nil
	}
	s.fileLock.Acquire(filePath)
	defer s.fileLock.Release(filePath)
	if _, err := os.Stat(filePath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer file.Close()

	reader, _, err := s.provider.Download(kt, signature)
	if err != nil {
		return err
	}
	defer reader.Close()
	if _, err := io.Copy(file, reader); err != nil {
		return err
	}
	return file.Sync()
}

func (s *Scheduler) observeShardDispatch(status string, start time.Time) {
	if s.metric != nil {
		s.metric.ObserveV2ShardDispatch(status, time.Since(start))
	}
}

func (s *Scheduler) Store() *Store {
	return s.store
}

func (s *Scheduler) Metrics() Metrics {
	return s.metric
}

func (s *Scheduler) GSEService() TransferFileClient {
	return s.gseService
}
