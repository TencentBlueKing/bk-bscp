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
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

const (
	actionUpsert = "upsert"
	actionDelete = "delete"
	actionGet    = "get"
	actionList   = "list"
	actionAppend = "append"
	actionRemove = "remove"
)

// ManageConfigKV 通用 configs 表 KV 管理，根据 action 路由到对应操作。
func (s *Service) ManageConfigKV(ctx context.Context,
	req *pbds.ManageConfigKVReq) (*pbds.ManageConfigKVResp, error) {

	switch req.Action {
	case actionUpsert:
		return s.handleConfigKVUpsert(ctx, req)
	case actionDelete:
		return s.handleConfigKVDelete(ctx, req)
	case actionGet:
		return s.handleConfigKVGet(ctx, req)
	case actionList:
		return s.handleConfigKVList(ctx, req)
	case actionAppend:
		return s.handleConfigKVAppend(ctx, req)
	case actionRemove:
		return s.handleConfigKVRemove(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported action %q, must be one of: "+
			"upsert, delete, get, list, append, remove", req.Action)
	}
}

// GetProcessConfigView 查询指定业务是否开启进程与配置管理可见性
func (s *Service) GetProcessConfigView(_ context.Context,
	req *pbds.GetProcessConfigViewReq) (*pbds.GetProcessConfigViewResp, error) {

	return &pbds.GetProcessConfigViewResp{Enabled: s.IsProcessConfigViewEnabled(req.BizId)}, nil
}

