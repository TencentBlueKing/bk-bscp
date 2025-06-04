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
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/TencentBlueKing/bk-bscp/pkg/metrics"
)

func initMetric() *metric {
	m := new(metric)
	m.currentUploadedFolderSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Name:      "upload_file_directory_size_bytes",
		Help:      "Size of the directory in bytes",
	}, []string{"bizID", "resourceID"})
	metrics.Register().MustRegister(m.currentUploadedFolderSize)

	m.uploadFileCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Name:      "upload_file_count_total",
			Help:      "Number of uploaded files",
		},
		[]string{"biz"},
	)
	metrics.Register().MustRegister(m.uploadFileCount)

	m.uploadDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metrics.Namespace,
			Name:      "upload_duration_seconds",
			Help:      "Time taken for file upload",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"biz"},
	)
	metrics.Register().MustRegister(m.uploadDuration)

	m.uploadTotalSize = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Name:      "upload_total_size_bytes",
			Help:      "Total size of uploaded files",
		},
		[]string{"biz"},
	)
	metrics.Register().MustRegister(m.uploadTotalSize)

	return m
}

type metric struct {
	// currentUploadedFolderSize Record the current uploaded folder size

	currentUploadedFolderSize *prometheus.GaugeVec

	// 文件上传数量
	uploadFileCount *prometheus.CounterVec
	// 文件上传耗时
	uploadDuration *prometheus.HistogramVec
	// 文件上传大小
	uploadTotalSize *prometheus.CounterVec
}
