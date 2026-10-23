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

package lock

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

func newTestTokenLock(t *testing.T, ttl uint) (*TokenLock, bedis.Client) {
	t.Helper()

	mr := miniredis.RunT(t)
	bds, err := bedis.NewRedisCache(cc.RedisCluster{Mode: cc.RedisStandaloneMode, Endpoints: []string{mr.Addr()}})
	if err != nil {
		t.Fatalf("new redis cache: %v", err)
	}
	return NewTokenLock(bds, ttl), bds
}

// forceExpire 把锁 key 的截止时间改到过去，模拟持有者超过 TTL 未续期
func forceExpire(t *testing.T, bds bedis.Client, res string) {
	t.Helper()

	ctx := context.Background()
	key := lockKey(res)
	raw, err := bds.Get(ctx, key)
	if err != nil {
		t.Fatalf("get lock value: %v", err)
	}
	owner, _, ok := strings.Cut(raw, lockValueSeparator)
	if !ok {
		t.Fatalf("unexpected lock value %q", raw)
	}
	past := time.Now().Add(-time.Minute)
	if err = bds.Set(ctx, key, formatLockValue(owner, past), 600); err != nil {
		t.Fatalf("rewrite lock value: %v", err)
	}
}

func readLockValue(t *testing.T, bds bedis.Client, res string) lockValue {
	t.Helper()

	raw, err := bds.Get(context.Background(), lockKey(res))
	if err != nil {
		t.Fatalf("get lock value: %v", err)
	}
	value, err := parseLockValue(raw)
	if err != nil {
		t.Fatalf("parse lock value %q: %v", raw, err)
	}
	return value
}

// TestTokenLockTryAcquireExcludesOthers 同一资源同时只有一个持有者，释放后其他实例才能获取
func TestTokenLockTryAcquireExcludesOthers(t *testing.T) {
	l, _ := newTestTokenLock(t, 600)

	first := l.TryAcquire("res")
	if first == nil {
		t.Fatalf("first acquire failed")
	}
	if second := l.TryAcquire("res"); second != nil {
		t.Fatalf("second acquire should fail while the lock is held")
	}

	first.Release()
	if third := l.TryAcquire("res"); third == nil {
		t.Fatalf("acquire should succeed after release")
	}
}

func TestTokenLockRejectsZeroTTL(t *testing.T) {
	l, _ := newTestTokenLock(t, 0)
	if handle := l.TryAcquire("res"); handle != nil {
		t.Fatal("zero-TTL lock must not be acquired")
	}
}

// TestTokenLockReleaseOnlyOwnToken 锁过期被接管后，旧持有者释放不得删除新持有者的锁：
// 否则两个实例都不再持锁，后续同步会并发执行（这是 RedisLock 纯时间戳 value 的缺陷）
func TestTokenLockReleaseOnlyOwnToken(t *testing.T) {
	l, bds := newTestTokenLock(t, 600)

	stale := l.TryAcquire("res")
	if stale == nil {
		t.Fatalf("first acquire failed")
	}
	forceExpire(t, bds, "res")

	next := l.TryAcquire("res")
	if next == nil {
		t.Fatalf("takeover of an expired lock should succeed")
	}
	nextValue := readLockValue(t, bds, "res")

	stale.Release()

	if readLockValue(t, bds, "res").token != nextValue.token {
		t.Fatalf("release of the stale owner removed or replaced the new owner's lock")
	}
	if err := next.Renew(context.Background()); err != nil {
		t.Fatalf("new owner lost the lock after stale release: %v", err)
	}

	next.Release()
	if raw, err := bds.Get(context.Background(), lockKey("res")); err != nil || raw != "" {
		t.Fatalf("release by the owner should delete the lock, raw=%q, err=%v", raw, err)
	}
}

// TestTokenLockRenewExtendsDeadline 续期必须延长截止时间并保持持有者不变
func TestTokenLockRenewExtendsDeadline(t *testing.T) {
	l, bds := newTestTokenLock(t, 600)

	handle := l.TryAcquire("res")
	if handle == nil {
		t.Fatalf("acquire failed")
	}
	before := readLockValue(t, bds, "res")

	if err := handle.Renew(context.Background()); err != nil {
		t.Fatalf("renew failed: %v", err)
	}
	after := readLockValue(t, bds, "res")
	if after.token != before.token {
		t.Fatalf("renew changed owner: %q -> %q", before.token, after.token)
	}
	if !after.deadline.After(before.deadline) {
		t.Fatalf("renew did not extend deadline: %v -> %v", before.deadline, after.deadline)
	}

	// 被接管后原持有者的续期必须失败，避免两个实例同时认为自己持锁
	forceExpire(t, bds, "res")
	if next := l.TryAcquire("res"); next == nil {
		t.Fatalf("takeover should succeed")
	}
	if err := handle.Renew(context.Background()); err == nil {
		t.Fatalf("renew should fail after the lock was taken over")
	}
}

// TestTokenLockTakeoverRequiresExpired 未过期的锁不允许被抢占，避免长同步被误接管
func TestTokenLockTakeoverRequiresExpired(t *testing.T) {
	l, _ := newTestTokenLock(t, 600)

	handle := l.TryAcquire("res")
	if handle == nil {
		t.Fatalf("acquire failed")
	}
	if other := l.TryAcquire("res"); other != nil {
		t.Fatalf("live lock must not be taken over")
	}
	if err := handle.Renew(context.Background()); err != nil {
		t.Fatalf("owner should still hold the lock: %v", err)
	}
}

// TestTokenLockLegacyValue 兼容旧版 RedisLock 的纯 RFC3339 value：
// 未过期时不可抢占，已过期时按接管处理
func TestTokenLockLegacyValue(t *testing.T) {
	l, bds := newTestTokenLock(t, 600)
	ctx := context.Background()
	key := lockKey("res")

	if err := bds.Set(ctx, key, time.Now().Add(time.Minute).Format(time.RFC3339), 600); err != nil {
		t.Fatalf("set legacy value: %v", err)
	}
	if handle := l.TryAcquire("res"); handle != nil {
		t.Fatalf("legacy live lock must not be taken over")
	}

	if err := bds.Set(ctx, key, time.Now().Add(-time.Minute).Format(time.RFC3339), 600); err != nil {
		t.Fatalf("set expired legacy value: %v", err)
	}
	handle := l.TryAcquire("res")
	if handle == nil {
		t.Fatalf("expired legacy lock should be taken over")
	}
	handle.Release()
}
