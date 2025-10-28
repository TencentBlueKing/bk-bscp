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
	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"github.com/samber/lo"

	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/cmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/task/step/process"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// Biz 同步业务步骤
func SyncCMDB(bizID uint32, operateType table.CCSyncStatus) *types.Step {
	logs.V(3).Infof("Start synchronizing CMDB, bizID=%d", bizID)

	syncCmdb := types.NewStep("sync-cmdb-task", cmdb.SyncCMDB.String()).
		SetAlias("sync-cmdb").
		SetMaxExecution(process.MaxExecutionTime).
		SetMaxTries(process.MaxTries)

	lo.Must0(syncCmdb.SetPayload(cmdb.SyncCMDBPayload{
		OperateType: operateType,
		BizID:       bizID,
	}))

	return syncCmdb
}
