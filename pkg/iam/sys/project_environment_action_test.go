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

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
)

// relatedResource 返回操作关联的唯一资源类型。
func relatedResource(t *testing.T, action client.ResourceAction) client.RelateResourceType {
	t.Helper()

	if len(action.RelatedResourceTypes) != 1 {
		t.Fatalf("action %q should relate to exactly one resource type", action.ID)
	}

	return action.RelatedResourceTypes[0]
}

// commonActionIDs 返回指定通用操作（业务只读/业务运维）包含的操作集合。
func commonActionIDs(t *testing.T, englishName string) map[client.ActionID]struct{} {
	t.Helper()

	for _, common := range GenerateCommonActions() {
		if common.EnglishName != englishName {
			continue
		}

		ids := make(map[client.ActionID]struct{}, len(common.Actions))
		for _, action := range common.Actions {
			ids[action.ID] = struct{}{}
		}

		return ids
	}

	t.Fatalf("common action %q is not registered", englishName)

	return nil
}

func staticActions(t *testing.T) map[client.ActionID]client.ResourceAction {
	t.Helper()

	actions := make(map[client.ActionID]client.ResourceAction)
	for _, action := range GenerateStaticActions() {
		actions[action.ID] = action
	}

	return actions
}

// 项目按项目实例隔离：创建按业务授权（此时项目实例还不存在），查看/编辑/删除按项目授权。
func TestProjectActionsAreScopedPerProject(t *testing.T) {
	actions := staticActions(t)

	create, ok := actions[ProjectCreate]
	if !ok {
		t.Fatalf("project action %q is not registered", ProjectCreate)
	}
	created := relatedResource(t, create)
	if created.SystemID != SystemIDCMDB || created.ID != Business {
		t.Errorf("project_create should be authorized on cmdb business, got %s/%s",
			created.SystemID, created.ID)
	}

	for _, id := range []client.ActionID{ProjectView, ProjectEdit, ProjectDelete} {
		action, ok := actions[id]
		if !ok {
			t.Errorf("project action %q is not registered", id)
			continue
		}

		related := relatedResource(t, action)
		if related.SystemID != SystemIDBSCP || related.ID != Project {
			t.Errorf("project action %q should be authorized on bscp project, got %s/%s",
				id, related.SystemID, related.ID)
		}
	}
}

// 环境是独立授权资源：创建按所属项目授权，查看/编辑/删除按环境实例授权。
func TestEnvironmentActionsAreRegistered(t *testing.T) {
	actions := staticActions(t)

	want := map[client.ActionID]client.TypeID{
		EnvironmentCreate: Project,
		EnvironmentView:   Environment,
		EnvironmentEdit:   Environment,
		EnvironmentDelete: Environment,
	}

	for id, wantType := range want {
		action, ok := actions[id]
		if !ok {
			t.Errorf("environment action %q is not registered", id)
			continue
		}

		related := relatedResource(t, action)
		if related.SystemID != SystemIDBSCP || related.ID != wantType {
			t.Errorf("environment action %q should be authorized on bscp %s, got %s/%s",
				id, wantType, related.SystemID, related.ID)
		}
	}
}

// 环境资源类型挂在项目下，项目挂在业务下，构成 业务 → 项目 → 环境 的拓扑。
func TestEnvironmentResourceTypeAndSelection(t *testing.T) {
	resourceTypes := make(map[client.TypeID]client.ResourceType)
	for _, rt := range GenerateStaticResourceTypes() {
		resourceTypes[rt.ID] = rt
	}

	projectType, ok := resourceTypes[Project]
	if !ok {
		t.Fatalf("resource type %q is not registered", Project)
	}
	if len(projectType.Parents) != 1 ||
		projectType.Parents[0].SystemID != SystemIDCMDB || projectType.Parents[0].ResourceID != Business {
		t.Errorf("project should have cmdb business as its only parent, got %v", projectType.Parents)
	}

	envType, ok := resourceTypes[Environment]
	if !ok {
		t.Fatalf("resource type %q is not registered", Environment)
	}
	if len(envType.Parents) != 1 ||
		envType.Parents[0].SystemID != SystemIDBSCP || envType.Parents[0].ResourceID != Project {
		t.Errorf("environment should have bscp project as its only parent, got %v", envType.Parents)
	}

	var found bool
	for _, selection := range GenerateStaticInstanceSelections() {
		if selection.ID != EnvironmentSelection {
			continue
		}
		found = true

		if len(selection.ResourceTypeChain) != 3 ||
			selection.ResourceTypeChain[0].SystemID != SystemIDCMDB ||
			selection.ResourceTypeChain[0].ID != Business ||
			selection.ResourceTypeChain[1].SystemID != SystemIDBSCP ||
			selection.ResourceTypeChain[1].ID != Project ||
			selection.ResourceTypeChain[2].SystemID != SystemIDBSCP ||
			selection.ResourceTypeChain[2].ID != Environment {
			t.Errorf("environment selection chain should be biz -> project -> environment, got %v",
				selection.ResourceTypeChain)
		}
	}
	if !found {
		t.Errorf("instance selection %q is not registered", EnvironmentSelection)
	}
}

