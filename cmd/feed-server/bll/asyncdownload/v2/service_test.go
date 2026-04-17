package v2

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

func TestConfigDefaults(t *testing.T) {
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

func TestServiceEnabledIgnoresConfigFlag(t *testing.T) {
	svc := &Service{}
	require.True(t, svc.Enabled())
}

func TestCreateTaskReusesInflightTask(t *testing.T) {
	svc, kt := newTestService(t)

	firstID, err := svc.CreateTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "root", "/tmp", "sig-1")
	require.NoError(t, err)

	secondID, err := svc.CreateTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "root", "/tmp", "sig-1")
	require.NoError(t, err)

	require.Equal(t, firstID, secondID)
}

func TestCreateTaskDoesNotReuseInflightTaskAcrossDestinations(t *testing.T) {
	svc, kt := newTestService(t)

	firstID, err := svc.CreateTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "root", "/tmp/releases-a", "sig-1")
	require.NoError(t, err)

	secondID, err := svc.CreateTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "root", "/tmp/releases-b", "sig-1")
	require.NoError(t, err)

	require.NotEqual(t, firstID, secondID)

	firstTask, err := svc.Store().GetTask(kt.Ctx, firstID)
	require.NoError(t, err)
	secondTask, err := svc.Store().GetTask(kt.Ctx, secondID)
	require.NoError(t, err)
	require.NotEqual(t, firstTask.BatchID, secondTask.BatchID)
	require.Equal(t, "/tmp/releases-a", firstTask.TargetFileDir)
	require.Equal(t, "/tmp/releases-b", secondTask.TargetFileDir)
}

func TestCreateTaskRecordsLifecycleMetrics(t *testing.T) {
	svc, kt := newTestService(t)

	taskID, err := svc.CreateTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "root", "/tmp", "sig-1")
	require.NoError(t, err)
	require.NotEmpty(t, taskID)

	metrics := svc.Metrics().(*testMetrics)
	require.Equal(t, float64(1), testutil.ToFloat64(
		metrics.v2BatchStateCounter.WithLabelValues("706", "192", types.AsyncDownloadBatchStateCollecting)))
	require.Equal(t, float64(1), testutil.ToFloat64(
		metrics.v2TaskStateCounter.WithLabelValues("706", "192", types.AsyncDownloadJobStatusPending)))
}

func TestCreateTaskPersistsTargetInfo(t *testing.T) {
	svc, kt := newTestService(t)

	taskID, err := svc.CreateTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)

	task, err := svc.Store().GetTask(kt.Ctx, taskID)
	require.NoError(t, err)
	require.Equal(t, "tester", task.TargetUser)
	require.Equal(t, "/data/releases", task.TargetFileDir)

	batch, err := svc.Store().GetBatch(kt.Ctx, task.BatchID)
	require.NoError(t, err)
	require.Equal(t, "tester", batch.TargetUser)
	require.Equal(t, "/data/releases", batch.TargetFileDir)
}

func TestCreateStatusAndDrainCompatibility(t *testing.T) {
	svc, sch, kt := newIntegratedTestHarness(t)

	taskID, err := svc.CreateTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "root", "/tmp", "sig-1")
	require.NoError(t, err)
	forceTaskBatchDue(t, svc, sch, kt, taskID)

	processed, err := sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	status, err := svc.GetTaskStatus(kt.Ctx, taskID)
	require.NoError(t, err)
	require.Contains(t, []string{
		types.AsyncDownloadJobStatusRunning,
		types.AsyncDownloadJobStatusSuccess,
	}, status)
}

func TestCreateStatusWithSimulatedGSEDownload(t *testing.T) {
	svc, sch, kt := newIntegratedTestHarness(t)

	taskID, err := svc.CreateTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)
	forceTaskBatchDue(t, svc, sch, kt, taskID)

	processed, err := sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	gseClient := mustGetFakeTransferClient(t, sch)
	require.NotNil(t, gseClient.lastTransferReq)
	require.Equal(t, "/data/releases", gseClient.lastTransferReq.Tasks[0].Target.StoreDir)
	require.Equal(t, "tester", gseClient.lastTransferReq.Tasks[0].Target.Agents[0].User)

	status, err := svc.GetTaskStatus(kt.Ctx, taskID)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadJobStatusSuccess, status)
}

func TestCreateStatusWithSimulatedGSEFailure(t *testing.T) {
	svc, sch, kt := newIntegratedTestHarness(t)
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

	taskID, err := svc.CreateTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)
	forceTaskBatchDue(t, svc, sch, kt, taskID)

	processed, err := sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	status, err := svc.GetTaskStatus(kt.Ctx, taskID)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadJobStatusFailed, status)
}

func TestCreateStatusWithSimulatedGSEUploadFailure(t *testing.T) {
	svc, sch, kt := newIntegratedTestHarness(t)
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

	taskID, err := svc.CreateTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)
	forceTaskBatchDue(t, svc, sch, kt, taskID)

	processed, err := sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	status, err := svc.GetTaskStatus(kt.Ctx, taskID)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadJobStatusFailed, status)
}

