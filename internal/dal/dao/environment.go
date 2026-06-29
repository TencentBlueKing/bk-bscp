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

	rawgen "gorm.io/gen"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
	// CountByProjectID 统计单个项目下的环境数量
	CountByProjectID(kit *kit.Kit, projectID uint32) (int64, error)
	// CountByProjectIDs 批量统计项目下的服务数量
	CountByProjectIDs(kit *kit.Kit, projectIDs []uint32) (map[uint32]uint32, error)
	// GetDefaultEnvironment 获取系统创建的默认环境
	GetDefaultEnvironment(kit *kit.Kit, bizID, projectID uint32) (*table.Environment, error)
	// CreateWithTx create one environments instance with transaction.
	CreateWithTx(kit *kit.Kit, tx *gen.QueryTx, environments *table.Environment) (uint32, error)
	CreateIfNotExistWithTx(kit *kit.Kit, tx *gen.QueryTx, environments *table.Environment) error
	// DeleteByProjectIDWithTx  delete environments by project id with transaction.
	DeleteByProjectIDWithTx(kit *kit.Kit, tx *gen.QueryTx, bizID, projectID uint32) error
}

var _ Environment = new(environmentDao)

type environmentDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
	event    Event
}

// DeleteByProjectIDWithTx implements [Environment].
func (dao *environmentDao) DeleteByProjectIDWithTx(kit *kit.Kit, tx *gen.QueryTx, bizID uint32, projectID uint32) error {
	m := tx.Environment
	q := tx.Environment.WithContext(kit.Ctx)

	if _, e := q.Where(m.BizID.Eq(bizID), m.ProjectID.Eq(projectID)).Delete(); e != nil {
		return e
	}

	return nil
}

// CountByProjectIDs 批量统计项目下的环境数量
func (dao *environmentDao) CountByProjectIDs(kit *kit.Kit, projectIDs []uint32) (map[uint32]uint32, error) {
	resMap := make(map[uint32]uint32)
	if len(projectIDs) == 0 {
		return resMap, nil
	}

	m := dao.genQ.Environment
	q := dao.genQ.Environment.WithContext(kit.Ctx)

	type Result struct {
		ProjectID uint32 `gorm:"column:project_id"`
		Count     uint32 `gorm:"column:cnt"`
	}
	var results []Result

	err := q.Select(m.ProjectID, m.ID.Count().As("cnt")).
		Where(m.ProjectID.In(projectIDs...)).
		Group(m.ProjectID).
		Scan(&results)

	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, "%s: %v", i18n.T(kit, "environment count failed"), err)
	}

	for _, r := range results {
		resMap[r.ProjectID] = r.Count
	}
	return resMap, nil
}

// CountByProjectID 统计单个项目下的环境数量
func (dao *environmentDao) CountByProjectID(kit *kit.Kit, projectID uint32) (int64, error) {
	m := dao.genQ.Environment

	count, err := dao.genQ.Environment.WithContext(kit.Ctx).Where(m.ProjectID.Eq(projectID)).Count()
	if err != nil {
		return 0, errf.Errorf(errf.DBOpFailed, "%s: %v", i18n.T(kit, "environment count failed"), err)
	}

	return count, nil
}

// CreateIfNotExistWithTx implements [Environment].
func (dao *environmentDao) CreateIfNotExistWithTx(kit *kit.Kit, tx *gen.QueryTx, env *table.Environment) error {
	if env == nil {
		return errf.Errorf(errf.InvalidArgument, "%s", i18n.T(kit, "environment is nil"))
	}

	// 1. 先校验合法性
	if err := env.ValidateCreate(kit); err != nil {
		return err
	}

	// 2. 校验通过后再生成 ID
	id, err := dao.idGen.One(kit, table.Name(env.TableName()))
	if err != nil {
		return err
	}
	env.ID = id

	// 3. 执行带冲突处理的创建
	q := tx.Environment.WithContext(kit.Ctx)
	return q.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"},
			{Name: "biz_id"},
			{Name: "project_id"},
			{Name: "name"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"updated_at"}),
	}).Create(env)
}

// CreateWithTx implements [Environment].
func (dao *environmentDao) CreateWithTx(kit *kit.Kit, tx *gen.QueryTx, g *table.Environment) (uint32, error) {
	if g == nil {
		return 0, errf.Errorf(errf.InvalidArgument, "%s", i18n.T(kit, "environment is nil"))
	}

	if err := g.ValidateCreate(kit); err != nil {
		return 0, err
	}

	// generate a project id and update to g.
	id, err := dao.idGen.One(kit, table.Name(g.TableName()))
	if err != nil {
		return 0, err
	}
	g.ID = id

	q := tx.Environment.WithContext(kit.Ctx)
	if e := q.Create(g); e != nil {
		return 0, e
	}

	ad := dao.auditDao.Decorator(kit, g.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.EnvName, g.Spec.Name),
		Status:           enumor.Success,
		Detail:           g.Spec.Memo,
	}).PrepareCreate(g)
	if e := ad.Do(tx.Query); e != nil {
		return 0, e
	}

	return g.ID, nil
}

