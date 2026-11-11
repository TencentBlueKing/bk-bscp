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

package apigw

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/TencentBlueKing/bk-bscp/docs"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

func loadEmbeddedMcpServers(t *testing.T) []McpServer {
	t.Helper()
	data, err := docs.Assets.ReadFile(mcpServersFile)
	if err != nil {
		t.Fatalf("read %s: %v", mcpServersFile, err)
	}
	servers, err := loadMcpServers(data)
	if err != nil {
		t.Fatalf("load mcp servers: %v", err)
	}
	return servers
}

// swaggerOperations 返回 operationId 到请求参数个数的映射
func swaggerOperations(t *testing.T) map[string]int {
	t.Helper()
	data, err := docs.Assets.ReadFile("swagger/bkapigw/swagger.json")
	if err != nil {
		t.Fatalf("read swagger: %v", err)
	}
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal swagger: %v", err)
	}

	ops := make(map[string]int)
	for _, methods := range doc.Paths {
		for _, raw := range methods {
			var op struct {
				OperationID string            `json:"operationId"`
				Parameters  []json.RawMessage `json:"parameters"`
			}
			if json.Unmarshal(raw, &op) != nil || op.OperationID == "" {
				continue
			}
			ops[op.OperationID] = len(op.Parameters)
		}
	}
	return ops
}

func TestEmbeddedMcpServers(t *testing.T) {
	servers := loadEmbeddedMcpServers(t)
	if err := validateMcpServers(servers, cc.DefaultName, cc.DefaultEnv); err != nil {
		t.Fatalf("validate mcp servers: %v", err)
	}

	ops := swaggerOperations(t)
	for _, s := range servers {
		for _, r := range s.ResourceNames {
			params, ok := ops[r]
			if !ok {
				t.Errorf("mcp server %s: resource %s not found in swagger", s.Name, r)
				continue
			}
			if params == 0 {
				t.Errorf("mcp server %s: resource %s has no request parameters", s.Name, r)
			}
		}
	}

	counts := map[string]int{}
	for _, s := range servers {
		counts[s.Name] = len(s.ResourceNames)
	}
	want := map[string]int{"kv-config": 30, "file-config": 25}
	if !reflect.DeepEqual(counts, want) {
		t.Errorf("mcp tool counts = %v, want %v", counts, want)
	}
	if got := len(mcpResourceNames(servers)); got != 45 {
		t.Errorf("mcp resource union = %d, want 45", got)
	}
}

func validMcpServer() McpServer {
	isPublic, status := true, 1
	return McpServer{
		Name:          "kv-config",
		Description:   "desc",
		CategoryNames: []string{mcpCategoryOfficial},
		ResourceNames: []string{"Config_ListKvs"},
		IsPublic:      &isPublic,
		Status:        &status,
		ProtocolType:  "streamable_http",
	}
}

func TestValidateMcpServers(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(s []McpServer) []McpServer
		wantErr string
	}{
		{name: "valid", mutate: func(s []McpServer) []McpServer { return s }},
		{name: "empty", mutate: func([]McpServer) []McpServer { return nil }, wantErr: "no mcp server"},
		{name: "bad name", mutate: func(s []McpServer) []McpServer { s[0].Name = "KV_config"; return s },
			wantErr: "must match"},
		{name: "duplicated name", mutate: func(s []McpServer) []McpServer { return append(s, s[0]) },
			wantErr: "duplicated"},
		{name: "missing description", mutate: func(s []McpServer) []McpServer { s[0].Description = ""; return s },
			wantErr: "description"},
		{name: "missing status", mutate: func(s []McpServer) []McpServer { s[0].Status = nil; return s },
			wantErr: "status"},
		{name: "missing is_public", mutate: func(s []McpServer) []McpServer { s[0].IsPublic = nil; return s },
			wantErr: "is_public"},
		{name: "missing protocol_type", mutate: func(s []McpServer) []McpServer { s[0].ProtocolType = ""; return s },
			wantErr: "protocol_type"},
		{name: "not official", mutate: func(s []McpServer) []McpServer { s[0].CategoryNames = nil; return s },
			wantErr: mcpCategoryOfficial},
		{name: "duplicated resource", mutate: func(s []McpServer) []McpServer {
			s[0].ResourceNames = []string{"Config_ListKvs", "Config_ListKvs"}
			return s
		}, wantErr: "duplicated"},
		{name: "tool name too long", mutate: func(s []McpServer) []McpServer {
			s[0].ResourceNames = []string{strings.Repeat("a", 43)}
			return s
		}, wantErr: "exceeds"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateMcpServers(c.mutate([]McpServer{validMcpServer()}), "bk-bscp", "prod")
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("err = %v, want contains %q", err, c.wantErr)
			}
		})
	}
}

type recordedRequest struct {
	method string
	path   string
	auth   string
	tenant string
	body   []byte
}

type fakeGateway struct {
	mu       sync.Mutex
	requests []recordedRequest
	status   int
}

