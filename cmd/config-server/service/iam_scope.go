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

// appViewAuthResources 服务内资源查看操作的鉴权资源：
// 业务访问 + 项目查看 + 环境查看 + 服务查看。
func appViewAuthResources(bizID, projectID, envID, appID uint32) []*meta.ResourceAttribute {
	return meta.AppAuthResources(bizID, projectID, envID, appID, meta.View)
}

// appWriteAuthResources 服务内资源写操作（新增/编辑/删除/恢复/撤销等）的鉴权资源：
// 业务访问 + 项目查看 + 环境查看 + 服务编辑。
func appWriteAuthResources(bizID, projectID, envID, appID uint32) []*meta.ResourceAttribute {
	return meta.AppAuthResources(bizID, projectID, envID, appID, meta.Update)
}
