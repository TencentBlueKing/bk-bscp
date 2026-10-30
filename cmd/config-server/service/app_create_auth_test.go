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

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
)

// 创建/克隆服务按目标环境授权：服务落在具体环境里，创建权限只覆盖被授权的环境，
// 不会因为一条业务维度的授权就扩散到业务下的全部项目与环境。
func TestAppCreateAuthResources(t *testing.T) {
	res := appCreateAuthResources(100001, 40001, 50001)
	require.Len(t, res, 2)

	byType := make(map[meta.ResourceType]*meta.ResourceAttribute, len(res))
	for _, r := range res {
		byType[r.Type] = r
		require.Equal(t, uint32(100001), r.BizID, "all resources should carry biz id")
	}

	bizRes := byType[meta.Biz]
	require.NotNil(t, bizRes)
	require.Equal(t, meta.FindBusinessResource, bizRes.Action)

	appRes := byType[meta.App]
	require.NotNil(t, appRes)
	require.Equal(t, meta.Create, appRes.Action)
	// 创建时尚无服务实例，ResourceID 是目标环境 ID：授权维度落在服务将要归属的环境上。
	require.Equal(t, uint32(50001), appRes.ResourceID, "app create is granted on the target environment")
	require.Equal(t, uint32(40001), appRes.ProjectID, "project ownership is needed to build the iam path")
	require.Equal(t, uint32(50001), appRes.EnvironmentID)

	// 不再额外要求环境编辑权限：app_create 本身已经按环境授予。
	require.NotContains(t, byType, meta.Environment)
}

// 服务列表的两个入口共用同一条规则：只校验业务访问，不按环境维度拦截。
// 环境与服务的逐项权限由列表注解带回前端置灰，环境隔离不靠列表接口实现；
// 否则只授了项目下部分环境的调用方整页拿不到数据，连申请入口都没有。
func TestAppListAuthResources(t *testing.T) {
	res := appListAuthResources(100001)
	require.Len(t, res, 1)
	require.Equal(t, meta.Biz, res[0].Type)
	require.Equal(t, meta.FindBusinessResource, res[0].Action)
	require.Equal(t, uint32(100001), res[0].BizID)
}
