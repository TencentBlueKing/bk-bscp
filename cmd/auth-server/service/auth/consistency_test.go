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

package auth

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	adaptorv4 "github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/adaptor"
	clientv4 "github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/client"
)

// V3 与 V4 对同一份资源、同一拓扑必须给出相同的动作、实例与鉴权路径，
// 避免两套映射实现随时间漂移。
func TestV3V4AuthMappingConsistency(t *testing.T) {
	cases := []struct {
		name string
		attr *meta.ResourceAttribute
	}{
		{"app-view", &meta.ResourceAttribute{
			Basic:         meta.Basic{Type: meta.App, Action: meta.View, ResourceID: 30001},
			BizID:         100001,
			ProjectID:     40001,
			EnvironmentID: 50001,
		}},
		{"app-edit", &meta.ResourceAttribute{
			Basic:         meta.Basic{Type: meta.App, Action: meta.Update, ResourceID: 30001},
			BizID:         100001,
			ProjectID:     40001,
			EnvironmentID: 50001,
		}},
		{"app-publish", &meta.ResourceAttribute{
			Basic:         meta.Basic{Type: meta.App, Action: meta.Publish, ResourceID: 30001},
			BizID:         100001,
			ProjectID:     40001,
			EnvironmentID: 50001,
		}},
		{"environment-view", &meta.ResourceAttribute{
			Basic:     meta.Basic{Type: meta.Environment, Action: meta.View, ResourceID: 50001},
			BizID:     100001,
			ProjectID: 40001,
		}},
		{"project-view", &meta.ResourceAttribute{
			Basic: meta.Basic{Type: meta.Project, Action: meta.View, ResourceID: 40001},
			BizID: 100001,
		}},
	}

	for _, topology := range []meta.ResourceTopology{meta.TopologyLegacy, meta.TopologyHierarchical} {
		for _, c := range cases {
			v3Action, v3Resources, err := AdaptAuthOptionsWithOptions(c.attr, PermissionOptions{
				ProjectEnvironmentAuth: true,
				Topology:               topology,
			})
			require.NoError(t, err, "%s/%s", topology, c.name)
			require.Len(t, v3Resources, 1, "%s/%s", topology, c.name)

			v4Mapped, err := adaptorv4.AdaptWithOptions(c.attr, adaptorv4.Options{
				ProjectEnvironmentAuth: true,
				Topology:               topology,
			})
			require.NoError(t, err, "%s/%s", topology, c.name)

			require.Equal(t, string(v3Action), v4Mapped.ActionID, "%s/%s action", topology, c.name)
			require.Equal(t, string(v3Resources[0].Type), v4Mapped.ResourceType,
				"%s/%s resource type", topology, c.name)
			require.Equal(t, v3Resources[0].ID, v4Mapped.ResourceID,
				"%s/%s resource id", topology, c.name)

			v4Resource := v4Mapped.AuthResource()
			require.NotNil(t, v4Resource, "%s/%s auth resource", topology, c.name)

			paths, ok := v3Resources[0].Attribute[client.IamPathKey].([]string)
			require.True(t, ok, "%s/%s v3 path", topology, c.name)
			require.Len(t, paths, 1, "%s/%s v3 path", topology, c.name)
			require.Equal(t, paths[0], v4Resource.Attributes[clientv4.IAMPathAttrKey],
				"%s/%s path", topology, c.name)
		}
	}
}

// 服务创建在两种拓扑下都落在各自的业务维度或环境维度，V3/V4 保持一致。
func TestV3V4AppCreateConsistency(t *testing.T) {
	attr := &meta.ResourceAttribute{
		Basic:         meta.Basic{Type: meta.App, Action: meta.Create, ResourceID: 50001},
		BizID:         100001,
		ProjectID:     40001,
		EnvironmentID: 50001,
	}

	// 旧拓扑：两边都按业务维度发起。
	v3Action, v3Resources, err := AdaptAuthOptionsWithOptions(attr, PermissionOptions{
		Topology: meta.TopologyLegacy,
	})
	require.NoError(t, err)
	require.Len(t, v3Resources, 1)
	require.Equal(t, string(sys.Business), string(v3Resources[0].Type))

	v4Mapped, err := adaptorv4.AdaptWithOptions(attr, adaptorv4.Options{Topology: meta.TopologyLegacy})
	require.NoError(t, err)
	require.Equal(t, string(v3Action), v4Mapped.ActionID)
	require.Equal(t, "biz", v4Mapped.ResourceType)
	require.Equal(t, "100001", v4Mapped.ResourceID)

	// 新拓扑：两边都按目标环境发起。V3 的 app_create 带着按业务授予的存量策略无法改关联资源，
	// 按环境创建使用独立操作；V4 是新系统，沿用 app_create。
	v3Action, v3Resources, err = AdaptAuthOptionsWithOptions(attr, PermissionOptions{
		Topology: meta.TopologyHierarchical,
	})
	require.NoError(t, err)
	require.Equal(t, sys.EnvironmentAppCreate, v3Action)
	require.Len(t, v3Resources, 1)
	require.Equal(t, "environment", string(v3Resources[0].Type))
	require.Equal(t, "50001", v3Resources[0].ID)

	v4Mapped, err = adaptorv4.AdaptWithOptions(attr, adaptorv4.Options{Topology: meta.TopologyHierarchical})
	require.NoError(t, err)
	require.Equal(t, string(sys.AppCreate), v4Mapped.ActionID)
	require.Equal(t, "environment", v4Mapped.ResourceType)
	require.Equal(t, "50001", v4Mapped.ResourceID)
}
