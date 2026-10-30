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

	"github.com/stretchr/testify/require"
)

// migrate 的 --debug 是持久化 flag，子命令也应能注册自己的 flag。
func TestMigrateDeleteIAMFlags(t *testing.T) {
	require.NotNil(t, migrateDeleteIAMCmd.Flags().Lookup("yes"))
	require.NotNil(t, migrateDeleteIAMCmd.Flags().Lookup("tenant-id"))
	require.NotNil(t, migrateCmd.PersistentFlags().Lookup("debug"))
}

// V4 的清空子命令与 V3 对齐：既能指定租户，也能在非交互环境跳过确认。
func TestMigrateDeleteIAMV4Flags(t *testing.T) {
	require.NotNil(t, migrateDeleteIAMV4Cmd.Flags().Lookup("yes"))
	require.NotNil(t, migrateDeleteIAMV4Cmd.Flags().Lookup("tenant-id"))
	require.Equal(t, "delete-iam-v4", migrateDeleteIAMV4Cmd.Use)

	found := false
	for _, sub := range migrateCmd.Commands() {
		if sub.Name() == "delete-iam-v4" {
			found = true
		}
	}
	require.True(t, found, "delete-iam-v4 must be registered under migrate")
}
