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
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeTokenSource 模拟权限中心按租户发放的系统 token，并记录拉取次数。
type fakeTokenSource struct {
	mu     sync.Mutex
	tokens map[string]string
	calls  int
}

func (f *fakeTokenSource) fetcher(tenantID string) func() (string, error) {
	return func() (string, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		f.calls++

		return f.tokens[tenantID], nil
	}
}

func newTestTokenCache() (*systemTokenCache, *time.Time) {
	now := time.Unix(1_700_000_000, 0)
	c := newSystemTokenCache(time.Minute)
	c.now = func() time.Time { return now }

	return c, &now
}

// 一个租户的 token 已在缓存中时，拿它冒充另一个租户必须被拒绝。
func TestSystemTokenCacheIsolatesTenants(t *testing.T) {
	c, _ := newTestTokenCache()
	src := &fakeTokenSource{tokens: map[string]string{"tenant-a": "token-a", "tenant-b": "token-b"}}

	ok, err := c.verify("tenant-a", "token-a", src.fetcher("tenant-a"))
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = c.verify("tenant-b", "token-a", src.fetcher("tenant-b"))
	require.NoError(t, err)
	require.False(t, ok, "token of tenant-a must not pass the callback of tenant-b")

	ok, err = c.verify("tenant-b", "token-b", src.fetcher("tenant-b"))
	require.NoError(t, err)
	require.True(t, ok)
}

// 缓存期内重复校验不再请求权限中心，过期后重新拉取。
func TestSystemTokenCacheReusesUntilExpired(t *testing.T) {
	c, now := newTestTokenCache()
	src := &fakeTokenSource{tokens: map[string]string{"t": "token"}}

	for i := 0; i < 3; i++ {
		ok, err := c.verify("t", "token", src.fetcher("t"))
		require.NoError(t, err)
		require.True(t, ok)
	}
	require.Equal(t, 1, src.calls)

	*now = now.Add(time.Minute + time.Second)
	ok, err := c.verify("t", "token", src.fetcher("t"))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 2, src.calls)
}

// token 轮换后，新 token 不必等缓存过期即可通过，旧 token 随即失效。
func TestSystemTokenCacheRefetchesOnMismatch(t *testing.T) {
	c, _ := newTestTokenCache()
	src := &fakeTokenSource{tokens: map[string]string{"t": "old"}}

	ok, err := c.verify("t", "old", src.fetcher("t"))
	require.NoError(t, err)
	require.True(t, ok)

	src.tokens["t"] = "new"
	ok, err = c.verify("t", "new", src.fetcher("t"))
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = c.verify("t", "old", src.fetcher("t"))
	require.NoError(t, err)
	require.False(t, ok)
}

// 空密码直接拒绝；权限中心返回空 token 时不能把空串当作合法凭证缓存下来。
func TestSystemTokenCacheRejectsEmpty(t *testing.T) {
	c, _ := newTestTokenCache()
	src := &fakeTokenSource{tokens: map[string]string{}}

	ok, err := c.verify("t", "", src.fetcher("t"))
	require.NoError(t, err)
	require.False(t, ok)
	require.Zero(t, src.calls, "empty password must not reach iam")

	_, err = c.verify("t", "anything", src.fetcher("t"))
	require.ErrorContains(t, err, "empty")

	_, cached := c.get("t")
	require.False(t, cached)
}

func TestSystemTokenCacheReturnsFetchError(t *testing.T) {
	c, _ := newTestTokenCache()

	_, err := c.verify("t", "token", func() (string, error) { return "", errors.New("iam unavailable") })
	require.ErrorContains(t, err, "iam unavailable")
}

// 回调会并发到达，缓存读写必须是并发安全的（配合 -race 运行）。
func TestSystemTokenCacheConcurrent(t *testing.T) {
	c := newSystemTokenCache(time.Minute)
	src := &fakeTokenSource{tokens: map[string]string{}}
	for i := 0; i < 4; i++ {
		src.tokens[fmt.Sprintf("tenant-%d", i)] = fmt.Sprintf("token-%d", i)
	}

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			tenant := fmt.Sprintf("tenant-%d", i%4)
			ok, err := c.verify(tenant, fmt.Sprintf("token-%d", i%4), src.fetcher(tenant))
			require.NoError(t, err)
			require.True(t, ok)
		}(i)
	}
	wg.Wait()
}
