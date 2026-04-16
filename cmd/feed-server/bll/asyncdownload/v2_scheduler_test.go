package asyncdownload

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/runtime/jsoni"
)

func TestV2SchedulerClaimsDueBatchAndSetsLease(t *testing.T) {
	sch, store := newTestV2Scheduler(t)
	batchID := seedCollectingBatch(t, store)

	processed, err := sch.processDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	batch := mustGetBatch(t, store, batchID)
	require.Equal(t, types.AsyncDownloadBatchStateDispatching, batch.State)
	require.NotZero(t, batch.DispatchLeaseUntil)
	require.Equal(t, 1, batch.DispatchAttempt)
}

func TestV2SchedulerEnabledIgnoresConfigFlag(t *testing.T) {
	sch := &v2Scheduler{cfg: cc.AsyncDownloadV2{Enabled: false}}
	require.True(t, sch.enabled())
}

func TestV2SchedulerRemovesDispatchingBatchFromDueQueue(t *testing.T) {
	sch, store := newTestV2Scheduler(t)
	batchID := seedCollectingBatch(t, store)

	processed, err := sch.processDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	dueBatchIDs, err := store.listDueBatchIDs(context.Background(), time.Now().Add(time.Minute), 10)
	require.NoError(t, err)
	require.NotContains(t, dueBatchIDs, batchID)
}

func TestV2SchedulerPersistsDispatchingStateBeforePostClaimFailure(t *testing.T) {
	sch, store := newTestV2Scheduler(t)
	batchID := seedCollectingBatch(t, store)

	store.saveBatchCalls = 0
	store.saveBatchHook = func(batch *types.AsyncDownloadV2Batch, call int) error {
		if batch.BatchID == batchID && call == 2 {
			return errors.New("save batch shard count failed")
		}
		return nil
	}

	err := sch.processBatch(context.Background(), batchID)
	require.ErrorContains(t, err, "save batch shard count failed")

	batch := mustGetBatch(t, store, batchID)
	require.Equal(t, types.AsyncDownloadBatchStateDispatching, batch.State)
	require.True(t, batch.OpenUntil.IsZero())

	dueBatchIDs, err := store.listDueBatchIDs(context.Background(), time.Now().Add(time.Minute), 10)
	require.NoError(t, err)
	require.NotContains(t, dueBatchIDs, batchID)
}

func TestV2SchedulerLimitsDueBatchFetch(t *testing.T) {
	sch, store := newTestV2Scheduler(t)
	seedManyDueBatches(t, store, 150)

	processed, err := sch.processDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 100, processed)
}

func TestV2RepairMarksOrphanTaskFailedAfterDispatchCutoff(t *testing.T) {
	sch, store := newTestV2Scheduler(t)
	batchID, taskID := seedBatchWithPendingTaskNotDispatched(t, store)

	err := sch.repairTerminalBatch(context.Background(), batchID, types.AsyncDownloadBatchStatePartial)
	require.NoError(t, err)

	task := mustGetTask(t, store, taskID)
	require.Equal(t, types.AsyncDownloadJobStatusFailed, task.State)
	require.Equal(t, "orphan_after_dispatch_cutoff", task.ErrMsg)
}

func TestV2RepairFailsTasksWhenBatchFails(t *testing.T) {
	sch, store := newTestV2Scheduler(t)
	batchID, taskIDs := seedFailedBatchWithPendingTasks(t, store)

	err := sch.finalizeBatchTasks(context.Background(), batchID, types.AsyncDownloadBatchStateFailed)
	require.NoError(t, err)

	for _, taskID := range taskIDs {
		task := mustGetTask(t, store, taskID)
		require.Equal(t, types.AsyncDownloadJobStatusFailed, task.State)
		require.Equal(t, "batch_failed", task.ErrMsg)
	}
}

func TestLegacyV1JobStillDrainsInFirstV2Release(t *testing.T) {
	sch, store := newLegacyCompatibleScheduler(t)
	legacyJobID := seedLegacyPendingJob(t, store)

	err := sch.runOneV1DrainPass(context.Background())
	require.NoError(t, err)

	job := mustGetLegacyJob(t, store, legacyJobID)
	require.NotEqual(t, types.AsyncDownloadJobStatusPending, job.Status)
}

