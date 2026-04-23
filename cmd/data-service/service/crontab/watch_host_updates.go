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
	"errors"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	// watch host update event
	hostUpdateEvent = "update"
	// watch resource types
	host = "host"
	// Config keys for cursor storage
	hostDetailCursorKey = "host_detail_cursor"
)

// getHostDetailCursorKey 获取带租户前缀的游标 key
func getHostDetailCursorKey(tenantID string) string {
	if tenantID == "" {
		return hostDetailCursorKey
	}
	return fmt.Sprintf("%s-%s", tenantID, hostDetailCursorKey)
}

// NewWatchHostUpdates init watch host updates
func NewWatchHostUpdates(
	set dao.Set,
	sd serviced.Service,
	cmdbService bkcmdb.Service,
	interval time.Duration,
) WatchHostUpdates {
	// when the cursor is lost, listen from 3 minutes ago
	timeAgo := time.Now().Add(-3 * time.Minute).Unix()
	return WatchHostUpdates{
		set:         set,
		state:       sd,
		cmdbService: cmdbService,
		timeAgo:     timeAgo,
		interval:    interval,
	}
}

// WatchHostUpdates watch host update events
type WatchHostUpdates struct {
	set         dao.Set
	state       serviced.Service
	cmdbService bkcmdb.Service
	timeAgo     int64
	interval    time.Duration
	mutex       sync.Mutex
}

// Run starts the watch task for host updates
func (w *WatchHostUpdates) Run() {
	logs.Infof("start watch host updates task")
	notifier := shutdown.AddNotifier()

	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			kt := kit.New()
			ctx, cancel := context.WithCancel(kt.Ctx)
			kt.Ctx = ctx

			select {
			case <-notifier.Signal:
				logs.Infof("stop host update watch success")
				cancel()
				return
			case <-ticker.C:
				if !w.state.IsMaster() {
					logs.Infof("current service instance is slave, skip host update watch")
					continue
				}
				logs.Infof("host update watch triggered")
				w.watchHostUpdatesByTenant(kt)
			}
		}
	}()
}

// watchHostUpdatesByTenant 按租户监听主机更新事件
func (w *WatchHostUpdates) watchHostUpdatesByTenant(kt *kit.Kit) {
	// 多租户模式：从 app 表获取租户列表并逐个监听
	if cc.DataService().FeatureFlags.EnableMultiTenantMode {
		apps, err := w.set.App().GetDistinctTenantIDs(kt)
		if err != nil {
			logs.Errorf("get distinct tenant IDs failed, err: %v", err)
			return
		}

		if len(apps) == 0 {
			logs.Warnf("no tenants found in app table for watch host updates")
			return
		}

		for _, app := range apps {
			if app.Spec.TenantID == "" {
				continue
			}
			tenantKit := *kt
			tenantKit.TenantID = app.Spec.TenantID
			tenantKit.Ctx = tenantKit.InternalRpcCtx()
			w.watchHostUpdates(&tenantKit)
		}
		return
	}

	// 单租户模式
	w.watchHostUpdates(kt)
}

// watchHostUpdates watch host update events
func (w *WatchHostUpdates) watchHostUpdates(kt *kit.Kit) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	// Listen to host update events
	req := &bkcmdb.WatchResourceRequest{
		BkResource:   host,
		BkEventTypes: []string{hostUpdateEvent},
		BkFields:     []string{"bk_host_id", "bk_agent_id"},
	}
	// get cursor key with tenant prefix
	cursorKey := getHostDetailCursorKey(kt.TenantID)
	config, err := w.set.Config().GetConfig(kt, cursorKey)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logs.Errorf("get cached cursor from config failed, key: %s, err: %v", cursorKey, err)
			return
		}
		// cursor not found, use timestamp
		req.BkStartFrom = &w.timeAgo
	} else if config != nil && config.Value != "" {
		req.BkCursor = config.Value
	} else {
		req.BkStartFrom = &w.timeAgo
	}

	watchResult, err := w.cmdbService.WatchHostResource(kt.Ctx, req)
	if err != nil {
		logs.Errorf("watch host resource failed, err: %v", err)
		return
	}

	if !watchResult.BkWatched {
		// No events found, skip
		return
	}
	logs.Infof("watch host resource success, events: %d", len(watchResult.BkEvents))

	// Process host update events
	if len(watchResult.BkEvents) > 0 {
		w.processHostEvents(kt, watchResult.BkEvents)
		// update cursor to config table
		lastEvent := watchResult.BkEvents[len(watchResult.BkEvents)-1]
		config := &table.Config{
			Key:   cursorKey,
			Value: lastEvent.BkCursor,
		}
		err := w.set.Config().UpsertConfig(kt, []*table.Config{config})
		if err != nil {
			logs.Errorf("update host detail cursor to config failed, err: %v", err)
		}
	}
}

