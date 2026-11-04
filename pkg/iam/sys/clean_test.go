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

package sys

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
)

// 线上可能对同一 ID 返回多条记录（重复注册残留），批量删除前必须去重。
func TestUniqueActionIDs(t *testing.T) {
	got := uniqueActionIDs([]client.ResourceAction{
		{ID: "app_view"},
		{ID: "app_create"},
		{ID: "app_view"},
	})
	require.Equal(t, []client.ActionID{"app_create", "app_view"}, got)

	require.Empty(t, uniqueActionIDs(nil))
}

func TestUniqueInstanceSelectionIDs(t *testing.T) {
	got := uniqueInstanceSelectionIDs([]client.InstanceSelection{
		{ID: "environment_selection"},
		{ID: "application_selection"},
		{ID: "environment_selection"},
	})
	require.Equal(t, []client.InstanceSelectionID{"application_selection", "environment_selection"}, got)
}

func TestUniqueResourceTypeIDs(t *testing.T) {
	got := uniqueResourceTypeIDs([]client.ResourceType{
		{ID: "app"},
		{ID: "project"},
		{ID: "app"},
	})
	require.Equal(t, []client.TypeID{"app", "project"}, got)
}

func TestCleanSummaryString(t *testing.T) {
	var empty *CleanSummary
	require.Equal(t, "未删除任何元素", empty.String())

	require.Equal(t, "操作 2 个、实例视图 1 个、资源类型 3 个",
		(&CleanSummary{Actions: 2, InstanceSelections: 1, ResourceTypes: 3}).String())
}
