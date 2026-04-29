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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// RenderCache caches CMDB aggregation results used by config rendering.
type RenderCache interface {
	GetTopoXML(ctx context.Context, tenantID string, bizID int, setEnv string) (string, bool)
	SetTopoXML(ctx context.Context, tenantID string, bizID int, setEnv string, xml string)
	GetBizObjectAttributes(ctx context.Context, tenantID string, bizID int) (map[string][]ObjectAttribute, bool)
	SetBizObjectAttributes(ctx context.Context, tenantID string, bizID int, attrs map[string][]ObjectAttribute)
	InvalidateBiz(ctx context.Context, tenantID string, bizID int) error
	AcquireBuildLock(ctx context.Context, tenantID string, bizID int, kind string, identity string) (bool, error)
	BuildLockTTL() time.Duration
}

// RenderCacheOptions defines ttl settings for CMDB render cache.
type RenderCacheOptions struct {
	TopoXMLTTL            time.Duration
	BizGlobalVariablesTTL time.Duration
	BuildLockTTL          time.Duration
}

// DefaultRenderCacheOptions returns production defaults compatible with the old project.
func DefaultRenderCacheOptions() RenderCacheOptions {
	return RenderCacheOptions{
		TopoXMLTTL:            time.Hour,
		BizGlobalVariablesTTL: 5 * time.Minute,
		BuildLockTTL:          30 * time.Second,
	}
}

func (o RenderCacheOptions) withDefaults() RenderCacheOptions {
	defaults := DefaultRenderCacheOptions()
	if o.TopoXMLTTL <= 0 {
		o.TopoXMLTTL = defaults.TopoXMLTTL
	}
	if o.BizGlobalVariablesTTL <= 0 {
		o.BizGlobalVariablesTTL = defaults.BizGlobalVariablesTTL
	}
	if o.BuildLockTTL <= 0 {
		o.BuildLockTTL = defaults.BuildLockTTL
	}
	return o
}

type renderCacheStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, ttlSeconds int) error
	HGet(ctx context.Context, hashKey string, field string) (string, error)
	HSets(ctx context.Context, hashKey string, kv map[string]string, ttlSeconds int) error
	SetNX(ctx context.Context, key string, value interface{}, ttlSeconds int) (bool, error)
	Delete(ctx context.Context, keys ...string) error
}

// RedisCMDBRenderCache is a Redis-backed cache shared by data-service replicas.
type RedisCMDBRenderCache struct {
	store   renderCacheStore
	options RenderCacheOptions
}

// NewRedisCMDBRenderCache creates a Redis-backed render cache.
func NewRedisCMDBRenderCache(redis bedis.Client, options RenderCacheOptions) RenderCache {
	if redis == nil {
		return nil
	}
	return newRedisCMDBRenderCacheWithStore(redis, options)
}

func newRedisCMDBRenderCacheWithStore(store renderCacheStore, options RenderCacheOptions) *RedisCMDBRenderCache {
	return &RedisCMDBRenderCache{
		store:   store,
		options: options.withDefaults(),
	}
}

func (c *RedisCMDBRenderCache) GetTopoXML(ctx context.Context, tenantID string, bizID int, setEnv string) (string, bool) {
	if c == nil || c.store == nil {
		return "", false
	}

	key := c.topoXMLKey(tenantID, bizID)
	field := topoXMLField(setEnv)
	xml, err := c.store.HGet(ctx, key, field)
	if err != nil {
		if errors.Is(err, bedis.ErrKeyNotExist) {
			return "", false
		}
		logs.Warnf("get cmdb topo xml cache failed, key: %s, field: %s, err: %v", key, field, err)
		return "", false
	}
	if xml == "" {
		return "", false
	}
	return xml, true
}

func (c *RedisCMDBRenderCache) SetTopoXML(ctx context.Context, tenantID string, bizID int, setEnv string, xml string) {
	if c == nil || c.store == nil {
		return
	}

	key := c.topoXMLKey(tenantID, bizID)
	field := topoXMLField(setEnv)
	if err := c.store.HSets(ctx, key, map[string]string{field: xml}, ttlSeconds(c.options.TopoXMLTTL)); err != nil {
		logs.Warnf("set cmdb topo xml cache failed, key: %s, err: %v", key, err)
	}
}

func (c *RedisCMDBRenderCache) GetBizObjectAttributes(
	ctx context.Context, tenantID string, bizID int) (map[string][]ObjectAttribute, bool) {
	if c == nil || c.store == nil {
		return nil, false
	}

	key := c.bizObjectAttributesKey(tenantID, bizID)
	value, err := c.store.Get(ctx, key)
	if err != nil {
		logs.Warnf("get cmdb biz object attributes cache failed, key: %s, err: %v", key, err)
		return nil, false
	}
	if value == "" {
		return nil, false
	}

	attrs := make(map[string][]ObjectAttribute)
	if err := json.Unmarshal([]byte(value), &attrs); err != nil {
		logs.Warnf("unmarshal cmdb biz object attributes cache failed, key: %s, err: %v", key, err)
		return nil, false
	}
	return cloneObjectAttributeMap(attrs), true
}

func (c *RedisCMDBRenderCache) SetBizObjectAttributes(
	ctx context.Context, tenantID string, bizID int, attrs map[string][]ObjectAttribute) {
	if c == nil || c.store == nil {
		return
	}

	key := c.bizObjectAttributesKey(tenantID, bizID)
	payload, err := json.Marshal(attrs)
	if err != nil {
		logs.Warnf("marshal cmdb biz object attributes cache failed, key: %s, err: %v", key, err)
		return
	}
	if err := c.store.Set(ctx, key, string(payload), ttlSeconds(c.options.BizGlobalVariablesTTL)); err != nil {
		logs.Warnf("set cmdb biz object attributes cache failed, key: %s, err: %v", key, err)
	}
}

