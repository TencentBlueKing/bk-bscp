/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package asyncdownload

import (
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/pkg/metrics"
)

var (
	asyncDownloadMetricOnce sync.Once
	asyncDownloadMetricInst *metric
)

// InitMetric init the async doenload related prometheus metrics
func InitMetric() *metric {
	asyncDownloadMetricOnce.Do(func() {
		m := new(metric)
		labels := prometheus.Labels{}

		m.jobDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace:   metrics.Namespace,
			Subsystem:   metrics.AsyncDownload,
			Name:        "job_duration_seconds",
			Help:        "the duration(seconds) to precess async download job",
			ConstLabels: labels,
			Buckets:     []float64{1, 2, 4, 6, 10, 15, 30, 50, 100, 150, 200, 400, 600},
		}, []string{"biz", "app", "file", "targets", "status"})
		metrics.Register().MustRegister(m.jobDurationSeconds)

		m.jobCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   metrics.Namespace,
			Subsystem:   metrics.AsyncDownload,
			Name:        "job_count",
			Help:        "the count of the async download job",
			ConstLabels: labels,
		}, []string{"biz", "app", "file", "targets", "status"})
		metrics.Register().MustRegister(m.jobCounter)

		m.taskDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace:   metrics.Namespace,
			Subsystem:   metrics.AsyncDownload,
			Name:        "task_duration_seconds",
			Help:        "the duration(seconds) to precess async download task",
			ConstLabels: labels,
			Buckets:     []float64{1, 2, 4, 6, 10, 15, 30, 50, 100, 150, 200, 400, 600},
		}, []string{"biz", "app", "file", "status"})
		metrics.Register().MustRegister(m.taskDurationSeconds)

		m.taskCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   metrics.Namespace,
			Subsystem:   metrics.AsyncDownload,
			Name:        "task_count",
			Help:        "the count of the async download task",
			ConstLabels: labels,
		}, []string{"biz", "app", "file", "status"})
		metrics.Register().MustRegister(m.taskCounter)

		m.sourceFilesSizeBytes = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   metrics.Namespace,
			Subsystem:   metrics.AsyncDownload,
			Name:        "source_files_size_bytes",
			Help:        "the size of the source files cache size in bytes",
			ConstLabels: labels,
		})
		metrics.Register().MustRegister(m.sourceFilesSizeBytes)

		m.sourceFilesCounter = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   metrics.Namespace,
			Subsystem:   metrics.AsyncDownload,
			Name:        "source_files_count",
			Help:        "the count of the source files count",
			ConstLabels: labels,
		})
		metrics.Register().MustRegister(m.sourceFilesCounter)

			m.batchDueBacklog = prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace:   metrics.Namespace,
				Subsystem:   metrics.AsyncDownload,
				Name:        "batch_due_backlog",
			Help:        "the number of async download v2 batches waiting to be dispatched",
			ConstLabels: labels,
		})
		metrics.Register().MustRegister(m.batchDueBacklog)

		m.batchOldestDueAgeSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   metrics.Namespace,
			Subsystem:   metrics.AsyncDownload,
			Name:        "batch_oldest_due_age_seconds",
			Help:        "the age in seconds of the oldest async download v2 due batch",
			ConstLabels: labels,
			})
			metrics.Register().MustRegister(m.batchOldestDueAgeSeconds)

			m.v2BatchStateCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace:   metrics.Namespace,
				Subsystem:   metrics.AsyncDownload,
				Name:        "v2_batch_state_count",
				Help:        "the count of async download v2 batch state entries",
				ConstLabels: labels,
			}, []string{"biz", "app", "file", "state"})
			metrics.Register().MustRegister(m.v2BatchStateCounter)

			m.v2BatchStateDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace:   metrics.Namespace,
				Subsystem:   metrics.AsyncDownload,
				Name:        "v2_batch_state_duration_seconds",
				Help:        "the duration(seconds) spent in async download v2 batch states",
				ConstLabels: labels,
				Buckets:     []float64{1, 2, 4, 6, 10, 15, 30, 50, 100, 150, 200, 400, 600},
			}, []string{"biz", "app", "file", "state"})
			metrics.Register().MustRegister(m.v2BatchStateDurationSeconds)

			m.v2TaskStateCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace:   metrics.Namespace,
				Subsystem:   metrics.AsyncDownload,
				Name:        "v2_task_state_count",
				Help:        "the count of async download v2 task state entries",
				ConstLabels: labels,
			}, []string{"biz", "app", "file", "state"})
			metrics.Register().MustRegister(m.v2TaskStateCounter)

			m.v2TaskStateDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace:   metrics.Namespace,
				Subsystem:   metrics.AsyncDownload,
				Name:        "v2_task_state_duration_seconds",
				Help:        "the duration(seconds) spent in async download v2 task states",
				ConstLabels: labels,
				Buckets:     []float64{1, 2, 4, 6, 10, 15, 30, 50, 100, 150, 200, 400, 600},
			}, []string{"biz", "app", "file", "state"})
			metrics.Register().MustRegister(m.v2TaskStateDurationSeconds)

			m.taskRepairCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace:   metrics.Namespace,
				Subsystem:   metrics.AsyncDownload,
				Name:        "task_repair_count",
			Help:        "the count of async download v2 task repairs",
			ConstLabels: labels,
		}, []string{"reason"})
		metrics.Register().MustRegister(m.taskRepairCounter)

		m.shardDispatchCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   metrics.Namespace,
			Subsystem:   metrics.AsyncDownload,
			Name:        "shard_dispatch_count",
			Help:        "the count of async download v2 shard dispatch attempts",
			ConstLabels: labels,
		}, []string{"status"})
		metrics.Register().MustRegister(m.shardDispatchCounter)

		m.shardDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace:   metrics.Namespace,
			Subsystem:   metrics.AsyncDownload,
			Name:        "shard_duration_seconds",
			Help:        "the duration(seconds) of async download v2 shard dispatches",
			ConstLabels: labels,
			Buckets:     []float64{1, 2, 4, 6, 10, 15, 30, 50, 100, 150, 200, 400, 600},
		}, []string{"status"})
		metrics.Register().MustRegister(m.shardDurationSeconds)

		asyncDownloadMetricInst = m
	})

	return asyncDownloadMetricInst
}

