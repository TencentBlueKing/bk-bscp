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

package gse

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// getTestConfig 从环境变量获取测试配置
func getGseService(t *testing.T) *Service {
	host := os.Getenv("GSE_HOST")
	appCode := os.Getenv("BK_APP_CODE")
	appSecret := os.Getenv("BK_APP_SECRET")

	if host == "" || appCode == "" || appSecret == "" {
		t.Skip("Skipping test: GSE_HOST, BK_APP_CODE, or BK_APP_SECRET environment variables not set")
	}

	return NewService(appCode, appSecret, host)
}

// TestOperateProcMulti 测试批量进程操作
func TestOperateProcMulti(t *testing.T) {
	gseService := getGseService(t)

	ctx := context.Background()

	// 构造测试请求
	req := &MultiProcOperateReq{
		ProcOperateReq: []ProcessOperate{
			{
				Meta: ProcessMeta{
					Namespace: "test-namespace",
					Name:      "test-process",
					Labels: map[string]string{
						"env": "test",
						"app": "bscp",
					},
				},
				OpType: OpTypeQuery,
				Spec: ProcessSpec{
					Identity: ProcessIdentity{
						ProcName:   "test-proc",
						SetupPath:  "/data/test",
						PidPath:    "/data/test/test-proc.pid",
						ConfigPath: "/data/test/config.yaml",
						LogPath:    "/data/test/logs",
						User:       "root",
					},
					Control: ProcessControl{
						StartCmd:   "./test-proc start",
						StopCmd:    "./test-proc stop",
						RestartCmd: "./test-proc restart",
						ReloadCmd:  "./test-proc reload",
						KillCmd:    "pkill test-proc",
						VersionCmd: "./test-proc version",
						HealthCmd:  "./test-proc health",
					},
					Resource: ProcessResource{
						CPU: 30.0, // CPU 限制 30%
						Mem: 10.0, // 内存限制 10%
					},
					MonitorPolicy: ProcessMonitorPolicy{
						AutoType:       1,  // 1=常驻进程，2=单次执行进程
						StartCheckSecs: 5,  // 启动后检查存活的时间（秒）
						StopCheckSecs:  5,  // 停止后检查存活的时间（秒）
						OpTimeout:      60, // 命令执行超时时间（秒）
					},
				},
			},
		},
	}

	// 执行测试
	resp, err := gseService.OperateProcMulti(ctx, req)

	// 断言
	require.NoError(t, err, "OperateProcMulti should not return error")
	require.NotNil(t, resp, "Response should not be nil")

	t.Logf("OperateProcMulti success, TaskID: %s", resp.TaskID)
}
