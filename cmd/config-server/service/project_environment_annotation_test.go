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

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbenvironment "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/environment"
	pbproject "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/project"
)

// 项目列表按项目实例返回逐项权限：有权限的项目不能被无权限项目带偏，
// 否则前端会把整页项目都置灰。
func TestListProjectsAnnotation(t *testing.T) {
	auth := &fakeAnnotationAuthorizer{
		// 每个项目 3 个操作：view/edit/delete
		allow: []bool{
			true, true, true,
			false, false, false,
		},
	}

	resp := &pbcs.ListProjectsResp{Projects: []*pbproject.Project{
		{Id: 40001, Attachment: &pbproject.ProjectAttachment{BizId: 100001}},
		{Id: 40002, Attachment: &pbproject.ProjectAttachment{BizId: 100001}},
	}}

	annotation, err := ListProjectsAnnotation(context.Background(), &kit.Kit{}, auth, resp)
	require.NoError(t, err)

	require.True(t, annotation.Perms["40001"][string(meta.View)])
	require.True(t, annotation.Perms["40001"][string(meta.Delete)])
	require.False(t, annotation.Perms["40002"][string(meta.View)])
	require.False(t, annotation.Perms["40002"][string(meta.Update)])
}

// 环境列表按环境实例返回逐项权限：无权限环境仍需返回给前端置灰展示，
// 因此注解必须覆盖返回的每一个环境，而不是只覆盖有权限的那些。
func TestListEnvironmentsAnnotation(t *testing.T) {
	auth := &fakeAnnotationAuthorizer{
		// 每个环境 3 个操作：view/edit/delete
		allow: []bool{
			true, false, false,
			false, false, false,
		},
	}

	resp := &pbcs.ListEnvironmentsResp{
		ProdEnvironments: []*pbenvironment.Environment{
			{Id: 50001, Attachment: &pbenvironment.EnvironmentAttachment{BizId: 100001, ProjectId: 40001}},
		},
		DevEnvironments: []*pbenvironment.Environment{
			{Id: 50002, Attachment: &pbenvironment.EnvironmentAttachment{BizId: 100001, ProjectId: 40001}},
		},
	}

	annotation, err := ListEnvironmentsAnnotation(context.Background(), &kit.Kit{}, auth, resp)
	require.NoError(t, err)

	require.True(t, annotation.Perms["50001"][string(meta.View)])
	require.False(t, annotation.Perms["50001"][string(meta.Update)])
	require.False(t, annotation.Perms["50002"][string(meta.View)])
}

// 空列表不触发鉴权调用，也不应该返回 nil 注解，避免前端拿不到 perms 字段。
func TestListAnnotationWithEmptyList(t *testing.T) {
	auth := &fakeAnnotationAuthorizer{}

	projectAnnotation, err := ListProjectsAnnotation(
		context.Background(), &kit.Kit{}, auth, &pbcs.ListProjectsResp{})
	require.NoError(t, err)
	require.NotNil(t, projectAnnotation)
	require.Empty(t, projectAnnotation.Perms)

	envAnnotation, err := ListEnvironmentsAnnotation(
		context.Background(), &kit.Kit{}, auth, &pbcs.ListEnvironmentsResp{})
	require.NoError(t, err)
	require.NotNil(t, envAnnotation)
	require.Empty(t, envAnnotation.Perms)
}
