package asyncdownload

import (
	"context"
	"testing"
	"time"

	prm "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/runtime/jsoni"
)

func TestAsyncDownloadV2ConfigDefaults(t *testing.T) {
	g := cc.GSE{}
	g.TrySetDefaultForTest()

	require.False(t, g.AsyncDownloadV2.Enabled)
	require.Equal(t, 10, g.AsyncDownloadV2.CollectWindowSeconds)
	require.Equal(t, 5000, g.AsyncDownloadV2.MaxTargetsPerBatch)
	require.Equal(t, 500, g.AsyncDownloadV2.ShardSize)
	require.Equal(t, 15, g.AsyncDownloadV2.DispatchHeartbeatSeconds)
	require.Equal(t, 60, g.AsyncDownloadV2.DispatchLeaseSeconds)
	require.Equal(t, 3, g.AsyncDownloadV2.MaxDispatchAttempts)
	require.Equal(t, 100, g.AsyncDownloadV2.MaxDueBatchesPerTick)
	require.Equal(t, 86400, g.AsyncDownloadV2.TaskTTLSeconds)
	require.Equal(t, 86400, g.AsyncDownloadV2.BatchTTLSeconds)
}

func TestAsyncDownloadV2ServiceEnabledIgnoresConfigFlag(t *testing.T) {
	svc := &v2Service{cfg: cc.AsyncDownloadV2{Enabled: false}}
	require.True(t, svc.enabled())
}

func TestCreateAsyncDownloadTaskV2ReusesInflightTask(t *testing.T) {
	svc, kt := newTestAsyncDownloadService(t)

	firstID, err := svc.CreateAsyncDownloadTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "root", "/tmp", "sig-1")
	require.NoError(t, err)

	secondID, err := svc.CreateAsyncDownloadTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "root", "/tmp", "sig-1")
	require.NoError(t, err)

	require.Equal(t, firstID, secondID)
}

func TestCreateAsyncDownloadTaskV2DoesNotReuseInflightTaskAcrossDestinations(t *testing.T) {
	svc, kt := newTestAsyncDownloadService(t)

	firstID, err := svc.CreateAsyncDownloadTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "root", "/tmp/releases-a", "sig-1")
	require.NoError(t, err)

	secondID, err := svc.CreateAsyncDownloadTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "root", "/tmp/releases-b", "sig-1")
	require.NoError(t, err)

	require.NotEqual(t, firstID, secondID)

	firstTask, err := svc.v2.store.getTask(kt.Ctx, firstID)
	require.NoError(t, err)
	secondTask, err := svc.v2.store.getTask(kt.Ctx, secondID)
	require.NoError(t, err)
	require.NotEqual(t, firstTask.BatchID, secondTask.BatchID)
	require.Equal(t, "/tmp/releases-a", firstTask.TargetFileDir)
	require.Equal(t, "/tmp/releases-b", secondTask.TargetFileDir)
}

func TestCreateAsyncDownloadTaskV2RecordsLifecycleMetrics(t *testing.T) {
	svc, kt := newTestAsyncDownloadService(t)

	taskID, err := svc.CreateAsyncDownloadTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "root", "/tmp", "sig-1")
	require.NoError(t, err)
	require.NotEmpty(t, taskID)

	require.Equal(t, float64(1), testutil.ToFloat64(
		svc.metric.v2BatchStateCounter.WithLabelValues("706", "192", types.AsyncDownloadBatchStateCollecting)))
	require.Equal(t, float64(1), testutil.ToFloat64(
		svc.metric.v2TaskStateCounter.WithLabelValues("706", "192", types.AsyncDownloadJobStatusPending)))
}

func TestGetAsyncDownloadTaskStatusFallsBackToV1DuringMigration(t *testing.T) {
	svc, kt := newTestAsyncDownloadService(t)
	taskID := seedLegacyV1Task(t, svc, kt)

	status, err := svc.GetAsyncDownloadTaskStatus(kt, 706, taskID)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadJobStatusPending, status)
}

