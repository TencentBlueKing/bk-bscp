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
	"testing"

	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

// TestIsIdempotentOperateError Operate 幂等识别：828/829 视为幂等成功，其余错误码照旧失败
func TestIsIdempotentOperateError(t *testing.T) {
	cases := []struct {
		name       string
		errorCode  int
		wantIgnore bool
	}{
		{"重复启动 828 -> 幂等成功", gse.ErrCodeAlreadyRunning, true},
		{"无需停止 829 -> 幂等成功", gse.ErrCodeNoNeedStop, true},
		{"成功 0 -> 非幂等路径", 0, false},
		{"仍在执行 115 -> 非幂等路径", 115, false},
		{"其他失败码 1 -> 照旧失败", 1, false},
		{"负数错误码 -> 照旧失败", -1, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsIdempotentOperateError(c.errorCode); got != c.wantIgnore {
				t.Fatalf("IsIdempotentOperateError(%d) = %v, want %v", c.errorCode, got, c.wantIgnore)
			}
		})
	}
}

// TestCompareExecConfig CompareWithCMDBProcessInfo 对比决策：
// 一致放行；不一致报错；停止操作且 CMDB 快照缺失回退 DB 配置放行；其他操作快照缺失报错
func TestCompareExecConfig(t *testing.T) {
	dbConfig := table.ProcessInfo{
		StartCmd: "/usr/bin/start",
		StopCmd:  "/usr/bin/stop",
		User:     "root",
	}
	latestSame := table.ProcessInfo{
		StartCmd: "/usr/bin/start",
		StopCmd:  "/usr/bin/stop",
		User:     "root",
	}
	latestDiff := table.ProcessInfo{
		StartCmd: "/usr/bin/start-v2",
		StopCmd:  "/usr/bin/stop-v2",
		User:     "root",
	}

	cases := []struct {
		name        string
		operateType table.ProcessOperateType
		db          table.ProcessInfo
		latest      table.ProcessInfo
		hasLatest   bool
		wantErr     bool
	}{
		{"新旧一致 -> start 放行", table.StartProcessOperate, dbConfig, latestSame, true, false},
		{"新旧一致 -> stop 放行", table.StopProcessOperate, dbConfig, latestSame, true, false},
		{"新旧一致 -> kill 放行", table.KillProcessOperate, dbConfig, latestSame, true, false},
		{"新旧不一致 -> start 报错", table.StartProcessOperate, dbConfig, latestDiff, true, true},
		{"新旧不一致 -> stop 报错", table.StopProcessOperate, dbConfig, latestDiff, true, true},
		{"新旧不一致 -> restart 报错", table.RestartProcessOperate, dbConfig, latestDiff, true, true},
		{"快照缺失 + stop -> 回退 DB 配置放行", table.StopProcessOperate, dbConfig, table.ProcessInfo{}, false, false},
		{"快照缺失 + start -> 报错", table.StartProcessOperate, dbConfig, table.ProcessInfo{}, false, true},
		{"快照缺失 + kill -> 报错", table.KillProcessOperate, dbConfig, table.ProcessInfo{}, false, true},
		{"快照缺失 + unregister -> 报错", table.UnregisterProcessOperate, dbConfig, table.ProcessInfo{}, false, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := compareExecConfig(c.operateType, c.db, c.latest, c.hasLatest)
			if (err != nil) != c.wantErr {
				t.Fatalf("compareExecConfig(%s, ...) err = %v, wantErr %v", c.operateType, err, c.wantErr)
			}
		})
	}
}
