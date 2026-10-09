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

	desc := BuildStages([]TaskItem{
		{TaskID: "a", Priority: 7, OpType: table.StopProcessOperate},
	})
	if !strings.Contains(desc.Stages[0].BlockMessage, "优先级小于此") {
		t.Fatalf("降序阻断文案不符: %s", desc.Stages[0].BlockMessage)
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

func TestBuildStagesStaggerIndex(t *testing.T) {
	t.Parallel()

	// 同优先级、同主机的 4 个实例任务，主机内序号应为 0/1/2/3（错峰延迟的基数）
	plan := BuildStages([]TaskItem{
		{TaskID: "i4", Priority: 1, OpType: table.StopProcessOperate, HostKey: "h1"},
		{TaskID: "i3", Priority: 1, OpType: table.StopProcessOperate, HostKey: "h1"},
		{TaskID: "i2", Priority: 1, OpType: table.StopProcessOperate, HostKey: "h1"},
		{TaskID: "i1", Priority: 1, OpType: table.StopProcessOperate, HostKey: "h1"},
	})

	for i, id := range []string{"i4", "i3", "i2", "i1"} {
		if got := plan.StaggerIndex(id); got != i {
			t.Fatalf("任务 %s 阶段内序号不符, got %d want %d", id, got, i)
		}
	}

	// 同优先级、不同主机的任务各自独立从 0 计数，互不影响
	plan2 := BuildStages([]TaskItem{
		{TaskID: "h1-a", Priority: 1, OpType: table.StartProcessOperate, HostKey: "h1"},
		{TaskID: "h2-a", Priority: 1, OpType: table.StartProcessOperate, HostKey: "h2"},
		{TaskID: "h1-b", Priority: 1, OpType: table.StartProcessOperate, HostKey: "h1"},
		{TaskID: "h2-b", Priority: 1, OpType: table.StartProcessOperate, HostKey: "h2"},
	})
	want := map[string]int{"h1-a": 0, "h1-b": 1, "h2-a": 0, "h2-b": 1}
	for id, wantIdx := range want {
		if got := plan2.StaggerIndex(id); got != wantIdx {
			t.Fatalf("任务 %s 主机内序号不符, got %d want %d", id, got, wantIdx)
		}
	}

	// 不同优先级分属不同阶段，各自独立从 0 计数
	plan3 := BuildStages([]TaskItem{
		{TaskID: "a", Priority: 1, OpType: table.StartProcessOperate, HostKey: "h1"},
		{TaskID: "b", Priority: 2, OpType: table.StartProcessOperate, HostKey: "h1"},
	})
	if plan3.StaggerIndex("a") != 0 || plan3.StaggerIndex("b") != 0 {
		t.Fatalf("不同阶段任务序号应各自从 0 计数: %v", plan3.StaggerIndexOf)
	}
}

// TestBuildStagesPerHostStaggerMixedProcesses 同主机同优先级下不同进程的实例合并计数错峰；
// 启动（升序）与停止（降序）两个编排方向下，主机内序号语义一致。
func TestBuildStagesPerHostStaggerMixedProcesses(t *testing.T) {
	t.Parallel()

	// 同主机 h1 上 nginx-1 / mysql-1 两个进程的 4 个实例合并计数，另一台主机独立从 0 起算
	plan := BuildStages([]TaskItem{
		{TaskID: "nginx-A", Priority: 1, OpType: table.StartProcessOperate, HostKey: "h1"},
		{TaskID: "mysql-B", Priority: 1, OpType: table.StartProcessOperate, HostKey: "h1"},
		{TaskID: "nginx-C", Priority: 1, OpType: table.StartProcessOperate, HostKey: "h1"},
		{TaskID: "mysql-D", Priority: 1, OpType: table.StartProcessOperate, HostKey: "h1"},
		{TaskID: "other-A", Priority: 1, OpType: table.StartProcessOperate, HostKey: "h2"},
	})
	want := map[string]int{"nginx-A": 0, "mysql-B": 1, "nginx-C": 2, "mysql-D": 3, "other-A": 0}
	for id, wantIdx := range want {
		if got := plan.StaggerIndex(id); got != wantIdx {
			t.Fatalf("任务 %s 主机内序号不符, got %d want %d", id, got, wantIdx)
		}
	}

	// 停止操作按优先级降序分阶段，主机内计数语义不变：跨阶段重置，同阶段内递增
	stopPlan := BuildStages([]TaskItem{
		{TaskID: "s1", Priority: 1, OpType: table.StopProcessOperate, HostKey: "h1"},
		{TaskID: "s2", Priority: 1, OpType: table.StopProcessOperate, HostKey: "h1"},
		{TaskID: "s3", Priority: 2, OpType: table.StopProcessOperate, HostKey: "h1"},
	})
	if stopPlan.StaggerIndex("s1") != 0 || stopPlan.StaggerIndex("s2") != 1 {
		t.Fatalf("同阶段同主机停止任务应递增计数: %v", stopPlan.StaggerIndexOf)
	}
	if stopPlan.StaggerIndex("s3") != 0 {
		t.Fatalf("下一阶段的同主机任务应重新从 0 计数: %v", stopPlan.StaggerIndexOf)
	}
}
