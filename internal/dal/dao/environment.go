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
	"fmt"

	rawgen "gorm.io/gen"

	"github.com/TencentBlueKing/bk-bscp/internal/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/enumor"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// Environment supplies all the environment related operations.
type Environment interface {
	// Create one environment instance
	Create(kit *kit.Kit, env *table.Environment) (uint32, error)
	// Update one environment's info
	Update(kit *kit.Kit, env *table.Environment) error
	// Delete one environment instance
	Delete(kit *kit.Kit, env *table.Environment) error
	// Get get environment with id.
	Get(kit *kit.Kit, bizID, projectID, envID uint32) (*table.Environment, error)
	// GetByName get environment only with id、name.
	GetByName(kit *kit.Kit, bizID, projectID uint32, name string) (*table.Environment, error)
	// List environments with options.
	List(kit *kit.Kit, bizID, projectID uint32, opt *types.BasePage) ([]*table.Environment, int64, error)
}

var _ Environment = new(environmentDao)

type environmentDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
	event    Event
}

// Delete implements [Environment].
func (dao *environmentDao) Delete(kit *kit.Kit, env *table.Environment) error {
	// 参数校验
	if err := env.ValidateDelete(kit); err != nil {
		return err
	}

	// 删除操作, 获取当前记录做审计
	m := dao.genQ.Environment
	q := dao.genQ.Environment.WithContext(kit.Ctx)
	oldOne, err := q.Where(m.ID.Eq(env.ID), m.BizID.Eq(env.Attachment.BizID), m.ProjectID.Eq(env.Attachment.ProjectID)).Take()
	if err != nil {
		return err
	}
	ad := dao.auditDao.Decorator(kit, env.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.ConfigItemName, oldOne.Spec.Name),
		Status:           enumor.Success,
		Detail:           oldOne.Spec.Memo,
	}).PrepareDelete(oldOne)

	// 多个使用事务处理
	deleteTx := func(tx *gen.Query) error {
		q = tx.Environment.WithContext(kit.Ctx)
		if _, e := q.Where(m.BizID.Eq(env.Attachment.BizID), m.ID.Eq(env.ID)).Delete(env); e != nil {
			return e
		}

		if e := ad.Do(tx); e != nil {
			return e
		}
		return nil
	}
	if e := dao.genQ.Transaction(deleteTx); e != nil {
		return e
	}

	return nil
}

// List environments's detail info with the filter's expression.
func (dao *environmentDao) List(kit *kit.Kit, bizID, projectID uint32, opt *types.BasePage) (
	[]*table.Environment, int64, error) {
	m := dao.genQ.Environment
	q := dao.genQ.Environment.WithContext(kit.Ctx)
	var (
		conds  []rawgen.Condition
		result []*table.Environment
		count  int64
		err    error
	)

	if bizID > 0 {
		conds = append(conds, m.BizID.Eq(bizID))
	}

	if projectID > 0 {
		conds = append(conds, m.ProjectID.Eq(projectID))
	}

	conds = dao.handleSearch(conds, opt.Search.AsMap())

	q = q.Order(m.DisplayOrder.Desc())

	if opt.All {
		result, err = q.Where(conds...).Find()
		count = int64(len(result))
	} else {
		result, count, err = q.Where(conds...).FindByPage(opt.Offset(), opt.LimitInt())
	}
	if err != nil {
		return nil, 0, err
	}

	return result, count, nil
}

// 支持名称、类型、描述、更新人、创建人搜索
func (dao *environmentDao) handleSearch(conds []rawgen.Condition, search map[string]any) []rawgen.Condition {
	if len(search) == 0 {
		return conds
	}
	m := dao.genQ.Environment

	if search["name"] != nil {
		name, _ := search["name"].(string)
		conds = append(conds, m.Name.Like("%"+name+"%"))
	}

	if search["type"] != nil {
		envType, _ := search["type"].(string)
		conds = append(conds, m.Type.Eq(envType))
	}

	if search["memo"] != nil {
		memo, _ := search["memo"].(string)
		conds = append(conds, m.Memo.Like("%"+memo+"%"))
	}

	if search["creator"] != nil {
		creator, _ := search["creator"].(string)
		conds = append(conds, m.Creator.Like("%"+creator+"%"))
	}

	if search["reviser"] != nil {
		reviser, _ := search["reviser"].(string)
		conds = append(conds, m.Reviser.Like("%"+reviser+"%"))
	}

	return conds
}

