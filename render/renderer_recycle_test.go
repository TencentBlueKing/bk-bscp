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

package render

import (
	"os"
	"os/exec"
	"testing"
)

// startIdleWorker 用一个空转进程模拟 worker，避免测试依赖 uv/python 环境
func startIdleWorker(t *testing.T) *renderWorker {
	t.Helper()

	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start helper process: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return &renderWorker{cmd: cmd}
}

func TestWorkerRSSMBReadsCurrentProcess(t *testing.T) {
	rssMB, ok := workerRSSMB(os.Getpid())
	if !ok {
		t.Skip("process RSS is unavailable on this platform")
	}
	if rssMB <= 0 {
		t.Fatalf("workerRSSMB = %d, want > 0", rssMB)
	}
}

func TestRecycleIfNeededOnRSSLimit(t *testing.T) {
	w := startIdleWorker(t)
	rssMB, ok := workerRSSMB(w.cmd.Process.Pid)
	if !ok {
		t.Skip("process RSS is unavailable on this platform")
	}

	// 上限取进程当前占用，必然触发重建
	r := &Renderer{workerRSSLimitMB: rssMB, workerMaxUses: 1000}
	r.recycleIfNeeded(w)

	if !w.dead {
		t.Fatalf("worker not recycled at rss limit %dMB", rssMB)
	}
}

func TestRecycleIfNeededKeepsWorkerBelowLimits(t *testing.T) {
	w := startIdleWorker(t)
	rssMB, ok := workerRSSMB(w.cmd.Process.Pid)
	if !ok {
		t.Skip("process RSS is unavailable on this platform")
	}

	r := &Renderer{workerRSSLimitMB: rssMB + 1024, workerMaxUses: 1000}
	r.recycleIfNeeded(w)

	if w.dead {
		t.Fatal("worker recycled while below rss limit")
	}
	if w.uses != 1 {
		t.Fatalf("uses = %d, want 1", w.uses)
	}
}

// 读不到进程常驻内存时（非 Linux 等），应退化为按渲染次数重建
func TestRecycleIfNeededFallsBackToUses(t *testing.T) {
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start helper process: %v", err)
	}
	_ = cmd.Wait()
	// 进程已退出，/proc 中不再有该 pid，等价于读不到 RSS
	if _, ok := workerRSSMB(cmd.Process.Pid); ok {
		t.Skip("pid still readable, cannot simulate missing RSS")
	}

	w := &renderWorker{cmd: cmd, uses: 4}
	r := &Renderer{workerRSSLimitMB: defaultWorkerRSSLimitMB, workerMaxUses: 5}
	r.recycleIfNeeded(w)

	if !w.dead {
		t.Fatal("worker not recycled after reaching max uses")
	}
}
