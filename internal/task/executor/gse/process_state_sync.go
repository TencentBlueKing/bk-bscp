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

package gse

import (
	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	gseSvc "github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/processor/gse"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	// ProcessStateSync 同步cmdb
	ProcessStateSync istep.StepName = "ProcessStateSync"
)

// NewProcessStateSyncExecutor 创建进程状态同步执行器
func NewProcessStateSyncExecutor(cmdbSvc bkcmdb.Service, gseSvc *gseSvc.Service,
	dao dao.Set) *processStateSyncExecutor {
	return &processStateSyncExecutor{
		cmdbSvc: cmdbSvc,
		dao:     dao,
		gseSvc:  gseSvc,
	}
}

// processStateSyncExecutor 进程状态同步执行器
type processStateSyncExecutor struct {
	cmdbSvc bkcmdb.Service
	gseSvc  *gseSvc.Service
	dao     dao.Set
}

// ProcessStateSyncPayload 进程状态同步任务的输入数据
type ProcessStateSyncPayload struct {
	BizID            uint32
	Process          *table.Process
	ProcessInstances []*table.ProcessInstance
}

// ProcessStateSync implements istep.Step.
func (p *processStateSyncExecutor) ProcessStateSync(c *istep.Context) error {
	payload := &ProcessStateSyncPayload{}
	if err := c.GetPayload(payload); err != nil {
		return err
	}

	gseService := gse.NewSyncGESService(int(payload.BizID), p.gseSvc, p.dao)
	proc, procInsts, err := gseService.SyncSingleProcessStatus(c.Context(), payload.Process, payload.ProcessInstances)
	if err != nil {
		return err
	}

	tx := p.dao.GenQuery().Begin()
	committed := false
	defer func() {
		if !committed {
			if err := tx.Rollback(); err != nil {
				logs.Errorf("[SyncSingleBiz ERROR] biz %d: rollback failed, err=%v", payload.BizID, err)
			}
		}
	}()

	// 1. 批量更新实例
	if len(procInsts) > 0 {
		if err := p.dao.ProcessInstance().BatchUpdateWithTx(kit.New(), tx, procInsts); err != nil {
			return err
		}
	}

	// 2. Process 更新
	if err := p.dao.Process().BatchUpdateWithTx(kit.New(), tx, []*table.Process{proc}); err != nil {
		logs.Errorf("[SyncSingleBiz ERROR] biz %d: update processes failed, err=%v", payload.BizID, err)
		return err
	}
	if err := tx.Commit(); err != nil {
		logs.Errorf("[SyncSingleBiz ERROR] biz %d: commit failed, err=%v", payload.BizID, err)
		return err
	}
	committed = true

	return nil
}

// RegisterExecutor register step
func RegisterExecutor(p *processStateSyncExecutor) {
	istep.Register(ProcessStateSync, istep.StepExecutorFunc(p.ProcessStateSync))
}
