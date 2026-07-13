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

package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/TencentBlueKing/bk-bscp/internal/components"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/repository"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/uuid"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbas "github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server"
	"github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server/types"
	"github.com/TencentBlueKing/bk-bscp/pkg/rest"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
)

var (
	// skipValidateUserPaths 那些路径不需要校验用户，比如v4 itsm中的回调
	skipValidateUserPaths = []string{
		"approval_callback",
	}
)

// initKitWithBKJWT 蓝鲸网关鉴权
func (a authorizer) initKitWithBKJWT(r *http.Request, k *kit.Kit, multiErr *multierror.Error) bool {
	if a.gwParser == nil {
		err := errors.New("gw pubkey is empty")
		multiErr.Errors = append(multiErr.Errors, errors.Wrap(err, "auth with bk_jwt"))
		return false
	}
	validateUser := true
	for _, path := range skipValidateUserPaths {
		if strings.HasSuffix(r.URL.Path, path) {
			validateUser = false
			break
		}
	}
	kt, err := a.gwParser.Parse(r.Context(), r.Header, validateUser)
	if err != nil {
		multiErr.Errors = append(multiErr.Errors, errors.Wrap(err, "auth with bk_jwt"))
		return false
	}

	// jwt 只会从jwt里面解析出 app_code
	// user 会从jwt获取, fallback 从 X-Bkapi-User-Name 头部获取(app校验成功, 说明有权限, 网关使用场景)
	k.AppCode = kt.AppCode
	k.User = kt.User
	k.TenantID = r.Header.Get(constant.BkTenantID) // 接口调用会带上这个租户ID
	return true
}

// initKitWithCookie 蓝鲸统一登入Cookie鉴权
func (a authorizer) initKitWithCookie(r *http.Request, k *kit.Kit, multiErr *multierror.Error) bool {
	loginCred, err := a.authLoginClient.GetLoginCredentialFromCookies(r)
	if err != nil {
		multiErr.Errors = append(multiErr.Errors, errors.Wrap(err, "auth with cookie"))
		return false
	}

	req := &pbas.UserCredentialReq{Uid: loginCred.UID, Token: loginCred.Token}

	resp, err := a.authClient.GetUserInfo(k.RpcCtx(), req)
	if err != nil {
		s := status.Convert(err)
		// 无权限的需要特殊跳转
		if s.Code() == codes.PermissionDenied {
			multiErr.Errors = append(multiErr.Errors, errors.Wrap(errf.ErrPermissionDenied, s.Message()))
		} else {
			multiErr.Errors = append(multiErr.Errors, errors.Wrap(errors.New(s.Message()), "auth with cookie"))
		}

		return false
	}

	// 登入态只支持用户名
	k.User = resp.Username
	k.TenantID = resp.TenantId
	k.TimeZone = resp.TimeZone
	k.BkToken = loginCred.Token
	return true
}

// initKitWithDevEnv Dev环境, 可以设置环境变量鉴权
func (a authorizer) initKitWithDevEnv(r *http.Request, k *kit.Kit, _ *multierror.Error) bool {
	if !a.isDevMode() {
		return false
	}
	user := os.Getenv("BK_USER_FOR_TEST")
	appCode := os.Getenv("BK_APP_CODE_FOR_TEST")

	if user != "" && appCode != "" {
		k.User = user
		k.AppCode = appCode
		if tenantID := r.Header.Get(constant.BkTenantID); tenantID != "" {
			k.TenantID = tenantID
		}
		return true
	}

	return false
}

