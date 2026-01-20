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

package pushmanager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"
)

func getPushProviderConfig(t *testing.T) cc.PushProviderConfig {
	bcsHost := os.Getenv("BCS_HOST")
	if bcsHost == "" {
		t.Skip("Skipping test: BCS_HOST environment variable not set")
	}
	token := os.Getenv("TOKEN")
	if token == "" {
		t.Skip("Skipping test: TOKEN environment variable not set")
	}
	domain := os.Getenv("DOMAIN")
	if domain == "" {
		t.Skip("Skipping test: DOMAIN environment variable not set")
	}
	cfg := cc.PushProviderConfig{
		Host:       bcsHost,
		Token:      token,
		Domain:     domain,
		PushType:   "mail",
		MailSuffix: "xxx", // 替换为合适的邮件后缀
	}

	t.Logf("BCS_HOST: %s, TOKEN: %s, DOMAIN: %s", bcsHost, token, domain)

	return cfg
}

// mock 通用响应
func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func TestCreatePushEvent_HTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/pushmanager/api/v1/domains/bscp/push_events", r.URL.Path)

		var req CreatePushEventRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		require.Equal(t, "test-event", req.Event.EventID)

		writeJSON(w, http.StatusOK, map[string]any{
			"code":     0,
			"message":  "Success",
			"event_id": "test-event",
		})
	}))
	defer srv.Close()

	svc := &pushManagerService{
		PushProviderConfig: getPushProviderConfig(t),
	}

	resp, err := svc.CreatePushEvent(context.Background(), &CreatePushEventRequest{
		Event: PushEvent{
			Domain: "bscp",
			EventDetail: PushEventDetail{
				Fields: PushEventFields{
					Types:         "mail",
					MailReceivers: "xxx",
					MailTitle:     "test title",
					MailContent:   "test content",
				},
			},
			Dimension: &Dimension{
				Fields: map[string]string{
					"cluster_id": "xxx",
					"namespace":  "xxx",
				},
			},
		},
	})

	require.NoError(t, err)
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestListPushEvents_HTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/pushmanager/api/v1/domains/bscp/push_events", r.URL.Path)
		require.Equal(t, "1", r.URL.Query().Get("page"))
		require.Equal(t, "100", r.URL.Query().Get("page_size"))

		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "Success",
			"events":  []any{},
			"total":   0,
		})
	}))
	defer srv.Close()

	svc := &pushManagerService{
		PushProviderConfig: getPushProviderConfig(t),
	}

	resp, err := svc.ListPushEvents(context.Background(), "bscp", &ListPushEventsRequest{})
	require.NoError(t, err)
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestDeletePushEvent_HTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/pushmanager/api/v1/domains/bscp/push_events/event-1", r.URL.Path)

		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "Success",
		})
	}))
	defer srv.Close()

	svc := &pushManagerService{
		PushProviderConfig: getPushProviderConfig(t),
	}

	_, err := svc.DeletePushEvent(context.Background(), "bscp", "event-1")
	require.NoError(t, err)
}

func TestUpdatePushEvent_HTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/pushmanager/api/v1/domains/bscp/push_events/event-1", r.URL.Path)

		var body UpdatePushEventRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)

		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "Success",
		})
	}))
	defer srv.Close()

	svc := &pushManagerService{
		PushProviderConfig: getPushProviderConfig(t),
	}

	_, err := svc.UpdatePushEvent(
		context.Background(),
		"bscp",
		"event-1",
		&UpdatePushEventRequest{
			Event: PushEvent{
				Status: 1,
			},
		},
	)

	require.NoError(t, err)
}
