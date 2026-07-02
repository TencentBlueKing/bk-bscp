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

func TestIsSliceExpression(t *testing.T) {
	cases := []struct {
		expr string
		want bool
	}{
		{"[0:10]", true},
		{"[-5:]", true},
		{"[:3]", true},
		{"*", false},
		{"[1, 2]", false},
		{"[1-9]", false},
	}
	for _, c := range cases {
		if got := IsSliceExpression(c.expr); got != c.want {
			t.Errorf("IsSliceExpression(%q) = %v, want %v", c.expr, got, c.want)
		}
	}
}

func TestExecuteSlice(t *testing.T) {
	ids := []uint32{1, 2, 3, 4, 5}
	cases := []struct {
		sliceExpr string
		want      []uint32
	}{
		{"[0:2]", []uint32{1, 2}},
		{"[-2:]", []uint32{4, 5}},
		{"[:3]", []uint32{1, 2, 3}},
		{"[2:]", []uint32{3, 4, 5}},
		{"*", []uint32{1, 2, 3, 4, 5}}, // 非切片表达式，原样返回
	}
	for _, c := range cases {
		got, err := ExecuteSlice(ids, c.sliceExpr)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("ExecuteSlice(%v, %q) = %v, err=%v, want %v", ids, c.sliceExpr, got, err, c.want)
		}
	}
}
