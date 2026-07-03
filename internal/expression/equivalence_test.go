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

// 等价性回归：显式覆盖梳理文档 AC-002 的关键边界，保证与 gsekit 匹配语义一致。

// [ab] 字面量陷阱：无逗号/范围/`!` 的 [ab] 按字面量 "ab" 处理，不是字符集。
func TestEquivLiteralBracketTrap(t *testing.T) {
	got, err := Match("ab", "[ab]")
	if err != nil || !got {
		t.Errorf("Match(ab, [ab]) = %v, err=%v, want true（字面量）", got, err)
	}
	got, err = Match("a", "[ab]")
	if err != nil || got {
		t.Errorf("Match(a, [ab]) = %v, err=%v, want false（非字符集）", got, err)
	}
}

// 单字符范围 [a-b] 才是字符集，匹配单个字符。
func TestEquivSingleAlphaRangeIsCharClass(t *testing.T) {
	for _, name := range []string{"a", "b"} {
		got, err := Match(name, "[a-b]")
		if err != nil || !got {
			t.Errorf("Match(%q, [a-b]) = %v, err=%v, want true", name, got, err)
		}
	}
	got, err := Match("c", "[a-b]")
	if err != nil || got {
		t.Errorf("Match(c, [a-b]) = %v, err=%v, want false", got, err)
	}
}

// 排除 [!seq] 匹配非 seq 的单字符。
func TestEquivExclude(t *testing.T) {
	got, err := Match("c", "[!ab]")
	if err != nil || !got {
		t.Errorf("Match(c, [!ab]) = %v, err=%v, want true", got, err)
	}
	got, err = Match("a", "[!ab]")
	if err != nil || got {
		t.Errorf("Match(a, [!ab]) = %v, err=%v, want false", got, err)
	}
}

// 前缀 + 枚举组合 4[6, 8, 9] 展开为 46/48/49。
func TestEquivPrefixEnumCombination(t *testing.T) {
	for _, name := range []string{"46", "48", "49"} {
		got, err := Match(name, "4[6, 8, 9]")
		if err != nil || !got {
			t.Errorf("Match(%q, 4[6, 8, 9]) = %v, err=%v, want true", name, got, err)
		}
	}
	got, err := Match("47", "4[6, 8, 9]")
	if err != nil || got {
		t.Errorf("Match(47, 4[6, 8, 9]) = %v, err=%v, want false", got, err)
	}
}

// 空集语义：表达式解析后命中为空，返回空列表（非全选）。
func TestEquivEmptyResult(t *testing.T) {
	candidates := []Candidate{
		{Expression: JoinProcessExpression("setA", "m", "svc", "p", "1"), CcProcessID: 1},
	}
	s := Scope{Environment: "3", SetName: "notExist"}
	got, err := ScopeToCcIDs(s, candidates)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(got) != 0 {
		t.Errorf("ScopeToCcIDs no-match = %v, want empty", got)
	}
}

// 梳理文档规范示例：正式环境、集群名为管控平台或 PaaS平台、CC 进程 ID 为 46/48/49。
func TestEquivDocCanonicalExample(t *testing.T) {
	candidates := []Candidate{
		{Expression: JoinProcessExpression("管控平台", "m", "svc", "p", "46"), CcProcessID: 46},
		{Expression: JoinProcessExpression("PaaS平台", "m", "svc", "p", "48"), CcProcessID: 48},
		{Expression: JoinProcessExpression("管控平台", "m", "svc", "p", "49"), CcProcessID: 49},
		{Expression: JoinProcessExpression("其它", "m", "svc", "p", "47"), CcProcessID: 47},
	}
	s := Scope{
		Environment: "3",
		SetName:     "[管控平台, PaaS平台]",
		ProcessID:   "4[6, 8, 9]",
	}
	got, err := ScopeToCcIDs(s, candidates)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	want := []uint32{46, 48, 49}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("canonical example = %v, want %v", got, want)
	}
}
