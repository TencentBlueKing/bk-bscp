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
	"github.com/TencentBlueKing/bk-bscp/internal/search"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/enumor"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// TemplateSpace supplies all the template space related operations.
type TemplateSpace interface {
	// Create one template space instance.
	Create(kit *kit.Kit, templateSpace *table.TemplateSpace) (uint32, error)
	// Update one template space's info.
	Update(kit *kit.Kit, templateSpace *table.TemplateSpace) error
	// List template spaces with options.
	List(kit *kit.Kit, bizID uint32, s search.Searcher, opt *types.BasePage) ([]*table.TemplateSpace, int64, error)
	// Delete one template space instance.
	Delete(kit *kit.Kit, templateSpace *table.TemplateSpace) error
	// GetByUniqueKey get template space by unique key.
	GetByUniqueKey(kit *kit.Kit, bizID uint32, name string) (*table.TemplateSpace, error)
	// GetAllBizs get all biz ids of template spaces.
	GetAllBizs(kit *kit.Kit) ([]uint32, error)
	// CreateDefault create default template space instance together with its default template set instance
	CreateDefault(kit *kit.Kit, bizID uint32) (uint32, error)
	// ListByIDs list template spaces by template space ids.
	ListByIDs(kit *kit.Kit, ids []uint32) ([]*table.TemplateSpace, error)
	// Get one template spaces by template space id.
	Get(kit *kit.Kit, bizID, id uint32) (*table.TemplateSpace, error)
}

var _ TemplateSpace = new(templateSpaceDao)

type templateSpaceDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
}

// Get implements TemplateSpace.
func (dao *templateSpaceDao) Get(kit *kit.Kit, bizID, id uint32) (*table.TemplateSpace, error) {
	m := dao.genQ.TemplateSpace

	return dao.genQ.TemplateSpace.WithContext(kit.Ctx).Where(m.ID.Eq(id), m.BizID.Eq(bizID)).Take()
}

// Create one template space instance.
// Every template space must have one default template set, so they should be created together.
func (dao *templateSpaceDao) Create(kit *kit.Kit, g *table.TemplateSpace) (uint32, error) {
	if err := g.ValidateCreate(kit); err != nil {
		return 0, err
	}

	tmplSpaceID, err := dao.idGen.One(kit, table.Name(g.TableName()))
	if err != nil {
		return 0, err
	}
	g.ID = tmplSpaceID

	sg := &table.TemplateSet{
		Spec: &table.TemplateSetSpec{
			Name:   constant.DefaultTmplSetName,
			Memo:   constant.DefaultTmplSetMemo,
			Public: true,
		},
		Attachment: &table.TemplateSetAttachment{
			BizID:           g.Attachment.BizID,
			TemplateSpaceID: g.ID,
		},
		Revision: &table.Revision{
			Creator: g.Revision.Creator,
			Reviser: g.Revision.Reviser,
		},
	}
	tmplSetID, err := dao.idGen.One(kit, table.Name(sg.TableName()))
	if err != nil {
		return 0, err
	}
	sg.ID = tmplSetID

	tmplSpaceAD := dao.auditDao.Decorator(kit, g.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.TemplateSpaceName, g.Spec.Name),
		Status:           enumor.Success,
		Detail:           g.Spec.Memo,
	}).PrepareCreate(g)
	tmplSetAD := dao.auditDao.Decorator(kit, sg.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.TemplateSpaceName+constant.ResSeparator+constant.TemplateSetName,
			g.Spec.Name, sg.Spec.Name),
		Status: enumor.Success,
		Detail: sg.Spec.Memo,
	}).PrepareCreate(sg)

	// 多个使用事务处理
	createTx := func(tx *gen.Query) error {
		if err := tx.TemplateSpace.WithContext(kit.Ctx).Create(g); err != nil {
			return err
		}

		// 连带创建模版空间下的默认套餐
		if err := tx.TemplateSet.WithContext(kit.Ctx).Create(sg); err != nil {
			return err
		}

		if err := tmplSpaceAD.Do(tx); err != nil {
			return err
		}
		if err := tmplSetAD.Do(tx); err != nil {
			return err
		}

		return nil
	}
	if err := dao.genQ.Transaction(createTx); err != nil {
		return 0, err
	}

	return g.ID, nil
}

