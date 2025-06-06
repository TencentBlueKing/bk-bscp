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

package service

import (
	"github.com/prometheus/client_golang/prometheus"
	prm "github.com/prometheus/client_golang/prometheus"

	"github.com/TencentBlueKing/bk-bscp/pkg/metrics"
)

// nolint:funlen
func initMetric(name string, blacklistBizIds []uint32) *metric {
	m := new(metric)
	labels := prm.Labels{"name": name}
	m.watchTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metrics.Namespace,
		Subsystem:   metrics.FSConfigConsume,
		Name:        "current_watch_count",
		Help:        "record the current total connection count of sidecars with watch",
		ConstLabels: labels,
	}, []string{"biz"})
	metrics.Register().MustRegister(m.watchTotal)

	m.watchCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Subsystem: metrics.FSConfigConsume,
			Name:      "total_watch_count",
			Help: "record the total connection count of sidecars with watch, that used to get the new connection count " +
				"within a specified time range",
			ConstLabels: labels,
		}, []string{"biz"})
	metrics.Register().MustRegister(m.watchCounter)

	m.clientMaxCPUUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metrics.Namespace,
		Subsystem:   metrics.FSConfigConsume,
		Name:        "client_max_cpu_usage",
		Help:        "record the maximum cpu usage",
		ConstLabels: labels,
	}, []string{"bizID", "appName"})
	metrics.Register().MustRegister(m.clientMaxCPUUsage)
	m.clientMaxMemUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metrics.Namespace,
		Subsystem:   metrics.FSConfigConsume,
		Name:        "client_max_memory_usage",
		Help:        "record the maximum memory usage",
		ConstLabels: labels,
	}, []string{"bizID", "appName"})
	metrics.Register().MustRegister(m.clientMaxMemUsage)
	m.clientCurrentCPUUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metrics.Namespace,
		Subsystem:   metrics.FSConfigConsume,
		Name:        "client_current_cpu_usage",
		Help:        "record the current cpu usage",
		ConstLabels: labels,
	}, []string{"bizID", "appName"})
	metrics.Register().MustRegister(m.clientCurrentCPUUsage)
	m.clientCurrentMemUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metrics.Namespace,
		Subsystem:   metrics.FSConfigConsume,
		Name:        "client_current_memory_usage",
		Help:        "record the current memory usage",
		ConstLabels: labels,
	}, []string{"bizID", "appName"})
	metrics.Register().MustRegister(m.clientCurrentMemUsage)

	m.downloadTotalSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metrics.Namespace,
		Subsystem:   metrics.FSConfigConsume,
		Name:        "file_download_total_size_bytes",
		Help:        "Total size of files downloaded, biz 0 means global",
		ConstLabels: labels,
	}, []string{"bizID"})
	metrics.Register().MustRegister(m.downloadTotalSize)

	m.downloadDelayRequests = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metrics.Namespace,
		Subsystem:   metrics.FSConfigConsume,
		Name:        "file_download_delay_requests",
		Help:        "Total number of downloaded file delayed requests, biz 0 means global",
		ConstLabels: labels,
	}, []string{"bizID"})
	metrics.Register().MustRegister(m.downloadDelayRequests)
	m.downloadDelayMilliseconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metrics.Namespace,
		Subsystem:   metrics.FSConfigConsume,
		Name:        "file_download_delay_milliseconds",
		Help:        "Delay milliseconds of downloaded file, biz 0 means global",
		ConstLabels: labels,
	}, []string{"bizID"})
	metrics.Register().MustRegister(m.downloadDelayMilliseconds)
	// 记录客户端版本变更事件
	versionChange := []string{"biz", "app", "clientType", "clientMode", "originalReleaseID",
		"targetReleaseID", "releaseChangeStatus"}
	m.changeTotalNumber = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   metrics.Namespace,
			Subsystem:   metrics.FSConfigConsume,
			Name:        "total_change_count",
			Help:        "record the total number of changes in a certain version",
			ConstLabels: labels,
		}, versionChange)
	metrics.Register().MustRegister(m.changeTotalNumber)
	m.changeTotalFileNumber = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   metrics.Namespace,
			Subsystem:   metrics.FSConfigConsume,
			Name:        "total_file_change_count",
			Help:        "record the total number of file changes in a certain version",
			ConstLabels: labels,
		}, versionChange)
	metrics.Register().MustRegister(m.changeTotalFileNumber)
	m.changeTotalFileSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   metrics.Namespace,
			Subsystem:   metrics.FSConfigConsume,
			Name:        "total_file_size",
			Help:        "record the total file size of changes in a certain version",
			ConstLabels: labels,
		}, versionChange)
	metrics.Register().MustRegister(m.changeTotalFileSize)
	m.changeTotalSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   metrics.Namespace,
			Subsystem:   metrics.FSConfigConsume,
			Name:        "total_change_seconds",
			Help:        "record the total time spent on changes in a certain version",
			ConstLabels: labels,
		}, versionChange)
	metrics.Register().MustRegister(m.changeTotalSeconds)

	black := make(map[uint32]struct{}, len(blacklistBizIds))
	for _, id := range blacklistBizIds {
		black[id] = struct{}{}
	}
	m.blacklist = black

	return m
}

type metric struct {
	// watchTotal record the current total connection count of sidecars with watch.
	watchTotal *prometheus.GaugeVec
	// watchCounter record the total connection count of sidecars with watch, used to get the new the connection count
	// within a specified time range.
	watchCounter *prometheus.CounterVec

	// clientMaxCPUUsage The maximum cpu usage of the client was collected
	clientMaxCPUUsage *prometheus.GaugeVec
	// clientMaxMemUsage the maximum memory usage was collected
	clientMaxMemUsage *prometheus.GaugeVec
	// clientCurrentCPUUsage the cpu usage of the client was collected
	clientCurrentCPUUsage *prometheus.GaugeVec
	// clientCurrentMemUsage the current memory usage of the client is collected
	clientCurrentMemUsage     *prometheus.GaugeVec
	downloadTotalSize         *prometheus.GaugeVec
	downloadDelayRequests     *prometheus.GaugeVec
	downloadDelayMilliseconds *prometheus.GaugeVec
	// 变更总数
	changeTotalNumber *prometheus.CounterVec
	// 变更总文件数
	changeTotalFileNumber *prometheus.CounterVec
	// 变更总文件大小
	changeTotalFileSize *prometheus.HistogramVec
	// 变更总耗时
	changeTotalSeconds *prometheus.HistogramVec
	blacklist          map[uint32]struct{}
}

// collectDownload collects metrics for download
func (m *metric) collectDownload(biz string, totalSize, delayRequests, delayMilliseconds int64) {
	m.downloadTotalSize.With(prm.Labels{"bizID": biz}).Set(float64(totalSize))
	m.downloadDelayRequests.With(prm.Labels{"bizID": biz}).Set(float64(delayRequests))
	m.downloadDelayMilliseconds.With(prm.Labels{"bizID": biz}).Set(float64(delayMilliseconds))
}

func (m *metric) shouldReport(bizID uint32) bool {
	_, exists := m.blacklist[bizID]
	return !exists
}
