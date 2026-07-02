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
	"sort"
	"testing"
)

// 黄金语料取自 gsekit apps/gsekit/utils/expression_utils/tests.py。
// 解析产物为"候选串集合"，顺序不影响最终匹配，故按多重集（排序后）比较。

func equalUnordered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}

func TestGetMatchType(t *testing.T) {
	cases := []struct {
		expr string
		want matchType
	}{
		{"[1, 2, 3]", matchBuildInEnum},
		{"1, 2, 3", matchWordList},
		{"1-100", matchRange},
		{"a-z", matchRange},
		{"z-a", matchWord},
		{"!ab", matchBuildInExclude},
		{"proc", matchWord},
	}
	for _, c := range cases {
		if got := getMatchType(c.expr); got != c.want {
			t.Errorf("getMatchType(%q) = %d, want %d", c.expr, got, c.want)
		}
	}
}

func TestIsSingleAlphaRange(t *testing.T) {
	cases := []struct {
		expr string
		want bool
	}{
		{"a-z", true},
		{"A-Z", true},
		{"z-a", false},
		{"1-9", false},
	}
	for _, c := range cases {
		if got := isSingleAlphaRange(c.expr); got != c.want {
			t.Errorf("isSingleAlphaRange(%q) = %v, want %v", c.expr, got, c.want)
		}
	}
}

func TestIsNumberRange(t *testing.T) {
	if !isNumberRange("1-100") {
		t.Error("isNumberRange(1-100) = false, want true")
	}
	if isNumberRange("9-1") {
		t.Error("isNumberRange(9-1) = true, want false")
	}
}

func TestIsRangeFormat(t *testing.T) {
	if !isRangeFormat("1-100") {
		t.Error("isRangeFormat(1-100) = false, want true")
	}
	if isRangeFormat("1-100-1009") {
		t.Error("isRangeFormat(1-100-1009) = true, want false")
	}
}

func TestParseWordListExpression(t *testing.T) {
	got := parseWordListExpression("module1, module2, 3,4")
	want := []string{"module1", "module2", "3", "4"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("parseWordListExpression = %v, want %v", got, want)
		}
	}
}

func TestParseRangeExpression(t *testing.T) {
	gotAlpha, err := parseRangeExpression("a-z")
	if err != nil || !equalUnordered(gotAlpha, []string{"[a-z]"}) {
		t.Errorf("parseRangeExpression(a-z) = %v, err=%v", gotAlpha, err)
	}
	gotNum, err := parseRangeExpression("1-100")
	if err != nil || !equalUnordered(gotNum, []string{"[1-9]", "[1-9][0-9]", "100"}) {
		t.Errorf("parseRangeExpression(1-100) = %v, err=%v", gotNum, err)
	}
	if _, err := parseRangeExpression("z-a"); err == nil {
		t.Error("parseRangeExpression(z-a) expected error, got nil")
	}
}

func TestParseEnumExpressionValues(t *testing.T) {
	// gsekit 的 parse_enum_expression 返回嵌套结构，经 expand_list_element 展平为候选值；
	// 此处直接校验展平后的候选值集合。
	got, err := parseEnumExpression("a-z, 1-100, 1-9")
	if err != nil {
		t.Fatalf("parseEnumExpression err=%v", err)
	}
	want := []string{"[a-z]", "[1-9]", "[1-9][0-9]", "100", "[1-9]"}
	if !equalUnordered(got, want) {
		t.Errorf("parseEnumExpression = %v, want %v", got, want)
	}
}

func TestParseExp2UnixShellStyleMain(t *testing.T) {
	got, err := parseExp2UnixShellStyleMain("module[1-3].proc[a-c].[1-100]")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	want := []string{
		"module[1-3].proc[a-c].[1-9]",
		"module[1-3].proc[a-c].[1-9][0-9]",
		"module[1-3].proc[a-c].100",
	}
	if !equalUnordered(got, want) {
		t.Errorf("parseExp2UnixShellStyleMain = %v, want %v", got, want)
	}

	gotExclude, err := parseExp2UnixShellStyleMain("[!9]")
	if err != nil || !equalUnordered(gotExclude, []string{"[!9]"}) {
		t.Errorf("parseExp2UnixShellStyleMain([!9]) = %v, err=%v", gotExclude, err)
	}

	if _, err := parseExp2UnixShellStyleMain("[...."); err == nil {
		t.Error("parseExp2UnixShellStyleMain([....) expected syntax error, got nil")
	}
}

func TestParseExp2UnixShellStyleDedup(t *testing.T) {
	got, err := ParseExp2UnixShellStyle("module[1-3, 1-3].proc[a-c].[1-100]")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	want := []string{
		"module[1-3].proc[a-c].[1-9]",
		"module[1-3].proc[a-c].[1-9][0-9]",
		"module[1-3].proc[a-c].100",
	}
	if !equalUnordered(got, want) {
		t.Errorf("ParseExp2UnixShellStyle = %v, want %v", got, want)
	}
}
