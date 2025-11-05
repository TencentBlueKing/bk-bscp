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
	"github.com/TencentBlueKing/bk-bscp/internal/task/step/process"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// SyncGseStatus 同步gse状态
func SyncGseStatus(bizID uint32, opType gse.OpType) *types.Step {
	logs.V(3).Infof("Start synchronizing GSE, bizID=%d, opType=%v", bizID, opType)

	syncCmdb := types.NewStep("sync-gse-task", cmdbGse.SyncGSE.String()).
		SetAlias("sync-gse").
		SetMaxExecution(process.MaxExecutionTime).
		SetMaxTries(process.MaxTries)

	lo.Must0(syncCmdb.SetPayload(cmdbGse.SyncGSEPayload{
		OpType: opType,
		BizID:  bizID,
	}))

	return syncCmdb
}
