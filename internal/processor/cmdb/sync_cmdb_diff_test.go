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

package cmdb

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

func procWithID(id uint32, alias string) *table.Process {
	return &table.Process{
		ID:   id,
		Spec: &table.ProcessSpec{Alias: alias},
		Attachment: &table.ProcessAttachment{
			TenantID:    "default",
			CcProcessID: 100,
		},
	}
}

// TestPickProcessMatch 基准行选择必须确定性：优先别名一致，否则取 ID 最小
func TestPickProcessMatch(t *testing.T) {
	if _, ok := pickProcessMatch(nil, "a"); ok {
		t.Fatalf("empty matches should return false")
	}

	// 别名一致优先，即使 ID 更大
	matches := []*table.Process{procWithID(5, "old"), procWithID(9, "new")}
	picked, ok := pickProcessMatch(matches, "new")
	if !ok || picked.ID != 9 {
		t.Fatalf("picked id = %v, ok = %v, want id 9", picked, ok)
	}

	// 无别名一致时取 ID 最小（与切片顺序无关）
	matches = []*table.Process{procWithID(9, "b"), procWithID(3, "a"), procWithID(7, "c")}
	picked, ok = pickProcessMatch(matches, "none")
	if !ok || picked.ID != 3 {
		t.Fatalf("picked id = %v, ok = %v, want id 3", picked, ok)
	}
}

// TestSameOrLegacyTenant 空租户视为历史数据可参与 diff，显式其他租户禁止参与
func TestSameOrLegacyTenant(t *testing.T) {
	cases := []struct {
		oldTenant, newTenant string
		want                 bool
	}{
		{"", "default", true},
		{"default", "default", true},
		{"tenantA", "default", false},
		{"default", "tenantA", false},
		{"", "tenantA", false},
	}
	for _, c := range cases {
		if got := sameOrLegacyTenant(c.oldTenant, c.newTenant); got != c.want {
			t.Fatalf("sameOrLegacyTenant(%q, %q) = %v, want %v", c.oldTenant, c.newTenant, got, c.want)
		}
	}
}

func TestDiffProcessesMatchesOnlyCurrentTenant(t *testing.T) {
	newP, current := newAgentIDProcessPair("agent", "agent", table.AgentStatusNormal, table.AgentStatusNormal)
	newP.Attachment.TenantID = constant.DefaultTenantID
	current.Attachment.TenantID = constant.DefaultTenantID
	_, other := newAgentIDProcessPair("agent", "agent", table.AgentStatusNormal, table.AgentStatusNormal)
	other.ID = 1
	other.Attachment.TenantID = "other"
	newP.Spec.SetName = "new set"
	diff, err := diffProcesses(newAgentIDSyncContext(), []*table.Process{other, current}, []*table.Process{newP})
	if err != nil {
		t.Fatal(err)
	}
	if got := processIDs(diff.ToUpdateProcesses); !reflect.DeepEqual(got, []uint32{current.ID}) {
		t.Fatalf("updated IDs = %v, want [%d]", got, current.ID)
	}
	if other.Spec.SetName != "" {
		t.Fatal("another tenant's process was modified")
	}
}

// TestProcessSyncLockResource 锁资源名以 tenant + biz 为维度，不同业务互不阻塞
func TestProcessSyncLockResource(t *testing.T) {
	if got := processSyncLockResource("default", 7); got != "cmdb:process:sync:default:7" {
		t.Fatalf("got %q", got)
	}
	if processSyncLockResource("default", 7) == processSyncLockResource("default", 8) {
		t.Fatalf("lock resource should differ by biz id")
	}
	if processSyncLockResource("tenantA", 7) == processSyncLockResource("tenantB", 7) {
		t.Fatalf("lock resource should differ by tenant")
	}
}

// stubSyncLocker 桩锁：failures 为还需拒绝的次数（<0 表示一直拒绝），用于验证抢锁超时语义
type stubSyncLocker struct {
	mu       sync.Mutex
	failures int
	attempts int
	handle   *stubSyncHandle
}

func (l *stubSyncLocker) TryAcquire(string) lock.Handle {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.attempts++
	if l.failures < 0 {
		return nil
	}
	if l.failures > 0 {
		l.failures--
		return nil
	}
	if l.handle == nil {
		l.handle = &stubSyncHandle{renewInterval: time.Millisecond}
	}
	return l.handle
}

func (l *stubSyncLocker) stats() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.attempts
}