// Update one template space instance.
func (dao *templateSpaceDao) Update(kit *kit.Kit, g *table.TemplateSpace) error {
	if err := g.ValidateUpdate(kit); err != nil {
		return err
	}

	// 更新操作, 获取当前记录做审计
	m := dao.genQ.TemplateSpace
	q := dao.genQ.TemplateSpace.WithContext(kit.Ctx)
	oldOne, err := q.Where(m.ID.Eq(g.ID), m.BizID.Eq(g.Attachment.BizID)).Take()
	if err != nil {
		return err
	}
	if oldOne.Spec.Name == constant.DefaultTmplSpaceCNName {
		return fmt.Errorf("can't update default template space")
	}
	ad := dao.auditDao.Decorator(kit, g.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.TemplateSpaceName, oldOne.Spec.Name), // 命名空间不更改名称
		Status:           enumor.Success,
		Detail:           g.Spec.Memo,
	}).PrepareUpdate(g)

	// 多个使用事务处理
	updateTx := func(tx *gen.Query) error {
		q = tx.TemplateSpace.WithContext(kit.Ctx)
		if _, err := q.Where(m.BizID.Eq(g.Attachment.BizID), m.ID.Eq(g.ID)).Select(m.Memo, m.Reviser).Updates(g); err != nil {
			return err
		}

		if err := ad.Do(tx); err != nil {
			return err
		}
		return nil
	}
	if err := dao.genQ.Transaction(updateTx); err != nil {
		return err
	}

	return nil
}

// List template spaces with options.
func (dao *templateSpaceDao) List(
	kit *kit.Kit, bizID uint32, s search.Searcher, opt *types.BasePage) ([]*table.TemplateSpace, int64, error) {
	m := dao.genQ.TemplateSpace
	q := dao.genQ.TemplateSpace.WithContext(kit.Ctx)

	var conds []rawgen.Condition
	// add search condition
	if s != nil {
		exprs := s.SearchExprs(dao.genQ)
		if len(exprs) > 0 {
			var do gen.ITemplateSpaceDo
			for i := range exprs {
				if i == 0 {
					do = q.Where(exprs[i])
				}
				do = do.Or(exprs[i])
			}
			conds = append(conds, do)
		}
	}

	d := q.Where(m.BizID.Eq(bizID)).Where(conds...).Order(m.Name)
	if opt.All {
		result, err := d.Find()
		if err != nil {
			return nil, 0, err
		}
		return result, int64(len(result)), err
	}

	return d.FindByPage(opt.Offset(), opt.LimitInt())

}

// Delete one template space instance.
func (dao *templateSpaceDao) Delete(kit *kit.Kit, g *table.TemplateSpace) error {
	// 参数校验
	if err := g.ValidateDelete(); err != nil {
		return err
	}

	// 删除操作, 获取当前记录做审计
	m := dao.genQ.TemplateSpace
	q := dao.genQ.TemplateSpace.WithContext(kit.Ctx)
	oldOne, err := q.Where(m.ID.Eq(g.ID), m.BizID.Eq(g.Attachment.BizID)).Take()
	if err != nil {
		return err
	}
	if oldOne.Spec.Name == constant.DefaultTmplSpaceCNName {
		return fmt.Errorf("can't delete default template space")
	}
	ad := dao.auditDao.Decorator(kit, g.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.TemplateSpaceName, oldOne.Spec.Name),
		Status:           enumor.Success,
		Detail:           oldOne.Spec.Memo,
	}).PrepareDelete(oldOne)

	// 多个使用事务处理
	deleteTx := func(tx *gen.Query) error {
		q = tx.TemplateSpace.WithContext(kit.Ctx)
		if _, err := q.Where(m.BizID.Eq(g.Attachment.BizID)).Delete(g); err != nil {
			return err
		}

		if err := ad.Do(tx); err != nil {
			return err
		}
		return nil
	}
	if err := dao.genQ.Transaction(deleteTx); err != nil {
		return err
	}

	return nil
}

