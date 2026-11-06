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

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// parseResources 只会按入参资源生成对应的 action 集合，
// 不会因为 credential 挂在业务资源上就额外补出业务查看 action。
// 这保证上层 Authorize 过滤掉已授权的业务资源后，申请信息不会再次出现业务访问项。
func TestParseResourcesDoesNotReintroduceBiz(t *testing.T) {
	a := &Auth{projectEnvAuth: true, topology: meta.TopologyHierarchical}

	cred := &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.Credential, Action: meta.View}, BizID: 1,
	}

	_, permissionMap, err := a.parseResources(kit.New(), []*meta.ResourceAttribute{cred})

	require.NoError(t, err)
	require.Contains(t, permissionMap, sys.CredentialView)
	require.NotContains(t, permissionMap, sys.BusinessViewResource)
}
