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
	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"
)

const (
	// SyncCMDB xxx
	SyncCMDB istep.StepName = "SyncCMDB"
)

// NewSyncBizExecutor xxx
func NewSyncBizExecutor() *SyncBizExecutor {
	return &SyncBizExecutor{}
}

// HelloExecutor hello step executor
type SyncBizExecutor struct {
}

// SyncCMDB implements istep.Step.
func (s *SyncBizExecutor) SyncCMDB(c *istep.Context) (err error) {

	// bizID, err := c.GetParam("bizID")
	// if err != nil {
	// 	return err
	// }

	// // 同步业务逻辑
	// bizList, err := s.cmdb.SearchBusinessByAccount(c.Context(), bkcmdb.SearchSetReq{
	// 	BkSupplierAccount: "0",
	// 	Fields:            []string{"bk_biz_id", "bk_biz_name"},
	// })
	// if err != nil {
	// 	return fmt.Errorf("get business data failed: %v", err)
	// }

	// var business bkcmdb.Business
	// if err := bizList.Decode(&business); err != nil {
	// 	return fmt.Errorf("parse business data: %v", err)
	// }

	// id, err := strconv.Atoi(bizID)
	// if err != nil {
	// 	return err
	// }

	// syncSvc := cmdb.SyncCMDBService{
	// 	BizID: bizID,
	// 	Svc:   nil,
	// }

	// err = syncSvc.SyncSingleBiz(c.Context())
	// if err != nil {
	// 	return err
	// }

	return nil
}

// Register register step
func Register(s *SyncBizExecutor) {
	istep.Register(SyncCMDB, istep.StepExecutorFunc(s.SyncCMDB))
}
