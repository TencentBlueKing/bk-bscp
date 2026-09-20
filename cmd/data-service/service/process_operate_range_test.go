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
	"encoding/json"
	"testing"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	pbproc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
)

// TestBuildOperateRangePluginRaw 插件路径：原样记录请求 expression_scope 五段，缺省段补 "*"（AC-001/AC-T01）。
func TestBuildOperateRangePluginRaw(t *testing.T) {
	operateRange := &pbproc.OperateRange{
		Environment: "1",
		ExpressionScope: &pbproc.ExpressionScope{
			SetName:   "[管控平台,PaaS平台]",
			ProcessId: "4[6,8,9]",
			// module/service/alias 留空，期望回退为 "*"
		},
	}

	got := buildOperateRange(nil, operateRange)
	want := table.OperateRange{
		SetName:      "[管控平台,PaaS平台]",
		ModuleName:   "*",
		ServiceName:  "*",
		ProcessAlias: "*",
		ProcessID:    "4[6,8,9]",
	}
	if got != want {
		t.Fatalf("plugin buildOperateRange = %+v, want %+v", got, want)
	}
}

// TestBuildOperateRangeNonPlugin 非插件路径：命中进程 CC 进程 ID 拼压缩表达式记入 process_id，
// 其余段 "*"（AC-T03）。
func TestBuildOperateRangeNonPlugin(t *testing.T) {
	procs := []*table.Process{
		{Attachment: &table.ProcessAttachment{CcProcessID: 6}},
		{Attachment: &table.ProcessAttachment{CcProcessID: 7}},
		{Attachment: &table.ProcessAttachment{CcProcessID: 8}},
	}
	got := buildOperateRange(procs, nil)
	want := table.OperateRange{
		SetName:      "*",
		ModuleName:   "*",
		ServiceName:  "*",
		ProcessAlias: "*",
		ProcessID:    "[6-8]",
	}
	if got != want {
		t.Fatalf("non-plugin buildOperateRange = %+v, want %+v", got, want)
	}
}

// TestFilterScaledDownInstances 一键清除缩容实例筛选：实例按 host_inst_seq 升序排列后
// 序位超过 proc_num 的为缩容实例，按 host_inst_seq 降序返回（从最后一个实例开始清除）；
// proc_num 与实例数量一致时无缩容实例，返回空。
func TestFilterScaledDownInstances(t *testing.T) {
	newInst := func(id, seq uint32) *table.ProcessInstance {
		return &table.ProcessInstance{
			ID:   id,
			Spec: &table.ProcessInstanceSpec{HostInstSeq: seq},
		}
	}

	// host_inst_seq 乱序输入：3、1、2
	instances := []*table.ProcessInstance{
		newInst(101, 3),
		newInst(102, 1),
		newInst(103, 2),
	}

	// proc_num=1，实例 3 个：缩容 2 个，从最后一个实例（host_inst_seq=3）开始降序返回
	proc := &table.Process{Spec: &table.ProcessSpec{ProcNum: 1}}
	got := filterScaledDownInstances(proc, instances)
	wantIDs := []uint32{101, 103}
	if len(got) != len(wantIDs) {
		t.Fatalf("filterScaledDownInstances len = %d, want %d", len(got), len(wantIDs))
	}
	for i, inst := range got {
		if inst.ID != wantIDs[i] {
			t.Fatalf("filterScaledDownInstances[%d] = instance %d, want %d", i, inst.ID, wantIDs[i])
		}
	}

	// proc_num 与实例数量一致（3=3）时无缩容实例，返回空
	proc.Spec.ProcNum = 3
	if got := filterScaledDownInstances(proc, instances); len(got) != 0 {
		t.Fatalf("filterScaledDownInstances = %v, want empty", got)
	}
}

// TestBuildConfigOperateRange 配置链路 buildOperateRange：插件模式原样存请求表达式；
// 非插件模式拼命中进程压缩表达式（AC-002/AC-T03）。
func TestBuildConfigOperateRange(t *testing.T) {
	s := &Service{}

	plugin := s.buildOperateRange(nil, true, &pbproc.OperateRange{
		ExpressionScope: &pbproc.ExpressionScope{
			ModuleName: "gse",
			ProcessId:  "[1-100]",
		},
	})
	wantPlugin := table.OperateRange{
		SetName:      "*",
		ModuleName:   "gse",
		ServiceName:  "*",
		ProcessAlias: "*",
		ProcessID:    "[1-100]",
	}
	if plugin != wantPlugin {
		t.Fatalf("config plugin buildOperateRange = %+v, want %+v", plugin, wantPlugin)
	}

	procs := []*table.Process{
		{Attachment: &table.ProcessAttachment{CcProcessID: 9}},
		{Attachment: &table.ProcessAttachment{CcProcessID: 8}},
		{Attachment: &table.ProcessAttachment{CcProcessID: 6}},
		{Attachment: &table.ProcessAttachment{CcProcessID: 6}},
	}
	nonPlugin := s.buildOperateRange(procs, false, nil)
	wantNonPlugin := table.OperateRange{
		SetName:      "*",
		ModuleName:   "*",
		ServiceName:  "*",
		ProcessAlias: "*",
		ProcessID:    "[6,8-9]",
	}
	if nonPlugin != wantNonPlugin {
		t.Fatalf("config non-plugin buildOperateRange = %+v, want %+v", nonPlugin, wantNonPlugin)
	}
}

// TestProcessConfigsEqual 更新托管下发预检的配置一致性对比：
// 字段一致（与字段顺序无关）视为相等，任一侧解析失败视为不一致。
func TestProcessConfigsEqual(t *testing.T) {
	full := table.ProcessInfo{
		BkStartParamRegex: ".*bin",
		WorkPath:          "/data/bkapp",
		PidFile:           "/data/bkapp/pid",
		User:              "root",
		StartCmd:          "start.sh",
		StopCmd:           "stop.sh",
	}

	fullJSON, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("marshal process info failed: %v", err)
	}

	cases := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{"两者一致", string(fullJSON), string(fullJSON), true},
		{"字段顺序不同但内容一致", `{"user":"root","work_path":"/data/bkapp"}`, `{"work_path":"/data/bkapp","user":"root"}`, true},
		{"字段值不同", string(fullJSON), `{"work_path":"/data/other"}`, false},
		{"空对象一致", `{}`, `{}`, true},
		{"解析失败视为不一致", `{"work_path":"/data"`, `{}`, false},
	}

	for _, ct := range cases {
		t.Run(ct.name, func(t *testing.T) {
			if got := processConfigsEqual(ct.a, ct.b); got != ct.want {
				t.Fatalf("processConfigsEqual(%q, %q) = %v, want %v", ct.a, ct.b, got, ct.want)
			}
		})
	}
}
