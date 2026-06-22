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
	"database/sql"
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

// Project supplies all the project related operations.
type Project interface {
	// Create one project instance
	Create(kit *kit.Kit, project *table.Project) (uint32, error)
	// Delete one project instance
	Delete(kit *kit.Kit, project *table.Project) error
	// Update one project's info
	Update(kit *kit.Kit, project *table.Project) error
	// Get get project with id.
	Get(kit *kit.Kit, bizID, projectID uint32) (*table.Project, error)
	// List projects with options.
	List(kit *kit.Kit, bizID uint32, opt *types.BasePage) ([]*table.Project, int64, error)
	// GetDefaultProject 获取系统创建的默认项目
	GetDefaultProject(kit *kit.Kit, bizID uint32) (*table.Project, error)
	// CreateWithTx create one project instance with transaction.
	CreateWithTx(kit *kit.Kit, tx *gen.QueryTx, project *table.Project) (uint32, error)
	CreateIfNotExistWithTx(kit *kit.Kit, tx *gen.QueryTx, project *table.Project) error
}

var _ Project = new(projectDao)

type projectDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
	event    Event
}

// CreateIfNotExistWithTx implements [Project].
func (dao *projectDao) CreateIfNotExistWithTx(kit *kit.Kit, tx *gen.QueryTx, project *table.Project) error {
	if project == nil {
		return errf.Errorf(errf.InvalidArgument, "%s", i18n.T(kit, "project is nil"))
	}

	// 1. 先进行基础的创建校验（避免校验失败却白白生成了 ID）
	if err := project.ValidateCreate(kit); err != nil {
		return err
	}

	// 2. 生成全局唯一 ID
	id, err := dao.idGen.One(kit, table.Name(project.TableName()))
	if err != nil {
		return err
	}

	project.ID = id
	project.Spec.Key = table.GenerateProjectKey(id)

	// 3. 执行带冲突处理的创建
	q := tx.Project.WithContext(kit.Ctx)
	return q.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"},
			{Name: "biz_id"},
			{Name: "is_default"},
		},
		// 发生冲突时，只更新时间戳
		DoUpdates: clause.AssignmentColumns([]string{"updated_at"}),
	}).Create(project)
}

// CreateWithTx implements [Project].
func (dao *projectDao) CreateWithTx(kit *kit.Kit, tx *gen.QueryTx, g *table.Project) (uint32, error) {
	if g == nil {
		return 0, errf.Errorf(errf.InvalidArgument, "%s", i18n.T(kit, "project is nil"))
	}

	// generate a project id and update to g.
	id, err := dao.idGen.One(kit, table.Name(g.TableName()))
	if err != nil {
		return 0, err
	}
	g.ID = id
	if g.Spec.Key == "" {
		g.Spec.Key = table.GenerateProjectKey(id)
	}

	if err := g.ValidateCreate(kit); err != nil {
		return 0, err
	}

	q := tx.Project.WithContext(kit.Ctx)
	if e := q.Create(g); e != nil {
		return 0, e
	}

	ad := dao.auditDao.Decorator(kit, g.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.ProjectName, g.Spec.Name),
		Status:           enumor.Success,
		Detail:           g.Spec.Memo,
	}).PrepareCreate(g)
	if e := ad.Do(tx.Query); e != nil {
		return 0, e
	}

	return g.ID, nil
}

// GetDefaultProject 根据业务ID、创建人(system)获取系统创建的默认项目
func (dao *projectDao) GetDefaultProject(kit *kit.Kit, bizID uint32) (*table.Project, error) {
	m := dao.genQ.Project
	q := dao.genQ.Project.WithContext(kit.Ctx)
	target := sql.NullBool{Bool: true, Valid: true}

	return q.Where(m.BizID.Eq(bizID), m.IsDefault.Eq(target)).Take()

}