func (f *fakeGateway) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.requests = append(f.requests, recordedRequest{
		method: r.Method,
		path:   r.URL.Path,
		auth:   r.Header.Get("X-Bkapi-Authorization"),
		tenant: r.Header.Get("X-Bk-Tenant-Id"),
		body:   body,
	})
	f.mu.Unlock()

	if f.status != 0 {
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(`{"error":{"code":"INVALID_ARGUMENT","message":"bad request"}}`))
		return
	}
	if strings.HasSuffix(r.URL.Path, "/mcp-servers/") {
		_, _ = w.Write([]byte(`{"data":[{"id":1,"name":"bk-bscp-prod-kv-config","action":"create"}]}`))
		return
	}
	_, _ = w.Write([]byte(`{"data":{}}`))
}

func newTestApiGw(t *testing.T, f *fakeGateway) (ApiGw, cc.Esb) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(srv.Close)

	esbOpt := cc.Esb{AppCode: "bk-bscp", AppSecret: "secret"}
	gw, err := NewApiGw(esbOpt, cc.ApiGateway{Host: srv.URL})
	if err != nil {
		t.Fatalf("new api gw: %v", err)
	}
	return gw, esbOpt
}

func TestSyncMcpServers(t *testing.T) {
	f := &fakeGateway{}
	gw, esbOpt := newTestApiGw(t, f)

	err := syncMcpServers(gw, "bk-bscp", "prod", esbOpt, cc.ApiGatewayMcp{
		Enabled:       true,
		GrantAppCodes: []string{"bk-bscp", "agent-a", ""},
	})
	if err != nil {
		t.Fatalf("sync mcp servers: %v", err)
	}

	wantApps := []string{"bk-bscp", "agent-a"}
	if len(f.requests) != 1+len(wantApps) {
		t.Fatalf("request count = %d, want %d", len(f.requests), 1+len(wantApps))
	}
	for _, r := range f.requests {
		if r.method != http.MethodPost {
			t.Errorf("%s method = %s, want POST", r.path, r.method)
		}
		if !strings.Contains(r.auth, `"bk_app_code": "bk-bscp"`) || !strings.Contains(r.auth, `"bk_app_secret": "secret"`) {
			t.Errorf("%s X-Bkapi-Authorization = %s", r.path, r.auth)
		}
		if r.tenant != "system" {
			t.Errorf("%s X-Bk-Tenant-Id = %s, want system", r.path, r.tenant)
		}
	}

	syncReq := f.requests[0]
	if syncReq.path != "/api/v2/sync/gateways/bk-bscp/stages/prod/mcp-servers/" {
		t.Fatalf("sync path = %s", syncReq.path)
	}
	var gotSync SyncStageMcpServersReq
	if err = json.Unmarshal(syncReq.body, &gotSync); err != nil {
		t.Fatalf("unmarshal sync body: %v", err)
	}
	if gotSync.Name != "prod" || len(gotSync.McpServers) != 2 {
		t.Fatalf("sync body name = %s, servers = %d", gotSync.Name, len(gotSync.McpServers))
	}
	for _, s := range gotSync.McpServers {
		if !reflect.DeepEqual(s.TargetAppCodes, wantApps) {
			t.Errorf("mcp server %s target_app_codes = %v, want %v", s.Name, s.TargetAppCodes, wantApps)
		}
	}

	wantResources := mcpResourceNames(loadEmbeddedMcpServers(t))
	for i, app := range wantApps {
		r := f.requests[1+i]
		if r.path != "/api/v2/sync/gateways/bk-bscp/permissions/grant/" {
			t.Fatalf("grant path = %s", r.path)
		}
		var got GrantPermissionsReq
		if err = json.Unmarshal(r.body, &got); err != nil {
			t.Fatalf("unmarshal grant body: %v", err)
		}
		want := GrantPermissionsReq{TargetAppCode: app, GrantDimension: "resource", ResourceNames: wantResources}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("grant body = %+v, want %+v", got, want)
		}
	}
}

func TestSyncMcpServersDisabled(t *testing.T) {
	f := &fakeGateway{}
	gw, esbOpt := newTestApiGw(t, f)

	if err := syncMcpServers(gw, "bk-bscp", "prod", esbOpt, cc.ApiGatewayMcp{Enabled: false}); err != nil {
		t.Fatalf("sync mcp servers: %v", err)
	}
	if len(f.requests) != 0 {
		t.Fatalf("request count = %d, want 0", len(f.requests))
	}
}

func TestSyncMcpServersGatewayError(t *testing.T) {
	f := &fakeGateway{status: http.StatusBadRequest}
	gw, esbOpt := newTestApiGw(t, f)

	err := syncMcpServers(gw, "bk-bscp", "prod", esbOpt, cc.ApiGatewayMcp{Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "status code: 400") {
		t.Fatalf("err = %v, want status code 400", err)
	}
	if len(f.requests) != 1 {
		t.Fatalf("request count = %d, want 1 (grant must not run after sync failure)", len(f.requests))
	}
}