func TestCreateAsyncDownloadTaskV2PersistsTargetInfo(t *testing.T) {
	svc, kt := newTestAsyncDownloadService(t)

	taskID, err := svc.CreateAsyncDownloadTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)

	task, err := svc.v2.store.getTask(kt.Ctx, taskID)
	require.NoError(t, err)
	require.Equal(t, "tester", task.TargetUser)
	require.Equal(t, "/data/releases", task.TargetFileDir)

	batch, err := svc.v2.store.getBatch(kt.Ctx, task.BatchID)
	require.NoError(t, err)
	require.Equal(t, "tester", batch.TargetUser)
	require.Equal(t, "/data/releases", batch.TargetFileDir)
}

func TestAsyncDownloadV2CreateStatusAndDrainCompatibility(t *testing.T) {
	svc, sch, kt := newIntegratedV2TestHarness(t)

	taskID, err := svc.CreateAsyncDownloadTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "root", "/tmp", "sig-1")
	require.NoError(t, err)

	task, err := svc.v2.store.getTask(kt.Ctx, taskID)
	require.NoError(t, err)
	batch, err := sch.store.getBatch(kt.Ctx, task.BatchID)
	require.NoError(t, err)
	batch.OpenUntil = time.Now().Add(-time.Second)
	require.NoError(t, sch.store.saveBatch(kt.Ctx, batch))

	processed, err := sch.processDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	status, err := svc.GetAsyncDownloadTaskStatus(kt, 706, taskID)
	require.NoError(t, err)
	require.Contains(t, []string{
		types.AsyncDownloadJobStatusRunning,
		types.AsyncDownloadJobStatusSuccess,
	}, status)
}

func TestAsyncDownloadV2CreateStatusWithSimulatedGSEDownload(t *testing.T) {
	svc, sch, kt := newIntegratedV2TestHarness(t)

	taskID, err := svc.CreateAsyncDownloadTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)

	task, err := svc.v2.store.getTask(kt.Ctx, taskID)
	require.NoError(t, err)
	batch, err := sch.store.getBatch(kt.Ctx, task.BatchID)
	require.NoError(t, err)
	batch.OpenUntil = time.Now().Add(-time.Second)
	require.NoError(t, sch.store.saveBatch(kt.Ctx, batch))

	processed, err := sch.processDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	gseClient, ok := sch.gseService.(*fakeTransferClient)
	require.True(t, ok)
	require.NotNil(t, gseClient.lastTransferReq)
	require.Equal(t, "/data/releases", gseClient.lastTransferReq.Tasks[0].Target.StoreDir)
	require.Equal(t, "tester", gseClient.lastTransferReq.Tasks[0].Target.Agents[0].User)

	status, err := svc.GetAsyncDownloadTaskStatus(kt, 706, taskID)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadJobStatusSuccess, status)
}

func TestAsyncDownloadV2CreateStatusWithSimulatedGSEFailure(t *testing.T) {
	svc, sch, kt := newIntegratedV2TestHarness(t)
	gseClient := mustGetFakeTransferClient(t, sch)
	gseClient.resultBuilder = func(_ string, req *gse.TransferFileReq) []gse.TransferFileResultDataResult {
		return []gse.TransferFileResultDataResult{{
			ErrorCode: 42,
			ErrorMsg:  "disk full",
			Content: gse.TransferFileResultDataResultContent{
				DestAgentID:     req.Tasks[0].Target.Agents[0].BkAgentID,
				DestContainerID: req.Tasks[0].Target.Agents[0].BkContainerID,
				DestFileDir:     req.Tasks[0].Target.StoreDir,
				DestFileName:    req.Tasks[0].Target.FileName,
			},
		}}
	}

	taskID, err := svc.CreateAsyncDownloadTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)
	forceTaskBatchDue(t, svc, sch, kt, taskID)

	processed, err := sch.processDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	status, err := svc.GetAsyncDownloadTaskStatus(kt, 706, taskID)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadJobStatusFailed, status)
}