// stubSyncHandle 桩持锁句柄：记录续期与释放，用于验证持锁期间续期、释放只发生一次
type stubSyncHandle struct {
	mu            sync.Mutex
	renewInterval time.Duration
	renews        int
	released      bool
	renewErr      error
}

func (h *stubSyncHandle) Renew(context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.renews++
	return h.renewErr
}

func (h *stubSyncHandle) RenewInterval() time.Duration { return h.renewInterval }

func (h *stubSyncHandle) Release() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.released = true
}

func (h *stubSyncHandle) stats() (renews int, released bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.renews, h.released
}

// TestSyncBizProcessesSkipsWithoutSuccess 抢锁超时不能返回成功：必须返回 ErrSyncSkipped 且无同步结果，
// 否则调用方会把「没同步」当成「已同步且无变更」，一路当成成功（一键同步任务会显示成功）
func TestSyncBizProcessesSkipsWithoutSuccess(t *testing.T) {
	locker := &stubSyncLocker{failures: -1}
	svc := &syncCMDBService{tenantID: "default", bizID: 7, redLock: locker, lockWait: 5 * time.Millisecond}

	res, err := svc.SyncBizProcesses(kit.NewWithTenant("default").Ctx)
	if !IsSyncSkipped(err) {
		t.Fatalf("err = %v, want ErrSyncSkipped", err)
	}
	if res != nil {
		t.Fatalf("result = %v, want nil on skip", res)
	}
	if locker.stats() == 0 {
		t.Fatalf("lock was never attempted")
	}
}

func TestLocalSyncLockEntryIsRemovedAfterUse(t *testing.T) {
	unlock := lockBizSyncLocal("tenant", 981237)
	key := "tenant-981237"
	bizSyncLocks.Lock()
	_, exists := bizSyncLocks.entries[key]
	bizSyncLocks.Unlock()
	if !exists {
		t.Fatal("local lock entry missing while held")
	}

	unlock()
	bizSyncLocks.Lock()
	_, exists = bizSyncLocks.entries[key]
	bizSyncLocks.Unlock()
	if exists {
		t.Fatal("local lock entry retained after last user released it")
	}
}

// TestKeepProcessSyncLockRenews 长同步必须按句柄给出的间隔续期，否则超过 TTL 会被其他实例按「过期」接管；
// 释放后不得继续续期，且重复调用解锁只释放一次
func TestKeepProcessSyncLockRenews(t *testing.T) {
	handle := &stubSyncHandle{renewInterval: 2 * time.Millisecond}
	svc := &syncCMDBService{tenantID: "default", bizID: 7}

	_, unlock, err := svc.keepProcessSyncLock(context.Background(), handle)
	if err != nil {
		t.Fatalf("keep lock: %v", err)
	}

	// 等待至少一次续期（最多 1s）
	deadline := time.Now().Add(time.Second)
	for {
		if renews, _ := handle.stats(); renews >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("lock was never renewed while held")
		}
		time.Sleep(time.Millisecond)
	}

	unlock()
	renews, released := handle.stats()
	if !released {
		t.Fatalf("lock was never released")
	}

	// 释放后再等两个续期间隔，确认续期 goroutine 已退出
	time.Sleep(10 * time.Millisecond)
	if after, _ := handle.stats(); after != renews {
		t.Fatalf("renew kept running after release: %d -> %d", renews, after)
	}

	// 解锁幂等：defer 之外被重复调用时不能二次释放
	unlock()
	if _, second := handle.stats(); !second {
		t.Fatalf("released flag lost after repeated unlock")
	}
}

func TestKeepProcessSyncLockCancelsWorkWhenRenewFails(t *testing.T) {
	handle := &stubSyncHandle{renewInterval: time.Millisecond, renewErr: errors.New("lock lost")}
	svc := &syncCMDBService{tenantID: "default", bizID: 7}
	ctx, unlock, err := svc.keepProcessSyncLock(context.Background(), handle)
	if err != nil {
		t.Fatalf("keep lock: %v", err)
	}
	defer unlock()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("work context was not canceled after renewal failed")
	}
	if err := ctx.Err(); err == nil {
		t.Fatal("work context should report cancellation")
	}
}

