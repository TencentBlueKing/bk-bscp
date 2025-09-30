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
	defaultCleanupBizHostInterval = 6 * time.Hour
	// 每次处理的记录数
	defaultCleanupBatchSize = 1000
	// CMDB 请求限流
	defaultCleanupQpsLimit = 50.0
	// 重复主机清理间隔
	defaultCleanupDuplicateHostInterval = 3 * time.Minute
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
	// 创建限流器
	rateLimiter := rate.NewLimiter(rate.Limit(qpsLimit), 1)

	return CleanupBizHost{
		set:         set,
		state:       sd,
		cmdbService: cmdbService,
		rateLimiter: rateLimiter,
	}
}

// CleanupBizHost 清理失效的业务主机关系
type CleanupBizHost struct {
	set         dao.Set
	state       serviced.Service
	cmdbService bkcmdb.Service
	rateLimiter *rate.Limiter
	mutex       sync.Mutex
}

// Run 启动清理任务
func (c *CleanupBizHost) Run() {
	logs.Infof("start cleanup biz host task")
	notifier := shutdown.AddNotifier()

	// 任务1：清理失效的业务主机关系
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
				// if !c.state.IsMaster() {
				// 	logs.Infof("current service instance is slave, skip cleanup biz host")
				// 	continue
				// }
				logs.Infof("starts to cleanup invalid biz host relationships")
				c.cleanupBizHost(kt)
			}
		}
	}()

	// 任务2：清理重复主机关联
	go func() {
		ticker := time.NewTicker(defaultCleanupDuplicateHostInterval)
		defer ticker.Stop()
		for {
			kt := kit.New()
			ctx, cancel := context.WithCancel(kt.Ctx)
			kt.Ctx = ctx

			select {
			case <-notifier.Signal:
				logs.Infof("stop cleanup duplicate host success")
				cancel()
				return
			case <-ticker.C:
				// if !c.state.IsMaster() {
				// 	logs.Infof("current service instance is slave, skip cleanup duplicate host")
				// 	continue
				// }
				logs.Infof("starts to cleanup duplicate host relationships")
				c.cleanupDuplicateHosts(kt)
			}
		}
	}()
}

// cleanupBizHost 清理失效的业务主机关系
func (c *CleanupBizHost) cleanupBizHost(kt *kit.Kit) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

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

// cleanupDuplicateHosts 清理重复主机关联
func (c *CleanupBizHost) cleanupDuplicateHosts(kt *kit.Kit) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		logs.Infof("cleanup duplicate hosts completed in %v", duration)
	}()

	// 查询重复的主机ID
	duplicateHosts, err := c.queryDuplicateHosts(kt)
	if err != nil {
		logs.Errorf("query duplicate hosts failed, err: %v", err)
		return
	}

	if len(duplicateHosts) == 0 {
		logs.Infof("no duplicate hosts found")
		return
	}

	logs.Infof("found %d duplicate hosts to validate", len(duplicateHosts))

	bizGroups := c.groupByBizID(duplicateHosts)
	totalCleaned := 0
	for bizID, records := range bizGroups {
		cleanedCount, err := c.validateAndCleanupBizHosts(kt, bizID, records)
		if err != nil {
			logs.Errorf("cleanup duplicate host %d failed, err: %v", bizID, err)
			continue
		}
		totalCleaned += cleanedCount
	}

	logs.Infof("duplicate host cleanup completed, total deleted: %d records", totalCleaned)
}

// queryDuplicateHosts 查询重复的主机ID并按业务分组
func (c *CleanupBizHost) queryDuplicateHosts(kt *kit.Kit) ([]*table.BizHost, error) {
	// 使用子查询找出重复的主机ID
	m := c.set.GenQuery().BizHost
	var duplicateHostIDs []int

	// 查询出现次数大于1的主机ID
	err := c.set.GenQuery().BizHost.WithContext(kt.Ctx).
		Select(m.HostID).
		Group(m.HostID).
		Having(m.HostID.Count().Gt(1)).
		Scan(&duplicateHostIDs)

	if err != nil {
		return nil, fmt.Errorf("query duplicate host IDs failed: %w", err)
	}

	if len(duplicateHostIDs) == 0 {
		return nil, nil
	}

	// 查询这些重复主机的所有记录
	records, err := c.set.GenQuery().BizHost.WithContext(kt.Ctx).
		Where(m.HostID.In(duplicateHostIDs...)).
		Find()

	if err != nil {
		return nil, fmt.Errorf("query duplicate host records failed: %w", err)
	}

	return records, nil
}