// Create one environment instance
func (dao *environmentDao) Create(kit *kit.Kit, g *table.Environment) (uint32, error) {
	if g == nil {
		return 0, errf.Errorf(errf.InvalidArgument, "%s", i18n.T(kit, "environment is nil"))
	}

	if err := g.ValidateCreate(kit); err != nil {
		return 0, err
	}

	// generate an environment id and update to g.
	id, err := dao.idGen.One(kit, table.Name(g.TableName()))
	if err != nil {
		return 0, err
	}
	g.ID = id

	ad := dao.auditDao.Decorator(kit, g.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.AppName, g.Spec.Name),
		Status:           enumor.Success,
		Detail:           g.Spec.Memo,
	}).PrepareCreate(g)
	eDecorator := dao.event.Eventf(kit)

	// 多个使用事务处理
	createTx := func(tx *gen.Query) error {
		q := tx.Environment.WithContext(kit.Ctx)
		if err = q.Create(g); err != nil {
			return errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kit, "create data failed, err: %v", err))
		}

		if err = ad.Do(tx); err != nil {
			logs.Errorf("execution of transactions failed, err: %v", err)
			return errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kit, "create environment failed, err: %v", err))
		}

		// fire the event with txn to ensure the if save the event failed then the business logic is failed anyway.
		one := types.Event{
			Spec: &table.EventSpec{
				Resource:   table.Application,
				ResourceID: g.ID,
				OpType:     table.InsertOp,
			},
			Attachment: &table.EventAttachment{BizID: g.Attachment.BizID},
			Revision:   &table.CreatedRevision{Creator: kit.User},
		}
		if err = eDecorator.Fire(one); err != nil {
			logs.Errorf("fire create environment: %s event failed, err: %v, rid: %s", g.ID, err, kit.Rid)
			return errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kit, "create environment failed, err: %v", err))
		}

		return nil
	}
	err = dao.genQ.Transaction(createTx)

	eDecorator.Finalizer(err)

	if err != nil {
		logs.Errorf("transaction processing failed %s", err)
		return 0, errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kit, "create environment failed, err: %v", err))
	}

	return id, nil
}

// Update an environment instance.
func (dao *environmentDao) Update(kit *kit.Kit, g *table.Environment) error {
	if g == nil {
		return errf.Errorf(errf.InvalidArgument, "%s", i18n.T(kit, "environment is nil"))
	}

	_, err := dao.Get(kit, g.Attachment.BizID, g.Attachment.ProjectID, g.ID)
	if err != nil {
		return errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kit, "update environment failed, err: %s", err))
	}

	// 更新操作, 获取当前记录做审计
	m := dao.genQ.Environment
	q := dao.genQ.Environment.WithContext(kit.Ctx)
	ad := dao.auditDao.Decorator(kit, g.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.AppName, g.Spec.Name),
		Status:           enumor.Success,
		Detail:           g.Spec.Memo,
	}).PrepareUpdate(g)
	eDecorator := dao.event.Eventf(kit)

	// 多个使用事务处理
	updateTx := func(tx *gen.Query) error {
		q = tx.Environment.WithContext(kit.Ctx)
		if _, err = q.Where(m.BizID.Eq(g.Attachment.BizID), m.ProjectID.Eq(g.Attachment.ProjectID), m.ID.Eq(g.ID)).
			Select(m.Name, m.Type, m.Memo, m.DisplayOrder, m.Protected, m.Reviser, m.UpdatedAt).Updates(g); err != nil {
			return err
		}

		if err = ad.Do(tx); err != nil {
			return err
		}

		// fire the event with txn to ensure the if save the event failed then the business logic is failed anyway.
		one := types.Event{
			Spec: &table.EventSpec{
				Resource:   table.Application,
				ResourceID: g.ID,
				OpType:     table.UpdateOp,
			},
			Attachment: &table.EventAttachment{BizID: g.Attachment.BizID},
			Revision:   &table.CreatedRevision{Creator: kit.User},
		}
		if err = eDecorator.Fire(one); err != nil {
			logs.Errorf("fire update environment: %s event failed, err: %v, rid: %s", g.ID, err, kit.Rid)
			return errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kit, "update environment failed, err: %s", err))
		}
		return nil
	}
	err = dao.genQ.Transaction(updateTx)

	eDecorator.Finalizer(err)

	if err != nil {
		return err
	}

	return nil
}

// Get 获取单个environment详情
func (dao *environmentDao) Get(kit *kit.Kit, bizID, projectID, envID uint32) (*table.Environment, error) {
	m := dao.genQ.Environment
	q := dao.genQ.Environment.WithContext(kit.Ctx)
	detail, err := q.Where(m.ID.Eq(envID), m.BizID.Eq(bizID), m.ProjectID.Eq(projectID)).Take()
	if err != nil {
		return nil, err
	}
	return detail, nil
}

// GetByName 通过 EnvironmentId、name 查询
func (dao *environmentDao) GetByName(kit *kit.Kit, bizID, projectID uint32, name string) (*table.Environment, error) {
	m := dao.genQ.Environment
	q := dao.genQ.Environment.WithContext(kit.Ctx)

	env, err := q.Where(m.BizID.Eq(bizID), m.ProjectID.Eq(projectID), m.Name.Eq(name)).Take()
	if err != nil {
		return nil, err
	}

	return env, nil
}
