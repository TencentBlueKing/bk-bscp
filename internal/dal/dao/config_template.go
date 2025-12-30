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

	"gorm.io/datatypes"
	rawgen "gorm.io/gen"

	"github.com/TencentBlueKing/bk-bscp/internal/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/utils"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/enumor"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbct "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/config-template"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// ConfigTemplate is config template DAO interface definition.
type ConfigTemplate interface {
	// CreateWithTx create one configTemplate instance.
	CreateWithTx(kit *kit.Kit, tx *gen.QueryTx, configTemplate *table.ConfigTemplate) (uint32, error)
	// ListAllByTemplateIDs list all configItem by templateIDs.
	ListAllByTemplateIDs(kit *kit.Kit, bizID uint32, templateIDs []uint32) ([]*table.ConfigTemplate, error)
	// GetByID get config template by id.
	GetByID(kit *kit.Kit, bizID uint32, configTemplateID uint32) (*table.ConfigTemplate, error)
	// Update one configTemplate instance.
	Update(kit *kit.Kit, configTemplate *table.ConfigTemplate) error
	ListByCCProcessID(kit *kit.Kit, bizID uint32, ccProcessID uint32) ([]uint32, error)
	ListByCCTemplateProcessID(kit *kit.Kit, bizID uint32, ccProcessTemplateID uint32) ([]uint32, error)
	// UpdateWithTx update one configTemplate instance within a transaction.
	UpdateWithTx(kit *kit.Kit, tx *gen.QueryTx, configTemplate *table.ConfigTemplate) error
	// DeleteWithTx delete one template instance with transaction.
	DeleteWithTx(kit *kit.Kit, tx *gen.QueryTx, configTemplate *table.ConfigTemplate) error
	List(kit *kit.Kit, bizID, templateSpaceID uint32, search *pbct.TemplateSearchCond, opt *types.BasePage) (
		[]*table.ConfigTemplate, int64, error)
	GetByUniqueKey(kit *kit.Kit, bizID, id uint32, name string) (*table.ConfigTemplate, error)
}

var _ ConfigTemplate = new(configTemplateDao)

type configTemplateDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
}

// GetByUniqueKey implements [ConfigTemplate].
func (dao *configTemplateDao) GetByUniqueKey(kit *kit.Kit, bizID uint32, id uint32, name string) (
	*table.ConfigTemplate, error) {
	m := dao.genQ.ConfigTemplate

	return dao.genQ.ConfigTemplate.WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID), m.Name.Eq(name), m.ID.Neq(id)).
		Take()
}

// List implements [ConfigTemplate].
func (dao *configTemplateDao) List(kit *kit.Kit, bizID uint32, templateSpaceID uint32, search *pbct.TemplateSearchCond,
	opt *types.BasePage) ([]*table.ConfigTemplate, int64, error) {
	m := dao.genQ.ConfigTemplate

	conds, err := dao.handleSearch(kit, bizID, templateSpaceID, search)
	if err != nil {
		return nil, 0, err
	}

	q := dao.genQ.ConfigTemplate.WithContext(kit.Ctx).Where(m.BizID.Eq(bizID)).Where(conds...)
	if opt.All {
		result, err := q.Order(m.ID.Desc()).Find()
		if err != nil {
			return nil, 0, err
		}
		return result, int64(len(result)), err
	}

	return q.Order(m.ID.Desc()).FindByPage(opt.Offset(), opt.LimitInt())
}

