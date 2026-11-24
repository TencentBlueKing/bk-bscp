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

// Package auth implements remote authorization against IAM V4.
package auth

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/adaptor"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// defaultConcurrency 是 Config.Concurrency 未指定时的兜底并发度，
// 不能为 0，否则 errgroup 会禁止所有 goroutine 执行。
const defaultConcurrency = 5

// gateway 是本包依赖的鉴权接口，抽出来便于单测替换。
type gateway interface {
	AuthByResources(kt *kit.Kit, subject client.Subject, actionID string,
		resources []client.AuthResource) (map[string]bool, error)
}

var _ gateway = (*client.Client)(nil)

// Config 是鉴权器的配置。
type Config struct {
	// CacheSize 鉴权结果缓存的条目上限，非正数表示禁用缓存
	CacheSize int
	// CacheTTL 鉴权结果的缓存时长，非正数表示禁用缓存
	CacheTTL time.Duration
	// Concurrency 批量鉴权切批后的并发度，非正数时取 defaultConcurrency
	Concurrency int
}

// Authorizer 基于 IAM V4 做远程鉴权。
type Authorizer struct {
	gw          gateway
	cache       *decisionCache
	concurrency int
}

// NewAuthorizer 构造 IAM V4 鉴权器。
func NewAuthorizer(gw gateway, cfg Config) (*Authorizer, error) {
	if gw == nil {
		return nil, errors.New("iam v4 gateway is nil")
	}

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}

	return &Authorizer{
		gw:          gw,
		cache:       newDecisionCache(cfg.CacheSize, cfg.CacheTTL),
		concurrency: concurrency,
	}, nil
}

// AuthorizeBatch 批量鉴权，返回与入参一一对应、顺序一致的判定结果。
func (a *Authorizer) AuthorizeBatch(kt *kit.Kit, user string, resources ...*meta.ResourceAttribute) (
	[]*meta.Decision, error) {

	if len(resources) == 0 {
		return []*meta.Decision{}, nil
	}
	start := time.Now()

	// 存储将 bscp 资源类型和操作转换为 IAMV4 的资源和操作映射
	mapped := make([]*adaptor.Mapped, len(resources))
	// 存储鉴权结果，按 resources 的顺序存储
	decisions := make([]*meta.Decision, len(resources))
	// pending 按操作 ID 收集仍需请求 IAMV4 的资源下标
	// 同一操作的资源可以合成一次批量请求
	pending := make(map[string][]int)

	var skipped, cached int

	// 遍历资源，做资源类型、操作类型的转换，通过缓存做初步鉴权
	for i, res := range resources {
		// 将 BSCP 自定义的资源和操作映射为 IAMV4 注册的资源和操作
		m, err := adaptor.Adapt(res)
		if err != nil {
			return nil, err
		}
		mapped[i] = m

		// BSCP 侧不做权限控制的资源直接放行
		if m.Skip {
			decisions[i] = &meta.Decision{Resource: res, Authorized: true}
			skipped++

			continue
		}

		// 通过缓存做初步鉴权
		if allowed, hit := a.cache.get(cacheKey(kt.TenantID, user, m)); hit {
			decisions[i] = &meta.Decision{Resource: res, Authorized: allowed}
			cached++

			continue
		}

		// 按操作 ID 收集仍需请求 IAMV4 的资源下标
		// 同一操作的资源可以合成一次批量请求
		pending[m.ActionID] = append(pending[m.ActionID], i)
	}

	batches := 0

	if len(pending) > 0 {
		// 请求权限中心时用户名不能为空
		if user == "" {
			return nil, errors.New("user is not set")
		}

		sent, err := a.resolve(kt, user, resources, mapped, pending, decisions)
		if err != nil {
			return nil, err
		}
		batches = sent
	}

	// 兜底检查：所有槽位都必须被填上，否则说明分批或回填逻辑有漏洞。
	for i, d := range decisions {
		if d == nil {
			return nil, fmt.Errorf("decision of resource %d is missing", i)
		}
	}

	// 用于判断 IAM V4 鉴权行为是否正常
	logs.Infof("iam v4 authorize batch, user: %s, total: %d, skipped: %d, cached: %d, "+
		"actions: %d, batches: %d, elapsed: %dms, rid: %s",
		user, len(resources), skipped, cached, len(pending), batches,
		time.Since(start).Milliseconds(), kt.Rid)

	return decisions, nil
}

