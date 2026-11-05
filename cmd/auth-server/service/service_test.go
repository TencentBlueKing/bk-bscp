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

package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// 探测必须带上模型租户：V4 客户端读 kit.TenantID，V3 客户端从 ctx 元数据取租户。
// 任一取值缺失都会让多租户环境下的探测被权限中心拒绝。
func TestTopologyProbeKitCarriesTenant(t *testing.T) {
	kt, ctx, cancel := topologyProbeKit("system")
	defer cancel()

	require.Equal(t, "system", kt.TenantID)
	require.Equal(t, "system", kit.FromGrpcContext(ctx).TenantID)

	_, ok := ctx.Deadline()
	require.True(t, ok, "probe context should carry the detect timeout")
}
