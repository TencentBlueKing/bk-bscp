package asyncdownload

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

type v2Store struct {
	bds bedis.Client
	cfg cc.AsyncDownloadV2

	saveBatchCalls int
	saveBatchHook  func(batch *types.AsyncDownloadV2Batch, call int) error
}

func newV2Store(bds bedis.Client, cfg cc.AsyncDownloadV2) *v2Store {
	return &v2Store{bds: bds, cfg: cfg}
}

func (s *v2Store) createBatchAndTask(ctx context.Context, fileVersionKey, batchID, targetID, taskID string,
	batch *types.AsyncDownloadV2Batch, task *types.AsyncDownloadV2Task) error {
	if err := s.saveBatch(ctx, batch); err != nil {
		return err
	}
	if err := s.saveTask(ctx, task); err != nil {
		return err
	}
	if err := s.bds.HSets(ctx, batchTargetsKey(batchID), map[string]string{targetID: taskID}, s.batchTTL()); err != nil {
		return err
	}
	if err := s.bds.HSets(ctx, batchTasksKey(batchID), map[string]string{taskID: targetID}, s.batchTTL()); err != nil {
		return err
	}
	if err := s.bds.Set(ctx, inflightKey(fileVersionKey,
		buildInflightTargetKey(targetID, task.TargetUser, task.TargetFileDir)), taskID, s.taskTTL()); err != nil {
		return err
	}
	if err := s.bds.Set(ctx, batchOpenKey(buildBatchScopeKey(fileVersionKey, batch.TargetUser, batch.TargetFileDir)),
		batchID, s.batchTTL()); err != nil {
		return err
	}
	return nil
}

func (s *v2Store) addTaskToBatch(ctx context.Context, batchID, fileVersionKey, targetID, taskID string,
	task *types.AsyncDownloadV2Task) error {
	if err := s.saveTask(ctx, task); err != nil {
		return err
	}
	if err := s.bds.HSets(ctx, batchTargetsKey(batchID), map[string]string{targetID: taskID}, s.batchTTL()); err != nil {
		return err
	}
	if err := s.bds.HSets(ctx, batchTasksKey(batchID), map[string]string{taskID: targetID}, s.batchTTL()); err != nil {
		return err
	}
	return s.bds.Set(ctx, inflightKey(fileVersionKey,
		buildInflightTargetKey(targetID, task.TargetUser, task.TargetFileDir)), taskID, s.taskTTL())
}

func (s *v2Store) saveBatch(ctx context.Context, batch *types.AsyncDownloadV2Batch) error {
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
	if err := s.bds.Set(ctx, batchMetaKey(batch.BatchID), string(payload), s.batchTTL()); err != nil {
		return err
	}
	if batch.State == types.AsyncDownloadBatchStateCollecting && !batch.OpenUntil.IsZero() {
		if _, err := s.bds.ZAdd(ctx, v2DueBatchesKey, float64(batch.OpenUntil.Unix()), batch.BatchID); err != nil {
			return err
		}
		return nil
	}
	_, err = s.bds.ZRem(ctx, v2DueBatchesKey, batch.BatchID)
	return err
}

func (s *v2Store) saveTask(ctx context.Context, task *types.AsyncDownloadV2Task) error {
	payload, err := jsoni.Marshal(task)
	if err != nil {
		return err
	}
	return s.bds.Set(ctx, taskMetaKey(task.TaskID), string(payload), s.taskTTL())
}

func (s *v2Store) getInflightTaskID(ctx context.Context, fileVersionKey, inflightTargetKey string) (string, error) {
	return s.bds.Get(ctx, inflightKey(fileVersionKey, inflightTargetKey))
}

func (s *v2Store) clearInflightTaskID(ctx context.Context, fileVersionKey, inflightTargetKey string) error {
	return s.bds.Delete(ctx, inflightKey(fileVersionKey, inflightTargetKey))
}

func (s *v2Store) getOpenBatchID(ctx context.Context, batchScopeKey string) (string, error) {
	return s.bds.Get(ctx, batchOpenKey(batchScopeKey))
}

func (s *v2Store) clearOpenBatchID(ctx context.Context, batchScopeKey string) error {
	return s.bds.Delete(ctx, batchOpenKey(batchScopeKey))
}

func (s *v2Store) getBatch(ctx context.Context, batchID string) (*types.AsyncDownloadV2Batch, error) {
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

func (s *v2Store) getTask(ctx context.Context, taskID string) (*types.AsyncDownloadV2Task, error) {
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

func (s *v2Store) getBatchTaskID(ctx context.Context, batchID, targetID string) (string, error) {
	taskID, err := s.bds.HGet(ctx, batchTargetsKey(batchID), targetID)
	if err == bedis.ErrKeyNotExist {
		return "", nil
	}
	return taskID, err
}

func (s *v2Store) listBatchTargets(ctx context.Context, batchID string) ([]string, error) {
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

func (s *v2Store) listBatchTasks(ctx context.Context, batchID string) ([]string, error) {
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

func (s *v2Store) listBatchTargetTasks(ctx context.Context, batchID string) (map[string]string, error) {
	return s.bds.HGetAll(ctx, batchTargetsKey(batchID))
}

func (s *v2Store) listDueBatchIDs(ctx context.Context, now time.Time, limit int) ([]string, error) {
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

func (s *v2Store) removeDueBatchID(ctx context.Context, batchID string) error {
	_, err := s.bds.ZRem(ctx, v2DueBatchesKey, batchID)
	return err
}

func (s *v2Store) listDispatchingBatchIDs(ctx context.Context) ([]string, error) {
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

func (s *v2Store) recordBatchDispatch(ctx context.Context, batchID string, mapping map[string]string) error {
	if len(mapping) == 0 {
		return nil
	}
	return s.bds.HSets(ctx, batchDispatchedTargetsKey(batchID), mapping, s.batchTTL())
}

func (s *v2Store) listBatchDispatchState(ctx context.Context, batchID string) (map[string]string, error) {
	return s.bds.HGetAll(ctx, batchDispatchedTargetsKey(batchID))
}

func (s *v2Store) batchTTL() int {
	if s.cfg.BatchTTLSeconds > 0 {
		return s.cfg.BatchTTLSeconds
	}
	return 86400
}

func (s *v2Store) taskTTL() int {
	if s.cfg.TaskTTLSeconds > 0 {
		return s.cfg.TaskTTLSeconds
	}
	return 86400
}
