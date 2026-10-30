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
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	iamauth "github.com/TencentBlueKing/bk-bscp/internal/iam/auth"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbapp "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/app"
)

// fakeAnnotationAuthorizer 只实现 AuthorizeDecision，其余方法由嵌入的接口兜底。
type fakeAnnotationAuthorizer struct {
	iamauth.Authorizer

	// allow 按资源顺序返回判定结果
	allow []bool
	seen  []*meta.ResourceAttribute
}

func (f *fakeAnnotationAuthorizer) AuthorizeDecision(_ *kit.Kit,
	resources ...*meta.ResourceAttribute) ([]*meta.Decision, bool, error) {

	decisions := make([]*meta.Decision, len(resources))
	f.seen = append(f.seen[:0], resources...)
	for idx, res := range resources {
		// 模拟鉴权侧回传的资源：服务资源会被补齐 project_id/environment_id，
		// 与调用方传入的属性不再相等。
		echo := *res
		echo.ProjectID = 40099
		echo.EnvironmentID = 50099

		authorized := idx < len(f.allow) && f.allow[idx]
		decisions[idx] = &meta.Decision{Resource: &echo, Authorized: authorized}
	}

	return decisions, true, nil
}

// 有权限的服务必须按顺序映射回各自的权限注解，
// 不能因为鉴权侧回填了项目/环境归属就整体判为无权限。
func TestListAppsAnnotationAlignsByIndex(t *testing.T) {
	auth := &fakeAnnotationAuthorizer{
		// 每个服务 5 个操作：view/edit/delete/publish/generate
		allow: []bool{
			true, true, true, true, true,
			false, false, false, false, false,
		},
	}

	resp := &pbcs.ListAppsResp{Details: []*pbapp.App{
		{Id: 30001, BizId: 100001, ProjectId: 40001, EnvId: 50001},
		{Id: 30002, BizId: 100001, ProjectId: 40001, EnvId: 50001},
	}}

	annotation, err := ListAppsAnnotation(context.Background(), &kit.Kit{}, auth, resp)
	require.NoError(t, err)

	require.True(t, annotation.Perms["30001"][string(meta.View)])
	require.True(t, annotation.Perms["30001"][string(meta.Update)])
	require.False(t, annotation.Perms["30002"][string(meta.View)])
	require.False(t, annotation.Perms["30002"][string(meta.Delete)])
	require.Len(t, auth.seen, 10)
	for _, resource := range auth.seen {
		require.Equal(t, uint32(100001), resource.BizID)
		require.Equal(t, uint32(40001), resource.ProjectID)
		require.Equal(t, uint32(50001), resource.EnvironmentID)
	}
}
