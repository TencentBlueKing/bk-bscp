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
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbas "github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server"
)

// fakeApplyClient 返回固定的鉴权判定，并记录申请信息接口收到的资源，
// 用于验证已拥有权限的资源不会被重复提交申请。
type fakeApplyClient struct {
	pbas.AuthClient

	decisions []bool
	applied   []*pbas.ResourceAttribute
}

func (f *fakeApplyClient) AuthorizeBatch(_ context.Context, req *pbas.AuthorizeBatchReq,
	_ ...grpc.CallOption) (*pbas.AuthorizeBatchResp, error) {

	decisions := make([]*pbas.Decision, len(req.Resources))
	for idx := range req.Resources {
		authorized := false
		if idx < len(f.decisions) {
			authorized = f.decisions[idx]
		}
		decisions[idx] = &pbas.Decision{Resource: req.Resources[idx], Authorized: authorized}
	}

	return &pbas.AuthorizeBatchResp{Decisions: decisions}, nil
}

func (f *fakeApplyClient) GetPermissionToApply(_ context.Context, req *pbas.GetPermissionToApplyReq,
	_ ...grpc.CallOption) (*pbas.GetPermissionToApplyResp, error) {

	f.applied = req.Resources

	return &pbas.GetPermissionToApplyResp{
		Permission: &pbas.IamPermission{},
		ApplyUrl:   "http://iam.example.com/apply",
	}, nil
}

// credentialListResources 模拟秘钥列表接口同时校验业务查看与秘钥查看两个资源。
func credentialListResources() []*meta.ResourceAttribute {
	return []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: 1},
		{Basic: meta.Basic{Type: meta.Credential, Action: meta.View}, BizID: 1},
	}
}

// 已有业务查看权限、缺少秘钥查看权限时，申请信息只应包含秘钥查看资源。
func TestAuthorizeAppliesOnlyMissingResources(t *testing.T) {
	client := &fakeApplyClient{decisions: []bool{true, false}}
	a := authorizer{authClient: client}

	err := a.Authorize(kit.New(), credentialListResources()...)

	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Len(t, client.applied, 1)
	require.Equal(t, string(meta.Credential), client.applied[0].Basic.Type)
	require.Equal(t, string(meta.View), client.applied[0].Basic.Action)
	require.Equal(t, uint32(1), client.applied[0].BizId)
}

// 业务、项目、环境、服务等层级资源按各自 Action 的判定过滤：
// 已通过的上级资源不会出现在申请信息里，只保留实际缺失的下级资源。
func TestAuthorizeAppliesOnlyMissingResourceInHierarchy(t *testing.T) {
	tests := []struct {
		name       string
		missing    *meta.ResourceAttribute
		wantType   meta.ResourceType
		wantAction meta.Action
	}{
		{
			name:       "credential view",
			missing:    &meta.ResourceAttribute{Basic: meta.Basic{Type: meta.Credential, Action: meta.View}, BizID: 1},
			wantType:   meta.Credential,
			wantAction: meta.View,
		},
		{
			name: "app view",
			missing: &meta.ResourceAttribute{
				Basic: meta.Basic{Type: meta.App, Action: meta.View, ResourceID: 2},
				BizID: 1, ProjectID: 3, EnvironmentID: 4,
			},
			wantType:   meta.App,
			wantAction: meta.View,
		},
		{
			name: "environment view",
			missing: &meta.ResourceAttribute{
				Basic: meta.Basic{Type: meta.Environment, Action: meta.View, ResourceID: 4},
				BizID: 1, ProjectID: 3,
			},
			wantType:   meta.Environment,
			wantAction: meta.View,
		},
		{
			name: "process operate",
			missing: &meta.ResourceAttribute{
				Basic: meta.Basic{Type: meta.ProcConfigMgmt, Action: meta.ProcessOperate}, BizID: 1,
			},
			wantType:   meta.ProcConfigMgmt,
			wantAction: meta.ProcessOperate,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			biz := &meta.ResourceAttribute{
				Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: 1,
			}
			client := &fakeApplyClient{decisions: []bool{true, false}}
			a := authorizer{authClient: client}

			err := a.Authorize(kit.New(), biz, tc.missing)

			require.Error(t, err)
			require.Equal(t, codes.PermissionDenied, status.Code(err))
			require.Len(t, client.applied, 1)
			require.Equal(t, string(tc.wantType), client.applied[0].Basic.Type)
			require.Equal(t, string(tc.wantAction), client.applied[0].Basic.Action)
		})
	}
}

// 业务查看与秘钥查看都缺失时，两个资源都应进入申请信息。
func TestAuthorizeAppliesAllResourcesWhenAllMissing(t *testing.T) {
	client := &fakeApplyClient{decisions: []bool{false, false}}
	a := authorizer{authClient: client}

	err := a.Authorize(kit.New(), credentialListResources()...)

	require.Error(t, err)
	require.Len(t, client.applied, 2)
	require.Equal(t, string(meta.FindBusinessResource), client.applied[0].Basic.Action)
	require.Equal(t, string(meta.View), client.applied[1].Basic.Action)
}

// 判定结果数量与资源数量不一致时无法可靠对齐，退回全量资源，避免漏申请。
func TestAuthorizeAppliesAllResourcesWhenDecisionsMismatched(t *testing.T) {
	client := &fakeApplyClient{decisions: []bool{false}}
	a := authorizer{authClient: client}

	err := a.Authorize(kit.New(), credentialListResources()...)

	require.Error(t, err)
	require.Len(t, client.applied, 2)
}

// 全部资源都通过校验时直接放行，不请求申请信息接口。
func TestAuthorizeSkipsApplyWhenAllAuthorized(t *testing.T) {
	client := &fakeApplyClient{decisions: []bool{true, true}}
	a := authorizer{authClient: client}

	err := a.Authorize(kit.New(), credentialListResources()...)

	require.NoError(t, err)
	require.Nil(t, client.applied)
}

func TestUnauthorizedResources(t *testing.T) {
	resources := credentialListResources()

	tests := []struct {
		name      string
		decisions []*meta.Decision
		want      int
	}{
		{
			name:      "all authorized",
			decisions: []*meta.Decision{{Authorized: true}, {Authorized: true}},
			want:      0,
		},
		{
			name:      "partial authorized",
			decisions: []*meta.Decision{{Authorized: true}, {Authorized: false}},
			want:      1,
		},
		{
			name:      "nil decision kept",
			decisions: []*meta.Decision{nil, {Authorized: false}},
			want:      2,
		},
		{
			name:      "mismatched decisions fallback to all",
			decisions: []*meta.Decision{{Authorized: true}},
			want:      2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Len(t, unauthorizedResources(tc.decisions, resources), tc.want)
		})
	}
}
