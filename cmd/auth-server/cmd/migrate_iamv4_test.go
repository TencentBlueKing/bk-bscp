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

package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// 租户 ID 必须落在 kit 上，客户端只从 kit 取值填充 X-Bk-Tenant-Id。
func TestNewIAMV4KitCarriesTenantID(t *testing.T) {
	origin := iamV4TenantID
	t.Cleanup(func() { iamV4TenantID = origin })

	iamV4TenantID = "tenant-x"

	kt := newIAMV4Kit()
	require.Equal(t, "tenant-x", kt.TenantID)
	require.NotNil(t, kt.Ctx)
}

// 模型注册命令都得能指定租户；默认值留空，由租户模式决定，避免多租户环境带上 IAM 不认的 default。
func TestModelCommandsRegisterTenantFlag(t *testing.T) {
	for _, sub := range []*cobra.Command{
		migrateInitCmd, migrateInitIAMV4Cmd, migrateDiffIAMV4Cmd, migratePruneIAMV4Cmd,
	} {
		flag := sub.Flags().Lookup("tenant-id")
		require.NotNilf(t, flag, "command %q should register --tenant-id", sub.Use)
		require.Emptyf(t, flag.DefValue, "command %q should leave --tenant-id empty by default", sub.Use)
	}
}
