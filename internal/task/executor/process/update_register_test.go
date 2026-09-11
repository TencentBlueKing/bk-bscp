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

package process

import (
	"errors"
	"fmt"
	"testing"

	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

// TestParseProcessConfigs 快照配置解析：快照缺失（CMDB 已删除 / 下发降级）即报错
func TestParseProcessConfigs(t *testing.T) {
	dbJSON := `{"start_cmd":"/usr/bin/start","stop_cmd":"/usr/bin/stop"}`
	latestJSON := `{"start_cmd":"/usr/bin/start-v2","stop_cmd":"/usr/bin/stop"}`

	cases := []struct {
		name          string
		configData    string
		latestData    string
		wantErr       bool
		wantLatestCmd string
	}{
		{"双配置齐全 -> 解析成功", dbJSON, latestJSON, false, "/usr/bin/start-v2"},
		{"快照缺失 -> 报错", dbJSON, "", true, ""},
		{"DB 配置非法 JSON -> 报错", "{bad", latestJSON, true, ""},
		{"快照非法 JSON -> 报错", dbJSON, "{bad", true, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			payload := &common.TaskPayload{
				ProcessPayload: &common.ProcessPayload{
					CcProcessID:      42,
					ConfigData:       c.configData,
					LatestConfigData: c.latestData,
				},
			}

			dbInfo, latestInfo, err := parseProcessConfigs(payload)
			if (err != nil) != c.wantErr {
				t.Fatalf("parseProcessConfigs() err = %v, wantErr %v", err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			if dbInfo.StartCmd != "/usr/bin/start" {
				t.Fatalf("dbInfo.StartCmd = %q, want /usr/bin/start", dbInfo.StartCmd)
			}
			if latestInfo.StartCmd != c.wantLatestCmd {
				t.Fatalf("latestInfo.StartCmd = %q, want %q", latestInfo.StartCmd, c.wantLatestCmd)
			}
		})
	}
}

// TestProcessInfoChanged 配置对比：一致无需更新托管，不一致才需要执行
func TestProcessInfoChanged(t *testing.T) {
	base := table.ProcessInfo{
		StartCmd: "/usr/bin/start",
		StopCmd:  "/usr/bin/stop",
		User:     "root",
	}
	same := table.ProcessInfo{
		StartCmd: "/usr/bin/start",
		StopCmd:  "/usr/bin/stop",
		User:     "root",
	}
	diff := table.ProcessInfo{
		StartCmd: "/usr/bin/start-v2",
		StopCmd:  "/usr/bin/stop",
		User:     "root",
	}

	if processInfoChanged(base, same) {
		t.Fatalf("processInfoChanged(base, same) = true, want false")
	}
	if !processInfoChanged(base, diff) {
		t.Fatalf("processInfoChanged(base, diff) = false, want true")
	}
}

// TestRegisterProcessSuccessDelta 成功数增量语义：
// 任务成功或失败发生在托管注册之后（启动 / 收尾）计 1；注册之前或注册本身失败计 0
func TestRegisterProcessSuccessDelta(t *testing.T) {
	cases := []struct {
		name      string
		cbErr     error
		wantDelta uint32
	}{
		{"任务成功 -> 增量 1", nil, 1},
		{"注册步骤失败 -> 增量 0", fmt.Errorf("%w: gse operate failed", ErrRegisterProcessStepFailed), 0},
		{"停止步骤失败（注册未执行）-> 增量 0", errors.New("execute process operate stop failed"), 0},
		{"校验步骤失败（注册未执行）-> 增量 0", errors.New("process cannot operate"), 0},
		{"启动步骤失败（注册已执行）-> 增量 1", fmt.Errorf("%w: gse operate failed", ErrStartProcessStepFailed), 1},
		{"收尾步骤失败（注册已执行）-> 增量 1", fmt.Errorf("%w: db failed", ErrOperationCompletedStepFailed), 1},
		{"收尾 GSE 状态查询失败（注册已执行）-> 增量 1",
			fmt.Errorf("[OperationCompletedStep STEP]: %w: get gse process status failed: %v",
				ErrOperationCompletedStepFailed, "gse timeout"), 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := registerProcessSuccessDelta(c.cbErr); got != c.wantDelta {
				t.Fatalf("registerProcessSuccessDelta(%v) = %d, want %d", c.cbErr, got, c.wantDelta)
			}
		})
	}
}
