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
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	iamauth "github.com/TencentBlueKing/bk-bscp/internal/iam/auth"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbenvironment "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/environment"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// fakeProjectDataClient 只实现创建项目用例涉及的 data-service 接口。
type fakeProjectDataClient struct {
	pbds.DataClient
	createResp *pbds.CreateResp
	envResp    *pbds.GetDefaultEnvironmentResp
	envErr     error
	envCalls   int
}

func (f *fakeProjectDataClient) CreateProject(_ context.Context, _ *pbds.CreateProjectReq,
	_ ...grpc.CallOption) (*pbds.CreateResp, error) {

	return f.createResp, nil
}

func (f *fakeProjectDataClient) GetDefaultEnvironment(_ context.Context, _ *pbds.GetDefaultEnvironmentReq,
	_ ...grpc.CallOption) (*pbds.GetDefaultEnvironmentResp, error) {

	f.envCalls++
	return f.envResp, f.envErr
}

// fakeCreatorAuthorizer 记录创建者授权，避免单测依赖真实权限中心。
type fakeCreatorAuthorizer struct {
	iamauth.Authorizer
	grants []*client.GrantResourceCreatorActionOption
}

func (f *fakeCreatorAuthorizer) Authorize(_ *kit.Kit, _ ...*meta.ResourceAttribute) error {
	return nil
}

func (f *fakeCreatorAuthorizer) GrantResourceCreatorAction(_ *kit.Kit,
	opts *client.GrantResourceCreatorActionOption) error {

	f.grants = append(f.grants, opts)
	return nil
}

func projectGRPCContext(user string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		strings.ToLower(constant.UserKey), user,
		strings.ToLower(constant.BkTenantID), "default",
	))
}

// 项目创建者拿到项目权限后，默认环境还要按环境实例单独授权：
// 项目与环境在 IAM 中是两类独立资源，项目权限不会传递到环境。
func TestCreateProjectGrantsDefaultEnvironmentCreatorAction(t *testing.T) {
	ds := &fakeProjectDataClient{
		createResp: &pbds.CreateResp{Id: 40001},
		envResp: &pbds.GetDefaultEnvironmentResp{
			Id:   50001,
			Spec: &pbenvironment.EnvironmentSpec{Name: "Default"},
		},
	}
	authorizer := new(fakeCreatorAuthorizer)
	svc := &Service{client: &ClientSet{DS: ds}, authorizer: authorizer}

	resp, err := svc.CreateProject(projectGRPCContext("alice"), &pbcs.CreateProjectReq{
		BizId: 2,
		Name:  "demo",
	})
	require.NoError(t, err)
	require.Equal(t, uint32(40001), resp.GetId())
	require.Equal(t, 1, ds.envCalls, "default environment should be queried once")

	require.Len(t, authorizer.grants, 2, "project and default environment both need creator grant")

	projectGrant := authorizer.grants[0]
	require.Equal(t, sys.Project, projectGrant.Type)
	require.Equal(t, "40001", projectGrant.ID)
	require.Equal(t, "alice", projectGrant.Creator)

	envGrant := authorizer.grants[1]
	require.Equal(t, sys.Environment, envGrant.Type)
	require.Equal(t, "50001", envGrant.ID)
	require.Equal(t, "Default", envGrant.Name)
	require.Equal(t, "alice", envGrant.Creator, "default environment belongs to the project creator")
	// 环境挂在项目下，祖先链必须带业务与项目，否则权限中心的实例视图校验会拒绝。
	require.Equal(t, []client.GrantResourceCreatorActionAncestor{
		{System: sys.SystemIDCMDB, Type: sys.Business, ID: "2"},
		{System: sys.SystemIDBSCP, Type: sys.Project, ID: "40001"},
	}, envGrant.Ancestors)
}

// 默认环境回查失败时不能影响项目创建：环境授权是补授动作，失败只记日志。
func TestCreateProjectToleratesDefaultEnvQueryFailure(t *testing.T) {
	ds := &fakeProjectDataClient{
		createResp: &pbds.CreateResp{Id: 40001},
		envErr:     errors.New("data service unavailable"),
	}
	authorizer := new(fakeCreatorAuthorizer)
	svc := &Service{client: &ClientSet{DS: ds}, authorizer: authorizer}

	resp, err := svc.CreateProject(projectGRPCContext("alice"), &pbcs.CreateProjectReq{
		BizId: 2,
		Name:  "demo",
	})
	require.NoError(t, err)
	require.Equal(t, uint32(40001), resp.GetId())

	// 只保留项目维度的授权，环境授权跳过。
	require.Len(t, authorizer.grants, 1)
	require.Equal(t, sys.Project, authorizer.grants[0].Type)
}

// 默认环境不存在时没有可授权的实例，同样不阻塞项目创建。
func TestCreateProjectSkipsMissingDefaultEnvironment(t *testing.T) {
	ds := &fakeProjectDataClient{
		createResp: &pbds.CreateResp{Id: 40001},
		envResp:    &pbds.GetDefaultEnvironmentResp{},
	}
	authorizer := new(fakeCreatorAuthorizer)
	svc := &Service{client: &ClientSet{DS: ds}, authorizer: authorizer}

	_, err := svc.CreateProject(projectGRPCContext("alice"), &pbcs.CreateProjectReq{
		BizId: 2,
		Name:  "demo",
	})
	require.NoError(t, err)
	require.Len(t, authorizer.grants, 1)
	require.Equal(t, sys.Project, authorizer.grants[0].Type)
}
