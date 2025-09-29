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
	"time"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"golang.org/x/time/rate"
)

const (
	// 每天执行一次清理任务
	defaultCleanupBizHostInterval = 1 * time.Minute
	// 每次处理的记录数
	defaultCleanupBatchSize = 1000
	// CMDB 请求限流
	defaultCleanupQpsLimit = 10.0
)

// NewCleanupBizHost init cleanup biz host task
func NewCleanupBizHost(
	set dao.Set, sd serviced.Service, cmdbService bkcmdb.Service, syncBizHost *SyncBizHost) CleanupBizHost {
	// 创建限流器
	rateLimiter := rate.NewLimiter(rate.Limit(defaultCleanupQpsLimit), 1)

	return CleanupBizHost{
		set:         set,
		state:       sd,
		cmdbService: cmdbService,
		rateLimiter: rateLimiter,
		syncBizHost: syncBizHost,
	}
}

// CleanupBizHost 清理失效的业务主机关系
type CleanupBizHost struct {
	set         dao.Set
	state       serviced.Service
	cmdbService bkcmdb.Service
	rateLimiter *rate.Limiter
	// 引用同步锁和cursor缓存
	syncBizHost *SyncBizHost
}

// Run 启动清理任务
func (c *CleanupBizHost) Run() {
	logs.Infof("start cleanup biz host task")
	notifier := shutdown.AddNotifier()
	go func() {
		// 启动定时器，按间隔执行
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
				// if !c.state.IsMaster() {
				// 	logs.Infof("current service instance is slave, skip cleanup biz host")
				// 	continue
				// }
				logs.Infof("starts to cleanup invalid biz host relationships")
				c.cleanupBizHost(kt)
			}
		}
	}()
}

// cleanupBizHost 清理失效的业务主机关系
func (c *CleanupBizHost) cleanupBizHost(kt *kit.Kit) {
	// 获取同步锁，阻塞等待直到获得锁
	logs.Infof("waiting for sync lock to start cleanup")
	c.acquireCleanupLock()
	defer c.releaseCleanupLock()
	logs.Infof("acquired sync lock, starting cleanup")

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		logs.Infof("cleanup biz host completed in %v", duration)
	}()

	// 查询最久未更新的业务主机关系记录
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

	// 按业务ID分组
	bizGroups := c.groupByBizID(oldestRecords)
	logs.Infof("grouped into %d businesses", len(bizGroups))

	// 验证每个业务的主机关系
	totalDeleted := 0
	for bizID, records := range bizGroups {
		deleted, err := c.validateAndCleanupBizHosts(kt, bizID, records)
		if err != nil {
			logs.Errorf("validate and cleanup biz %d hosts failed, err: %v", bizID, err)
			continue
		}
		totalDeleted += deleted
		logs.Infof("cleaned up %d invalid records for biz %d", deleted, bizID)
	}

	logs.Infof("cleanup completed, total deleted: %d records", totalDeleted)
}

// queryOldestBizHosts 查询最久未更新的业务主机关系记录
func (c *CleanupBizHost) queryOldestBizHosts(kt *kit.Kit) ([]*table.BizHost, error) {
	m := c.set.GenQuery().BizHost
	records, err := c.set.GenQuery().BizHost.WithContext(kt.Ctx).
		Order(m.LastUpdated). // 按最后更新时间升序排列
		Limit(defaultCleanupBatchSize).
		Find()

	if err != nil {
		return nil, fmt.Errorf("query oldest biz hosts failed: %w", err)
	}

	return records, nil
}

// groupByBizID 按业务ID分组
func (c *CleanupBizHost) groupByBizID(records []*table.BizHost) map[int][]*table.BizHost {
	groups := make(map[int][]*table.BizHost)
	for _, record := range records {
		groups[record.BizID] = append(groups[record.BizID], record)
	}
	return groups
}

// validateAndCleanupBizHosts 验证并清理指定业务的主机关系
func (c *CleanupBizHost) validateAndCleanupBizHosts(kt *kit.Kit, bizID int, records []*table.BizHost) (int, error) {
	if len(records) == 0 {
		return 0, nil
	}

	// 分批处理主机ID，每批最多500个
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

	return totalDeleted, nil
}

// validateAndCleanupBatch 验证并清理一批主机关系
func (c *CleanupBizHost) validateAndCleanupBatch(kt *kit.Kit, bizID int, records []*table.BizHost) (int, error) {
	// 应用限流
	if err := c.rateLimiter.Wait(kt.Ctx); err != nil {
		return 0, fmt.Errorf("rate limiter wait failed: %w", err)
	}

	// 提取主机ID列表
	hostIDs := make([]int, 0, len(records))
	for _, record := range records {
		hostIDs = append(hostIDs, record.HostID)
	}

	// 调用新的 CMDB API 获取有效的主机业务关系
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

	// 构建有效的主机ID集合（只包含还存在绑定关系的主机）
	validHostIDs := make(map[int]bool)
	for _, relation := range relationResult.Data {
		validHostIDs[relation.BkHostID] = true
	}

	// 检查并删除失效的记录
	deletedCount := 0
	for _, record := range records {
		if !validHostIDs[record.HostID] {
			// 主机不再与该业务绑定，删除记录
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

// acquireCleanupLock 获取清理锁（阻塞等待）
func (c *CleanupBizHost) acquireCleanupLock() {
	c.syncBizHost.syncLock.Lock()
}

// releaseCleanupLock 释放清理锁
func (c *CleanupBizHost) releaseCleanupLock() {
	c.syncBizHost.syncLock.Unlock()
}
