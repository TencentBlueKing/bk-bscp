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
		HostCounter:   make(map[HostProcessKey]int),
		ModuleCounter: make(map[ModuleAliasKey]int),
	}
}

// TestBuildProcessChangesTopoFields 校验 F-001：一键同步只对 service_name / environment 两个拓扑字段
// 做增量 diff 与直接覆盖写回；set_name / module_name 变化不触发更新（范围决策 2026-07-21）
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
			name:           "T3 only set_name/module_name changed",
			oldServiceName: "svc", newServiceName: "svc",
			oldEnvironment: "1", newEnvironment: "1",
			oldSetName: "set-old", newSetName: "set-new",
			oldModuleName: "mod-old", newModuleName: "mod-new",
			wantUpdate: false,
		},
		{
			name:           "T4 topo fields unchanged",
			oldServiceName: "svc", newServiceName: "svc",
			oldEnvironment: "1", newEnvironment: "1",
			wantUpdate: false,
		},
		{
			name:           "T5 both fields overwritten to empty",
			oldServiceName: "svc-old", newServiceName: "",
			oldEnvironment: "1", newEnvironment: "",
			wantUpdate:      true,
			wantServiceName: "", wantEnvironment: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			daoSet := &fakeReusableDaoSet{
				proc: &fakeReusableProcessDao{reusable: nil},
				inst: &fakeEmptyInstanceDao{},
			}
			ctx := newTopoSyncContext(daoSet)
			newP := &table.Process{
				Attachment: &table.ProcessAttachment{BizID: 3, CcProcessID: 1000, ModuleID: 10, HostID: 100},
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
				ID:         5,
				Attachment: &table.ProcessAttachment{BizID: 3, CcProcessID: 1000, ModuleID: 10, HostID: 100},
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
			if got := res.ToUpdateProcess.Spec.ServiceName; got != c.wantServiceName {
				t.Fatalf("ServiceName = %q, want %q", got, c.wantServiceName)
			}
			if got := res.ToUpdateProcess.Spec.Environment; got != c.wantEnvironment {
				t.Fatalf("Environment = %q, want %q", got, c.wantEnvironment)
			}
			// set_name / module_name 不属于同步范围：不因它们触发更新，也不做断言写回
		})
	}
}

// TestBuildProcessChangesReusableRefreshesTopoFields 校验 TR-001：别名 + 两个拓扑字段同时变更且命中
// 可复用 deleted 记录时，恢复进程的 service_name / environment 刷新为 CMDB 新值、不残留旧值
func TestBuildProcessChangesReusableRefreshesTopoFields(t *testing.T) {
	reusable := &table.Process{
		ID:         9,
		Attachment: &table.ProcessAttachment{BizID: 3, CcProcessID: 1000, ModuleID: 10, HostID: 100},
		Spec: &table.ProcessSpec{
			Alias:       "new-alias",
			SourceData:  "{}",
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
	if got := res.ToUpdateProcess.Spec.ServiceName; got != "svc-new" {
		t.Fatalf("restored ServiceName = %q, want svc-new", got)
	}
	if got := res.ToUpdateProcess.Spec.Environment; got != "3" {
		t.Fatalf("restored Environment = %q, want 3", got)
	}
}
