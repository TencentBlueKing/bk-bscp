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
	// find host biz relations api qps limit
	findHostBizRelationsApiQpsLimit = 60.0
	// watch biz host relation create event
	bizHostRelationCreateEvent = "create"
	// watch biz host relation delete event
	bizHostRelationDeleteEvent = "delete"
	// watch resource types
	hostRelation = "host_relation"
	// config key for biz host cursor
	bizHostCursorKey = "biz_host_cursor"
)

// NewWatchBizHostRelation init watch biz host relation
func NewWatchBizHostRelation(
	set dao.Set,
	sd serviced.Service,
	cmdbService bkcmdb.Service,
	qpsLimit float64,
	interval time.Duration,
) WatchBizHostRelation {
	// when the cursor is lost, listen from 3 minutes ago
	timeAgo := time.Now().Add(-3 * time.Minute).Unix()
	if qpsLimit <= 0 || qpsLimit > findHostBizRelationsApiQpsLimit {
		qpsLimit = findHostBizRelationsApiQpsLimit
	}
	// create rate limiter
	rateLimiter := rate.NewLimiter(rate.Limit(qpsLimit), 1)

	return WatchBizHostRelation{
		set:         set,
		state:       sd,
		cmdbService: cmdbService,
		timeAgo:     timeAgo,
		rateLimiter: rateLimiter,
		interval:    interval,
	}
}

// WatchBizHostRelation watch business host relationship changes
type WatchBizHostRelation struct {
	set         dao.Set
	state       serviced.Service
	cmdbService bkcmdb.Service
	timeAgo     int64
	interval    time.Duration
	mutex       sync.Mutex
	rateLimiter *rate.Limiter // Rate limiter for CMDB API calls
}

// Run starts the watch task for host relations
func (w *WatchBizHostRelation) Run() {
	logs.Infof("start watch biz host relation task")
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
}

// watchBizHost watch business host relationship changes
func (w *WatchBizHostRelation) watchBizHost(kt *kit.Kit) {
	w.mutex.Lock()
	defer func() {
		w.mutex.Unlock()
	}()
	// Listen to host relationship change events
	req := &bkcmdb.WatchResourceRequest{
		BkResource: hostRelation, // Listen to host relationships
		// listen to create and delete events
		BkEventTypes: []string{bizHostRelationCreateEvent, bizHostRelationDeleteEvent},
		BkFields:     []string{"bk_biz_id", "bk_host_id"},
	}
	// get cursor from config table, if not exist, use timestamp to get events
	config, err := w.set.Config().GetConfig(kt, bizHostCursorKey)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logs.Errorf("get cached cursor from config failed, key: %s, err: %v", bizHostCursorKey, err)
			return
		}
		// cursor not found, use timestamp
		req.BkStartFrom = &w.timeAgo
	} else if config != nil && config.Value != "" {
		req.BkCursor = config.Value
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
	logs.Infof("watch host relation resource success, events: %d", len(watchResult.Data.BkEvents))

	if len(watchResult.Data.BkEvents) > 0 {
		w.processEvents(kt, watchResult.Data.BkEvents)
		// update cursor to config table
		lastEvent := watchResult.Data.BkEvents[len(watchResult.Data.BkEvents)-1]
		config := &table.Config{
			Key:   bizHostCursorKey,
			Value: lastEvent.BkCursor,
		}
		err := w.set.Config().UpsertConfig(kt, []*table.Config{config})
		if err != nil {
			logs.Errorf("update biz host cursor to config failed, err: %v", err)
		}
	}
}

// processEvents process event list
func (w *WatchBizHostRelation) processEvents(kt *kit.Kit, events []bkcmdb.HostRelationEvent) {
	// 记录非BSCP业务，避免重复查询数据库判断是否属于BSCP
	invaluedBiz := make(map[int]struct{}, 0)
	for _, event := range events {
		if err := w.processEvent(kt, event, invaluedBiz); err != nil {
			logs.Errorf("process event failed, event: %+v, err: %v", event, err)
			// Skip failed events, rely on full data sync and other fallback measures
			continue
		}
	}
}

// processEvent process single event
func (w *WatchBizHostRelation) processEvent(
	kt *kit.Kit, event bkcmdb.HostRelationEvent,
	invaluedBiz map[int]struct{},
) error {
	switch event.BkEventType {
	case bizHostRelationCreateEvent:
		return w.handleHostRelationCreateEvent(kt, event, invaluedBiz)
	case bizHostRelationDeleteEvent:
		return w.handleHostRelationDeleteEvent(kt, event, invaluedBiz)
	default:
		logs.Warnf("unknown event type: %s", event.BkEventType)
		return nil
	}
}

