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
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// NewCmdbResourceWatcher 监听cmdb资源变化
func NewCmdbResourceWatcher(set dao.Set, sd serviced.Service,
	cmdb bkcmdb.Service) *cmdbResourceWatcher {
	return &cmdbResourceWatcher{
		set:   set,
		state: sd,
		cmdb:  cmdb,
	}
}

type cmdbResourceWatcher struct {
	set   dao.Set
	state serviced.Service
	cmdb  bkcmdb.Service
}

func (c *cmdbResourceWatcher) Run() {
	logs.Infof("Start listening cmdb resource changes")
	notifier := shutdown.AddNotifier()

	go func() {
		ticker := time.NewTicker(defaultSyncCmdbTime)
		defer ticker.Stop()
		for {

			kt := kit.New()
			ctx, cancel := context.WithCancel(kt.Ctx)
			kt.Ctx = ctx

			select {
			case <-notifier.Signal:
				logs.Infof("stop synchronizing cmdb data success")
				cancel()
				notifier.Done()
				return
			case <-ticker.C:
				if !c.state.IsMaster() {
					logs.Infof("current service instance is slave, skip sync cmdb")
					continue
				}
				// 顺序监听每种资源类型
				for _, res := range []bkcmdb.ResourceType{bkcmdb.ResourceBiz, bkcmdb.ResourceModule,
					bkcmdb.ResourceHost, bkcmdb.ResourceProcess} {
					if err := c.watchCMDBResources(ctx, res); err != nil {
						logs.Errorf("watch %s resource failed: %v", res, err)
					}
				}

			}

		}
	}()
}

// watchCMDBResources 监听并处理指定资源类型
func (c *cmdbResourceWatcher) watchCMDBResources(ctx context.Context, resource bkcmdb.ResourceType) error {
	resp, err := c.cmdb.ResourceWatch(ctx, &bkcmdb.WatchResourceRequest{
		BkResource:        resource.String(),
		BkEventTypes:      []string{},
		BkFields:          []string{},
		BkStartFrom:       new(int64),
		BkSupplierAccount: "0",
	})
	if err != nil {
		return fmt.Errorf("request CMDB watch for %s failed: %w", resource, err)
	}

	var result bkcmdb.WatchData
	if err := resp.Decode(&result); err != nil {
		return fmt.Errorf("decode response for %s failed: %w", resource, err)
	}

	if !resp.Result {
		return fmt.Errorf("watch %s resource failed: %s", resource, resp.Message)
	}

	if !result.BkWatched {
		// 没有事件则跳过
		return nil
	}

	for _, event := range result.BkEvents {
		logs.Infof("[CMDB Watch] resource=%s event=%s cursor=%s",
			event.BkResource, event.BkEventType, event.BkCursor)

		c.handleEvent(event)
	}
	return nil
}

// handleEvent 根据资源和事件类型分派处理
func (c *cmdbResourceWatcher) handleEvent(resource bkcmdb.BkEventObj) {
	switch resource.BkResource {
	case bkcmdb.ResourceBiz:
		// c.handleSetEvent(event)
	case "module":
		// c.handleModuleEvent(event)
	case bkcmdb.ResourceProcess:
		c.handleProcessEvent(resource)
	case "host":
		// c.handleHostEvent(event)
	default:
		logs.Warnf("unknown CMDB resource type: %s", resource)
	}
}

// handleProcessEvent 处理进程(Process)事件
func (c *cmdbResourceWatcher) handleProcessEvent(resource bkcmdb.BkEventObj) {

	var result bkcmdb.ProcessInfo
	if err := resource.Decode(&result); err != nil {
		return
	}

	// 根据 event.BkEventType 进行分类更新
	switch resource.BkEventType {
	case bkcmdb.EventCreate:
		// c.syncProcessCreate(event)
	case bkcmdb.EventUpdate:
		// c.syncProcessUpdate(event)
	case bkcmdb.EventDelete:
		// c.syncProcessDelete(event)
	default:
		logs.Warnf("unknown event type for process: %s", resource.BkEventType.String())
	}
}

// handleSetEvent 处理集群(Set)事件
// func (c *cmdbResourceWatcher) handleSetEvent(event bkcmdb.EventType) {
// 	switch event.BkEventType {
// 	// case "create":
// 	// 	c.syncSetCreate(event)
// 	// case "update":
// 	// 	c.syncSetUpdate(event)
// 	// case "delete":
// 	// 	c.syncSetDelete(event)
// 	default:
// 		logs.Warnf("unknown event type for set: %s", event.BkEventType)
// 	}
// }

// // handleModuleEvent 处理模块(Module)事件
// func (c *cmdbResourceWatcher) handleModuleEvent(event bkcmdb.WatchEvent) {
// 	switch event.BkEventType {
// 	case "create":
// 		c.syncModuleCreate(event)
// 	case "update":
// 		c.syncModuleUpdate(event)
// 	case "delete":
// 		c.syncModuleDelete(event)
// 	default:
// 		logs.Warnf("unknown event type for module: %s", event.BkEventType)
// 	}
// }

// // handleHostEvent 处理主机(Host)事件
//
//	func (c *cmdbResourceWatcher) handleHostEvent(event bkcmdb.WatchEvent) {
//		switch event.BkEventType {
//		case "create":
//			c.syncHostCreate(event)
//		case "update":
//			c.syncHostUpdate(event)
//		case "delete":
//			c.syncHostDelete(event)
//		default:
//			logs.Warnf("unknown event type for host: %s", event.BkEventType)
//		}
//	}
