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

package v4

import (
	"context"
	"testing"

	"github.com/davecgh/go-spew/spew"

	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/uuid"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// TestGetApproveResult tests the GetApproveResult method of ITSMV4Adapter
func TestGetApproveResult(t *testing.T) {
	// 设置必要的配置
	setupITSMConfig()

	// 创建带有必要信息的上下文
	kt := &kit.Kit{
		Ctx:      context.Background(),
		User:     "test_user",
		TenantID: "system",
		Rid:      "test-" + uuid.UUID(),
		AppCode:  "bk-bscp", // 设置测试用的 AppCode
	}
	ctx := kt.InternalRpcCtx()

	// 创建适配器实例
	adapter := &ITSMV4Adapter{}

	// 测试工单ID - 注意：这个ID需要是实际存在的工单ID，否则会报错
	ticketID := "102025083023304300009701"
	activeKey := "activityobject_20250813130257_2"

	// 调用GetApproveResult方法
	result, err := adapter.GetApproveResult(ctx, api.GetApproveResultReq{
		TicketID:    ticketID,
		ActivityKey: activeKey,
	})

	// 打印结果或错误
	if err != nil {
		t.Logf("调用GetApproveResult出错: %v", err)
		return
	}

	// 打印结果
	t.Logf("审批结果: %v", spew.Sdump(result))
}

// setupITSMConfig 设置ITSM配置
func setupITSMConfig() {
	cc.InitRuntime(&cc.DataServiceSetting{
		ITSM: cc.ITSMConfig{
			GatewayHost: "",
			BscpGateway: "",
			BscpPageUrl: "",
			EnableV4:    true,
			AppCode:     "bk_bscp",
			AppSecret:   "",
			User:        "admin",
			SystemId:    "bk_bscp",
		},
	})
}
