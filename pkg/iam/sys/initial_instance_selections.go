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

// GenerateStaticInstanceSelections return need register instance selection.
func GenerateStaticInstanceSelections() []client.InstanceSelection {
	return []client.InstanceSelection{
		{
			ID:     BusinessSelection,
			Name:   "业务列表",
			NameEn: "Business List",
			ResourceTypeChain: []client.ResourceChain{
				{
					SystemID: SystemIDCMDB,
					ID:       Business,
				},
			},
		},
		{
			// 服务挂在环境下：实例视图的链路必须与资源类型的父级一致，
			// 否则权限中心按「业务 → 项目 → 环境 → 服务」选资源时会取不到实例。
			ID:     ApplicationSelection,
			Name:   "服务列表",
			NameEn: "Application List",
			ResourceTypeChain: []client.ResourceChain{
				{
					SystemID: SystemIDCMDB,
					ID:       Business,
				},
				{
					SystemID: SystemIDBSCP,
					ID:       Project,
				},
				{
					SystemID: SystemIDBSCP,
					ID:       Environment,
				},
				{
					SystemID: SystemIDBSCP,
					ID:       Application,
				},
			},
		},
		{
			ID:     ProjectSelection,
			Name:   "项目列表",
			NameEn: "Project List",
			ResourceTypeChain: []client.ResourceChain{
				{
					SystemID: SystemIDCMDB,
					ID:       Business,
				},
				{
					SystemID: SystemIDBSCP,
					ID:       Project,
				},
			},
		},
		{
			ID:     EnvironmentSelection,
			Name:   "环境列表",
			NameEn: "Environment List",
			ResourceTypeChain: []client.ResourceChain{
				{
					SystemID: SystemIDCMDB,
					ID:       Business,
				},
				{
					SystemID: SystemIDBSCP,
					ID:       Project,
				},
				{
					SystemID: SystemIDBSCP,
					ID:       Environment,
				},
			},
		},
	}
}
