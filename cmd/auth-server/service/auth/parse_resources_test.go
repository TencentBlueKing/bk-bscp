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
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// parseResources 只会按入参资源生成对应的 action 集合，
// 不会因为 credential 挂在业务资源上就额外补出业务查看 action。
// 这保证上层 Authorize 过滤掉已授权的业务资源后，申请信息不会再次出现业务访问项。
func TestParseResourcesDoesNotReintroduceBiz(t *testing.T) {
	a := &Auth{projectEnvAuth: true}

	cred := &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.Credential, Action: meta.View}, BizID: 1,
	}

	_, permissionMap, err := a.parseResources(kit.New(), []*meta.ResourceAttribute{cred})

	require.NoError(t, err)
	require.Contains(t, permissionMap, sys.CredentialView)
	require.NotContains(t, permissionMap, sys.BusinessViewResource)
}

// 项目和环境的名称不在 CMDB 里，必须由 data-service 按实例 ID 回查，
// 否则未授权的项目/环境会以空名称出现在申请信息里。
func TestGetInstIDNameMapResolvesProjectAndEnvironment(t *testing.T) {
	ds := &fakeAppDS{
		projectInfos: map[uint32]*pbds.InstanceInfo{
			40001: {Id: "40001", DisplayName: "默认项目"},
		},
		environmentInfos: map[uint32]*pbds.InstanceInfo{
			50001: {Id: "50001", DisplayName: "测试环境"},
		},
	}
	a := newAppScopeAuth(ds)

	nameMap, err := a.getInstIDNameMap(testKit(), map[client.TypeID][]string{
		sys.Project:     {"40001"},
		sys.Environment: {"50001"},
	})

	require.NoError(t, err)
	require.Equal(t, "默认项目", nameMap[instanceNameKey(sys.Project, "40001")])
	require.Equal(t, "测试环境", nameMap[instanceNameKey(sys.Environment, "50001")])
}

// 项目与环境的实例 ID 各自独立编号，同号实例不能互相覆盖名称。
func TestGetInstIDNameMapSeparatesSameIDByResourceType(t *testing.T) {
	ds := &fakeAppDS{
		projectInfos: map[uint32]*pbds.InstanceInfo{
			7: {Id: "7", DisplayName: "项目7"},
		},
		environmentInfos: map[uint32]*pbds.InstanceInfo{
			7: {Id: "7", DisplayName: "环境7"},
		},
	}
	a := newAppScopeAuth(ds)

	nameMap, err := a.getInstIDNameMap(testKit(), map[client.TypeID][]string{
		sys.Project:     {"7"},
		sys.Environment: {"7"},
	})

	require.NoError(t, err)
	require.Equal(t, "项目7", nameMap[instanceNameKey(sys.Project, "7")])
	require.Equal(t, "环境7", nameMap[instanceNameKey(sys.Environment, "7")])
}

// 申请信息里的每条实例链都要带上名称：项目、环境节点空名会让前端展示空白的资源实例。
// 回查不到名称的实例留空即可，不能影响申请链接的生成。
func TestAssignInstanceNamesFillsEveryInstance(t *testing.T) {
	iamResourceType := &meta.IamResourceType{
		Type: string(sys.Application),
		Instances: [][]*meta.IamResourceInstance{
			{
				{Type: string(sys.Business), ID: "100001"},
				{Type: string(sys.Project), ID: "40001"},
				{Type: string(sys.Environment), ID: "50001"},
				{Type: string(sys.Application), ID: "30001"},
			},
			{
				{Type: string(sys.Business), ID: "200002"},
				{Type: string(sys.Application), ID: "30002"},
			},
		},
	}

	assignInstanceNames(testKit(), iamResourceType, map[string]string{
		instanceNameKey(sys.Business, "100001"):   "蓝鲸",
		instanceNameKey(sys.Project, "40001"):     "默认项目",
		instanceNameKey(sys.Environment, "50001"): "测试环境",
		instanceNameKey(sys.Application, "30001"): "demo-app",
	})

	instances := iamResourceType.Instances[0]
	require.Equal(t, "蓝鲸", instances[0].Name)
	require.Equal(t, "默认项目", instances[1].Name)
	require.Equal(t, "测试环境", instances[2].Name)
	require.Equal(t, "demo-app", instances[3].Name)

	// 回查不到名称的实例保持空名称，不影响其余实例。
	require.Empty(t, iamResourceType.Instances[1][0].Name)
	require.Empty(t, iamResourceType.Instances[1][1].Name)
}
