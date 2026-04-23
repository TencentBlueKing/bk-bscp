package v2

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

func TestSchedulerClaimsDueBatchAndSetsLease(t *testing.T) {
	sch, store := newTestScheduler(t)
	batchID := seedCollectingBatch(t, store)

	processed, err := sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	batch := mustGetBatch(t, store, batchID)
	require.Equal(t, types.AsyncDownloadBatchStateDispatching, batch.State)
	require.NotZero(t, batch.DispatchLeaseUntil)
	require.Equal(t, 1, batch.DispatchAttempt)
}

func TestSchedulerEnabledIgnoresConfigFlag(t *testing.T) {
	sch := &Scheduler{}
	require.True(t, sch.Enabled())
}

func TestSchedulerRemovesDispatchingBatchFromDueQueue(t *testing.T) {
	sch, store := newTestScheduler(t)
	batchID := seedCollectingBatch(t, store)

	processed, err := sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	dueBatchIDs, err := store.ListDueBatchIDs(context.Background(), time.Now().Add(time.Minute), 10)
	require.NoError(t, err)
	require.NotContains(t, dueBatchIDs, batchID)
}

func TestSchedulerPersistsDispatchingStateBeforePostClaimFailure(t *testing.T) {
	sch, store := newTestScheduler(t)
	batchID := seedCollectingBatch(t, store)

	store.SetSaveBatchHook(func(batch *types.AsyncDownloadV2Batch, call int) error {
		if batch.BatchID == batchID && call == 2 {
			return errors.New("save batch shard count failed")
		}
		return nil
	})

	err := sch.ProcessBatch(context.Background(), batchID)
	require.ErrorContains(t, err, "save batch shard count failed")

	batch := mustGetBatch(t, store, batchID)
	require.Equal(t, types.AsyncDownloadBatchStateDispatching, batch.State)
	require.True(t, batch.OpenUntil.IsZero())

	dueBatchIDs, err := store.ListDueBatchIDs(context.Background(), time.Now().Add(time.Minute), 10)
	require.NoError(t, err)
	require.NotContains(t, dueBatchIDs, batchID)
}

func TestSchedulerLimitsDueBatchFetch(t *testing.T) {
	sch, store := newTestScheduler(t)
	seedManyDueBatches(t, store, 150)

	processed, err := sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 100, processed)
}

func TestRepairMarksTimeoutTaskAfterDispatchCutoff(t *testing.T) {
	sch, store := newTestScheduler(t)
	batchID, taskID := seedBatchWithPendingTaskNotDispatched(t, store)

	err := sch.RepairTimeoutBatch(context.Background(), batchID)
	require.NoError(t, err)

	task := mustGetTask(t, store, taskID)
	require.Equal(t, types.AsyncDownloadJobStatusTimeout, task.State)
	require.Equal(t, "dispatch_timeout", task.ErrMsg)
}

func TestRepairFailsTasksWhenBatchFails(t *testing.T) {
	sch, store := newTestScheduler(t)
	batchID, taskIDs := seedFailedBatchWithPendingTasks(t, store)

	err := sch.FinalizeBatchTasks(context.Background(), batchID, types.AsyncDownloadBatchStateFailed)
	require.NoError(t, err)

	for _, taskID := range taskIDs {
		task := mustGetTask(t, store, taskID)
		require.Equal(t, types.AsyncDownloadJobStatusFailed, task.State)
		require.Equal(t, "batch_failed", task.ErrMsg)
	}
}

func TestSchedulerRecordsShardDispatchMetric(t *testing.T) {
	sch, store := newTestScheduler(t)
	batchID := seedCollectingBatch(t, store)

	processed, err := sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	require.Equal(t, float64(1), testutil.ToFloat64(sch.Metrics().(*testMetrics).shardDispatchCounter.WithLabelValues("success")))

	batch := mustGetBatch(t, store, batchID)
	require.Equal(t, types.AsyncDownloadBatchStateDispatching, batch.State)
}

func TestRepairRecordsTaskRepairMetric(t *testing.T) {
	sch, store := newTestScheduler(t)
	batchID, _ := seedBatchWithPendingTaskNotDispatched(t, store)

	err := sch.RepairTimeoutBatch(context.Background(), batchID)
	require.NoError(t, err)
	require.Equal(t, float64(1), testutil.ToFloat64(
		sch.Metrics().(*testMetrics).taskRepairCounter.WithLabelValues("dispatch_timeout")))
}

