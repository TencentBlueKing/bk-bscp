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
	"sort"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/runtime/jsoni"
)

type Store struct {
	bds bedis.Client
	cfg cc.AsyncDownloadV2

	saveBatchCalls int
	saveBatchHook  func(batch *types.AsyncDownloadV2Batch, call int) error
}

func NewStore(bds bedis.Client, cfg cc.AsyncDownloadV2) *Store {
	return &Store{bds: bds, cfg: cfg}
}

func (s *Store) CreateBatchAndTask(ctx context.Context, fileVersionKey, batchID, targetID, taskID string,
	batch *types.AsyncDownloadV2Batch, task *types.AsyncDownloadV2Task) error {
	if err := s.SaveBatch(ctx, batch); err != nil {
		return err
	}
	if err := s.SaveTask(ctx, task); err != nil {
		return err
	}
	if err := s.bds.HSets(ctx, batchTargetsKey(batchID), map[string]string{targetID: taskID}, s.BatchTTL()); err != nil {
		return err
	}
	if err := s.bds.HSets(ctx, batchTasksKey(batchID), map[string]string{taskID: targetID}, s.BatchTTL()); err != nil {
		return err
	}
	if err := s.bds.Set(ctx, inflightKey(fileVersionKey,
		BuildInflightTargetKey(targetID, task.TargetUser, task.TargetFileDir)), taskID, s.TaskTTL()); err != nil {
		return err
	}
	if err := s.bds.Set(ctx, batchOpenKey(BuildBatchScopeKey(fileVersionKey, batch.TargetUser, batch.TargetFileDir)),
		batchID, s.BatchTTL()); err != nil {
		return err
	}
	return nil
}

func (s *Store) AddTaskToBatch(ctx context.Context, batchID, fileVersionKey, targetID, taskID string,
	task *types.AsyncDownloadV2Task) error {
	if err := s.SaveTask(ctx, task); err != nil {
		return err
	}
	if err := s.bds.HSets(ctx, batchTargetsKey(batchID), map[string]string{targetID: taskID}, s.BatchTTL()); err != nil {
		return err
	}
	if err := s.bds.HSets(ctx, batchTasksKey(batchID), map[string]string{taskID: targetID}, s.BatchTTL()); err != nil {
		return err
	}
	return s.bds.Set(ctx, inflightKey(fileVersionKey,
		BuildInflightTargetKey(targetID, task.TargetUser, task.TargetFileDir)), taskID, s.TaskTTL())
}

func (s *Store) SaveBatch(ctx context.Context, batch *types.AsyncDownloadV2Batch) error {
	s.saveBatchCalls++
	if s.saveBatchHook != nil {
		if err := s.saveBatchHook(batch, s.saveBatchCalls); err != nil {
			return err
		}
	}
	payload, err := jsoni.Marshal(batch)
	if err != nil {
		return err
	}
	if setErr := s.bds.Set(ctx, batchMetaKey(batch.BatchID), string(payload), s.BatchTTL()); setErr != nil {
		return setErr
	}
	if batch.State == types.AsyncDownloadBatchStateCollecting && !batch.OpenUntil.IsZero() {
		if _, addErr := s.bds.ZAdd(ctx, v2DueBatchesKey, float64(batch.OpenUntil.Unix()), batch.BatchID); addErr != nil {
			return addErr
		}
		return nil
	}
	_, err = s.bds.ZRem(ctx, v2DueBatchesKey, batch.BatchID)
	return err
}

func (s *Store) SaveTask(ctx context.Context, task *types.AsyncDownloadV2Task) error {
	payload, err := jsoni.Marshal(task)
	if err != nil {
		return err
	}
	return s.bds.Set(ctx, taskMetaKey(task.TaskID), string(payload), s.TaskTTL())
}

func (s *Store) GetInflightTaskID(ctx context.Context, fileVersionKey, inflightTargetKey string) (string, error) {
	return s.bds.Get(ctx, inflightKey(fileVersionKey, inflightTargetKey))
}

func (s *Store) ClearInflightTaskID(ctx context.Context, fileVersionKey, inflightTargetKey string) error {
	return s.bds.Delete(ctx, inflightKey(fileVersionKey, inflightTargetKey))
}

func (s *Store) GetOpenBatchID(ctx context.Context, batchScopeKey string) (string, error) {
	return s.bds.Get(ctx, batchOpenKey(batchScopeKey))
}

func (s *Store) ClearOpenBatchID(ctx context.Context, batchScopeKey string) error {
	return s.bds.Delete(ctx, batchOpenKey(batchScopeKey))
}

