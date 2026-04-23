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
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
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
	// listBizHostsChunkSize limits the number of host IDs sent per ListBizHosts call
	// to avoid oversized CMDB requests.
	listBizHostsChunkSize = 500
	// findHostBizRelationsChunkSize limits the number of host IDs sent per
	// FindHostBizRelations call.
	findHostBizRelationsChunkSize = 500
)

// getBizHostCursorKey 获取带租户前缀的游标 key
func getBizHostCursorKey(tenantID string) string {
	if tenantID == "" {
		return bizHostCursorKey
	}
	return fmt.Sprintf("%s-%s", tenantID, bizHostCursorKey)
}

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
				w.watchBizHostByTenant(kt)
			}
		}
	}()
}

// watchBizHostByTenant 按租户监听业务主机关系变化
func (w *WatchBizHostRelation) watchBizHostByTenant(kt *kit.Kit) {
	// 多租户模式：从 app 表获取租户列表并逐个监听
	if cc.DataService().FeatureFlags.EnableMultiTenantMode {
		apps, err := w.set.App().GetDistinctTenantIDs(kt)
		if err != nil {
			logs.Errorf("get distinct tenant IDs failed, err: %v", err)
			return
		}

		if len(apps) == 0 {
			logs.Warnf("no tenants found in app table for watch biz host relation")
			return
		}

		for _, app := range apps {
			if app.Spec.TenantID == "" {
				continue
			}
			tenantKit := *kt
			tenantKit.TenantID = app.Spec.TenantID
			tenantKit.Ctx = tenantKit.InternalRpcCtx()
			w.watchBizHost(&tenantKit)
		}
		return
	}

	// 单租户模式
	w.watchBizHost(kt)
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
	// get cursor key with tenant prefix
	cursorKey := getBizHostCursorKey(kt.TenantID)
	// get cursor from config table, if not exist, use timestamp to get events
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

	watchResult, err := w.cmdbService.WatchHostRelationResource(kt.Ctx, req)
	if err != nil {
		logs.Errorf("watch host relation resource failed, err: %v", err)
		return
	}

	if !watchResult.BkWatched {
		// No events found, skip
		return
	}
	logs.Infof("watch host relation resource success, events: %d", len(watchResult.BkEvents))

	if len(watchResult.BkEvents) > 0 {
		w.processEvents(kt, watchResult.BkEvents)
		// update cursor to config table
		lastEvent := watchResult.BkEvents[len(watchResult.BkEvents)-1]
		config := &table.Config{
			Key:   cursorKey,
			Value: lastEvent.BkCursor,
		}
		err := w.set.Config().UpsertConfig(kt, []*table.Config{config})
		if err != nil {
			logs.Errorf("update biz host cursor to config failed, err: %v", err)
		}
	}
}

// compressedIntent represents the final state of a (bizID, hostID) pair
// within a single watch batch after event compression.
type compressedIntent struct {
	bizID  int
	hostID int
	// finalOp is one of bizHostRelationCreateEvent / bizHostRelationDeleteEvent.
	finalOp string
}

// compressEvents collapses events by (bizID, hostID), keeping the last event
// type seen for each pair. Order of first appearance is preserved for
// deterministic downstream processing.
func compressEvents(events []bkcmdb.HostRelationEvent) []compressedIntent {
	type key struct{ biz, host int }
	ordered := make([]key, 0, len(events))
	state := make(map[key]string, len(events))

	for _, ev := range events {
		if ev.BkDetail == nil || ev.BkDetail.BkBizID == nil || ev.BkDetail.BkHostID == nil {
			logs.Warnf("invalid host relation event detail: %+v", ev.BkDetail)
			continue
		}
		if ev.BkEventType != bizHostRelationCreateEvent && ev.BkEventType != bizHostRelationDeleteEvent {
			logs.Warnf("unknown event type: %s", ev.BkEventType)
			continue
		}
		k := key{biz: *ev.BkDetail.BkBizID, host: *ev.BkDetail.BkHostID}
		if _, ok := state[k]; !ok {
			ordered = append(ordered, k)
		}
		state[k] = ev.BkEventType
	}

	intents := make([]compressedIntent, 0, len(ordered))
	for _, k := range ordered {
		intents = append(intents, compressedIntent{bizID: k.biz, hostID: k.host, finalOp: state[k]})
	}
	return intents
}

// processEvents compresses the event batch, filters non-BSCP biz once per biz,
// and then executes batched CMDB/DB operations per biz.
func (w *WatchBizHostRelation) processEvents(kt *kit.Kit, events []bkcmdb.HostRelationEvent) {
	intents := compressEvents(events)
	if len(intents) == 0 {
		return
	}
	if len(intents) != len(events) {
		logs.Infof("host relation events compressed: %d -> %d", len(events), len(intents))
	}

	// Resolve BSCP membership once per unique biz.
	uniqBizs := make(map[int]struct{})
	for _, intent := range intents {
		uniqBizs[intent.bizID] = struct{}{}
	}
	bscpBizs := make(map[int]bool, len(uniqBizs))
	for bizID := range uniqBizs {
		belongs, err := w.set.App().CheckBizExists(kt, uint32(bizID))
		if err != nil {
			logs.Errorf("check if biz %d belongs to BSCP failed, err: %v", bizID, err)
			continue
		}
		bscpBizs[bizID] = belongs
	}

	// Group final intents by biz.
	createByBiz := make(map[int][]int)
	deleteByBiz := make(map[int][]int)
	for _, intent := range intents {
		if !bscpBizs[intent.bizID] {
			continue
		}
		switch intent.finalOp {
		case bizHostRelationCreateEvent:
			createByBiz[intent.bizID] = append(createByBiz[intent.bizID], intent.hostID)
		case bizHostRelationDeleteEvent:
			deleteByBiz[intent.bizID] = append(deleteByBiz[intent.bizID], intent.hostID)
		}
	}

	for bizID, hostIDs := range createByBiz {
		if err := w.processBizCreates(kt, bizID, hostIDs); err != nil {
			logs.Errorf("process biz[%d] creates failed, hosts=%d, err: %v", bizID, len(hostIDs), err)
			continue
		}
	}
	for bizID, hostIDs := range deleteByBiz {
		if err := w.processBizDeletes(kt, bizID, hostIDs); err != nil {
			logs.Errorf("process biz[%d] deletes failed, hosts=%d, err: %v", bizID, len(hostIDs), err)
			continue
		}
	}
}

