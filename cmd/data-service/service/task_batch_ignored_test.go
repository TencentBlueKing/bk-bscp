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

	taskTypes "github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// TestConvertTaskStatusIgnored IGNORED 为任务框架原生终态，展示状态直接透传，
// 不得落入未知状态兜底分支被误判为 FAILURE。
func TestConvertTaskStatusIgnored(t *testing.T) {
	if got := convertTaskStatus(taskTypes.TaskStatusIgnored); got != TaskStatusIgnored {
		t.Fatalf("convertTaskStatus(IGNORED) = %s, want IGNORED", got)
	}
}

// TestExpandTaskStatusForQueryIgnored IGNORED 查询直接映射为框架原生状态，
// 存储层可按其过滤与分页，无需内存转译。
func TestExpandTaskStatusForQueryIgnored(t *testing.T) {
	got := expandTaskStatusForQuery(TaskStatusIgnored)
	if len(got) != 1 || got[0] != taskTypes.TaskStatusIgnored {
		t.Fatalf("expandTaskStatusForQuery(IGNORED) = %v, want [IGNORED]", got)
	}
}

// TestConvertTaskToDetailIgnored 框架 IGNORED 任务直接透传展示状态，
// 不再依赖 GsePayload.ErrorCode 转译判定。
func TestConvertTaskToDetailIgnored(t *testing.T) {
	kt := &kit.Kit{Ctx: context.Background()}
	task := &taskTypes.Task{
		TaskID:  "task-ignored-01",
		Status:  taskTypes.TaskStatusIgnored,
		Creator: "tester",
		Message: "task finished with ignored steps",
		CommonPayload: `{"gsePayload":{"errorCode":828,"errorMsg":"already running"},` +
			`"processPayload":{"alias":"demo","funcName":"run"}}`,
	}

	detail, err := convertTaskToDetail(kt, task)
	if err != nil {
		t.Fatalf("convertTaskToDetail err: %v", err)
	}
	if detail.Status != TaskStatusIgnored {
		t.Fatalf("detail.Status = %s, want IGNORED", detail.Status)
	}
	if detail.Message != task.Message {
		t.Fatalf("detail.Message = %s, want %s", detail.Message, task.Message)
	}
}
