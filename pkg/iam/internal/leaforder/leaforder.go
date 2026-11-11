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

// Package leaforder 按祖先链深度给资源类型排序，供权限中心模型清理（V3 与 V4 共用）。
package leaforder

import (
	"sort"
	"strings"
)

// Sort 返回按祖先链深度从深到浅排序的副本，同深度按 ID 升序。
//
// 权限中心不允许删除仍被其它资源类型当作祖先引用的类型，按 ID 字母序删除会先删根类型
// 并失败，叶子随后也删不掉；资源类型必须先叶子后祖先。
//
// idOf 返回元素自身的 ID，ancestorsOf 返回它的祖先 ID 列表。
// 祖先不在本批元素内（跨系统祖先、未注册残留）时按深度 0 处理；
// 存在循环引用时同样按 0 处理，避免递归无法结束。
func Sort[T any](items []T, idOf func(T) string, ancestorsOf func(T) []string) []T {
	byID := make(map[string]T, len(items))
	for _, item := range items {
		byID[idOf(item)] = item
	}

	depth := make(map[string]int, len(items))
	visiting := make(map[string]bool, len(items))

	var depthOf func(id string) int
	depthOf = func(id string) int {
		if d, ok := depth[id]; ok {
			return d
		}
		if visiting[id] {
			return 0
		}

		item, ok := byID[id]
		if !ok {
			return 0
		}

		visiting[id] = true
		deepest := 0
		for _, ancestor := range ancestorsOf(item) {
			if d := depthOf(ancestor) + 1; d > deepest {
				deepest = d
			}
		}
		delete(visiting, id)
		depth[id] = deepest

		return deepest
	}

	sorted := append([]T(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		di, dj := depthOf(idOf(sorted[i])), depthOf(idOf(sorted[j]))
		if di != dj {
			return di > dj
		}

		return strings.Compare(idOf(sorted[i]), idOf(sorted[j])) < 0
	})

	return sorted
}
