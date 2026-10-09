/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
    10| * limitations under the License.
*/

package cmdb

import (
	"context"
	"testing"
	"time"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// osTypeStubCMDB 空 CMDB 桩，供 buildProcessEntities 相关测试使用；
// 通过 list_hosts 补全 os_type，主机数据由用例注入
type osTypeStubCMDB struct {
	bkcmdb.Service
	hosts []bkcmdb.HostInfo
}

func (s *osTypeStubCMDB) ListBizHosts(_ context.Context, _ *bkcmdb.ListBizHostsRequest) (
	*bkcmdb.CMDBListData[bkcmdb.HostInfo], error) {
	return &bkcmdb.CMDBListData[bkcmdb.HostInfo]{Info: s.hosts}, nil
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

// TestBuildProcessEntitiesSetsOsType 通过 list_hosts 补全进程 os_type：
// 按云区域 + 内网 IP 关联主机，并将 CC 编码映射为业务语义（R-001）
func TestBuildProcessEntitiesSetsOsType(t *testing.T) {
	s := &syncCMDBService{
		bizID: 3,
		svc: &osTypeStubCMDB{hosts: []bkcmdb.HostInfo{
			{BkHostID: 100, BkCloudID: 0, BkHostInnerIP: "127.0.0.1", BkOSType: "2"},
		}},
	}
	data := []*bkcmdb.ProcessRelatedInfoItem{newOsTypeProcessItem(0, "127.0.0.1")}

	procs := s.buildProcessEntities(kit.New(), data, "default")

	if len(procs) != 1 {
		t.Fatalf("processes count = %d, want 1", len(procs))
	}
	if got := procs[0].Spec.OsType; got != osTypeWin {
		t.Fatalf("process os_type = %q, want %s", got, osTypeWin)
	}
}

// TestBuildProcessEntitiesOsTypeEmptyWhenHostMissing list_hosts 未命中主机时 os_type 留空，
// 不阻断同步（R-002：空值不覆盖已有非空值，由 BuildProcessChanges 空值保护兜底）
func TestBuildProcessEntitiesOsTypeEmptyWhenHostMissing(t *testing.T) {
	s := &syncCMDBService{bizID: 3, svc: &osTypeStubCMDB{}}
	data := []*bkcmdb.ProcessRelatedInfoItem{newOsTypeProcessItem(0, "127.0.0.1")}

	procs := s.buildProcessEntities(kit.New(), data, "default")

	if len(procs) != 1 {
		t.Fatalf("processes count = %d, want 1", len(procs))
	}
	if got := procs[0].Spec.OsType; got != "" {
		t.Fatalf("process os_type = %q, want empty", got)
	}
}

// TestResolveOsType 校验进程重建时的 os_type 兜底：新值为空沿用旧值，避免空值覆盖（R-002）。
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
// 恢复后的进程 os_type：新值非空采用新值，新值为空沿用旧进程值（R-002）
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