func TestAsyncDownloadV2CreateStatusWithSimulatedGSEUploadFailure(t *testing.T) {
	svc, sch, kt := newIntegratedV2TestHarness(t)
	gseClient := mustGetFakeTransferClient(t, sch)
	gseClient.resultBuilder = func(_ string, req *gse.TransferFileReq) []gse.TransferFileResultDataResult {
		return []gse.TransferFileResultDataResult{{
			ErrorCode: 42,
			ErrorMsg:  "source upload failed",
			Content: gse.TransferFileResultDataResultContent{
				Type:              "upload",
				SourceAgentID:     req.Tasks[0].Source.Agent.BkAgentID,
				SourceContainerID: req.Tasks[0].Source.Agent.BkContainerID,
				SourceFileDir:     req.Tasks[0].Source.StoreDir,
				SourceFileName:    req.Tasks[0].Source.FileName,
			},
		}}
	}

	taskID, err := svc.CreateAsyncDownloadTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)
	forceTaskBatchDue(t, svc, sch, kt, taskID)

	processed, err := sch.processDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	status, err := svc.GetAsyncDownloadTaskStatus(kt, 706, taskID)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadJobStatusFailed, status)
}

func TestAsyncDownloadV2CreateStatusWithSimulatedGSEPartial(t *testing.T) {
	svc, sch, kt := newIntegratedV2TestHarness(t)
	gseClient := mustGetFakeTransferClient(t, sch)
	gseClient.resultBuilder = func(_ string, req *gse.TransferFileReq) []gse.TransferFileResultDataResult {
		results := make([]gse.TransferFileResultDataResult, 0, len(req.Tasks[0].Target.Agents))
		for i, agent := range req.Tasks[0].Target.Agents {
			result := gse.TransferFileResultDataResult{
				ErrorCode: 0,
				Content: gse.TransferFileResultDataResultContent{
					DestAgentID:     agent.BkAgentID,
					DestContainerID: agent.BkContainerID,
					DestFileDir:     req.Tasks[0].Target.StoreDir,
					DestFileName:    req.Tasks[0].Target.FileName,
				},
			}
			if i == 1 {
				result.ErrorCode = 42
				result.ErrorMsg = "permission denied"
			}
			results = append(results, result)
		}
		return results
	}

	taskID1, err := svc.CreateAsyncDownloadTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)
	taskID2, err := svc.CreateAsyncDownloadTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-b", "container-b", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)
	forceTaskBatchDue(t, svc, sch, kt, taskID1)

	processed, err := sch.processDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	status1, err := svc.GetAsyncDownloadTaskStatus(kt, 706, taskID1)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadJobStatusSuccess, status1)

	status2, err := svc.GetAsyncDownloadTaskStatus(kt, 706, taskID2)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadJobStatusFailed, status2)

	task, err := svc.v2.store.getTask(kt.Ctx, taskID1)
	require.NoError(t, err)
	batch, err := sch.store.getBatch(kt.Ctx, task.BatchID)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadBatchStatePartial, batch.State)
}