func TestPendingTaskWithoutDispatchMappingDoesNotExtendLeaseWithoutProgress(t *testing.T) {
	sch, store := newTestScheduler(t)
	batchID, taskID := seedBatchWithPendingTaskNotDispatched(t, store)

	batch := mustGetBatch(t, store, batchID)
	shortLeaseUntil := time.Now().Add(120 * time.Millisecond)
	batch.DispatchLeaseUntil = shortLeaseUntil
	require.NoError(t, store.SaveBatch(context.Background(), batch))

	time.Sleep(40 * time.Millisecond)

	processed, err := sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, processed, 0)

	batch = mustGetBatch(t, store, batchID)
	require.Equal(t, shortLeaseUntil.UnixMilli(), batch.DispatchLeaseUntil.UnixMilli())

	time.Sleep(time.Until(shortLeaseUntil) + 40*time.Millisecond)

	processed, err = sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, processed, 0)

	task := mustGetTask(t, store, taskID)
	require.Equal(t, types.AsyncDownloadJobStatusTimeout, task.State)
	require.Equal(t, "dispatch_timeout", task.ErrMsg)

	batch = mustGetBatch(t, store, batchID)
	require.Equal(t, types.AsyncDownloadBatchStateFailed, batch.State)
	require.Equal(t, "dispatch_timeout", batch.FinalReason)
	require.Equal(t, 1, batch.TimeoutCount)
}

func TestLifecycleMetrics(t *testing.T) {
	svc, sch, kt := newIntegratedTestHarness(t)

	taskID, err := svc.CreateTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)
	forceTaskBatchDue(t, svc, sch, kt, taskID)

	processed, err := sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	metrics := sch.Metrics().(*testMetrics)
	require.Equal(t, float64(1), testutil.ToFloat64(
		metrics.v2BatchStateCounter.WithLabelValues("706", "192", types.AsyncDownloadBatchStateCollecting)))
	require.Equal(t, float64(1), testutil.ToFloat64(
		metrics.v2BatchStateCounter.WithLabelValues("706", "192", types.AsyncDownloadBatchStateDispatching)))
	require.Equal(t, float64(1), testutil.ToFloat64(
		metrics.v2BatchStateCounter.WithLabelValues("706", "192", types.AsyncDownloadBatchStateDone)))
	require.Equal(t, float64(1), testutil.ToFloat64(
		metrics.v2TaskStateCounter.WithLabelValues("706", "192", types.AsyncDownloadJobStatusPending)))
	require.Equal(t, float64(1), testutil.ToFloat64(
		metrics.v2TaskStateCounter.WithLabelValues("706", "192", types.AsyncDownloadJobStatusRunning)))
	require.Equal(t, float64(1), testutil.ToFloat64(
		metrics.v2TaskStateCounter.WithLabelValues("706", "192", types.AsyncDownloadJobStatusSuccess)))
	require.Equal(t, uint64(1), histogramSampleCount(t,
		metrics.v2BatchStateDurationSeconds.WithLabelValues("706", "192", types.AsyncDownloadBatchStateCollecting)))
	require.Equal(t, uint64(1), histogramSampleCount(t,
		metrics.v2TaskStateDurationSeconds.WithLabelValues("706", "192", types.AsyncDownloadJobStatusPending)))
}

func TestUsesBatchTenantForGSECalls(t *testing.T) {
	svc, sch, kt := newIntegratedTestHarness(t)
	kt.TenantID = "tenant-v2-a"

	taskID, err := svc.CreateTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)
	forceTaskBatchDue(t, svc, sch, kt, taskID)

	processed, err := sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	gseClient := mustGetFakeTransferClient(t, sch)
	require.Equal(t, "tenant-v2-a", gseClient.lastTransferTenantID)
	require.Equal(t, "tenant-v2-a", gseClient.lastResultTenantID)
}

func newTestScheduler(t *testing.T) (*Scheduler, *Store) {
	t.Helper()

	mr := miniredis.RunT(t)
	opt := cc.RedisCluster{Mode: cc.RedisStandaloneMode, Endpoints: []string{mr.Addr()}}
	bds, err := bedis.NewRedisCache(opt)
	require.NoError(t, err)

	cfg := cc.AsyncDownloadV2{
		Enabled:                  true,
		CollectWindowSeconds:     10,
		MaxTargetsPerBatch:       5000,
		ShardSize:                500,
		DispatchHeartbeatSeconds: 15,
		DispatchLeaseSeconds:     60,
		MaxDispatchAttempts:      3,
		MaxDueBatchesPerTick:     100,
		TaskTTLSeconds:           86400,
		BatchTTLSeconds:          86400,
	}
	store := NewStore(bds, cfg)
	return NewScheduler(store, nil, nil, lock.NewRedisLock(bds, 5), lock.NewFileLock(), newTestMetrics(),
		"server-agent", "server-container", "root", t.TempDir(), cfg), store
}

