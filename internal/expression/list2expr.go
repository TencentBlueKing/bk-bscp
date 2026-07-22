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
	"strconv"
	"strings"
)

// List2Expr 把名称类字符串列表拼成一段表达式：
//   - 空列表   -> "*"（匹配任意）
//   - 单个元素 -> 原值（不加括号）
//   - 多个元素 -> "[" + 去重升序枚举 + "]"（字面量枚举）
//
// 名称段（集群/模块/服务实例/进程别名）是离散标识符，不做数字区间压缩：
// 若把形如 "01"/"02" 的名称当数字压成 "[1-2]"，前导零丢失后无法匹配回原始 CMDB 名称。
// 字面量枚举对匹配无损（比区间更精确）；进程 ID 的连续区间压缩见 IDsToExpr。
func List2Expr(values []string) string {
	uniq := dedupe(values)
	switch len(uniq) {
	case 0:
		return defaultSegment
	case 1:
		return uniq[0]
	default:
		sorted := append([]string(nil), uniq...)
		sort.Strings(sorted)
		return leftBracket + strings.Join(sorted, comma) + rightBracket
	}
}

// IDsToExpr 把 CC 进程 ID 列表拼成一段表达式：
//   - 空列表   -> "*"
//   - 单个元素 -> 原值
//   - 多个元素 -> "[" + 升序枚举 + "]"，连续数字压成 a-b（对齐 gsekit compressed_list）
//
// 进程 ID 恒为规范十进制整数（无前导零），区间压缩安全且可无损匹配回原集合。
func IDsToExpr(ids []uint32) string {
	nums := dedupeInts(ids)
	switch len(nums) {
	case 0:
		return defaultSegment
	case 1:
		return strconv.Itoa(nums[0])
	default:
		sort.Ints(nums)
		return leftBracket + strings.Join(compressRanges(nums), comma) + rightBracket
	}
}

// dedupe 去重并保持首次出现顺序。
func dedupe(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// dedupeInts 去重 uint32 列表为 []int（顺序无关，调用方负责排序）。
func dedupeInts(ids []uint32) []int {
	seen := make(map[int]struct{}, len(ids))
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		n := int(id)
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

// compressRanges 把升序整数列表压成枚举片段，连续区间记为 a-b（对齐 gsekit compressed_list）。
func compressRanges(sorted []int) []string {
	ranges := make([]string, 0, len(sorted))
	for i := 0; i < len(sorted); {
		j := i
		for j+1 < len(sorted) && sorted[j+1] == sorted[j]+1 {
			j++
		}
		if i == j {
			ranges = append(ranges, strconv.Itoa(sorted[i]))
		} else {
			ranges = append(ranges, strconv.Itoa(sorted[i])+hyphen+strconv.Itoa(sorted[j]))
		}
		i = j + 1
	}
	return ranges
}
