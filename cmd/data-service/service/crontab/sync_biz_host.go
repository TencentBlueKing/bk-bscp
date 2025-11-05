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
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	// Default QPS limit for list biz hosts api
	listBizHostsApiQpsLimit = 80.0
	// Default page size for list host requests
	defaultPageSize = 500
)

// NewSyncBizHost init sync biz host with configurable settings
func NewSyncBizHost(
	set dao.Set,
	sd serviced.Service,
	cmdbService bkcmdb.Service,
	qpsLimit float64,
	syncInterval time.Duration,
) SyncBizHost {
	if qpsLimit <= 0 || qpsLimit > listBizHostsApiQpsLimit {
		qpsLimit = listBizHostsApiQpsLimit
	}

	// Create rate limiter with configurable QPS
	rateLimiter := rate.NewLimiter(rate.Limit(qpsLimit), 1)

	return SyncBizHost{
		set:          set,
		state:        sd,
		cmdbService:  cmdbService,
		rateLimiter:  rateLimiter,
		qpsLimit:     qpsLimit,
		syncInterval: syncInterval,
	}
}

// SyncBizHost sync business host relationship
type SyncBizHost struct {
	set         dao.Set
	state       serviced.Service
	cmdbService bkcmdb.Service
	// rate limiter for CMDB requests
	rateLimiter *rate.Limiter
	// qps limit for CMDB requests
	qpsLimit float64
	// sync interval for biz host sync
	syncInterval time.Duration
	// mutex for sync biz host
	mutex sync.Mutex
}

// Run the sync biz host task
func (c *SyncBizHost) Run() {
	logs.Infof("start sync biz host task")
	notifier := shutdown.AddNotifier()
	go func() {
		ticker := time.NewTicker(c.syncInterval)
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
				if !c.state.IsMaster() {
					logs.Infof("current service instance is slave, skip sync biz host")
					continue
				}
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

	// Query BSCP businesses
	bizList, err := c.queryBSCPBusiness(kt)
	if err != nil {
		logs.Errorf("query BSCP business failed, err: %v", err)
		return
	}
	logs.Infof("query BSCP business success, total business: %d", len(bizList))

	// Query host information by business ID
	for _, biz := range bizList {
		if err := c.syncBusinessHosts(kt, int(biz)); err != nil {
			logs.Errorf("sync business %d hosts failed, err: %v", biz, err)
			// if sync failed, continue to sync next business
			continue
		}
	}
}

// syncBusinessHosts sync host information for a single business
func (c *SyncBizHost) syncBusinessHosts(kt *kit.Kit, bizID int) error {
	start := 0
	limit := defaultPageSize
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

		// If current page has no data, query is complete
		if len(hostResult.Info) == 0 {
			break
		}

		var batchBizHosts []*table.BizHost
		for _, host := range hostResult.Info {
			bizHost := &table.BizHost{
				BizID:         uint(bizID),
				HostID:        uint(host.BkHostID),
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
		if len(hostResult.Info) < limit {
			break
		}

		// Prepare to query next page
		start += limit
	}

	return nil
}

// queryBSCPBusiness query BSCP businesses
func (c *SyncBizHost) queryBSCPBusiness(kt *kit.Kit) ([]uint32, error) {
	bizList, err := c.set.App().QueryDistinctBizIDs(kt)
	if err != nil {
		return nil, fmt.Errorf("query biz IDs failed: %w", err)
	}

	return bizList, nil
}
