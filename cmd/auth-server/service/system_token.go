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
	"crypto/subtle"
	"errors"
	"sync"
	"time"
)

// systemTokenTTL 系统 token 的本地缓存时长。
const systemTokenTTL = time.Minute

// systemTokenCache 按租户缓存权限中心的系统 token，用于校验资源回调 Basic Auth 的密码。
// system token 按租户发放，缓存必须按租户隔离：若共用一份，
// 一个租户的 token 在缓存期内会通过另一个租户的回调校验，回调随之返回后者的资源。
type systemTokenCache struct {
	ttl time.Duration
	now func() time.Time

	mu      sync.Mutex
	entries map[string]cachedToken
}

type cachedToken struct {
	token     string
	fetchedAt time.Time
}

func newSystemTokenCache(ttl time.Duration) *systemTokenCache {
	return &systemTokenCache{ttl: ttl, now: time.Now, entries: make(map[string]cachedToken)}
}

// verify 校验 password 是否为 tenantID 当前的系统 token。
// 缓存命中且一致时直接放行；未命中、过期或不一致时用 fetch 重新获取后再比较，
// 这样 token 轮换后不必等缓存过期。
func (c *systemTokenCache) verify(tenantID, password string, fetch func() (string, error)) (bool, error) {
	if password == "" {
		return false, nil
	}

	if token, ok := c.get(tenantID); ok && tokenEqual(password, token) {
		return true, nil
	}

	token, err := fetch()
	if err != nil {
		return false, err
	}

	if token == "" {
		return false, errors.New("system token is empty")
	}

	c.set(tenantID, token)

	return tokenEqual(password, token), nil
}

func (c *systemTokenCache) get(tenantID string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[tenantID]
	if !ok || c.now().Sub(entry.fetchedAt) > c.ttl {
		return "", false
	}

	return entry.token, true
}

func (c *systemTokenCache) set(tenantID, token string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[tenantID] = cachedToken{token: token, fetchedAt: c.now()}
}

// tokenEqual 用常量时间比较，避免通过响应耗时逐字节猜出 token。
func tokenEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
