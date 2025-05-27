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
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"

	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/metrics"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	"github.com/TencentBlueKing/bk-bscp/pkg/rest"
)

// CheckDefaultTmplSpace create default template space if not existent
func (p *proxy) CheckDefaultTmplSpace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bizIDStr := chi.URLParam(r, "biz_id")
		bizIDInt, err := strconv.Atoi(bizIDStr)
		if err != nil {
			render.Render(w, r, rest.BadRequest(err))
			return
		}
		bizID := uint32(bizIDInt)
		if bizsOfTS.Has(bizID) {
			next.ServeHTTP(w, r)
			return
		}

		kt := kit.MustGetKit(r.Context())
		// create default template space when not existent
		in := &pbcs.CreateDefaultTmplSpaceReq{BizId: bizID}
		if _, err := p.cfgClient.CreateDefaultTmplSpace(kt.RpcCtx(), in); err != nil {
			render.Render(w, r, rest.BadRequest(err))
			return
		}
		bizsOfTS.Set(bizID)

		next.ServeHTTP(w, r)
	})
}

// HttpServerHandledTotal count http operands
func (p *proxy) HttpServerHandledTotal(serviceName, handler string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			kt := kit.MustGetKit(r.Context())
			var status string
			if serviceName == "" {
				serviceName = chi.RouteContext(r.Context()).RoutePattern()
			}
			if handler == "" {
				handler = r.URL.String()
			}
			defer func() {
				metrics.BSCPServerHandledTotal.
					WithLabelValues(serviceName, handler, status, strconv.Itoa(int(kt.BizID)), kt.User).
					Inc()
			}()
			next.ServeHTTP(w, r)
			status = strconv.Itoa(w.(interface {
				http.ResponseWriter
				Status() int
			}).Status())
		}
		return http.HandlerFunc(fn)
	}
}

func (p *proxy) metricsMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			bizID, appID := extractBizAndAppID(r)
			bw := &bodyCaptureWriter{
				ResponseWriter: middleware.NewWrapResponseWriter(w, r.ProtoMajor),
				statusCode:     http.StatusOK,
			}
			errorMsg := ""
			defer func() {
				duration := time.Since(start)
				if bw.statusCode >= 400 {
					var errorResponse struct {
						Error struct {
							Code    string `json:"code"`
							Message string `json:"message"`
							Data    any    `json:"data"`
							Details any    `json:"details"`
						} `json:"error"`
					}
					if err := json.Unmarshal(bw.body.Bytes(), &errorResponse); err == nil {
						errorMsg = errorResponse.Error.Message
					} else {
						errorMsg = bw.body.String()
					}
				}
				statusCode := strconv.Itoa(bw.statusCode)

				p.mc.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, statusCode, bizID, appID, errorMsg).Inc()
				p.mc.requestDuration.WithLabelValues(r.Method, r.URL.Path, bizID, appID).Observe(duration.Seconds())
			}()

			next.ServeHTTP(bw, r)
		}
		return http.HandlerFunc(fn)
	}
}

func extractBizAndAppID(r *http.Request) (bizID, appID string) {
	// 优先使用 chi.URLParam
	bizID = chi.URLParam(r, "biz_id")
	appID = chi.URLParam(r, "app_id")

	// 如果 URLParam 没取到，再从路径中尝试提取
	if bizID == "" || appID == "" {
		parts := strings.Split(r.URL.Path, "/")
		for idx, v := range parts {
			if bizID == "" && (v == "biz" || v == "biz_id") && len(parts) > idx+1 {
				bizID = parts[idx+1]
			}
			if appID == "" && (v == "app" || v == "app_id") && len(parts) > idx+1 {
				appID = parts[idx+1]
			}
		}
	}

	return
}

type bodyCaptureWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (w *bodyCaptureWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyCaptureWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}