// compressHostUpdateEvents collapses host update events by bk_host_id, keeping
// only the last seen agentID per hostID. Order of first appearance is
// preserved for deterministic downstream processing.
func compressHostUpdateEvents(events []bkcmdb.HostEvent) map[int]string {
	latest := make(map[int]string, len(events))
	for _, ev := range events {
		if ev.BkEventType != hostUpdateEvent {
			logs.Warnf("unknown host event type: %s", ev.BkEventType)
			continue
		}
		if ev.BkDetail == nil || ev.BkDetail.BkHostID == nil {
			logs.Warnf("invalid host update event detail: %+v", ev.BkDetail)
			continue
		}
		agentID := ""
		if ev.BkDetail.BkAgentID != nil {
			agentID = *ev.BkDetail.BkAgentID
		}
		latest[*ev.BkDetail.BkHostID] = agentID
	}
	return latest
}

// processHostEvents compresses host update events to the latest agentID per
// hostID, batch-loads existing biz_host rows once, then batch-upserts only the
// rows whose agentID actually changed.
func (w *WatchHostUpdates) processHostEvents(kt *kit.Kit, events []bkcmdb.HostEvent) {
	latest := compressHostUpdateEvents(events)
	if len(latest) == 0 {
		return
	}
	if len(latest) != len(events) {
		logs.Infof("host update events compressed: %d -> %d", len(events), len(latest))
	}

	hostIDs := make([]uint, 0, len(latest))
	for hostID := range latest {
		hostIDs = append(hostIDs, uint(hostID))
	}

	existing, err := w.set.BizHost().ListAllByHostIDs(kt, hostIDs)
	if err != nil {
		logs.Errorf("query biz hosts by hostIDs failed, count=%d, err: %v", len(hostIDs), err)
		return
	}
	if len(existing) == 0 {
		return
	}

	toUpdate := make([]*table.BizHost, 0, len(existing))
	for _, bizHost := range existing {
		newAgentID, ok := latest[int(bizHost.HostID)]
		if !ok {
			continue
		}
		if bizHost.AgentID == newAgentID {
			continue
		}
		bizHost.AgentID = newAgentID
		toUpdate = append(toUpdate, bizHost)
	}

	if len(toUpdate) == 0 {
		return
	}

	if err := w.set.BizHost().BatchUpsert(kt, toUpdate); err != nil {
		logs.Warnf("batch upsert biz host agentID failed, count=%d, err: %v", len(toUpdate), err)
		return
	}
}

// InitHostDetailCursor initializes host detail cursor to the latest position
// This function gets the latest cursor from CMDB and updates it to config table
func InitHostDetailCursor(tenantID string, set dao.Set, cmdbService bkcmdb.Service, timeAgo int64) error {
	logs.Infof("start init host detail cursor for tenant: %s", tenantID)
	kt := kit.NewWithTenant(tenantID)
	ctx, cancel := context.WithTimeout(kt.Ctx, 10*time.Second)
	defer cancel()
	kt.Ctx = ctx

	req := &bkcmdb.WatchResourceRequest{
		BkResource:   host,
		BkEventTypes: []string{hostUpdateEvent},
		BkFields:     []string{"bk_host_id", "bk_agent_id"},
		BkStartFrom:  &timeAgo,
	}

	watchResult, err := cmdbService.WatchHostResource(kt.Ctx, req)
	if err != nil {
		return fmt.Errorf("watch host resource failed: %w", err)
	}

	if len(watchResult.BkEvents) == 0 {
		// 监听成功情况下，若无事件则会返回一个不含详情但是含有cursor的事件
		return fmt.Errorf("watch host resource failed: no events found")
	}

	cursorKey := getHostDetailCursorKey(tenantID)
	cursor := watchResult.BkEvents[len(watchResult.BkEvents)-1].BkCursor
	config := &table.Config{
		Key:   cursorKey,
		Value: cursor,
	}

	err = set.Config().UpsertConfig(kt, []*table.Config{config})
	if err != nil {
		return fmt.Errorf("update host detail cursor to config failed: %w", err)
	}

	logs.Infof("successfully initialized host detail cursor to: %s (tenant: %s)", cursor, tenantID)
	return nil
}