// handle search
func (dao *configTemplateDao) handleSearch(kit *kit.Kit, bizID, templateSpaceID uint32, search *pbct.TemplateSearchCond) (
	[]rawgen.Condition, error) {
	if search.String() == "" {
		return []rawgen.Condition{}, nil
	}

	var conds []rawgen.Condition
	m := dao.genQ.ConfigTemplate

	if search.GetTemplateName() != "" {
		conds = append(conds, m.Name.Like("%"+search.GetTemplateName()+"%"))
	}

	if search.GetReviser() != "" {
		conds = append(conds, (m.Reviser.Eq(search.GetReviser())))
	}

	templateIDSet := make(map[uint32]struct{})
	if len(search.GetTemplateId()) != 0 {
		for _, id := range search.GetTemplateId() {
			templateIDSet[id] = struct{}{}
		}
	}
	// 根据文件名搜索
	if search.GetFileName() != "" {
		t := dao.genQ.Template
		var items []struct{ ID uint32 }
		err := dao.genQ.Template.WithContext(kit.Ctx).Where(t.BizID.Eq(bizID), t.TemplateSpaceID.Eq(templateSpaceID)).Where(
			utils.RawCond(`CASE WHEN RIGHT(path, 1) = '/' THEN CONCAT(path,name)
			 ELSE CONCAT_WS('/', path, name) END LIKE ?`, "%"+search.GetFileName()+"%")).Scan(&items)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			// 添加一个永远不会匹配的条件
			conds = append(conds, dao.genQ.ConfigTemplate.ID.Eq(0))
			return conds, nil
		}

		for _, v := range items {
			templateIDSet[v.ID] = struct{}{}
		}
	}

	if len(templateIDSet) > 0 {
		ids := make([]uint32, 0, len(templateIDSet))
		for id := range templateIDSet {
			ids = append(ids, id)
		}
		conds = append(conds, m.TemplateID.In(ids...))
	}

	return conds, nil
}

// DeleteWithTx implements ConfigTemplate.
func (dao *configTemplateDao) DeleteWithTx(kit *kit.Kit, tx *gen.QueryTx, ct *table.ConfigTemplate) error {

	// 删除操作, 获取当前记录做审计
	m := tx.ConfigTemplate
	q := tx.ConfigTemplate.WithContext(kit.Ctx)
	oldOne, err := q.Where(m.ID.Eq(ct.ID), m.BizID.Eq(ct.Attachment.BizID)).Take()
	if err != nil {
		return err
	}

	ad := dao.auditDao.Decorator(kit, ct.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.ConfigTemplateName, ct.Spec.Name),
		Status:           enumor.Success,
	}).PrepareDelete(oldOne)
	if err := ad.Do(tx.Query); err != nil {
		return err
	}

	if _, err := q.Where(m.BizID.Eq(ct.Attachment.BizID)).Delete(ct); err != nil {
		return err
	}

	return nil
}

// UpdateWithTx implements ConfigTemplate.
func (dao *configTemplateDao) UpdateWithTx(kit *kit.Kit, tx *gen.QueryTx, ct *table.ConfigTemplate) error {
	if ct == nil {
		return errf.New(errf.InvalidParameter, "config template is nil")
	}

	m := tx.ConfigTemplate

	ad := dao.auditDao.Decorator(kit, ct.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.ConfigTemplateName, ct.Spec.Name),
		Status:           enumor.Success,
	}).PrepareUpdate(ct)
	if err := ad.Do(tx.Query); err != nil {
		return fmt.Errorf("audit update config template failed, err: %v", err)
	}

	q := tx.ConfigTemplate.WithContext(kit.Ctx)
	if _, err := q.Omit(m.BizID, m.ID).
		Where(m.BizID.Eq(ct.Attachment.BizID), m.ID.Eq(ct.ID)).Updates(ct); err != nil {
		return err
	}

	return nil
}

// ListByCCProcessID implements ConfigTemplate.
func (dao *configTemplateDao) ListByCCProcessID(kit *kit.Kit, bizID uint32, ccProcessID uint32) ([]uint32, error) {
	m := dao.genQ.ConfigTemplate

	var ids []uint32

	err := dao.genQ.ConfigTemplate.WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID)).
		Where(rawgen.Cond(datatypes.JSONArrayQuery(m.CcProcessIDs.ColumnName().String()).Contains(ccProcessID))...).
		Pluck(m.ID, &ids)
	if err != nil {
		return nil, err
	}

	return ids, nil
}