func TestAsyncDownloadV2RunningTaskRepairsAfterDispatchLeaseTimeout(t *testing.T) {
	svc, sch, kt := newIntegratedV2TestHarness(t)
	gseClient := mustGetFakeTransferClient(t, sch)
	gseClient.resultBuilder = func(_ string, req *gse.TransferFileReq) []gse.TransferFileResultDataResult {
		return []gse.TransferFileResultDataResult{{
			ErrorCode: 115,
			Content: gse.TransferFileResultDataResultContent{
				DestAgentID:     req.Tasks[0].Target.Agents[0].BkAgentID,
				DestContainerID: req.Tasks[0].Target.Agents[0].BkContainerID,
				DestFileDir:     req.Tasks[0].Target.StoreDir,
				DestFileName:    req.Tasks[0].Target.FileName,
			},
		}}
	}

	taskID, err := svc.CreateAsyncDownloadTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)
	forceTaskBatchDue(t, svc, sch, kt, taskID)

	processed, err := sch.processDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	status, err := svc.GetAsyncDownloadTaskStatus(kt, 706, taskID)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadJobStatusRunning, status)

	task, err := svc.v2.store.getTask(kt.Ctx, taskID)
	require.NoError(t, err)
	batch, err := sch.store.getBatch(kt.Ctx, task.BatchID)
	require.NoError(t, err)
	batch.DispatchLeaseUntil = time.Now().Add(-time.Second)
	require.NoError(t, sch.store.saveBatch(kt.Ctx, batch))

	processed, err = sch.processDueBatches(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, processed, 0)

	status, err = svc.GetAsyncDownloadTaskStatus(kt, 706, taskID)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadJobStatusFailed, status)

	task, err = svc.v2.store.getTask(kt.Ctx, taskID)
	require.NoError(t, err)
	require.Equal(t, "orphan_after_dispatch_cutoff", task.ErrMsg)
}

func newTestAsyncDownloadService(t *testing.T) (*Service, *kit.Kit) {
	t.Helper()

	svc, _, kt := newIntegratedV2TestHarness(t)
	return svc, kt
}

func forceTaskBatchDue(t *testing.T, svc *Service, sch *v2Scheduler, kt *kit.Kit, taskID string) {
	t.Helper()
	task, err := svc.v2.store.getTask(kt.Ctx, taskID)
	require.NoError(t, err)
	batch, err := sch.store.getBatch(kt.Ctx, task.BatchID)
	require.NoError(t, err)
	batch.OpenUntil = time.Now().Add(-time.Second)
	require.NoError(t, sch.store.saveBatch(kt.Ctx, batch))
}

func mustGetFakeTransferClient(t *testing.T, sch *v2Scheduler) *fakeTransferClient {
	t.Helper()
	gseClient, ok := sch.gseService.(*fakeTransferClient)
	require.True(t, ok)
	return gseClient
}

func newIntegratedV2TestHarness(t *testing.T) (*Service, *v2Scheduler, *kit.Kit) {
	t.Helper()

	mr := miniredis.RunT(t)
	opt := cc.RedisCluster{Mode: cc.RedisStandaloneMode, Endpoints: []string{mr.Addr()}}
	bds, err := bedis.NewRedisCache(opt)
	require.NoError(t, err)

	cfg := cc.AsyncDownloadV2{
		Enabled:                  true,
		CollectWindowSeconds:     1,
		MaxTargetsPerBatch:       5000,
		ShardSize:                500,
		DispatchHeartbeatSeconds: 15,
		DispatchLeaseSeconds:     60,
		MaxDispatchAttempts:      3,
		MaxDueBatchesPerTick:     100,
		TaskTTLSeconds:           86400,
		BatchTTLSeconds:          86400,
	}
	mc := newTestMetric()
	redLock := lock.NewRedisLock(bds, 5)
	gseClient := &fakeTransferClient{}
	svc := &Service{
		enabled: true,
		redis:   bds,
		redLock: redLock,
		metric:  mc,
		v2:      newV2Service(bds, redLock, mc, cfg),
	}
	sch := newV2Scheduler(newV2Store(bds, cfg), gseClient, fakeDownloader{content: "demo"}, redLock, lock.NewFileLock(), mc,
		"server-agent", "server-container", "root", t.TempDir(), cfg)
	kt := kit.NewWithTenant("t-1")
	return svc, sch, kt
}

