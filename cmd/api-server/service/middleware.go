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
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
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

			var bizID string
			bizID, _ = extractBizAndAppID(r)
			if len(bizID) == 0 {
				bizID = strconv.Itoa(int(kt.BizID))
			}

			var status string
			if serviceName == "" {
				serviceName = chi.RouteContext(r.Context()).RoutePattern()
			}
			if handler == "" {
				handler = r.URL.String()
			}
			defer func() {
				metrics.BSCPServerHandledTotal.
					WithLabelValues(serviceName, handler, status, bizID, kt.User).
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

// 检查（或创建）默认的项目（Project）与环境（Environment）
func (p *proxy) checkOrCreateDefaultProjectEnv(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kt := kit.MustGetKit(r.Context())

		// 1. 调用底层的 RPC 服务去校验或创建默认的项目与环境
		in := &pbcs.EnsureDefaultProjectEnvReq{BizId: kt.BizID}
		resp, err := p.cfgClient.EnsureDefaultProjectEnv(kt.RpcCtx(), in)
		if err != nil {
			render.Render(w, r, rest.BadRequest(err))
			return
		}

		// 2. 将获取到的默认 ProjectID 和 EnvID 赋值给当前请求的 kit 上下文
		kt.ProjectID = resp.ProjectId
		kt.EnvID = resp.EnvId

		ctx := kit.WithKit(r.Context(), kt)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AppProjectEnvVerified 校验 App 是否属于指定的项目与环境。
// 必须放在 checkOrCreateDefaultProjectEnv 之后，依赖 kt.ProjectID / kt.EnvID 已被赋值。
func (p *proxy) AppProjectEnvVerified(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kt := kit.MustGetKit(r.Context())

		appIDStr := chi.URLParam(r, "app_id")
		if appIDStr == "" {
			err := errors.New("app_id is required in url params")
			render.Render(w, r, rest.BadRequest(err))
			return
		}

		appID, err := strconv.Atoi(appIDStr)
		if err != nil {
			render.Render(w, r, rest.BadRequest(err))
			return
		}

		// 调用 config-server GetApp 校验 App 归属于该项目+环境
		_, err = p.cfgClient.GetApp(kt.RpcCtx(), &pbcs.GetAppReq{
			BizId:     kt.BizID,
			AppId:     uint32(appID),
			ProjectId: kt.ProjectID,
			EnvId:     kt.EnvID,
		})
		if err != nil {
			logs.Errorf("verify app project/env failed, bizId=%d appId=%d projectId=%d envId=%d err=%v rid=%s",
				kt.BizID, uint32(appID), kt.ProjectID, kt.EnvID, err, kt.Rid)
			render.Render(w, r, rest.BadRequest(fmt.Errorf("app does not belong to the specified project or environment")))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// HookProjectVerified 校验 Hook 是否属于指定的项目。
// 必须放在 checkOrCreateDefaultProjectEnv 之后，依赖 kt.ProjectID 已被赋值。
// 通过 hook_id 调用 GetHook 获取详情
func (p *proxy) HookProjectVerified(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kt := kit.MustGetKit(r.Context())

		hookIDStr := chi.URLParam(r, "hook_id")
		if hookIDStr == "" {
			err := errors.New("hook_id is required in url params")
			render.Render(w, r, rest.BadRequest(err))
			return
		}

		hookID, err := strconv.Atoi(hookIDStr)
		if err != nil {
			render.Render(w, r, rest.BadRequest(err))
			return
		}

		// 调用 config-server GetHook 校验 Hook 归属于该项目
		_, err = p.cfgClient.GetHook(kt.RpcCtx(), &pbcs.GetHookReq{
			BizId:     kt.BizID,
			ProjectId: kt.ProjectID,
			HookId:    uint32(hookID),
		})
		if err != nil {
			logs.Errorf("verify hook project failed, bizId=%d hookId=%d projectId=%d err=%v rid=%s",
				kt.BizID, uint32(hookID), kt.ProjectID, err, kt.Rid)
			render.Render(w, r, rest.BadRequest(fmt.Errorf("hook does not belong to the specified project")))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GroupProjectVerified 校验 Group 是否属于指定的项目。
// 必须放在 checkOrCreateDefaultProjectEnv 之后，依赖 kt.ProjectID 已被赋值。
// 用于新路由（带 {project_id} 的 additional_bindings），依赖 URL 中的 project_id 参数已通过中间件注入 kt.ProjectID。
// 通过 group_id 调用 GetGroup 获取详情并校验归属。
func (p *proxy) GroupProjectVerified(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kt := kit.MustGetKit(r.Context())

		groupIDStr := chi.URLParam(r, "group_id")
		if groupIDStr == "" {
			err := errors.New("group_id is required in url params")
			render.Render(w, r, rest.BadRequest(err))
			return
		}

		groupID, err := strconv.Atoi(groupIDStr)
		if err != nil {
			render.Render(w, r, rest.BadRequest(err))
			return
		}

		// 调用 config-server GetGroup 校验 Group 归属于该项目
		_, err = p.cfgClient.GetGroup(kt.RpcCtx(), &pbcs.GetGroupReq{
			BizId:     kt.BizID,
			GroupId:   uint32(groupID),
			ProjectId: kt.ProjectID,
		})
		if err != nil {
			logs.Errorf("verify group project failed, bizId=%d groupId=%d projectId=%d err=%v rid=%s",
				kt.BizID, uint32(groupID), kt.ProjectID, err, kt.Rid)
			render.Render(w, r, rest.BadRequest(fmt.Errorf("group does not belong to the specified project")))
			return
		}

		next.ServeHTTP(w, r)
	})
}
