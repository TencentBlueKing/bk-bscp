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

// Package expression 移植 gsekit apps/gsekit/utils/expression_utils，提供与 gsekit 等价的
// 进程表达式解析与匹配能力，保证同一表达式在 bscp 与 gsekit 命中相同的进程集合。
package expression

import (
	"strconv"
	"strings"
)

// getUpperRange 以 begin 为下界，获取可正则化的最大范围（对齐 gsekit range2re.get_upper_range）。
// 从个位起将连续的 0 置为 9，遇到首个非 0 位也置 9 后停止。
func getUpperRange(begin int) (int, int) {
	digits := []byte(strconv.Itoa(begin))
	for i := len(digits) - 1; i >= 0; i-- {
		orig := digits[i]
		digits[i] = '9'
		if orig != '0' {
			break
		}
	}
	end, _ := strconv.Atoi(string(digits))
	return begin, end
}

// getLowerRange 以 end 为上界，获取可正则化的最大范围（对齐 gsekit range2re.get_lower_range）。
// 从个位起将连续的 9 置为 0，遇到首个非 9 位也置 0 后停止。
func getLowerRange(end int) (int, int) {
	digits := []byte(strconv.Itoa(end))
	for i := len(digits) - 1; i >= 0; i-- {
		orig := digits[i]
		digits[i] = '0'
		if orig != '9' {
			break
		}
	}
	begin, _ := strconv.Atoi(string(digits))
	return begin, end
}

// splitRangeLeft 从 begin->end 切割成若干可正则化范围，最后一个范围的上界 >= end。
func splitRangeLeft(begin, end int) [][2]int {
	splitList := [][2]int{}
	for begin < end {
		b, e := getUpperRange(begin)
		splitList = append(splitList, [2]int{b, e})
		begin = e + 1
	}
	return splitList
}

// splitRangeRight 从 end->begin 切割成若干可正则化范围，最后一个范围的下界 <= begin。
func splitRangeRight(begin, end int) [][2]int {
	splitList := [][2]int{}
	for begin < end {
		b, e := getLowerRange(end)
		splitList = append(splitList, [2]int{b, e})
		end = b - 1
	}
	// 反转，保持从小到大顺序
	for i, j := 0, len(splitList)-1; i < j; i, j = i+1, j-1 {
		splitList[i], splitList[j] = splitList[j], splitList[i]
	}
	return splitList
}

// Range2Re 获取可匹配 [begin, end] 内所有整数的正则片段列表（对齐 gsekit range2re.range2re）。
func Range2Re(begin, end int) []string {
	if begin == end {
		return []string{strconv.Itoa(begin)}
	}

	splitByLeft := splitRangeLeft(begin, end)
	midLeft := splitByLeft[len(splitByLeft)-1]
	splitByLeft = splitByLeft[:len(splitByLeft)-1]

	// 从 begin->end 切割的最后一个范围 >= end，需对剩余范围做一次准确切割
	splitByRight := splitRangeRight(midLeft[0], end)
	midRight := splitByRight[0]
	splitByRight = splitByRight[1:]

	splitRanges := [][2]int{}
	splitRanges = append(splitRanges, splitByLeft...)

	// 有交集时取 left.begin - right.end，否则两段都保留
	if midRight[0] < midLeft[1] && midLeft[0] < midRight[1] {
		splitRanges = append(splitRanges, [2]int{midLeft[0], midRight[1]})
	} else {
		splitRanges = append(splitRanges, midLeft, midRight)
	}
	splitRanges = append(splitRanges, splitByRight...)

	reList := make([]string, 0, len(splitRanges))
	for _, r := range splitRanges {
		reList = append(reList, rangePartToRe(r[0], r[1]))
	}
	return reList
}

// rangePartToRe 将一个可正则化范围逐位转为正则片段（begin/end 位数相同）。
func rangePartToRe(begin, end int) string {
	beginStr := strconv.Itoa(begin)
	endStr := strconv.Itoa(end)
	var sb strings.Builder
	for i := 0; i < len(beginStr); i++ {
		if beginStr[i] == endStr[i] {
			sb.WriteByte(beginStr[i])
		} else {
			sb.WriteByte('[')
			sb.WriteByte(beginStr[i])
			sb.WriteByte('-')
			sb.WriteByte(endStr[i])
			sb.WriteByte(']')
		}
	}
	return sb.String()
}
