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

package cmdbGse

import (
	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	gseSvc "github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/processor/cmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/processor/gse"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	// SyncCMDB 同步cmdb
	SyncCMDB istep.StepName = "SyncCMDB"
	// SyncGSE 同步GSE
	SyncGSE istep.StepName = "SyncGSE"
)

// NewSyncCMDBExecutor new sync cmdb gse executor
func NewSyncCmdbGseExecutor(gseSvc *gseSvc.Service, cmdbSvc bkcmdb.Service,
	dao dao.Set) *syncCmdbGseExecutor {
	return &syncCmdbGseExecutor{
		cmdbSvc: cmdbSvc,
		dao:     dao,
		gseSvc:  gseSvc,
	}
}

// syncCmdbGseExecutor sync cmdb gse executor
type syncCmdbGseExecutor struct {
	cmdbSvc bkcmdb.Service
	gseSvc  *gseSvc.Service
	dao     dao.Set
}

// SyncCMDBPayload 同步cmdb相关负载
type SyncCMDBPayload struct {
	BizID    uint32
	TenantID string
}

// SyncGSEPayload 同步gse相关负载
type SyncGSEPayload struct {
	TenantID string
	BizID    uint32
	OpType   gseSvc.OpType
}

// SyncCMDB implements istep.Step.
func (s *syncCmdbGseExecutor) SyncCMDB(c *istep.Context) error {
	payload := &SyncCMDBPayload{}
	if err := c.GetPayload(payload); err != nil {
		return err
	}

	// 同步cc数据
	syncSvc := cmdb.NewSyncCMDBService(payload.TenantID, int(payload.BizID), s.cmdbSvc, s.dao)
	if err := syncSvc.SyncSingleBiz(c.Context()); err != nil {
		logs.Errorf("tenant: %s biz: %d sync cmdb data failed: %v", payload.TenantID, payload.BizID, err)
		return err
	}

	return nil
}

// SyncGSE implements istep.Step.
func (s *syncCmdbGseExecutor) SyncGSE(c *istep.Context) error {
	payload := &SyncGSEPayload{}
	if err := c.GetPayload(payload); err != nil {
		return err
	}
	// 同步gse状态
	gseService := gse.NewSyncGESService(payload.TenantID, int(payload.BizID), s.gseSvc, s.dao)
	if err := gseService.SyncSingleBiz(c.Context()); err != nil {
		logs.Errorf("tenant: %s biz: %d sync gse data failed: %v", payload.TenantID, payload.BizID, err)
		return err
	}

	return nil
}

// RegisterExecutor register step
func RegisterExecutor(s *syncCmdbGseExecutor) {
	istep.Register(SyncCMDB, istep.StepExecutorFunc(s.SyncCMDB))
	istep.Register(SyncGSE, istep.StepExecutorFunc(s.SyncGSE))
}
