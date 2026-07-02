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

// 黄金语料取自 gsekit apps/gsekit/utils/expression_utils/tests.py，保证与 gsekit 逐条等价。

func TestGetUpperRange(t *testing.T) {
	cases := []struct {
		begin     int
		wantBegin int
		wantEnd   int
	}{
		{100, 100, 999},
		{1880, 1880, 1899},
	}
	for _, c := range cases {
		gotBegin, gotEnd := getUpperRange(c.begin)
		if gotBegin != c.wantBegin || gotEnd != c.wantEnd {
			t.Errorf("getUpperRange(%d) = (%d, %d), want (%d, %d)",
				c.begin, gotBegin, gotEnd, c.wantBegin, c.wantEnd)
		}
	}
}

func TestGetLowerRange(t *testing.T) {
	cases := []struct {
		end       int
		wantBegin int
		wantEnd   int
	}{
		{15, 10, 15},
		{199, 0, 199},
	}
	for _, c := range cases {
		gotBegin, gotEnd := getLowerRange(c.end)
		if gotBegin != c.wantBegin || gotEnd != c.wantEnd {
			t.Errorf("getLowerRange(%d) = (%d, %d), want (%d, %d)",
				c.end, gotBegin, gotEnd, c.wantBegin, c.wantEnd)
		}
	}
}

func TestSplitRangeLeft(t *testing.T) {
	cases := []struct {
		begin, end int
		want       [][2]int
	}{
		{1, 100, [][2]int{{1, 9}, {10, 99}}},
		{1, 105, [][2]int{{1, 9}, {10, 99}, {100, 999}}},
	}
	for _, c := range cases {
		got := splitRangeLeft(c.begin, c.end)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitRangeLeft(%d, %d) = %v, want %v", c.begin, c.end, got, c.want)
		}
	}
}

func TestSplitRangeRight(t *testing.T) {
	cases := []struct {
		begin, end int
		want       [][2]int
	}{
		{1, 100, [][2]int{{0, 99}, {100, 100}}},
		{1, 105, [][2]int{{0, 99}, {100, 105}}},
	}
	for _, c := range cases {
		got := splitRangeRight(c.begin, c.end)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitRangeRight(%d, %d) = %v, want %v", c.begin, c.end, got, c.want)
		}
	}
}

func TestRange2Re(t *testing.T) {
	cases := []struct {
		begin, end int
		want       []string
	}{
		{1, 100, []string{"[1-9]", "[1-9][0-9]", "100"}},
		{1, 105, []string{"[1-9]", "[1-9][0-9]", "10[0-5]"}},
		{50, 50, []string{"50"}},
	}
	for _, c := range cases {
		got := Range2Re(c.begin, c.end)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Range2Re(%d, %d) = %v, want %v", c.begin, c.end, got, c.want)
		}
	}
}
