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
	// Check biz host relation events every 1 minute
	defaultWatchBizHostInterval = 1 * time.Minute
	// Check host update events every 30 seconds
	defaultWatchHostInterval = 30 * time.Second
	// Default QPS limit for CMDB API requests in watch mode
	defaultWatchQpsLimit = 80.0
)

// NewWatchBizHost init watch biz host
func NewWatchBizHost(
	set dao.Set,
	sd serviced.Service,
	cmdbService bkcmdb.Service,
	redisClient bedis.Client,
	qpsLimit float64,
) WatchBizHost {
	// when the cursor is lost, listen from 30 minutes ago
	timeAgo := time.Now().Add(-30 * time.Minute).Unix()
	if qpsLimit <= 0 || qpsLimit > defaultWatchQpsLimit {
		qpsLimit = defaultWatchQpsLimit
	}
	// create rate limiter
	rateLimiter := rate.NewLimiter(rate.Limit(qpsLimit), 1)

	return WatchBizHost{
		set:         set,
		state:       sd,
		cmdbService: cmdbService,
		timeAgo:     timeAgo,
		redisClient: redisClient,
		rateLimiter: rateLimiter,
	}
}

// WatchBizHost watch business host relationship changes
type WatchBizHost struct {
	set         dao.Set
	state       serviced.Service
	cmdbService bkcmdb.Service
	timeAgo     int64
	redisClient bedis.Client
	mutex       sync.Mutex
	rateLimiter *rate.Limiter // Rate limiter for CMDB API calls
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
	deleteEvent = "delete"
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
		BkResource: hostRelation, // Listen to host relationships
		// listen to create and delete events
		BkEventTypes: []string{createEvent, deleteEvent},
		BkFields:     []string{"bk_biz_id", "bk_host_id"},
	}
	// get cursor from Redis cache, if not exist, use timestamp to get events
	cachedCursor, err := w.redisClient.Get(kt.Ctx, bizHostCursorKey)
	if err != nil {
		logs.Errorf("get cached cursor from redis failed, key: %s, err: %v", bizHostCursorKey, err)
		return
	}
	if cachedCursor != "" {
		req.BkCursor = cachedCursor
	} else {
		req.BkStartFrom = &w.timeAgo
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
		return
	}

	if len(watchResult.Data.BkEvents) > 0 {
		w.processEvents(kt, watchResult.Data.BkEvents)
		// update cursor to redis
		lastEvent := watchResult.Data.BkEvents[len(watchResult.Data.BkEvents)-1]
		err := w.redisClient.Set(kt.Ctx, bizHostCursorKey, lastEvent.BkCursor, 7*24*3600)
		if err != nil {
			logs.Errorf("update biz host cursor to redis failed, err: %v", err)
		}
	}
}

// processEvents process event list
func (w *WatchBizHost) processEvents(kt *kit.Kit, events []bkcmdb.HostRelationEvent) {
	for _, event := range events {
		if err := w.processEvent(kt, event); err != nil {
			logs.Errorf("process event failed, event: %+v, err: %v", event, err)
			// Skip failed events, rely on full data sync and other fallback measures
			continue
		}
	}
}

// processEvent process single event
func (w *WatchBizHost) processEvent(kt *kit.Kit, event bkcmdb.HostRelationEvent) error {
	switch event.BkEventType {
	case createEvent:
		return w.handleHostCreateRelationEvent(kt, event)
	case deleteEvent:
		return w.handleHostRelationDeleteEvent(kt, event)
	default:
		logs.Warnf("unknown event type: %s", event.BkEventType)
		return nil
	}
}

