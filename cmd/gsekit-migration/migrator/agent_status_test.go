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

package migrator

import "testing"

func TestResolveAgentStatus(t *testing.T) {
	// status_code 含义：2=运行中，其余为非运行；映射规则须与运行时 buildProcessEntities 一致
	statusMap := map[string]int{"a-running": 2, "a-busy": 4, "a-stopped": 6}

	cases := []struct {
		name    string
		agentID string
		want    string
	}{
		{"running maps to normal", "a-running", "normal"},
		{"busy maps to abnormal", "a-busy", "abnormal"},
		{"stopped maps to abnormal", "a-stopped", "abnormal"},
		{"missing in map maps to abnormal", "a-unknown", "abnormal"},
		{"empty agent id maps to abnormal", "", "abnormal"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// queryFailed=false：GSE 查询成功，按真实状态映射
			if got := resolveAgentStatus(c.agentID, statusMap, false); got != c.want {
				t.Errorf("resolveAgentStatus(%q) = %q, want %q", c.agentID, got, c.want)
			}
		})
	}
}

func TestResolveAgentStatusQueryFailedDefaultsNormal(t *testing.T) {
	// 查询失败时兜底为 normal，避免迁移后用户页面出现大量 agent 异常误报；
	// 后续周期性 CMDB 同步会自动纠正为真实状态。
	cases := []struct {
		name    string
		agentID string
	}{
		{"non-empty agent id", "a1"},
		{"empty agent id", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveAgentStatus(c.agentID, nil, true); got != "normal" {
				t.Errorf("resolveAgentStatus(queryFailed) = %q, want normal", got)
			}
		})
	}
}

func TestCollectAgentIDs(t *testing.T) {
	procs := []GSEKitProcess{
		{BkAgentID: "a1"},
		{BkAgentID: ""},
		{BkAgentID: "a1"},
		{BkAgentID: "a2"},
		{BkAgentID: ""},
	}

	got := collectAgentIDs(procs)

	if len(got) != 2 {
		t.Fatalf("expected 2 unique non-empty agent ids, got %d: %v", len(got), got)
	}
	seen := make(map[string]bool, len(got))
	for _, id := range got {
		if id == "" {
			t.Errorf("collectAgentIDs returned empty agent id")
		}
		if seen[id] {
			t.Errorf("collectAgentIDs returned duplicate agent id %q", id)
		}
		seen[id] = true
	}
	if !seen["a1"] || !seen["a2"] {
		t.Errorf("collectAgentIDs missing expected ids, got %v", got)
	}
}
