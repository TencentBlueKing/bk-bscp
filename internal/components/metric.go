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

package components

import (
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/TencentBlueKing/bk-bscp/pkg/metrics"
)

const (
	// ThirdPartyAPISubSys defines the subsystem name for third-party API metrics.
	ThirdPartyAPISubSys = "third_party_api"
)

var (
	thirdPartyMetric *tpMetric
	tpMetricOnce     sync.Once
)

type tpMetric struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

func initThirdPartyMetric() *tpMetric {
	tpMetricOnce.Do(func() {
		m := &tpMetric{}

		m.requestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Subsystem: ThirdPartyAPISubSys,
			Name:      "requests_total",
			Help:      "Total number of third-party API requests",
		}, []string{"component", "method", "status", "caller"})
		metrics.Register().MustRegister(m.requestsTotal)

		m.requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: metrics.Namespace,
			Subsystem: ThirdPartyAPISubSys,
			Name:      "request_duration_milliseconds",
			Help:      "Duration of third-party API requests in milliseconds",
			Buckets:   []float64{10, 30, 50, 100, 200, 500, 1000, 2000, 5000, 10000},
		}, []string{"component", "method", "caller"})
		metrics.Register().MustRegister(m.requestDuration)

		thirdPartyMetric = m
	})
	return thirdPartyMetric
}

// recordResponseMetrics records metrics for a successful HTTP response.
func recordResponseMetrics(resp *resty.Response) {
	m := initThirdPartyMetric()

	rawURL := resp.Request.RawRequest.URL.String()
	component := extractComponent(rawURL)
	method := resp.Request.Method
	status := statusGroup(resp.StatusCode())
	caller := getCallerFromStack()

	m.requestsTotal.WithLabelValues(component, method, status, caller).Inc()
	m.requestDuration.WithLabelValues(component, method, caller).Observe(float64(resp.Time().Milliseconds()))
}

// recordErrorMetrics records metrics for a failed HTTP request.
func recordErrorMetrics(req *resty.Request) {
	m := initThirdPartyMetric()

	rawURL := req.RawRequest.URL.String()
	component := extractComponent(rawURL)
	method := req.Method
	caller := getCallerFromStack()

	m.requestsTotal.WithLabelValues(component, method, "error", caller).Inc()
}

// knownComponents maps URL host keywords to component names.
var knownComponents = []string{"cmdb", "gse", "itsm", "bcs", "paas", "user", "notice", "push", "nodeman"}

// extractComponent identifies the third-party component from a request URL.
func extractComponent(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "unknown"
	}
	host := strings.ToLower(u.Hostname())
	for _, keyword := range knownComponents {
		if strings.Contains(host, keyword) {
			return keyword
		}
	}
	return host
}

// statusGroup converts an HTTP status code to a group label (2xx, 3xx, 4xx, 5xx).
func statusGroup(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return strconv.Itoa(code)
	}
}

// skipPrefixes are package path prefixes to skip when walking the call stack.
var skipPrefixes = []string{
	"github.com/go-resty/resty",
	"github.com/TencentBlueKing/bk-bscp/internal/components.",
}

// getCallerFromStack walks the call stack to find the business caller
// that initiated the third-party API request.
func getCallerFromStack() string {
	pcs := make([]uintptr, 20)
	n := runtime.Callers(3, pcs)
	if n == 0 {
		return "unknown"
	}
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if shouldSkipFrame(frame.Function) {
			if !more {
				break
			}
			continue
		}
		return extractShortFuncName(frame.Function)
	}
	return "unknown"
}

// shouldSkipFrame returns true if the frame belongs to resty internals or
// the components package itself.
func shouldSkipFrame(funcName string) bool {
	for _, prefix := range skipPrefixes {
		if strings.Contains(funcName, prefix) {
			return true
		}
	}
	return false
}

// extractShortFuncName extracts a readable short function name.
// "github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb.(*CMDBService).SearchObjectAttr"
//
//	-> "CMDBService.SearchObjectAttr"
func extractShortFuncName(fullName string) string {
	// Find the last '/' to strip the module path
	if idx := strings.LastIndex(fullName, "/"); idx >= 0 {
		fullName = fullName[idx+1:]
	}
	// Strip the package name (e.g. "bkcmdb.")
	if idx := strings.Index(fullName, "."); idx >= 0 {
		fullName = fullName[idx+1:]
	}
	// Remove pointer receiver markers
	fullName = strings.TrimLeft(fullName, "(*")
	fullName = strings.ReplaceAll(fullName, ")", "")
	return fullName
}