func TestV2MetricsRegisterBacklogAndRepairCounters(t *testing.T) {
	m := InitMetric()

	require.NotNil(t, m.batchDueBacklog)
	require.NotNil(t, m.batchOldestDueAgeSeconds)
	require.NotNil(t, m.taskRepairCounter)
}

func TestV2SchedulerRecordsShardDispatchMetric(t *testing.T) {
	sch, store := newTestV2Scheduler(t)
	batchID := seedCollectingBatch(t, store)

	processed, err := sch.processDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	require.Equal(t, float64(1), testutil.ToFloat64(sch.metric.shardDispatchCounter.WithLabelValues("success")))

	batch := mustGetBatch(t, store, batchID)
	require.Equal(t, types.AsyncDownloadBatchStateDispatching, batch.State)
}

func TestV2RepairRecordsTaskRepairMetric(t *testing.T) {
	sch, store := newTestV2Scheduler(t)
	batchID, _ := seedBatchWithPendingTaskNotDispatched(t, store)

	err := sch.repairTerminalBatch(context.Background(), batchID, types.AsyncDownloadBatchStatePartial)
	require.NoError(t, err)
	require.Equal(t, float64(1), testutil.ToFloat64(
		sch.metric.taskRepairCounter.WithLabelValues("orphan_after_dispatch_cutoff")))
}

func TestAsyncDownloadV2RecordsLifecycleMetrics(t *testing.T) {
	svc, sch, kt := newIntegratedV2TestHarness(t)

	taskID, err := svc.CreateAsyncDownloadTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)
	forceTaskBatchDue(t, svc, sch, kt, taskID)

	processed, err := sch.processDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	require.Equal(t, float64(1), testutil.ToFloat64(
		sch.metric.v2BatchStateCounter.WithLabelValues("706", "192", "/cfg/protocol.tar.gz",
			types.AsyncDownloadBatchStateCollecting)))
	require.Equal(t, float64(1), testutil.ToFloat64(
		sch.metric.v2BatchStateCounter.WithLabelValues("706", "192", "/cfg/protocol.tar.gz",
			types.AsyncDownloadBatchStateDispatching)))
	require.Equal(t, float64(1), testutil.ToFloat64(
		sch.metric.v2BatchStateCounter.WithLabelValues("706", "192", "/cfg/protocol.tar.gz",
			types.AsyncDownloadBatchStateDone)))

	require.Equal(t, float64(1), testutil.ToFloat64(
		sch.metric.v2TaskStateCounter.WithLabelValues("706", "192", "/cfg/protocol.tar.gz",
			types.AsyncDownloadJobStatusPending)))
	require.Equal(t, float64(1), testutil.ToFloat64(
		sch.metric.v2TaskStateCounter.WithLabelValues("706", "192", "/cfg/protocol.tar.gz",
			types.AsyncDownloadJobStatusRunning)))
	require.Equal(t, float64(1), testutil.ToFloat64(
		sch.metric.v2TaskStateCounter.WithLabelValues("706", "192", "/cfg/protocol.tar.gz",
			types.AsyncDownloadJobStatusSuccess)))

	require.Equal(t, uint64(1), histogramSampleCount(t,
		sch.metric.v2BatchStateDurationSeconds.WithLabelValues("706", "192", "/cfg/protocol.tar.gz",
			types.AsyncDownloadBatchStateCollecting)))
	require.Equal(t, uint64(1), histogramSampleCount(t,
		sch.metric.v2TaskStateDurationSeconds.WithLabelValues("706", "192", "/cfg/protocol.tar.gz",
			types.AsyncDownloadJobStatusPending)))
}

func newTestV2Scheduler(t *testing.T) (*v2Scheduler, *v2Store) {
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
	store := newV2Store(bds, cfg)
	return newV2Scheduler(store, nil, nil, lock.NewRedisLock(bds, 5), lock.NewFileLock(), newTestMetric(),
		"server-agent", "server-container", "root", t.TempDir(), cfg), store
}

