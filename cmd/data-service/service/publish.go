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

package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
	"github.com/TencentBlueKing/bk-bscp/internal/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/enumor"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/cache-service"
	pbgroup "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/group"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/runtime/selector"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// Publish exec publish strategy.
// nolint: funlen
func (s *Service) Publish(ctx context.Context, req *pbds.PublishReq) (*pbds.PublishResp, error) {
	// 只给流水线插件做兼容，该接口暂时还不能去除
	grpcKit := kit.FromGrpcContext(ctx)

	app, err := s.dao.App().Get(grpcKit, req.BizId, req.AppId)
	if err != nil {
		return nil, err
	}
	// 要么不审批立即上线，要么审批后自动上线
	publishType := table.Immediately
	if app.Spec.IsApprove {
		publishType = table.Automatically
	}
	return s.SubmitPublishApprove(ctx, &pbds.SubmitPublishApproveReq{
		BizId:           req.BizId,
		AppId:           req.AppId,
		ReleaseId:       req.ReleaseId,
		Memo:            req.Memo,
		All:             req.All,
		GrayPublishMode: req.GrayPublishMode,
		Default:         req.Default,
		Groups:          req.Groups,
		Labels:          req.Labels,
		GroupName:       req.GroupName,
		PublishType:     string(publishType),
		PublishTime:     "",
		IsCompare:       false,
	})
}