// Delete implements [Project].
func (dao *projectDao) Delete(kit *kit.Kit, project *table.Project) error {
	// 参数校验
	if err := project.ValidateDelete(kit); err != nil {
		return err
	}

	// 删除操作, 获取当前记录做审计
	m := dao.genQ.Project
	q := dao.genQ.Project.WithContext(kit.Ctx)
	oldOne, err := q.Where(m.ID.Eq(project.ID), m.BizID.Eq(project.Attachment.BizID)).Take()
	if err != nil {
		return err
	}
	ad := dao.auditDao.Decorator(kit, project.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.ConfigItemName, oldOne.Spec.Name),
		Status:           enumor.Success,
		Detail:           oldOne.Spec.Memo,
	}).PrepareDelete(oldOne)

	// 多个使用事务处理
	deleteTx := func(tx *gen.Query) error {
		q = tx.Project.WithContext(kit.Ctx)
		if _, e := q.Where(m.BizID.Eq(project.Attachment.BizID), m.ID.Eq(project.ID)).Delete(project); e != nil {
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

// List projects's detail info with the filter's expression.
func (dao *projectDao) List(kit *kit.Kit, bizID uint32, opt *types.BasePage) ([]*table.Project, int64, error) {
	m := dao.genQ.Project
	q := dao.genQ.Project.WithContext(kit.Ctx)
	var (
		conds  []rawgen.Condition
		result []*table.Project
		count  int64
		err    error
	)

	conds = append(conds, m.BizID.Eq(bizID))
	conds = dao.handleSearch(conds, opt.Search.AsMap())
	q = q.Order(m.ID.Desc())
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

// 支持名称、Key、描述、更新人、创建人搜索
func (dao *projectDao) handleSearch(conds []rawgen.Condition, search map[string]any) []rawgen.Condition {
	if len(search) == 0 {
		return conds
	}
	m := dao.genQ.Project

	if search["name"] != nil {
		name, _ := search["name"].(string)
		conds = append(conds, m.Name.Like("%"+name+"%"))
	}

	if search["key"] != nil {
		key, _ := search["key"].(string)
		conds = append(conds, m.Key.Like("%"+key+"%"))
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

// Create one project instance
func (dao *projectDao) Create(kit *kit.Kit, g *table.Project) (uint32, error) {
	if g == nil {
		return 0, errf.Errorf(errf.InvalidArgument, "%s", i18n.T(kit, "project is nil"))
	}

	// generate a project id and update to g.
	id, err := dao.idGen.One(kit, table.Name(g.TableName()))
	if err != nil {
		return 0, err
	}
	g.ID = id
	g.Spec.Key = table.GenerateProjectKey(id)

	if err = g.ValidateCreate(kit); err != nil {
		return 0, err
	}

	ad := dao.auditDao.Decorator(kit, g.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.ProjectName, g.Spec.Name),
		Status:           enumor.Success,
		Detail:           g.Spec.Memo,
	}).PrepareCreate(g)

	msg := i18n.T(kit, "project creation failed")

	createTx := func(tx *gen.Query) error {
		q := tx.Project.WithContext(kit.Ctx)
		if err = q.Create(g); err != nil {
			return errf.Errorf(
				errf.DBOpFailed,
				"%s: %v",
				msg,
				err,
			)
		}

		if err = ad.Do(tx); err != nil {
			logs.Errorf("execution of transactions failed, err: %v", err)
			return errf.Errorf(
				errf.DBOpFailed,
				"%s: %v",
				msg,
				err,
			)
		}

		return nil
	}

	if err = dao.genQ.Transaction(createTx); err != nil {
		logs.Errorf("transaction processing failed %s", err)
		return 0, errf.Errorf(
			errf.DBOpFailed,
			"%s: %v",
			msg,
			err,
		)
	}

	return id, nil
}

// Update an project instance.
func (dao *projectDao) Update(kit *kit.Kit, g *table.Project) error {
	if g == nil {
		return errf.Errorf(errf.InvalidArgument, "%s", i18n.T(kit, "project is nil"))
	}

	if err := g.ValidateUpdate(kit); err != nil {
		return err
	}

	// 更新操作, 获取当前记录做审计
	m := dao.genQ.Project
	q := dao.genQ.Project.WithContext(kit.Ctx)
	ad := dao.auditDao.Decorator(kit, g.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.ProjectName, g.Spec.Name),
		Status:           enumor.Success,
		Detail:           g.Spec.Memo,
	}).PrepareUpdate(g)

	msg := i18n.T(kit, "project update failed")

	updateTx := func(tx *gen.Query) error {
		q = tx.Project.WithContext(kit.Ctx)
		if _, err := q.Where(m.BizID.Eq(g.Attachment.BizID), m.ID.Eq(g.ID)).Updates(g); err != nil {
			return errf.Errorf(
				errf.DBOpFailed,
				"%s: %v",
				msg,
				err,
			)
		}

		if err := ad.Do(tx); err != nil {
			return errf.Errorf(
				errf.DBOpFailed,
				"%s: %v",
				msg,
				err,
			)
		}

		return nil
	}

	if err := dao.genQ.Transaction(updateTx); err != nil {
		return errf.Errorf(
			errf.DBOpFailed,
			"%s: %v",
			msg,
			err,
		)
	}

	return nil
}

// Get 获取单个project详情
func (dao *projectDao) Get(kit *kit.Kit, bizID uint32, projectID uint32) (*table.Project, error) {
	m := dao.genQ.Project
	q := dao.genQ.Project.WithContext(kit.Ctx)
	detail, err := q.Where(m.ID.Eq(projectID), m.BizID.Eq(bizID)).Take()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errf.Errorf(errf.RecordNotFound, "%s", i18n.T(kit, "project does not exist"))
		}
		return nil, errf.Errorf(errf.DBOpFailed, "%s: %v", i18n.T(kit, "project query failed"), err)
	}
	return detail, nil
}
