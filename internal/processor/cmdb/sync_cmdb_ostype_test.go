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
	"testing"
	"time"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// TestMapOsType 校验 R-001：CC bk_os_type 数字编码到业务语义字符串的映射
func TestMapOsType(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"linux", "1", "linux"},
		{"win", "2", "win"},
		{"aix", "3", "aix"},
		{"empty", "", ""},
		{"unknown", "9", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mapOsType(c.raw); got != c.want {
				t.Fatalf("mapOsType(%q) = %q, want %q", c.raw, got, c.want)
			}
		})
	}
}

// TestBuildHostOsTypeIndex 校验按 (bk_cloud_id, bk_host_innerip) 建立映射后的 os_type 索引，
// 空值与未覆盖编码不入索引（R-002 空值不参与覆盖）
func TestBuildHostOsTypeIndex(t *testing.T) {
	hosts := []bkcmdb.HostInfo{
		{BkCloudID: 0, BkHostInnerIP: "127.0.0.1", BkOSType: "1"},
		{BkCloudID: 1, BkHostInnerIP: "127.0.0.2", BkOSType: ""},
		{BkCloudID: 2, BkHostInnerIP: "127.0.0.3", BkOSType: "9"},
		{BkCloudID: 3, BkHostInnerIP: "127.0.0.4", BkOSType: "2"},
	}

	idx := buildHostOsTypeIndex(hosts)

	if got := idx[hostOsTypeKey{cloudID: 0, innerIP: "127.0.0.1"}]; got != "linux" {
		t.Fatalf("index linux = %q, want linux", got)
	}
	if got := idx[hostOsTypeKey{cloudID: 3, innerIP: "127.0.0.4"}]; got != "win" {
		t.Fatalf("index win = %q, want win", got)
	}
	if _, ok := idx[hostOsTypeKey{cloudID: 1, innerIP: "127.0.0.2"}]; ok {
		t.Fatal("empty bk_os_type should not be indexed")
	}
	if _, ok := idx[hostOsTypeKey{cloudID: 2, innerIP: "127.0.0.3"}]; ok {
		t.Fatal("unknown bk_os_type should not be indexed")
	}
	if len(idx) != 2 {
		t.Fatalf("index size = %d, want 2", len(idx))
	}
}

// osTypeStubCMDB 仅实现 ListBizHosts，用于驱动 buildProcessEntities 的 os_type 补全逻辑
type osTypeStubCMDB struct {
	bkcmdb.Service
	hosts []bkcmdb.HostInfo
	err   error
}

func (m *osTypeStubCMDB) ListBizHosts(_ context.Context, _ *bkcmdb.ListBizHostsRequest) (
	*bkcmdb.CMDBListData[bkcmdb.HostInfo], error) {
	if m.err != nil {
		return nil, m.err
	}
	return &bkcmdb.CMDBListData[bkcmdb.HostInfo]{Count: len(m.hosts), Info: m.hosts}, nil
}

func newOsTypeProcessItem(cloudID int, innerIP string) *bkcmdb.ProcessRelatedInfoItem {
	return &bkcmdb.ProcessRelatedInfoItem{
		Set:             &bkcmdb.ProcessSetInfo{BkSetID: 1, BkSetName: "set1"},
		Module:          &bkcmdb.ProcessModuleInfo{BkModuleID: 10, BkModuleName: "module1"},
		Host:            &bkcmdb.ProcessHostInfo{BkHostID: 100, BkCloudID: cloudID, BkHostInnerIP: innerIP},
		ServiceInstance: &bkcmdb.ProcessServiceInstInfo{ID: 1, Name: "svc1"},
		ProcessTemplate: &bkcmdb.ProcessTemplateRefInfo{ID: 5},
		Process:         &bkcmdb.ProcessDetailInfo{BkBizID: 3, BkProcessID: 1000, BkProcessName: "proc1", ProcNum: 1},
	}
}

// TestBuildProcessEntitiesSetsOsType 校验 F-003：主机维度获取的 os_type 关联写入进程
func TestBuildProcessEntitiesSetsOsType(t *testing.T) {
	svc := &osTypeStubCMDB{
		hosts: []bkcmdb.HostInfo{
			{BkCloudID: 0, BkHostInnerIP: "127.0.0.1", BkOSType: "1"},
		},
	}
	s := &syncCMDBService{bizID: 3, svc: svc}
	kt := kit.New()

	data := []*bkcmdb.ProcessRelatedInfoItem{newOsTypeProcessItem(0, "127.0.0.1")}

	procs := s.buildProcessEntities(kt, data, "default")

	if len(procs) != 1 {
		t.Fatalf("processes count = %d, want 1", len(procs))
	}
	if got := procs[0].Spec.OsType; got != "linux" {
		t.Fatalf("process os_type = %q, want linux", got)
	}
}

