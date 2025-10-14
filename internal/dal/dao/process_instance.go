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
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// ProcessInstance xxx
type ProcessInstance interface {
	// List released config items with options.
	GetProcessInstancesByID(kit *kit.Kit, bizID uint32, processID []uint32) ([]*table.ProcessInstance, error)
}

var _ ProcessInstance = new(processInstanceDao)

type processInstanceDao struct {
	genQ     *gen.Query
	idGen    IDGenInterface
	auditDao AuditDao
}

// GetProcessInstancesByID implements ProcessInstance.
func (dao *processInstanceDao) GetProcessInstancesByID(kit *kit.Kit, bizID uint32, processID []uint32) (
	[]*table.ProcessInstance, error) {
	m := dao.genQ.ProcessInstance
	q := dao.genQ.ProcessInstance.WithContext(kit.Ctx)

	result, err := q.Where(m.BizID.Eq(bizID), m.ProcessID.In(processID...)).Find()
	if err != nil {
		return nil, err
	}

	return result, err
}
