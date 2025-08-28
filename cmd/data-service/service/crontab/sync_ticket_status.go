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
	"fmt"
	"strings"
	"sync"
	"time"

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
	snList := []string{}
	strategyMap := map[string]*table.Strategy{}
	for _, strategy := range strategys {
		snList = append(snList, strategy.Spec.ItsmTicketSn)
		strategyMap[strategy.Spec.ItsmTicketSn] = strategy
	}

	if len(snList) == 0 {
		return
	}

	page, pageSize := 0, 100

	for {
		resp, err := c.itsm.ListTickets(kt.Ctx, api.ListTicketsReq{
			Page:     page,
			PageSize: pageSize,
			Sns:      snList,
		})
		if err != nil {
			logs.Errorf("list approve itsm tickets %v failed, err: %s", snList, err.Error())
			return
		}

		if resp.Count == 0 {
			break
		}

		var approveReqs []*pbds.ApproveReq

		if cc.DataService().ITSM.EnableV4 {
			approveReqs, err = c.handleTicketStatusV4(kt, resp.Results, strategyMap)
		} else {
			approveReqs, err = c.handleTicketStatusV2(kt, resp.Results, strategyMap)
		}
		if err != nil {
			logs.Errorf("handle Ticket v2 status failed, err: %s", err.Error())
			return
		}

		for _, req := range approveReqs {
			if _, err := c.srv.Approve(kt.Ctx, req); err != nil {
				logs.Errorf("sync ticket status approve failed, strategyId=%d, err=%v", req.StrategyId, err)
				continue // 不中断整个批次
			}
		}

		page += pageSize
		if page >= resp.Count {
			break
		}

	}

}

// 批量 V2：运行中读取日志判断通过/拒绝；其它状态一律撤销
func (c *SyncTicketStatus) handleTicketStatusV2(kt *kit.Kit, tickets []*api.Ticket, strategyMap map[string]*table.Strategy,
) ([]*pbds.ApproveReq, error) {
	approveReqs := make([]*pbds.ApproveReq, 0, len(tickets))

	for _, ticket := range tickets {
		strategy, ok := strategyMap[ticket.SN]
		if !ok || strategy == nil {
			return nil, fmt.Errorf("strategy not found for ticket %s", ticket.SN)
		}

		status := strings.ToUpper(ticket.Status)

		if status == constant.TicketRunningStatu {
			// 运行中：看日志
			req := newApproveReq(strategy, string(table.PendingPublish))

			logsResp, err := c.itsm.GetTicketLogs(kt.Ctx, api.GetTicketLogsReq{TicketID: ticket.SN})
			if err != nil {
				return nil, fmt.Errorf("GetTicketLogs failed, sn=%s, err=%v", ticket.SN, err)
			}

			approveMap := parseApproveLogs(logsResp.Items)
			if len(approveMap) == 0 {
				// 没有有效日志：不下发请求（保持沉默）
				continue
			}

			if rejected, ok := approveMap[constant.ItsmRejectedApproveResult]; ok {
				reason, err := c.getApproveReason(kt, ticket.SN, strategy.Spec.ItsmTicketStateID)
				if err != nil {
					return nil, fmt.Errorf("GetApproveNodeResult failed, sn=%s, err=%v", ticket.SN, err)
				}
				req.PublishStatus = string(table.RejectedApproval)
				req.Reason = reason
				req.ApprovedBy = rejected
				approveReqs = append(approveReqs, req)
				continue
			}

			if passed, ok := approveMap[constant.ItsmPassedApproveResult]; ok {
				req.ApprovedBy = passed
				approveReqs = append(approveReqs, req)
				continue
			}

			// 未命中任何已知结果：跳过
			continue
		}

		// 非运行中：撤销
		req := newApproveReq(strategy, string(table.RevokedPublish))
		approveReqs = append(approveReqs, req)
	}

	// 批处理场景：错误都已日志并跳过，函数返回 nil error
	return approveReqs, nil
}

// 批量 V4：走 TicketDetail
func (c *SyncTicketStatus) handleTicketStatusV4(kt *kit.Kit, tickets []*api.Ticket, strategyMap map[string]*table.Strategy,
) ([]*pbds.ApproveReq, error) {

	approveReqs := make([]*pbds.ApproveReq, 0, len(tickets))

	for _, ticket := range tickets {
		strategy, ok := strategyMap[ticket.SN]
		if !ok || strategy == nil {
			return nil, fmt.Errorf("strategy not found for ticket %s", ticket.SN)
		}

		req := newApproveReq(strategy, string(table.RevokedPublish))

		detail, err := c.itsm.TicketDetail(kt.Ctx, api.TicketDetailReq{ID: ticket.ID})
		if err != nil {
			return nil, fmt.Errorf("TicketDetail failed, sn=%s, id=%s, err=%v", ticket.SN, ticket.ID, err)
		}

		for _, p := range detail.CurrentProcessors {
			req.ApprovedBy = append(req.ApprovedBy, p.Processor)
		}
		req.Reason = detail.CallbackResult.Message

		approveReqs = append(approveReqs, req)
	}

	return approveReqs, nil
}

func newApproveReq(strategy *table.Strategy, publishStatus string) *pbds.ApproveReq {
	return &pbds.ApproveReq{
		BizId:         strategy.Attachment.BizID,
		AppId:         strategy.Attachment.AppID,
		ReleaseId:     strategy.Spec.ReleaseID,
		PublishStatus: publishStatus,
		StrategyId:    strategy.ID,
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

func (c *SyncTicketStatus) getApproveReason(kt *kit.Kit, sn, stateID string) (string, error) {
	data, err := c.itsm.GetApproveNodeResult(kt.Ctx, api.GetApproveNodeResultReq{
		TicketID: sn,
		StateID:  stateID,
	})
	if err != nil {
		return "", err
	}
	return data.ApproveRemark, nil
}
