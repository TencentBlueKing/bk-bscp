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
	"strings"
	"sync"
	"time"

	"github.com/TencentBlueKing/bk-bscp/cmd/data-service/service"
	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm"
	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

const (
	defaultSyncTicketStatusInterval = 30 * time.Second
)

// NewSyncTicketStatus init sync ticket status
func NewSyncTicketStatus(set dao.Set, sd serviced.Service, srv *service.Service) SyncTicketStatus {
	return SyncTicketStatus{
		set:   set,
		state: sd,
		srv:   srv,
		itsm:  itsm.NewITSMService(),
	}
}

// SyncTicketStatus xxx
type SyncTicketStatus struct {
	set   dao.Set
	state serviced.Service
	mutex sync.Mutex
	srv   *service.Service
	itsm  itsm.Service
}

// Run the sync ticket status
func (c *SyncTicketStatus) Run() {
	logs.Infof("start synchronization task for the itsm tickets")
	notifier := shutdown.AddNotifier()
	go func() {
		ticker := time.NewTicker(defaultSyncTicketStatusInterval)
		defer ticker.Stop()
		for {
			kt := kit.New()
			ctx, cancel := context.WithCancel(kt.Ctx)
			kt.Ctx = ctx

			select {
			case <-notifier.Signal:
				logs.Infof("stop sync tickets status success")
				cancel()
				notifier.Done()
				return
			case <-ticker.C:
				if !c.state.IsMaster() {
					logs.Infof("current service instance is slave, skip sync tickets status")
					continue
				}
				logs.Infof("starts to synchronize the tickets status")
				c.syncTicketStatus(kt)
			}
		}
	}()
}

// sync the ticket status
// nolint: funlen
func (c *SyncTicketStatus) syncTicketStatus(kt *kit.Kit) {
	c.mutex.Lock()
	defer func() {
		c.mutex.Unlock()
	}()

	// 获取CREATED、待上线，待审批状态的strategy记录
	strategys, err := c.set.Strategy().ListStrategyByItsm(kt)
	if err != nil {
		logs.Errorf("list strategy by itsm failed: %s", err.Error())
		return
	}
	for _, strategy := range strategys {
		err := c.processTicket(kt, strategy)
		if err != nil {
			logs.Errorf("process ticket failed: %s", err.Error())
			continue
		}
	}
}

func (c *SyncTicketStatus) processTicket(kit *kit.Kit, strategy *table.Strategy) error {
	logs.Infof("process ticket %s", strategy.Spec.ItsmTicketSn)
	// 获取单据状态，只处理已结束的单据
	kit.TenantID = strategy.Attachment.TenantID
	kit.User = strategy.Revision.Creator

	ticksetStatus, err := c.itsm.GetTicketStatus(kit.InternalRpcCtx(), api.GetTicketStatusReq{
		TicketID: strategy.Spec.ItsmTicketSn,
	})
	if err != nil {
		logs.Errorf("get itsm ticket status failed: %s", err.Error())
		return nil
	}
	if ticksetStatus.CurrentStatus == constant.TicketRunningStatus {
		logs.Warnf("ticket %s is running, ignore", strategy.Spec.ItsmTicketSn)
		return nil
	}
	// 查看单据审批状态，最终是拒绝还是同步

	// 查看状态是审批通过还是拒绝
	approveReq := &pbds.ApproveReq{
		BizId:         strategy.Attachment.BizID,
		AppId:         strategy.Attachment.AppID,
		ReleaseId:     strategy.Spec.ReleaseID,
		PublishStatus: string(table.PendingPublish),
		StrategyId:    strategy.ID,
	}

	// 获取active key
	// 根据版本和审批类型获取 stateIDKey 和 approveType
	stateIDKey := itsm.BuildStateIDKey(kit.TenantID, table.ApproveType(strategy.Spec.ApproveType))

	// 获取 ITSM 配置
	itsmSign, err := c.set.Config().GetConfig(kit, stateIDKey)
	if err != nil {
		logs.Errorf("get itsm config failed: %s", err.Error())
		return err
	}
	activeKey := itsmSign.Value
	approveResult, err := c.itsm.GetApproveResult(kit.InternalRpcCtx(), api.GetApproveResultReq{
		TicketID:    strategy.Spec.ItsmTicketSn,
		ActivityKey: activeKey,
	})
	if err != nil {
		logs.Errorf("get itsm approve result failed, err=%v, rid=%s", err, kit.Rid)
		return err
	}
	logs.Infof("itsm approve result: %v", approveResult)
	if approveResult.Result == nil {
		// 还没有结果， 等待
		logs.Errorf("itsm state is in processing,ticket %s", strategy.Spec.ItsmTicketSn)
		return nil
	}
	if *approveResult.Result {
		// 通过
		approveReq.PublishStatus = string(table.PendingPublish)
		approveReq.ApprovedBy = approveResult.PassUsers
	} else {
		// 拒绝
		approveReq.PublishStatus = string(table.RejectedApproval)
		approveReq.ApprovedBy = approveResult.RejectUsers
		approveReq.Reason = strings.Join(approveResult.Reasons, ",")
	}
	logs.Infof("itsm approve req: %v", approveReq)
	_, err = c.srv.Approve(kit.InternalRpcCtx(), approveReq)
	if err != nil {
		logs.Errorf("approve failed, err=%v, rid=%s", err, kit.Rid)
		return err
	}
	return nil
}
