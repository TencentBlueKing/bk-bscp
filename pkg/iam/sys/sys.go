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

// Package sys NOTES
package sys

import (
	"context"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// Sys iam system related operate.
type Sys struct {
	client *client.Client
}

// NewSys create sys to iam sys related operate.
func NewSys(client *client.Client) (*Sys, error) {
	if client == nil {
		return nil, errf.New(errf.InvalidParameter, "client is nil")
	}

	sys := &Sys{
		client: client,
	}
	return sys, nil
}

// GetSystemToken get system token from iam, used to validate if request is from iam.
func (s *Sys) GetSystemToken(ctx context.Context) (string, error) {
	return s.client.GetSystemToken(ctx)
}

// Register register auth model to iam.
//
// 资源之间存在依赖：操作引用资源类型与实例视图，实例视图引用资源类型。因此写入顺序固定为
//
//	删除：Action → InstanceSelection → ResourceType
//	新增：ResourceType → InstanceSelection → Action（父资源必须先存在）
//	更新：InstanceSelection、Action 必须放在新增资源类型之后
//
// 分类阶段不做任何写入：把更新与分类混在一起会导致「操作引用的资源类型尚未创建」而失败，
// 例如新增 project 资源类型、同时把既有 project_view 的关联资源类型改为 project 时，
// 若先更新操作就会报 related resource type[project] not exists。
// ActionGroup / 创建者动作 / 通用操作都依赖 Action，始终放在最后。
// 先删除再新增，是因为实例视图的名称在系统中唯一，同名时创建会失败。
func (s *Sys) Register(ctx context.Context, host string) error {
	system, err := s.registerSystem(ctx, host)
	if err != nil {
		return err
	}

	removedResTypeMap, newResTypes := classResType(system.ResourceTypes)
	removedInstSelectionMap, newInstSelections := classInstSelection(system.InstanceSelections)
	removedResActionMap, newResActions := classAction(system.Actions)

	if err = s.deleteRemovedModel(ctx, removedResActionMap, removedInstSelectionMap, removedResTypeMap); err != nil {
		return err
	}

	if len(newResTypes) > 0 {
		if err := s.client.RegisterResourcesTypes(ctx, newResTypes); err != nil {
			logs.Errorf("register resource types failed, types: %v, err: %v", newResTypes, err)
			return err
		}
	}

	if len(newInstSelections) > 0 {
		if err := s.client.RegisterInstanceSelections(ctx, newInstSelections); err != nil {
			logs.Errorf("register instance selections failed, selections: %v, err: %v", newInstSelections, err)
			return err
		}
	}

	if err := s.updateExistingModel(ctx, system); err != nil {
		return err
	}

	if len(newResActions) > 0 {
		if err := s.client.RegisterActions(ctx, newResActions); err != nil {
			logs.Errorf("register resource actions failed, actions: %v, err: %v", newResActions, err)
			return err
		}
	}

	if err := s.registerActionGroup(ctx, system.ActionGroups); err != nil {
		return err
	}

	if err := s.registerResCreatorAction(ctx, system.ResourceCreatorActions); err != nil {
		return err
	}

	if err := s.registerCommonAction(ctx, system.CommonActions); err != nil {
		return err
	}

	return nil
}

// updateExistingModel 更新线上已存在、但本地定义有变化的资源类型、实例视图与操作。
// 必须在新增资源类型之后调用：它们的父级或引用可能正是本次新增的资源类型。
func (s *Sys) updateExistingModel(ctx context.Context, system *client.RegisteredSystemInfo) error {
	existingTypes := make(map[client.TypeID]struct{}, len(system.ResourceTypes))
	for _, resourceType := range system.ResourceTypes {
		existingTypes[resourceType.ID] = struct{}{}
	}

	for _, resourceType := range GenerateStaticResourceTypes() {
		if _, ok := existingTypes[resourceType.ID]; !ok {
			continue
		}

		if err := s.client.UpdateResourcesType(ctx, resourceType); err != nil {
			logs.Errorf("update resource type failed, id: %s, type: %v, err: %v",
				resourceType.ID, resourceType, err)
			return err
		}
	}

	existingSelections := make(map[client.InstanceSelectionID]struct{}, len(system.InstanceSelections))
	for _, selection := range system.InstanceSelections {
		existingSelections[selection.ID] = struct{}{}
	}

	for _, selection := range GenerateStaticInstanceSelections() {
		if _, ok := existingSelections[selection.ID]; !ok {
			continue
		}

		if err := s.client.UpdateInstanceSelection(ctx, selection); err != nil {
			logs.Errorf("update instance selection failed, id: %s, selection: %v, err: %v",
				selection.ID, selection, err)
			return err
		}
	}

	existingActions := make(map[client.ActionID]struct{}, len(system.Actions))
	for _, action := range system.Actions {
		existingActions[action.ID] = struct{}{}
	}

	for _, action := range GenerateStaticActions() {
		if _, ok := existingActions[action.ID]; !ok {
			continue
		}

		if err := s.client.UpdateAction(ctx, action); err != nil {
			logs.Errorf("update resource action failed, id: %s, action: %v, err: %v",
				action.ID, action, err)
			return err
		}
	}

	return nil
}

// registerCommonAction register or update common actions
func (s *Sys) registerCommonAction(ctx context.Context, actions []client.CommonAction) error {
	commonActions := GenerateCommonActions()
	if len(actions) == 0 {
		if len(commonActions) != 0 {
			if err := s.client.RegisterCommonActions(ctx, commonActions); err != nil {
				logs.Errorf("register common actions failed, common actions: %v, err: %v", commonActions, err)
				return err
			}
		}
	} else {
		if err := s.client.UpdateCommonActions(ctx, commonActions); err != nil {
			logs.Errorf("update common actions failed, common actions: %v, err: %v", commonActions, err)
			return err
		}
	}

	return nil
}

// registerResCreatorAction register or update resource creator actions.
func (s *Sys) registerResCreatorAction(ctx context.Context, actions client.ResourceCreatorActions) error {
	resourceCreatorActions := GenerateResourceCreatorActions()
	if len(actions.Config) == 0 {
		if len(resourceCreatorActions.Config) != 0 {
			if err := s.client.RegisterResourceCreatorActions(ctx, resourceCreatorActions); err != nil {
				logs.Errorf("register resource creator actions failed, resource creator actions: %v, err: %v",
					resourceCreatorActions, err)
				return err
			}
		}
	} else {
		if err := s.client.UpdateResourceCreatorActions(ctx, resourceCreatorActions); err != nil {
			logs.Errorf("update resource creator actions failed, resource creator actions: %v, err: %v",
				resourceCreatorActions, err)
			return err
		}
	}

	return nil
}

// registerActionGroup register or update action group.
func (s *Sys) registerActionGroup(ctx context.Context, groups []client.ActionGroup) error {
	actionGroups := GenerateStaticActionGroups()
	if len(actionGroups) != 0 {
		if len(groups) == 0 {
			if err := s.client.RegisterActionGroups(ctx, actionGroups); err != nil {
				logs.Errorf("register action groups failed, action groups: %v, err: %v", actionGroups, err)
				return err
			}

		} else {
			if err := s.client.UpdateActionGroups(ctx, actionGroups); err != nil {
				logs.Errorf("update action groups failed, action groups: %v, err: %v", actionGroups, err)
				return err
			}
		}
	}

	return nil
}

// deleteRemovedModel delete removed auth model.
func (s *Sys) deleteRemovedModel(ctx context.Context, resActions map[client.ActionID]struct{},
	selections map[client.InstanceSelectionID]struct{}, resTypes map[client.TypeID]struct{}) error {

	if len(resActions) > 0 {
		removedResourceActionIDs := make([]client.ActionID, len(resActions))
		idx := 0
		// before deleting action, the dependent action policies must be deleted
		for resourceActionID := range resActions {
			if err := s.client.DeleteActionPolicies(ctx, resourceActionID); err != nil {
				logs.Errorf("delete action policies failed, id: %s, err: %v", resourceActionID, err)
				return err
			}

			removedResourceActionIDs[idx] = resourceActionID
			idx++
		}
		if err := s.client.DeleteActions(ctx, removedResourceActionIDs); err != nil {
			logs.Errorf("delete resource actions failed, actions: %v, err: %v", removedResourceActionIDs, err)
			return err
		}
	}

	if len(selections) > 0 {
		removedInstanceSelectionIDs := make([]client.InstanceSelectionID, len(selections))
		idx := 0
		for resourceActionID := range selections {
			removedInstanceSelectionIDs[idx] = resourceActionID
			idx++
		}
		if err := s.client.DeleteInstanceSelections(ctx, removedInstanceSelectionIDs); err != nil {
			logs.Errorf("delete instance selections failed, selections: %v, err: %v",
				removedInstanceSelectionIDs, err)
			return err
		}
	}

	if len(resTypes) > 0 {
		removedResourceTypeIDs := make([]client.TypeID, len(resTypes))
		idx := 0
		for resourceType := range resTypes {
			removedResourceTypeIDs[idx] = resourceType
			idx++
		}
		if err := s.client.DeleteResourcesTypes(ctx, removedResourceTypeIDs); err != nil {
			logs.Errorf("delete resource types failed, types: %v, err: %v", removedResourceTypeIDs, err)
			return err
		}
	}

	return nil
}

// classAction 把本地操作分为「线上已存在（后续更新）」与「需要新增」两类，只读不写。
func classAction(actions []client.ResourceAction) (
	map[client.ActionID]struct{}, []client.ResourceAction) {

	old := make(map[client.ActionID]bool)
	removed := make(map[client.ActionID]struct{})
	news := make([]client.ResourceAction, 0)
	for _, resourceAction := range actions {
		old[resourceAction.ID] = true
		removed[resourceAction.ID] = struct{}{}
	}

	for _, resourceAction := range GenerateStaticActions() {
		// registered resource action exist in current resource actions, should not be removed
		delete(removed, resourceAction.ID)
		if !old[resourceAction.ID] {
			news = append(news, resourceAction)
		}
	}

	return removed, news
}

// classInstSelection 把本地实例视图分为「线上已存在（后续更新）」与「需要新增」两类，只读不写。
func classInstSelection(selections []client.InstanceSelection) (
	map[client.InstanceSelectionID]struct{}, []client.InstanceSelection) {

	old := make(map[client.InstanceSelectionID]bool)
	removed := make(map[client.InstanceSelectionID]struct{})
	news := make([]client.InstanceSelection, 0)
	for _, instanceSelection := range selections {
		old[instanceSelection.ID] = true
		removed[instanceSelection.ID] = struct{}{}
	}

	for _, resourceType := range GenerateStaticInstanceSelections() {
		// registered instance selection removed in current instance selections, should not be removed
		delete(removed, resourceType.ID)
		if !old[resourceType.ID] {
			news = append(news, resourceType)
		}
	}

	return removed, news
}

// classResType 把本地资源类型分为「线上已存在（无需处理）」与「需要新增」两类，只读不写。
func classResType(resTypes []client.ResourceType) (
	map[client.TypeID]struct{}, []client.ResourceType) {

	old := make(map[client.TypeID]bool)
	removed := make(map[client.TypeID]struct{})
	news := make([]client.ResourceType, 0)
	for _, resourceType := range resTypes {
		old[resourceType.ID] = true
		removed[resourceType.ID] = struct{}{}
	}

	for _, resourceType := range GenerateStaticResourceTypes() {
		// registered resource type exist in current resource types, should not be removed
		delete(removed, resourceType.ID)
		if !old[resourceType.ID] {
			news = append(news, resourceType)
		}
	}

	return removed, news
}

// registerSystem register or update system to iam.
func (s *Sys) registerSystem(ctx context.Context, host string) (*client.RegisteredSystemInfo, error) {
	resp, err := s.client.GetSystemInfo(ctx, []client.SystemQueryField{})
	if err != nil && err != client.ErrNotFound {
		logs.Errorf("get system info failed, err: %v", err)
		return nil, err
	}

	sys := client.System{
		ID:          SystemIDBSCP,
		Name:        SystemNameBSCP,
		EnglishName: SystemNameBSCPEn,
		Clients:     SystemIDBSCP,
		ProviderConfig: &client.SysConfig{
			Host: host,
			Auth: "basic",
		},
	}

	// if iam bscp system has not been registered, register system
	if err == client.ErrNotFound {

		if err = s.client.RegisterSystem(ctx, sys); err != nil {
			logs.Errorf("register system failed, system: %v, err: %v", sys, err)
			return nil, err
		}

		if logs.V(5) {
			logs.Infof("register new system succeed, system: %v", sys)
		}

	} else {
		// else update bscp system in iam
		if err = s.client.UpdateSystem(ctx, sys); err != nil {
			logs.Errorf("update system host config failed, host: %s, err: %v", host, err)
			return nil, err
		}

		// 权限中心可能没有回填 provider_config，直接取字段会空指针。
		oldHost := ""
		if resp.Data.BaseInfo.ProviderConfig != nil {
			oldHost = resp.Data.BaseInfo.ProviderConfig.Host
		}
		logs.V(5).Infof("update system host succeed, old: %s, new: %s", oldHost, host)
	}

	return &resp.Data, nil
}