// handleHostRelationEvent handle host relation event
func (w *WatchBizHostRelation) handleHostRelationCreateEvent(
	kt *kit.Kit,
	event bkcmdb.HostRelationEvent,
	invaluedBiz map[int]struct{},
) error {
	if event.BkDetail == nil {
		logs.Warnf("host relation event has nil detail, skipping")
		return nil
	}

	detail := event.BkDetail
	if detail.BkBizID == nil || detail.BkHostID == nil {
		logs.Warnf("invalid host relation event detail: %+v", detail)
		return nil
	}
	if _, ok := invaluedBiz[*detail.BkBizID]; ok {
		return nil
	}
	belongsToBSCP, err := w.set.App().CheckBizExists(kt, uint32(*detail.BkBizID))
	if err != nil {
		logs.Errorf("check if biz %d belongs to BSCP failed, err: %v", *detail.BkBizID, err)
		return fmt.Errorf("check biz belongs to BSCP failed: %w", err)
	}

	if !belongsToBSCP {
		invaluedBiz[*detail.BkBizID] = struct{}{}
		return nil
	}

	bizHost := &table.BizHost{
		BizID:  uint(*detail.BkBizID),
		HostID: uint(*detail.BkHostID),
	}
	// query host detail
	hostResult, err := w.cmdbService.ListBizHosts(kt.Ctx, &bkcmdb.ListBizHostsRequest{
		BkBizID: *detail.BkBizID,
		Page: bkcmdb.PageParam{
			Start: 0,
			Limit: 1,
		},
		Fields: []string{"bk_host_id", "bk_agent_id", "bk_host_innerip"},
		HostPropertyFilter: &bkcmdb.HostPropertyFilter{
			Condition: bkcmdb.HostPropertyConditionAnd,
			Rules: []bkcmdb.HostPropertyRule{
				{Field: "bk_host_id", Operator: bkcmdb.HostPropertyOperatorEqual, Value: *detail.BkHostID},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("list biz hosts failed: %w", err)
	}
	if !hostResult.Result {
		return fmt.Errorf("list biz hosts failed: %s", hostResult.Message)
	}

	if len(hostResult.Data.Info) == 0 {
		return nil
	}
	host := hostResult.Data.Info[0]
	bizHost.AgentID = host.BkAgentID
	bizHost.BKHostInnerIP = host.BkHostInnerIP

	if err := w.set.BizHost().Upsert(kt, bizHost); err != nil {
		return fmt.Errorf("upsert biz[%d] host[%d] failed: %w", detail.BkBizID, detail.BkHostID, err)
	}
	return nil
}

// handleHostRelationDeleteEvent handle host relation delete event
func (w *WatchBizHostRelation) handleHostRelationDeleteEvent(
	kt *kit.Kit,
	event bkcmdb.HostRelationEvent,
	invaluedBiz map[int]struct{},
) error {
	if event.BkDetail == nil {
		logs.Warnf("host relation event has nil detail, skipping")
		return nil
	}

	detail := event.BkDetail
	if detail.BkBizID == nil || detail.BkHostID == nil {
		logs.Warnf("invalid host relation event detail: %+v", detail)
		return nil
	}
	if _, ok := invaluedBiz[*detail.BkBizID]; ok {
		return nil
	}
	// check if biz belongs to BSCP (with cache optimization)
	belongsToBSCP, err := w.set.App().CheckBizExists(kt, uint32(*detail.BkBizID))
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

	if err := w.set.BizHost().Delete(kt, uint(*detail.BkBizID), uint(*detail.BkHostID)); err != nil {
		return fmt.Errorf("delete biz[%d] host[%d] failed: %w", detail.BkBizID, detail.BkHostID, err)
	}

	return nil
}

// verifyHostBizRelation verify host biz relation exists
func (w *WatchBizHostRelation) verifyHostBizRelation(kt *kit.Kit, bizID int, hostID int) (bool, error) {
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

// InitBizHostCursor initializes biz host cursor to the latest position
// This function gets the latest cursor from CMDB and updates it to config table
func InitBizHostCursor(set dao.Set, cmdbService bkcmdb.Service, timeAgo int64) error {
	logs.Infof("start init biz host cursor")
	kt := kit.New()
	ctx, cancel := context.WithTimeout(kt.Ctx, 10*time.Minute)
	defer cancel()
	kt.Ctx = ctx

	req := &bkcmdb.WatchResourceRequest{
		BkResource:   hostRelation,
		BkEventTypes: []string{bizHostRelationCreateEvent, bizHostRelationDeleteEvent},
		BkFields:     []string{"bk_biz_id", "bk_host_id"},
		BkStartFrom:  &timeAgo,
	}

	watchResult, err := cmdbService.WatchHostRelationResource(kt.Ctx, req)
	if err != nil {
		return fmt.Errorf("watch host relation resource failed: %w", err)
	}
	if !watchResult.Result {
		return fmt.Errorf("watch host relation resource failed: %s", watchResult.Message)
	}

	if len(watchResult.Data.BkEvents) == 0 {
		// 监听成功情况下，若无事件则会返回一个不含详情但是含有cursor的事件
		return fmt.Errorf("watch host relation resource failed: no events found")
	}

	cursor := watchResult.Data.BkEvents[len(watchResult.Data.BkEvents)-1].BkCursor
	config := &table.Config{
		Key:   bizHostCursorKey,
		Value: cursor,
	}

	err = set.Config().UpsertConfig(kt, []*table.Config{config})
	if err != nil {
		return fmt.Errorf("update biz host cursor to config failed: %w", err)
	}

	logs.Infof("successfully initialized biz host cursor to: %s", cursor)
	return nil
}