// Authorize 单点鉴权，是 AuthorizeBatch 的便捷形式。
func (a *Authorizer) Authorize(kt *kit.Kit, user string, res *meta.ResourceAttribute) (bool, error) {
	decisions, err := a.AuthorizeBatch(kt, user, res)
	if err != nil {
		return false, err
	}

	if len(decisions) != 1 {
		return false, fmt.Errorf("expect 1 decision, got %d", len(decisions))
	}

	return decisions[0].Authorized, nil
}

// batch 一次批量鉴权请求。
type batch struct {
	actionID string
	// resources 去重后的待鉴权资源，数量不超过 client.MaxAuthBatchSize
	resources []client.AuthResource
	// indexes 本批覆盖的入参下标，可能多于 resources，存在资源重复的情况
	indexes []int
}

// resolve 向权限中心查询 pending 中的条目并回填 decisions，返回实际发出的请求批数。
func (a *Authorizer) resolve(kt *kit.Kit, user string, resources []*meta.ResourceAttribute,
	mapped []*adaptor.Mapped, pending map[string][]int, decisions []*meta.Decision) (int, error) {

	batches := planBatches(mapped, pending)

	// 每批的结果写进各自的槽位，避免并发写同一个 map。
	results := make([]map[string]bool, len(batches))

	eg := new(errgroup.Group)
	eg.SetLimit(a.concurrency)

	for i := range batches {
		idx, b := i, batches[i]

		eg.Go(func() error {
			got, err := a.gw.AuthByResources(kt, client.NewUserSubject(user), b.actionID, b.resources)
			if err != nil {
				return err
			}
			results[idx] = got

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return 0, err
	}

	for i, b := range batches {
		for _, idx := range b.indexes {
			res := mapped[idx].AuthResource()
			if res == nil {
				return 0, fmt.Errorf("action %s has no resource to authorize", mapped[idx].ActionID)
			}

			// 权限中心未返回该资源时按无权限处理
			allowed := results[i][res.ID]

			decisions[idx] = &meta.Decision{Resource: resources[idx], Authorized: allowed}
			a.cache.set(cacheKey(kt.TenantID, user, mapped[idx]), allowed)
		}
	}

	return len(batches), nil
}

// planBatches 把待查询条目按操作分组，再按单次上限切批。
//
// 批量接口的响应按 resource_id 索引，同一 ID 传多次会被合并成一条，
// 因此同批内的资源先去重再发送，回填时按资源 ID 反查结果。
func planBatches(mapped []*adaptor.Mapped, pending map[string][]int) []batch {
	batches := make([]batch, 0, len(pending))

	// 遍历 pending，按操作 ID 分组
	for actionID, indexes := range pending {
		// 保存当前操作的资源和下标
		current := batch{actionID: actionID}
		// 记录当前操作中已经处理过的资源 ID，避免重复处理
		seen := make(map[string]bool, len(indexes))

		for _, i := range indexes {
			res := mapped[i].AuthResource()
			if res == nil {
				continue
			}

			current.indexes = append(current.indexes, i)

			if seen[res.ID] {
				continue
			}
			seen[res.ID] = true
			current.resources = append(current.resources, *res)

			// 如果当前操作的资源数量达到上限，则将当前操作添加到 batches 中，并创建新的操作
			// 并清空当前操作的资源和下标
			if len(current.resources) == client.MaxAuthBatchSize {
				batches = append(batches, current)
				current = batch{actionID: actionID}
				// 清空当前操作中已经处理过的资源 ID
				seen = make(map[string]bool, len(indexes))
			}
		}
		// 如果当前操作的资源下标不为空，则将当前操作添加到 batches 中
		if len(current.indexes) > 0 {
			batches = append(batches, current)
		}
	}

	return batches
}
