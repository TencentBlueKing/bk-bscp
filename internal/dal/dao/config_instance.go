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
	rawgen "gorm.io/gen"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// ConfigInstanceSearchCondition 配置实例搜索条件
type ConfigInstanceSearchCondition struct {
	CcProcessIds     []uint32
	ConfigTemplateId uint32
}

// ConfigInstance supplies all the config instance related operations.
type ConfigInstance interface {
	// List lists config instances with options.
	List(kit *kit.Kit, bizID uint32, search *ConfigInstanceSearchCondition,
		opt *types.BasePage) ([]*table.ConfigInstance, int64, error)
}

var _ ConfigInstance = new(configInstanceDao)

type configInstanceDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
}

// List implements ConfigInstance.
func (dao *configInstanceDao) List(kit *kit.Kit, bizID uint32, search *ConfigInstanceSearchCondition,
	opt *types.BasePage) ([]*table.ConfigInstance, int64, error) {
	m := dao.genQ.ConfigInstance
	q := dao.genQ.ConfigInstance.WithContext(kit.Ctx)

	var conds []rawgen.Condition
	if search != nil {
		conds = dao.handleSearch(search)
	}

	d := q.Where(m.BizID.Eq(bizID)).Where(conds...).Order(m.ID.Desc())

	if opt.All {
		result, err := d.Find()
		if err != nil {
			return nil, 0, err
		}
		return result, int64(len(result)), err
	}
	return d.FindByPage(opt.Offset(), opt.LimitInt())
}

func (dao *configInstanceDao) handleSearch(search *ConfigInstanceSearchCondition) []rawgen.Condition {
	var conds []rawgen.Condition
	m := dao.genQ.ConfigInstance

	// ConfigTemplateId 过滤
	if search.ConfigTemplateId > 0 {
		conds = append(conds, m.ConfigTemplateID.Eq(search.ConfigTemplateId))
	}

	// CcProcessIds 过滤
	if len(search.CcProcessIds) > 0 {
		conds = append(conds, m.CcProcessID.In(search.CcProcessIds...))
	}

	return conds
}
