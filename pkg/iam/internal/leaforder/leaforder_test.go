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

package leaforder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testNode struct {
	id        string
	ancestors []string
}

func nodeIDs(nodes []testNode) []string {
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.id)
	}

	return ids
}

// 叶子必须排在祖先前面：权限中心不允许删除仍被引用为祖先的资源类型。
// 同深度的元素按 ID 升序，保证结果稳定。
func TestSortLeafFirst(t *testing.T) {
	nodes := []testNode{
		{id: "biz"},
		{id: "project", ancestors: []string{"biz"}},
		{id: "environment", ancestors: []string{"project"}},
		{id: "app", ancestors: []string{"environment"}},
		{id: "audit", ancestors: []string{"biz"}},
	}

	sorted := Sort(nodes, func(n testNode) string { return n.id },
		func(n testNode) []string { return n.ancestors })

	require.Equal(t,
		[]string{"app", "environment", "audit", "project", "biz"},
		nodeIDs(sorted))
}

// 祖先不在本批元素内（跨系统祖先、未注册残留）时按深度 0 处理，不影响其余元素排序。
func TestSortTreatsUnknownAncestorAsRoot(t *testing.T) {
	nodes := []testNode{
		{id: "project", ancestors: []string{"biz"}},
		{id: "app", ancestors: []string{"external", "project"}},
	}

	sorted := Sort(nodes, func(n testNode) string { return n.id },
		func(n testNode) []string { return n.ancestors })

	require.Equal(t, []string{"app", "project"}, nodeIDs(sorted))
}

// 循环引用不能死循环，元素仍要完整返回。
func TestSortHandlesCycle(t *testing.T) {
	nodes := []testNode{
		{id: "a", ancestors: []string{"b"}},
		{id: "b", ancestors: []string{"a"}},
	}

	sorted := Sort(nodes, func(n testNode) string { return n.id },
		func(n testNode) []string { return n.ancestors })

	require.Len(t, sorted, 2)
}

// 排序返回副本，不修改调用方切片。
func TestSortDoesNotMutateInput(t *testing.T) {
	nodes := []testNode{
		{id: "biz"},
		{id: "project", ancestors: []string{"biz"}},
	}

	_ = Sort(nodes, func(n testNode) string { return n.id },
		func(n testNode) []string { return n.ancestors })

	require.Equal(t, []string{"biz", "project"}, nodeIDs(nodes))
}