// handleHostRelationEvent handle host relation event
func (w *WatchBizHost) handleHostCreateRelationEvent(kt *kit.Kit, event bkcmdb.HostRelationEvent) error {
	if event.BkDetail == nil {
		logs.Warnf("host relation event has nil detail, skipping")
		return nil
	}

	detail := event.BkDetail
	if detail.BkBizID == nil || detail.BkHostID == nil {
		logs.Warnf("invalid host relation event detail: %+v", detail)
		return nil
	}
	// create host relation if biz belongs to BSCP
	belongsToBSCP, err := w.isBizBelongsToBSCP(kt, *detail.BkBizID)
	if err != nil {
		logs.Errorf("check if biz %d belongs to BSCP failed, err: %v", *detail.BkBizID, err)
		return fmt.Errorf("check biz belongs to BSCP failed: %w", err)
	}

	if !belongsToBSCP {
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

// handleHostRelationDeleteEvent handle host relation delete event
func (w *WatchBizHost) handleHostRelationDeleteEvent(kt *kit.Kit, event bkcmdb.HostRelationEvent) error {
	if event.BkDetail == nil {
		logs.Warnf("host relation event has nil detail, skipping")
		return nil
	}

	detail := event.BkDetail
	if detail.BkBizID == nil || detail.BkHostID == nil {
		logs.Warnf("invalid host relation event detail: %+v", detail)
		return nil
	}

	// check if biz belongs to BSCP
	belongsToBSCP, err := w.isBizBelongsToBSCP(kt, *detail.BkBizID)
	if err != nil {
		logs.Errorf("check if biz %d belongs to BSCP failed, err: %v", *detail.BkBizID, err)
		return fmt.Errorf("check biz belongs to BSCP failed: %w", err)
	}

	if !belongsToBSCP {
		// biz does not belong to BSCP, skip deletion
		return nil
	}

	// check if host biz relation exists through CMDB API (need rate limiting)
	relationExists, err := w.verifyHostBizRelation(kt, *detail.BkBizID, *detail.BkHostID)
	if err != nil {
		logs.Errorf("verify host biz relation failed, biz: %d, host: %d, err: %v", *detail.BkBizID, *detail.BkHostID, err)
		return fmt.Errorf("verify host biz relation failed: %w", err)
	}

	if relationExists {
		// host biz relation exists, skip deletion
		return nil
	}

	if err := w.set.BizHost().Delete(kt, *detail.BkBizID, *detail.BkHostID); err != nil {
		return fmt.Errorf("delete biz[%d] host[%d] failed: %w", detail.BkBizID, detail.BkHostID, err)
	}

	return nil
}

// isBizBelongsToBSCP check if biz belongs to BSCP
func (w *WatchBizHost) isBizBelongsToBSCP(kt *kit.Kit, bizID int) (bool, error) {
	m := w.set.GenQuery().App
	app, err := w.set.GenQuery().App.WithContext(kt.Ctx).
		Where(m.BizID.Eq(uint32(bizID))).
		First()
	if err == dao.ErrRecordNotFound {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query biz %d belongs to BSCP failed: %w", bizID, err)
	}
	return app != nil, nil
}

// verifyHostBizRelation verify host biz relation exists
func (w *WatchBizHost) verifyHostBizRelation(kt *kit.Kit, bizID int, hostID int) (bool, error) {
	// apply rate limiter
	if err := w.rateLimiter.Wait(kt.Ctx); err != nil {
		return false, fmt.Errorf("rate limiter wait failed: %w", err)
	}

	req := &bkcmdb.FindHostBizRelationsRequest{
		BkBizID:  bizID,
		BkHostID: []int{hostID},
	}

	relationResult, err := w.cmdbService.FindHostBizRelations(kt.Ctx, req)
	if err != nil {
		return false, fmt.Errorf("find host biz relations failed: %w", err)
	}

	if !relationResult.Result {
		return false, fmt.Errorf("find host biz relations failed: %s", relationResult.Message)
	}

	// check if relation exists
	return len(relationResult.Data) > 0, nil
}

// watchHostUpdates watch host update events
func (w *WatchBizHost) watchHostUpdates(kt *kit.Kit) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	// Listen to host update events
	req := &bkcmdb.WatchResourceRequest{
		BkResource:   host,
		BkEventTypes: []string{updateEvent},
		BkFields:     []string{"bk_host_id", "bk_agent_id"},
	}
	cachedCursor, err := w.redisClient.Get(kt.Ctx, hostDetailCursorKey)
	if err != nil {
		logs.Errorf("get cached cursor from redis failed, key: %s, err: %v", hostDetailCursorKey, err)
		return
	}
	if cachedCursor != "" {
		req.BkCursor = cachedCursor
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

	// Process host update events
	if len(watchResult.Data.BkEvents) > 0 {
		w.processHostEvents(kt, watchResult.Data.BkEvents)
		// update cursor to Redis cache
		lastEvent := watchResult.Data.BkEvents[len(watchResult.Data.BkEvents)-1]
		err := w.redisClient.Set(kt.Ctx, hostDetailCursorKey, lastEvent.BkCursor, 7*24*3600)
		if err != nil {
			logs.Errorf("update host detail cursor to redis failed, err: %v", err)
		}
	}
}

// processHostEvents process host event list
func (w *WatchBizHost) processHostEvents(kt *kit.Kit, events []bkcmdb.HostEvent) {
	for _, event := range events {
		if err := w.processHostEvent(kt, event); err != nil {
			logs.Warnf("process host event failed, event: %s, err: %v", event.BkCursor, err)
			// Skip failed events, rely on full data sync and other fallback measures
			continue
		}
	}
}

// processHostEvent process single host event
func (w *WatchBizHost) processHostEvent(kt *kit.Kit, event bkcmdb.HostEvent) error {
	switch event.BkEventType {
	case updateEvent:
		return w.handleHostUpdateEvent(kt, event)
	default:
		// unknown host event type, skip
		logs.Warnf("unknown host event type: %s", event.BkEventType)
		return nil
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
