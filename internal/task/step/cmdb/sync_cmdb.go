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

	cmdbGse "github.com/TencentBlueKing/bk-bscp/internal/task/executor/cmdb_gse"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// SyncCMDB 同步业务步骤
func SyncCMDB(tenantID string, bizID uint32) *types.Step {
	logs.V(3).Infof("Start synchronizing CMDB, tenantID=%s, bizID=%d", tenantID, bizID)

	tf := cc.G().TaskFramework.SyncCMDB.SyncCMDB
	syncCmdb := types.NewStep("sync-cmdb-task", cmdbGse.SyncCMDB.String()).
		SetAlias("sync-cmdb").
		SetMaxExecution(tf.MaxExecution).
		SetMaxTries(tf.MaxRetries)

	lo.Must0(syncCmdb.SetPayload(cmdbGse.SyncCMDBPayload{
		BizID:    bizID,
		TenantID: tenantID,
	}))

	return syncCmdb
}
