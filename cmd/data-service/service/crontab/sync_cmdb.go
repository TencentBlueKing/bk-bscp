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

// Package crontab example Synchronize the online status of the client
package crontab

import (
	"context"
	"time"

	"golang.org/x/time/rate"

	"github.com/TencentBlueKing/bk-bscp/cmd/data-service/service"
	"github.com/TencentBlueKing/bk-bscp/internal/components/bkuser"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	defaultSyncCmdbTime = 20 * time.Minute
)

// NewSyncCMDB init sync ticket status
func NewSyncCMDB(set dao.Set, sd serviced.Service, svc *service.Service,
	qpsLimit float64, cleanupInterval time.Duration) *syncCMDB {
	if qpsLimit <= 0 || qpsLimit > findHostBizRelationsApiQpsLimit {
		qpsLimit = findHostBizRelationsApiQpsLimit
	}
	rateLimiter := rate.NewLimiter(rate.Limit(qpsLimit), 1)
	return &syncCMDB{
		set:             set,
		state:           sd,
		svc:             svc,
		rateLimiter:     rateLimiter,
		cleanupInterval: cleanupInterval,
	}
}

// syncCMDB xxx
type syncCMDB struct {
	set             dao.Set
	state           serviced.Service
	svc             *service.Service
	rateLimiter     *rate.Limiter
	cleanupInterval time.Duration
}

func (s *syncCMDB) Run() {
	logs.Infof("[syncCMDBAndGSE] Start synchronizing CMDB & GSE data task")
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
				logs.Infof("[syncCMDBAndGSE] Stop synchronizing CMDB & GSE data success")
				cancel()
				notifier.Done()
				return
			case <-ticker.C:
				if !s.state.IsMaster() {
					logs.Warnf("[syncCMDBAndGSE] Current instance is slave, skip sync at=%s", time.Now().Format(time.RFC3339))
					continue
				}

				s.syncCMDBByTenant(kt)
			}
		}
	}()
}

// syncCMDBByTenant 按租户同步 CMDB 数据
func (s *syncCMDB) syncCMDBByTenant(kt *kit.Kit) {
	start := time.Now()
	rid := kt.Rid
	syncAt := start.Format(time.RFC3339)

	// 多租户模式：获取所有启用的租户并逐个同步
	if cc.DataService().FeatureFlags.EnableMultiTenantMode {
		tenants, err := bkuser.ListEnabledTenants(kt.Ctx)
		if err != nil {
			logs.Errorf("[syncCMDBAndGSE][error] rid=%s at=%s failed to list tenants: %v", rid, syncAt, err)
			return
		}

		if len(tenants) == 0 {
			logs.Warnf("[syncCMDBAndGSE][warn] rid=%s at=%s no enabled tenants found", rid, syncAt)
			return
		}

		for _, tenant := range tenants {
			kt.TenantID = tenant.ID
			if err := s.svc.SynchronizeCmdbData(kt.Ctx, tenant.ID, []int{}); err != nil {
				logs.Errorf(
					"[syncCMDBAndGSE][error] rid=%s tenant=%s at=%s failed to synchronize cmdb/gse data: %v",
					rid, tenant.ID, syncAt, err,
				)
				// 继续同步其他租户
				continue
			}
			logs.Infof("[syncCMDBAndGSE][success] rid=%s tenant=%s at=%s - synchronize cmdb/gse success", rid, tenant.ID, syncAt)
		}

		cost := time.Since(start)
		logs.Infof("[syncCMDBAndGSE][success] rid=%s at=%s cost=%s - all tenants sync completed", rid, syncAt, cost)
		return
	}

	// 单租户模式：使用空租户ID
	if err := s.svc.SynchronizeCmdbData(kt.Ctx, "", []int{}); err != nil {
		logs.Errorf(
			"[syncCMDBAndGSE][error] rid=%s at=%s failed to synchronize cmdb/gse data: %v (type=%T)",
			rid, syncAt, err, err,
		)
	} else {
		cost := time.Since(start)
		logs.Infof(
			"[syncCMDBAndGSE][success] rid=%s at=%s cost=%s - synchronize cmdb/gse success",
			rid, syncAt, cost,
		)
	}
}
