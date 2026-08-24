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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v4"

	"github.com/TencentBlueKing/bk-bscp/internal/runtime/gwparser"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// testGatewayRid kit.Validate 要求 rid 长度在 16~48 之间
const testGatewayRid = "0123456789abcdef0123"

// newTestJWTParser 生成一对 RSA 密钥, 返回基于公钥的解析器与用私钥签发 token 的函数
func newTestJWTParser(t *testing.T) (gwparser.Parser, func(jwt.MapClaims) string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}

	parser, err := gwparser.NewJWTParser(string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})))
	if err != nil {
		t.Fatalf("new jwt parser: %v", err)
	}

	sign := func(claims jwt.MapClaims) string {
		token, sErr := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
		if sErr != nil {
			t.Fatalf("sign jwt: %v", sErr)
		}
		return token
	}

	return parser, sign
}

// gatewayClaims 构造网关 JWT claims; userVerified=false 模拟网关免用户认证场景
func gatewayClaims(appCode, userName string, userVerified bool) jwt.MapClaims {
	return jwt.MapClaims{
		"app":  map[string]any{"app_code": appCode, "verified": true},
		"user": map[string]any{"username": userName, "verified": userVerified},
	}
}

// newGatewayRequest 构造一个携带网关 JWT 的 manage_config_kv 请求
func newGatewayRequest(token string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/manage_config_kv", nil)
	req.Header.Set("X-Bkapi-JWT", token)
	req.Header.Set("X-Bkapi-Request-Id", testGatewayRid)
	return req
}

// TestPlatformAppKeyAuthentication_GatewayJWTMatch 网关 JWT 中 app_code 为平台自身时应放行
func TestPlatformAppKeyAuthentication_GatewayJWTMatch(t *testing.T) {
	setBaseAppCred(t, "demo_app", "demo_secret")
	parser, sign := newTestJWTParser(t)

	var gotKit *kit.Kit
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotKit = kit.MustGetKit(r.Context())
	})

	a := authorizer{gwParser: parser}
	rr := httptest.NewRecorder()
	a.PlatformAppKeyAuthentication(next).ServeHTTP(rr, newGatewayRequest(sign(gatewayClaims("demo_app", "", false))))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if gotKit == nil {
		t.Fatalf("expected next handler to be called on platform app jwt")
	}
	if gotKit.AppCode != "demo_app" {
		t.Fatalf("expected kit.AppCode=demo_app, got %q", gotKit.AppCode)
	}
}

// TestPlatformAppKeyAuthentication_GatewayJWTOtherApp 其他应用经网关调用应拒绝
func TestPlatformAppKeyAuthentication_GatewayJWTOtherApp(t *testing.T) {
	setBaseAppCred(t, "demo_app", "demo_secret")
	parser, sign := newTestJWTParser(t)

	var nextCalled bool
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	})

	a := authorizer{gwParser: parser}
	rr := httptest.NewRecorder()
	a.PlatformAppKeyAuthentication(next).ServeHTTP(rr, newGatewayRequest(sign(gatewayClaims("other_app", "", false))))

	if nextCalled {
		t.Fatalf("expected next handler NOT to be called for non-platform app_code")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

// TestPlatformAppKeyAuthentication_GatewayJWTBadSignature 非本网关签发的 JWT 应拒绝
func TestPlatformAppKeyAuthentication_GatewayJWTBadSignature(t *testing.T) {
	setBaseAppCred(t, "demo_app", "demo_secret")
	parser, _ := newTestJWTParser(t)
	_, signWithOtherKey := newTestJWTParser(t)

	var nextCalled bool
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	})

	a := authorizer{gwParser: parser}
	rr := httptest.NewRecorder()
	token := signWithOtherKey(gatewayClaims("demo_app", "", false))
	a.PlatformAppKeyAuthentication(next).ServeHTTP(rr, newGatewayRequest(token))

	if nextCalled {
		t.Fatalf("expected next handler NOT to be called on jwt signed by unknown key")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

// TestPlatformAppKeyAuthentication_GatewayJWTUserName 免用户认证时, 调用方自报的用户名应透传到 kit
func TestPlatformAppKeyAuthentication_GatewayJWTUserName(t *testing.T) {
	setBaseAppCred(t, "demo_app", "demo_secret")
	parser, sign := newTestJWTParser(t)

	var gotUser string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotUser = kit.MustGetKit(r.Context()).User
	})

	req := newGatewayRequest(sign(gatewayClaims("demo_app", "", false)))
	req.Header.Set("X-Bkapi-User-Name", "alice")

	a := authorizer{gwParser: parser}
	rr := httptest.NewRecorder()
	a.PlatformAppKeyAuthentication(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if gotUser != "alice" {
		t.Fatalf("expected kit.User=alice, got %q", gotUser)
	}
}

// TestPlatformAppKeyAuthentication_GatewayJWTNoUserName 未自报用户名时不影响放行
func TestPlatformAppKeyAuthentication_GatewayJWTNoUserName(t *testing.T) {
	setBaseAppCred(t, "demo_app", "demo_secret")
	parser, sign := newTestJWTParser(t)

	var nextCalled bool
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	})

	a := authorizer{gwParser: parser}
	rr := httptest.NewRecorder()
	a.PlatformAppKeyAuthentication(next).ServeHTTP(rr, newGatewayRequest(sign(gatewayClaims("demo_app", "", false))))

	if !nextCalled || rr.Code != http.StatusOK {
		t.Fatalf("expected request to pass without user name, got status %d", rr.Code)
	}
}

// TestPlatformAppKeyAuthentication_GatewayJWTNotConfigured 平台未配置 app_code 时应 fail-closed
func TestPlatformAppKeyAuthentication_GatewayJWTNotConfigured(t *testing.T) {
	setBaseAppCred(t, "", "")
	parser, sign := newTestJWTParser(t)

	var nextCalled bool
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	})

	a := authorizer{gwParser: parser}
	rr := httptest.NewRecorder()
	a.PlatformAppKeyAuthentication(next).ServeHTTP(rr, newGatewayRequest(sign(gatewayClaims("demo_app", "", false))))

	if nextCalled {
		t.Fatalf("expected next handler NOT to be called when platform app_code unconfigured")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

// TestPlatformAppKeyAuthentication_RejectsDirectAppKey 该接口只经网关暴露, 直连 app 凭证即便正确也应拒绝
func TestPlatformAppKeyAuthentication_RejectsDirectAppKey(t *testing.T) {
	setBaseAppCred(t, "demo_app", "demo_secret")
	parser, _ := newTestJWTParser(t)

	var nextCalled bool
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/manage_config_kv", nil)
	req.Header.Set("X-Bkapi-Authorization", `{"bk_app_code":"demo_app","bk_app_secret":"demo_secret"}`)

	a := authorizer{gwParser: parser}
	rr := httptest.NewRecorder()
	a.PlatformAppKeyAuthentication(next).ServeHTTP(rr, req)

	if nextCalled {
		t.Fatalf("expected next handler NOT to be called for direct app credential")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

// TestPlatformAppKeyAuthentication_NoCredential 未携带任何凭证应直接拒绝, 不回退 Cookie
func TestPlatformAppKeyAuthentication_NoCredential(t *testing.T) {
	setBaseAppCred(t, "demo_app", "demo_secret")

	var nextCalled bool
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	})

	a := authorizer{}
	rr := httptest.NewRecorder()
	a.PlatformAppKeyAuthentication(next).
		ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/config/manage_config_kv", nil))

	if nextCalled {
		t.Fatalf("expected next handler NOT to be called without any credential")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}
