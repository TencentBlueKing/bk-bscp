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

package priority

import (
	"strings"
	"testing"

	taskTypes "github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

func stageNames(stages []*taskTypes.Stage) []string {
	names := make([]string, 0, len(stages))
	for _, stage := range stages {
		names = append(names, stage.Name)
	}
	return names
}

func assertNames(t *testing.T, stages []*taskTypes.Stage, want ...string) {
	t.Helper()

	got := stageNames(stages)
	if len(got) != len(want) {
		t.Fatalf("阶段数量不符, got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("阶段顺序不符, got %v want %v", got, want)
		}
	}
}

func TestBuildStagesStartAscending(t *testing.T) {
	t.Parallel()

	plan := BuildStages([]TaskItem{
		{TaskID: "b", Priority: 2, OpType: table.StartProcessOperate},
		{TaskID: "a", Priority: 1, OpType: table.StartProcessOperate},
		{TaskID: "c", Priority: 3, OpType: table.StartProcessOperate},
	})

	assertNames(t, plan.Stages, "priority-1", "priority-2", "priority-3")
	if plan.StageSeq("a") != 0 || plan.StageSeq("b") != 1 || plan.StageSeq("c") != 2 {
		t.Fatalf("任务与阶段归属不符: %+v", plan.StageSeqOf)
	}
	// 没有托管类任务时首个阶段不应与前一阶段并发
	if plan.Stages[0].StartWithPrevious {
		t.Fatal("首个阶段不应设置 StartWithPrevious")
	}
}

func TestBuildStagesStopDescending(t *testing.T) {
	t.Parallel()

	plan := BuildStages([]TaskItem{
		{TaskID: "a", Priority: 1, OpType: table.StopProcessOperate},
		{TaskID: "c", Priority: 3, OpType: table.StopProcessOperate},
		{TaskID: "b", Priority: 2, OpType: table.StopProcessOperate},
	})

	assertNames(t, plan.Stages, "priority-3", "priority-2", "priority-1")
	if plan.StageSeq("c") != 0 || plan.StageSeq("a") != 2 {
		t.Fatalf("降序归属不符: %+v", plan.StageSeqOf)
	}
}

func TestBuildStagesSamePriorityShareStage(t *testing.T) {
	t.Parallel()

	plan := BuildStages([]TaskItem{
		{TaskID: "a", Priority: 5, OpType: table.StartProcessOperate},
		{TaskID: "b", Priority: 5, OpType: table.StartProcessOperate},
	})

	assertNames(t, plan.Stages, "priority-5")
	if plan.Stages[0].Total != 2 {
		t.Fatalf("同优先级任务应归入同一阶段, total=%d", plan.Stages[0].Total)
	}
}

func TestBuildStagesManagedOperationsRunConcurrently(t *testing.T) {
	t.Parallel()

	plan := BuildStages([]TaskItem{
		{TaskID: "reg", OpType: table.RegisterProcessOperate},
		{TaskID: "unreg", OpType: table.UnregisterProcessOperate},
		{TaskID: "a", Priority: 1, OpType: table.StartProcessOperate},
		{TaskID: "b", Priority: 2, OpType: table.StartProcessOperate},
	})

	assertNames(t, plan.Stages, "immediate", "priority-1", "priority-2")

	// 托管类失败不阻断优先级阶段
	if plan.Stages[0].OnFailure != taskTypes.StageFailureContinue {
		t.Fatalf("托管类阶段应为 continue, got %s", plan.Stages[0].OnFailure)
	}
	// 首个优先级阶段与托管类阶段同时下发
	if !plan.Stages[1].StartWithPrevious {
		t.Fatal("首个优先级阶段应与托管类阶段同时下发")
	}
	// 后续优先级阶段必须等待
	if plan.Stages[2].StartWithPrevious {
		t.Fatal("后续优先级阶段不应与前一阶段并发")
	}
	if plan.Stages[0].Total != 2 {
		t.Fatalf("托管类任务数不符: %d", plan.Stages[0].Total)
	}
}

func TestBuildStagesOnlyManagedOperations(t *testing.T) {
	t.Parallel()

	plan := BuildStages([]TaskItem{
		{TaskID: "reg", OpType: table.RegisterProcessOperate},
		{TaskID: "upd", OpType: table.UpdateRegisterProcessOperate},
	})

	assertNames(t, plan.Stages, "immediate")
	if plan.Stages[0].StartWithPrevious {
		t.Fatal("唯一阶段不应设置 StartWithPrevious")
	}
}

func TestBuildStagesBlockMessageMatchesOrder(t *testing.T) {
	t.Parallel()

	asc := BuildStages([]TaskItem{
		{TaskID: "a", Priority: 7, OpType: table.StartProcessOperate},
	})
	if !strings.Contains(asc.Stages[0].BlockMessage, "优先级大于此") {
		t.Fatalf("升序阻断文案不符: %s", asc.Stages[0].BlockMessage)
	}
	if !strings.Contains(asc.Stages[0].BlockMessage, "4103003") {
		t.Fatalf("升序阻断文案应带错误码 4103003: %s", asc.Stages[0].BlockMessage)
	}

	desc := BuildStages([]TaskItem{
		{TaskID: "a", Priority: 7, OpType: table.StopProcessOperate},
	})
	if !strings.Contains(desc.Stages[0].BlockMessage, "优先级小于此") {
		t.Fatalf("降序阻断文案不符: %s", desc.Stages[0].BlockMessage)
	}
	if !strings.Contains(desc.Stages[0].BlockMessage, "4103003") {
		t.Fatalf("降序阻断文案应带错误码 4103003: %s", desc.Stages[0].BlockMessage)
	}
}

func TestBuildStagesAssignsContiguousSeq(t *testing.T) {
	t.Parallel()

	plan := BuildStages([]TaskItem{
		{TaskID: "reg", OpType: table.RegisterProcessOperate},
		{TaskID: "a", Priority: 1, OpType: table.StartProcessOperate},
		{TaskID: "b", Priority: 9, OpType: table.StartProcessOperate},
	})

	for i, stage := range plan.Stages {
		if stage.Seq != i {
			t.Fatalf("阶段序号应连续递增, stage[%d].Seq=%d", i, stage.Seq)
		}
	}
}