type metric struct {

	// jobDurationSeconds record the duration of the async download job.
	jobDurationSeconds *prometheus.HistogramVec

	// jobCounter record the count of the async download job.
	jobCounter *prometheus.CounterVec

	// taskDurationSeconds record the duration of the async download task.
	taskDurationSeconds *prometheus.HistogramVec

	// taskCounter record the count of the async download task.
	taskCounter *prometheus.CounterVec

	// sourceFilesSizeBytes record the size of the source files cache size in bytes.
	sourceFilesSizeBytes prometheus.Gauge

	// sourceFilesCounter record the count of the source files count.
	sourceFilesCounter prometheus.Gauge

	// batchDueBacklog records the current count of due v2 batches.
	batchDueBacklog prometheus.Gauge

	// batchOldestDueAgeSeconds records the age of the oldest due v2 batch.
	batchOldestDueAgeSeconds prometheus.Gauge

	// v2BatchStateCounter records V2 batch state transitions and creations.
	v2BatchStateCounter *prometheus.CounterVec

	// v2BatchStateDurationSeconds records time spent in V2 batch states.
	v2BatchStateDurationSeconds *prometheus.HistogramVec

	// v2TaskStateCounter records V2 task state transitions and creations.
	v2TaskStateCounter *prometheus.CounterVec

	// v2TaskStateDurationSeconds records time spent in V2 task states.
	v2TaskStateDurationSeconds *prometheus.HistogramVec

	// taskRepairCounter records task repair actions for v2 batches.
	taskRepairCounter *prometheus.CounterVec

	// shardDispatchCounter records v2 shard dispatch attempts.
	shardDispatchCounter *prometheus.CounterVec

	// shardDurationSeconds records v2 shard dispatch durations.
	shardDurationSeconds *prometheus.HistogramVec
}

func (m *metric) observeV2BatchCreated(batch *types.AsyncDownloadV2Batch) {
	if m == nil || batch == nil || m.v2BatchStateCounter == nil {
		return
	}
	m.v2BatchStateCounter.WithLabelValues(
		strconv.Itoa(int(batch.BizID)),
		strconv.Itoa(int(batch.AppID)),
		path.Join(batch.FilePath, batch.FileName),
		batch.State,
	).Inc()
}

func (m *metric) observeV2BatchTransition(batch *types.AsyncDownloadV2Batch, oldState string) {
	if m == nil || batch == nil || oldState == "" || oldState == batch.State {
		return
	}

	if m.v2BatchStateDurationSeconds != nil {
		duration := time.Since(batch.CreatedAt).Seconds()
		if oldState != types.AsyncDownloadBatchStateCollecting && !batch.DispatchStartedAt.IsZero() {
			duration = time.Since(batch.DispatchStartedAt).Seconds()
		}
		m.v2BatchStateDurationSeconds.WithLabelValues(
			strconv.Itoa(int(batch.BizID)),
			strconv.Itoa(int(batch.AppID)),
			path.Join(batch.FilePath, batch.FileName),
			oldState,
		).Observe(duration)
	}

	if m.v2BatchStateCounter != nil {
		m.v2BatchStateCounter.WithLabelValues(
			strconv.Itoa(int(batch.BizID)),
			strconv.Itoa(int(batch.AppID)),
			path.Join(batch.FilePath, batch.FileName),
			batch.State,
		).Inc()
	}
}

func (m *metric) observeV2TaskCreated(task *types.AsyncDownloadV2Task) {
	if m == nil || task == nil || m.v2TaskStateCounter == nil {
		return
	}
	m.v2TaskStateCounter.WithLabelValues(
		strconv.Itoa(int(task.BizID)),
		strconv.Itoa(int(task.AppID)),
		path.Join(task.FilePath, task.FileName),
		task.State,
	).Inc()
}

func (m *metric) observeV2TaskTransition(task *types.AsyncDownloadV2Task, oldState string, oldUpdatedAt time.Time) {
	if m == nil || task == nil || oldState == "" || oldState == task.State {
		return
	}

	if m.v2TaskStateDurationSeconds != nil {
		duration := time.Since(task.CreatedAt).Seconds()
		if oldState != types.AsyncDownloadJobStatusPending && !oldUpdatedAt.IsZero() {
			duration = time.Since(oldUpdatedAt).Seconds()
		}
		m.v2TaskStateDurationSeconds.WithLabelValues(
			strconv.Itoa(int(task.BizID)),
			strconv.Itoa(int(task.AppID)),
			path.Join(task.FilePath, task.FileName),
			oldState,
		).Observe(duration)
	}

	if m.v2TaskStateCounter != nil {
		m.v2TaskStateCounter.WithLabelValues(
			strconv.Itoa(int(task.BizID)),
			strconv.Itoa(int(task.AppID)),
			path.Join(task.FilePath, task.FileName),
			task.State,
		).Inc()
	}
}
