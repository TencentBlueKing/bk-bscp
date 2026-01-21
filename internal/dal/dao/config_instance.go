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

	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// ConfigInstanceSearchCondition 配置实例搜索条件
type ConfigInstanceSearchCondition struct {
	CcProcessIds     []uint32
	ConfigTemplateId uint32
	CcProcessId      uint32
	ModuleInstSeq    uint32
}

// ConfigInstance supplies all the config instance related operations.
type ConfigInstance interface {
	// List lists config instances with options.
	List(kit *kit.Kit, bizID uint32, search *ConfigInstanceSearchCondition,
		opt *types.BasePage) ([]*table.ConfigInstance, int64, error)
	// Upsert creates or updates a config instance
	Upsert(kit *kit.Kit, configInstance *table.ConfigInstance) error
	// 获取配置实例
	GetConfigInstance(kit *kit.Kit, bizID uint32, search *ConfigInstanceSearchCondition) (*table.ConfigInstance, error)
	ListConfigInstancesByTemplateID(kit *kit.Kit, bizID uint32, configTemplateIDs []uint32) ([]*table.ConfigInstance, error)
}

var _ ConfigInstance = new(configInstanceDao)

type configInstanceDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
}

// ListConfigInstancesByTemplateID implements [ConfigInstance].
func (dao *configInstanceDao) ListConfigInstancesByTemplateID(kit *kit.Kit, bizID uint32,
	configTemplateIDs []uint32) ([]*table.ConfigInstance, error) {
	m := dao.genQ.ConfigInstance

	return dao.genQ.ConfigInstance.WithContext(kit.Ctx).
		Where(m.BizID.Eq(bizID)).
		Where(m.ConfigTemplateID.In(configTemplateIDs...)).Find()
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

	// ModuleInstSeq 过滤
	if search.ModuleInstSeq > 0 {
		conds = append(conds, m.ModuleInstSeq.Eq(search.ModuleInstSeq))
	}

	return conds
}

// Upsert creates or updates a config instance
// 如果配置实例已存在（根据 biz_id, config_template_id, cc_process_id, module_inst_seq 判断），则更新
// 如果不存在，则创建新的配置实例
func (dao *configInstanceDao) Upsert(kit *kit.Kit, configInstance *table.ConfigInstance) error {
	if configInstance == nil {
		return fmt.Errorf("config instance is nil")
	}

	m := dao.genQ.ConfigInstance
	q := m.WithContext(kit.Ctx)

	// 查询是否已存在相同的配置实例
	existing, err := q.Where(
		m.BizID.Eq(configInstance.Attachment.BizID),
		m.ConfigTemplateID.Eq(configInstance.Attachment.ConfigTemplateID),
		m.CcProcessID.Eq(configInstance.Attachment.CcProcessID),
		m.ModuleInstSeq.Eq(configInstance.Attachment.ModuleInstSeq),
	).First()

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("query config instance failed: %w", err)
	}

	// 如果记录不存在，创建新记录
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 生成 ID
		id, err := dao.idGen.One(kit, table.ConfigInstancesTable)
		if err != nil {
			return fmt.Errorf("generate config instance id failed: %w", err)
		}
		configInstance.ID = id

		// 创建记录
		if err := q.Create(configInstance); err != nil {
			return fmt.Errorf("create config instance failed: %w", err)
		}
		return nil
	}

	// 如果记录已存在，更新记录
	configInstance.ID = existing.ID
	if _, err := q.Where(m.ID.Eq(existing.ID)).
		Select(m.ConfigVersionID, m.GenerateTaskID, m.TenantID, m.Reviser, m.UpdatedAt, m.Md5, m.Content).
		Updates(configInstance); err != nil {
		return fmt.Errorf("update config instance failed: %w", err)
	}

	return nil
}

// GetConfigInstance 获取配置实例
func (dao *configInstanceDao) GetConfigInstance(kit *kit.Kit, bizID uint32, search *ConfigInstanceSearchCondition) (*table.ConfigInstance, error) {
	m := dao.genQ.ConfigInstance
	q := m.WithContext(kit.Ctx)

	existing, err := q.Where(
		m.BizID.Eq(bizID),
		m.ConfigTemplateID.Eq(search.ConfigTemplateId),
		m.CcProcessID.Eq(search.CcProcessId),
		m.ModuleInstSeq.Eq(search.ModuleInstSeq),
	).First()
	if err != nil {
		return nil, err
	}

	return existing, nil
}