func seedCollectingBatch(t *testing.T, store *Store) string {
	t.Helper()
	ctx := context.Background()
	now := time.Now().Add(-time.Minute)
	batch := &types.AsyncDownloadV2Batch{
		BatchID:       "batch-1",
		TenantID:      "t-1",
		BizID:         706,
		AppID:         192,
		FilePath:      "/cfg",
		FileName:      "protocol.tar.gz",
		FileSignature: "sig-1",
		State:         types.AsyncDownloadBatchStateCollecting,
		OpenUntil:     now,
		CreatedAt:     now.Add(-time.Minute),
		TargetCount:   1,
	}
	task := &types.AsyncDownloadV2Task{
		TaskID:        "task-1",
		BatchID:       batch.BatchID,
		TargetID:      BuildTargetID("agent-a", "container-a"),
		BizID:         batch.BizID,
		AppID:         batch.AppID,
		TenantID:      batch.TenantID,
		FilePath:      batch.FilePath,
		FileName:      batch.FileName,
		FileSignature: batch.FileSignature,
		State:         types.AsyncDownloadJobStatusPending,
		CreatedAt:     now.Add(-time.Minute),
		UpdatedAt:     now.Add(-time.Minute),
	}
	err := store.CreateBatchAndTask(ctx, BuildFileVersionKey(batch.BizID, batch.AppID, batch.FilePath, batch.FileName,
		batch.FileSignature), batch.BatchID, task.TargetID, task.TaskID, batch, task)
	require.NoError(t, err)
	return batch.BatchID
}

func seedManyDueBatches(t *testing.T, store *Store, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		batchID := seedCollectingBatch(t, store)
		batch, err := store.GetBatch(context.Background(), batchID)
		require.NoError(t, err)
		batch.BatchID = "batch-many-" + time.Now().Add(time.Duration(i)*time.Nanosecond).Format("150405.000000000")
		task := &types.AsyncDownloadV2Task{
			TaskID:        "task-many-" + batch.BatchID,
			BatchID:       batch.BatchID,
			TargetID:      BuildTargetID("agent-a", batch.BatchID),
			BizID:         batch.BizID,
			AppID:         batch.AppID,
			TenantID:      batch.TenantID,
			FilePath:      batch.FilePath,
			FileName:      batch.FileName,
			FileSignature: batch.FileSignature,
			State:         types.AsyncDownloadJobStatusPending,
			CreatedAt:     batch.CreatedAt,
			UpdatedAt:     batch.CreatedAt,
		}
		err = store.CreateBatchAndTask(context.Background(), BuildFileVersionKey(batch.BizID, batch.AppID, batch.FilePath,
			batch.FileName, batch.FileSignature), batch.BatchID, task.TargetID, task.TaskID, batch, task)
		require.NoError(t, err)
	}
}

func seedBatchWithPendingTaskNotDispatched(t *testing.T, store *Store) (string, string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().Add(-2 * time.Minute)
	batch := &types.AsyncDownloadV2Batch{
		BatchID:            "batch-repair-1",
		TenantID:           "t-1",
		BizID:              706,
		AppID:              192,
		FilePath:           "/cfg",
		FileName:           "protocol.tar.gz",
		FileSignature:      "sig-1",
		State:              types.AsyncDownloadBatchStateDispatching,
		CreatedAt:          now,
		DispatchStartedAt:  now,
		DispatchLeaseUntil: now,
		DispatchAttempt:    1,
		TargetCount:        1,
	}
	task := &types.AsyncDownloadV2Task{
		TaskID:        "task-repair-1",
		BatchID:       batch.BatchID,
		TargetID:      BuildTargetID("agent-a", "container-a"),
		BizID:         batch.BizID,
		AppID:         batch.AppID,
		TenantID:      batch.TenantID,
		FilePath:      batch.FilePath,
		FileName:      batch.FileName,
		FileSignature: batch.FileSignature,
		State:         types.AsyncDownloadJobStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	require.NoError(t, store.CreateBatchAndTask(ctx, BuildFileVersionKey(batch.BizID, batch.AppID, batch.FilePath,
		batch.FileName, batch.FileSignature), batch.BatchID, task.TargetID, task.TaskID, batch, task))
	require.NoError(t, store.RemoveDueBatchID(ctx, batch.BatchID))
	return batch.BatchID, task.TaskID
}

func seedFailedBatchWithPendingTasks(t *testing.T, store *Store) (string, []string) {
	t.Helper()
	batchID, taskID := seedBatchWithPendingTaskNotDispatched(t, store)
	return batchID, []string{taskID}
}

func mustGetBatch(t *testing.T, store *Store, batchID string) *types.AsyncDownloadV2Batch {
	t.Helper()
	batch, err := store.GetBatch(context.Background(), batchID)
	require.NoError(t, err)
	return batch
}

func mustGetTask(t *testing.T, store *Store, taskID string) *types.AsyncDownloadV2Task {
	t.Helper()
	task, err := store.GetTask(context.Background(), taskID)
	require.NoError(t, err)
	return task
}
