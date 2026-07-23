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

import "testing"

func TestList2Expr(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		want   string
	}{
		{"nil", nil, "*"},
		{"empty", []string{}, "*"},
		{"single word", []string{"管控平台"}, "管控平台"},
		{"single after dedup", []string{"a", "a"}, "a"},
		{"multi word sorted", []string{"b", "a"}, "[a,b]"},
		// 名称段是字面量枚举，不做数字区间压缩（避免前导零/数字名被误压后无法匹配回原名）。
		{"numeric names not compressed", []string{"6", "7", "8"}, "[6,7,8]"},
		{"leading zeros preserved", []string{"02", "01"}, "[01,02]"},
		{"mixed literal sorted", []string{"proc", "2", "1"}, "[1,2,proc]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := List2Expr(c.values); got != c.want {
				t.Fatalf("List2Expr(%v) = %q, want %q", c.values, got, c.want)
			}
		})
	}
}

func TestIDsToExpr(t *testing.T) {
	cases := []struct {
		name string
		ids  []uint32
		want string
	}{
		{"nil", nil, "*"},
		{"single", []uint32{6}, "6"},
		{"consecutive", []uint32{6, 7, 8}, "[6-8]"},
		{"gap", []uint32{6, 8, 9}, "[6,8-9]"},
		{"unordered dedup", []uint32{9, 8, 6, 6}, "[6,8-9]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IDsToExpr(c.ids); got != c.want {
				t.Fatalf("IDsToExpr(%v) = %q, want %q", c.ids, got, c.want)
			}
		})
	}
}
