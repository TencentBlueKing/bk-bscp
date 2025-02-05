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

package dao

import (
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/orm"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/sharding"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/enumor"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// AuditDao supplies all the audit operations.
type AuditDao interface {
	// Decorator is used to handle the audit process as a pipeline
	// according CUD scenarios.
	Decorator(kit *kit.Kit, bizID uint32, a *table.AuditField) AuditPrepare
	// One insert one resource's audit.
	One(kit *kit.Kit, audit *table.Audit, opt *AuditOption) error
	// ListAuditsAppStrategy List audit apo strategy.
	ListAuditsAppStrategy(
		kit *kit.Kit, req *pbds.ListAuditsReq) ([]*types.ListAuditsAppStrategy, int64, error)
	// UpdateByStrategyID update audit kv by strategyID.
	UpdateByStrategyID(kit *kit.Kit, tx *gen.QueryTx, strategyID uint32, m map[string]interface{}) error
	// UpdateByStrategyIDs update audit kv by strategyIDs.
	UpdateByStrategyIDs(
		kit *kit.Kit, tx *gen.QueryTx, strategyID []uint32, m map[string]interface{}) error
}

// AuditOption defines all the needed infos to audit a resource.
type AuditOption struct {
	// resource's transaction infos.
	Txn *sqlx.Tx
	// ResShardingUid is the resource's sharding instance.
	ResShardingUid string
	genQ           *gen.Query
}

var _ AuditDao = new(audit)

// NewAuditDao create the audit DAO
func NewAuditDao(db *gorm.DB, orm orm.Interface, sd *sharding.Sharding, idGen IDGenInterface) (AuditDao, error) {
	return &audit{
		db:         db,
		genQ:       gen.Use(db),
		orm:        orm,
		sd:         sd,
		adSharding: sd.Audit(),
		idGen:      idGen,
	}, nil
}

type audit struct {
	db   *gorm.DB
	genQ *gen.Query
	orm  orm.Interface
	// sd is the common resource's sharding manager.
	sd *sharding.Sharding
	// adSharding is the audit's sharding instance
	adSharding *sharding.One
	idGen      IDGenInterface
}

// Decorator return audit decorator for to record audit.
func (au *audit) Decorator(kit *kit.Kit, bizID uint32, a *table.AuditField) AuditPrepare {
	return initAuditBuilder(kit, bizID, a, au)
}

// One audit one resource's operation.
func (au *audit) One(kit *kit.Kit, audit *table.Audit, opt *AuditOption) error {
	if audit == nil || opt == nil {
		return errors.New("invalid input audit or opt")
	}

	// generate an audit id and update to audit.
	id, err := au.idGen.One(kit, table.AuditTable)
	if err != nil {
		return err
	}

	audit.ID = id

	var q gen.IAuditDo

	if opt.genQ != nil && au.db.Migrator().CurrentDatabase() == opt.genQ.CurrentDatabase() {
		// 使用同一个库，事务处理
		q = opt.genQ.Audit.WithContext(kit.Ctx)
	} else {
		// 使用独立的 DB
		q = au.genQ.Audit.WithContext(kit.Ctx)
	}

	if err := q.Create(audit); err != nil {
		return fmt.Errorf("insert audit failed, err: %v", err)
	}
	return nil
}

// ListAuditsAppStrategy List audit apo strategy.
func (au *audit) ListAuditsAppStrategy(
	kit *kit.Kit, req *pbds.ListAuditsReq) ([]*types.ListAuditsAppStrategy, int64, error) {
	var publishs []*types.ListAuditsAppStrategy
	var noPublishs []*types.ListAuditsAppStrategy

	audit := au.genQ.Audit

	query, err := au.createQuery(kit, req)
	if err != nil {
		return nil, 0, err
	}

	// priority display publish version config
	publishCount, err := query.Where(audit.Action.Eq(string(enumor.Publish))).
		Order(audit.CreatedAt.Desc()).
		ScanByPage(&publishs, int(req.Start), int(req.Limit))
	if err != nil {
		return nil, 0, err
	}

	// 非上线版本配置条数开始索引位置
	var residueOffset uint32
	if req.Start > uint32(publishCount) {
		residueOffset = req.Start - uint32(publishCount)
	}

	query2, err := au.createQuery(kit, req)
	if err != nil {
		return nil, 0, err
	}
	noPublishCount, err := query2.Not(audit.Action.Eq(string(enumor.Publish))).
		Order(audit.CreatedAt.Desc()).
		ScanByPage(&noPublishs, int(residueOffset), int(req.Limit)-len(publishs))
	if err != nil {
		return nil, 0, err
	}

	publishs = append(publishs, noPublishs...)
	return publishs, publishCount + noPublishCount, nil
}

// createQuery create same query
// nolint funlen
func (au *audit) createQuery(kit *kit.Kit, req *pbds.ListAuditsReq) (gen.IAuditDo, error) {
	audit := au.genQ.Audit
	app := au.genQ.App
	strategy := au.genQ.Strategy
	client := au.genQ.Client

	// 后续改造中去掉audit.ResourceType.In，现在加上为了适配原来的数据
	result := audit.WithContext(kit.Ctx).Select(audit.ID, audit.ResourceType, audit.ResourceID, audit.Action,
		audit.BizID, audit.AppID, audit.Operator, audit.CreatedAt, audit.ResInstance, audit.OperateWay, audit.Status,
		audit.IsCompare, audit.Detail,
		app.Name, app.Creator, client.ReleaseChangeStatus, client.FailedDetailReason,
		strategy.PublishType, strategy.PublishTime, strategy.PublishTime, strategy.FinalApprovalTime,
		strategy.PublishStatus, strategy.RejectReason, strategy.Approver, strategy.ApproverProgress,
		strategy.UpdatedAt, strategy.Reviser, strategy.Creator, strategy.ReleaseID, strategy.Scope,
		strategy.ItsmTicketSn, strategy.ItsmTicketUrl, strategy.ItsmTicketStateID, strategy.ItsmTicketStatus,
		strategy.ItsmTicketType, strategy.ApproveType, strategy.Memo).
		LeftJoin(app, app.ID.EqCol(audit.AppID)).
		LeftJoin(strategy, strategy.ID.EqCol(audit.StrategyId)).
		LeftJoin(client, audit.ResourceID.EqCol(client.ID), audit.ResourceType.Eq(string(enumor.Instance))).
		Where(audit.BizID.Eq(req.BizId), audit.ResourceType.In(string(enumor.App), string(enumor.Config),
			string(enumor.Hook), string(enumor.Release), string(enumor.Group),
			string(enumor.Template), string(enumor.Credential), string(enumor.Instance), string(enumor.Variable)))

	if req.Id != 0 {
		result = result.Where(audit.ID.Eq(req.Id))
	}

	// if not query all app, need current app_id
	if !req.All {
		result = result.Where(audit.AppID.Eq(req.AppId))
	}

	if req.StartTime != "" {
		startTime, err := time.Parse(time.DateTime, req.StartTime)
		if err != nil {
			return nil, err
		}
		result = result.Where(audit.CreatedAt.Gte(startTime))
	}

	if req.EndTime != "" {
		endTime, err := time.Parse(time.DateTime, req.EndTime)
		if err != nil {
			return nil, err
		}
		// database has milliseconds left, take the upper limit
		endTime = endTime.Add(time.Second)
		result = result.Where(audit.CreatedAt.Lt(endTime))
	}

	if req.Name != "" {
		result = result.Where(app.Name.Like("%" + req.Name + "%"))
	}

	if len(req.ResourceType) != 0 {
		result = result.Where(audit.ResourceType.In(req.ResourceType...))
	}

	if len(req.Action) != 0 {
		result = result.Where(audit.Action.In(req.Action...))
	}

	if req.ResInstance != "" {
		result = result.Where(audit.ResInstance.Like("%" + req.ResInstance + "%"))
	}

	if len(req.Status) != 0 {
		auditStatus := audit.WithContext(kit.Ctx).Where(audit.Status.In(req.Status...))
		// 失败状态的数据需要特殊处理，在clients表
		for _, v := range req.Status {
			if v == string(enumor.Failure) {
				auditStatus.Or(client.ReleaseChangeStatus.Eq(string(table.Failed)))
				// 前端可能传两个failure过来
				break
			}
		}
		result = result.Where(auditStatus)
	}

	if req.Operator != "" {
		result = result.Where(audit.Operator.Like("%" + req.Operator + "%"))
	}

	if len(req.OperateWay) != 0 {
		result = result.Where(audit.OperateWay.In(req.OperateWay...))
	}

	return result, nil
}

// UpdateByStrategyID update audit kv by strategyID.
func (au *audit) UpdateByStrategyID(kit *kit.Kit, tx *gen.QueryTx, strategyID uint32, m map[string]interface{}) error {
	s := tx.Audit
	_, err := s.WithContext(kit.Ctx).Where(s.StrategyId.Eq(strategyID)).Updates(m)
	return err
}

// UpdateByStrategyIDs update audit kv by strategyIDs.
func (au *audit) UpdateByStrategyIDs(
	kit *kit.Kit, tx *gen.QueryTx, strategyIDs []uint32, m map[string]interface{}) error {
	if len(strategyIDs) == 0 {
		return nil
	}
	s := tx.Audit
	_, err := s.WithContext(kit.Ctx).Where(s.StrategyId.In(strategyIDs...)).Updates(m)
	return err
}
