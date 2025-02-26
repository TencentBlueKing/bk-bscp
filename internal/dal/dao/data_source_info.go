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

// Package dao NOTES
package dao

import (
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// DataSourceInfo mapped from table <data_source_infos>
type DataSourceInfo interface {
	// Get data source infos by id
	Get(kit *kit.Kit, bizID, id uint32) (*table.DataSourceInfo, error)
}

var _ DataSourceInfo = new(dataSourceInfoDao)

type dataSourceInfoDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
}

// Get implements DataSourceInfo.
func (dao *dataSourceInfoDao) Get(kit *kit.Kit, bizID uint32, id uint32) (
	*table.DataSourceInfo, error) {
	m := dao.genQ.DataSourceInfo
	return dao.genQ.DataSourceInfo.WithContext(kit.Ctx).Where(m.BizID.Eq(bizID), m.ID.Eq(id)).Take()
}
