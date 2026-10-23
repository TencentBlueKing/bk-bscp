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

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// TestResolveAgentStatus agent_id 为空时 agent_status 一律为 abnormal
func TestResolveAgentStatus(t *testing.T) {
	cases := []struct {
		name    string
		agentID string
		status  table.AgentStatus
		want    table.AgentStatus
	}{
		{"empty agent id forces abnormal", "", table.AgentStatusNormal, table.AgentStatusAbnormal},
		{"empty agent id and empty status", "", "", table.AgentStatusAbnormal},
		{"normal kept", "agent-1", table.AgentStatusNormal, table.AgentStatusNormal},
		{"abnormal kept", "agent-1", table.AgentStatusAbnormal, table.AgentStatusAbnormal},
		{"empty status kept", "agent-1", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveAgentStatus(c.agentID, c.status); got != c.want {
				t.Fatalf("resolveAgentStatus(%q, %q) = %q, want %q", c.agentID, c.status, got, c.want)
			}
		})
	}
}

// newAgentIDSyncContext 构建仅需进程实例查询的同步上下文（无实例 → 判定为可安全更新）
func newAgentIDSyncContext() *SyncContext {
	return &SyncContext{
		Kit: kit.New(),
		Dao: &fakeReusableDaoSet{
			proc: &fakeReusableProcessDao{},
			inst: &fakeEmptyInstanceDao{},
		},
		Now:           time.Now(),
		ModuleCounter: make(map[ModuleAliasKey]int),
	}
}

func newAgentIDProcessPair(newAgentID, oldAgentID string, newStatus,
	oldStatus table.AgentStatus) (*table.Process, *table.Process) {
	newP := &table.Process{
		Attachment: &table.ProcessAttachment{BizID: 3, CcProcessID: 1000, ModuleID: 10,
			HostID: 100, AgentID: newAgentID},
		Spec: &table.ProcessSpec{Alias: "proc1", SourceData: "{}", AgentStatus: newStatus},
	}
	oldP := &table.Process{
		ID: 5,
		Attachment: &table.ProcessAttachment{BizID: 3, CcProcessID: 1000, ModuleID: 10,
			HostID: 100, AgentID: oldAgentID},
		Spec: &table.ProcessSpec{Alias: "proc1", SourceData: "{}", AgentStatus: oldStatus},
	}
	return newP, oldP
}

// TestBuildProcessChangesRefreshesAgentID 仅 agent_id 变化（含本地为空的存量数据）也会产生更新，
// 且刷新为 CMDB 值
func TestBuildProcessChangesRefreshesAgentID(t *testing.T) {
	cases := []struct {
		name        string
		newAgentID  string
		oldAgentID  string
		wantAgentID string
	}{
		{"empty local filled by cmdb", "agent-new", "", "agent-new"},
		{"stale local refreshed", "agent-new", "agent-old", "agent-new"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			newP, oldP := newAgentIDProcessPair(c.newAgentID, c.oldAgentID,
				table.AgentStatusNormal, table.AgentStatusNormal)

			res, err := BuildProcessChanges(newAgentIDSyncContext(),
				&BuildProcessChangesParams{NewProcess: newP, OldProcess: oldP})
			if err != nil {
				t.Fatalf("BuildProcessChanges failed: %v", err)
			}
			if res.ToUpdateProcess == nil {
				t.Fatal("expected ToUpdateProcess for agent_id change")
			}
			if got := res.ToUpdateProcess.Attachment.AgentID; got != c.wantAgentID {
				t.Fatalf("updated agent_id = %q, want %q", got, c.wantAgentID)
			}
			if got := res.ToUpdateProcess.Spec.AgentStatus; got != table.AgentStatusNormal {
				t.Fatalf("updated agent_status = %q, want normal", got)
			}
		})
	}
}

// TestBuildProcessChangesClearsAgentIDWhenCMDBEmpty 主机注销或 agent 卸载后 CMDB 返回空 agent_id，
// 本地随之清空并置为 abnormal，不保留失效的 agent 标识
func TestBuildProcessChangesClearsAgentIDWhenCMDBEmpty(t *testing.T) {
	newP, oldP := newAgentIDProcessPair("", "agent-old",
		table.AgentStatusNormal, table.AgentStatusNormal)

	res, err := BuildProcessChanges(newAgentIDSyncContext(),
		&BuildProcessChangesParams{NewProcess: newP, OldProcess: oldP})
	if err != nil {
		t.Fatalf("BuildProcessChanges failed: %v", err)
	}
	if res.ToUpdateProcess == nil {
		t.Fatal("expected ToUpdateProcess for agent_id clearing")
	}
	if got := res.ToUpdateProcess.Attachment.AgentID; got != "" {
		t.Fatalf("updated agent_id = %q, want empty", got)
	}
	if got := res.ToUpdateProcess.Spec.AgentStatus; got != table.AgentStatusAbnormal {
		t.Fatalf("updated agent_status = %q, want abnormal", got)
	}
}

