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

	"golang.org/x/time/rate"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	// sync data once a week
	defaultSyncBizHostInterval = 7 * 24 * time.Hour
	// Default QPS limit for CMDB requests
	defaultQpsLimit = 80.0
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
	// mutex for sync biz host
	mutex sync.Mutex
}

// Run the sync biz host task
func (c *SyncBizHost) Run() {
	logs.Infof("start sync biz host task")
	notifier := shutdown.AddNotifier()
	go func() {
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
				c.SyncBizHost(kt)
			}
		}
	}()
}

// SyncBizHost sync business host relationship
func (c *SyncBizHost) SyncBizHost(kt *kit.Kit) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		logs.Infof("sync biz host completed in %v", duration)
	}()

	// get 3 minutes ago event cursor and cache to redis
	if err := c.cacheEventCursors(kt); err != nil {
		logs.Errorf("cache event cursors failed, err: %v", err)
		return
	}

	// Query BSCP businesses
	bizList, err := c.queryBSCPBusiness(kt)
	if err != nil {
		logs.Errorf("query BSCP business failed, err: %v", err)
		return
	}

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
	for {
		req := &bkcmdb.ListBizHostsRequest{
			BkBizID: bizID,
			Page: bkcmdb.PageParam{
				Start: start,
				Limit: limit,
			},
			Fields: []string{"bk_biz_id", "bk_host_id", "bk_agent_id", "bk_host_innerip"},
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
				BizID:         bizID,
				HostID:        host.BkHostID,
				AgentID:       host.BkAgentID,
				BKHostInnerIP: host.BkHostInnerIP,
			}
			batchBizHosts = append(batchBizHosts, bizHost)
		}
		if len(batchBizHosts) > 0 {
			if err := c.set.BizHost().BatchUpsert(kt, batchBizHosts); err != nil {
				return fmt.Errorf("batch upsert biz hosts failed: %w", err)
			}
		}

		// If returned data is less than limit, it's the last page
		if len(hostResult.Data.Info) < limit {
			break
		}

		// Prepare to query next page
		start += limit
	}

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

// cacheEventCursors cache event cursor to Redis
func (c *SyncBizHost) cacheEventCursors(kt *kit.Kit) error {
	// get 3 minutes ago timestamp
	threeMinutesAgo := time.Now().Add(-3 * time.Minute).Unix()

	// get biz host relation event cursor
	bizHostCursor, err := c.getEventCursor(kt, hostRelation, threeMinutesAgo)
	if err != nil {
		return fmt.Errorf("get biz host cursor failed: %w", err)
	}
	if bizHostCursor != "" {
		if err = c.redisClient.Set(kt.Ctx, bizHostCursorKey, bizHostCursor, 7*24*3600); err != nil {
			return fmt.Errorf("cache biz host cursor to redis failed: %w", err)
		}
	}

	// get host detail update event cursor
	hostDetailCursor, err := c.getEventCursor(kt, "host", threeMinutesAgo)
	if err != nil {
		return fmt.Errorf("get host detail cursor failed: %w", err)
	}
	if hostDetailCursor != "" {
		if err := c.redisClient.Set(kt.Ctx, hostDetailCursorKey, hostDetailCursor, 7*24*3600); err != nil {
			return fmt.Errorf("cache host detail cursor to redis failed: %w", err)
		}
	}

	return nil
}

// getEventCursor get event cursor at specified time
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
		if len(watchResult.Data.BkEvents) == 0 {
			return "", nil
		}
		// extract last event cursor from response
		lastEvent := watchResult.Data.BkEvents[len(watchResult.Data.BkEvents)-1]
		return lastEvent.BkCursor, nil
	case host:
		watchResult, err := c.cmdbService.WatchHostResource(kt.Ctx, req)
		if err != nil {
			return "", err
		}
		if !watchResult.Result {
			return "", fmt.Errorf("watch host resource failed: %s", watchResult.Message)
		}
		if len(watchResult.Data.BkEvents) == 0 {
			return "", nil
		}
		// extract last event cursor from response
		lastEvent := watchResult.Data.BkEvents[len(watchResult.Data.BkEvents)-1]
		return lastEvent.BkCursor, nil
	default:
		return "", fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}