// TestCheckEmptyProcessBuild 「构建结果为空」的两种成因必须区分：CC 成功空页返回 nil（交给 diff
// 按删除语义收敛），拉取到非空数据却一条都没构建出来则返回错误（不得触发删除）
func TestCheckEmptyProcessBuild(t *testing.T) {
	cases := []struct {
		name    string
		fetched int
		built   int
		wantErr bool
	}{
		{name: "success empty list means no process", fetched: 0, built: 0, wantErr: false},
		{name: "all items built", fetched: 3, built: 3, wantErr: false},
		{name: "fetched but built none is build failure", fetched: 3, built: 0, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkEmptyProcessBuild(7, c.fetched, c.built)
			if (err != nil) != c.wantErr {
				t.Fatalf("checkEmptyProcessBuild(%d, %d) err = %v, wantErr %v",
					c.fetched, c.built, err, c.wantErr)
			}
		})
	}
}

// TestDiffProcessesEmptyNewDeletesAll CMDB 成功返回空列表时，库内该业务存活进程必须全部进入删除集，
// 否则最后一个进程被删除后本地行会永久残留 synced
func TestDiffProcessesEmptyNewDeletesAll(t *testing.T) {
	old := []*table.Process{
		procWithCCID(1, 101, "a", table.Synced),
		procWithCCID(2, 102, "b", table.Synced),
	}
	diff, err := diffProcesses(newFakeSyncContext(), old, nil)
	if err != nil {
		t.Fatalf("diffProcesses: %v", err)
	}
	if got := processIDs(diff.ToDeleteProcesses); !reflect.DeepEqual(got, []uint32{1, 2}) {
		t.Fatalf("ToDeleteProcesses = %v, want [1 2]", got)
	}
}

// TestDiffProcessesSkipsAlreadyDeleted 已软删除的行不再重复进入删除集，避免全量清理后每轮同步反复写库
func TestDiffProcessesSkipsAlreadyDeleted(t *testing.T) {
	old := []*table.Process{
		procWithCCID(1, 101, "a", table.Synced),
		procWithCCID(2, 102, "b", table.Deleted),
	}
	diff, err := diffProcesses(newFakeSyncContext(), old, nil)
	if err != nil {
		t.Fatalf("diffProcesses: %v", err)
	}
	if got := processIDs(diff.ToDeleteProcesses); !reflect.DeepEqual(got, []uint32{1}) {
		t.Fatalf("ToDeleteProcesses = %v, want [1]", got)
	}
}

// TestDiffProcessesSkipsOtherTenantRows 删除扫描与收敛/恢复走同一租户判定：
// 显式属于其他租户的行不得进入删除集；legacy 空租户行（历史归属）与本租户行照常参与
func TestDiffProcessesSkipsOtherTenantRows(t *testing.T) {
	cases := []struct {
		name      string
		kitTenant string
		wantIDs   []uint32
	}{
		// 单租户模式：kit 租户为空，归一化为 default；其余租户行必须被跳过
		{name: "single tenant mode", kitTenant: "", wantIDs: []uint32{2, 3}},
		// 多租户模式：列表已按租户过滤，这层是重复保护，语义保持一致
		{name: "multi tenant mode", kitTenant: "tenantB", wantIDs: []uint32{3}},
		{name: "explicit default tenant", kitTenant: "default", wantIDs: []uint32{2, 3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			other := procWithCCID(1, 101, "a", table.Synced)
			other.Attachment.TenantID = "tenantA"
			legacy := procWithCCID(2, 102, "b", table.Synced)
			legacy.Attachment.TenantID = ""
			current := procWithCCID(3, 103, "c", table.Synced)
			current.Attachment.TenantID = constant.DefaultTenantID
			if c.kitTenant != "" {
				current.Attachment.TenantID = c.kitTenant
			}

			ctx := newFakeSyncContext()
			ctx.Kit.TenantID = c.kitTenant
			diff, err := diffProcesses(ctx, []*table.Process{other, legacy, current}, nil)
			if err != nil {
				t.Fatalf("diffProcesses: %v", err)
			}
			if got := processIDs(diff.ToDeleteProcesses); !reflect.DeepEqual(got, c.wantIDs) {
				t.Fatalf("ToDeleteProcesses = %v, want %v", got, c.wantIDs)
			}
		})
	}
}

