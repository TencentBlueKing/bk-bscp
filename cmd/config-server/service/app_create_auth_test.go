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
	"google.golang.org/grpc"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// 创建与克隆的服务鉴权项相同：克隆就是在目标环境里新建服务，鉴权是业务访问 +
// 目标项目/环境的查看权限 + 目标环境上的创建权限；克隆请求不带源服务实例，
// 不额外校验源服务的 app_edit。
func TestAppCreateAuthResources(t *testing.T) {
	res := appCreateAuthResources(100001, 40001, 50001)
	require.Len(t, res, 4)

	byType := make(map[meta.ResourceType]*meta.ResourceAttribute, len(res))
	for _, r := range res {
		byType[r.Type] = r
		require.Equal(t, uint32(100001), r.BizID, "all resources should carry biz id")
	}

	bizRes := byType[meta.Biz]
	require.NotNil(t, bizRes)
	require.Equal(t, meta.FindBusinessResource, bizRes.Action)

	// 创建者至少要看得见目标项目与环境，不允许只凭创建授权盲建。
	projectRes := byType[meta.Project]
	require.NotNil(t, projectRes, "project view is required to create an app")
	require.Equal(t, meta.View, projectRes.Action)
	require.Equal(t, uint32(40001), projectRes.ResourceID)

	envRes := byType[meta.Environment]
	require.NotNil(t, envRes, "environment view is required to create an app")
	require.Equal(t, meta.View, envRes.Action)
	require.Equal(t, uint32(50001), envRes.ResourceID)

	appRes := byType[meta.App]
	require.NotNil(t, appRes)
	require.Equal(t, meta.Create, appRes.Action)
	// 创建时尚无服务实例，ResourceID 是目标环境 ID：授权维度落在服务将要归属的环境上。
	require.Equal(t, uint32(50001), appRes.ResourceID, "app create is granted on the target environment")
	require.Equal(t, uint32(40001), appRes.ProjectID, "project ownership is needed to build the iam path")
	require.Equal(t, uint32(50001), appRes.EnvironmentID)
}

// 服务列表的两个入口共用同一条规则：业务访问 + 项目查看 + 环境查看。
// 服务挂在项目/环境下，缺其中任一项就不返回该范围内的服务；
// 服务维度的逐项动作权限仍由列表注解带回前端置灰并引导申请。
func TestAppListAuthResources(t *testing.T) {
	res := appListAuthResources(100001, 40001, 50001)
	require.Len(t, res, 3)

	require.Equal(t, meta.Biz, res[0].Type)
	require.Equal(t, meta.FindBusinessResource, res[0].Action)
	require.Equal(t, uint32(100001), res[0].BizID)

	require.Equal(t, meta.Project, res[1].Type)
	require.Equal(t, meta.View, res[1].Action)
	require.Equal(t, uint32(40001), res[1].ResourceID)

	require.Equal(t, meta.Environment, res[2].Type)
	require.Equal(t, meta.View, res[2].Action)
	require.Equal(t, uint32(50001), res[2].ResourceID)

	// 未声明环境时环境项不删减：归属为 0 由 auth-server 按业务默认环境补齐，
	// 与单个服务接口的规则一致，入口内不做特殊处理。
	res = appListAuthResources(100001, 40001, 0)
	require.Len(t, res, 3)
	require.Equal(t, meta.Environment, res[2].Type)
	require.Zero(t, res[2].ResourceID)
}

// fakeAppCloneDataClient 只实现克隆用例涉及的 data-service 接口。
type fakeAppCloneDataClient struct {
	pbds.DataClient
	cloneResp *pbds.CreateResp
	cloneErr  error
}

func (f *fakeAppCloneDataClient) CloneApp(_ context.Context, _ *pbds.CloneAppReq,
	_ ...grpc.CallOption) (*pbds.CreateResp, error) {

	return f.cloneResp, f.cloneErr
}

// 克隆同样产生新的服务实例，必须把该实例的运维权限授予操作者，
// 否则用户自己克隆出来的服务自己也无权管理。
func TestCloneAppGrantsCreatorAction(t *testing.T) {
	ds := &fakeAppCloneDataClient{cloneResp: &pbds.CreateResp{Id: 60001}}
	authorizer := new(fakeCreatorAuthorizer)
	svc := &Service{client: &ClientSet{DS: ds}, authorizer: authorizer}

	resp, err := svc.CloneApp(projectGRPCContext("alice"), &pbcs.CloneAppReq{
		BizId:     2,
		Name:      "cloned_app",
		ProjectId: 40001,
		EnvId:     50001,
	})
	require.NoError(t, err)
	require.Equal(t, uint32(60001), resp.GetId())

	require.Len(t, authorizer.grants, 1, "cloned app should grant creator action once")
	grant := authorizer.grants[0]
	require.Equal(t, sys.Application, grant.Type)
	require.Equal(t, "60001", grant.ID)
	require.Equal(t, "cloned_app", grant.Name)
	require.Equal(t, "alice", grant.Creator)
	require.Equal(t, []client.GrantResourceCreatorActionAncestor{
		{System: sys.SystemIDCMDB, Type: sys.Business, ID: "2"},
		{System: sys.SystemIDBSCP, Type: sys.Project, ID: "40001"},
		{System: sys.SystemIDBSCP, Type: sys.Environment, ID: "50001"},
	}, grant.Ancestors)
}