func (c *RedisCMDBRenderCache) InvalidateBiz(ctx context.Context, tenantID string, bizID int) error {
	if c == nil || c.store == nil {
		return nil
	}
	return c.store.Delete(ctx, c.topoXMLKey(tenantID, bizID), c.bizObjectAttributesKey(tenantID, bizID))
}

func (c *RedisCMDBRenderCache) AcquireBuildLock(
	ctx context.Context, tenantID string, bizID int, kind string, identity string) (bool, error) {
	if c == nil || c.store == nil {
		return true, nil
	}
	return c.store.SetNX(ctx, c.buildLockKey(tenantID, bizID, kind, identity), "1", ttlSeconds(c.options.BuildLockTTL))
}

func (c *RedisCMDBRenderCache) BuildLockTTL() time.Duration {
	if c == nil {
		return 0
	}
	return c.options.BuildLockTTL
}

func (c *RedisCMDBRenderCache) topoXMLKey(tenantID string, bizID int) string {
	return fmt.Sprintf("bscp:cmdb:render:v1:tenant:%s:biz:%d:topo_xml", normalizeTenantID(tenantID), bizID)
}

func (c *RedisCMDBRenderCache) bizObjectAttributesKey(tenantID string, bizID int) string {
	return fmt.Sprintf("bscp:cmdb:render:v1:tenant:%s:biz:%d:biz_global_variables", normalizeTenantID(tenantID), bizID)
}

func (c *RedisCMDBRenderCache) buildLockKey(tenantID string, bizID int, kind string, identity string) string {
	hash := sha256.Sum256([]byte(identity))
	return fmt.Sprintf(
		"bscp:cmdb:render:v1:tenant:%s:biz:%d:lock:%s:%s",
		normalizeTenantID(tenantID), bizID, kind, hex.EncodeToString(hash[:]),
	)
}

func topoXMLField(setEnv string) string {
	hash := sha256.Sum256([]byte(setEnv))
	return hex.EncodeToString(hash[:])
}

func normalizeTenantID(tenantID string) string {
	if tenantID == "" {
		return "default"
	}
	return tenantID
}

func ttlSeconds(ttl time.Duration) int {
	seconds := int(ttl.Seconds())
	if seconds <= 0 {
		return 1
	}
	return seconds
}

type memoryCMDBRenderCache struct {
	mu                  sync.RWMutex
	topoXML             map[string]string
	bizObjectAttributes map[string]map[string][]ObjectAttribute
}

// NewMemoryCMDBRenderCache creates an in-memory render cache for tests.
func NewMemoryCMDBRenderCache() RenderCache {
	return &memoryCMDBRenderCache{
		topoXML:             make(map[string]string),
		bizObjectAttributes: make(map[string]map[string][]ObjectAttribute),
	}
}

func (c *memoryCMDBRenderCache) GetTopoXML(_ context.Context, tenantID string, bizID int, setEnv string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	xml, exists := c.topoXML[buildTopoXMLCacheKey(tenantID, bizID, setEnv)]
	return xml, exists
}

func (c *memoryCMDBRenderCache) SetTopoXML(_ context.Context, tenantID string, bizID int, setEnv string, xml string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.topoXML[buildTopoXMLCacheKey(tenantID, bizID, setEnv)] = xml
}

func (c *memoryCMDBRenderCache) GetBizObjectAttributes(
	_ context.Context, tenantID string, bizID int) (map[string][]ObjectAttribute, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	attrs, exists := c.bizObjectAttributes[buildBizObjectAttributesMemoryKey(tenantID, bizID)]
	if !exists {
		return nil, false
	}
	return cloneObjectAttributeMap(attrs), true
}

func (c *memoryCMDBRenderCache) SetBizObjectAttributes(
	_ context.Context, tenantID string, bizID int, attrs map[string][]ObjectAttribute) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.bizObjectAttributes[buildBizObjectAttributesMemoryKey(tenantID, bizID)] = cloneObjectAttributeMap(attrs)
}

func (c *memoryCMDBRenderCache) InvalidateBiz(_ context.Context, tenantID string, bizID int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.bizObjectAttributes, buildBizObjectAttributesMemoryKey(tenantID, bizID))
	prefix := fmt.Sprintf("tenant:%s:biz:%d:set_env:", normalizeTenantID(tenantID), bizID)
	for key := range c.topoXML {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.topoXML, key)
		}
	}
	return nil
}

func (c *memoryCMDBRenderCache) AcquireBuildLock(
	_ context.Context, _ string, _ int, _ string, _ string) (bool, error) {
	return true, nil
}

func (c *memoryCMDBRenderCache) BuildLockTTL() time.Duration {
	return 0
}

func buildTopoXMLCacheKey(tenantID string, bizID int, setEnv string) string {
	return fmt.Sprintf("tenant:%s:biz:%d:set_env:%s", normalizeTenantID(tenantID), bizID, setEnv)
}

func buildBizObjectAttributesMemoryKey(tenantID string, bizID int) string {
	return fmt.Sprintf("tenant:%s:biz:%d", normalizeTenantID(tenantID), bizID)
}

func cloneObjectAttributeMap(attrs map[string][]ObjectAttribute) map[string][]ObjectAttribute {
	if attrs == nil {
		return nil
	}

	copied := make(map[string][]ObjectAttribute, len(attrs))
	for objID, values := range attrs {
		copiedValues := make([]ObjectAttribute, len(values))
		copy(copiedValues, values)
		copied[objID] = copiedValues
	}
	return copied
}