// TestResolveOsType 校验进程重建时的 os_type 兜底：新值为空沿用旧值，避免空值覆盖（R-002）
func TestResolveOsType(t *testing.T) {
	cases := []struct {
		name      string
		newOsType string
		oldOsType string
		want      string
	}{
		{"new non-empty wins", "win", "linux", "win"},
		{"new empty keeps old", "", "linux", "linux"},
		{"both empty", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveOsType(c.newOsType, c.oldOsType); got != c.want {
				t.Fatalf("resolveOsType(%q, %q) = %q, want %q", c.newOsType, c.oldOsType, got, c.want)
			}
		})
	}
}

// fakeReusableProcessDao 仅实现 reusable 分支所需的进程查询
type fakeReusableProcessDao struct {
	dao.Process
	reusable *table.Process
}

func (d *fakeReusableProcessDao) GetByCcProcessIDAndAliasTx(
	_ *kit.Kit, _ *gen.QueryTx, _, _ uint32, _ string) (*table.Process, error) {
	return d.reusable, nil
}

// fakeEmptyInstanceDao 进程实例查询恒返回空，使 isSafe 判定与扩缩容均无副作用
type fakeEmptyInstanceDao struct {
	dao.ProcessInstance
}

func (d *fakeEmptyInstanceDao) ListByProcessIDTx(
	_ *kit.Kit, _ *gen.QueryTx, _ uint32, _ uint32) ([]*table.ProcessInstance, error) {
	return nil, nil
}

// fakeReusableDaoSet 组合上述两个 fake DAO
type fakeReusableDaoSet struct {
	dao.Set
	proc *fakeReusableProcessDao
	inst *fakeEmptyInstanceDao
}

func (s *fakeReusableDaoSet) Process() dao.Process                 { return s.proc }
func (s *fakeReusableDaoSet) ProcessInstance() dao.ProcessInstance { return s.inst }

// TestBuildProcessChangesReusableResolvesOsType 校验别名变更复用 deleted 记录恢复进程时，
// 恢复后的进程 os_type 与主机一致：新值非空采用新值，新值为空沿用旧进程值（R-002）
func TestBuildProcessChangesReusableResolvesOsType(t *testing.T) {
	cases := []struct {
		name         string
		newOsType    string
		reusableOs   string
		oldOsType    string
		wantRestored string
	}{
		{"cmdb non-empty applied", "linux", "", "win", "linux"},
		{"cmdb empty keeps old", "", "aix", "win", "win"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reusable := &table.Process{
				ID:         9,
				Attachment: &table.ProcessAttachment{BizID: 3, CcProcessID: 1000, ModuleID: 10, HostID: 100},
				Spec:       &table.ProcessSpec{Alias: "new-alias", OsType: c.reusableOs, SourceData: "{}"},
			}
			daoSet := &fakeReusableDaoSet{
				proc: &fakeReusableProcessDao{reusable: reusable},
				inst: &fakeEmptyInstanceDao{},
			}
			ctx := &SyncContext{
				Kit:           kit.New(),
				Dao:           daoSet,
				Now:           time.Now(),
				HostCounter:   make(map[HostProcessKey]int),
				ModuleCounter: make(map[ModuleAliasKey]int),
			}
			newP := &table.Process{
				Attachment: &table.ProcessAttachment{BizID: 3, CcProcessID: 1000, ModuleID: 10, HostID: 100},
				Spec:       &table.ProcessSpec{Alias: "new-alias", OsType: c.newOsType, SourceData: "{}"},
			}
			oldP := &table.Process{
				ID:         5,
				Attachment: &table.ProcessAttachment{BizID: 3, CcProcessID: 1000, ModuleID: 10, HostID: 100},
				Spec:       &table.ProcessSpec{Alias: "old-alias", OsType: c.oldOsType, SourceData: "{}"},
			}

			res, err := BuildProcessChanges(ctx, &BuildProcessChangesParams{NewProcess: newP, OldProcess: oldP})
			if err != nil {
				t.Fatalf("BuildProcessChanges failed: %v", err)
			}
			if res.ToUpdateProcess == nil {
				t.Fatal("expected ToUpdateProcess (restored reusable process)")
			}
			if got := res.ToUpdateProcess.Spec.OsType; got != c.wantRestored {
				t.Fatalf("restored process os_type = %q, want %q", got, c.wantRestored)
			}
		})
	}
}

// TestBuildProcessEntitiesOsTypeFetchErrorDoesNotBlock 校验 list_hosts 异常时不阻断进程构建，
// os_type 落为空字符串（不改变同步任务整体容错机制）
func TestBuildProcessEntitiesOsTypeFetchErrorDoesNotBlock(t *testing.T) {
	svc := &osTypeStubCMDB{err: errors.New("cmdb unavailable")}
	s := &syncCMDBService{bizID: 3, svc: svc}
	kt := kit.New()

	data := []*bkcmdb.ProcessRelatedInfoItem{newOsTypeProcessItem(0, "127.0.0.1")}

	procs := s.buildProcessEntities(kt, data, "default")

	if len(procs) != 1 {
		t.Fatalf("processes count = %d, want 1", len(procs))
	}
	if got := procs[0].Spec.OsType; got != "" {
		t.Fatalf("process os_type = %q, want empty on fetch error", got)
	}
}
