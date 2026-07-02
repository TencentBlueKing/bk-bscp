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

// 黄金语料取自 gsekit apps/gsekit/utils/expression_utils/tests.py TestMatch。

func TestMatch(t *testing.T) {
	got, err := Match("module-a.proc1.99", "module-[a-c].proc[1-10].[1-9999]")
	if err != nil || !got {
		t.Errorf("Match positive = %v, err=%v, want true", got, err)
	}

	got, err = Match("module-a.proc1.99", "module-[!a].proc[1-10].[1-9999]")
	if err != nil || got {
		t.Errorf("Match exclude = %v, err=%v, want false", got, err)
	}
}

func TestListMatch(t *testing.T) {
	names := []string{
		"module-a.proc1.10",
		"module-b.proc2.20",
		"module-c.proc3.30",
		"module-d.proc4.40",
		"module-e.proc5.50",
	}
	got, err := ListMatch(names, "module-[a-d].proc[!2].[10-19, 30-39]")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	want := []string{"module-a.proc1.10", "module-c.proc3.30"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListMatch = %v, want %v", got, want)
	}
}

// 跨字段场景：分隔符作为字面锚点，`*` 不应越过分隔符错配（对齐 gsekit
// test_list_match_service_name_contain_process_name）。
func TestListMatchSeparatorAnchor(t *testing.T) {
	splitter := ExpressionSplitter
	exprA := "set" + splitter + "*" + splitter + "127.0.0.1_proc_name" + splitter + "127" + splitter + "50"
	exprB := "set" + splitter + "*" + splitter + "127.0.0.1_proc_name" + splitter + "12" + splitter + "50"

	got, err := ListMatch(
		[]string{exprA, exprB},
		"*"+splitter+"*"+splitter+"*"+splitter+"127"+splitter+"*",
	)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	want := []string{exprA}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListMatch separator = %v, want %v", got, want)
	}
}

// fnmatch 通配符基础语义，对齐 Python fnmatch。
func TestMatchWildcards(t *testing.T) {
	cases := []struct {
		name, pattern string
		want          bool
	}{
		{"proc1", "proc*", true},
		{"proc1", "proc?", true},
		{"proc12", "proc?", false},
		{"proc.name", "proc*", true}, // fnmatch 的 * 跨越 '.'（不同于 shell glob）
		{"abc", "abc", true},
		{"abd", "abc", false},
	}
	for _, c := range cases {
		got, err := Match(c.name, c.pattern)
		if err != nil || got != c.want {
			t.Errorf("Match(%q, %q) = %v, err=%v, want %v", c.name, c.pattern, got, err, c.want)
		}
	}
}
