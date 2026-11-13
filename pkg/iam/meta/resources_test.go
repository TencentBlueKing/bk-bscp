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

package meta

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 配置项、KV、版本、上线共用一条规则：业务访问 + 项目查看 + 环境查看 + 服务动作，
// 四项必须同时提交。任意一处自行拼装或漏掉项目/环境维度，都会出现
// "能改配置项却读不到 KV""能生成版本却上不了线"这类互相矛盾的权限结果。
// 创建/克隆：业务访问 + 项目查看 + 环境查看 + 目标环境上的创建动作。
// 创建者至少要看得见目标项目与环境，因此四项缺一不可。
func TestAppCreateAuthResourcesCoverAllDimensions(t *testing.T) {
	const (
		bizID     = uint32(100001)
		projectID = uint32(40001)
		envID     = uint32(50001)
	)

	res := AppCreateAuthResources(bizID, projectID, envID)
	require.Len(t, res, 4)

	for _, r := range res {
		require.Equal(t, bizID, r.BizID, "every resource should carry biz id")
	}

	byType := make(map[ResourceType]*ResourceAttribute, len(res))
	for _, r := range res {
		byType[r.Type] = r
	}

	require.Equal(t, FindBusinessResource, byType[Biz].Action)

	require.NotNil(t, byType[Project], "project view is missing")
	require.Equal(t, View, byType[Project].Action)
	require.Equal(t, projectID, byType[Project].ResourceID)

	require.NotNil(t, byType[Environment], "environment view is missing")
	require.Equal(t, View, byType[Environment].Action)
	require.Equal(t, envID, byType[Environment].ResourceID)

	// 创建时尚无服务实例，服务动作落在目标环境上。
	require.NotNil(t, byType[App], "app create is missing")
	require.Equal(t, Create, byType[App].Action)
	require.Equal(t, envID, byType[App].ResourceID)
	require.Equal(t, projectID, byType[App].ProjectID)
	require.Equal(t, envID, byType[App].EnvironmentID)
}

func TestAppAuthResourcesCoverAllDimensions(t *testing.T) {
	const (
		bizID     = uint32(100001)
		projectID = uint32(40001)
		envID     = uint32(50001)
		appID     = uint32(60001)
	)

	tests := []struct {
		name        string
		res         []*ResourceAttribute
		appActions  []Action
		expectCount int
	}{
		{
			name:        "view",
			res:         AppAuthResources(bizID, projectID, envID, appID, View),
			appActions:  []Action{View},
			expectCount: 4,
		},
		{
			name:        "write",
			res:         AppAuthResources(bizID, projectID, envID, appID, Update),
			appActions:  []Action{Update},
			expectCount: 4,
		},
		{
			name:        "generate release and publish",
			res:         AppAuthResources(bizID, projectID, envID, appID, GenerateRelease, Publish),
			appActions:  []Action{GenerateRelease, Publish},
			expectCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Len(t, tt.res, tt.expectCount)

			var bizRes, projectRes, envRes *ResourceAttribute
			appActions := make([]Action, 0, len(tt.appActions))
			for _, res := range tt.res {
				require.Equal(t, bizID, res.BizID, "every resource should carry biz id")

				switch res.Type {
				case Biz:
					bizRes = res
				case Project:
					projectRes = res
				case Environment:
					envRes = res
				case App:
					require.Equal(t, appID, res.ResourceID)
					appActions = append(appActions, res.Action)
				default:
					t.Fatalf("unexpected resource type %s", res.Type)
				}
			}

			require.NotNil(t, bizRes, "business access is the precondition of every api")
			require.Equal(t, FindBusinessResource, bizRes.Action)

			require.NotNil(t, projectRes, "project view is missing")
			require.Equal(t, View, projectRes.Action)
			require.Equal(t, projectID, projectRes.ResourceID)
			require.Equal(t, projectID, projectRes.ProjectID)

			require.NotNil(t, envRes, "environment view is missing")
			require.Equal(t, View, envRes.Action)
			require.Equal(t, envID, envRes.ResourceID)

			require.Equal(t, tt.appActions, appActions, "app actions must be checked on the app instance")
		})
	}
}
