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

package tools

import "strings"

// TrieNode Trie 数据结构
type TrieNode struct {
	children map[string]*TrieNode
	isEnd    bool
}

func newTrieNode() *TrieNode {
	return &TrieNode{children: make(map[string]*TrieNode)}
}

func insertPath(root *TrieNode, path string) bool {
	current := root
	conflict := false
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if _, exists := current.children[part]; !exists {
			current.children[part] = newTrieNode()
		}
		current = current.children[part]

		// 如果当前节点已是某条路径的末尾，表示路径冲突
		if current.isEnd {
			conflict = true
		}
	}

	// 标记当前节点为路径的末尾
	current.isEnd = true

	// 如果当前路径有子路径，也表示冲突
	if len(current.children) > 0 {
		conflict = true
	}

	return conflict
}

// CheckExistingPathConflict 检测文件路径冲突
// 树结构（Trie），可以快速检测路径前缀冲突
func CheckExistingPathConflict(existing []string) (uint32, map[string]bool) {
	root := newTrieNode()
	conflictPaths := make(map[string]bool)
	var conflictNums uint32

	for _, path := range existing {
		if insertPath(root, path) {
			conflictPaths[path] = true
			conflictNums++
		}
	}

	return conflictNums, conflictPaths
}