// processBizCreates resolves host details via a single batched ListBizHosts
// call per chunk and upserts the results in one BatchUpsert.
func (w *WatchBizHostRelation) processBizCreates(kt *kit.Kit, bizID int, hostIDs []int) error {
	for start := 0; start < len(hostIDs); start += listBizHostsChunkSize {
		end := start + listBizHostsChunkSize
		if end > len(hostIDs) {
			end = len(hostIDs)
		}
		chunk := hostIDs[start:end]

		hostResult, err := w.cmdbService.ListBizHosts(kt.Ctx, &bkcmdb.ListBizHostsRequest{
			BkBizID: bizID,
			Page: bkcmdb.PageParam{
				Start: 0,
				Limit: len(chunk),
			},
			Fields: []string{"bk_host_id", "bk_agent_id", "bk_host_innerip"},
			HostPropertyFilter: &bkcmdb.HostPropertyFilter{
				Condition: bkcmdb.HostPropertyConditionAnd,
				Rules: []bkcmdb.HostPropertyRule{
					{Field: "bk_host_id", Operator: bkcmdb.HostPropertyOperatorIn, Value: chunk},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("list biz[%d] hosts failed: %w", bizID, err)
		}
		if len(hostResult.Info) == 0 {
			continue
		}

		bizHosts := make([]*table.BizHost, 0, len(hostResult.Info))
		for _, h := range hostResult.Info {
			bizHosts = append(bizHosts, &table.BizHost{
				TenantID:      kt.TenantID,
				BizID:         uint(bizID),
				HostID:        uint(h.BkHostID),
				AgentID:       h.BkAgentID,
				BKHostInnerIP: h.BkHostInnerIP,
			})
		}
		if err := w.set.BizHost().BatchUpsert(kt, bizHosts); err != nil {
			return fmt.Errorf("batch upsert biz[%d] hosts failed: %w", bizID, err)
		}
	}
	return nil
}

// processBizDeletes verifies which hostIDs are still associated with the biz
// via a single batched FindHostBizRelations call per chunk and then removes
// the truly-deleted ones in one BatchDelete.
func (w *WatchBizHostRelation) processBizDeletes(kt *kit.Kit, bizID int, hostIDs []int) error {
	toDelete := make([]uint, 0, len(hostIDs))

	for start := 0; start < len(hostIDs); start += findHostBizRelationsChunkSize {
		end := start + findHostBizRelationsChunkSize
		if end > len(hostIDs) {
			end = len(hostIDs)
		}
		chunk := hostIDs[start:end]

		if err := w.rateLimiter.Wait(kt.Ctx); err != nil {
			return fmt.Errorf("rate limiter wait failed: %w", err)
		}
		relations, err := w.cmdbService.FindHostBizRelations(kt.Ctx, &bkcmdb.FindHostBizRelationsRequest{
			BkBizID:  bizID,
			BkHostID: chunk,
		})
		if err != nil {
			return fmt.Errorf("find host biz relations for biz[%d] failed: %w", bizID, err)
		}

		existSet := make(map[int]struct{}, len(relations))
		for _, r := range relations {
			existSet[r.BkHostID] = struct{}{}
		}
		for _, hostID := range chunk {
			if _, ok := existSet[hostID]; ok {
				continue
			}
			toDelete = append(toDelete, uint(hostID))
		}
	}

	if len(toDelete) == 0 {
		return nil
	}

	if err := w.set.BizHost().BatchDelete(kt, uint(bizID), toDelete); err != nil {
		return fmt.Errorf("batch delete biz[%d] hosts failed: %w", bizID, err)
	}
	return nil
}

// InitBizHostCursor initializes biz host cursor to the latest position
// This function gets the latest cursor from CMDB and updates it to config table
func InitBizHostCursor(tenantID string, set dao.Set, cmdbService bkcmdb.Service, timeAgo int64) error {
	logs.Infof("start init biz host cursor for tenant: %s", tenantID)
	kt := kit.NewWithTenant(tenantID)
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

	if len(watchResult.BkEvents) == 0 {
		// 监听成功情况下，若无事件则会返回一个不含详情但是含有cursor的事件
		return fmt.Errorf("watch host relation resource failed: no events found")
	}

	cursorKey := getBizHostCursorKey(tenantID)
	cursor := watchResult.BkEvents[len(watchResult.BkEvents)-1].BkCursor
	config := &table.Config{
		Key:   cursorKey,
		Value: cursor,
	}

	err = set.Config().UpsertConfig(kt, []*table.Config{config})
	if err != nil {
		return fmt.Errorf("update biz host cursor to config failed: %w", err)
	}

	logs.Infof("successfully initialized biz host cursor to: %s (tenant: %s)", cursor, tenantID)
	return nil
}
