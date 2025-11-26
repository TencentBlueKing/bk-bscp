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

	"github.com/TencentBlueKing/bk-bscp/internal/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/enumor"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// ConfigTemplate is config template DAO interface definition.
type ConfigTemplate interface {
	// CreateWithTx create one configTemplate instance.
	CreateWithTx(kit *kit.Kit, tx *gen.QueryTx, configTemplate *table.ConfigTemplate) (uint32, error)
	// ListAllByTemplateIDs list all configItem by templateIDs.
	ListAllByTemplateIDs(kit *kit.Kit, bizID uint32, templateIDs []uint32) ([]*table.ConfigTemplate, error)
}

var _ ConfigTemplate = new(configTemplateDao)

type configTemplateDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
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
