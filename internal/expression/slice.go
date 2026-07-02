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
	"regexp"
	"strconv"
)

// slicePattern 对齐 gsekit match.SLICE_PATTERN：`\[([-+]?\d+)?\s?:\s?([-+]?\d+)?]`。
var slicePattern = regexp.MustCompile(`\[([-+]?\d+)?\s?:\s?([-+]?\d+)?\]`)

// IsSliceExpression 判断表达式段是否为切片语法（对齐 gsekit SLICE_PATTERN.match，起始锚定）。
func IsSliceExpression(expr string) bool {
	loc := slicePattern.FindStringIndex(expr)
	return loc != nil && loc[0] == 0
}

// ExecuteSlice 对 ids 列表按切片表达式做 Python 列表切片语义（对齐 gsekit execute_slice）。
// 非切片表达式（如 `*`）或切片数不唯一时，原样返回。
func ExecuteSlice(ids []uint32, sliceExpression string) ([]uint32, error) {
	matches := slicePattern.FindAllStringSubmatch(sliceExpression, -1)
	if len(matches) != 1 {
		return ids, nil
	}

	begin, hasBegin := parseSliceIndex(matches[0][1])
	end, hasEnd := parseSliceIndex(matches[0][2])
	return pythonSlice(ids, begin, hasBegin, end, hasEnd), nil
}

// parseSliceIndex 解析切片边界，空串返回 (0,false) 表示 None。
func parseSliceIndex(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

// pythonSlice 复刻 Python 列表切片 names[begin:end] 语义（支持负索引与 None）。
func pythonSlice(ids []uint32, begin int, hasBegin bool, end int, hasEnd bool) []uint32 {
	l := len(ids)

	start := 0
	if hasBegin {
		start = clampIndex(begin, l)
	}
	stop := l
	if hasEnd {
		stop = clampIndex(end, l)
	}

	if start >= stop {
		return []uint32{}
	}
	return ids[start:stop]
}

// clampIndex 将 Python 切片索引归一到 [0, l]。
func clampIndex(idx, l int) int {
	if idx < 0 {
		idx += l
		if idx < 0 {
			return 0
		}
		return idx
	}
	if idx > l {
		return l
	}
	return idx
}