func newLegacyCompatibleScheduler(t *testing.T) (*Scheduler, *v2Store) {
	t.Helper()

	cc.InitService(cc.FeedServerName)
	cc.InitRuntime(&cc.FeedServerSetting{
		GSE: cc.GSE{
			CacheDir: t.TempDir(),
		},
	})

	mr := miniredis.RunT(t)
	opt := cc.RedisCluster{Mode: cc.RedisStandaloneMode, Endpoints: []string{mr.Addr()}}
	bds, err := bedis.NewRedisCache(opt)
	require.NoError(t, err)

	cfg := cc.AsyncDownloadV2{
		Enabled:              true,
		MaxDueBatchesPerTick: 100,
		TaskTTLSeconds:       86400,
		BatchTTLSeconds:      86400,
	}
	store := newV2Store(bds, cfg)
	return &Scheduler{
		gseService:    &fakeTransferClient{transferTaskID: "gse-task-1"},
		ctx:           context.Background(),
		bds:           bds,
		redLock:       lock.NewRedisLock(bds, 5),
		fileLock:      lock.NewFileLock(),
		provider:      fakeDownloader{content: "demo"},
		serverAgentID: "server-agent",
		metric:        newTestMetric(),
		v2: newV2Scheduler(store, nil, nil, lock.NewRedisLock(bds, 5), lock.NewFileLock(), newTestMetric(),
			"server-agent", "server-container", "root", t.TempDir(), cfg),
	}, store
}

func seedCollectingBatch(t *testing.T, store *v2Store) string {
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
		TargetID:      buildTargetID("agent-a", "container-a"),
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
	err := store.createBatchAndTask(ctx, buildFileVersionKey(batch.BizID, batch.AppID, batch.FilePath, batch.FileName,
		batch.FileSignature), batch.BatchID, task.TargetID, task.TaskID, batch, task)
	require.NoError(t, err)
	return batch.BatchID
}

func histogramSampleCount(t *testing.T, collector interface{}) uint64 {
	t.Helper()
	metric, ok := collector.(interface{ Write(*dto.Metric) error })
	require.True(t, ok)
	dtoMetric := &dto.Metric{}
	require.NoError(t, metric.Write(dtoMetric))
	return dtoMetric.GetHistogram().GetSampleCount()
}

func TestAsyncDownloadV2UsesBatchTenantForGSECalls(t *testing.T) {
	svc, sch, kt := newIntegratedV2TestHarness(t)
	kt.TenantID = "tenant-v2-a"

	taskID, err := svc.CreateAsyncDownloadTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)
	forceTaskBatchDue(t, svc, sch, kt, taskID)

	processed, err := sch.processDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	gseClient := mustGetFakeTransferClient(t, sch)
	require.Equal(t, "tenant-v2-a", gseClient.lastTransferTenantID)
	require.Equal(t, "tenant-v2-a", gseClient.lastResultTenantID)
}

func seedManyDueBatches(t *testing.T, store *v2Store, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		batchID := seedCollectingBatch(t, store)
		batch, err := store.getBatch(context.Background(), batchID)
		require.NoError(t, err)
		batch.BatchID = "batch-many-" + time.Now().Add(time.Duration(i)*time.Nanosecond).Format("150405.000000000")
		task := &types.AsyncDownloadV2Task{
			TaskID:        "task-many-" + batch.BatchID,
			BatchID:       batch.BatchID,
			TargetID:      buildTargetID("agent-a", batch.BatchID),
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
		err = store.createBatchAndTask(context.Background(), buildFileVersionKey(batch.BizID, batch.AppID, batch.FilePath,
			batch.FileName, batch.FileSignature), batch.BatchID, task.TargetID, task.TaskID, batch, task)
		require.NoError(t, err)
	}
}

func seedBatchWithPendingTaskNotDispatched(t *testing.T, store *v2Store) (string, string) {
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
		TargetID:      buildTargetID("agent-a", "container-a"),
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
	require.NoError(t, store.createBatchAndTask(ctx, buildFileVersionKey(batch.BizID, batch.AppID, batch.FilePath,
		batch.FileName, batch.FileSignature), batch.BatchID, task.TargetID, task.TaskID, batch, task))
	require.NoError(t, store.removeDueBatchID(ctx, batch.BatchID))
	return batch.BatchID, task.TaskID
}

func seedFailedBatchWithPendingTasks(t *testing.T, store *v2Store) (string, []string) {
	t.Helper()
	batchID, taskID := seedBatchWithPendingTaskNotDispatched(t, store)
	return batchID, []string{taskID}
}

