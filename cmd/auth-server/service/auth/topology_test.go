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
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	clientv4 "github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/model"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// fakeV3Lister 是 V3 拓扑探测的替身。
type fakeV3Lister struct {
	types []client.ResourceType
	err   error
}

func (f fakeV3Lister) ResourceTypes(_ context.Context) ([]client.ResourceType, error) {
	return f.types, f.err
}

// fakeV4Lister 是 V4 拓扑探测的替身。
type fakeV4Lister struct {
	types []clientv4.ResourceType
	err   error
}

func (f fakeV4Lister) ListResourceTypes(_ *kit.Kit, _, _ int) ([]clientv4.ResourceType, int, error) {
	return f.types, len(f.types), f.err
}

// 服务资源挂环境时判定为新拓扑。
func TestDetectTopologyV3Hierarchical(t *testing.T) {
	lister := fakeV3Lister{types: []client.ResourceType{
		{ID: sys.Project, Parents: []client.Parent{{SystemID: sys.SystemIDCMDB, ResourceID: sys.Business}}},
		{ID: sys.Application, Parents: []client.Parent{{SystemID: sys.SystemIDBSCP, ResourceID: sys.Environment}}},
	}}

	topology, err := DetectTopologyV3(context.Background(), lister)
	require.NoError(t, err)
	require.Equal(t, meta.TopologyHierarchical, topology)
}

// 服务资源直接挂业务时判定为旧拓扑。
func TestDetectTopologyV3Legacy(t *testing.T) {
	lister := fakeV3Lister{types: []client.ResourceType{
		{ID: sys.Application, Parents: []client.Parent{{SystemID: sys.SystemIDCMDB, ResourceID: sys.Business}}},
	}}

	topology, err := DetectTopologyV3(context.Background(), lister)
	require.NoError(t, err)
	require.Equal(t, meta.TopologyLegacy, topology)
}

// 服务资源类型未注册时无法判定，返回错误由调用方回退。
func TestDetectTopologyV3NotRegistered(t *testing.T) {
	_, err := DetectTopologyV3(context.Background(), fakeV3Lister{})
	require.ErrorContains(t, err, "not registered")
}

func TestDetectTopologyV3ListerError(t *testing.T) {
	_, err := DetectTopologyV3(context.Background(), fakeV3Lister{err: errors.New("gateway unavailable")})
	require.ErrorContains(t, err, "gateway unavailable")
}

// V4 服务资源祖先链包含环境时判定为新拓扑。
func TestDetectTopologyV4Hierarchical(t *testing.T) {
	lister := fakeV4Lister{types: []clientv4.ResourceType{
		{ID: model.ResourceTypeApp, Ancestors: []string{
			model.ResourceTypeBiz, model.ResourceTypeProject, model.ResourceTypeEnvironment,
		}},
	}}

	topology, err := DetectTopologyV4(kit.New(), lister)
	require.NoError(t, err)
	require.Equal(t, meta.TopologyHierarchical, topology)
}

// V4 服务资源只挂业务时判定为旧拓扑。
func TestDetectTopologyV4Legacy(t *testing.T) {
	lister := fakeV4Lister{types: []clientv4.ResourceType{
		{ID: model.ResourceTypeApp, Ancestors: []string{model.ResourceTypeBiz}},
	}}

	topology, err := DetectTopologyV4(kit.New(), lister)
	require.NoError(t, err)
	require.Equal(t, meta.TopologyLegacy, topology)
}

func TestDetectTopologyV4NotRegistered(t *testing.T) {
	_, err := DetectTopologyV4(kit.New(), fakeV4Lister{})
	require.ErrorContains(t, err, "not registered")
}

// 显式配置的拓扑不再探测。
func TestResolveTopologyExplicit(t *testing.T) {
	probed := false
	probe := func() (meta.ResourceTopology, error) {
		probed = true
		return meta.TopologyLegacy, nil
	}

	topology, err := ResolveTopology(false, "hierarchical", probe)
	require.NoError(t, err)
	require.Equal(t, meta.TopologyHierarchical, topology)
	require.False(t, probed)
}

// auto 时以探测结果为准。
func TestResolveTopologyAutoUsesProbe(t *testing.T) {
	probe := func() (meta.ResourceTopology, error) { return meta.TopologyHierarchical, nil }

	topology, err := ResolveTopology(false, "auto", probe)
	require.NoError(t, err)
	require.Equal(t, meta.TopologyHierarchical, topology)
}

// 探测失败时启动失败，不再静默回退旧拓扑：新拓扑下按 legacy 裁剪路径会让服务级授权全部失效。
// 错误信息需提示显式配置 topology，便于运维在权限中心不可用时自行选择。
func TestResolveTopologyProbeFailedFailsStartup(t *testing.T) {
	probe := func() (meta.ResourceTopology, error) { return "", errors.New("iam unavailable") }

	for _, enabled := range []bool{false, true} {
		_, err := ResolveTopology(enabled, "auto", probe)
		require.ErrorContains(t, err, "iam unavailable")
		require.ErrorContains(t, err, "iam.project_environment_auth.topology")
	}
}

// 开启项目/环境校验但权限中心仍是旧拓扑时启动失败，避免所有项目/环境操作被拒绝。
func TestResolveTopologyEnabledRejectsLegacy(t *testing.T) {
	probe := func() (meta.ResourceTopology, error) { return meta.TopologyLegacy, nil }

	_, err := ResolveTopology(true, "auto", probe)
	require.ErrorContains(t, err, "topology is legacy")
}

// 关闭开关时旧拓扑可正常启动，与改造前行为一致。
func TestResolveTopologyDisabledAllowsLegacy(t *testing.T) {
	probe := func() (meta.ResourceTopology, error) { return meta.TopologyLegacy, nil }

	topology, err := ResolveTopology(false, "auto", probe)
	require.NoError(t, err)
	require.Equal(t, meta.TopologyLegacy, topology)
}