func (s *Store) GetBatch(ctx context.Context, batchID string) (*types.AsyncDownloadV2Batch, error) {
	payload, err := s.bds.Get(ctx, batchMetaKey(batchID))
	if err != nil {
		return nil, err
	}
	if payload == "" {
		return nil, fmt.Errorf("async download v2 batch %s not exists in redis", batchID)
	}
	batch := new(types.AsyncDownloadV2Batch)
	if err := jsoni.UnmarshalFromString(payload, batch); err != nil {
		return nil, err
	}
	return batch, nil
}

func (s *Store) GetTask(ctx context.Context, taskID string) (*types.AsyncDownloadV2Task, error) {
	payload, err := s.bds.Get(ctx, taskMetaKey(taskID))
	if err != nil {
		return nil, err
	}
	if payload == "" {
		return nil, fmt.Errorf("async download v2 task %s not exists in redis", taskID)
	}
	task := new(types.AsyncDownloadV2Task)
	if err := jsoni.UnmarshalFromString(payload, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *Store) GetBatchTaskID(ctx context.Context, batchID, targetID string) (string, error) {
	taskID, err := s.bds.HGet(ctx, batchTargetsKey(batchID), targetID)
	if err == bedis.ErrKeyNotExist {
		return "", nil
	}
	return taskID, err
}

func (s *Store) ListBatchTargets(ctx context.Context, batchID string) ([]string, error) {
	items, err := s.bds.HGetAll(ctx, batchTargetsKey(batchID))
	if err != nil {
		return nil, err
	}
	targets := make([]string, 0, len(items))
	for targetID := range items {
		targets = append(targets, targetID)
	}
	sort.Strings(targets)
	return targets, nil
}

func (s *Store) ListBatchTasks(ctx context.Context, batchID string) ([]string, error) {
	items, err := s.bds.HGetAll(ctx, batchTasksKey(batchID))
	if err != nil {
		return nil, err
	}
	taskIDs := make([]string, 0, len(items))
	for taskID := range items {
		taskIDs = append(taskIDs, taskID)
	}
	sort.Strings(taskIDs)
	return taskIDs, nil
}

func (s *Store) ListBatchTargetTasks(ctx context.Context, batchID string) (map[string]string, error) {
	return s.bds.HGetAll(ctx, batchTargetsKey(batchID))
}

func (s *Store) ListDueBatchIDs(ctx context.Context, now time.Time, limit int) ([]string, error) {
	if limit <= 0 {
		return []string{}, nil
	}
	items, err := s.bds.ZRangeByScoreWithScores(ctx, v2DueBatchesKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   strconv.FormatInt(now.Unix(), 10),
		Count: int64(limit),
	})
	if err != nil {
		return nil, err
	}
	batchIDs := make([]string, 0, len(items))
	for _, item := range items {
		member, ok := item.Member.(string)
		if !ok || member == "" {
			continue
		}
		batchIDs = append(batchIDs, member)
	}
	return batchIDs, nil
}

func (s *Store) RemoveDueBatchID(ctx context.Context, batchID string) error {
	_, err := s.bds.ZRem(ctx, v2DueBatchesKey, batchID)
	return err
}

func (s *Store) ListDispatchingBatchIDs(ctx context.Context) ([]string, error) {
	keys, err := s.bds.Keys(ctx, batchMetaPattern())
	if err != nil {
		return nil, err
	}
	batchIDs := make([]string, 0)
	for _, key := range keys {
		payload, err := s.bds.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if payload == "" {
			continue
		}
		batch := new(types.AsyncDownloadV2Batch)
		if err := jsoni.UnmarshalFromString(payload, batch); err != nil {
			return nil, err
		}
		if batch.State == types.AsyncDownloadBatchStateDispatching {
			batchIDs = append(batchIDs, batch.BatchID)
		}
	}
	sort.Strings(batchIDs)
	return batchIDs, nil
}

func (s *Store) RecordBatchDispatch(ctx context.Context, batchID string, mapping map[string]string) error {
	if len(mapping) == 0 {
		return nil
	}
	return s.bds.HSets(ctx, batchDispatchedTargetsKey(batchID), mapping, s.BatchTTL())
}

func (s *Store) ListBatchDispatchState(ctx context.Context, batchID string) (map[string]string, error) {
	return s.bds.HGetAll(ctx, batchDispatchedTargetsKey(batchID))
}

func (s *Store) BatchTTL() int {
	if s.cfg.BatchTTLSeconds > 0 {
		return s.cfg.BatchTTLSeconds
	}
	return 86400
}

func (s *Store) TaskTTL() int {
	if s.cfg.TaskTTLSeconds > 0 {
		return s.cfg.TaskTTLSeconds
	}
	return 86400
}

func (s *Store) Client() bedis.Client {
	return s.bds
}

func (s *Store) SetSaveBatchHook(hook func(batch *types.AsyncDownloadV2Batch, call int) error) {
	s.saveBatchCalls = 0
	s.saveBatchHook = hook
}
