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

import "github.com/TencentBlueKing/bk-bscp/pkg/iam/client"

// ResourceTypeIDMap resource type map.
var ResourceTypeIDMap = map[client.TypeID]string{
	Business:    "业务",
	Application: "服务",
	Project:     "项目",
	Environment: "环境",
}

// GenerateStaticResourceTypes generate all the resource types registered to IAM.
func GenerateStaticResourceTypes() []client.ResourceType {
	resourceTypeList := make([]client.ResourceType, 0)

	// add business resources
	resourceTypeList = append(resourceTypeList, genBusinessResources()...)
	return resourceTypeList
}

func genBusinessResources() []client.ResourceType {
	// 顺序按祖先链从根到叶：注册时父资源必须先创建成功。
	return []client.ResourceType{
		{
			ID:            Project,
			Name:          ResourceTypeIDMap[Project],
			NameEn:        "Project",
			Description:   "项目",
			DescriptionEn: "project under a business",
			Parents: []client.Parent{{
				SystemID:   SystemIDCMDB,
				ResourceID: Business,
			}},
			ProviderConfig: client.ResourceConfig{
				Path: "/api/v1/auth/iam/find/resource",
			},
			Version: 1,
		},
		{
			// 环境按项目隔离：拥有项目权限即拥有该项目下全部环境的权限，
			// 也可以只把某个环境单独授权给用户。
			ID:            Environment,
			Name:          ResourceTypeIDMap[Environment],
			NameEn:        "Environment",
			Description:   "环境",
			DescriptionEn: "environment under a project",
			Parents: []client.Parent{{
				SystemID:   SystemIDBSCP,
				ResourceID: Project,
			}},
			ProviderConfig: client.ResourceConfig{
				Path: "/api/v1/auth/iam/find/resource",
			},
			Version: 1,
		},
		{
			// 服务挂在环境下，与 V4 的祖先链保持一致：
			// 按业务、项目或环境维度的授权都能通过三级路径覆盖到该服务。
			ID:            Application,
			Name:          ResourceTypeIDMap[Application],
			NameEn:        "Application",
			Description:   "服务",
			DescriptionEn: "application under an environment",
			Parents: []client.Parent{{
				SystemID:   SystemIDBSCP,
				ResourceID: Environment,
			}},
			ProviderConfig: client.ResourceConfig{
				Path: "/api/v1/auth/iam/find/resource",
			},
			Version: 1,
		},
	}
}
