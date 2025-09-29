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

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	defaultWatchBizHostInterval = 1 * time.Minute  // Check events every 1 minute
	defaultWatchHostInterval    = 30 * time.Second // Check host update events every 30 seconds
)

// NewWatchBizHost init watch biz host
func NewWatchBizHost(set dao.Set, sd serviced.Service, cmdbService bkcmdb.Service) WatchBizHost {
	timeAgo := time.Now().Add(-30 * time.Minute).Unix()
	return WatchBizHost{
		set:                   set,
		state:                 sd,
		cmdbService:           cmdbService,
		startTime:             timeAgo,
		bizHostEventCursor:    "", // Initial cursor is empty
		hostDetailEventCursor: "", // Initial cursor is empty
	}
}

// WatchBizHost watch business host relationship changes
type WatchBizHost struct {
	set                   dao.Set
	state                 serviced.Service
	cmdbService           bkcmdb.Service
	mutex                 sync.Mutex // For host relation events
	hostMutex             sync.Mutex // For host events
	startTime             int64      // Start time for listening events
	bizHostEventCursor    string     // Event cursor for host relation events
	hostDetailEventCursor string     // Event cursor for host events
}

// Run starts two independent watch tasks for host relations and host updates
func (w *WatchBizHost) Run() {
	logs.Infof("start watch biz host task - launching two independent watch goroutines")
	notifier := shutdown.AddNotifier()

	// Task 1: Host relation watch task (business-host relationships)
	go func() {
		ticker := time.NewTicker(defaultWatchBizHostInterval)
		defer ticker.Stop()
		for {
			kt := kit.New()
			ctx, cancel := context.WithCancel(kt.Ctx)
			kt.Ctx = ctx

			select {
			case <-notifier.Signal:
				logs.Infof("stop host relation watch success")
				cancel()
				return
			case <-ticker.C:
				if !w.state.IsMaster() {
					logs.Infof("current service instance is slave, skip host relation watch")
					continue
				}
				logs.Infof("host relation watch triggered")
				w.watchBizHost(kt)
			}
		}
	}()

	// Task 2: Host update watch task (host agent_id updates)
	go func() {
		ticker := time.NewTicker(defaultWatchHostInterval)
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

// Event types
const (
	createEvent = "create"
	updateEvent = "update"
)

// Resource types
const (
	hostRelation = "host_relation"
	host         = "host"
)

// watchBizHost watch business host relationship changes
func (w *WatchBizHost) watchBizHost(kt *kit.Kit) {
	w.mutex.Lock()
	defer func() {
		w.mutex.Unlock()
	}()

	// Listen to host relationship change events
	req := &bkcmdb.WatchResourceRequest{
		BkResource:   hostRelation, // Listen to host relationships
		BkEventTypes: []string{createEvent, updateEvent},
		BkFields:     []string{"bk_biz_id", "bk_host_id"},
	}
	if w.bizHostEventCursor != "" {
		// For non-first listening, use the previous cursor
		req.BkCursor = w.bizHostEventCursor
	} else {
		// For first listening, get events from the last 30 minutes
		req.BkStartFrom = &w.startTime
	}

	watchResult, err := w.cmdbService.WatchHostRelationResource(kt.Ctx, req)
	if err != nil {
		logs.Errorf("watch host relation resource failed, err: %v", err)
		return
	}

	if !watchResult.Result {
		logs.Errorf("watch host relation resource failed: %s", watchResult.Message)
		return
	}
	if !watchResult.Data.BkWatched {
		// No events found, skip
		logs.Infof("no host relation events found")
		return
	}

	// Process events (no type conversion needed - type alias)
	if len(watchResult.Data.BkEvents) > 0 {
		if err := w.processEvents(kt, watchResult.Data.BkEvents); err != nil {
			logs.Errorf("process events failed, err: %v", err)
			return
		}
		// Update cursor to the last event's cursor
		lastEvent := watchResult.Data.BkEvents[len(watchResult.Data.BkEvents)-1]
		w.bizHostEventCursor = lastEvent.BkCursor
	}
}

// processEvents process event list
func (w *WatchBizHost) processEvents(kt *kit.Kit, events []bkcmdb.HostRelationEvent) error {
	for _, event := range events {
		if err := w.processEvent(kt, event); err != nil {
			logs.Errorf("process event failed, event: %+v, err: %v", event, err)
			// Skip failed events, rely on full data sync and other fallback measures
			continue
		}
	}
	return nil
}

// processEvent process single event
func (w *WatchBizHost) processEvent(kt *kit.Kit, event bkcmdb.HostRelationEvent) error {
	switch event.BkEventType {
	case createEvent, updateEvent:
		return w.handleHostRelationEvent(kt, event)
	default:
		logs.Warnf("unknown event type: %s", event.BkEventType)
		return nil
	}
}

// handleHostRelationEvent handle host relation event
func (w *WatchBizHost) handleHostRelationEvent(kt *kit.Kit, event bkcmdb.HostRelationEvent) error {
	if event.BkDetail == nil {
		logs.Warnf("host relation event has nil detail, skipping")
		return nil
	}

	detail := event.BkDetail
	if detail.BkBizID == nil || detail.BkHostID == nil {
		logs.Warnf("invalid host relation event detail: %+v", detail)
		return nil
	}

	bizHost := &table.BizHost{
		BizID:  *detail.BkBizID,
		HostID: *detail.BkHostID,
	}

	if err := w.set.BizHost().Upsert(kt, bizHost); err != nil {
		return fmt.Errorf("upsert biz[%d] host[%d] failed: %w", detail.BkBizID, detail.BkHostID, err)
	}
	return nil
}

// watchHostUpdates watch host update events
func (w *WatchBizHost) watchHostUpdates(kt *kit.Kit) {
	w.hostMutex.Lock()
	defer func() {
		w.hostMutex.Unlock()
	}()

	// Listen to host update events
	req := &bkcmdb.WatchResourceRequest{
		BkResource:   host,                  // Listen to host resource
		BkEventTypes: []string{updateEvent}, // Only care about update events
		BkFields:     []string{"bk_host_id", "bk_agent_id"},
	}

	if w.hostDetailEventCursor != "" {
		// For non-first listening, use the previous cursor
		req.BkCursor = w.hostDetailEventCursor
	} else {
		// For first listening, get events from the last 30 minutes
		req.BkStartFrom = &w.startTime
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
		logs.Infof("no host update events found")
		return
	}
	logs.Infof("watch host resource success, total events: %d", len(watchResult.Data.BkEvents))

	// Process host update events (no type conversion needed - type alias)
	if len(watchResult.Data.BkEvents) > 0 {
		w.processHostEvents(kt, watchResult.Data.BkEvents)
		// Update host cursor to the last event's cursor
		w.hostDetailEventCursor = watchResult.Data.BkEvents[len(watchResult.Data.BkEvents)-1].BkCursor
	}
}

// processHostEvents process host event list
func (w *WatchBizHost) processHostEvents(kt *kit.Kit, events []bkcmdb.HostEvent) {
	successCount := 0
	failureCount := 0
	for _, event := range events {
		if err := w.processHostEvent(kt, event); err != nil {
			// 记录失败游标
			logs.Errorf("process host event failed, event: %s, err: %v", event.BkCursor, err)
			// Skip failed events, rely on full data sync and other fallback measures
			failureCount++
			continue
		}
		successCount++
	}
	logs.Infof("successfully processed %d/%d host events", successCount, len(events))
	logs.Infof("failed to process %d/%d host events", failureCount, len(events))
}

// processHostEvent process single host event
func (w *WatchBizHost) processHostEvent(kt *kit.Kit, event bkcmdb.HostEvent) error {
	switch event.BkEventType {
	case updateEvent:
		return w.handleHostUpdateEvent(kt, event)
	default:
		logs.Warnf("unknown host event type: %s", event.BkEventType)
		return errors.New("unknown host event type")
	}
}

// handleHostUpdateEvent handle host update event
func (w *WatchBizHost) handleHostUpdateEvent(kt *kit.Kit, event bkcmdb.HostEvent) error {
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

	// Check if this host exists in biz_host table
	existingBizHosts, err := w.set.BizHost().ListAllByHostID(kt, hostID)
	if err != nil {
		return errors.New("query biz hosts for hostID failed")
	}

	if len(existingBizHosts) == 0 {
		// 1、主机详情事件为全量事件，存在较多主机不存在于biz_host表中
		// 2、部分业务和主机关系同步存在延迟
		return errors.New("no biz hosts found for hostID")
	}
	if len(existingBizHosts) > 1 {
		// 主机应该只属于一个业务，存在多个业务关系则认为存在异常
		logs.Warnf("found multiple business relationships for host %d", hostID)
		// PASS
	}

	// Update agentID for all business relationships of this host
	for _, bizHost := range existingBizHosts {
		// Update the agentID
		bizHost.AgentID = agentID
		if err := w.set.BizHost().UpdateByBizHost(kt, bizHost); err != nil {
			// Update failed means the relationship may have been removed, skip
			logs.Warnf("update biz[%d] host[%d] agentID failed: %v", bizHost.BizID, bizHost.HostID, err)
			return errors.New("update biz host agentID failed")
		}
	}

	return nil
}
