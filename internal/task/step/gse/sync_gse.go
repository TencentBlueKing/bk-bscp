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
	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"github.com/samber/lo"

	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	cmdbGse "github.com/TencentBlueKing/bk-bscp/internal/task/executor/cmdb_gse"
	processStateSync "github.com/TencentBlueKing/bk-bscp/internal/task/executor/gse"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// SyncGseStatus 同步gse状态
func SyncGseStatus(tenantID string, bizID uint32, opType gse.OpType) *types.Step {
	logs.V(3).Infof("Start synchronizing GSE, tenantID=%s, bizID=%d, opType=%v", tenantID, bizID, opType)

	tf := cc.G().TaskFramework.SyncGSE.SyncGseStatus
	syncCmdb := types.NewStep("sync-gse-task", cmdbGse.SyncGSE.String()).
		SetAlias("sync-gse").
		SetMaxExecution(tf.MaxExecution).
		SetMaxTries(tf.MaxRetries)

	lo.Must0(syncCmdb.SetPayload(cmdbGse.SyncGSEPayload{
		TenantID: tenantID,
		BizID:    bizID,
		OpType:   opType,
	}))

	return syncCmdb
}

// ProcessStateSync 同步 gse 进程和托管状态
func ProcessStateSync(tenantID string, bizID uint32, process *table.Process,
	processInstances []*table.ProcessInstance) *types.Step {

	ptf := cc.G().TaskFramework.SyncGSE.ProcessStateSync
	syncCmdb := types.NewStep("process-state-sync-task", processStateSync.ProcessStateSync.String()).
		SetAlias("process-state-sync").
		SetMaxExecution(ptf.MaxExecution).
		SetMaxTries(ptf.MaxRetries)

	lo.Must0(syncCmdb.SetPayload(processStateSync.ProcessStateSyncPayload{
		TenantID:         tenantID,
		BizID:            bizID,
		Process:          process,
		ProcessInstances: processInstances,
	}))

	return syncCmdb
}
