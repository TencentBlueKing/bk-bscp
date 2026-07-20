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

package bscp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseFrontendVariant(t *testing.T) {
	cases := []struct {
		in      string
		want    FrontendVariant
		wantErr bool
	}{
		{in: "", want: FrontendNew},
		{in: "new", want: FrontendNew},
		{in: "old", want: FrontendOld},
		{in: "invalid", wantErr: true},
	}

	for _, c := range cases {
		got, err := ParseFrontendVariant(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseFrontendVariant(%q) expected error, got nil", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseFrontendVariant(%q) unexpected err: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseFrontendVariant(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNewEmbedWebWithVariantSelectsAssets(t *testing.T) {
	renderBody := func(v FrontendVariant) string {
		web := NewEmbedWebWithVariant(v)
		rr := httptest.NewRecorder()
		web.RenderIndexHandler(&IndexConfig{}).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		return rr.Body.String()
	}

	if body := renderBody(FrontendNew); !strings.Contains(body, "new-ui") {
		t.Errorf("new variant index body = %q, want contain %q", body, "new-ui")
	}
	if body := renderBody(FrontendOld); !strings.Contains(body, "old-ui") {
		t.Errorf("old variant index body = %q, want contain %q", body, "old-ui")
	}
}

// TestNewEmbedWebDefaultsToNew 确保保留无参构造的向后兼容行为（默认新 UI）。
func TestNewEmbedWebDefaultsToNew(t *testing.T) {
	web := NewEmbedWeb()
	rr := httptest.NewRecorder()
	web.RenderIndexHandler(&IndexConfig{}).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if body := rr.Body.String(); !strings.Contains(body, "new-ui") {
		t.Errorf("NewEmbedWeb index body = %q, want contain %q", body, "new-ui")
	}
}