// 服务挂在环境下，与 V4 的祖先链保持一致：
// 资源类型父级是环境，实例视图链路是 业务 → 项目 → 环境 → 服务。
func TestApplicationIsUnderEnvironment(t *testing.T) {
	resourceTypes := make(map[client.TypeID]client.ResourceType)
	for _, rt := range GenerateStaticResourceTypes() {
		resourceTypes[rt.ID] = rt
	}

	appType, ok := resourceTypes[Application]
	if !ok {
		t.Fatalf("resource type %q is not registered", Application)
	}
	if len(appType.Parents) != 1 ||
		appType.Parents[0].SystemID != SystemIDBSCP || appType.Parents[0].ResourceID != Environment {
		t.Errorf("application should have bscp environment as its only parent, got %v", appType.Parents)
	}

	var found bool
	for _, selection := range GenerateStaticInstanceSelections() {
		if selection.ID != ApplicationSelection {
			continue
		}
		found = true

		chain := selection.ResourceTypeChain
		if len(chain) != 4 ||
			chain[0].SystemID != SystemIDCMDB || chain[0].ID != Business ||
			chain[1].SystemID != SystemIDBSCP || chain[1].ID != Project ||
			chain[2].SystemID != SystemIDBSCP || chain[2].ID != Environment ||
			chain[3].SystemID != SystemIDBSCP || chain[3].ID != Application {
			t.Errorf("application selection chain should be biz -> project -> environment -> app, got %v", chain)
		}
	}
	if !found {
		t.Errorf("instance selection %q is not registered", ApplicationSelection)
	}
}

// 服务创建按环境授权：创建时尚无服务实例，授权落在服务将要归属的环境上，
// 授权页面因此需要先选项目、再选环境。
func TestAppCreateIsScopedPerEnvironment(t *testing.T) {
	actions := staticActions(t)

	action, ok := actions[AppCreate]
	if !ok {
		t.Fatalf("app action %q is not registered", AppCreate)
	}

	related := relatedResource(t, action)
	if related.SystemID != SystemIDBSCP || related.ID != Environment {
		t.Errorf("app_create should be authorized on bscp environment, got %s/%s",
			related.SystemID, related.ID)
	}

	if len(related.InstanceSelections) != 1 ||
		related.InstanceSelections[0].SystemID != SystemIDBSCP ||
		related.InstanceSelections[0].ID != EnvironmentSelection {
		t.Errorf("app_create should select instances from %q, got %v",
			EnvironmentSelection, related.InstanceSelections)
	}
}

// 环境创建者自动授权需要在资源创建者配置里注册环境。
func TestEnvironmentResourceCreatorActions(t *testing.T) {
	creatorActions := GenerateResourceCreatorActions()

	for _, config := range creatorActions.Config {
		if config.ResourceID != Environment {
			continue
		}

		ids := make(map[client.ActionID]bool, len(config.Actions))
		for _, action := range config.Actions {
			ids[action.ID] = true
		}

		for _, want := range []client.ActionID{EnvironmentView, EnvironmentEdit, EnvironmentDelete} {
			if !ids[want] {
				t.Errorf("environment creator should be granted %q", want)
			}
		}

		return
	}

	t.Errorf("resource creator actions of %q are not registered", Environment)
}

// 通用操作「业务只读」必须包含环境查看，否则勾选业务只读后环境仍不可见。
func TestBizViewerCommonActionIncludesEnvironmentView(t *testing.T) {
	ids := commonActionIDs(t, "business view")
	if _, ok := ids[EnvironmentView]; !ok {
		t.Errorf("业务只读 should include %q", EnvironmentView)
	}
}

// 通用操作「业务运维」应覆盖全部已注册操作：漏一个操作，勾选业务运维后仍会缺这项权限。
func TestBizOperatorCommonActionCoversAllActions(t *testing.T) {
	ids := commonActionIDs(t, "business ops")
	for _, action := range GenerateStaticActions() {
		if _, ok := ids[action.ID]; !ok {
			t.Errorf("业务运维 should include action %q", action.ID)
		}
	}
}
