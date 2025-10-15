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
				w.watchHostUpdates(kt)
			}
		}
	}()
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
	config, err := w.set.Config().GetConfig(kt, hostDetailCursorKey)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logs.Errorf("get cached cursor from config failed, key: %s, err: %v", hostDetailCursorKey, err)
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

	if !watchResult.Result {
		logs.Errorf("watch host resource failed: %s", watchResult.Message)
		return
	}
	if !watchResult.Data.BkWatched {
		// No events found, skip
		return
	}
	logs.Infof("watch host resource success, events: %d", len(watchResult.Data.BkEvents))

	// Process host update events
	if len(watchResult.Data.BkEvents) > 0 {
		w.processHostEvents(kt, watchResult.Data.BkEvents)
		// update cursor to config table
		lastEvent := watchResult.Data.BkEvents[len(watchResult.Data.BkEvents)-1]
		config := &table.Config{
			Key:   hostDetailCursorKey,
			Value: lastEvent.BkCursor,
		}
		err := w.set.Config().UpsertConfig(kt, []*table.Config{config})
		if err != nil {
			logs.Errorf("update host detail cursor to config failed, err: %v", err)
		}
	}
}

// processHostEvents process host event list
func (w *WatchHostUpdates) processHostEvents(kt *kit.Kit, events []bkcmdb.HostEvent) {
	// 记录已经查询过的主机，避免重复查询数据库
	invaluedHost := make(map[int]struct{}, 0)
	for _, event := range events {
		if err := w.processHostEvent(kt, event, invaluedHost); err != nil {
			logs.Warnf("process host event failed, event: %s, err: %v", event.BkCursor, err)
			// Skip failed events, rely on full data sync and other fallback measures
			continue
		}
	}
}

// processHostEvent process single host event
func (w *WatchHostUpdates) processHostEvent(
	kt *kit.Kit,
	event bkcmdb.HostEvent,
	invaluedHost map[int]struct{},
) error {
	switch event.BkEventType {
	case hostUpdateEvent:
		return w.handleHostUpdateEvent(kt, event, invaluedHost)
	default:
		// unknown host event type, skip
		logs.Warnf("unknown host event type: %s", event.BkEventType)
		return nil
	}
}

// handleHostUpdateEvent handle host update event
func (w *WatchHostUpdates) handleHostUpdateEvent(
	kt *kit.Kit,
	event bkcmdb.HostEvent,
	invaluedHost map[int]struct{},
) error {
	if event.BkDetail == nil {
		return errors.New("host update event has nil detail")
	}

	detail := event.BkDetail
	if detail.BkHostID == nil {
		return errors.New("invalid host update event detail")
	}

	hostID := *detail.BkHostID
	agentID := ""
	if detail.BkAgentID != nil {
		agentID = *detail.BkAgentID
	}
	if _, ok := invaluedHost[hostID]; ok {
		return nil
	}

	// Check if this host exists in biz_host table
	existingBizHosts, err := w.set.BizHost().ListAllByHostID(kt, uint(hostID))
	if err != nil {
		return fmt.Errorf("query biz hosts for hostID %d failed: %w", hostID, err)
	}

	if len(existingBizHosts) == 0 {
		invaluedHost[hostID] = struct{}{}
		return nil
	}
	if len(existingBizHosts) > 1 {
		// host should only belong to one biz, if multiple biz relations exist, it is considered abnormal
		logs.Warnf("found multiple business relationships for host %d", hostID)
	}

	// Update agentID for all business relationships of this host
	for _, bizHost := range existingBizHosts {
		// Update the agentID
		bizHost.AgentID = agentID
		if err := w.set.BizHost().UpdateByBizHost(kt, bizHost); err != nil {
			// Update failed means the relationship may have been removed, skip
			logs.Warnf("update biz[%d] host[%d] agentID failed: %v", bizHost.BizID, bizHost.HostID, err)
			continue
		}
	}

	return nil
}

// InitHostDetailCursor initializes host detail cursor to the latest position
// This function gets the latest cursor from CMDB and updates it to config table
func InitHostDetailCursor(set dao.Set, cmdbService bkcmdb.Service, timeAgo int64) error {
	kt := kit.New()
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
	if !watchResult.Result {
		return fmt.Errorf("watch host resource failed: %s", watchResult.Message)
	}

	if len(watchResult.Data.BkEvents) == 0 {
		// 监听成功情况下，若无事件则会返回一个不含详情但是含有cursor的事件
		return fmt.Errorf("watch host resource failed: no events found")
	}

	cursor := watchResult.Data.BkEvents[len(watchResult.Data.BkEvents)-1].BkCursor
	config := &table.Config{
		Key:   hostDetailCursorKey,
		Value: cursor,
	}

	err = set.Config().UpsertConfig(kt, []*table.Config{config})
	if err != nil {
		return fmt.Errorf("update host detail cursor to config failed: %w", err)
	}

	logs.Infof("successfully initialized host detail cursor to: %s", cursor)
	return nil
}
