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

	"github.com/TencentBlueKing/bk-bscp/cmd/data-service/service"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	defaultSycnCmdbTime = 60 * time.Second
)

// NewSyncTicketStatus init sync ticket status
func NewSycnCMDB(set dao.Set, sd serviced.Service, srv *service.Service) SyncCMDB {
	return SyncCMDB{
		set:   set,
		state: sd,
		srv:   srv,
	}
}

// SyncCMDB xxx
type SyncCMDB struct {
	set   dao.Set
	state serviced.Service
	srv   *service.Service
}

func (s *SyncCMDB) Run() {
	logs.Infof("Start synchronizing cmdb data")
	notifier := shutdown.AddNotifier()
	go func() {
		ticker := time.NewTicker(defaultSycnCmdbTime)
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
				if !s.state.IsMaster() {
					logs.Infof("current service instance is slave, skip sync cmdb")
					continue
				}

				err := s.srv.SynchronizeCmdbData(kt.Ctx, []int{})
				if err != nil {
					logs.Errorf("synchronizing cmdb data failed: %v", err)
				}
				logs.Infof("synchronizing cmdb success")
			}
		}
	}()
}
