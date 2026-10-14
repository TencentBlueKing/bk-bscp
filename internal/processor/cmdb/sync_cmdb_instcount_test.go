/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmdb

import (
	"testing"
	"time"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// fakeInstCountProcessDao 进程查询桩：同模块同别名进程列表恒为空
type fakeInstCountProcessDao struct {
	dao.Process
}

func (d *fakeInstCountProcessDao) ListByModuleIDAndAliasWithTx(
	_ *kit.Kit, _ *gen.QueryTx, _, _ uint32, _ string) ([]uint32, error) {
	return nil, nil
}

// fakeInstCountInstanceDao 实例查询桩：按预置数据返回实例与计数
type fakeInstCountInstanceDao struct {
	dao.ProcessInstance
	existing    []*table.ProcessInstance // ListByProcessIDTx 返回的已有实例
	toShrink    []*table.ProcessInstance // ListByProcessIDOrderBySeqDescTx 返回的待缩容实例
	countByProc map[uint32]int           // CountByProcessIDsWithTx 返回的实例计数
}

func (d *fakeInstCountInstanceDao) ListByProcessIDTx(
	_ *kit.Kit, _ *gen.QueryTx, _ uint32, _ uint32) ([]*table.ProcessInstance, error) {
	return d.existing, nil
}

func (d *fakeInstCountInstanceDao) ListByProcessIDOrderBySeqDescTx(
	_ *kit.Kit, _ *gen.QueryTx, _ uint32, _ uint32, limit int) ([]*table.ProcessInstance, error) {
	if limit <= 0 {
		return nil, nil
	}
	if len(d.toShrink) > limit {
		return d.toShrink[:limit], nil
	}
	return d.toShrink, nil
}

func (d *fakeInstCountInstanceDao) GetMaxModuleInstSeqByProcessIDsWithTx(
	_ *kit.Kit, _ *gen.QueryTx, _ uint32, _ []uint32) (int, error) {
	return 0, nil
}

func (d *fakeInstCountInstanceDao) GetMaxHostInstSeqTx(
	_ *kit.Kit, _ *gen.QueryTx, _ uint32, _ []uint32) (int, error) {
	return 0, nil
}

func (d *fakeInstCountInstanceDao) CountByProcessIDsWithTx(
	_ *kit.Kit, _ *gen.QueryTx, _ uint32, _ []uint32) (map[uint32]int, error) {
	return d.countByProc, nil
}

// fakeInstCountDaoSet 组合上述 fake DAO
type fakeInstCountDaoSet struct {
	dao.Set
	proc *fakeInstCountProcessDao
	inst *fakeInstCountInstanceDao
}

func (s *fakeInstCountDaoSet) Process() dao.Process                 { return s.proc }
func (s *fakeInstCountDaoSet) ProcessInstance() dao.ProcessInstance { return s.inst }

// newInstCountSyncContext 构建实例数校准测试用的同步上下文
func newInstCountSyncContext(inst *fakeInstCountInstanceDao) *SyncContext {
	return &SyncContext{
		Kit: kit.New(),
		Dao: &fakeInstCountDaoSet{
			proc: &fakeInstCountProcessDao{},
			inst: inst,
		},
		Now:           time.Now(),
		HostCounter:   make(map[HostProcessKey]int),
		ModuleCounter: make(map[ModuleAliasKey]int),
	}
}

// newInstCountProcessPair 构建一对 ProcNum 相同的进程（CMDB 与 DB 均为 procNum）
func newInstCountProcessPair(procNum uint) (*table.Process, *table.Process) {
	newP := &table.Process{
		Attachment: &table.ProcessAttachment{BizID: 3, CcProcessID: 1000, ModuleID: 10,
			HostID: 100, AgentID: "agent-1"},
		Spec: &table.ProcessSpec{Alias: "proc1", SourceData: "{}", ProcNum: procNum},
	}
	oldP := &table.Process{
		ID: 5,
		Attachment: &table.ProcessAttachment{BizID: 3, CcProcessID: 1000, ModuleID: 10,
			HostID: 100, AgentID: "agent-1"},
		Spec: &table.ProcessSpec{Alias: "proc1", SourceData: "{}", ProcNum: procNum},
	}
	return newP, oldP
}

// newInstWithHostSeq 构建带 HostInstSeq 的实例
func newInstWithHostSeq(id uint32, hostInstSeq uint32) *table.ProcessInstance {
	return &table.ProcessInstance{
		ID:         id,
		Attachment: &table.ProcessInstanceAttachment{BizID: 3, CcProcessID: 1000, ProcessID: 5},
		Spec:       &table.ProcessInstanceSpec{HostInstSeq: hostInstSeq},
	}
}

// TestBuildProcessChangesFillsMissingInstances 校验一键同步实例数校准：
// ProcNum 未变化（3==3）但 DB 实例缺失（如实例 2 被清理，仅剩序号 1、3）时，
// 同步应补齐缺失的实例，并优先填补空洞序号 2
func TestBuildProcessChangesFillsMissingInstances(t *testing.T) {
	inst := &fakeInstCountInstanceDao{
		existing: []*table.ProcessInstance{
			newInstWithHostSeq(1, 1),
			newInstWithHostSeq(3, 3),
		},
	}
	ctx := newInstCountSyncContext(inst)

	newP, oldP := newInstCountProcessPair(3)
	oldInstCount := 2

	res, err := BuildProcessChanges(ctx, &BuildProcessChangesParams{
		NewProcess:   newP,
		OldProcess:   oldP,
		OldInstCount: &oldInstCount,
	})
	if err != nil {
		t.Fatalf("BuildProcessChanges failed: %v", err)
	}

	if len(res.ToAddInstances) != 1 {
		t.Fatalf("added instances = %d, want 1 (missing instance should be refilled)", len(res.ToAddInstances))
	}
	if got := res.ToAddInstances[0].Spec.HostInstSeq; got != 2 {
		t.Fatalf("added instance HostInstSeq = %d, want 2 (fill the gap)", got)
	}
	if res.ToUpdateProcess == nil {
		t.Fatal("expected ToUpdateProcess for instance reconciliation")
	}
}

// TestBuildProcessChangesNoChangeWhenInstCountMatches 校验 ProcNum 与实际实例数一致时
// 不产生任何变更，避免多余写库
func TestBuildProcessChangesNoChangeWhenInstCountMatches(t *testing.T) {
	inst := &fakeInstCountInstanceDao{
		existing: []*table.ProcessInstance{
			newInstWithHostSeq(1, 1),
			newInstWithHostSeq(2, 2),
			newInstWithHostSeq(3, 3),
		},
	}
	ctx := newInstCountSyncContext(inst)

	newP, oldP := newInstCountProcessPair(3)
	oldInstCount := 3

	res, err := BuildProcessChanges(ctx, &BuildProcessChangesParams{
		NewProcess:   newP,
		OldProcess:   oldP,
		OldInstCount: &oldInstCount,
	})
	if err != nil {
		t.Fatalf("BuildProcessChanges failed: %v", err)
	}

	if res.ToUpdateProcess != nil || res.ToAddProcess != nil ||
		len(res.ToAddInstances) > 0 || len(res.ToDeleteInstanceIDs) > 0 {
		t.Fatal("expected no change when inst count matches ProcNum")
	}
}

// TestBuildProcessChangesShrinksExceedingInstances 校验实例数超出目标时按 ProcNum 收缩
func TestBuildProcessChangesShrinksExceedingInstances(t *testing.T) {
	inst := &fakeInstCountInstanceDao{
		toShrink: []*table.ProcessInstance{
			newInstWithHostSeq(4, 4),
			newInstWithHostSeq(5, 5),
		},
	}
	ctx := newInstCountSyncContext(inst)

	newP, oldP := newInstCountProcessPair(3)
	oldInstCount := 5

	res, err := BuildProcessChanges(ctx, &BuildProcessChangesParams{
		NewProcess:   newP,
		OldProcess:   oldP,
		OldInstCount: &oldInstCount,
	})
	if err != nil {
		t.Fatalf("BuildProcessChanges failed: %v", err)
	}

	if len(res.ToDeleteInstanceIDs) != 2 {
		t.Fatalf("deleted instance ids = %d, want 2", len(res.ToDeleteInstanceIDs))
	}
	if len(res.ToAddInstances) != 0 {
		t.Fatalf("added instances = %d, want 0", len(res.ToAddInstances))
	}
}

// TestDiffProcessesFillsMissingInstances 校验 diffProcesses 全链路：
// 通过批量实例计数发现 ProcNum 未变但实例漂移时，产生补齐实例的差异
func TestDiffProcessesFillsMissingInstances(t *testing.T) {
	newP, oldP := newInstCountProcessPair(3)

	inst := &fakeInstCountInstanceDao{
		existing: []*table.ProcessInstance{
			newInstWithHostSeq(1, 1),
			newInstWithHostSeq(3, 3),
		},
		countByProc: map[uint32]int{5: 2},
	}
	ctx := newInstCountSyncContext(inst)

	diff, err := diffProcesses(ctx, []*table.Process{oldP}, []*table.Process{newP})
	if err != nil {
		t.Fatalf("diffProcesses failed: %v", err)
	}

	if len(diff.ToAddInstances) != 1 {
		t.Fatalf("added instances = %d, want 1 (missing instance should be refilled)", len(diff.ToAddInstances))
	}
	if len(diff.ToDeleteProcesses) != 0 {
		t.Fatalf("deleted processes = %d, want 0", len(diff.ToDeleteProcesses))
	}
}