// UnifiedAuthentication HTTP API 鉴权, 异常返回json信息
func (a authorizer) UnifiedAuthentication(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		k := &kit.Kit{
			Ctx: r.Context(),
			Rid: components.RequestIDValue(r.Context()),
		}
		k.Lang = tools.GetLangFromReq(r)
		multiErr := &multierror.Error{}
		// log 打印，将header 信息打印出来
		slog.Info("request header", "header", r.Header, "path", path.Base(r.URL.Path))
		switch {
		case a.initKitWithBKJWT(r, k, multiErr):
		case a.initKitWithCookie(r, k, multiErr):
		case a.initKitWithDevEnv(r, k, multiErr):
		default:
			// API类返回规范的JSON错误信息
			loginURL, loginPlainURL := a.authLoginClient.BuildLoginURL(r)
			render.Render(w, r, rest.NotLoggedInErr(multiErr, loginURL, loginPlainURL))
			return
		}

		ctx := kit.WithKit(r.Context(), k)
		r.Header.Set(constant.AppCodeKey, k.AppCode)
		r.Header.Set(constant.RidKey, k.Rid)
		r.Header.Set(constant.UserKey, k.User)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// uploadAppKeyAuthHeader app 凭证认证请求头, 复用蓝鲸网关鉴权头格式
const uploadAppKeyAuthHeader = "X-Bkapi-Authorization"

// UploadAppKeyAuthentication 上传类接口的 app 凭证认证中间件。
// 第三方携带 X-Bkapi-Authorization: {"bk_app_code":"..","bk_app_secret":".."} 绕开网关直连,
// 与本服务 cc.G().BaseConf(配置 baseConf.app_code/app_secret)一致即放行。
// 未携带凭证或本服务未配置 app 凭证时, 回退到 UnifiedAuthentication, 保证 Cookie/JWT 仍可用。
func (a authorizer) UploadAppKeyAuthentication(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		rawAuth := r.Header.Get(uploadAppKeyAuthHeader)
		expectCode := cc.G().BaseConf.AppCode
		expectSecret := cc.G().BaseConf.AppSecret

		// 未携带凭证 / 本服务未配置 app 凭证 → 回退统一认证(Cookie/JWT)
		if rawAuth == "" || expectCode == "" || expectSecret == "" {
			a.UnifiedAuthentication(next).ServeHTTP(w, r)
			return
		}

		auth := new(components.BKAPIGWAuth)
		if err := json.Unmarshal([]byte(rawAuth), auth); err != nil || auth.AppCode == "" || auth.AppSecret == "" {
			// 头存在但不是 app 凭证格式 → 回退, 兼容其它鉴权来源
			a.UnifiedAuthentication(next).ServeHTTP(w, r)
			return
		}

		// 常量时间比较, 防时序攻击
		codeMatch := subtle.ConstantTimeCompare([]byte(auth.AppCode), []byte(expectCode)) == 1
		secretMatch := subtle.ConstantTimeCompare([]byte(auth.AppSecret), []byte(expectSecret)) == 1
		if !codeMatch || !secretMatch {
			render.Render(w, r, rest.Unauthorized(errors.New("invalid app code or app secret")))
			return
		}

		k := &kit.Kit{
			Ctx:      r.Context(),
			Rid:      components.RequestIDValue(r.Context()),
			AppCode:  auth.AppCode,
			User:     "bk_app:" + auth.AppCode, // 无用户态, 给审计保留可识别来源
			TenantID: r.Header.Get(constant.BkTenantID),
		}
		k.Lang = tools.GetLangFromReq(r)

		ctx := kit.WithKit(r.Context(), k)
		r.Header.Set(constant.AppCodeKey, k.AppCode)
		r.Header.Set(constant.RidKey, k.Rid)
		r.Header.Set(constant.UserKey, k.User)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// WebAuthentication HTTP 前端鉴权, 异常跳转302到登入页面
func (a authorizer) WebAuthentication(webHost string) func(http.Handler) http.Handler {
	ignoreExtMap := map[string]struct{}{
		".js":  {},
		".css": {},
		".map": {},
	}

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// 静态资源过滤, 注意不会带鉴权信息
			fileExt := filepath.Ext(r.URL.Path)
			if _, ok := ignoreExtMap[fileExt]; ok {
				next.ServeHTTP(w, r)
				return
			}

			k := &kit.Kit{
				Ctx: r.Context(),
				Rid: components.RequestIDValue(r.Context()),
			}
			multiErr := &multierror.Error{}

			switch {
			case a.initKitWithCookie(r, k, multiErr):
			case a.initKitWithDevEnv(r, k, multiErr):
			default:
				// 如果无权限, 跳转到403页面
				for _, err := range multiErr.Errors {
					if errors.Is(err, errf.ErrPermissionDenied) {
						msg := base64.StdEncoding.EncodeToString([]byte(errf.GetErrMsg(err)))
						redirectURL := fmt.Sprintf("/403.html?msg=%s", url.QueryEscape(msg))
						slog.Info("web auth failed, redirect to 403 page", "err", err)
						http.Redirect(w, r, redirectURL, http.StatusFound)
						return
					}
				}

				// web类型做302跳转登入
				slog.Info("web auth failed, redirect to login page", "err", multiErr)
				http.Redirect(w, r, a.authLoginClient.BuildLoginRedirectURL(r, webHost), http.StatusFound)
				return
			}

			ctx := kit.WithKit(r.Context(), k)
			r.Header.Set(constant.AppCodeKey, k.AppCode)
			r.Header.Set(constant.RidKey, k.Rid)
			r.Header.Set(constant.UserKey, k.User)

			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}

// ContentVerified 内容操作校验中间件, 需要放到UnifiedAuthentication和BizVerified后面
// 服务下的配置项内容需要校验服务权限，模版空间下的模版配置项内容需要校验模版空间权限
func (a authorizer) ContentVerified(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		kt := kit.MustGetKit(r.Context())

		appID, tmplSpaceID, err := repository.GetContentLevelID(r)
		if err != nil {
			render.Render(w, r, rest.BadRequest(err))
			return
		}

		if appID > 0 {
			// NOTE: authenticate app on iam

			space, err := a.authClient.QuerySpaceByAppID(kt.RpcCtx(), &pbas.QuerySpaceByAppIDReq{AppId: appID})
			if err != nil {
				render.Render(w, r, rest.BadRequest(err))
				return
			}
			kt.AppID = appID
			kt.SpaceID = space.SpaceId
			kt.SpaceTypeID = space.SpaceTypeId
		}

		if tmplSpaceID > 0 {
			// NOTE: authenticate template space on iam

			kt.TmplSpaceID = tmplSpaceID
		}

		ctx := kit.WithKit(r.Context(), kt)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// AppVerified App校验中间件, 需要放到 UnifiedAuthentication 后面, url 需要添加 {app_id} 变量
func (a authorizer) AppVerified(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
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
		space, err := a.authClient.QuerySpaceByAppID(kt.RpcCtx(), &pbas.QuerySpaceByAppIDReq{AppId: uint32(appID)})
		if err != nil {
			render.Render(w, r, rest.GRPCErr(err))
			return
		}

		kt.AppID = uint32(appID)
		kt.SpaceID = space.SpaceId
		kt.SpaceTypeID = space.SpaceTypeId
		ctx := kit.WithKit(r.Context(), kt)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

func getBizID(r *http.Request) string {
	bizIDStr := chi.URLParam(r, "biz_id")
	if bizIDStr != "" {
		return bizIDStr
	}

	parts := strings.Split(r.URL.Path, "/")
	for idx, v := range parts {
		if v == "biz_id" && len(parts) > idx+1 {
			return parts[idx+1]
		}
		if v == "biz" && len(parts) > idx+1 {
			return parts[idx+1]
		}
	}

	return ""
}

// BizVerified 业务ID鉴权, url必须满足/{biz_id}; /biz_id/{n} 或者 /biz/{n}
func (a authorizer) BizVerified(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		kt := kit.MustGetKit(r.Context())
		bizIDStr := getBizID(r)
		if bizIDStr == "" {
			err := errors.New("biz id is required in url params")
			render.Render(w, r, rest.BadRequest(err))
			return
		}

		// 设置语言
		lang := tools.GetLangFromReq(r)
		kt.Lang = lang

		bizIDUint64, err := strconv.ParseUint(bizIDStr, 10, 32)
		if err != nil {
			render.Render(w, r, rest.BadRequest(err))
			return
		}
		bizID := uint32(bizIDUint64)
		kt.BizID = bizID

		if !a.HasBiz(r.Context(), bizID) {
			err := fmt.Errorf("biz id %d does not exist", bizID)
			render.Render(w, r, rest.BadRequest(err))
			return
		}

		// skip validate biz permission when user is for test
		if cc.G().IsDevMode() && strings.HasPrefix(kt.User, constant.BKUserForTestPrefix) {
			ctx := kit.WithKit(r.Context(), kt)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		kt.OperateWay = r.Header.Get(constant.OperateWayKey)

		ctx := kit.WithKit(r.Context(), kt)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// Project 校验中间件
func (a authorizer) VerifyProjectExists(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kt := kit.MustGetKit(r.Context())
		projectIDStr := chi.URLParam(r, "project_id")

		// 如果 URL 里没传 project_id
		if projectIDStr == "" {
			render.Render(w, r, rest.BadRequest(errors.New("project_id is required")))
			return
		}

		projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
		if err != nil {
			render.Render(w, r, rest.BadRequest(err))
			return
		}

		resp, err := a.authClient.VerifyProject(kt.RpcCtx(), &pbas.VerifyProjectReq{
			BizId:     kt.BizID,
			ProjectId: uint32(projectID),
		})
		if err != nil {
			logs.Errorf("verify project failed, bizID: %d, projectID: %d, err: %v", kt.BizID, uint32(projectID), err)
			msg := err.Error()
			if st, ok := status.FromError(err); ok {
				msg = st.Message()
			}

			render.Render(w, r, rest.BadRequest(errors.New(msg)))
			return
		}

		if !resp.Exists {
			render.Render(w, r, rest.BadRequest(fmt.Errorf("project_id %d does not exist", projectID)))
			return
		}

		kt.ProjectID = uint32(projectID)

		next.ServeHTTP(w, r.WithContext(kit.WithKit(r.Context(), kt)))
	})
}

// Env 校验中间件
func (a authorizer) VerifyEnvExists(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kt := kit.MustGetKit(r.Context())

		envIDStr := chi.URLParam(r, "env_id")
		if envIDStr == "" {
			render.Render(w, r, rest.BadRequest(errors.New("env_id is required")))
			return
		}

		envID, err := strconv.ParseUint(envIDStr, 10, 32)
		if err != nil {
			render.Render(w, r, rest.BadRequest(err))
			return
		}

		resp, err := a.authClient.VerifyEnv(kt.RpcCtx(), &pbas.VerifyEnvReq{
			BizId:     kt.BizID,
			ProjectId: kt.ProjectID,
			EnvId:     uint32(envID),
		})
		if err != nil {
			logs.Errorf("verify env failed, bizID: %d, projectID: %d, envID: %d, err: %v", kt.BizID, kt.ProjectID, uint32(envID), err)
			msg := err.Error()
			if st, ok := status.FromError(err); ok {
				msg = st.Message()
			}

			render.Render(w, r, rest.BadRequest(errors.New(msg)))
			return
		}

		if !resp.Exists {
			render.Render(w, r, rest.BadRequest(fmt.Errorf("env_id %d does not exist", envID)))
			return
		}

		kt.EnvID = uint32(envID)

		next.ServeHTTP(w, r.WithContext(kit.WithKit(r.Context(), kt)))
	})
}

// dummyVerified dummy鉴权方式，测试使用
func dummyVerified(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		k := &kit.Kit{
			Ctx:         r.Context(),
			User:        "",
			Rid:         components.RequestIDValue(r.Context()),
			AppID:       0,
			AppCode:     "dummyApp", // 测试 App
			SpaceID:     "",
			SpaceTypeID: "",
		}
		ctx := kit.WithKit(r.Context(), k)

		r.Header.Set(constant.AppCodeKey, k.AppCode)
		r.Header.Set(constant.RidKey, k.Rid)
		r.Header.Set(constant.UserKey, k.User)

		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// BKRepoVerified bk_repo 回调鉴权
func BKRepoVerified(next http.Handler) http.Handler {
	return dummyVerified(next)
}

// IAMVerify IAM 回调鉴权
func (a *authorizer) IAMVerify(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		k := &kit.Kit{
			Ctx: r.Context(),
		}

		if err := a.iamRequestFilter(w, r, k); err != nil {
			render.Render(w, r, rest.BadRequest(errors.Wrap(errf.ErrPermissionDenied, "authorized failed")))
			return
		}

		ctx := kit.WithKit(r.Context(), k)

		next.ServeHTTP(w, r.WithContext(ctx))
	}

	return http.HandlerFunc(fn)
}

func (a *authorizer) iamRequestFilter(w http.ResponseWriter, req *http.Request, kit *kit.Kit) error {
	isAuthorized, err := a.checkRequestAuthorization(req)
	if err != nil {
		return errf.New(http.StatusInternalServerError, err.Error())
	}
	if !isAuthorized {
		return errf.New(types.UnauthorizedErrorCode, "authorized failed")
	}

	rid := getRid(req.Header)
	req.Header.Set(constant.RidKey, rid)

	// set rid to response header, used to troubleshoot the problem.
	w.Header().Set(client.RequestIDHeader, rid)

	// use sys language as bscp language
	req.Header.Set(constant.LanguageKey, req.Header.Get("Blueking-Language"))

	user := req.Header.Get(constant.UserKey)
	if len(user) == 0 {
		req.Header.Set(constant.UserKey, "auth")
	}

	appCode := req.Header.Get(constant.AppCodeKey)
	if len(appCode) == 0 {
		req.Header.Set(constant.AppCodeKey, client.SystemIDIAM)
	}

	kit.AppCode = appCode
	kit.Rid = rid
	kit.User = user
	kit.TenantID = req.Header.Get(constant.BkTenantID) // 多租户环境权限中心回调时会带上

	return nil
}

// getRid get request id from header. if rid is empty, generate a rid to return.
func getRid(h http.Header) string {
	if rid := h.Get(client.RequestIDHeader); len(rid) != 0 {
		return rid
	}

	if rid := h.Get(constant.RidKey); len(rid) != 0 {
		return rid
	}

	return uuid.UUID()
}

func (a *authorizer) checkRequestAuthorization(req *http.Request) (bool, error) {
	rid := req.Header.Get(client.RequestIDHeader)
	name, pwd, ok := req.BasicAuth()
	if !ok || name != client.SystemIDIAM {
		logs.Errorf("request have no basic authorization, rid: %s", rid)
		return false, nil
	}

	// grpc call
	kt := kit.New()
	kt.TenantID = req.Header.Get(constant.BkTenantID) // 多租户环境权限中心回调时会带上
	kt.Ctx = req.Context()

	resp, err := a.authClient.IAMVerify(kt.RpcCtx(), &pbas.IAMVerifyReq{Token: pwd})
	if err != nil {
		return false, err
	}

	return resp.GetIsAuthorized(), nil
}
