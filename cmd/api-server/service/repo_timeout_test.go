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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/render"

	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

func newJSONReq() *http.Request {
	req := httptest.NewRequest(http.MethodPut, "/api/v1/biz/2/content/upload", nil)
	return req.WithContext(context.WithValue(req.Context(), render.ContentTypeCtxKey, render.ContentTypeJSON))
}

// TestRenderRepoErr_Timeout context 超时应返回 504
func TestRenderRepoErr_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond) // 确保 deadline 已过

	kt := &kit.Kit{Ctx: ctx}
	rr := httptest.NewRecorder()
	renderRepoErr(rr, newJSONReq(), kt, errors.New("upstream boom"))

	if rr.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504 on deadline exceeded, got %d", rr.Code)
	}
}

// TestRenderRepoErr_Canceled provider 返回 context.Canceled 应返回 504
func TestRenderRepoErr_Canceled(t *testing.T) {
	kt := &kit.Kit{Ctx: context.Background()}
	rr := httptest.NewRecorder()
	renderRepoErr(rr, newJSONReq(), kt, context.Canceled)

	if rr.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504 on canceled, got %d", rr.Code)
	}
}

// TestRenderRepoErr_BadRequest 普通错误仍返回 400
func TestRenderRepoErr_BadRequest(t *testing.T) {
	kt := &kit.Kit{Ctx: context.Background()}
	rr := httptest.NewRecorder()
	renderRepoErr(rr, newJSONReq(), kt, errors.New("invalid sign"))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on normal error, got %d", rr.Code)
	}
}
