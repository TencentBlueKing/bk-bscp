package asyncdownload

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	prm "github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	v2pkg "github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/asyncdownload/v2"
	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/runtime/jsoni"
)

func TestGetAsyncDownloadTaskStatusFallsBackToV1DuringMigration(t *testing.T) {
	svc, kt := newParentIntegrationService(t)
	taskID := seedLegacyV1Task(t, svc, kt)

	status, err := svc.GetAsyncDownloadTaskStatus(kt, 706, taskID)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadJobStatusPending, status)
}

func TestLegacyV1JobStillDrainsInFirstV2Release(t *testing.T) {
	sch, store := newLegacyCompatibleScheduler(t)
	legacyJobID := seedLegacyPendingJob(t, store)

	err := sch.runOneV1DrainPass(context.Background())
	require.NoError(t, err)

	job := mustGetLegacyJob(t, store, legacyJobID)
	require.NotEqual(t, types.AsyncDownloadJobStatusPending, job.Status)
}

func newParentIntegrationService(t *testing.T) (*Service, *kit.Kit) {
	t.Helper()

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
	mc := newParentTestMetric()
	redLock := lock.NewRedisLock(bds, 5)
	return &Service{
		enabled: true,
		redis:   bds,
		redLock: redLock,
		metric:  mc,
		v2:      v2pkg.NewService(bds, redLock, mc, cfg),
	}, kit.NewWithTenant("t-1")
}

func newLegacyCompatibleScheduler(t *testing.T) (*Scheduler, *v2pkg.Store) {
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
	store := v2pkg.NewStore(bds, cfg)
	return &Scheduler{
		gseService:    &parentFakeTransferClient{transferTaskID: "gse-task-1"},
		ctx:           context.Background(),
		bds:           bds,
		redLock:       lock.NewRedisLock(bds, 5),
		fileLock:      lock.NewFileLock(),
		provider:      parentFakeDownloader{content: "demo"},
		serverAgentID: "server-agent",
		metric:        newParentTestMetric(),
		v2: v2pkg.NewScheduler(store, nil, nil, lock.NewRedisLock(bds, 5), lock.NewFileLock(), newParentTestMetric(),
			"server-agent", "server-container", "root", t.TempDir(), cfg),
	}, store
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

func seedLegacyPendingJob(t *testing.T, store *v2pkg.Store) string {
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
	require.NoError(t, store.Client().Set(context.Background(), job.JobID, string(payload), 300))
	return job.JobID
}

func mustGetLegacyJob(t *testing.T, store *v2pkg.Store, jobID string) *types.AsyncDownloadJob {
	t.Helper()
	payload, err := store.Client().Get(context.Background(), jobID)
	require.NoError(t, err)
	job := new(types.AsyncDownloadJob)
	require.NoError(t, jsoni.UnmarshalFromString(payload, job))
	return job
}

func newParentTestMetric() *metric {
	return &metric{
		jobDurationSeconds: prm.NewHistogramVec(prm.HistogramOpts{Name: "job_duration_seconds_parent_test"},
			[]string{"biz", "app", "file", "targets", "status"}),
		jobCounter: prm.NewCounterVec(prm.CounterOpts{Name: "job_count_parent_test"},
			[]string{"biz", "app", "file", "targets", "status"}),
		taskDurationSeconds: prm.NewHistogramVec(prm.HistogramOpts{Name: "task_duration_seconds_parent_test"},
			[]string{"biz", "app", "file", "status"}),
		taskCounter: prm.NewCounterVec(prm.CounterOpts{Name: "task_count_parent_test"},
			[]string{"biz", "app", "file", "status"}),
		sourceFilesSizeBytes: prm.NewGauge(prm.GaugeOpts{Name: "source_files_size_bytes_parent_test"}),
		sourceFilesCounter:   prm.NewGauge(prm.GaugeOpts{Name: "source_files_count_parent_test"}),
		batchDueBacklog:      prm.NewGauge(prm.GaugeOpts{Name: "batch_due_backlog_parent_test"}),
		batchOldestDueAgeSeconds: prm.NewGauge(prm.GaugeOpts{
			Name: "batch_oldest_due_age_seconds_parent_test",
		}),
		v2BatchStateCounter: prm.NewCounterVec(prm.CounterOpts{Name: "v2_batch_state_count_parent_test"},
			[]string{"biz", "app", "state"}),
		v2BatchStateDurationSeconds: prm.NewHistogramVec(
			prm.HistogramOpts{Name: "v2_batch_state_duration_seconds_parent_test"},
			[]string{"biz", "app", "state"}),
		v2TaskStateCounter: prm.NewCounterVec(prm.CounterOpts{Name: "v2_task_state_count_parent_test"},
			[]string{"biz", "app", "state"}),
		v2TaskStateDurationSeconds: prm.NewHistogramVec(
			prm.HistogramOpts{Name: "v2_task_state_duration_seconds_parent_test"},
			[]string{"biz", "app", "state"}),
		taskRepairCounter: prm.NewCounterVec(prm.CounterOpts{Name: "task_repair_count_parent_test"},
			[]string{"reason"}),
		shardDispatchCounter: prm.NewCounterVec(prm.CounterOpts{Name: "shard_dispatch_count_parent_test"},
			[]string{"status"}),
		shardDurationSeconds: prm.NewHistogramVec(prm.HistogramOpts{Name: "shard_duration_seconds_parent_test"},
			[]string{"status"}),
	}
}

type parentFakeTransferClient struct {
	transferTaskID string
}

func (f *parentFakeTransferClient) AsyncExtensionsTransferFile(context.Context,
	*gse.TransferFileReq) (*gse.CommonTaskRespData, error) {
	taskID := f.transferTaskID
	if taskID == "" {
		taskID = "gse-task-1"
	}
	return &gse.CommonTaskRespData{Result: gse.CommonTaskRespResult{TaskID: taskID}}, nil
}

func (f *parentFakeTransferClient) AsyncTerminateTransferFile(context.Context,
	*gse.TerminateTransferFileTaskReq) (*gse.CommonTaskRespData, error) {
	return &gse.CommonTaskRespData{}, nil
}

func (f *parentFakeTransferClient) GetExtensionsTransferFileResult(context.Context,
	*gse.GetTransferFileResultReq) (*gse.TransferFileResultData, error) {
	return &gse.TransferFileResultData{}, nil
}

type parentFakeDownloader struct {
	content string
}

func (f parentFakeDownloader) Download(*kit.Kit, string) (io.ReadCloser, int64, error) {
	return io.NopCloser(strings.NewReader(f.content)), int64(len(f.content)), nil
}