// ListByCCTemplateProcessID implements ConfigTemplate.
func (dao *configTemplateDao) ListByCCTemplateProcessID(kit *kit.Kit, bizID uint32, ccProcessTemplateID uint32) ([]uint32, error) {
	m := dao.genQ.ConfigTemplate

	var ids []uint32

	err := dao.genQ.ConfigTemplate.WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID)).
		Where(rawgen.Cond(datatypes.JSONArrayQuery(m.CcTemplateProcessIDs.ColumnName().String()).Contains(ccProcessTemplateID))...).
		Pluck(m.ID, &ids)
	if err != nil {
		return nil, err
	}

	return ids, nil
}

// Update implements ConfigTemplate.
func (dao *configTemplateDao) Update(kit *kit.Kit, ct *table.ConfigTemplate) error {
	if ct == nil {
		return errf.New(errf.InvalidParameter, "config item is nil")
	}

	m := dao.genQ.ConfigTemplate
	q := dao.genQ.ConfigTemplate.WithContext(kit.Ctx)

	ad := dao.auditDao.Decorator(kit, ct.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.ConfigTemplateName, ct.Spec.Name),
		Status:           enumor.Success,
	}).PrepareUpdate(ct)

	updateTx := func(tx *gen.Query) error {
		q = tx.ConfigTemplate.WithContext(kit.Ctx)
		if _, err := q.Omit(m.BizID, m.ID).Select(m.Name, m.CcProcessIDs, m.CcTemplateProcessIDs, m.UpdatedAt, m.Reviser, m.HighlightStyle).
			Where(m.BizID.Eq(ct.Attachment.BizID), m.ID.Eq(ct.ID)).Updates(ct); err != nil {
			return err
		}

		if err := ad.Do(tx); err != nil {
			return fmt.Errorf("audit update config template failed, err: %v", err)
		}
		return nil
	}

	if err := dao.genQ.Transaction(updateTx); err != nil {
		logs.Errorf("update config template: %d failed, err: %v, rid: %v", ct.ID, err, kit.Rid)
		return err
	}

	return nil
}

// ListAllByTemplateIDs implements ConfigTemplate.
func (dao *configTemplateDao) ListAllByTemplateIDs(kit *kit.Kit, bizID uint32, templateIDs []uint32) (
	[]*table.ConfigTemplate, error) {
	m := dao.genQ.ConfigTemplate

	return dao.genQ.ConfigTemplate.WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID), m.TemplateID.In(templateIDs...)).
		Find()
}

// CreateWithTx implements ConfigTemplate.
func (dao *configTemplateDao) CreateWithTx(kit *kit.Kit, tx *gen.QueryTx, ct *table.ConfigTemplate) (
	uint32, error) {
	if ct == nil {
		return 0, errors.New("config template is nil")
	}

	id, err := dao.idGen.One(kit, table.ConfigTemplatesTable)
	if err != nil {
		return 0, err
	}

	ct.ID = id
	ad := dao.auditDao.Decorator(kit, ct.Attachment.BizID, &table.AuditField{
		ResourceInstance: fmt.Sprintf(constant.ConfigTemplateName, ct.Spec.Name),
		Status:           enumor.Success,
	}).PrepareCreate(ct)

	if err := tx.ConfigTemplate.WithContext(kit.Ctx).Create(ct); err != nil {
		return 0, err
	}

	if err := ad.Do(tx.Query); err != nil {
		return 0, fmt.Errorf("audit create config template failed, err: %v", err)
	}

	return id, nil
}

// GetByID implements ConfigTemplate.
func (dao *configTemplateDao) GetByID(kit *kit.Kit, bizID uint32, configTemplateID uint32) (*table.ConfigTemplate, error) {
	m := dao.genQ.ConfigTemplate

	return dao.genQ.ConfigTemplate.WithContext(kit.Ctx).
		Where(m.ID.Eq(configTemplateID), m.BizID.Eq(bizID)).
		Take()
}
