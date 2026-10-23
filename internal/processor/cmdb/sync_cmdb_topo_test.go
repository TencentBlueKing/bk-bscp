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
	"testing"
	"time"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// newTopoSyncContext 构造带空实例 DAO（isSafeToUpdateProcess 判定为安全）的同步上下文
func newTopoSyncContext(daoSet *fakeReusableDaoSet) *SyncContext {
	return &SyncContext{
		Kit:           kit.New(),
		Dao:           daoSet,
		Now:           time.Now(),
		ModuleCounter: make(map[ModuleAliasKey]int),
	}
}

// TestBuildProcessChangesTopoFields 校验 F-001：一键同步对 set_name / module_name / service_name /
// environment 四个拓扑字段做增量 diff 与直接覆盖写回（watch 链已不再订阅 set 资源，集群名只能由全量同步
// 覆盖；module_name 虽由 module 事件更新，但事件处理失败后游标照常推进，同样需要全量同步兜底）
// （范围决策 2026-07-21，2026-10 增补 set_name，2026-10 增补 module_name）
func TestBuildProcessChangesTopoFields(t *testing.T) {
	cases := []struct {
		name            string
		oldServiceName  string
		newServiceName  string
		oldEnvironment  string
		newEnvironment  string
		oldSetName      string
		newSetName      string
		oldModuleName   string
		newModuleName   string
		wantUpdate      bool
		wantSetName     string
		wantModuleName  string
		wantServiceName string
		wantEnvironment string
	}{
		{
			name:           "T1 only service_name changed",
			oldServiceName: "svc-old", newServiceName: "svc-new",
			oldEnvironment: "1", newEnvironment: "1",
			wantUpdate:      true,
			wantServiceName: "svc-new", wantEnvironment: "1",
		},
		{
			name:           "T2 only environment changed",
			oldServiceName: "svc", newServiceName: "svc",
			oldEnvironment: "1", newEnvironment: "3",
			wantUpdate:      true,
			wantServiceName: "svc", wantEnvironment: "3",
		},
		{
			name:           "T3 only set_name changed",
			oldServiceName: "svc", newServiceName: "svc",
			oldEnvironment: "1", newEnvironment: "1",
			oldSetName: "set-old", newSetName: "set-new",
			wantUpdate:  true,
			wantSetName: "set-new", wantServiceName: "svc", wantEnvironment: "1",
		},
		{
			name:           "T4 only module_name changed",
			oldServiceName: "svc", newServiceName: "svc",
			oldEnvironment: "1", newEnvironment: "1",
			oldModuleName: "mod-old", newModuleName: "mod-new",
			wantUpdate:     true,
			wantModuleName: "mod-new", wantServiceName: "svc", wantEnvironment: "1",
		},
		{
			name:           "T5 topo fields unchanged",
			oldServiceName: "svc", newServiceName: "svc",
			oldEnvironment: "1", newEnvironment: "1",
			oldSetName: "set", newSetName: "set",
			wantUpdate: false,
		},
		{
			name:           "T6 all fields overwritten to empty",
			oldServiceName: "svc-old", newServiceName: "",
			oldEnvironment: "1", newEnvironment: "",
			oldSetName: "set-old", newSetName: "",
			oldModuleName: "mod-old", newModuleName: "",
			wantUpdate:  true,
			wantSetName: "", wantModuleName: "", wantServiceName: "", wantEnvironment: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			daoSet := &fakeReusableDaoSet{
				proc: &fakeReusableProcessDao{reusable: nil},
				inst: &fakeEmptyInstanceDao{},
			}
			ctx := newTopoSyncContext(daoSet)
			// agent_id 两侧一致且非空，避免触发 agent 字段兜底干扰拓扑字段断言
			newP := &table.Process{
				Attachment: &table.ProcessAttachment{BizID: 3, CcProcessID: 1000, ModuleID: 10,
					HostID: 100, AgentID: "agent-1"},
				Spec: &table.ProcessSpec{
					Alias:       "alias",
					SourceData:  "{}",
					ServiceName: c.newServiceName,
					Environment: c.newEnvironment,
					SetName:     c.newSetName,
					ModuleName:  c.newModuleName,
				},
			}
			oldP := &table.Process{
				ID: 5,
				Attachment: &table.ProcessAttachment{BizID: 3, CcProcessID: 1000, ModuleID: 10,
					HostID: 100, AgentID: "agent-1"},
				Spec: &table.ProcessSpec{
					Alias:       "alias",
					SourceData:  "{}",
					ServiceName: c.oldServiceName,
					Environment: c.oldEnvironment,
					SetName:     c.oldSetName,
					ModuleName:  c.oldModuleName,
				},
			}

			res, err := BuildProcessChanges(ctx, &BuildProcessChangesParams{NewProcess: newP, OldProcess: oldP})
			if err != nil {
				t.Fatalf("BuildProcessChanges failed: %v", err)
			}

			if !c.wantUpdate {
				if res.ToUpdateProcess != nil {
					t.Fatalf("expected no update, got %+v", res.ToUpdateProcess.Spec)
				}
				return
			}

			if res.ToUpdateProcess == nil {
				t.Fatal("expected ToUpdateProcess non-nil")
			}
			if got := res.ToUpdateProcess.Spec.SetName; got != c.wantSetName {
				t.Fatalf("SetName = %q, want %q", got, c.wantSetName)
			}
			if got := res.ToUpdateProcess.Spec.ModuleName; got != c.wantModuleName {
				t.Fatalf("ModuleName = %q, want %q", got, c.wantModuleName)
			}
			if got := res.ToUpdateProcess.Spec.ServiceName; got != c.wantServiceName {
				t.Fatalf("ServiceName = %q, want %q", got, c.wantServiceName)
			}
			if got := res.ToUpdateProcess.Spec.Environment; got != c.wantEnvironment {
				t.Fatalf("Environment = %q, want %q", got, c.wantEnvironment)
			}
		})
	}
}