// TestSyncProcessDataEmptyNewMarksAllDeleted 空列表必须真正落库为删除（此前 SyncProcessData
// 在 newProcesses 为空时提前返回，是整业务进程被清空后本地行残留的第二个拦截点）
func TestSyncProcessDataEmptyNewMarksAllDeleted(t *testing.T) {
	procDao := &fakeRecordingProcessDao{}
	instDao := &fakeNoopInstanceDao{}
	daoSet := &fakeSyncDaoSet{proc: procDao, inst: instDao}
	old := []*table.Process{
		procWithCCID(1, 101, "a", table.Synced),
		procWithCCID(2, 102, "b", table.Synced),
	}

	res, err := SyncProcessData(kit.New(), daoSet, nil, 7, old, nil)
	if err != nil {
		t.Fatalf("SyncProcessData: %v", err)
	}
	if !res.Changed {
		t.Fatalf("Changed = false, want true (deletion must be reported as a change)")
	}
	if got := processIDs(procDao.deleted); !reflect.DeepEqual(got, []uint32{1, 2}) {
		t.Fatalf("marked deleted = %v, want [1 2]", got)
	}
	if got := instDao.cleaned; !reflect.DeepEqual(got, []uint32{1, 2}) {
		t.Fatalf("cleaned instance process ids = %v, want [1 2]", got)
	}
}

// TestSyncProcessDataNoChangeKeepsChangedFalse 库内已是 deleted 时全量清理不产生写放大
func TestSyncProcessDataNoChangeKeepsChangedFalse(t *testing.T) {
	procDao := &fakeRecordingProcessDao{}
	daoSet := &fakeSyncDaoSet{proc: procDao, inst: &fakeNoopInstanceDao{}}
	old := []*table.Process{
		procWithCCID(1, 101, "a", table.Deleted),
		procWithCCID(2, 102, "b", table.Deleted),
	}

	res, err := SyncProcessData(kit.New(), daoSet, nil, 7, old, nil)
	if err != nil {
		t.Fatalf("SyncProcessData: %v", err)
	}
	if res.Changed {
		t.Fatalf("Changed = true, want false")
	}
	if len(procDao.deleted) != 0 {
		t.Fatalf("marked deleted = %v, want none", processIDs(procDao.deleted))
	}
}

func procWithCCID(id, ccID uint32, alias string, status table.CCSyncStatus) *table.Process {
	return &table.Process{
		ID: id,
		Attachment: &table.ProcessAttachment{
			TenantID:    "default",
			BizID:       7,
			CcProcessID: ccID,
			HostID:      id,
			ModuleID:    10,
		},
		Spec: &table.ProcessSpec{Alias: alias, SourceData: "{}", CcSyncStatus: status},
	}
}

func processIDs(processes []*table.Process) []uint32 {
	ids := make([]uint32, 0, len(processes))
	for _, p := range processes {
		ids = append(ids, p.ID)
	}
	return ids
}

func newFakeSyncContext() *SyncContext {
	return &SyncContext{
		Kit:           kit.New(),
		Dao:           &fakeSyncDaoSet{proc: &fakeRecordingProcessDao{}, inst: &fakeNoopInstanceDao{}},
		Now:           time.Now(),
		ModuleCounter: make(map[ModuleAliasKey]int),
	}
}

// fakeRecordingProcessDao 记录被标记删除的进程
type fakeRecordingProcessDao struct {
	dao.Process
	deleted []*table.Process
}

func (d *fakeRecordingProcessDao) UpdateSyncStatusAndAliasTx(
	_ *kit.Kit, _ *gen.QueryTx, data []*table.Process) error {
	d.deleted = append(d.deleted, data...)
	return nil
}

// fakeNoopInstanceDao 实例查询恒为空，并记录被清理实例的进程 ID
type fakeNoopInstanceDao struct {
	dao.ProcessInstance
	cleaned []uint32
}

func (d *fakeNoopInstanceDao) ListByProcessIDsWithTx(
	_ *kit.Kit, _ *gen.QueryTx, _ uint32, _ []uint32) ([]*table.ProcessInstance, error) {
	return nil, nil
}

func (d *fakeNoopInstanceDao) DeleteStoppedUnmanagedWithTx(
	_ *kit.Kit, _ *gen.QueryTx, _ uint32, processIDs []uint32) error {
	d.cleaned = append(d.cleaned, processIDs...)
	return nil
}

// fakeSyncDaoSet 仅实现全量清理路径所需的 DAO
type fakeSyncDaoSet struct {
	dao.Set
	proc *fakeRecordingProcessDao
	inst *fakeNoopInstanceDao
}

func (s *fakeSyncDaoSet) Process() dao.Process                 { return s.proc }
func (s *fakeSyncDaoSet) ProcessInstance() dao.ProcessInstance { return s.inst }