// GetAllBizs get all bizs of template spaces.
func (dao *templateSpaceDao) GetAllBizs(kit *kit.Kit) ([]uint32, error) {
	m := dao.genQ.TemplateSpace
	q := dao.genQ.TemplateSpace.WithContext(kit.Ctx)
	var bizIDs []uint32

	if err := q.Distinct(m.BizID).Pluck(m.BizID, &bizIDs); err != nil {
		return nil, err
	}

	return bizIDs, nil
}

// CreateDefault create default template space instance together with its default template set instance
func (dao *templateSpaceDao) CreateDefault(kit *kit.Kit, bizID uint32) (uint32, error) {
	// if the default template space already exists, return directly
	if tmplSpace, err := dao.GetByUniqueKey(kit, bizID, constant.DefaultTmplSpaceCNName); err == nil {
		return tmplSpace.ID, nil
	}

	// use system user to create default template space
	kit.User = constant.BKSystemUser
	g := &table.TemplateSpace{
		ID: 0,
		Spec: &table.TemplateSpaceSpec{
			Name: constant.DefaultTmplSpaceCNName,
			Memo: constant.DefaultTmplSpaceMemo,
		},
		Attachment: &table.TemplateSpaceAttachment{
			BizID: kit.BizID,
		},
		Revision: &table.Revision{
			Creator: kit.User,
			Reviser: kit.User,
		},
	}
	tmplSpaceID, err := dao.idGen.One(kit, table.Name(g.TableName()))
	if err != nil {
		return 0, err
	}
	g.ID = tmplSpaceID

	sg := &table.TemplateSet{
		Spec: &table.TemplateSetSpec{
			Name:   constant.DefaultTmplSetName,
			Memo:   constant.DefaultTmplSetMemo,
			Public: true,
		},
		Attachment: &table.TemplateSetAttachment{
			BizID:           g.Attachment.BizID,
			TemplateSpaceID: g.ID,
		},
		Revision: &table.Revision{
			Creator: g.Revision.Creator,
			Reviser: g.Revision.Reviser,
		},
	}
	tmplSetID, err := dao.idGen.One(kit, table.Name(sg.TableName()))
	if err != nil {
		return 0, err
	}
	sg.ID = tmplSetID

	tmplSpaceAD := dao.auditDao.Decorator(kit, g.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.TemplateSpaceName, g.Spec.Name),
		Status:           enumor.Success,
		Detail:           g.Spec.Memo,
	}).PrepareCreate(g)
	tmplSetAD := dao.auditDao.Decorator(kit, sg.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.TemplateSpaceName+constant.ResSeparator+constant.TemplateSetName,
			g.Spec.Name, sg.Spec.Name),
		Status: enumor.Success,
		Detail: sg.Spec.Memo,
	}).PrepareCreate(sg)

	// 多个使用事务处理
	createTx := func(tx *gen.Query) error {
		if err := tx.TemplateSpace.WithContext(kit.Ctx).Create(g); err != nil {
			return err
		}

		// 连带创建模版空间下的默认套餐
		if err := tx.TemplateSet.WithContext(kit.Ctx).Create(sg); err != nil {
			return err
		}

		if err := tmplSpaceAD.Do(tx); err != nil {
			return err
		}
		if err := tmplSetAD.Do(tx); err != nil {
			return err
		}

		return nil
	}
	if err := dao.genQ.Transaction(createTx); err != nil {
		return 0, err
	}

	return g.ID, nil
}

// GetByUniqueKey get template space by unique key
func (dao *templateSpaceDao) GetByUniqueKey(kit *kit.Kit, bizID uint32, name string) (*table.TemplateSpace, error) {
	m := dao.genQ.TemplateSpace
	q := dao.genQ.TemplateSpace.WithContext(kit.Ctx)

	templateSpace, err := q.Where(m.BizID.Eq(bizID), m.Name.Eq(name)).Take()
	if err != nil {
		return nil, fmt.Errorf("get template space failed, err: %v", err)
	}

	return templateSpace, nil
}

// ListByIDs list template spaces by template space ids.
func (dao *templateSpaceDao) ListByIDs(kit *kit.Kit, ids []uint32) ([]*table.TemplateSpace, error) {
	m := dao.genQ.TemplateSpace
	q := dao.genQ.TemplateSpace.WithContext(kit.Ctx)
	result, err := q.Where(m.ID.In(ids...)).Find()
	if err != nil {
		return nil, err
	}

	return result, nil
}
