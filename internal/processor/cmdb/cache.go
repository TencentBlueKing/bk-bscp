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

package cmdb

import (
	"fmt"
	"sync"
	"time"
)

// CacheEntry 缓存条目，包含值和过期时间
type CacheEntry struct {
	Value      interface{}
	ExpiresAt  time.Time
	CreateTime time.Time
}

// IsExpired 检查缓存是否已过期
func (e *CacheEntry) IsExpired() bool {
	if e.ExpiresAt.IsZero() {
		return false // 永不过期
	}
	return time.Now().After(e.ExpiresAt)
}

// GlobalObjectAttrCache CMDB 对象属性的全局缓存
// 缓存 SearchObjectAttr 的查询结果，避免重复查询相同的对象属性
// 使用 TTL 机制，过期时间默认为 1 小时
type GlobalObjectAttrCache struct {
	mu    sync.RWMutex
	cache map[string]*CacheEntry
	ttl   time.Duration
}

var (
	// globalObjAttrCache 全局对象属性缓存单例
	globalObjAttrCache *GlobalObjectAttrCache
	// initOnce 确保缓存只初始化一次
	initOnce sync.Once
)

// GetGlobalObjectAttrCache 获取全局对象属性缓存单例
func GetGlobalObjectAttrCache() *GlobalObjectAttrCache {
	initOnce.Do(func() {
		globalObjAttrCache = &GlobalObjectAttrCache{
			cache: make(map[string]*CacheEntry),
			ttl:   time.Hour, // 默认 1 小时过期时间
		}
	})
	return globalObjAttrCache
}

// buildCacheKey 构建缓存键
// 格式: "bscp:cmdb:<object-type>:fields:biz:<bizID>"
func buildCacheKey(bizID int, objID string) string {
	return fmt.Sprintf("bscp:cmdb:%s:fields:biz:%d", objID, bizID)
}

// Get 从缓存中获取数据
// 如果缓存存在且未过期，返回缓存值和 true
// 如果缓存不存在或已过期，返回 nil 和 false
// 过期条目不主动删除，由后续 Set 覆写自然淘汰
func (c *GlobalObjectAttrCache) Get(bizID int, objID string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := buildCacheKey(bizID, objID)
	entry, exists := c.cache[key]
	if !exists {
		return nil, false
	}

	if entry.IsExpired() {
		return nil, false
	}

	return entry.Value, true
}

// Set 将数据存储到缓存中
// 使用全局配置的 TTL（默认 1 小时）
func (c *GlobalObjectAttrCache) Set(bizID int, objID string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := buildCacheKey(bizID, objID)
	c.cache[key] = &CacheEntry{
		Value:      value,
		ExpiresAt:  time.Now().Add(c.ttl),
		CreateTime: time.Now(),
	}
}

// SetWithTTL 使用自定义 TTL 将数据存储到缓存中
func (c *GlobalObjectAttrCache) SetWithTTL(bizID int, objID string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := buildCacheKey(bizID, objID)
	expiresAt := time.Now()
	if ttl > 0 {
		expiresAt = expiresAt.Add(ttl)
	} else {
		// 如果 ttl <= 0，表示永不过期
		expiresAt = time.Time{}
	}

	c.cache[key] = &CacheEntry{
		Value:      value,
		ExpiresAt:  expiresAt,
		CreateTime: time.Now(),
	}
}

// Delete 删除缓存中的数据
func (c *GlobalObjectAttrCache) Delete(bizID int, objID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := buildCacheKey(bizID, objID)
	delete(c.cache, key)
}

// Clear 清空所有缓存
func (c *GlobalObjectAttrCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*CacheEntry)
}

// SetTTL 设置全局 TTL
func (c *GlobalObjectAttrCache) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl > 0 {
		c.ttl = ttl
	}
}

// GetTTL 获取全局 TTL
func (c *GlobalObjectAttrCache) GetTTL() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.ttl
}

// GetStats 获取缓存统计信息（用于调试）
type CacheStats struct {
	TotalEntries   int
	ValidEntries   int
	ExpiredEntries int
}

// GetStats 获取缓存统计信息
func (c *GlobalObjectAttrCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		TotalEntries: len(c.cache),
	}

	for _, entry := range c.cache {
		if entry.IsExpired() {
			stats.ExpiredEntries++
		} else {
			stats.ValidEntries++
		}
	}

	return stats
}
