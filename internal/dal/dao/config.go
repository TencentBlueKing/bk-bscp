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

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// Config supplies all the config related operations.
type Config interface {
	// GetConfig Get itsm config.
	GetConfig(kit *kit.Kit, key string) (*table.Config, error)
	// UpsertConfig insert or update itsm config.
	UpsertConfig(kit *kit.Kit, itsmConfigs []*table.Config) error
}

var _ Config = new(configDao)

type configDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao // nolint
}

// GetConfig Get itsm config.
func (dao *configDao) GetConfig(kit *kit.Kit, key string) (*table.Config, error) {
	m := dao.genQ.Config
	return m.WithContext(kit.Ctx).Where(
		m.Key.Eq(key)).Take()
}

// SetConfig Set itsm config.
func (dao *configDao) UpsertConfig(kit *kit.Kit, itsmConfigs []*table.Config) error {
	m := dao.genQ.Config
	for _, v := range itsmConfigs {
		config, err := m.WithContext(kit.Ctx).Where(
			m.Key.Eq(v.Key)).Take()
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// 没有记录的时候直接创建
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// generate an content id and update to content.
			id, err := dao.idGen.One(kit, table.ConfigTable)
			if err != nil {
				return err
			}
			v.ID = id

			err = m.WithContext(kit.Ctx).Create(v)
			if err != nil {
				return err
			}
			continue
		}

		// 值不一样直接更新
		if config.Value != v.Value {
			_, err = m.WithContext(kit.Ctx).Where(m.Key.Eq(v.Key)).Select(m.Value).Updates(v)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
