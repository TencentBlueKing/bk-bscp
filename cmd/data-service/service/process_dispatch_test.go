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

package service

import (
	"testing"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// TestResolveDispatchItems 组装待下发任务项：
// 实例按 processID 关联进程并透传原始状态快照；实例无对应进程时报错
func TestResolveDispatchItems(t *testing.T) {
	proc := &table.Process{ID: 7}
	inst := &table.ProcessInstance{
		ID:         100,
		Attachment: &table.ProcessInstanceAttachment{ProcessID: 7},
		Spec: &table.ProcessInstanceSpec{
			Status:        table.ProcessStatusRunning,
			ManagedStatus: table.ProcessManagedStatusManaged,
		},
	}
	orphan := &table.ProcessInstance{
		ID:         101,
		Attachment: &table.ProcessInstanceAttachment{ProcessID: 8},
	}

	// 正常关联：透传操作类型与原始状态
	items, err := resolveDispatchItems(kit.New(), []*table.Process{proc},
		[]*table.ProcessInstance{inst}, table.StopProcessOperate)
	if err != nil {
		t.Fatalf("resolveDispatchItems() err = %v, want nil", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].finalOpType != table.StopProcessOperate {
		t.Fatalf("finalOpType = %v, want %v", items[0].finalOpType, table.StopProcessOperate)
	}
	if items[0].originalStatus != table.ProcessStatusRunning {
		t.Fatalf("originalStatus = %v, want %v", items[0].originalStatus, table.ProcessStatusRunning)
	}
	if items[0].originalManaged != table.ProcessManagedStatusManaged {
		t.Fatalf("originalManaged = %v, want %v", items[0].originalManaged, table.ProcessManagedStatusManaged)
	}
	if items[0].proc != proc || items[0].instance != inst {
		t.Fatalf("resolved instance/proc not attached")
	}

	// 实例无对应进程 -> 报错
	if _, err = resolveDispatchItems(kit.New(), []*table.Process{proc},
		[]*table.ProcessInstance{orphan}, table.StopProcessOperate); err == nil {
		t.Fatalf("resolveDispatchItems(orphan) err = nil, want error")
	}
}

// TestIsStartSemantic 启动语义判定：注册 / 启动 / 重启 / 重载需过滤缩容实例，其余操作不过滤
func TestIsStartSemantic(t *testing.T) {
	cases := []struct {
		operateType string
		want        bool
	}{
		{string(table.TaskActionRegister), true},
		{string(table.TaskActionStart), true},
		{string(table.TaskActionRestart), true},
		{string(table.TaskActionReload), true},
		{string(table.TaskActionStop), false},
		{string(table.TaskActionKill), false},
		{"", false},
		{"unknown", false},
	}

	for _, c := range cases {
		t.Run(c.operateType, func(t *testing.T) {
			if got := isStartSemantic(c.operateType); got != c.want {
				t.Fatalf("isStartSemantic(%q) = %v, want %v", c.operateType, got, c.want)
			}
		})
	}
}
