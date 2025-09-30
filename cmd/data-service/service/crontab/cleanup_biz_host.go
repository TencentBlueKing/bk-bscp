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
	// biz host cleanup task interval
	defaultCleanupBizHostInterval = 6 * time.Hour
	// number of records to process each time
	defaultCleanupBatchSize = 1000
	// CMDB request rate limit
	defaultCleanupQpsLimit = 50.0
)

// NewCleanupBizHost init cleanup biz host task
func NewCleanupBizHost(
	set dao.Set,
	sd serviced.Service,
	cmdbService bkcmdb.Service,
	qpsLimit float64,
) CleanupBizHost {
	if qpsLimit <= 0 || qpsLimit > defaultCleanupQpsLimit {
		qpsLimit = defaultCleanupQpsLimit
	}
	rateLimiter := rate.NewLimiter(rate.Limit(qpsLimit), 1)

	return CleanupBizHost{
		set:         set,
		state:       sd,
		cmdbService: cmdbService,
		rateLimiter: rateLimiter,
	}
}

// CleanupBizHost cleanup invalid biz host relationships
type CleanupBizHost struct {
	set         dao.Set
	state       serviced.Service
	cmdbService bkcmdb.Service
	rateLimiter *rate.Limiter
	mutex       sync.Mutex
}

// Run start cleanup task
func (c *CleanupBizHost) Run() {
	logs.Infof("start cleanup biz host task")
	notifier := shutdown.AddNotifier()

	// task1: cleanup invalid biz host relationships
	go func() {
		ticker := time.NewTicker(defaultCleanupBizHostInterval)
		defer ticker.Stop()
		for {
			kt := kit.New()
			ctx, cancel := context.WithCancel(kt.Ctx)
			kt.Ctx = ctx

			select {
			case <-notifier.Signal:
				logs.Infof("stop cleanup biz host success")
				cancel()
				notifier.Done()
				return
			case <-ticker.C:
				if !c.state.IsMaster() {
					logs.Infof("current service instance is slave, skip cleanup biz host")
					continue
				}
				logs.Infof("starts to cleanup invalid biz host relationships")
				c.cleanupBizHost(kt)
			}
		}
	}()
}

// cleanupBizHost cleanup invalid biz host relationships
func (c *CleanupBizHost) cleanupBizHost(kt *kit.Kit) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		logs.Infof("cleanup biz host completed in %v", duration)
	}()

	// query oldest biz host relationships
	oldestRecords, err := c.queryOldestBizHosts(kt)
	if err != nil {
		logs.Errorf("query oldest biz host records failed, err: %v", err)
		return
	}

	if len(oldestRecords) == 0 {
		logs.Infof("no biz host records to cleanup")
		return
	}

	logs.Infof("found %d oldest biz host records to validate", len(oldestRecords))

	// group by biz ID
	bizGroups := c.groupByBizID(oldestRecords)
	logs.Infof("grouped into %d businesses", len(bizGroups))

	// validate each biz host relationships
	totalDeleted := 0
	for bizID, records := range bizGroups {
		deleted := c.validateAndCleanupBizHosts(kt, bizID, records)
		totalDeleted += deleted
		logs.Infof("cleaned up %d invalid records for biz %d", deleted, bizID)
	}

	logs.Infof("cleanup completed, total deleted: %d records", totalDeleted)
}

// queryOldestBizHosts query oldest biz host relationships
func (c *CleanupBizHost) queryOldestBizHosts(kt *kit.Kit) ([]*table.BizHost, error) {
	m := c.set.GenQuery().BizHost
	records, err := c.set.GenQuery().BizHost.WithContext(kt.Ctx).
		Order(m.LastUpdated). // order by last updated time
		Limit(defaultCleanupBatchSize).
		Find()

	if err != nil {
		return nil, fmt.Errorf("query oldest biz hosts failed: %w", err)
	}

	return records, nil
}

// groupByBizID group by biz ID
func (c *CleanupBizHost) groupByBizID(records []*table.BizHost) map[int][]*table.BizHost {
	groups := make(map[int][]*table.BizHost)
	for _, record := range records {
		groups[record.BizID] = append(groups[record.BizID], record)
	}
	return groups
}

// validateAndCleanupBizHosts validate and cleanup specified biz host relationships
func (c *CleanupBizHost) validateAndCleanupBizHosts(kt *kit.Kit, bizID int, records []*table.BizHost) int {
	if len(records) == 0 {
		return 0
	}

	// batch process host IDs
	totalDeleted := 0
	batchSize := 500
	for i := 0; i < len(records); i += batchSize {
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}

		batch := records[i:end]
		deleted, err := c.validateAndCleanupBatch(kt, bizID, batch)
		if err != nil {
			logs.Errorf("validate and cleanup batch failed, bizID: %d, batch: %d-%d, err: %v",
				bizID, i, end-1, err)
			continue
		}
		totalDeleted += deleted
	}

	return totalDeleted
}

// validateAndCleanupBatch validate and cleanup a batch of host relationships
func (c *CleanupBizHost) validateAndCleanupBatch(kt *kit.Kit, bizID int, records []*table.BizHost) (int, error) {
	// apply rate limiter
	if err := c.rateLimiter.Wait(kt.Ctx); err != nil {
		return 0, fmt.Errorf("rate limiter wait failed: %w", err)
	}

	// extract host IDs
	hostIDs := make([]int, 0, len(records))
	for _, record := range records {
		hostIDs = append(hostIDs, record.HostID)
	}

	// call new CMDB API to get valid host biz relationships
	req := &bkcmdb.FindHostBizRelationsRequest{
		BkBizID:  bizID,
		BkHostID: hostIDs,
	}

	relationResult, err := c.cmdbService.FindHostBizRelations(kt.Ctx, req)
	if err != nil {
		return 0, fmt.Errorf("find host biz relations failed: %w", err)
	}

	if !relationResult.Result {
		return 0, fmt.Errorf("find host biz relations failed: %s", relationResult.Message)
	}

	// build valid host IDs set (only include hosts with binding relations)
	validHostIDs := make(map[int]bool)
	for _, relation := range relationResult.Data {
		validHostIDs[relation.BkHostID] = true
	}

	// check and delete invalid records
	deletedCount := 0
	for _, record := range records {
		if !validHostIDs[record.HostID] {
			// host is no longer bound to this biz, delete record
			if err := c.set.BizHost().Delete(kt, record.BizID, record.HostID); err != nil {
				logs.Errorf("delete invalid biz host record failed, bizID: %d, hostID: %d, err: %v",
					record.BizID, record.HostID, err)
				continue
			}
			deletedCount++
			logs.Infof("deleted invalid biz host record: bizID=%d, hostID=%d", record.BizID, record.HostID)
		}
	}

	return deletedCount, nil
}
