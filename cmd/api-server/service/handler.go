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
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	"github.com/TencentBlueKing/bk-bscp/pkg/rest"
)

// HealthyHandler livenessProbe 健康检查
func (p *proxy) HealthyHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

// ReadyHandler ReadinessProbe 健康检查
func (p *proxy) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	p.Healthz(w, r)
}

// Healthz service health check.
func (p *proxy) Healthz(w http.ResponseWriter, r *http.Request) {
	if shutdown.IsShuttingDown() {
		logs.Errorf("service healthz check failed, current service is shutting down")
		w.WriteHeader(http.StatusServiceUnavailable)
		rest.WriteResp(w, rest.NewBaseResp(errf.UnHealth, "current service is shutting down"))
		return
	}

	if err := p.state.Healthz(); err != nil {
		logs.Errorf("etcd healthz check failed, err: %v", err)
		rest.WriteResp(w, rest.NewBaseResp(errf.UnHealth, "etcd healthz error, "+err.Error()))
		return
	}

	rest.WriteResp(w, rest.NewBaseResp(errf.OK, "healthy"))
}

// LogoutHandler return redirect url
func (p *proxy) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	data := p.authorizer.LogOut(r)
	render.Render(w, r, rest.OKRender(data))
}

// UserInfoHandler 鉴权后的用户信息接口
func UserInfoHandler(w http.ResponseWriter, r *http.Request) {
	k := kit.MustGetKit(r.Context())

	user := kit.User{Username: k.User, TenantID: k.TenantID}
	render.Render(w, r, rest.OKRender(user))
}

// FeatureFlags feature flags
type FeatureFlags struct {
	// BizView 是否开启业务体验
	BizView bool `json:"BIZ_VIEW"`
	// ResourceLimit 业务资源限制
	ResourceLimit cc.ResourceLimit `json:"RESOURCE_LIMIT"`
	// TrpcGoPlugin trpc go plugin
	TrpcGoPlugin TrpcGoPlugin `json:"TRPC_GO_PLUGIN"`
	// EnableTenantMode 是否开启租户模式
	EnableTenantMode bool `json:"ENABLE_TENANT_MODE"`
	// TenantMode 租户模式
	TenantMode string `json:"TENANT_MODE"`
	// ProcessConfigView 进程与配置管理是否可见
	ProcessConfigView bool `json:"PROCESS_CONFIG_VIEW"`
}

// TrpcGoPlugin trpc go plugin
type TrpcGoPlugin struct {
	Enable           bool   `json:"enable"`
	ModuleDomain     string `json:"module_domain"`
	BscpModuleDomain string `json:"bscp_module_domain"`
}

// FeatureFlagsHandler 特性开关接口
// FeatureFlagsHandler godoc
//
//	@Summary		获取特性开关及资源限制配置
//	@Description	根据业务ID（biz）获取该业务下的特性开关以及资源配额限制。
//	@Tags			系统相关
//	@Accept			json
//	@Produce		json
//	@Param			biz	query		string								false	"业务ID/业务标识"
//	@Success		200	{object}	rest.OKResponse{data=FeatureFlags}	"成功获取特性开关列表"
//	@Router			/api/v1/inner/feature_flags [get]
//	@ID				get_feature_flags
func (p *proxy) FeatureFlagsHandler(w http.ResponseWriter, r *http.Request) {
	featureFlags := FeatureFlags{}

	biz := r.URL.Query().Get("biz")
	// set biz_view feature flag
	bizViewConf := cc.ApiServer().FeatureFlags.BizView
	featureFlags.BizView = *bizViewConf.Default
	if enable, ok := bizViewConf.Spec[biz]; ok && enable != nil {
		featureFlags.BizView = *enable
	}
	// set biz resource limit
	resourceLimitConf := cc.ApiServer().FeatureFlags.ResourceLimit
	featureFlags.ResourceLimit = resourceLimitConf.Default

	if resource, ok := resourceLimitConf.Spec[biz]; ok {
		if resource.MaxFileSize != 0 {
			featureFlags.ResourceLimit.MaxFileSize = resource.MaxFileSize
		}
		if resource.AppConfigCnt != 0 {
			featureFlags.ResourceLimit.AppConfigCnt = resource.AppConfigCnt
		}
		if resource.TmplSetTmplCnt != 0 {
			featureFlags.ResourceLimit.TmplSetTmplCnt = resource.TmplSetTmplCnt
		}
		if resource.MaxUploadContentLength != 0 {
			featureFlags.ResourceLimit.MaxUploadContentLength = resource.MaxUploadContentLength
		}
		// NOCC:golint/todo(忽略)
		// nolint TODO：其他资源限制
	}

	featureFlags.TrpcGoPlugin = TrpcGoPlugin(cc.ApiServer().FeatureFlags.TrpcGoPlugin)
	featureFlags.EnableTenantMode = cc.ApiServer().FeatureFlags.EnableTenantMode
	featureFlags.TenantMode = string(cc.ApiServer().FeatureFlags.TenantMode)

	// set process_config_view feature flag from DB via config-server gRPC
	featureFlags.ProcessConfigView = false
	if biz != "" {
		kt := kit.MustGetKit(r.Context())
		resp, grpcErr := p.cfgClient.ManageConfigKV(kt.RpcCtx(),
			&pbcs.ManageConfigKVReq{Action: "get", Key: "pcv_biz"})
		if grpcErr != nil {
			logs.Errorf("query process config view failed: %v", grpcErr)
		} else if len(resp.Items) > 0 {
			for _, id := range strings.Split(resp.Items[0].Value, ",") {
				if strings.TrimSpace(id) == biz {
					featureFlags.ProcessConfigView = true
					break
				}
			}
		}
	}

	render.Render(w, r, rest.OKRender(featureFlags))
}
