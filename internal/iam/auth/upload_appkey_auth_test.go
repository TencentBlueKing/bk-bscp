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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// setBaseAppCred 临时设置 cc.G().BaseConf 的 app 凭证, 测试结束重置
func setBaseAppCred(t *testing.T, code, secret string) {
	t.Helper()
	settings := cc.GlobalSettings{}
	settings.BaseConf.AppCode = code
	settings.BaseConf.AppSecret = secret
	cc.SetG(settings)
	t.Cleanup(func() {
		cc.SetG(cc.GlobalSettings{})
	})
}

// TestUploadAppKeyAuthentication_Match 正确 app 凭证应放行并填充 kit
func TestUploadAppKeyAuthentication_Match(t *testing.T) {
	setBaseAppCred(t, "demo_app", "demo_secret")

	var nextCalled bool
	var gotAppCode string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		nextCalled = true
		kt := kit.MustGetKit(r.Context())
		gotAppCode = kt.AppCode
	})

	a := authorizer{}
	h := a.UploadAppKeyAuthentication(next)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/biz/2/content/upload", nil)
	req.Header.Set("X-Bkapi-Authorization", `{"bk_app_code":"demo_app","bk_app_secret":"demo_secret"}`)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !nextCalled {
		t.Fatalf("expected next handler to be called on valid credential")
	}
	if gotAppCode != "demo_app" {
		t.Fatalf("expected kit.AppCode=demo_app, got %q", gotAppCode)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

// TestUploadAppKeyAuthentication_Mismatch 错误 app 凭证应拒绝且不进入下游
func TestUploadAppKeyAuthentication_Mismatch(t *testing.T) {
	setBaseAppCred(t, "demo_app", "demo_secret")

	var nextCalled bool
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	})

	a := authorizer{}
	h := a.UploadAppKeyAuthentication(next)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/biz/2/content/upload", nil)
	req.Header.Set("X-Bkapi-Authorization", `{"bk_app_code":"demo_app","bk_app_secret":"wrong_secret"}`)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if nextCalled {
		t.Fatalf("expected next handler NOT to be called on invalid credential")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}
