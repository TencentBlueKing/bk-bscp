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
	"context"
	"fmt"
	"sort"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// CleanSummary 描述本次清空删掉的模型元素数量。
type CleanSummary struct {
	Actions            int
	InstanceSelections int
	ResourceTypes      int
}

// String 输出可读的清理结果。
func (s *CleanSummary) String() string {
	if s == nil {
		return "未删除任何元素"
	}

	return fmt.Sprintf("操作 %d 个、实例视图 %d 个、资源类型 %d 个",
		s.Actions, s.InstanceSelections, s.ResourceTypes)
}

// DeleteAll 清空当前系统在权限中心的模型：先删所有操作（含策略），再删实例视图，最后删资源类型。
//
// 操作组、创建者动作与通用操作只有整组替换接口，删完操作后它们的引用会悬空，
// 由随后的 Register（migrate init-iam）用本地定义整组覆盖，因此这里不单独处理。
// 删除顺序遵循依赖关系：策略 → 操作 → 实例视图 → 资源类型，前一步失败直接返回。
func (s *Sys) DeleteAll(ctx context.Context) (*CleanSummary, error) {
	info, err := s.client.GetSystemInfo(ctx, []client.SystemQueryField{
		client.FieldActions, client.FieldInstanceSelections, client.FieldResourceTypes,
	})
	if err != nil {
		if err == client.ErrNotFound {
			return nil, fmt.Errorf("权限中心未注册 %s 系统，无需清理", SystemIDBSCP)
		}

		return nil, err
	}

	actions := uniqueActionIDs(info.Data.Actions)
	selections := uniqueInstanceSelectionIDs(info.Data.InstanceSelections)
	resourceTypes := uniqueResourceTypeIDs(info.Data.ResourceTypes)

	summary := &CleanSummary{
		Actions:            len(actions),
		InstanceSelections: len(selections),
		ResourceTypes:      len(resourceTypes),
	}

	// 策略必须先于操作删除，否则操作删不掉；个别操作删策略失败不阻断整体清空。
	for _, id := range actions {
		if err := s.client.DeleteActionPolicies(ctx, id); err != nil {
			logs.Warnf("delete action policies failed, action: %s, err: %v", id, err)
		}
	}

	if len(actions) > 0 {
		if err := s.client.DeleteActions(ctx, actions); err != nil {
			return summary, fmt.Errorf("delete actions failed: %w", err)
		}
	}

	if len(selections) > 0 {
		if err := s.client.DeleteInstanceSelections(ctx, selections); err != nil {
			return summary, fmt.Errorf("delete instance selections failed: %w", err)
		}
	}

	if len(resourceTypes) > 0 {
		if err := s.client.DeleteResourcesTypes(ctx, resourceTypes); err != nil {
			return summary, fmt.Errorf("delete resource types failed: %w", err)
		}
	}

	return summary, nil
}

// uniqueActionIDs 汇总线上操作 ID，按 ID 去重并排序。
// 权限中心可能对同一 ID 返回多条记录（重复注册残留），重复的 ID 会让批量删除请求失败。
func uniqueActionIDs(actions []client.ResourceAction) []client.ActionID {
	seen := make(map[client.ActionID]struct{}, len(actions))
	result := make([]client.ActionID, 0, len(actions))
	for _, action := range actions {
		if _, ok := seen[action.ID]; ok {
			continue
		}
		seen[action.ID] = struct{}{}
		result = append(result, action.ID)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })

	return result
}

// uniqueInstanceSelectionIDs 汇总线上实例视图 ID，按 ID 去重并排序。
func uniqueInstanceSelectionIDs(selections []client.InstanceSelection) []client.InstanceSelectionID {
	seen := make(map[client.InstanceSelectionID]struct{}, len(selections))
	result := make([]client.InstanceSelectionID, 0, len(selections))
	for _, selection := range selections {
		if _, ok := seen[selection.ID]; ok {
			continue
		}
		seen[selection.ID] = struct{}{}
		result = append(result, selection.ID)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })

	return result
}

// uniqueResourceTypeIDs 汇总线上资源类型 ID，按 ID 去重并排序。
func uniqueResourceTypeIDs(resourceTypes []client.ResourceType) []client.TypeID {
	seen := make(map[client.TypeID]struct{}, len(resourceTypes))
	result := make([]client.TypeID, 0, len(resourceTypes))
	for _, resourceType := range resourceTypes {
		if _, ok := seen[resourceType.ID]; ok {
			continue
		}
		seen[resourceType.ID] = struct{}{}
		result = append(result, resourceType.ID)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })

	return result
}
