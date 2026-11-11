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
	"strings"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/internal/leaforder"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// CleanSummary 描述本次清空删掉的模型元素数量。
type CleanSummary struct {
	Actions            int
	InstanceSelections int
	ResourceTypes      int
	// Failures 记录删除失败的模型元素，非空表示线上仍可能有残留
	Failures []string
}

// String 输出可读的清理结果。
func (s *CleanSummary) String() string {
	if s == nil {
		return "未删除任何元素"
	}

	result := fmt.Sprintf("操作 %d 个、实例视图 %d 个、资源类型 %d 个",
		s.Actions, s.InstanceSelections, s.ResourceTypes)
	if len(s.Failures) > 0 {
		result += fmt.Sprintf("，其中 %d 个删除失败", len(s.Failures))
	}

	return result
}

// DeleteAll 清空当前系统在权限中心的模型：先删所有操作（含策略），再删实例视图，最后删资源类型。
//
// 操作组、创建者动作与通用操作只有整组替换接口，删完操作后它们的引用会悬空，
// 由随后的 Register（migrate init-iam）用本地定义整组覆盖，因此这里不单独处理。
// 删除顺序遵循依赖关系：策略 → 操作 → 实例视图 → 资源类型。
//
// 逐个元素删除而不是整批提交：权限中心的批量删除是整体校验，只要有一个元素
// 删不掉（仍被实例视图/其他资源类型引用、策略未清干净等），整批都会失败，
// 表现为"命令报错退出、什么都没删"或"删了一部分就停"。逐个删能最大化清空范围，
// 失败的元素记录后继续处理其余元素，最后回读线上模型，仍有残留则返回错误，
// 避免命令打印"删除完成"却留下元素。
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
	resourceTypes := sortResourceTypesForDeletion(info.Data.ResourceTypes)

	summary := &CleanSummary{
		Actions:            len(actions),
		InstanceSelections: len(selections),
		ResourceTypes:      len(resourceTypes),
	}
	failures := make([]string, 0)

	// 策略必须先于操作删除，否则操作删不掉；个别操作删策略失败不阻断整体清空。
	for _, id := range actions {
		if err := s.client.DeleteActionPolicies(ctx, id); err != nil {
			logs.Warnf("delete action policies failed, action: %s, err: %v", id, err)
		}

		if err := s.client.DeleteActions(ctx, []client.ActionID{id}); err != nil {
			logs.Errorf("delete action failed, action: %s, err: %v", id, err)
			failures = append(failures, fmt.Sprintf("操作 %s: %v", id, err))
		}
	}

	for _, id := range selections {
		if err := s.client.DeleteInstanceSelections(ctx, []client.InstanceSelectionID{id}); err != nil {
			logs.Errorf("delete instance selection failed, selection: %s, err: %v", id, err)
			failures = append(failures, fmt.Sprintf("实例视图 %s: %v", id, err))
		}
	}

	// 资源类型按祖先链从深到浅删除：权限中心不允许删除仍被子类型引用的类型。
	for _, id := range resourceTypes {
		if err := s.client.DeleteResourcesTypes(ctx, []client.TypeID{id}); err != nil {
			logs.Errorf("delete resource type failed, type: %s, err: %v", id, err)
			failures = append(failures, fmt.Sprintf("资源类型 %s: %v", id, err))
		}
	}

	summary.Failures = failures

	leftovers, err := s.leftoverModelElements(ctx)
	if err != nil {
		return summary, fmt.Errorf("回读线上模型校验清理结果失败: %w", err)
	}

	if len(leftovers) > 0 {
		return summary, fmt.Errorf("线上仍残留 %d 个模型元素未删除: %s",
			len(leftovers), strings.Join(leftovers, ", "))
	}

	if len(failures) > 0 {
		return summary, fmt.Errorf("%d 个元素删除失败: %s", len(failures), strings.Join(failures, "; "))
	}

	return summary, nil
}

// leftoverModelElements 回读线上模型，返回仍未清理的元素描述。
// 权限中心可能对删除请求返回成功但实际保留了元素，只有回读才能确认真的清空。
func (s *Sys) leftoverModelElements(ctx context.Context) ([]string, error) {
	info, err := s.client.GetSystemInfo(ctx, []client.SystemQueryField{
		client.FieldActions, client.FieldInstanceSelections, client.FieldResourceTypes,
	})
	if err != nil {
		if err == client.ErrNotFound {
			return nil, nil
		}

		return nil, err
	}

	leftovers := make([]string, 0)
	for _, id := range uniqueActionIDs(info.Data.Actions) {
		leftovers = append(leftovers, "操作 "+string(id))
	}
	for _, id := range uniqueInstanceSelectionIDs(info.Data.InstanceSelections) {
		leftovers = append(leftovers, "实例视图 "+string(id))
	}
	for _, id := range uniqueResourceTypeIDs(info.Data.ResourceTypes) {
		leftovers = append(leftovers, "资源类型 "+string(id))
	}

	return leftovers, nil
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

// sortResourceTypesForDeletion 按祖先链深度从深到浅排序，返回待删除的资源类型 ID。
//
// 权限中心不允许删除仍被其他资源类型当作父级引用的类型，因此必须先删叶子再删祖先；
// 按 ID 字母序删除会出现「先删根类型失败、后续全部跳过」的情况，导致资源类型残留。
// 跨系统的父级不参与排序（不在本系统模型里，删不掉但也不阻塞）。
func sortResourceTypesForDeletion(resourceTypes []client.ResourceType) []client.TypeID {
	sorted := leaforder.Sort(resourceTypes,
		func(resourceType client.ResourceType) string { return string(resourceType.ID) },
		func(resourceType client.ResourceType) []string {
			parents := make([]string, 0, len(resourceType.Parents))
			for _, parent := range resourceType.Parents {
				if parent.SystemID != "" && parent.SystemID != SystemIDBSCP {
					continue
				}
				parents = append(parents, string(parent.ResourceID))
			}

			return parents
		})

	// 权限中心可能对同一 ID 返回多条记录（重复注册残留），删除请求带了重复项会整体失败。
	result := make([]client.TypeID, 0, len(sorted))
	seen := make(map[client.TypeID]struct{}, len(sorted))
	for _, resourceType := range sorted {
		if _, ok := seen[resourceType.ID]; ok {
			continue
		}
		seen[resourceType.ID] = struct{}{}
		result = append(result, resourceType.ID)
	}

	return result
}