// SubmitPublishApprove submit publish strategy.
// nolint funlen
func (s *Service) SubmitPublishApprove(
	ctx context.Context, req *pbds.SubmitPublishApproveReq) (*pbds.PublishResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	app, err := s.dao.App().Get(grpcKit, req.BizId, req.AppId)
	if err != nil {
		return nil, err
	}

	release, err := s.dao.Release().Get(grpcKit, req.BizId, req.AppId, req.ReleaseId)
	if err != nil {
		return nil, err
	}
	if release.Spec.Deprecated {
		return nil, fmt.Errorf(i18n.T(grpcKit, "release %s is deprecated, can not be submited", release.Spec.Name))
	}

	// 获取最近的上线版本
	strategy, err := s.dao.Strategy().GetLast(grpcKit, req.BizId, req.AppId, 0, 0)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		strategy = &table.Strategy{
			Spec: &table.StrategySpec{},
		}
	}

	// 有在上线的版本则提示不能上线
	if strategy.Spec.PublishStatus == table.PendingApproval || strategy.Spec.PublishStatus == table.PendingPublish {
		return nil, errors.New(i18n.T(grpcKit, "there is a release in publishing currently"))
	}

	tx := s.dao.GenQuery().Begin()
	committed := false
	defer func() {
		if !committed {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, grpcKit.Rid)
			}
		}
	}()

	// group name
	var groupIDs []uint32
	var groupName []string
	// group 解析处理, 通过label创建
	groupIDs, groupName, err = s.parseGroup(grpcKit, req, tx)
	if err != nil {
		logs.Errorf("parse group failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	// parse publish option
	opt := s.parsePublishOption(req, app)
	opt.Groups = groupIDs
	opt.Revision = &table.CreatedRevision{
		Creator: grpcKit.User,
	}

	pshID, err := s.dao.Publish().SubmitWithTx(grpcKit, tx, opt)
	if err != nil {
		logs.Errorf("publish strategy failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	if req.All {
		groupName = []string{"ALL"}
	}

	resInstance := fmt.Sprintf(constant.ConfigReleaseName+constant.ResSeparator+constant.ConfigReleaseScope,
		release.Spec.Name, strings.Join(groupName, constant.NameSeparator))

	// audit this to create strategy details
	ad := s.dao.AuditDao().Decorator(grpcKit, opt.BizID, &table.AuditField{
		ResourceInstance: resInstance,
		Status:           enumor.AuditStatus(opt.PublishStatus),
		AppId:            app.AppID(),
		StrategyId:       pshID,
		IsCompare:        req.IsCompare,
		Detail:           req.Memo,
	}).PreparePublish(strategy)
	if err = ad.Do(tx.Query); err != nil {
		return nil, err
	}

	// 定时上线
	err = s.setPublishTime(grpcKit, pshID, req)
	if err != nil {
		return nil, err
	}

	// itsm流程创建ticket
	if app.Spec.IsApprove {
		scope := strings.Join(groupName, constant.NameSeparator)
		ticketData, errCreate := s.submitCreateApproveTicket(
			grpcKit, app, release.Spec.Name, scope, req.Memo, ad.GetAuditID(), release.ID)
		if errCreate != nil {
			logs.Errorf("submit create approve ticket, err: %v, rid: %s", errCreate, grpcKit.Rid)
			return nil, errCreate
		}

		err = s.dao.Strategy().UpdateByID(grpcKit, tx, pshID, map[string]interface{}{
			"itsm_ticket_type":     constant.ItsmTicketTypeCreate,
			"itsm_ticket_url":      ticketData.TicketURL,
			"itsm_ticket_sn":       ticketData.SN,
			"itsm_ticket_status":   constant.ItsmTicketStatusCreated,
			"itsm_ticket_state_id": ticketData.StateID,
		})

		if err != nil {
			logs.Errorf("update strategy by id err: %v, rid: %s", err, grpcKit.Rid)
			return nil, err
		}
	}

	// 不是空值表示被客户端拉取过
	var havePull bool
	if app.Spec.LastConsumedTime != nil {
		havePull = true
	}

	haveCredentials, err := s.checkAppHaveCredentials(grpcKit, req.BizId, req.AppId)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		logs.Errorf("commit transaction failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}
	committed = true

	resp := &pbds.PublishResp{
		PublishedStrategyHistoryId: pshID,
		HaveCredentials:            haveCredentials,
		HavePull:                   havePull,
	}
	return resp, nil
}

// Approve publish approve.
// nolint funlen
func (s *Service) Approve(ctx context.Context, req *pbds.ApproveReq) (*pbds.ApproveResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)
	logs.Infof("start approve operateway: %s, user: %s, req: %v", grpcKit.OperateWay, grpcKit.User, req)

	release, err := s.dao.Release().Get(grpcKit, req.BizId, req.AppId, req.ReleaseId)
	if err != nil {
		return nil, err
	}
	if release.Spec.Deprecated {
		return nil, errors.New(i18n.T(grpcKit, "release %s is deprecated, can not be revoke", release.Spec.Name))
	}

	strategy, err := s.dao.Strategy().GetLast(grpcKit, req.BizId, req.AppId, req.ReleaseId, req.StrategyId)
	if err != nil {
		return nil, err
	}

	app, err := s.dao.App().GetByID(grpcKit, req.AppId)
	if err != nil {
		return nil, err
	}

	// 从itsm回调的，如果状态跟数据库一样或者待上线状态直接返回结果
	if grpcKit.OperateWay == "" && (strategy.Spec.PublishStatus == table.PublishStatus(req.PublishStatus) ||
		strategy.Spec.PublishStatus == table.PublishStatus(table.PendingPublish) ||
		strategy.Spec.PublishStatus == table.PublishStatus(table.AlreadyPublish)) {
		return &pbds.ApproveResp{}, nil
	}

	var message string
	// 获取itsm ticket状态，不审批的不查
	// message 不为空的情况：itsm操作后数据不正常的message皆不为空，但数据库需要更新
	if app.Spec.IsApprove {
		req, message, err = s.checkTicketStatus(grpcKit,
			strategy.Spec.ItsmTicketSn, strategy.Spec.ItsmTicketStateID, req)
		if err != nil {
			return nil, err
		}
		logs.Infof("check ticket status, operateWay: %s, kit user: %s, approved by: %v, message: %s",
			grpcKit.OperateWay, grpcKit.User, req.ApprovedBy, message)
	}

	// 默认要回滚，除非已经提交
	tx := s.dao.GenQuery().Begin()
	committed := false
	defer func() {
		if !committed {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, grpcKit.Rid)
			}
		}
	}()

	var updateContent map[string]interface{}
	itsmUpdata := api.ApprovalTicketReq{}
	switch req.PublishStatus {
	case string(table.RevokedPublish):
		updateContent, err = s.revokeApprove(grpcKit, req, strategy)
		if err != nil {
			return nil, err
		}
		itsmUpdata = api.ApprovalTicketReq{
			TicketID:      strategy.Spec.ItsmTicketSn,
			Operator:      strategy.Revision.Creator,
			ActionMessage: fmt.Sprintf("BSCP 代理用户 %s 撤回: %s", grpcKit.User, req.Reason),
			ActionType:    "WITHDRAW",
		}
	case string(table.RejectedApproval):
		updateContent, err = s.rejectApprove(grpcKit, req, strategy)
		if err != nil {
			return nil, err
		}
		itsmUpdata = api.ApprovalTicketReq{
			TicketID: strategy.Spec.ItsmTicketSn,
			StateId:  strategy.Spec.ItsmTicketStateID,
			Approver: grpcKit.User,
			Action:   "false",
			Desc:     req.Reason,
		}
	case string(table.PendingPublish):
		updateContent, err = s.passApprove(grpcKit, tx, req, strategy)
		if err != nil {
			return nil, err
		}
		itsmUpdata = api.ApprovalTicketReq{
			TicketID: strategy.Spec.ItsmTicketSn,
			StateId:  strategy.Spec.ItsmTicketStateID,
			Approver: grpcKit.User,
			Action:   "true",
		}
	case string(table.AlreadyPublish):
		updateContent, err = s.publishApprove(grpcKit, tx, req, strategy)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New(i18n.T(grpcKit, "invalid publish_status: %s", req.PublishStatus))
	}

	updateContent["reviser"] = grpcKit.User
	updateContent["final_approval_time"] = time.Now().UTC()
	err = s.dao.Strategy().UpdateByID(grpcKit, tx, strategy.ID, updateContent)
	if err != nil {
		return nil, err
	}

	// update audit details
	err = s.dao.AuditDao().UpdateByStrategyID(grpcKit, tx, strategy.ID, map[string]interface{}{
		"status": updateContent["publish_status"],
	})
	if err != nil {
		return nil, err
	}

	// 从页面进来且需要审批的数据则同步itsm
	if app.Spec.IsApprove && grpcKit.OperateWay == string(enumor.WebUI) && message == "" &&
		strategy.Spec.ItsmTicketStatus == constant.ItsmTicketStatusCreated {
		// 撤销状态下，直接撤销
		if req.PublishStatus == string(table.RevokedPublish) {
			_, err = s.itsm.RevokedTicket(grpcKit.Ctx, itsmUpdata)
			if err != nil {
				return nil, err
			}
		}

		if req.PublishStatus == string(table.RejectedApproval) || req.PublishStatus == string(table.PendingPublish) {
			err = s.itsm.ApprovalTicket(grpcKit.Ctx, itsmUpdata)
			if err != nil {
				return nil, err
			}
		}
	}

	// 不是空值表示被客户端拉取过
	var havePull bool
	if app.Spec.LastConsumedTime != nil {
		havePull = true
	}

	haveCredentials, err := s.checkAppHaveCredentials(grpcKit, req.BizId, req.AppId)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		logs.Errorf("commit transaction failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}
	committed = true
	return &pbds.ApproveResp{
		HaveCredentials: haveCredentials,
		HavePull:        havePull,
		Message:         message,
	}, nil
}

// GenerateReleaseAndPublish generate release and publish.
// nolint: funlen
func (s *Service) GenerateReleaseAndPublish(ctx context.Context, req *pbds.GenerateReleaseAndPublishReq) (
	*pbds.PublishResp, error) {

	grpcKit := kit.FromGrpcContext(ctx)

	app, err := s.dao.App().GetByID(grpcKit, req.AppId)
	if err != nil {
		logs.Errorf("get app failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	if _, e := s.dao.Release().GetByName(grpcKit, req.BizId, req.AppId, req.ReleaseName); e == nil {
		return nil, errors.New(i18n.T(grpcKit, "release name %s already exists", req.ReleaseName))
	}

	// 获取最近的上线版本
	strategy, err := s.dao.Strategy().GetLast(grpcKit, req.BizId, req.AppId, 0, 0)
	if err != nil {
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
			strategy = &table.Strategy{
				Spec: &table.StrategySpec{},
			}
		}
	}

	// 有在上线的版本则提示不能上线
	if strategy.Spec.PublishStatus == table.PendingApproval || strategy.Spec.PublishStatus == table.PendingPublish {
		return nil, errors.New(i18n.T(grpcKit, "there is a release in publishing currently"))
	}

	// 默认要回滚，除非已经提交
	tx := s.dao.GenQuery().Begin()
	committed := false
	defer func() {
		if !committed {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, grpcKit.Rid)
			}
		}
	}()

	groupIDs, groupName, err := s.genReleaseAndPublishGroupID(grpcKit, tx, req)
	if err != nil {
		return nil, err
	}

	// create release.
	release := &table.Release{
		Spec: &table.ReleaseSpec{
			Name: req.ReleaseName,
			Memo: req.ReleaseMemo,
		},
		Attachment: &table.ReleaseAttachment{
			BizID: req.BizId,
			AppID: req.AppId,
		},
		Revision: &table.CreatedRevision{
			Creator: grpcKit.User,
		},
	}
	releaseID, err := s.dao.Release().CreateWithTx(grpcKit, tx, release)
	if err != nil {
		logs.Errorf("create release failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}
	// create released hook.
	if err = s.createReleasedHook(grpcKit, tx, req.BizId, req.AppId, releaseID); err != nil {
		logs.Errorf("create released hook failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	switch app.Spec.ConfigType {
	case table.File:

		// Note: need to change batch operator to query config item and it's commit.
		// query app's all config items.
		cfgItems, e := s.getAppConfigItems(grpcKit)
		if e != nil {
			logs.Errorf("query app config item list failed, err: %v, rid: %s", e, grpcKit.Rid)
			return nil, e
		}

		// get app template revisions which are template config items
		tmplRevisions, e := s.getAppTmplRevisions(grpcKit)
		if e != nil {
			logs.Errorf("get app template revisions failed, err: %v, rid: %s", e, grpcKit.Rid)
			return nil, e
		}

		// if no config item, return directly.
		if len(cfgItems) == 0 && len(tmplRevisions) == 0 {
			return nil, errors.New("app config items is empty")
		}

		// do template and non-template config item related operations for create release.
		if err = s.doConfigItemOperations(grpcKit, req.Variables, tx, release.ID, tmplRevisions, cfgItems); err != nil {
			logs.Errorf("do template action for create release failed, err: %v, rid: %s", err, grpcKit.Rid)
			return nil, err
		}
	case table.KV:
		if err = s.doKvOperations(grpcKit, tx, req.AppId, req.BizId, release.ID); err != nil {
			logs.Errorf("do kv action for create release failed, err: %v, rid: %s", err, grpcKit.Rid)
			return nil, err
		}
	}

	// publish with transaction.
	kt := kit.FromGrpcContext(ctx)

	opt := &types.PublishOption{
		BizID:     req.BizId,
		AppID:     req.AppId,
		ReleaseID: releaseID,
		All:       req.All,
		Memo:      req.ReleaseMemo,
		Groups:    groupIDs,
		Revision: &table.CreatedRevision{
			Creator: kt.User,
		},
		PublishType:   table.Immediately,
		PublishStatus: table.AlreadyPublish,
		PubState:      string(table.Publishing),
		ApproveType:   string(app.Spec.ApproveType),
	}

	// if approval required, current approver required, pub_state unpublished
	if app.Spec.IsApprove {
		opt.PublishType = table.Automatically
		opt.PublishStatus = table.PendingApproval
		opt.Approver = app.Spec.Approver
		opt.ApproverProgress = app.Spec.Approver
		opt.PubState = string(table.Unpublished)
	}

	pshID, err := s.dao.Publish().SubmitWithTx(grpcKit, tx, opt)
	if err != nil {
		logs.Errorf("submit with tx failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	if req.All {
		groupName = []string{"ALL"}
	}

	resInstance := fmt.Sprintf(constant.ConfigReleaseName+constant.ResSeparator+constant.ConfigReleaseScope,
		release.Spec.Name, strings.Join(groupName, constant.NameSeparator))

	// audit this to create strategy details
	ad := s.dao.AuditDao().Decorator(grpcKit, opt.BizID, &table.AuditField{
		ResourceInstance: resInstance,
		Status:           enumor.AuditStatus(opt.PublishStatus),
		StrategyId:       pshID,
	}).PreparePublish(strategy)
	if err = ad.Do(tx.Query); err != nil {
		return nil, err
	}

	// itsm流程创建ticket
	if app.Spec.IsApprove {
		scope := strings.Join(groupName, constant.NameSeparator)
		ticketData, errCreate := s.submitCreateApproveTicket(
			grpcKit, app, release.Spec.Name, scope, req.ReleaseMemo, ad.GetAuditID(), release.ID)
		if errCreate != nil {
			logs.Errorf("submit create approve ticket, err: %v, rid: %s", errCreate, grpcKit.Rid)
			return nil, errCreate
		}

		err = s.dao.Strategy().UpdateByID(grpcKit, tx, pshID, map[string]interface{}{
			"itsm_ticket_type":     constant.ItsmTicketTypeCreate,
			"itsm_ticket_url":      ticketData.TicketURL,
			"itsm_ticket_sn":       ticketData.SN,
			"itsm_ticket_status":   constant.ItsmTicketStatusCreated,
			"itsm_ticket_state_id": ticketData.StateID,
		})

		if err != nil {
			logs.Errorf("update strategy by id err: %v, rid: %s", err, grpcKit.Rid)
			return nil, err
		}
	}
	// commit transaction.
	if err = tx.Commit(); err != nil {
		logs.Errorf("commit transaction failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	committed = true
	return &pbds.PublishResp{PublishedStrategyHistoryId: pshID}, nil
}

// revokeApprove revoke publish approve.
func (s *Service) revokeApprove(
	kit *kit.Kit, req *pbds.ApproveReq, strategy *table.Strategy) (map[string]interface{}, error) {

	// 只有待上线以及待审批的类型才允许撤回
	if strategy.Spec.PublishStatus != table.PendingPublish && strategy.Spec.PublishStatus != table.PendingApproval {
		return nil, errors.New(i18n.T(kit, "revoked not allowed, current publish status is: %s",
			strategy.Spec.PublishStatus))
	}

	return map[string]interface{}{
		"publish_status":     table.RevokedPublish,
		"reject_reason":      req.Reason,
		"approver_progress":  strategy.Revision.Creator,
		"itsm_ticket_status": constant.ItsmTicketStatusRevoked,
	}, nil
}

// rejectApprove reject publish approve.
func (s *Service) rejectApprove(
	kit *kit.Kit, req *pbds.ApproveReq, strategy *table.Strategy) (map[string]interface{}, error) {

	if strategy.Spec.PublishStatus != table.PendingApproval {
		return nil, errors.New(i18n.T(kit, "rejected not allowed, current publish status is: %s",
			strategy.Spec.PublishStatus))
	}

	if req.Reason == "" {
		return nil, errors.New(i18n.T(kit, "reason can not empty"))
	}

	var rejector string
	// 判断是否在审批人队列
	users := strings.Split(strategy.Spec.ApproverProgress, ",")
	for _, v := range users {
		if v == kit.User {
			rejector = v
			break
		}
		for _, vv := range req.ApprovedBy {
			if v == vv {
				rejector = vv
				break
			}
		}
	}

	// 需要审批但不是审批人的情况返回无权限审批
	if rejector == "" {
		return nil, errors.New(i18n.T(kit, "no permission to approve"))
	}

	return map[string]interface{}{
		"publish_status":     table.RejectedApproval,
		"reject_reason":      req.Reason,
		"approver_progress":  rejector,
		"itsm_ticket_status": constant.ItsmTicketStatusRejected,
	}, nil
}

// passApprove pass publish approve.
func (s *Service) passApprove(
	kit *kit.Kit, tx *gen.QueryTx, req *pbds.ApproveReq, strategy *table.Strategy) (map[string]interface{}, error) {

	if strategy.Spec.PublishStatus != table.PendingApproval {
		return nil, errors.New(i18n.T(kit, "pass not allowed, current publish status is: %s",
			strategy.Spec.PublishStatus))
	}

	// 判断是否在审批人队列
	isApprover := false
	progressUsers := strings.Split(strategy.Spec.ApproverProgress, ",")
	// 新的审批人列表
	var newProgressUsers []string
	for _, v := range progressUsers {
		isRemove := false
		// 与itsm已经通过的审批人列表做对比
		for _, vv := range req.ApprovedBy {
			if vv == v {
				isRemove = true
				break
			}
		}
		if v == kit.User {
			isApprover = true
			isRemove = true
		}

		// 不需要移除的审批人列表
		if !isRemove {
			newProgressUsers = append(newProgressUsers, v)
		}
	}

	// 页面过来的数据不是审批人的情况返回无权限审批
	if !isApprover && kit.OperateWay == string(enumor.WebUI) {
		return nil, errors.New(i18n.T(kit, "no permission to approve"))
	}

	result := make(map[string]interface{})
	publishStatus := table.PendingApproval
	// 或签通过或者是只有一个审批人的情况
	if strategy.Spec.ApproveType == string(table.OrSign) || strategy.Spec.Approver == kit.User {
		publishStatus = table.PendingPublish
		result["approver_progress"] = kit.User // 需要更新下给前端展示
		result["itsm_ticket_status"] = constant.ItsmTicketStatusPassed
	} else {
		// 会签通过
		// 最后一个的情况下，直接待上线
		if len(newProgressUsers) == 0 || kit.OperateWay == "" {
			publishStatus = table.PendingPublish
			result["approver_progress"] = strategy.Spec.Approver
			result["itsm_ticket_status"] = constant.ItsmTicketStatusPassed
		} else {
			// 审批人列表更新
			result["approver_progress"] = strings.Join(newProgressUsers, constant.NameSeparator)
		}
	}

	// 自动上线则直接上线
	if publishStatus == table.PendingPublish && strategy.Spec.PublishType == table.Automatically {
		opt := types.PublishOption{
			BizID:     req.BizId,
			AppID:     req.AppId,
			ReleaseID: req.ReleaseId,
			All:       false,
		}

		if len(strategy.Spec.Scope.Groups) == 0 {
			opt.All = true
		}

		err := s.dao.Publish().UpsertPublishWithTx(kit, tx, &opt, strategy)

		if err != nil {
			return nil, err
		}
		publishStatus = table.AlreadyPublish
	}

	result["publish_status"] = publishStatus
	return result, nil
}

// publishApprove publish approve.
func (s *Service) publishApprove(
	kit *kit.Kit, tx *gen.QueryTx, req *pbds.ApproveReq, strategy *table.Strategy) (map[string]interface{}, error) {

	if strategy.Spec.PublishStatus != table.PendingPublish {
		return nil, errors.New(i18n.T(kit, "publish not allowed, current publish status is: %s",
			strategy.Spec.PublishStatus))
	}

	opt := types.PublishOption{
		BizID:     req.BizId,
		AppID:     req.AppId,
		ReleaseID: req.ReleaseId,
		All:       false,
	}

	if len(strategy.Spec.Scope.Groups) == 0 {
		opt.All = true
	}

	err := s.dao.Publish().UpsertPublishWithTx(kit, tx, &opt, strategy)

	if err != nil {
		return nil, err
	}
	publishStatus := table.AlreadyPublish

	return map[string]interface{}{
		"pub_state":      table.Publishing,
		"publish_status": publishStatus,
	}, nil
}

// parse publish option
func (s *Service) parsePublishOption(req *pbds.SubmitPublishApproveReq, app *table.App) *types.PublishOption {

	opt := &types.PublishOption{
		BizID:         req.BizId,
		AppID:         req.AppId,
		ReleaseID:     req.ReleaseId,
		All:           req.All,
		Default:       req.Default,
		Memo:          req.Memo,
		PublishType:   table.PublishType(req.PublishType),
		PublishTime:   req.PublishTime,
		PublishStatus: table.PendingPublish,
		PubState:      string(table.Publishing),
		ApproveType:   string(app.Spec.ApproveType),
	}

	// if approval required, current approver required, pub_state unpublished
	if app.Spec.IsApprove {
		opt.PublishStatus = table.PendingApproval
		opt.Approver = app.Spec.Approver
		opt.ApproverProgress = app.Spec.Approver
		opt.PubState = string(table.Unpublished)
	}

	// publish immediately
	if req.PublishType == string(table.Immediately) {
		opt.PublishStatus = table.AlreadyPublish
	}

	return opt
}

// checkAppHaveCredentials check if there is available credential for app.
// 1. credential scope can match app name.
// 2. credential is enabled.
func (s *Service) checkAppHaveCredentials(grpcKit *kit.Kit, bizID, appID uint32) (bool, error) {
	app, err := s.dao.App().Get(grpcKit, bizID, appID)
	if err != nil {
		return false, err
	}
	matchedCredentials := make([]uint32, 0)
	scopes, err := s.dao.CredentialScope().ListAll(grpcKit, bizID)
	if err != nil {
		return false, err
	}
	if len(scopes) == 0 {
		return false, nil
	}
	for _, scope := range scopes {
		match, e := scope.Spec.CredentialScope.MatchApp(app.Spec.Name)
		if e != nil {
			return false, e
		}
		if match {
			matchedCredentials = append(matchedCredentials, scope.Attachment.CredentialId)
		}
	}
	credentials, e := s.dao.Credential().BatchListByIDs(grpcKit, bizID, matchedCredentials)
	if e != nil {
		return false, e
	}
	for _, credential := range credentials {
		if credential.Spec.Enable {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) genReleaseAndPublishGroupID(grpcKit *kit.Kit, tx *gen.QueryTx,
	req *pbds.GenerateReleaseAndPublishReq) ([]uint32, []string, error) {

	groupIDs := make([]uint32, 0)
	groupNames := make([]string, 0)

	if !req.All {
		if req.GrayPublishMode == "" {
			// !NOTE: Compatible with previous pipelined plugins version
			req.GrayPublishMode = table.PublishByGroups.String()
		}
		publishMode := table.GrayPublishMode(req.GrayPublishMode)
		if e := publishMode.Validate(); e != nil {
			return groupIDs, groupNames, e
		}
		// validate and query group ids.
		if publishMode == table.PublishByGroups {
			for _, name := range req.Groups {
				group, e := s.dao.Group().GetByName(grpcKit, req.BizId, name)
				if e != nil {
					return groupIDs, groupNames, fmt.Errorf("group %s not exist", name)
				}
				groupIDs = append(groupIDs, group.ID)
				groupNames = append(groupNames, group.Spec.Name)
			}
		}
		if publishMode == table.PublishByLabels {
			groupID, e := s.getOrCreateGroupByLabels(grpcKit, tx, req.BizId, req.AppId, req.GroupName, req.Labels)
			if e != nil {
				logs.Errorf("create group by labels failed, err: %v, rid: %s", e, grpcKit.Rid)
				return groupIDs, groupNames, e
			}
			groupIDs = append(groupIDs, groupID)
			groupNames = append(groupNames, req.GroupName)
		}
	}

	return groupIDs, groupNames, nil
}

func (s *Service) getOrCreateGroupByLabels(grpcKit *kit.Kit, tx *gen.QueryTx, bizID, appID uint32, groupName string,
	labels []*structpb.Struct) (uint32, error) {
	elements := make([]selector.Element, 0)
	for _, label := range labels {
		element, err := pbgroup.UnmarshalElement(label)
		if err != nil {
			return 0, fmt.Errorf("unmarshal group label failed, err: %v", err)
		}
		elements = append(elements, *element)
	}
	sel := &selector.Selector{
		LabelsAnd: elements,
	}
	groups, err := s.dao.Group().ListAppValidGroups(grpcKit, bizID, appID)
	if err != nil {
		return 0, err
	}
	exists := make([]*table.Group, 0)
	for _, group := range groups {
		if group.Spec.Selector.Equal(sel) {
			exists = append(exists, group)
		}
	}
	// if same labels group exists, return it's id.
	if len(exists) > 0 {
		return exists[0].ID, nil
	}
	// else create new one.
	if groupName != "" {
		// if group name is not empty, use it as group name.
		_, err = s.dao.Group().GetByName(grpcKit, bizID, groupName)
		// if group name already exists, return error.
		if err == nil {
			return 0, fmt.Errorf("group %s already exists", groupName)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}
	} else {
		// generate group name by time.
		groupName = time.Now().Format("20060102150405.000")
		groupName = fmt.Sprintf("g_%s", strings.ReplaceAll(groupName, ".", ""))
	}
	group := table.Group{
		Spec: &table.GroupSpec{
			Name:     groupName,
			Public:   false,
			Mode:     table.GroupModeCustom,
			Selector: sel,
		},
		Attachment: &table.GroupAttachment{
			BizID: bizID,
		},
		Revision: &table.Revision{
			Creator: grpcKit.User,
			Reviser: grpcKit.User,
		},
	}
	groupID, err := s.dao.Group().CreateWithTx(grpcKit, tx, &group)
	if err != nil {
		return 0, err
	}
	if err := s.dao.GroupAppBind().BatchCreateWithTx(grpcKit, tx, []*table.GroupAppBind{
		{
			GroupID: groupID,
			AppID:   appID,
			BizID:   bizID,
		},
	}); err != nil {
		return 0, err
	}
	return groupID, nil
}

func (s *Service) createReleasedHook(grpcKit *kit.Kit, tx *gen.QueryTx, bizID, appID, releaseID uint32) error {
	pre, err := s.dao.ReleasedHook().Get(grpcKit, bizID, appID, 0, table.PreHook)
	if err == nil {
		pre.ID = 0
		pre.ReleaseID = releaseID
		if _, e := s.dao.ReleasedHook().CreateWithTx(grpcKit, tx, pre); e != nil {
			logs.Errorf("create released pre-hook failed, err: %v, rid: %s", e, grpcKit.Rid)
			return e
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		logs.Errorf("query released pre-hook failed, err: %v, rid: %s", err, grpcKit.Rid)
		return err
	}
	post, err := s.dao.ReleasedHook().Get(grpcKit, bizID, appID, 0, table.PostHook)
	if err == nil {
		post.ID = 0
		post.ReleaseID = releaseID
		if _, e := s.dao.ReleasedHook().CreateWithTx(grpcKit, tx, post); e != nil {
			logs.Errorf("create released post-hook failed, err: %v, rid: %s", e, grpcKit.Rid)
			return e
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		logs.Errorf("query released post-hook failed, err: %v, rid: %s", err, grpcKit.Rid)
		return err
	}
	return nil
}

// submitCreateApproveTicket create new itsm create approve ticket
// nolint: funlen
func (s *Service) submitCreateApproveTicket(kt *kit.Kit, app *table.App, releaseName, scope, memo string,
	aduitId, releaseID uint32) (*api.CreateTicketData, error) {

	// 根据版本和审批类型获取 stateIDKey 和 approveType
	stateIDKey, approveType := buildApproveConfig(kt.TenantID, app.Spec.ApproveType, cc.DataService().ITSM.EnableV4)

	// 获取 ITSM 配置
	itsmSign, err := s.dao.Config().GetConfig(kt, stateIDKey)
	if err != nil {
		return nil, err
	}
	itsmService, err := s.dao.Config().GetConfig(kt, constant.CreateApproveItsmServiceID)
	if err != nil {
		return nil, err
	}

	// 获取业务名
	bizName, err := s.getBizName(kt, app.BizID)
	if err != nil {
		return nil, err
	}

	// 组装字段
	fields := buildFields(bizName, app, releaseName, scope, aduitId, releaseID, approveType, memo)

	// 创建工单
	resp, err := s.itsm.CreateTicket(kt.Ctx, api.CreateTicketReq{
		WorkFlowKey: fmt.Sprintf("%s-%s", kt.TenantID, constant.CreateApproveItsmWorkflowID),
		ServiceID:   itsmService.Value,
		Fields:      fields,
		Operator:    kt.User,
		Meta: map[string]any{
			"state_processors": map[string]any{itsmSign.Value: app.Spec.Approver},
		},
	})
	if err != nil {
		return nil, err
	}

	// 获取审批节点 stateID
	resp.StateID, err = s.resolveStateID(kt, resp.SN, itsmSign.Value, cc.DataService().ITSM.EnableV4)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// 提取配置
func buildApproveConfig(tenantID string, approveType table.ApproveType, enableV4 bool) (string, table.ApproveType) {
	if enableV4 {
		if approveType == table.CountSign {
			return fmt.Sprintf("%s-%s", tenantID, constant.CreateCountSignApproveItsmStateID), table.CountSignCH
		}
		return fmt.Sprintf("%s-%s", tenantID, constant.CreateOrSignApproveItsmStateID), table.OrSignCH
	}

	if approveType == table.CountSign {
		return constant.CreateCountSignApproveItsmStateID, table.CountSignCH
	}
	return constant.CreateOrSignApproveItsmStateID, table.OrSignCH
}

// 获取业务名
func (s *Service) getBizName(kt *kit.Kit, bizID uint32) (string, error) {
	bizList, err := s.esb.Cmdb().ListAllBusiness(kt.Ctx)
	if err != nil {
		return "", err
	}
	if len(bizList.Info) == 0 {
		return "", errors.New(i18n.T(kt, "biz list is empty"))
	}
	for _, biz := range bizList.Info {
		if biz.BizID == int64(bizID) {
			return biz.BizName, nil
		}
	}
	return "", fmt.Errorf("biz %d not found", bizID)
}

// 构建 fields
func buildFields(bizName string, app *table.App, releaseName, scope string, aduitId, releaseID uint32,
	approveType table.ApproveType, memo string) []map[string]any {

	return []map[string]any{
		{"key": "title", "value": "服务配置中心(BSCP)版本上线审批"},
		{"key": "BIZ", "value": fmt.Sprintf("%s(%d)", bizName, app.BizID)},
		{"key": "APP", "value": app.Spec.Name},
		{"key": "RELEASE_NAME", "value": releaseName},
		{"key": "SCOPE", "value": scope},
		{"key": "COMPARE", "value": fmt.Sprintf("%s/space/%d/records/all?limit=1&id=%d",
			cc.DataService().ITSM.BscpPageUrl, app.BizID, aduitId)},
		{"key": "BIZ_ID", "value": app.BizID},
		{"key": "APP_ID", "value": app.ID},
		{"key": "RELEASE_ID", "value": releaseID},
		{"key": "APPROVE_TYPE", "value": approveType},
		{"key": "MEMO", "value": memo},
		{"key": "approve_type", "value": approveType},
		{"key": "approve", "value": app.Spec.Approver},
	}
}

// 解析 stateID
func (s *Service) resolveStateID(kt *kit.Kit, ticketSN, activityKey string, enableV4 bool) (int, error) {
	if enableV4 {
		tasks, err := s.itsm.ApprovalTasks(kt.Ctx, api.ApprovalTasksReq{
			TicketID:    ticketSN,
			ActivityKey: activityKey,
		})
		if err != nil {
			return 0, err
		}
		if len(tasks.Items) == 0 {
			return 0, fmt.Errorf("approval tasks is empty")
		}
		return strconv.Atoi(tasks.Items[0].ID)
	}
	return strconv.Atoi(activityKey)
}

// 定时上线
func (s *Service) setPublishTime(kt *kit.Kit, pshID uint32, req *pbds.SubmitPublishApproveReq) error {
	if req.PublishType == string(table.Scheduled) {
		publishTime, err := time.Parse(time.DateTime, req.PublishTime)
		if err != nil {
			logs.Errorf("parse time failed, err: %v, rid: %s", err, kt.Rid)
			return err
		}

		_, err = s.cs.SetPublishTime(kt.Ctx, &pbcs.SetPublishTimeReq{
			BizId:       req.BizId,
			StrategyId:  pshID,
			PublishTime: publishTime.Unix(),
			AppId:       req.AppId,
		})
		if err != nil {
			logs.Errorf("set publish time failed, err: %v, rid: %s", err, kt.Rid)
			return err
		}
	}
	return nil
}

// group 解析处理, 通过label创建
func (s *Service) parseGroup(
	grpcKit *kit.Kit, req *pbds.SubmitPublishApproveReq, tx *gen.QueryTx) ([]uint32, []string, error) {
	// group name
	groupIDs := make([]uint32, 0)
	groupName := []string{}
	if !req.All {
		if req.GrayPublishMode == "" {
			// !NOTE: Compatible with previous pipelined plugins version
			req.GrayPublishMode = table.PublishByGroups.String()
		}
		publishMode := table.GrayPublishMode(req.GrayPublishMode)
		if e := publishMode.Validate(); e != nil {
			return groupIDs, groupName, e
		}
		// validate and query group ids.
		if publishMode == table.PublishByGroups {
			for _, groupID := range req.Groups {
				if groupID == 0 {
					groupIDs = append(groupIDs, groupID)
					continue
				}
				group, e := s.dao.Group().Get(grpcKit, groupID, req.BizId)
				if e != nil {
					return groupIDs, groupName, fmt.Errorf("group %d not exist", groupID)
				}
				groupIDs = append(groupIDs, group.ID)
				groupName = append(groupName, group.Spec.Name)
			}
		}
		if publishMode == table.PublishByLabels {
			groupID, gErr := s.getOrCreateGroupByLabels(grpcKit, tx, req.BizId, req.AppId, req.GroupName, req.Labels)
			if gErr != nil {
				logs.Errorf("create group by labels failed, err: %v, rid: %s", gErr, grpcKit.Rid)
				return groupIDs, groupName, fmt.Errorf("get group by labels failed: %s", gErr)
			}
			groupIDs = append(groupIDs, groupID)
			groupName = append(groupName, req.GroupName)
		}
	}
	return groupIDs, groupName, nil
}

func (s *Service) checkTicketStatus(kt *kit.Kit, sn string, stateID int, req *pbds.ApproveReq) (*pbds.ApproveReq, string, error) {
	if req.PublishStatus == string(table.AlreadyPublish) {
		return req, "", nil
	}

	if cc.DataService().ITSM.EnableV4 {
		return s.handleTicketStatusV4(kt, sn, req)
	}
	return s.handleTicketStatusV2(kt, sn, stateID, req)
}

func (s *Service) handleTicketStatusV2(kt *kit.Kit, sn string, stateID int, req *pbds.ApproveReq) (*pbds.ApproveReq, string, error) {
	statusResp, err := s.itsm.GetTicketStatus(kt.Ctx, api.GetTicketStatusReq{TicketID: sn})
	if err != nil {
		return req, "", err
	}

	switch statusResp.CurrentStatus {
	case constant.TicketRunningStatu:
		return s.handleRunningStatus(kt, sn, stateID, req)
	case constant.TicketRevokedStatu:
		req.PublishStatus = string(table.RevokedPublish)
		return req, i18n.T(kt, "this ticket has been revoked, no further processing is required"), nil
	case constant.TicketFinishedStatu:
		if req.PublishStatus == string(table.RevokedPublish) {
			return req, "", nil
		}
		return req, i18n.T(kt, "this ticket has been finished, no further processing is required"), nil
	default:
		req.PublishStatus = string(table.RevokedPublish)
		req.Reason = "invalid tikcet status: " + statusResp.CurrentStatus
		return req, i18n.T(kt, "approval has been revoked, invalid tikcet status: %s", statusResp.CurrentStatus), nil
	}
}

func (s *Service) handleRunningStatus(kt *kit.Kit, sn string, stateID int, req *pbds.ApproveReq) (*pbds.ApproveReq, string, error) {
	// 页面撤回直接返回
	if kt.OperateWay == string(enumor.WebUI) && req.PublishStatus == string(table.RevokedPublish) {
		return req, "", nil
	}

	logs, err := s.itsm.GetTicketLogs(kt.Ctx, api.GetTicketLogsReq{TicketID: sn})
	if err != nil {
		return req, "", err
	}

	approveMap := s.parseApproveLogs(logs.Items)

	if rejectedUsers, ok := approveMap[constant.ItsmRejectedApproveResult]; ok {
		reason, err := s.getApproveReason(kt, sn, stateID)
		if err != nil {
			return req, "", err
		}
		req.PublishStatus = string(table.RejectedApproval)
		req.ApprovedBy = rejectedUsers
		req.Reason = reason
		return req, i18n.T(kt, "this ticket has been approved, no further processing is required"), nil
	}

	if passedUsers, ok := approveMap[constant.ItsmPassedApproveResult]; ok {
		if kt.OperateWay == string(enumor.WebUI) && req.PublishStatus == string(table.RejectedApproval) {
			return req, "", nil
		}
		req.PublishStatus = string(table.PendingPublish)
		req.ApprovedBy = passedUsers
		for _, user := range passedUsers {
			if user == kt.User || kt.OperateWay != string(enumor.WebUI) {
				kt.User = user
				return req, i18n.T(kt, "this ticket has been approved, no further processing is required"), nil
			}
		}
	}
	return req, "", nil
}

func (s *Service) parseApproveLogs(items []*api.TicketLogsDataItems) map[string][]string {
	result := make(map[string][]string)
	for _, v := range items {
		if strings.Contains(v.Message, constant.ItsmRejectedApproveResult) {
			result[constant.ItsmRejectedApproveResult] = append(result[constant.ItsmRejectedApproveResult], v.Operator)
		} else if strings.Contains(v.Message, constant.ItsmPassedApproveResult) {
			result[constant.ItsmPassedApproveResult] = append(result[constant.ItsmPassedApproveResult], v.Operator)
		}
	}
	return result
}

func (s *Service) getApproveReason(kt *kit.Kit, sn string, stateID int) (string, error) {
	data, err := s.itsm.GetApproveNodeResult(kt.Ctx, api.GetApproveNodeResultReq{
		TicketID: sn,
		StateID:  stateID,
	})
	if err != nil {
		return "", err
	}
	return data.ApproveRemark, nil
}

func (s *Service) handleTicketStatusV4(kt *kit.Kit, sn string, req *pbds.ApproveReq) (*pbds.ApproveReq, string, error) {
	detail, err := s.itsm.TicketDetail(kt.Ctx, api.TicketDetailReq{ID: sn})
	if err != nil {
		return req, "", err
	}
	for _, v := range detail.CurrentProcessors {
		req.ApprovedBy = append(req.ApprovedBy, v.Processor)
	}
	req.PublishStatus = string(table.RevokedPublish)
	req.Reason = detail.CallbackResult.Message
	return req, "", nil
}

// ApprovalCallback implements pbds.DataServer.
func (s *Service) ApprovalCallback(ctx context.Context, req *pbds.ApprovalCallbackReq) (*pbds.ApprovalCallbackResp, error) {
	// grpcKit := kit.FromGrpcContext(ctx)
	// // 单据创建时会触发回调
	// // 单据更新状态时也会触发回调
	// // 通过回调返回的状态来同步我们表中的状态
	// // 只需要处理 处理中 的状态
	// strategy, err := s.dao.Strategy().GetStrategyBySnAndState(grpcKit, req.Ticket.Id, table.RunningItsmTicketStatus)
	// if err != nil {
	// 	return nil, err
	// }

	// if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
	// 	return nil, err
	// }

	// // 默认要回滚，除非已经提交
	// tx := s.dao.GenQuery().Begin()
	// committed := false
	// defer func() {
	// 	if !committed {
	// 		if rErr := tx.Rollback(); rErr != nil {
	// 			logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, grpcKit.Rid)
	// 		}
	// 	}
	// }()

	// // 只处理已完成、终止、撤单的状态
	// if req.Ticket.Status == table.FinishedItsmTicketStatus.String() ||
	// 	req.Ticket.Status == table.TerminationItsmTicketStatus.String() ||
	// 	req.Ticket.Status == table.RevokedItsmTicketStatus.String() {

	// }

	// var updateContent map[string]any

	// switch req.Ticket.Status {
	// // 撤单
	// case table.RevokedItsmTicketStatus.String():
	// 	// 只有待上线以及审批中的才能撤单
	// 	if strategy.Spec.PublishStatus != table.PendingPublish && strategy.Spec.PublishStatus != table.PendingApproval {
	// 		return nil, errors.New(i18n.T(grpcKit, "revoked not allowed, current publish status is: %s",
	// 			strategy.Spec.PublishStatus))
	// 	}

	// 	updateContent = map[string]any{
	// 		"publish_status":     table.RevokedPublish,
	// 		"reject_reason":      req.GetTicket().GetCallbackResult().String(), // 目前把回调的结果当原因，如果获取不到，那只能通过日志
	// 		"approver_progress":  strategy.Revision.Creator,
	// 		"itsm_ticket_status": constant.ItsmTicketStatusRevoked,
	// 	}
	// case table.TerminationItsmTicketStatus.String():

	// case table.FinishedItsmTicketStatus.String():
	// 	updateContent = map[string]any{
	// 		"publish_status":     table.RevokedPublish,
	// 		"reject_reason":
	// 		"approver_progress":  strategy.Revision.Creator,
	// 		"itsm_ticket_status": constant.ItsmTicketStatusRevoked,
	// 	}

	// }

	return &pbds.ApprovalCallbackResp{
		Code:    0,
		Message: "",
	}, nil
}

// SubmitApproval implements pbds.DataServer.
func (s *Service) SubmitApproval(ctx context.Context, req *pbds.SubmitApprovalReq) (*pbds.SubmitApprovalResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	logs.Infof("start approve operateway: %s, user: %s, req: %v", grpcKit.OperateWay, grpcKit.User, req)

	release, err := s.dao.Release().Get(grpcKit, req.BizId, req.AppId, req.ReleaseId)
	if err != nil {
		return nil, err
	}
	if release.Spec.Deprecated {
		return nil, errors.New(i18n.T(grpcKit, "release %s is deprecated, can not be revoke", release.Spec.Name))
	}

	strategy, err := s.dao.Strategy().GetLast(grpcKit, req.BizId, req.AppId, req.ReleaseId, 0)
	if err != nil {
		return nil, err
	}

	switch req.Action {
	// 同意和拒绝
	case "approve", "refuse":
		err = s.itsm.ApprovalTicket(grpcKit.Ctx, api.ApprovalTicketReq{
			TicketID:     strategy.Spec.ItsmTicketSn,
			TaskID:       strconv.Itoa(strategy.Spec.ItsmTicketStateID),
			Operator:     grpcKit.TenantID,
			OperatorType: grpcKit.User,
			Action:       req.Action,
			Desc:         req.Reason,
		})
		if err != nil {
			return nil, err
		}
	// 撤单
	case "revoked":
		resp, errR := s.itsm.RevokedTicket(grpcKit.Ctx, api.ApprovalTicketReq{
			TicketID: strategy.Spec.ItsmTicketSn,
			SystemID: cc.DataService().ITSM.SystemId,
		})
		if errR != nil {
			return nil, errR
		}
		if !resp.Result {
			return nil, fmt.Errorf("单据撤回失败")
		}
	}

	return &pbds.SubmitApprovalResp{
		Message: "单据审核中",
	}, nil
}