func TestCreateStatusWithSimulatedGSEPartial(t *testing.T) {
	svc, sch, kt := newIntegratedTestHarness(t)
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

	taskID1, err := svc.CreateTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)
	taskID2, err := svc.CreateTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-b", "container-b", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)
	forceTaskBatchDue(t, svc, sch, kt, taskID1)

	processed, err := sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	status1, err := svc.GetTaskStatus(kt.Ctx, taskID1)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadJobStatusSuccess, status1)

	status2, err := svc.GetTaskStatus(kt.Ctx, taskID2)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadJobStatusFailed, status2)

	task, err := svc.Store().GetTask(kt.Ctx, taskID1)
	require.NoError(t, err)
	batch, err := sch.Store().GetBatch(kt.Ctx, task.BatchID)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadBatchStatePartial, batch.State)
}

func TestRepeatedRunningResultExtendsLeaseWhenGSEHeartbeatIsObserved(t *testing.T) {
	svc, sch, kt := newIntegratedTestHarness(t)
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

	taskID, err := svc.CreateTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)
	forceTaskBatchDue(t, svc, sch, kt, taskID)

	processed, err := sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	task, err := svc.Store().GetTask(kt.Ctx, taskID)
	require.NoError(t, err)
	batch, err := sch.Store().GetBatch(kt.Ctx, task.BatchID)
	require.NoError(t, err)

	shortLeaseUntil := time.Now().Add(120 * time.Millisecond)
	batch.DispatchLeaseUntil = shortLeaseUntil
	require.NoError(t, sch.Store().SaveBatch(kt.Ctx, batch))

	time.Sleep(40 * time.Millisecond)

	processed, err = sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, processed, 0)

	batch, err = sch.Store().GetBatch(kt.Ctx, task.BatchID)
	require.NoError(t, err)
	require.True(t, batch.DispatchLeaseUntil.After(shortLeaseUntil))
	require.True(t, batch.DispatchHeartbeatAt.After(shortLeaseUntil.Add(-120*time.Millisecond)))

	time.Sleep(time.Until(shortLeaseUntil) + 40*time.Millisecond)

	status, err := svc.GetTaskStatus(kt.Ctx, taskID)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadJobStatusRunning, status)

	require.Empty(t, gseClient.terminateReqs)
}

func TestRunningTaskTimesOutAfterAbsoluteDispatchTimeout(t *testing.T) {
	svc, sch, kt := newIntegratedTestHarness(t)
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

	taskID, err := svc.CreateTask(kt, 706, 192, "/cfg", "protocol.tar.gz",
		"agent-a", "container-a", "tester", "/data/releases", "sig-1")
	require.NoError(t, err)
	forceTaskBatchDue(t, svc, sch, kt, taskID)

	processed, err := sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	task, err := svc.Store().GetTask(kt.Ctx, taskID)
	require.NoError(t, err)
	batch, err := sch.Store().GetBatch(kt.Ctx, task.BatchID)
	require.NoError(t, err)

	batch.DispatchStartedAt = time.Now().Add(-21 * time.Minute)
	batch.DispatchLeaseUntil = time.Now().Add(time.Minute)
	require.NoError(t, sch.Store().SaveBatch(kt.Ctx, batch))

	processed, err = sch.ProcessDueBatches(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, processed, 0)

	status, err := svc.GetTaskStatus(kt.Ctx, taskID)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadJobStatusTimeout, status)

	task, err = svc.Store().GetTask(kt.Ctx, taskID)
	require.NoError(t, err)
	require.Equal(t, "dispatch_timeout", task.ErrMsg)

	batch, err = sch.Store().GetBatch(kt.Ctx, task.BatchID)
	require.NoError(t, err)
	require.Equal(t, types.AsyncDownloadBatchStateFailed, batch.State)
	require.Equal(t, "dispatch_timeout", batch.FinalReason)

	require.Len(t, gseClient.terminateReqs, 1)
	require.Equal(t, "gse-task-1", gseClient.terminateReqs[0].TaskID)
}

func newTestService(t *testing.T) (*Service, *kit.Kit) {
	t.Helper()
	svc, _, kt := newIntegratedTestHarness(t)
	return svc, kt
}

func newIntegratedTestHarness(t *testing.T) (*Service, *Scheduler, *kit.Kit) {
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
	mc := newTestMetrics()
	redLock := lock.NewRedisLock(bds, 5)
	gseClient := &fakeTransferClient{}
	svc := NewService(bds, redLock, mc, cfg)
	sch := NewScheduler(NewStore(bds, cfg), gseClient, fakeDownloader{content: "demo"}, redLock, lock.NewFileLock(), mc,
		"server-agent", "server-container", "root", t.TempDir(), cfg)
	kt := kit.NewWithTenant("t-1")
	return svc, sch, kt
}

func forceTaskBatchDue(t *testing.T, svc *Service, sch *Scheduler, kt *kit.Kit, taskID string) {
	t.Helper()
	task, err := svc.Store().GetTask(kt.Ctx, taskID)
	require.NoError(t, err)
	batch, err := sch.Store().GetBatch(kt.Ctx, task.BatchID)
	require.NoError(t, err)
	batch.OpenUntil = time.Now().Add(-time.Second)
	require.NoError(t, sch.Store().SaveBatch(kt.Ctx, batch))
}
