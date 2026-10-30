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

package options

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// migrate 子命令的 flags 走 cobra 解析，全局 flag set 必须放行未注册的 flag；
// 常驻服务启动则保持严格解析。
func TestHasMigrateCommand(t *testing.T) {
	require.True(t, hasMigrateCommand([]string{"migrate", "delete-iam", "--yes"}))
	require.True(t, hasMigrateCommand([]string{"-c", "/path/auth_server.yaml", "migrate", "init-iam"}))

	require.False(t, hasMigrateCommand(nil))
	require.False(t, hasMigrateCommand([]string{"-c", "/path/auth_server.yaml"}))
	require.False(t, hasMigrateCommand([]string{"--port", "9613"}))
}
