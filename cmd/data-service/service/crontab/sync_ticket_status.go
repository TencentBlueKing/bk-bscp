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

	"google.golang.org/grpc/metadata"

	"github.com/TencentBlueKing/bk-bscp/cmd/data-service/service"
	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm"
	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

const (
	defaultSyncTicketStatusInterval = 10 * time.Second
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
				// c.syncTicketStatus(kt)
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

	// 获取running、待上线，待审批状态的strategy记录
	strategys, err := c.set.Strategy().ListStrategyByItsm(kt)
	if err != nil {
		logs.Errorf("list strategy by itsm failed: %s", err.Error())
		return
	}
	// 根据 tenantID 分组
	tenantSNMap := make(map[string][]string)                         // tenantID -> sn 列表
	tenantStrategyMap := make(map[string]map[string]*table.Strategy) // tenantID -> (sn -> strategy)

	for _, strategy := range strategys {
		tenantID := strategy.Attachment.TenantID
		if tenantSNMap[tenantID] == nil {
			tenantSNMap[tenantID] = []string{}
		}
		tenantSNMap[tenantID] = append(tenantSNMap[tenantID], strategy.Spec.ItsmTicketSn)

		if tenantStrategyMap[tenantID] == nil {
			tenantStrategyMap[tenantID] = make(map[string]*table.Strategy)
		}
		tenantStrategyMap[tenantID][strategy.Spec.ItsmTicketSn] = strategy
	}

	for tenantID, snList := range tenantSNMap {
		if len(snList) == 0 {
			continue
		}

		ctx := kt.Ctx
		// 对于 v4 版本，添加租户信息到上下文
		if cc.DataService().ITSM.EnableV4 {
			md := metadata.MD{
				strings.ToLower(constant.BkTenantID): []string{tenantID},
			}
			ctx = metadata.NewIncomingContext(kt.Ctx, md)
		}

		c.handleTicketStatus(ctx, snList, tenantStrategyMap[tenantID])
	}

}

// handleTicketStatus 处理工单状态，统一处理V2和V4版本
func (c *SyncTicketStatus) handleTicketStatus(ctx context.Context, ticketIDs []string, strategyMap map[string]*table.Strategy) {
	// V2和V4版本使用相同的处理逻辑：运行中读取日志判断通过/拒绝；其它状态一律撤销
	for _, id := range ticketIDs {
		strategy, ok := strategyMap[id]
		if !ok || strategy == nil {
			logs.Errorf("strategy not found for ticket %s", id)
			return
		}

		// 获取单据状态
		ticket, err := c.itsm.GetTicketStatus(ctx, api.GetTicketStatusReq{
			TicketID: id,
		})
		if err != nil {
			logs.Errorf("get itsm ticket %s status failed, err: %v", id, err)
			return
		}

		// 正常状态的单据
		if ticket.CurrentStatus == constant.TicketRunningStatu {
			req := &pbds.ApproveReq{
				BizId:         strategyMap[id].Attachment.BizID,
				AppId:         strategyMap[id].Attachment.AppID,
				ReleaseId:     strategyMap[id].Spec.ReleaseID,
				PublishStatus: string(table.PendingPublish),
				StrategyId:    strategyMap[id].ID,
			}

			logsResp, errResult := c.itsm.GetTicketLogs(ctx, api.GetTicketLogsReq{TicketID: id})
			if errResult != nil {
				logs.Errorf("GetTicketLogs failed, err: %s", errResult.Error())
				return
			}
			approveMap := parseApproveLogs(logsResp.Items)
			if len(approveMap) == 0 {
				logs.Infof("no approve logs, id=%s", id)
				continue
			}

			// 失败需要有reason
			if _, ok := approveMap[constant.ItsmRejectedApproveResult]; ok {
				reason, err := c.getApproveReason(ctx, id, strategy.Spec.ItsmTicketStateID)
				if err != nil {
					logs.Errorf("GetApproveReason failed, sn=%s, err=%v", id, err)
					return
				}

				req.Reason = reason
				req.PublishStatus = string(table.RejectedApproval)
				req.ApprovedBy = approveMap[constant.ItsmRejectedApproveResult]
			}

			if _, ok := approveMap[constant.ItsmPassedApproveResult]; ok {
				req.ApprovedBy = approveMap[constant.ItsmPassedApproveResult]
			}

			_, err := c.srv.Approve(ctx, req)
			if err != nil {
				logs.Errorf("sync ticket status approve failed, err: %s", err.Error())
				continue
			}
		} else if ticket.CurrentStatus == constant.TicketRevokedStatu {
			// 其他状态的单据直接撤销
			req := &pbds.ApproveReq{
				BizId:         strategyMap[id].Attachment.BizID,
				AppId:         strategyMap[id].Attachment.AppID,
				ReleaseId:     strategyMap[id].Spec.ReleaseID,
				PublishStatus: string(table.RevokedPublish),
				StrategyId:    strategyMap[id].ID,
			}
			_, err := c.srv.Approve(ctx, req)
			if err != nil {
				logs.Errorf("sync ticket status approve failed, err: %s", err.Error())
				continue
			}
		}
	}
}

func parseApproveLogs(items []*api.TicketLogsDataItems) map[string][]string {
	result := make(map[string][]string)
	for _, v := range items {
		switch {
		case strings.Contains(v.Message, constant.ItsmRejectedApproveResult):
			result[constant.ItsmRejectedApproveResult] = append(result[constant.ItsmRejectedApproveResult], v.Operator)
		case strings.Contains(v.Message, constant.ItsmPassedApproveResult):
			result[constant.ItsmPassedApproveResult] = append(result[constant.ItsmPassedApproveResult], v.Operator)
		}
	}
	return result
}

func (c *SyncTicketStatus) getApproveReason(ctx context.Context, sn, stateID string) (string, error) {
	data, err := c.itsm.GetApproveNodeResult(ctx, api.GetApproveNodeResultReq{
		TicketID: sn,
		StateID:  stateID,
	})
	if err != nil {
		return "", err
	}
	return data.ApproveRemark, nil
}