func (s *Service) handleConfigKVUpsert(ctx context.Context,
	req *pbds.ManageConfigKVReq) (*pbds.ManageConfigKVResp, error) {

	kt := kit.FromGrpcContext(ctx)

	if len(req.Kvs) == 0 {
		return nil, fmt.Errorf("kvs is required for upsert action")
	}

	configs := make([]*table.Config, 0, len(req.Kvs))
	for _, kv := range req.Kvs {
		if kv.Key == "" {
			return nil, fmt.Errorf("key must not be empty")
		}
		configs = append(configs, &table.Config{Key: kv.Key, Value: kv.Value})
	}

	if err := s.dao.Config().UpsertConfig(kt, configs); err != nil {
		logs.Errorf("upsert config kvs failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 写入成功后同步更新缓存中的已注册 key
	for _, kv := range req.Kvs {
		s.configKVCache.Set(kv.Key, kv.Value)
	}

	return &pbds.ManageConfigKVResp{}, nil
}

func (s *Service) handleConfigKVDelete(ctx context.Context,
	req *pbds.ManageConfigKVReq) (*pbds.ManageConfigKVResp, error) {

	kt := kit.FromGrpcContext(ctx)

	if req.Key == "" {
		return nil, fmt.Errorf("key is required for delete action")
	}

	if err := s.dao.Config().DeleteConfigByKey(kt, req.Key); err != nil {
		logs.Errorf("delete config key %s failed: %v, rid: %s", req.Key, err, kt.Rid)
		return nil, err
	}

	s.configKVCache.Delete(req.Key)

	return &pbds.ManageConfigKVResp{}, nil
}

func (s *Service) handleConfigKVGet(ctx context.Context,
	req *pbds.ManageConfigKVReq) (*pbds.ManageConfigKVResp, error) {

	if req.Key == "" {
		return nil, fmt.Errorf("key is required for get action")
	}

	kt := kit.FromGrpcContext(ctx)
	config, err := s.dao.Config().GetConfig(kt, req.Key)
	if err != nil {
		return &pbds.ManageConfigKVResp{}, nil
	}
	return &pbds.ManageConfigKVResp{
		Items: []*pbds.ConfigKVItem{{Key: config.Key, Value: config.Value}},
	}, nil
}

func (s *Service) handleConfigKVList(ctx context.Context,
	req *pbds.ManageConfigKVReq) (*pbds.ManageConfigKVResp, error) {

	kt := kit.FromGrpcContext(ctx)

	configs, err := s.dao.Config().ListConfigByKeyPrefix(kt, req.KeyPrefix)
	if err != nil {
		logs.Errorf("list configs with prefix %q failed: %v, rid: %s", req.KeyPrefix, err, kt.Rid)
		return nil, err
	}

	items := make([]*pbds.ConfigKVItem, 0, len(configs))
	for _, c := range configs {
		items = append(items, &pbds.ConfigKVItem{Key: c.Key, Value: c.Value})
	}

	return &pbds.ManageConfigKVResp{Items: items}, nil
}

// handleConfigKVAppend 将 value 中的元素追加到指定 key 的逗号分隔列表中（自动去重）。
// 请求格式: kvs: [{key: "pcv_biz", value: "5,10"}]
func (s *Service) handleConfigKVAppend(ctx context.Context,
	req *pbds.ManageConfigKVReq) (*pbds.ManageConfigKVResp, error) {

	if len(req.Kvs) == 0 {
		return nil, fmt.Errorf("kvs is required for append action")
	}

	kt := kit.FromGrpcContext(ctx)

	merged := make(map[string]string, len(req.Kvs))
	for _, kv := range req.Kvs {
		if kv.Key == "" {
			return nil, fmt.Errorf("key must not be empty")
		}
		merged[kv.Key] = s.mergeCSV(kt, kv.Key, kv.Value, false)
	}

	configs := make([]*table.Config, 0, len(merged))
	for k, v := range merged {
		configs = append(configs, &table.Config{Key: k, Value: v})
	}
	if err := s.dao.Config().UpsertConfig(kt, configs); err != nil {
		logs.Errorf("append config kvs failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	for k, v := range merged {
		s.configKVCache.Set(k, v)
	}

	return &pbds.ManageConfigKVResp{}, nil
}

// handleConfigKVRemove 从指定 key 的逗号分隔列表中移除 value 中的元素。
// 请求格式: kvs: [{key: "pcv_biz", value: "5"}]
func (s *Service) handleConfigKVRemove(ctx context.Context,
	req *pbds.ManageConfigKVReq) (*pbds.ManageConfigKVResp, error) {

	if len(req.Kvs) == 0 {
		return nil, fmt.Errorf("kvs is required for remove action")
	}

	kt := kit.FromGrpcContext(ctx)

	merged := make(map[string]string, len(req.Kvs))
	for _, kv := range req.Kvs {
		if kv.Key == "" {
			return nil, fmt.Errorf("key must not be empty")
		}
		merged[kv.Key] = s.mergeCSV(kt, kv.Key, kv.Value, true)
	}

	configs := make([]*table.Config, 0, len(merged))
	for k, v := range merged {
		configs = append(configs, &table.Config{Key: k, Value: v})
	}
	if err := s.dao.Config().UpsertConfig(kt, configs); err != nil {
		logs.Errorf("remove config kvs failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	for k, v := range merged {
		s.configKVCache.Set(k, v)
	}

	return &pbds.ManageConfigKVResp{}, nil
}

// mergeCSV 读取 key 当前值，将 incoming 按逗号拆分后追加或移除，返回合并后的新值。
func (s *Service) mergeCSV(kt *kit.Kit, key, incoming string, remove bool) string {
	existing := ""
	config, err := s.dao.Config().GetConfig(kt, key)
	if err == nil {
		existing = config.Value
	}

	current := make(map[string]struct{})
	if existing != "" {
		for _, v := range strings.Split(existing, ",") {
			if t := strings.TrimSpace(v); t != "" {
				current[t] = struct{}{}
			}
		}
	}

	for _, v := range strings.Split(incoming, ",") {
		t := strings.TrimSpace(v)
		if t == "" {
			continue
		}
		if remove {
			delete(current, t)
		} else {
			current[t] = struct{}{}
		}
	}

	parts := make([]string, 0, len(current))
	for v := range current {
		parts = append(parts, v)
	}
	return strings.Join(parts, ",")
}

const (
	configCacheRefreshInterval = 5 * time.Minute

	// ConfigKeyPcvBiz 进程与配置管理可见性业务白名单
	ConfigKeyPcvBiz = "pcv_biz"

	// ConfigKeyPushConfigToTargetBiz GSE 文件传输下发（PushConfigToTarget）业务白名单，
	// 仅当 yaml 开关 usePushConfigToTarget 开启且业务命中该白名单时才走文件传输下发
	ConfigKeyPushConfigToTargetBiz = "push_config_to_target_biz"
)

// cachedConfigKeys 需要在启动时加载并定期刷新的 key 列表。
var cachedConfigKeys = []string{
	ConfigKeyPcvBiz,
	ConfigKeyPushConfigToTargetBiz,
}

// ConfigKVCache configs 表中特定 key 的内存缓存
type ConfigKVCache struct {
	mu   sync.RWMutex
	data map[string]string
}

// Get returns the cached value for a key.
func (c *ConfigKVCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	return v, ok
}

// Set updates an entry in the cache only if the key is registered in cachedConfigKeys.
func (c *ConfigKVCache) Set(key, value string) {
	if !isCachedKey(key) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}

// Delete removes an entry from the cache.
func (c *ConfigKVCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

func (c *ConfigKVCache) replace(m map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = m
}

func isCachedKey(key string) bool {
	for _, k := range cachedConfigKeys {
		if k == key {
			return true
		}
	}
	return false
}

// StopConfigKVCache stops the background config KV cache refresh goroutine.
func (s *Service) StopConfigKVCache() {
	if s.configKVCacheCancel != nil {
		s.configKVCacheCancel()
	}
}

// InitConfigKVCache loads registered keys from DB into cache and starts background refresh.
func (s *Service) InitConfigKVCache() {
	s.configKVCache = &ConfigKVCache{data: make(map[string]string)}
	if err := s.refreshConfigKVCache(); err != nil {
		logs.Errorf("initial config kv cache load failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.configKVCacheCancel = cancel
	go s.configKVCacheRefreshLoop(ctx)
}

func (s *Service) refreshConfigKVCache() error {
	kt := kit.New()
	configs, err := s.dao.Config().ListConfigByKeys(kt, cachedConfigKeys)
	if err != nil {
		return fmt.Errorf("load cached config keys: %w", err)
	}
	m := make(map[string]string, len(configs))
	for _, c := range configs {
		m[c.Key] = c.Value
	}
	s.configKVCache.replace(m)
	return nil
}

func (s *Service) configKVCacheRefreshLoop(ctx context.Context) {
	ticker := time.NewTicker(configCacheRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logs.Infof("config kv cache refresh loop stopped")
			return
		case <-ticker.C:
			if err := s.refreshConfigKVCache(); err != nil {
				logs.Errorf("config kv cache refresh failed: %v", err)
			}
		}
	}
}

// IsProcessConfigViewEnabled checks whether process config view is enabled for a given biz.
func (s *Service) IsProcessConfigViewEnabled(bizID uint32) bool {
	val := s.getConfigValue(ConfigKeyPcvBiz)
	if val == "" {
		return false
	}
	target := strconv.FormatUint(uint64(bizID), 10)
	for _, id := range strings.Split(val, ",") {
		if strings.TrimSpace(id) == target {
			return true
		}
	}
	return false
}

// ListProcessConfigEnabledBizIDs 从 configs.pcv_biz 解析已开启进程配置的业务 ID，按升序去重。
// 定时全量同步以该名单为准，不请求 CMDB 拉全量业务，避免返回顺序变化影响轮转。
func (s *Service) ListProcessConfigEnabledBizIDs() []int {
	return parsePcvBizIDs(s.getConfigValue(ConfigKeyPcvBiz))
}

// ListPushConfigToTargetBizIDs 从 configs.push_config_to_target_biz 解析 GSE 文件传输下发白名单业务 ID，按升序去重。
func (s *Service) ListPushConfigToTargetBizIDs() []uint32 {
	ids := parsePcvBizIDs(s.getConfigValue(ConfigKeyPushConfigToTargetBiz))
	out := make([]uint32, 0, len(ids))
	for _, id := range ids {
		out = append(out, uint32(id))
	}
	return out
}

// parsePcvBizIDs 解析逗号分隔的业务白名单，非法值和 <=0 丢弃，结果按 ID 升序。
func parsePcvBizIDs(val string) []int {
	if val == "" {
		return nil
	}
	seen := make(map[int]struct{})
	ids := make([]int, 0)
	for _, part := range strings.Split(val, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err != nil || id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

// getConfigValue 优先从缓存获取，缓存未命中则兜底查 DB。
func (s *Service) getConfigValue(key string) string {
	if s.configKVCache != nil {
		if val, ok := s.configKVCache.Get(key); ok {
			return val
		}
	}
	kt := kit.New()
	config, err := s.dao.Config().GetConfig(kt, key)
	if err != nil {
		return ""
	}
	return config.Value
}
