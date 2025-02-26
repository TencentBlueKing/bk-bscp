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
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// ReleasedTableContent mapped from table <released_table_contents>
type ReleasedTableContent interface {
	// List the table data after publishing
	List(kit *kit.Kit, releaseKvID uint32, opt *types.BasePage) ([]*table.ReleasedTableContent, int64, error)
	// BatchCreateWithTx multiple data source content with transaction.
	BatchCreateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.ReleasedTableContent) error
}

var _ ReleasedTableContent = new(releasedTableContentDao)

type releasedTableContentDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
}

// List implements ReleasedTableContent.
func (dao *releasedTableContentDao) List(kit *kit.Kit, releaseKvID uint32, opt *types.BasePage) (
	[]*table.ReleasedTableContent, int64, error) {
	m := dao.genQ.ReleasedTableContent
	q := dao.genQ.ReleasedTableContent.WithContext(kit.Ctx)
	q = q.Where(m.ReleaseKvID.Eq(releaseKvID))

	if opt.All {
		result, err := q.Find()
		if err != nil {
			return nil, 0, err
		}
		return result, int64(len(result)), err
	}

	return q.FindByPage(opt.Offset(), opt.LimitInt())
}

// BatchCreateWithTx implements ReleasedTableContent.
func (dao *releasedTableContentDao) BatchCreateWithTx(kit *kit.Kit, tx *gen.QueryTx, data []*table.ReleasedTableContent) error {
	if len(data) == 0 {
		return nil
	}
	ids, err := dao.idGen.Batch(kit, table.ReleasedTableContentTable, len(data))
	if err != nil {
		return err
	}
	for i, item := range data {
		if err := item.ValidateCreate(kit); err != nil {
			return err
		}
		item.ID = ids[i]
	}
	if err := tx.ReleasedTableContent.WithContext(kit.Ctx).CreateInBatches(data, 500); err != nil {
		return err
	}

	return nil
}