// TestBuildProcessChangesForcesAbnormalOnEmptyAgentID agent_id 为空的存量数据，
// 同步时 agent_status 被纠正为 abnormal
func TestBuildProcessChangesForcesAbnormalOnEmptyAgentID(t *testing.T) {
	newP, oldP := newAgentIDProcessPair("", "", table.AgentStatusNormal, table.AgentStatusNormal)

	res, err := BuildProcessChanges(newAgentIDSyncContext(),
		&BuildProcessChangesParams{NewProcess: newP, OldProcess: oldP})
	if err != nil {
		t.Fatalf("BuildProcessChanges failed: %v", err)
	}
	if res.ToUpdateProcess == nil {
		t.Fatal("expected ToUpdateProcess for agent_status correction")
	}
	if got := res.ToUpdateProcess.Spec.AgentStatus; got != table.AgentStatusAbnormal {
		t.Fatalf("updated agent_status = %q, want abnormal", got)
	}
	if got := res.ToUpdateProcess.Attachment.AgentID; got != "" {
		t.Fatalf("updated agent_id = %q, want empty", got)
	}
}

// TestBuildProcessChangesNoChangeWhenAgentIDEqual agent_id 一致时不产生多余更新
func TestBuildProcessChangesNoChangeWhenAgentIDEqual(t *testing.T) {
	newP, oldP := newAgentIDProcessPair("agent-1", "agent-1",
		table.AgentStatusNormal, table.AgentStatusNormal)

	res, err := BuildProcessChanges(newAgentIDSyncContext(),
		&BuildProcessChangesParams{NewProcess: newP, OldProcess: oldP})
	if err != nil {
		t.Fatalf("BuildProcessChanges failed: %v", err)
	}
	if res.ToUpdateProcess != nil || res.ToAddProcess != nil || res.ToDeleteProcess != nil {
		t.Fatal("expected no change when agent_id and other fields are equal")
	}
}

// TestBuildProcessEntitiesEmptyAgentIDIsAbnormal 全量同步落库时空 agent_id 的 agent_status 为 abnormal
func TestBuildProcessEntitiesEmptyAgentIDIsAbnormal(t *testing.T) {
	svc := &osTypeStubCMDB{}
	s := &syncCMDBService{bizID: 3, svc: svc}

	data := []*bkcmdb.ProcessRelatedInfoItem{newOsTypeProcessItem(0, "127.0.0.1")}

	procs := s.buildProcessEntities(kit.New(), data, "default")

	if len(procs) != 1 {
		t.Fatalf("processes count = %d, want 1", len(procs))
	}
	if got := procs[0].Attachment.AgentID; got != "" {
		t.Fatalf("agent_id = %q, want empty", got)
	}
	if got := procs[0].Spec.AgentStatus; got != table.AgentStatusAbnormal {
		t.Fatalf("agent_status = %q, want abnormal", got)
	}
}

func TestValidateProcessRelatedInfoRejectsIncompleteIdentity(t *testing.T) {
	valid := newOsTypeProcessItem(0, "127.0.0.1")
	cases := []struct {
		name string
		item *bkcmdb.ProcessRelatedInfoItem
	}{
		{name: "nil association", item: nil},
		{name: "missing process", item: &bkcmdb.ProcessRelatedInfoItem{}},
		{name: "negative process ID", item: func() *bkcmdb.ProcessRelatedInfoItem {
			copy := *valid
			process := *valid.Process
			process.BkProcessID = -1
			copy.Process = &process
			return &copy
		}()},
		{name: "process ID overflow", item: func() *bkcmdb.ProcessRelatedInfoItem {
			copy := *valid
			process := *valid.Process
			process.BkProcessID = 1 << 32
			copy.Process = &process
			return &copy
		}()},
		{name: "wrong business", item: func() *bkcmdb.ProcessRelatedInfoItem {
			copy := *valid
			process := *valid.Process
			process.BkBizID = 4
			copy.Process = &process
			return &copy
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateProcessRelatedInfo([]*bkcmdb.ProcessRelatedInfoItem{tc.item}, 3); err == nil {
				t.Fatal("expected invalid process association to fail validation")
			}
		})
	}
	if err := validateProcessRelatedInfo([]*bkcmdb.ProcessRelatedInfoItem{valid}, 3); err != nil {
		t.Fatalf("valid association rejected: %v", err)
	}
}

// TestValidateProcessRelatedInfoAllowsMissingProcessTemplate 非服务模板创建的进程，
// CC 可能不返回 process_template（或返回 0）；此类条目应通过校验，落库模板 ID 为 0
func TestValidateProcessRelatedInfoAllowsMissingProcessTemplate(t *testing.T) {
	nilTemplate := newOsTypeProcessItem(0, "127.0.0.1")
	nilTemplate.ProcessTemplate = nil

	zeroTemplate := newOsTypeProcessItem(0, "127.0.0.1")
	zeroTemplate.ProcessTemplate = &bkcmdb.ProcessTemplateRefInfo{}

	items := map[string]*bkcmdb.ProcessRelatedInfoItem{
		"nil template":  nilTemplate,
		"zero template": zeroTemplate,
	}
	for name, item := range items {
		if err := validateProcessRelatedInfo([]*bkcmdb.ProcessRelatedInfoItem{item}, 3); err != nil {
			t.Fatalf("%s association rejected: %v", name, err)
		}
	}

	s := &syncCMDBService{bizID: 3}
	procs := s.buildProcessEntities(kit.New(), []*bkcmdb.ProcessRelatedInfoItem{nilTemplate}, "default")
	if len(procs) != 1 {
		t.Fatalf("processes count = %d, want 1", len(procs))
	}
	if got := procs[0].Attachment.ProcessTemplateID; got != 0 {
		t.Fatalf("ProcessTemplateID = %d, want 0", got)
	}
}
