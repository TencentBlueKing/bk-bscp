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

package auth

import (
	"context"
	"fmt"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	clientv4 "github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/model"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// v3TopologyLister 是 V3 探测资源拓扑所需的最小能力，*sys.Sys 实现。
type v3TopologyLister interface {
	ResourceTypes(ctx context.Context) ([]client.ResourceType, error)
}

// v4TopologyLister 是 V4 探测资源拓扑所需的最小能力，*clientv4.Client 实现。
type v4TopologyLister interface {
	ListResourceTypes(kt *kit.Kit, page, pageSize int) ([]clientv4.ResourceType, int, error)
}

// DetectTopologyV3 探测权限中心 V3 已注册的资源拓扑。
func DetectTopologyV3(ctx context.Context, lister v3TopologyLister) (meta.ResourceTopology, error) {
	if lister == nil {
		return "", fmt.Errorf("iam v3 topology lister is nil")
	}

	resourceTypes, err := lister.ResourceTypes(ctx)
	if err != nil {
		return "", err
	}

	return topologyOfV3(resourceTypes)
}

// topologyOfV3 判定 V3 资源拓扑：服务资源的父链包含环境即为新模型。
func topologyOfV3(resourceTypes []client.ResourceType) (meta.ResourceTopology, error) {
	for _, resourceType := range resourceTypes {
		if resourceType.ID != sys.Application {
			continue
		}

		for _, parent := range resourceType.Parents {
			if parent.ResourceID == sys.Environment {
				return meta.TopologyHierarchical, nil
			}
		}

		// 服务已注册但没有环境父级，说明权限中心仍是「业务 → 服务」的旧模型。
		return meta.TopologyLegacy, nil
	}

	return "", fmt.Errorf("resource type %s is not registered in iam", sys.Application)
}

// DetectTopologyV4 探测权限中心 V4 已注册的资源拓扑。
func DetectTopologyV4(kt *kit.Kit, lister v4TopologyLister) (meta.ResourceTopology, error) {
	if lister == nil {
		return "", fmt.Errorf("iam v4 topology lister is nil")
	}

	resourceTypes := make([]clientv4.ResourceType, 0)
	for page := 1; ; page++ {
		types, count, err := lister.ListResourceTypes(kt, page, clientv4.MaxPageSize)
		if err != nil {
			return "", err
		}

		resourceTypes = append(resourceTypes, types...)
		if len(types) == 0 || len(resourceTypes) >= count {
			break
		}
	}

	return topologyOfV4(resourceTypes)
}

// topologyOfV4 判定 V4 资源拓扑：服务资源的祖先链包含环境即为新模型。
func topologyOfV4(resourceTypes []clientv4.ResourceType) (meta.ResourceTopology, error) {
	for _, resourceType := range resourceTypes {
		if resourceType.ID != model.ResourceTypeApp {
			continue
		}

		for _, ancestor := range resourceType.Ancestors {
			if ancestor == model.ResourceTypeEnvironment {
				return meta.TopologyHierarchical, nil
			}
		}

		return meta.TopologyLegacy, nil
	}

	return "", fmt.Errorf("resource type %s is not registered in iam", model.ResourceTypeApp)
}

// ResolveTopology 解析最终生效的资源拓扑。
//
// configured 为配置取值，空或 auto 时调用 probe 探测；探测失败直接返回错误，不再回退：
// 权限中心已是新拓扑时按 legacy 裁剪服务路径会让服务级授权全部匹配不到，
// 按旧拓扑启动的故障远比启动失败难排查，因此只能显式配置 topology 才能继续。
// 开启项目/环境维度校验却处于旧拓扑时同样直接报错：此时项目/环境资源类型并不存在，
// 继续启动只会让项目、环境相关操作全部被权限中心拒绝。
func ResolveTopology(enabled bool, configured string, probe func() (meta.ResourceTopology, error)) (
	meta.ResourceTopology, error) {

	topology := meta.ResourceTopology(configured)

	if configured == "" || configured == "auto" {
		detected, err := probe()
		if err != nil {
			return "", fmt.Errorf("detect iam resource topology failed: %w; "+
				"please set iam.project_environment_auth.topology explicitly (legacy or hierarchical)", err)
		}

		topology = detected
	}

	if enabled && topology.IsLegacy() {
		return "", fmt.Errorf("iam.project_environment_auth.enabled is true but the iam resource topology is legacy, " +
			"please register the project/environment model first: " +
			"bk-bscp-authservice migrate init-iam (v3) or migrate init-iam-v4 (v4)")
	}

	return topology, nil
}
