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

package crontab

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"golang.org/x/time/rate"
)

const (
	// sync data once a week
	defaultSyncBizHostInterval = 1 * time.Minute
	// Default QPS limit for CMDB requests
	defaultQpsLimit = 50.0
	// Default page size for list host requests
	defaultPageSize = 500
	// Cursor cache keys
	bizHostCursorKey    = "biz_host_cursor"
	hostDetailCursorKey = "host_detail_cursor"
)

// NewSyncBizHost init sync biz host with configurable settings
func NewSyncBizHost(
	set dao.Set,
	sd serviced.Service,
	cmdbService bkcmdb.Service,
	redisClient bedis.Client,
	pageSize int,
	qpsLimit float64,
) SyncBizHost {
	// Validate and set default values
	if pageSize <= 0 || pageSize > defaultPageSize {
		pageSize = defaultPageSize
	}
	if qpsLimit <= 0 {
		qpsLimit = defaultQpsLimit
	}

	// Create rate limiter with configurable QPS
	rateLimiter := rate.NewLimiter(rate.Limit(qpsLimit), 1)

	return SyncBizHost{
		set:         set,
		state:       sd,
		cmdbService: cmdbService,
		redisClient: redisClient,
		rateLimiter: rateLimiter,
		pageSize:    pageSize,
		qpsLimit:    qpsLimit,
	}
}

// SyncBizHost sync business host relationship
type SyncBizHost struct {
	set         dao.Set
	state       serviced.Service
	cmdbService bkcmdb.Service
	redisClient bedis.Client
	// rate limiter for CMDB requests
	rateLimiter *rate.Limiter
	// page size for list host requests
	pageSize int
	// qps limit for CMDB requests
	qpsLimit float64
	// sync lock for coordination with event watch
	syncLock sync.Mutex
}

// Run the sync biz host task
func (c *SyncBizHost) Run() {
	logs.Infof("start sync biz host task")
	notifier := shutdown.AddNotifier()
	go func() {
		// 首次启动时立即执行一次全量同步
		logs.Infof("performing initial full sync on service startup")
		kt := kit.New()
		ctx, cancel := context.WithCancel(kt.Ctx)
		kt.Ctx = ctx
		c.syncBizHost(kt)
		cancel()

		// 启动定时器，按间隔执行
		ticker := time.NewTicker(defaultSyncBizHostInterval)
		defer ticker.Stop()
		for {
			kt := kit.New()
			ctx, cancel := context.WithCancel(kt.Ctx)
			kt.Ctx = ctx

			select {
			case <-notifier.Signal:
				logs.Infof("stop sync biz host success")
				cancel()
				notifier.Done()
				return
			case <-ticker.C:
				// if !c.state.IsMaster() {
				// 	logs.Infof("current service instance is slave, skip sync biz host")
				// 	continue
				// }
				logs.Infof("starts to synchronize the biz host")
				c.syncBizHost(kt)
			}
		}
	}()
}

// SyncBizHost sync business host relationship
func (c *SyncBizHost) syncBizHost(kt *kit.Kit) {
	// 获取同步锁，确保全量同步优先执行
	c.syncLock.Lock()
	defer c.syncLock.Unlock()
	logs.Infof("acquired sync lock for full sync")

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		logs.Infof("sync biz host completed in %v", duration)
	}()

	// 获取3分钟前的事件cursor并缓存
	if err := c.cacheEventCursors(kt); err != nil {
		logs.Errorf("cache event cursors failed, err: %v", err)
		// 继续执行全量同步，不因cursor获取失败而中断
	}

	// Query BSCP businesses
	bizList, err := c.queryBSCPBusiness(kt)
	if err != nil {
		logs.Errorf("query BSCP business failed, err: %v", err)
		return
	}
	logs.Infof("query BSCP business success, total businesses: %d", len(bizList))

	// Query host information by business ID
	for _, biz := range bizList {
		if err := c.syncBusinessHosts(kt, biz); err != nil {
			logs.Errorf("sync business %d hosts failed, err: %v", biz, err)
			// if sync failed, continue to sync next business
			continue
		}
	}
}

// syncBusinessHosts sync host information for a single business
func (c *SyncBizHost) syncBusinessHosts(kt *kit.Kit, bizID int) error {
	start := 0
	limit := c.pageSize
	totalSynced := 0
	for {
		req := &bkcmdb.ListBizHostsRequest{
			BkBizID: bizID,
			Page: bkcmdb.PageParam{
				Start: start,
				Limit: limit,
			},
			Fields: []string{"bk_biz_id", "bk_host_id", "bk_agent_id"},
		}

		// Apply rate limiting before each request
		if err := c.rateLimiter.Wait(kt.Ctx); err != nil {
			return fmt.Errorf("rate limiter wait failed: %w", err)
		}

		hostResult, err := c.cmdbService.ListBizHosts(kt.Ctx, req)
		if err != nil {
			return fmt.Errorf("list biz hosts failed: %w", err)
		}

		if !hostResult.Result {
			return fmt.Errorf("list biz hosts failed: %s", hostResult.Message)
		}

		// If current page has no data, query is complete
		if len(hostResult.Data.Info) == 0 {
			break
		}

		var batchBizHosts []*table.BizHost
		for _, host := range hostResult.Data.Info {
			bizHost := &table.BizHost{
				BizID:   bizID,
				HostID:  host.BkHostID,
				AgentID: host.BkAgentID,
			}
			batchBizHosts = append(batchBizHosts, bizHost)
		}
		if len(batchBizHosts) > 0 {
			if err := c.set.BizHost().BatchUpsert(kt, batchBizHosts); err != nil {
				return fmt.Errorf("batch upsert biz hosts failed: %w", err)
			}
			totalSynced += len(batchBizHosts)
		}

		// If returned data is less than limit, it's the last page
		if len(hostResult.Data.Info) < limit {
			break
		}

		// Prepare to query next page
		start += limit
	}

	logs.Infof("completed sync for business %d, total hosts: %d", bizID, totalSynced)
	return nil
}

