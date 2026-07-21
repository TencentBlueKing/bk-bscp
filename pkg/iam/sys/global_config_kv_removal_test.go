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

import "testing"

// removedGlobalConfigKVActionID 是已下线的全局配置 KV 管理权限点 action id。
// 用字符串字面量而非常量断言，保证常量删除后测试仍可编译，防止权限点被再次注册回来。
const removedGlobalConfigKVActionID = "manage_global_config_kv"

// TestGlobalConfigKVActionNotRegistered 校验全局配置 KV 管理权限点已从 IAM 静态注册产物中彻底移除，
// 使业务方在权限中心不再能检索/申请该权限点。
func TestGlobalConfigKVActionNotRegistered(t *testing.T) {
	for _, action := range GenerateStaticActions() {
		if string(action.ID) == removedGlobalConfigKVActionID {
			t.Errorf("static action %q should have been removed", removedGlobalConfigKVActionID)
		}
	}

	for _, group := range GenerateStaticActionGroups() {
		for _, action := range group.Actions {
			if string(action.ID) == removedGlobalConfigKVActionID {
				t.Errorf("action group %q still references removed action %q", group.Name, removedGlobalConfigKVActionID)
			}
		}
	}

	for _, common := range GenerateCommonActions() {
		for _, action := range common.Actions {
			if string(action.ID) == removedGlobalConfigKVActionID {
				t.Errorf("common action %q still references removed action %q", common.Name, removedGlobalConfigKVActionID)
			}
		}
	}
}
