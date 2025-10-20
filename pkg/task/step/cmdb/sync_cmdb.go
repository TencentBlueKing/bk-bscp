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
	"strconv"
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"github.com/TencentBlueKing/bk-bscp/pkg/task/executor/cmdb"
)

// Biz 同步业务步骤
func SyncCMDB(bizID int) *types.Step {
	add := types.NewStep("sync-cmdb-task", cmdb.SyncCMDB.String()).
		SetAlias("sync-cmdb").
		AddParam("bizID", strconv.Itoa(bizID)).
		SetMaxExecution(3 * time.Minute).
		SetMaxTries(3)

	return add
}
