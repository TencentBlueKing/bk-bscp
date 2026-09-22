/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://www.opensource.org/licenses/MIT
 *
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"testing"
	"time"

	taskTypes "github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"github.com/samber/lo"

	executorCommon "github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	configExecutor "github.com/TencentBlueKing/bk-bscp/internal/task/executor/config"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// newStaggerTask 构建带错峰延迟步骤的空任务，模拟 PushConfigTask 构建产物
func newStaggerTask() *taskTypes.Task {
	name := string(configExecutor.ConfigStaggerDelayStepName)
	step := taskTypes.NewStep(name, name).
		SetMaxExecution(5 * time.Minute)
	lo.Must0(step.SetPayload(configExecutor.PushConfigPayload{}))
	return &taskTypes.Task{Steps: []*taskTypes.Step{step}}
}

// getStaggerSeconds 读取错峰步骤 payload 中的延迟秒数
func getStaggerSeconds(t *testing.T, task *taskTypes.Task) int {
	t.Helper()
	payload := &configExecutor.PushConfigPayload{}
	if err := task.Steps[0].GetPayload(payload); err != nil {
		t.Fatalf("get stagger payload failed: %v", err)
	}
	return payload.StaggerSeconds
}

// TestSetConfigPushStaggerSameHost 同主机多个下发的任务按序号 0/3/6s 递增延迟，
// 首个任务（序号 0）不写延迟直接执行；延迟时长计入步骤执行上限
func TestSetConfigPushStaggerSameHost(t *testing.T) {
	t.Parallel()

	kt := &kit.Kit{TenantID: "tenant"}
	interval, baseDelay := 3, 5*time.Minute
	counters := make(map[string]int)

	// 第 1 个任务：主机内序号 0，不延迟
	task1 := newStaggerTask()
	setConfigPushStagger(kt, task1, 100, 200,
		&executorCommon.TaskPayload{ProcessPayload: &executorCommon.ProcessPayload{AgentID: "agent-h1"}},
		counters, interval, baseDelay)
	if got := getStaggerSeconds(t, task1); got != 0 {
		t.Fatalf("同主机首个任务不应延迟, got %d", got)
	}

	// 第 2 个任务：延迟 3s，步骤执行上限相应放宽
	task2 := newStaggerTask()
	setConfigPushStagger(kt, task2, 100, 200,
		&executorCommon.TaskPayload{ProcessPayload: &executorCommon.ProcessPayload{AgentID: "agent-h1"}},
		counters, interval, baseDelay)
	if got := getStaggerSeconds(t, task2); got != 3 {
		t.Fatalf("同主机第二个任务应延迟 3s, got %d", got)
	}
	if got := task2.Steps[0].GetMaxExecution(); got != baseDelay+3*time.Second {
		t.Fatalf("步骤执行上限应为 base+3s, got %s", got)
	}

	// 第 3 个任务：延迟 6s
	task3 := newStaggerTask()
	setConfigPushStagger(kt, task3, 100, 200,
		&executorCommon.TaskPayload{ProcessPayload: &executorCommon.ProcessPayload{AgentID: "agent-h1"}},
		counters, interval, baseDelay)
	if got := getStaggerSeconds(t, task3); got != 6 {
		t.Fatalf("同主机第三个任务应延迟 6s, got %d", got)
	}
}

// TestSetConfigPushStaggerDifferentHosts 不同主机的任务互不影响，均不延迟
func TestSetConfigPushStaggerDifferentHosts(t *testing.T) {
	t.Parallel()

	kt := &kit.Kit{TenantID: "tenant"}
	counters := make(map[string]int)
	payloads := []*executorCommon.TaskPayload{
		{ProcessPayload: &executorCommon.ProcessPayload{AgentID: "agent-h1"}},
		{ProcessPayload: &executorCommon.ProcessPayload{AgentID: "agent-h2"}},
	}

	for i, payload := range payloads {
		task := newStaggerTask()
		setConfigPushStagger(kt, task, 100, 200, payload, counters, 3, 5*time.Minute)
		if got := getStaggerSeconds(t, task); got != 0 {
			t.Fatalf("不同主机任务均不应延迟, 任务 %d got %d", i, got)
		}
	}
}

// TestSetConfigPushStaggerHostKeyFallback AgentID 缺失时主机标识退化为 管控区域:内网IP
func TestSetConfigPushStaggerHostKeyFallback(t *testing.T) {
	t.Parallel()

	kt := &kit.Kit{TenantID: "tenant"}
	interval, baseDelay := 3, 5*time.Minute
	counters := make(map[string]int)

	noAgent := func(ip string) *executorCommon.TaskPayload {
		return &executorCommon.TaskPayload{
			ProcessPayload: &executorCommon.ProcessPayload{CloudID: 0, InnerIP: ip},
		}
	}

	// 同 IP 的两个任务：0 / 3s
	task1 := newStaggerTask()
	setConfigPushStagger(kt, task1, 100, 200, noAgent("127.0.0.1"), counters, interval, baseDelay)
	if got := getStaggerSeconds(t, task1); got != 0 {
		t.Fatalf("同 IP 首个任务不应延迟, got %d", got)
	}
	task2 := newStaggerTask()
	setConfigPushStagger(kt, task2, 100, 200, noAgent("127.0.0.1"), counters, interval, baseDelay)
	if got := getStaggerSeconds(t, task2); got != 3 {
		t.Fatalf("同 IP 第二个任务应延迟 3s, got %d", got)
	}

	// 不同 IP 独立计数，不延迟
	task3 := newStaggerTask()
	setConfigPushStagger(kt, task3, 100, 200, noAgent("127.0.0.2"), counters, interval, baseDelay)
	if got := getStaggerSeconds(t, task3); got != 0 {
		t.Fatalf("不同 IP 任务不应延迟, got %d", got)
	}
}

// TestSetConfigPushStaggerDisabled interval <= 0 时不启用错峰，同主机任务也不写延迟
func TestSetConfigPushStaggerDisabled(t *testing.T) {
	t.Parallel()

	kt := &kit.Kit{TenantID: "tenant"}
	counters := make(map[string]int)
	payload := &executorCommon.TaskPayload{ProcessPayload: &executorCommon.ProcessPayload{AgentID: "agent-h1"}}

	task1 := newStaggerTask()
	setConfigPushStagger(kt, task1, 100, 200, payload, counters, 0, 5*time.Minute)
	task2 := newStaggerTask()
	setConfigPushStagger(kt, task2, 100, 200, payload, counters, 0, 5*time.Minute)

	if got := getStaggerSeconds(t, task2); got != 0 {
		t.Fatalf("interval=0 时不应写延迟, got %d", got)
	}
}
