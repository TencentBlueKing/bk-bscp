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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/render"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/repository"
	"github.com/TencentBlueKing/bk-bscp/internal/iam/auth"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// stubProvider 嵌入 repository.Provider 接口, 仅覆写 handler 依赖的 Metadata/DownloadLink,
// 其余方法保持接口零值(不会被调用)。
type stubProvider struct {
	repository.Provider

	metaErr          error
	links            []string
	linkErr          error
	downloadLinkCall int
}

func (s *stubProvider) Metadata(kt *kit.Kit, sign string) (*repository.ObjectMetadata, error) {
	if s.metaErr != nil {
		return nil, s.metaErr
	}
	return &repository.ObjectMetadata{Sha256: sign}, nil
}

func (s *stubProvider) DownloadLink(kt *kit.Kit, sign string, fetchLimit uint32) ([]string, error) {
	s.downloadLinkCall++
	if s.linkErr != nil {
		return nil, s.linkErr
	}
	return s.links, nil
}

// stubAuthorizer 嵌入 auth.Authorizer 接口, 仅覆写 Authorize; 其余方法不会被 handler 调用。
type stubAuthorizer struct {
	auth.Authorizer

	authErr error
}

func (s *stubAuthorizer) Authorize(kt *kit.Kit, res ...*meta.ResourceAttribute) error {
	return s.authErr
}

const validSign = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 64 位

func newDownloadURLReq(sign string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/biz/2/content/download_url", nil)
	if sign != "" {
		req.Header.Set(constant.ContentIDHeaderKey, sign)
	}
	kt := &kit.Kit{Ctx: context.Background(), BizID: 2, AppID: 3}
	ctx := kit.WithKit(req.Context(), kt)
	ctx = context.WithValue(ctx, render.ContentTypeCtxKey, render.ContentTypeJSON)
	return req.WithContext(ctx)
}

type downloadURLEnvelope struct {
	Data DownloadURLResponse `json:"data"`
}

// TestDownloadFileURL_OK 正常: Metadata 命中 + DownloadLink 返回单条 URL, 响应只含 URL 与有效期。
func TestDownloadFileURL_OK(t *testing.T) {
	provider := &stubProvider{links: []string{"https://x/y"}}
	svc := &repoService{authorizer: &stubAuthorizer{}, provider: provider}

	rr := httptest.NewRecorder()
	svc.DownloadFileURL(rr, newDownloadURLReq(validSign))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var env downloadURLEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode body: %v, body=%s", err, rr.Body.String())
	}
	if env.Data.DownloadURL != "https://x/y" {
		t.Fatalf("expected download_url=https://x/y, got %q", env.Data.DownloadURL)
	}
	if env.Data.ExpireSeconds != repository.TempDownloadURLExpireSeconds {
		t.Fatalf("expected expire_seconds=%d, got %d",
			repository.TempDownloadURLExpireSeconds, env.Data.ExpireSeconds)
	}
}

// TestDownloadFileURL_SignMissing sign 缺失 → GetFileSign 报错 → 400, 不调用 DownloadLink。
func TestDownloadFileURL_SignMissing(t *testing.T) {
	provider := &stubProvider{links: []string{"https://x/y"}}
	svc := &repoService{authorizer: &stubAuthorizer{}, provider: provider}

	rr := httptest.NewRecorder()
	svc.DownloadFileURL(rr, newDownloadURLReq(""))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing sign, got %d", rr.Code)
	}
	if provider.downloadLinkCall != 0 {
		t.Fatalf("DownloadLink must not be called when sign invalid")
	}
}

// TestDownloadFileURL_SignInvalid sign 非法(非 64 位) → 400。
func TestDownloadFileURL_SignInvalid(t *testing.T) {
	provider := &stubProvider{links: []string{"https://x/y"}}
	svc := &repoService{authorizer: &stubAuthorizer{}, provider: provider}

	rr := httptest.NewRecorder()
	svc.DownloadFileURL(rr, newDownloadURLReq("bad-sign"))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid sign, got %d", rr.Code)
	}
}

// TestDownloadFileURL_ContentNotUploaded 内容未上传 → 报错且不调用 DownloadLink, 不泄露 URL。
func TestDownloadFileURL_ContentNotUploaded(t *testing.T) {
	provider := &stubProvider{metaErr: errf.ErrFileContentNotFound, links: []string{"https://x/y"}}
	svc := &repoService{authorizer: &stubAuthorizer{}, provider: provider}

	rr := httptest.NewRecorder()
	svc.DownloadFileURL(rr, newDownloadURLReq(validSign))

	if rr.Code == http.StatusOK {
		t.Fatalf("expected error status on content not uploaded, got 200")
	}
	if provider.downloadLinkCall != 0 {
		t.Fatalf("DownloadLink must not be called when content not uploaded (AC-T01)")
	}
	if strings.Contains(rr.Body.String(), "https://x/y") {
		t.Fatalf("must not leak download url when content not uploaded")
	}
}

// TestDownloadFileURL_DownloadLinkErr DownloadLink 返回 error → 400。
func TestDownloadFileURL_DownloadLinkErr(t *testing.T) {
	provider := &stubProvider{linkErr: errors.New("boom")}
	svc := &repoService{authorizer: &stubAuthorizer{}, provider: provider}

	rr := httptest.NewRecorder()
	svc.DownloadFileURL(rr, newDownloadURLReq(validSign))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on DownloadLink error, got %d", rr.Code)
	}
}

// TestDownloadFileURL_MultiLinks 多副本 → 取首个 URL, 有效期 3600。
func TestDownloadFileURL_MultiLinks(t *testing.T) {
	provider := &stubProvider{links: []string{"u1", "u2"}}
	svc := &repoService{authorizer: &stubAuthorizer{}, provider: provider}

	rr := httptest.NewRecorder()
	svc.DownloadFileURL(rr, newDownloadURLReq(validSign))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}
	var env downloadURLEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if env.Data.DownloadURL != "u1" {
		t.Fatalf("expected first url u1, got %q", env.Data.DownloadURL)
	}
	if env.Data.ExpireSeconds != repository.TempDownloadURLExpireSeconds {
		t.Fatalf("expected expire_seconds=%d, got %d",
			repository.TempDownloadURLExpireSeconds, env.Data.ExpireSeconds)
	}
}

// TestDownloadFileURL_EmptyLinks 空切片 → 报错(防越界), 不 panic。
func TestDownloadFileURL_EmptyLinks(t *testing.T) {
	provider := &stubProvider{links: []string{}}
	svc := &repoService{authorizer: &stubAuthorizer{}, provider: provider}

	rr := httptest.NewRecorder()
	svc.DownloadFileURL(rr, newDownloadURLReq(validSign))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty links, got %d", rr.Code)
	}
}

// TestDownloadFileURL_Unauthorized 鉴权失败 → 非 200, 不调用 DownloadLink, 不泄露 URL(AC-S01)。
func TestDownloadFileURL_Unauthorized(t *testing.T) {
	provider := &stubProvider{links: []string{"https://x/y"}}
	svc := &repoService{authorizer: &stubAuthorizer{authErr: errors.New("no permission")}, provider: provider}

	rr := httptest.NewRecorder()
	svc.DownloadFileURL(rr, newDownloadURLReq(validSign))

	if rr.Code == http.StatusOK {
		t.Fatalf("expected non-200 on authorize failure, got 200")
	}
	if provider.downloadLinkCall != 0 {
		t.Fatalf("DownloadLink must not be called when unauthorized")
	}
	if strings.Contains(rr.Body.String(), "https://x/y") {
		t.Fatalf("must not leak download url when unauthorized")
	}
}
