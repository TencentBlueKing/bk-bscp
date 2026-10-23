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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	// ownerTokenBytes 持有者 token 的随机字节数
	ownerTokenBytes = 16
	// lockValueSeparator 锁 value 中「持有者 token」与「截止时间」的分隔符
	lockValueSeparator = "|"
	// lockKeyTTLSlack 锁 key 的 Redis 过期时间相对锁 TTL 的余量：
	// 截止时间存在 value 里，key 需多存活一段时间，否则截止时间刚过 key 就被删除，
	// 其他实例无法读到「已过期」的 value 也就无法按过期语义接管
	lockKeyTTLSlack = 5
)

// Handle 一次加锁的持有者句柄：续期与释放只作用于自己写入的 key。
// 锁过期被其他实例接管后，旧持有者既不能续期，也不能删除新持有者的锁。
type Handle interface {
	// Renew 续期：仅当 key 仍由本持有者写入时延长截止时间，否则返回错误；ctx 用于中止阻塞的 Redis 请求
	Renew(ctx context.Context) error
	// RenewInterval 建议的续期间隔（锁 TTL 的一半）
	RenewInterval() time.Duration
	// Release 释放：仅当 key 仍由本持有者写入时才删除
	Release()
}

// 续期与释放的原子脚本：先比对 value 中的持有者 token 前缀，再写/删，
// 避免读取与写入之间锁被接管导致旧持有者删掉新持有者的锁。
const (
	// renewLockScript 续期：KEYS[1]=锁 key，ARGV[1]=持有者 token，ARGV[2]=新 value，ARGV[3]=key 过期秒数
	renewLockScript = `
local v = redis.call('GET', KEYS[1])
if not v then return 0 end
if string.sub(v, 1, string.len(ARGV[1]) + 1) ~= ARGV[1] .. '|' then return 0 end
redis.call('SET', KEYS[1], ARGV[2], 'EX', ARGV[3])
return 1
`
	// releaseLockScript 释放：KEYS[1]=锁 key，ARGV[1]=持有者 token
	releaseLockScript = `
local v = redis.call('GET', KEYS[1])
if not v then return 0 end
if string.sub(v, 1, string.len(ARGV[1]) + 1) ~= ARGV[1] .. '|' then return 0 end
return redis.call('DEL', KEYS[1])
`
	// casLockScript 原子比较并替换，用于过期接管：当前值仍等于读到的旧值时才写入新值，
	// 避免抢占瞬间覆盖其他实例刚写入的未过期锁
	casLockScript = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
	redis.call('SET', KEYS[1], ARGV[2], 'EX', ARGV[3])
	return 1
end
return 0
`
)

// TokenLock 带持有者 token 与续期能力的分布式锁，用于耗时不可预估的长任务互斥
// （如 CMDB 全量进程同步：分页拉取 + GSE 查询 + 大批量事务）。
//
// 与 RedisLock 的差异：
//   - value 由「纯截止时间」变为「<token>|<截止时间>」，释放与续期必须匹配 token，
//     锁过期被接管后旧持有者不会误删新持有者的锁；
//   - 提供 Renew，持有者可在任务执行期间按 TTL 的一半续期，续期失败会被调用方感知；
//   - 过期接管、续期、释放都走 Lua 原子脚本，不存在「读完再写」的覆盖窗口。
//
// 兼容性：与 RedisLock 共用 Lock:<res> 命名空间，滚动升级期间遵循同一锁协议。
// 新实例能读旧格式（纯 RFC3339，按过期接管）；旧实例无法解析新格式，
// 在等待窗口内保持跳过。此租约锁不提供数据库 fencing token，不能防止暂停/网络分区后的旧持有者继续运行。
type TokenLock struct {
	ctx context.Context
	bds bedis.Client
	ttl uint
}

// NewTokenLock 初始化带 token 的分布式锁，ttl 为锁的存活时间（秒）
func NewTokenLock(bds bedis.Client, ttl uint) *TokenLock {
	return &TokenLock{ctx: context.Background(), bds: bds, ttl: ttl}
}

// TryAcquire 尝试获取锁，不等待：成功返回持有者句柄，失败或 Redis 异常返回 nil。
// 只有确认原持有者的截止时间已过才按「过期接管」抢占。
func (l *TokenLock) TryAcquire(res string) Handle {
	if l.ttl == 0 {
		logs.Errorf("token lock %s: ttl must be greater than zero", lockKey(res))
		return nil
	}
	key := lockKey(res)
	token, err := newOwnerToken()
	if err != nil {
		logs.Errorf("token lock %s: generate owner token failed, err: %v", key, err)
		return nil
	}
	value := formatLockValue(token, time.Now().Add(l.duration()))

	ok, err := l.bds.SetNX(l.ctx, key, value, l.keyTTLSeconds())
	if err != nil {
		logs.Errorf("token lock %s: setnx failed, err: %v", key, err)
		return nil
	}
	if ok {
		return &lockHandle{lock: l, key: key, token: token}
	}

	raw, err := l.bds.Get(l.ctx, key)
	if err != nil {
		logs.Errorf("token lock %s: get failed, err: %v", key, err)
		return nil
	}
	if raw == "" {
		// 与 SetNX 之间 key 被释放：本轮不抢，交给调用方按等待窗口重试
		logs.Warnf("token lock %s: key disappeared right after setnx", key)
		return nil
	}
	cur, err := parseLockValue(raw)
	if err != nil {
		// 值不可解析（脏数据/未知格式）：无法判定是否仍被持有，不做抢占，避免误抢正常同步
		logs.Errorf("token lock %s: parse lock value %q failed, err: %v", key, raw, err)
		return nil
	}
	if time.Now().Before(cur.deadline) {
		return nil
	}

	acquired, err := l.evalInt(l.ctx, casLockScript, key, raw, value, l.keyTTLSeconds())
	if err != nil {
		logs.Errorf("token lock %s: take over expired lock failed, err: %v", key, err)
		return nil
	}
	if acquired != 1 {
		// 读到过期值之后、写入之前，其他实例已经接管：本次放弃，不覆盖对方
		return nil
	}
	logs.Warnf("token lock %s: acquired by taking over an expired lock", key)
	return &lockHandle{lock: l, key: key, token: token}
}

// lockHandle TokenLock 的持锁句柄
type lockHandle struct {
	lock  *TokenLock
	key   string
	token string
}

// Renew 续期：value 中的 token 不匹配时说明锁已被接管，返回错误由调用方处理
func (h *lockHandle) Renew(ctx context.Context) error {
	if ctx == nil {
		ctx = h.lock.ctx
	}
	value := formatLockValue(h.token, time.Now().Add(h.lock.duration()))
	renewed, err := h.lock.evalInt(ctx, renewLockScript, h.key, h.token, value, h.lock.keyTTLSeconds())
	if err != nil {
		return fmt.Errorf("renew lock %s failed: %w", h.key, err)
	}
	if renewed != 1 {
		return fmt.Errorf("lock %s is no longer held by this owner", h.key)
	}
	return nil
}

// RenewInterval 续期间隔取 TTL 的一半：单次续期失败时仍有半个 TTL 的余量重试
func (h *lockHandle) RenewInterval() time.Duration {
	return max(h.lock.duration()/2, time.Second)
}

// Release 释放：token 不匹配（锁已过期并被接管）时不删除，避免删掉新持有者的锁
func (h *lockHandle) Release() {
	released, err := h.lock.evalInt(h.lock.ctx, releaseLockScript, h.key, h.token)
	if err != nil {
		logs.Errorf("release token lock %s failed, err: %v", h.key, err)
		return
	}
	if released != 1 {
		logs.Warnf("skip release token lock %s: lock is not held by this owner", h.key)
	}
}

func (l *TokenLock) duration() time.Duration {
	return time.Duration(l.ttl) * time.Second
}

func (l *TokenLock) keyTTLSeconds() int {
	return int(l.ttl) + lockKeyTTLSlack
}

// evalInt 执行返回整数的 Lua 脚本
func (l *TokenLock) evalInt(ctx context.Context, script, key string, args ...interface{}) (int64, error) {
	cmd := append([]interface{}{"eval", script, 1, key}, args...)
	res, err := l.bds.Do(ctx, cmd...)
	if err != nil {
		return 0, err
	}
	n, ok := res.(int64)
	if !ok {
		return 0, fmt.Errorf("unexpected eval result type %T", res)
	}
	return n, nil
}

// lockKey 与 RedisLock 保持同一命名空间，滚动升级期间新旧实例仍互斥
func lockKey(res string) string {
	return fmt.Sprintf("Lock:%s", res)
}

// lockValue 锁 value 的解析结果
type lockValue struct {
	// token 持有者 token；旧版 RedisLock 写入的 value 没有 token，此处为空串
	token string
	// deadline 锁的截止时间
	deadline time.Time
}

func formatLockValue(token string, deadline time.Time) string {
	return token + lockValueSeparator + deadline.UTC().Format(time.RFC3339Nano)
}

// parseLockValue 解析锁 value：新格式 <token>|<截止时间>；
// 兼容旧版 RedisLock 的纯 RFC3339 时间戳（token 为空，永远不会匹配本实例的 token）
func parseLockValue(raw string) (lockValue, error) {
	owner, deadlineStr, ok := strings.Cut(raw, lockValueSeparator)
	if !ok {
		deadline, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return lockValue{}, fmt.Errorf("invalid lock value %q: %w", raw, err)
		}
		return lockValue{deadline: deadline}, nil
	}
	deadline, err := time.Parse(time.RFC3339Nano, deadlineStr)
	if err != nil {
		return lockValue{}, fmt.Errorf("invalid lock deadline %q: %w", deadlineStr, err)
	}
	return lockValue{token: owner, deadline: deadline}, nil
}

func newOwnerToken() (string, error) {
	buf := make([]byte, ownerTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
