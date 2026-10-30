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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbas "github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server"
)

// fakeVerifyClient 模拟 auth-server 的 IAMVerify，返回固定的下游错误或判定结果。
type fakeVerifyClient struct {
	pbas.AuthClient

	err        error
	authorized bool
}

func (f *fakeVerifyClient) IAMVerify(_ context.Context, _ *pbas.IAMVerifyReq,
	_ ...grpc.CallOption) (*pbas.IAMVerifyResp, error) {

	if f.err != nil {
		return nil, f.err
	}

	return &pbas.IAMVerifyResp{IsAuthorized: f.authorized}, nil
}

// 回调鉴权失败时响应必须是稳定文案：下游错误（auth-server 地址、连接细节）只落日志，
// 不能顺着响应体回给权限中心。
func TestIAMVerifyHidesDownstreamError(t *testing.T) {
	downstream := errors.New(
		"rpc error: code = Unavailable desc = dial tcp 10.0.0.1:9513: connect: connection refused")
	a := authorizer{authClient: &fakeVerifyClient{err: downstream}}

	nextCalled := false
	h := a.IAMVerify(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { nextCalled = true }))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/iam/find/resource", nil)
	req.SetBasicAuth("bk_iam", "token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.False(t, nextCalled, "failed verification must not reach the next handler")
	require.Equal(t, http.StatusBadRequest, rr.Code)

	body := rr.Body.String()
	require.Contains(t, body, "authorized failed")
	require.NotContains(t, body, "9513")
	require.NotContains(t, body, "10.0.0.1")
	require.NotContains(t, body, "connection refused")
}

// 凭证不匹配与下游错误返回同一句文案，调用方无法据此区分内部状态。
func TestIAMVerifyRejectsBadCredentialWithStableMessage(t *testing.T) {
	a := authorizer{authClient: &fakeVerifyClient{}}

	h := a.IAMVerify(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("request with bad credential must not pass")
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/iam/find/resource", nil)
	// 回调的 Basic 用户名 V3 是 iam、V4 是 bk_iam，其余一律拒绝。
	req.SetBasicAuth("other", "token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), "authorized failed")
}
