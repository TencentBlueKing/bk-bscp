package v2

import (
	"context"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	prm "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

type testMetrics struct {
	batchDueBacklog             prm.Gauge
	batchOldestDueAgeSeconds    prm.Gauge
	v2BatchStateCounter         *prm.CounterVec
	v2BatchStateDurationSeconds *prm.HistogramVec
	v2TaskStateCounter          *prm.CounterVec
	v2TaskStateDurationSeconds  *prm.HistogramVec
	taskRepairCounter           *prm.CounterVec
	shardDispatchCounter        *prm.CounterVec
	shardDurationSeconds        *prm.HistogramVec
}

func newTestMetrics() *testMetrics {
	return &testMetrics{
		batchDueBacklog: prm.NewGauge(prm.GaugeOpts{Name: "batch_due_backlog_test"}),
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

func (m *testMetrics) ObserveV2BatchCreated(batch *types.AsyncDownloadV2Batch) {
	if batch == nil {
		return
	}
	m.v2BatchStateCounter.WithLabelValues(intToString(batch.BizID), intToString(batch.AppID), batch.State).Inc()
}

func (m *testMetrics) ObserveV2BatchTransition(batch *types.AsyncDownloadV2Batch, oldState string) {
	if batch == nil || oldState == "" || oldState == batch.State {
		return
	}
	m.v2BatchStateDurationSeconds.WithLabelValues(intToString(batch.BizID), intToString(batch.AppID), oldState).
		Observe(1)
	m.v2BatchStateCounter.WithLabelValues(intToString(batch.BizID), intToString(batch.AppID), batch.State).Inc()
}

func (m *testMetrics) ObserveV2TaskCreated(task *types.AsyncDownloadV2Task) {
	if task == nil {
		return
	}
	m.v2TaskStateCounter.WithLabelValues(intToString(task.BizID), intToString(task.AppID), task.State).Inc()
}

func (m *testMetrics) ObserveV2TaskTransition(task *types.AsyncDownloadV2Task, oldState string, _ time.Time) {
	if task == nil || oldState == "" || oldState == task.State {
		return
	}
	m.v2TaskStateDurationSeconds.WithLabelValues(intToString(task.BizID), intToString(task.AppID), oldState).
		Observe(1)
	m.v2TaskStateCounter.WithLabelValues(intToString(task.BizID), intToString(task.AppID), task.State).Inc()
}

func (m *testMetrics) SetV2DueBacklog(count int) {
	m.batchDueBacklog.Set(float64(count))
}

func (m *testMetrics) SetV2OldestDueAgeSeconds(age float64) {
	m.batchOldestDueAgeSeconds.Set(age)
}

func (m *testMetrics) IncV2TaskRepair(reason string) {
	m.taskRepairCounter.WithLabelValues(reason).Inc()
}

func (m *testMetrics) ObserveV2ShardDispatch(status string, duration time.Duration) {
	m.shardDispatchCounter.WithLabelValues(status).Inc()
	m.shardDurationSeconds.WithLabelValues(status).Observe(duration.Seconds())
}

func histogramSampleCount(t *testing.T, collector interface{}) uint64 {
	t.Helper()
	metric, ok := collector.(interface{ Write(*dto.Metric) error })
	require.True(t, ok)
	dtoMetric := &dto.Metric{}
	require.NoError(t, metric.Write(dtoMetric))
	return dtoMetric.GetHistogram().GetSampleCount()
}

func mustGetFakeTransferClient(t *testing.T, sch *Scheduler) *fakeTransferClient {
	t.Helper()
	gseClient, ok := sch.GSEService().(*fakeTransferClient)
	require.True(t, ok)
	return gseClient
}

type fakeTransferClient struct {
	transferTaskID       string
	lastTransferReq      *gse.TransferFileReq
	lastTransferTenantID string
	lastResultTenantID   string
	results              map[string][]gse.TransferFileResultDataResult
	resultBuilder        func(taskID string, req *gse.TransferFileReq) []gse.TransferFileResultDataResult
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

func intToString(v uint32) string {
	return strconv.Itoa(int(v))
}