// TestBuildProcessChangesReusableRefreshesTopoFields 校验 TR-001：别名 + 拓扑字段同时变更且命中
// 可复用 deleted 记录时，恢复进程的 set_name / module_name / service_name / environment
// 刷新为 CMDB 新值、不残留旧值
func TestBuildProcessChangesReusableRefreshesTopoFields(t *testing.T) {
	reusable := &table.Process{
		ID:         9,
		Attachment: &table.ProcessAttachment{BizID: 3, CcProcessID: 1000, ModuleID: 10, HostID: 100},
		Spec: &table.ProcessSpec{
			Alias:       "new-alias",
			SourceData:  "{}",
			SetName:     "set-stale",
			ModuleName:  "mod-stale",
			ServiceName: "svc-stale",
			Environment: "1",
		},
	}
	daoSet := &fakeReusableDaoSet{
		proc: &fakeReusableProcessDao{reusable: reusable},
		inst: &fakeEmptyInstanceDao{},
	}
	ctx := newTopoSyncContext(daoSet)

	newP := &table.Process{
		Attachment: &table.ProcessAttachment{BizID: 3, CcProcessID: 1000, ModuleID: 10, HostID: 100},
		Spec: &table.ProcessSpec{
			Alias:       "new-alias",
			SourceData:  "{}",
			SetName:     "set-new",
			ModuleName:  "mod-new",
			ServiceName: "svc-new",
			Environment: "3",
		},
	}
	oldP := &table.Process{
		ID:         5,
		Attachment: &table.ProcessAttachment{BizID: 3, CcProcessID: 1000, ModuleID: 10, HostID: 100},
		Spec: &table.ProcessSpec{
			Alias:       "old-alias",
			SourceData:  "{}",
			ServiceName: "svc-old",
			Environment: "1",
		},
	}

	res, err := BuildProcessChanges(ctx, &BuildProcessChangesParams{NewProcess: newP, OldProcess: oldP})
	if err != nil {
		t.Fatalf("BuildProcessChanges failed: %v", err)
	}
	if res.ToUpdateProcess == nil {
		t.Fatal("expected ToUpdateProcess (restored reusable process)")
	}
	if got := res.ToUpdateProcess.Spec.SetName; got != "set-new" {
		t.Fatalf("restored SetName = %q, want set-new", got)
	}
	if got := res.ToUpdateProcess.Spec.ModuleName; got != "mod-new" {
		t.Fatalf("restored ModuleName = %q, want mod-new", got)
	}
	if got := res.ToUpdateProcess.Spec.ServiceName; got != "svc-new" {
		t.Fatalf("restored ServiceName = %q, want svc-new", got)
	}
	if got := res.ToUpdateProcess.Spec.Environment; got != "3" {
		t.Fatalf("restored Environment = %q, want 3", got)
	}
}