// queryBSCPBusiness query BSCP businesses
func (c *SyncBizHost) queryBSCPBusiness(kt *kit.Kit) ([]int, error) {
	m := c.set.GenQuery().App
	bizIDs, err := c.set.GenQuery().App.WithContext(kt.Ctx).
		Select(m.BizID.Distinct()).
		Find()
	if err != nil {
		return nil, fmt.Errorf("query biz IDs failed: %w", err)
	}

	var bizList []int
	for _, app := range bizIDs {
		bizList = append(bizList, int(app.BizID))
	}

	return bizList, nil
}

// cacheEventCursors 缓存事件cursor到Redis
func (c *SyncBizHost) cacheEventCursors(kt *kit.Kit) error {
	// 获取3分钟前的时间戳
	threeMinutesAgo := time.Now().Add(-3 * time.Minute).Unix()

	// 获取业务主机关系事件的cursor
	bizHostCursor, err := c.getEventCursor(kt, hostRelation, threeMinutesAgo)
	if err != nil {
		logs.Errorf("get biz host cursor failed, err: %v", err)
	} else if bizHostCursor != "" {
		if err := c.redisClient.Set(kt.Ctx, bizHostCursorKey, bizHostCursor, 7*24*3600); err != nil {
			logs.Errorf("cache biz host cursor to redis failed, err: %v", err)
		} else {
			logs.Infof("cached biz host cursor to redis: %s", bizHostCursor)
		}
	}

	// 获取主机详情更新事件的cursor
	hostDetailCursor, err := c.getEventCursor(kt, "host", threeMinutesAgo)
	if err != nil {
		logs.Errorf("get host detail cursor failed, err: %v", err)
	} else if hostDetailCursor != "" {
		if err := c.redisClient.Set(kt.Ctx, hostDetailCursorKey, hostDetailCursor, 7*24*3600); err != nil {
			logs.Errorf("cache host detail cursor to redis failed, err: %v", err)
		} else {
			logs.Infof("cached host detail cursor to redis: %s", hostDetailCursor)
		}
	}

	return nil
}

// getEventCursor 获取指定时间点的事件cursor
func (c *SyncBizHost) getEventCursor(kt *kit.Kit, resourceType string, startTime int64) (string, error) {
	req := &bkcmdb.WatchResourceRequest{
		BkResource:   resourceType,
		BkEventTypes: []string{"create", "update"},
		BkFields:     []string{"bk_biz_id", "bk_host_id"},
		BkStartFrom:  &startTime,
	}

	switch resourceType {
	case hostRelation:
		watchResult, err := c.cmdbService.WatchHostRelationResource(kt.Ctx, req)
		if err != nil {
			return "", err
		}
		if !watchResult.Result {
			return "", fmt.Errorf("watch host relation resource failed: %s", watchResult.Message)
		}
		// 从响应中提取最后一个事件的cursor
		if len(watchResult.Data.BkEvents) > 0 {
			lastEvent := watchResult.Data.BkEvents[len(watchResult.Data.BkEvents)-1]
			return lastEvent.BkCursor, nil
		}
	case host:
		watchResult, err := c.cmdbService.WatchHostResource(kt.Ctx, req)
		if err != nil {
			return "", err
		}
		if !watchResult.Result {
			return "", fmt.Errorf("watch host resource failed: %s", watchResult.Message)
		}
		// 从响应中提取最后一个事件的cursor
		if len(watchResult.Data.BkEvents) > 0 {
			lastEvent := watchResult.Data.BkEvents[len(watchResult.Data.BkEvents)-1]
			return lastEvent.BkCursor, nil
		}
	default:
		return "", fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	// 未监听到事件，返回空字符串
	return "", nil
}

// GetCachedCursor 从Redis获取缓存的cursor
func (c *SyncBizHost) GetCachedCursor(kt *kit.Kit, key string) string {
	cursor, err := c.redisClient.Get(kt.Ctx, key)
	if err != nil {
		logs.Errorf("get cached cursor from redis failed, key: %s, err: %v", key, err)
		return ""
	}
	return cursor
}

// UpdateCachedCursor 更新Redis中缓存的cursor
func (c *SyncBizHost) UpdateCachedCursor(kt *kit.Kit, key, cursor string) {
	if err := c.redisClient.Set(kt.Ctx, key, cursor, 7*24*3600); err != nil {
		logs.Errorf("update cached cursor to redis failed, key: %s, cursor: %s, err: %v", key, cursor, err)
	} else {
		logs.Infof("updated cached cursor to redis for %s: %s", key, cursor)
	}
}
