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

package pbtb

import (
	"testing"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

// TestPbTaskBatchPassThrough 断言 PbTaskBatch 将 OperateRange 五段表达式字符串一一透传到 pb，
// 缺省段随存储（AC-003 展示数据）。
func TestPbTaskBatchPassThrough(t *testing.T) {
	td := &table.TaskExecutionData{
		Environment: "1",
		OperateRange: table.OperateRange{
			SetName:      "[管控平台,PaaS平台]",
			ModuleName:   "*",
			ServiceName:  "*",
			ProcessAlias: "*",
			ProcessID:    "4[6,8,9]",
		},
	}
	tb := &table.TaskBatch{
		ID: 1,
		Spec: &table.TaskBatchSpec{
			TaskObject: table.TaskObjectProcess,
			TaskAction: table.TaskActionStart,
			TaskData:   td.String(),
			Status:     table.TaskBatchStatusRunning,
		},
	}

	got := PbTaskBatch(tb)
	if got == nil || got.TaskData == nil || got.TaskData.OperateRange == nil {
		t.Fatalf("PbTaskBatch returned nil task data: %+v", got)
	}
	or := got.TaskData.OperateRange
	if or.SetName != "[管控平台,PaaS平台]" || or.ModuleName != "*" || or.ServiceName != "*" ||
		or.ProcessAlias != "*" || or.ProcessId != "4[6,8,9]" {
		t.Fatalf("OperateRange not passed through: %+v", or)
	}
	if got.TaskData.Environment != "1" {
		t.Fatalf("environment = %q, want 1", got.TaskData.Environment)
	}
}
