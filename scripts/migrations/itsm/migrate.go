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

// Package itsm 在 ITSM 注册服务，包括：创建命名空间、更新命名空间、删除命名空间, 允许重复执行
package itsm

import (
	"context"
	"fmt"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

var (
	// nolint:unused
	daoSet dao.Set
)

// InitServices 初始化BSCP相关流程服务
func InitServices(ctx context.Context, createTemplate bool) error {
	// initial DAO set
	set, err := dao.NewDaoSet(cc.DataService().Sharding, cc.DataService().Credential, cc.DataService().Gorm)
	if err != nil {
		return fmt.Errorf("initial dao set failed, err: %v", err)
	}

	daoSet = set

	if cc.DataService().ITSM.EnableV4 {
		err = CreateSystem(ctx, createTemplate)

	} else {
		err = InitApproveITSMServices()
	}
	if err != nil {
		return fmt.Errorf("init approve itsm services failed, err: %s", err.Error())
	}
	return nil
}