func seedLegacyV1Task(t *testing.T, svc *Service, kt *kit.Kit) string {
	t.Helper()

	taskID := "AsyncDownloadTask:706:legacy"
	jobID := "AsyncDownloadJob:706:192:/cfg/protocol.tar.gz:legacy"
	task := &types.AsyncDownloadTask{
		BizID:             706,
		AppID:             192,
		JobID:             jobID,
		TargetAgentID:     "agent-a",
		TargetContainerID: "container-a",
		FilePath:          "/cfg",
		FileName:          "protocol.tar.gz",
		FileSignature:     "sig-1",
		Status:            types.AsyncDownloadJobStatusPending,
		CreateTime:        time.Now(),
	}
	job := &types.AsyncDownloadJob{
		TenantID:           kt.TenantID,
		BizID:              706,
		AppID:              192,
		JobID:              jobID,
		FilePath:           "/cfg",
		FileName:           "protocol.tar.gz",
		FileSignature:      "sig-1",
		Status:             types.AsyncDownloadJobStatusPending,
		CreateTime:         time.Now(),
		SuccessTargets:     map[string]gse.TransferFileResultDataResultContent{},
		FailedTargets:      map[string]gse.TransferFileResultDataResultContent{},
		DownloadingTargets: map[string]gse.TransferFileResultDataResultContent{},
		TimeoutTargets:     map[string]gse.TransferFileResultDataResultContent{},
	}

	taskData, err := jsoni.Marshal(task)
	require.NoError(t, err)
	jobData, err := jsoni.Marshal(job)
	require.NoError(t, err)
	require.NoError(t, svc.redis.Set(kt.Ctx, taskID, string(taskData), 300))
	require.NoError(t, svc.redis.Set(kt.Ctx, jobID, string(jobData), 300))
	return taskID
}

func newTestMetric() *metric {
	return &metric{
		jobDurationSeconds: prm.NewHistogramVec(prm.HistogramOpts{Name: "job_duration_seconds_test"},
			[]string{"biz", "app", "file", "targets", "status"}),
		jobCounter: prm.NewCounterVec(prm.CounterOpts{Name: "job_count_test"},
			[]string{"biz", "app", "file", "targets", "status"}),
		taskDurationSeconds: prm.NewHistogramVec(prm.HistogramOpts{Name: "task_duration_seconds_test"},
			[]string{"biz", "app", "file", "status"}),
		taskCounter: prm.NewCounterVec(prm.CounterOpts{Name: "task_count_test"},
			[]string{"biz", "app", "file", "status"}),
		sourceFilesSizeBytes: prm.NewGauge(prm.GaugeOpts{Name: "source_files_size_bytes_test"}),
			sourceFilesCounter:   prm.NewGauge(prm.GaugeOpts{Name: "source_files_count_test"}),
			batchDueBacklog:      prm.NewGauge(prm.GaugeOpts{Name: "batch_due_backlog_test"}),
			batchOldestDueAgeSeconds: prm.NewGauge(prm.GaugeOpts{
				Name: "batch_oldest_due_age_seconds_test",
			}),
			v2BatchStateCounter: prm.NewCounterVec(prm.CounterOpts{Name: "v2_batch_state_count_test"},
				[]string{"biz", "app", "state"}),
			v2BatchStateDurationSeconds: prm.NewHistogramVec(
				prm.HistogramOpts{Name: "v2_batch_state_duration_seconds_test"},
				[]string{"biz", "app", "state"}),
			v2TaskStateCounter: prm.NewCounterVec(prm.CounterOpts{Name: "v2_task_state_count_test"},
				[]string{"biz", "app", "state"}),
			v2TaskStateDurationSeconds: prm.NewHistogramVec(
				prm.HistogramOpts{Name: "v2_task_state_duration_seconds_test"},
				[]string{"biz", "app", "state"}),
			taskRepairCounter: prm.NewCounterVec(prm.CounterOpts{Name: "task_repair_count_test"},
				[]string{"reason"}),
			shardDispatchCounter: prm.NewCounterVec(prm.CounterOpts{Name: "shard_dispatch_count_test"},
			[]string{"status"}),
		shardDurationSeconds: prm.NewHistogramVec(prm.HistogramOpts{Name: "shard_duration_seconds_test"},
			[]string{"status"}),
	}
}
