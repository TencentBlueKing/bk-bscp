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

package expression

import (
	"reflect"
	"testing"
)

// 黄金语料取自 gsekit tests.py TestSerializers.test_gen_expression。
func TestGenExpression(t *testing.T) {
	s := ExpressionScope{
		Environment:  "3",
		SetName:      "set",
		ModuleName:   "*",
		ServiceName:  "127.0.0.1_proc_name",
		ProcessAlias: "*",
		ProcessID:    "50",
	}
	want := "set" + ExpressionSplitter + "*" + ExpressionSplitter + "127.0.0.1_proc_name" +
		ExpressionSplitter + "*" + ExpressionSplitter + "50"
	if got := GenExpression(s); got != want {
		t.Errorf("GenExpression = %q, want %q", got, want)
	}
}

// 缺省字段应回退为 `*`。
func TestGenExpressionDefaults(t *testing.T) {
	s := ExpressionScope{Environment: "3", SetName: "set"}
	want := "set" + ExpressionSplitter + "*" + ExpressionSplitter + "*" +
		ExpressionSplitter + "*" + ExpressionSplitter + "*"
	if got := GenExpression(s); got != want {
		t.Errorf("GenExpression defaults = %q, want %q", got, want)
	}
}

// 端到端：表达式范围 → 命中 CC 进程 ID（对齐 expression_scope_to_scope 6 步）。
func TestExpressionScopeToCcIDs(t *testing.T) {
	candidates := []Candidate{
		{Expression: JoinProcessExpression("管控平台", "m1", "svc1", "procA", "46"), CcProcessID: 46},
		{Expression: JoinProcessExpression("PaaS平台", "m2", "svc2", "procB", "48"), CcProcessID: 48},
		{Expression: JoinProcessExpression("其它集群", "m3", "svc3", "procC", "49"), CcProcessID: 49},
	}
	// 集群名为管控平台或 PaaS平台，进程 ID 为 46/48/49
	s := ExpressionScope{
		Environment:  "3",
		SetName:      "[管控平台, PaaS平台]",
		ModuleName:   "*",
		ServiceName:  "*",
		ProcessAlias: "*",
		ProcessID:    "4[6, 8, 9]",
	}
	got, err := ExpressionScopeToCcIDs(s, candidates)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	want := []uint32{46, 48}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpressionScopeToCcIDs = %v, want %v", got, want)
	}
}

// 切片：先匹配得到 ID 列表，再对结果列表切片。
func TestExpressionScopeToCcIDsWithSlice(t *testing.T) {
	candidates := []Candidate{
		{Expression: JoinProcessExpression("set", "m", "svc", "p", "10"), CcProcessID: 10},
		{Expression: JoinProcessExpression("set", "m", "svc", "p", "20"), CcProcessID: 20},
		{Expression: JoinProcessExpression("set", "m", "svc", "p", "30"), CcProcessID: 30},
	}
	s := ExpressionScope{
		Environment: "3",
		SetName:     "*",
		ProcessID:   "[0:2]", // 匹配全部后取前两个
	}
	got, err := ExpressionScopeToCcIDs(s, candidates)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	want := []uint32{10, 20}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpressionScopeToCcIDs slice = %v, want %v", got, want)
	}
}
