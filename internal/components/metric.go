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
	"context"
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

type componentCallerCtxKey struct{}

type componentCallerInfo struct {
	component string
	caller    string
}

// withComponentCaller stores resolved component and caller into the context.
func withComponentCaller(ctx context.Context, component, caller string) context.Context {
	return context.WithValue(ctx, componentCallerCtxKey{}, &componentCallerInfo{
		component: component,
		caller:    caller,
	})
}

// componentCallerFromCtx retrieves pre-resolved component and caller from the context.
// Falls back to runtime stack walking if not present.
func componentCallerFromCtx(ctx context.Context) (string, string) {
	if info, ok := ctx.Value(componentCallerCtxKey{}).(*componentCallerInfo); ok {
		return info.component, info.caller
	}
	return resolveComponentAndCaller()
}

// recordResponseMetrics records metrics for a successful HTTP response.
func recordResponseMetrics(resp *resty.Response) {
	m := initThirdPartyMetric()

	component, caller := componentCallerFromCtx(resp.Request.Context())
	method := resp.Request.Method
	status := statusGroup(resp.StatusCode())

	m.requestsTotal.WithLabelValues(component, method, status, caller).Inc()
	m.requestDuration.WithLabelValues(component, method, caller).Observe(float64(resp.Time().Milliseconds()))
}

// recordErrorMetrics records metrics for a failed HTTP request.
func recordErrorMetrics(req *resty.Request) {
	m := initThirdPartyMetric()

	component, caller := componentCallerFromCtx(req.RawRequest.Context())
	method := req.Method

	m.requestsTotal.WithLabelValues(component, method, "error", caller).Inc()
}

const (
	unknownComponent = "unknown"
	unknownCaller    = "unknown"

	// componentsPkgPrefix 是 internal/components 下子包的完整路径前缀，
	// 用于从调用栈 frame.Function 中识别出是哪个组件发起的请求。
	componentsPkgPrefix = "github.com/TencentBlueKing/bk-bscp/internal/components/"
)

// extractComponentFromFrame 从调用栈 frame.Function 中提取组件名，
// 即 internal/components 下第一级子包的包名。
//
// 例如:
//
//	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb.(*CMDBService).doRequest"
//	  -> "bkcmdb"
//	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/v4.CreateTicket"
//	  -> "itsm"
func extractComponentFromFrame(funcName string) (string, bool) {
	if !strings.HasPrefix(funcName, componentsPkgPrefix) {
		return "", false
	}
	rest := funcName[len(componentsPkgPrefix):]

	// 取到包边界：'/' 表示更深一级子包（如 itsm/v4），'.' 表示包内函数。
	end := len(rest)
	for i := 0; i < len(rest); i++ {
		if rest[i] == '/' || rest[i] == '.' {
			end = i
			break
		}
	}
	if end == 0 {
		return "", false
	}
	return rest[:end], true
}

// resolveComponentAndCaller 在同一次调用栈回溯里同时解析出 component 和 caller：
//   - component: 第一个属于 internal/components/<子包> 的 frame 所对应的组件名
//   - caller:    第一个跳过 resty + components 根 + 底层 HTTP wrapper 之后的 frame 的短函数名
func resolveComponentAndCaller() (component, caller string) {
	component = unknownComponent
	caller = unknownCaller

	pcs := make([]uintptr, 20)
	// 跳过 runtime.Callers 自身 + 当前函数 + 调用者（recordXxxMetrics）
	n := runtime.Callers(3, pcs)
	if n == 0 {
		return component, caller
	}

	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()

		if component == unknownComponent {
			if c, ok := extractComponentFromFrame(frame.Function); ok {
				component = c
			}
		}
		if caller == unknownCaller && !shouldSkipFrame(frame.Function) {
			caller = extractShortFuncName(frame.Function)
		}

		if component != unknownComponent && caller != unknownCaller {
			return component, caller
		}
		if !more {
			return component, caller
		}
	}
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

// skipFuncSuffixes 用于跳过组件包内的「底层 HTTP 封装函数」。
var skipFuncSuffixes = []string{
	".doRequest",
	".ItsmRequest",
}

// shouldSkipFrame returns true if the frame belongs to resty internals,
func shouldSkipFrame(funcName string) bool {
	for _, prefix := range skipPrefixes {
		if strings.Contains(funcName, prefix) {
			return true
		}
	}
	for _, suffix := range skipFuncSuffixes {
		if strings.HasSuffix(funcName, suffix) {
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
