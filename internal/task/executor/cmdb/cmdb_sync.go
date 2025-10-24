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

package cmdb

import (
	"fmt"

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/processor/cmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

const (
	// SyncCMDB xxx
	SyncCMDB istep.StepName = "SyncCMDB"
)

// NewSyncCMDBExecutor xxx
func NewSyncCMDBExecutor(svc bkcmdb.Service, dao dao.Set) *syncCmdbExecutor {
	return &syncCmdbExecutor{
		svc: svc,
		dao: dao,
	}
}

// SyncCMDBPayload 同步cmdb相关负载
type SyncCMDBPayload struct {
	OperateType table.CCSyncStatus
	BizID       uint32
}

// HelloExecutor hello step executor
type syncCmdbExecutor struct {
	svc bkcmdb.Service
	dao dao.Set
}

// SyncCMDB implements istep.Step.
func (s *syncCmdbExecutor) SyncCMDB(c *istep.Context) (err error) {
	payload := &SyncCMDBPayload{}
	if err = c.GetPayload(payload); err != nil {
		return err
	}
	// 同步业务逻辑
	bizList, err := s.svc.SearchBusinessByAccount(c.Context(), bkcmdb.SearchSetReq{
		BkSupplierAccount: "0",
		Fields:            []string{"bk_biz_id", "bk_biz_name"},
	})
	if err != nil {
		return fmt.Errorf("get business data failed: %v", err)
	}

	var business bkcmdb.Business
	if err := bizList.Decode(&business); err != nil {
		return fmt.Errorf("parse business data: %v", err)
	}

	syncSvc := cmdb.NewSyncCMDBService(int(payload.BizID), s.svc, s.dao)

	return syncSvc.SyncSingleBiz(c.Context())
}

// RegisterExecutor register step
func RegisterExecutor(s *syncCmdbExecutor) {
	istep.Register(SyncCMDB, istep.StepExecutorFunc(s.SyncCMDB))
}