func seedLegacyPendingJob(t *testing.T, store *v2Store) string {
	t.Helper()
	job := &types.AsyncDownloadJob{
		TenantID:      "t-1",
		BizID:         706,
		AppID:         192,
		JobID:         "AsyncDownloadJob:706:192:/cfg/protocol.tar.gz:legacy",
		FilePath:      "/cfg",
		FileName:      "protocol.tar.gz",
		FileSignature: "sig-1",
		TargetFileDir: "/tmp",
		TargetUser:    "root",
		Targets: []*types.AsyncDownloadTarget{{
			AgentID:     "agent-a",
			ContainerID: "container-a",
		}},
		Status:             types.AsyncDownloadJobStatusPending,
		CreateTime:         time.Now().Add(-time.Minute),
		SuccessTargets:     map[string]gse.TransferFileResultDataResultContent{},
		FailedTargets:      map[string]gse.TransferFileResultDataResultContent{},
		DownloadingTargets: map[string]gse.TransferFileResultDataResultContent{},
		TimeoutTargets:     map[string]gse.TransferFileResultDataResultContent{},
	}
	payload, err := jsoni.Marshal(job)
	require.NoError(t, err)
	require.NoError(t, store.bds.Set(context.Background(), job.JobID, string(payload), 300))
	return job.JobID
}

func mustGetBatch(t *testing.T, store *v2Store, batchID string) *types.AsyncDownloadV2Batch {
	t.Helper()
	batch, err := store.getBatch(context.Background(), batchID)
	require.NoError(t, err)
	return batch
}

func mustGetTask(t *testing.T, store *v2Store, taskID string) *types.AsyncDownloadV2Task {
	t.Helper()
	task, err := store.getTask(context.Background(), taskID)
	require.NoError(t, err)
	return task
}

func mustGetLegacyJob(t *testing.T, store *v2Store, jobID string) *types.AsyncDownloadJob {
	t.Helper()
	payload, err := store.bds.Get(context.Background(), jobID)
	require.NoError(t, err)
	job := new(types.AsyncDownloadJob)
	require.NoError(t, jsoni.UnmarshalFromString(payload, job))
	return job
}

type fakeTransferClient struct {
	transferTaskID  string
	lastTransferReq *gse.TransferFileReq
	lastTransferTenantID string
	lastResultTenantID   string
	results         map[string][]gse.TransferFileResultDataResult
	resultBuilder   func(taskID string, req *gse.TransferFileReq) []gse.TransferFileResultDataResult
}

func (f *fakeTransferClient) AsyncExtensionsTransferFile(ctx context.Context,
	req *gse.TransferFileReq) (*gse.CommonTaskRespData, error) {
	f.lastTransferReq = req
	f.lastTransferTenantID = kit.FromGrpcContext(ctx).TenantID
	if f.transferTaskID == "" {
		f.transferTaskID = "gse-task-1"
	}
	if f.results == nil {
		f.results = make(map[string][]gse.TransferFileResultDataResult)
	}
	if _, ok := f.results[f.transferTaskID]; !ok {
		results := make([]gse.TransferFileResultDataResult, 0, len(req.Tasks[0].Target.Agents))
		if f.resultBuilder != nil {
			results = f.resultBuilder(f.transferTaskID, req)
		} else {
			for _, agent := range req.Tasks[0].Target.Agents {
				results = append(results, gse.TransferFileResultDataResult{
					ErrorCode: 0,
					Content: gse.TransferFileResultDataResultContent{
						DestAgentID:     agent.BkAgentID,
						DestContainerID: agent.BkContainerID,
						DestFileDir:     req.Tasks[0].Target.StoreDir,
						DestFileName:    req.Tasks[0].Target.FileName,
					},
				})
			}
		}
		f.results[f.transferTaskID] = results
	}
	return &gse.CommonTaskRespData{Result: gse.CommonTaskRespResult{TaskID: f.transferTaskID}}, nil
}

func (f *fakeTransferClient) AsyncTerminateTransferFile(context.Context,
	*gse.TerminateTransferFileTaskReq) (*gse.CommonTaskRespData, error) {
	return &gse.CommonTaskRespData{}, nil
}

func (f *fakeTransferClient) GetExtensionsTransferFileResult(ctx context.Context,
	req *gse.GetTransferFileResultReq) (*gse.TransferFileResultData, error) {
	f.lastResultTenantID = kit.FromGrpcContext(ctx).TenantID
	results := f.results[req.TaskID]
	return &gse.TransferFileResultData{Result: results}, nil
}

type fakeDownloader struct {
	content string
}

func (f fakeDownloader) Download(*kit.Kit, string) (io.ReadCloser, int64, error) {
	return io.NopCloser(strings.NewReader(f.content)), int64(len(f.content)), nil
}
