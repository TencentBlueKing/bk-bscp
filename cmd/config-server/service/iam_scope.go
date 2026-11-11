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

package service

import "github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"

// projectViewResource 构造项目维度的查看权限校验资源。
//
// 项目/环境维度引入后，分组、脚本、模板、变量、配置项等下游资源都归属于某个项目，
// 对其操作需要同时校验项目查看权限，避免仅凭业务访问或服务权限即可越级操作。
// 子资源的新增、编辑、删除一律只校验所属项目的查看权限，不再要求项目编辑/删除权限；
// 项目的编辑/删除权限只在项目本身的管理操作上校验。
// 关闭项目/环境维度校验开关时，该资源会被 auth-server 跳过，保持原有的业务维度校验逻辑。
func projectViewResource(bizID, projectID uint32) *meta.ResourceAttribute {
	return &meta.ResourceAttribute{
		Basic:     meta.Basic{Type: meta.Project, Action: meta.View, ResourceID: projectID},
		BizID:     bizID,
		ProjectID: projectID,
	}
}

// environmentViewResource 构造环境维度的查看权限校验资源。
//
// 配置项属于服务，服务又挂在环境下：配置项操作在服务维度之外，还需校验所属环境的查看权限。
// 环境 ID 由 auth-server 侧按实例回查真实父级（业务、项目），调用方只需给出环境实例 ID。
func environmentViewResource(bizID, envID uint32) *meta.ResourceAttribute {
	return &meta.ResourceAttribute{
		Basic: meta.Basic{Type: meta.Environment, Action: meta.View, ResourceID: envID},
		BizID: bizID,
	}
}

// appViewAuthResources 服务内资源查看操作的鉴权资源：
// 业务访问 + 项目查看 + 环境查看 + 服务查看。
func appViewAuthResources(bizID, projectID, envID, appID uint32) []*meta.ResourceAttribute {
	return appAuthResources(bizID, projectID, envID, appID, meta.View)
}

// appWriteAuthResources 服务内资源写操作（新增/编辑/删除/恢复/撤销等）的鉴权资源：
// 业务访问 + 项目查看 + 环境查看 + 服务编辑。
func appWriteAuthResources(bizID, projectID, envID, appID uint32) []*meta.ResourceAttribute {
	return appAuthResources(bizID, projectID, envID, appID, meta.Update)
}

// appAuthResources 构造服务内资源操作的鉴权资源，四项必须同时通过：
// 业务访问、项目查看、环境查看与服务动作（查看/编辑/生成版本/上线等）。
//
// 配置项、KV、版本、上线全部挂在服务下，规则必须同一条：任意一处自行拼装或漏掉维度，
// 都会出现"能改配置项却读不到 KV""能生成版本却上不了线"这类不一致的权限体验。
// 开启项目/环境维度校验时四项同时校验；关闭时项目/环境资源被 auth-server 跳过，
// 退回业务 + 服务维度。
// 服务动作可传多个，用于一次调用同时要求多个动作的场景（如生成版本并上线）。
// 服务的业务/项目/环境归属由 auth-server 按服务实例回查，这里的 projectID/envID
// 只用于项目、环境两项资源本身。
func appAuthResources(bizID, projectID, envID, appID uint32, actions ...meta.Action) []*meta.ResourceAttribute {
	res := make([]*meta.ResourceAttribute, 0, 3+len(actions))
	res = append(res,
		&meta.ResourceAttribute{
			Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource},
			BizID: bizID,
		},
		projectViewResource(bizID, projectID),
		environmentViewResource(bizID, envID),
	)

	for _, action := range actions {
		res = append(res, &meta.ResourceAttribute{
			Basic: meta.Basic{Type: meta.App, Action: action, ResourceID: appID},
			BizID: bizID,
		})
	}

	return res
}
