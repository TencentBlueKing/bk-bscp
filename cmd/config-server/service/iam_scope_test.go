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

// 配置项、KV、版本、上线共用一条规则：业务访问 + 项目查看 + 环境查看 + 服务动作，
// 四项必须同时提交。任意一处自行拼装或漏掉项目/环境维度，都会出现
// "能改配置项却读不到 KV""能生成版本却上不了线"这类互相矛盾的权限结果。
func TestAppAuthResourcesCoverAllDimensions(t *testing.T) {
	const (
		bizID     = uint32(100001)
		projectID = uint32(40001)
		envID     = uint32(50001)
		appID     = uint32(60001)
	)

	tests := []struct {
		name        string
		res         []*meta.ResourceAttribute
		appActions  []meta.Action
		expectCount int
	}{
		{
			name:        "view",
			res:         appViewAuthResources(bizID, projectID, envID, appID),
			appActions:  []meta.Action{meta.View},
			expectCount: 4,
		},
		{
			name:        "write",
			res:         appWriteAuthResources(bizID, projectID, envID, appID),
			appActions:  []meta.Action{meta.Update},
			expectCount: 4,
		},
		{
			name:        "generate release and publish",
			res:         appAuthResources(bizID, projectID, envID, appID, meta.GenerateRelease, meta.Publish),
			appActions:  []meta.Action{meta.GenerateRelease, meta.Publish},
			expectCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Len(t, tt.res, tt.expectCount)

			var bizRes, projectRes, envRes *meta.ResourceAttribute
			appActions := make([]meta.Action, 0, len(tt.appActions))
			for _, res := range tt.res {
				require.Equal(t, bizID, res.BizID, "every resource should carry biz id")

				switch res.Type {
				case meta.Biz:
					bizRes = res
				case meta.Project:
					projectRes = res
				case meta.Environment:
					envRes = res
				case meta.App:
					require.Equal(t, appID, res.ResourceID)
					appActions = append(appActions, res.Action)
				default:
					t.Fatalf("unexpected resource type %s", res.Type)
				}
			}

			require.NotNil(t, bizRes, "business access is the precondition of every api")
			require.Equal(t, meta.FindBusinessResource, bizRes.Action)

			require.NotNil(t, projectRes, "project view is missing")
			require.Equal(t, meta.View, projectRes.Action)
			require.Equal(t, projectID, projectRes.ResourceID)

			require.NotNil(t, envRes, "environment view is missing")
			require.Equal(t, meta.View, envRes.Action)
			require.Equal(t, envID, envRes.ResourceID)

			require.Equal(t, tt.appActions, appActions, "app actions must be checked on the app instance")
		})
	}
}
