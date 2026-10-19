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

package components

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
)

func TestRestyReqToCurlMasksAuthentication(t *testing.T) {
	req := resty.New().R()
	req.Method = http.MethodGet
	req.Header.Set("X-Bkapi-Authorization", "private-gateway-credential")
	req.Header.Set("Authorization", "private-bearer-credential")
	req.Header.Set("Cookie", "private-session-credential")
	req.Header.Set("X-Request-Id", "visible-request-id")
	req.RawRequest, _ = http.NewRequest(http.MethodGet, "https://example.com", nil)
	got := restyReqToCurl(req)
	for _, secret := range []string{"private-gateway-credential", "private-bearer-credential", "private-session-credential"} {
		if strings.Contains(got, secret) {
			t.Fatalf("request log exposed %s", secret)
		}
	}
	if !strings.Contains(got, "visible-request-id") {
		t.Fatal("request ID missing")
	}
	if req.Header.Get("X-Bkapi-Authorization") != "private-gateway-credential" {
		t.Fatal("authentication header modified")
	}
}

func TestSharedClientsConcurrentInitialization(t *testing.T) {
	var wg sync.WaitGroup
	clients := make(chan *resty.Client, 32)
	polling := make(chan *resty.Client, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); clients <- GetClient(); polling <- GetLongPollingClient(45 * time.Second) }()
	}
	wg.Wait()
	close(clients)
	close(polling)
	for client := range clients {
		if client != GetClient() {
			t.Fatal("global client not shared")
		}
	}
	for client := range polling {
		if client != GetLongPollingClient(45*time.Second) {
			t.Fatal("polling client not shared")
		}
		if client == GetClient() {
			t.Fatal("polling changed global client")
		}
	}
	if GetClient().GetClient().Timeout != timeout {
		t.Fatal("global timeout changed")
	}
}

func TestRefineCode(t *testing.T) {
	code, err := refineCode("00")
	assert.NoError(t, err)
	assert.True(t, code == 0)
}
