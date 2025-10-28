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

	v4 "github.com/TencentBlueKing/bk-bscp/internal/components/itsm/v4"
	"github.com/TencentBlueKing/bk-bscp/internal/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// CreateSystem 创建系统 一次性
func CreateSystem(ctx context.Context, createTemplate bool) error {
	kit := kit.FromGrpcContext(ctx)
	if err := v4.SystemCreate(ctx); err != nil {
		return fmt.Errorf("itsm system create failed, err: %v", err)
	}

	if !createTemplate {
		// 不需要创建模板，直接返回
		return nil
	}

	resp, err := v4.ItsmV4SystemMigrate(ctx)
	if err != nil {
		fmt.Printf("init approve itsm services failed, err: %s\n", err.Error())
		return err
	}

	// 通过 workflow_keys 获取 activity_key
	workflow, err := v4.ListWorkflow(ctx, v4.ListWorkflowReq{
		WorkflowKeys: resp.CreateApproveItsmWorkflowID.Value,
	})
	if err != nil {
		fmt.Printf("itsm list workflows failed, err: %s\n", err.Error())
		return err
	}
	// 存入配置表，如果是多租户则以租户ID为前缀
	prefix := ""
	if kit.TenantID != "" {
		prefix = fmt.Sprintf("%s-", kit.TenantID)
	}
	itsmConfigs := []*table.Config{
		{
			Key:   fmt.Sprintf("%s%s", prefix, constant.CreateApproveItsmWorkflowID),
			Value: resp.CreateApproveItsmWorkflowID.Value,
		}, {
			Key:   fmt.Sprintf("%s%s", prefix, constant.CreateCountSignApproveItsmStateID),
			Value: workflow[constant.ItsmApproveCountSignType],
		}, {
			Key:   fmt.Sprintf("%s%s", prefix, constant.CreateOrSignApproveItsmStateID),
			Value: workflow[constant.ItsmApproveOrSignType],
		},
	}

	return daoSet.Config().UpsertConfig(kit, itsmConfigs)
}