// GetDefaultEnvironment implements [Environment].
func (dao *environmentDao) GetDefaultEnvironment(kit *kit.Kit, bizID uint32, projectID uint32) (
	*table.Environment, error) {

	m := dao.genQ.Environment
	q := dao.genQ.Environment.WithContext(kit.Ctx)

	return q.Where(m.BizID.Eq(bizID), m.ProjectID.Eq(projectID), m.Creator.Eq(table.System)).Take()
}

// Delete implements [Environment].
func (dao *environmentDao) Delete(kit *kit.Kit, env *table.Environment) error {
	if env == nil {
		return errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kit, "environment is nil"))
	}

	if err := env.ValidateDelete(kit); err != nil {
		return err
	}

	msg := i18n.T(kit, "environment deletion failed")

	m := dao.genQ.Environment
	q := dao.genQ.Environment.WithContext(kit.Ctx)
	oldOne, err := q.Where(m.ID.Eq(env.ID), m.BizID.Eq(env.Attachment.BizID), m.ProjectID.Eq(env.Attachment.ProjectID)).Take()
	if err != nil {
		return errf.Errorf(errf.DBOpFailed, "%s: %v", msg, err)
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
			return errf.Errorf(errf.DBOpFailed, "%s: %v", msg, err)
		}

		if e := ad.Do(tx); e != nil {
			return errf.Errorf(errf.DBOpFailed, "%s: %v", msg, err)
		}
		return nil
	}
	if e := dao.genQ.Transaction(deleteTx); e != nil {
		return errf.Errorf(errf.DBOpFailed, "%s: %v", msg, err)
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
		ResourceInstance: fmt.Sprintf(constant.EnvName, g.Spec.Name),
		Status:           enumor.Success,
		Detail:           g.Spec.Memo,
	}).PrepareCreate(g)

	msg := i18n.T(kit, "environment creation failed")

	createTx := func(tx *gen.Query) error {
		q := tx.Environment.WithContext(kit.Ctx)
		if err = q.Create(g); err != nil {
			return errf.Errorf(errf.DBOpFailed, "%s: %v", msg, err)
		}

		if err = ad.Do(tx); err != nil {
			logs.Errorf("execution of transactions failed, err: %v", err)
			return errf.Errorf(errf.DBOpFailed, "%s: %v", msg, err)
		}

		return nil
	}

	if err = dao.genQ.Transaction(createTx); err != nil {
		return 0, errf.Errorf(errf.DBOpFailed, "%s: %v", msg, err)
	}

	return id, nil
}

// Update an environment instance.
func (dao *environmentDao) Update(kit *kit.Kit, g *table.Environment) error {
	if g == nil {
		return errf.Errorf(errf.InvalidArgument, "%s", i18n.T(kit, "environment is nil"))
	}

	if err := g.ValidateUpdate(kit); err != nil {
		return err
	}

	// 更新操作, 获取当前记录做审计
	m := dao.genQ.Environment
	q := dao.genQ.Environment.WithContext(kit.Ctx)

	ad := dao.auditDao.Decorator(kit, g.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.EnvName, g.Spec.Name),
		Status:           enumor.Success,
		Detail:           g.Spec.Memo,
	}).PrepareUpdate(g)

	msg := i18n.T(kit, "environment update failed")

	updateTx := func(tx *gen.Query) error {
		q = tx.Environment.WithContext(kit.Ctx)
		if _, err := q.Where(m.BizID.Eq(g.Attachment.BizID), m.ID.Eq(g.ID)).Updates(g); err != nil {
			return errf.Errorf(errf.DBOpFailed, "%s: %v", msg, err)
		}

		if err := ad.Do(tx); err != nil {
			return errf.Errorf(errf.DBOpFailed, "%s: %v", msg, err)
		}
		return nil
	}
	if err := dao.genQ.Transaction(updateTx); err != nil {
		return errf.Errorf(errf.DBOpFailed, "%s: %v", msg, err)
	}

	return nil
}

// Get 获取单个environment详情
func (dao *environmentDao) Get(kit *kit.Kit, bizID, projectID, envID uint32) (*table.Environment, error) {
	m := dao.genQ.Environment
	q := dao.genQ.Environment.WithContext(kit.Ctx)
	detail, err := q.Where(m.ID.Eq(envID), m.BizID.Eq(bizID), m.ProjectID.Eq(projectID)).Take()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errf.Errorf(errf.RecordNotFound, "%s", i18n.T(kit, "environment does not exist"))
		}
		return nil, errf.Errorf(errf.DBOpFailed, "%s: %v", i18n.T(kit, "environment query failed"), err)
	}
	return detail, nil
}

// GetByName 通过 EnvironmentId、name 查询
func (dao *environmentDao) GetByName(kit *kit.Kit, bizID, projectID uint32, name string) (*table.Environment, error) {
	m := dao.genQ.Environment
	q := dao.genQ.Environment.WithContext(kit.Ctx)

	return q.Where(m.BizID.Eq(bizID), m.ProjectID.Eq(projectID), m.Name.Eq(name)).Take()
}
