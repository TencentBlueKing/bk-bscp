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

import (
	"context"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbcrs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/credential-scope"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// ListCredentialScopes get credential scopes
func (s *Service) ListCredentialScopes(ctx context.Context,
	req *pbcs.ListCredentialScopesReq) (*pbcs.ListCredentialScopesResp, error) {

	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Credential, Action: meta.View}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(grpcKit, res...)
	if err != nil {
		return nil, err
	}

	r := &pbds.ListCredentialScopesReq{
		BizId:        req.BizId,
		CredentialId: req.CredentialId,
		ProjectId:    grpcKit.ResolvedProjectID(req.ProjectId),
	}
	rp, err := s.client.DS.ListCredentialScopes(grpcKit.RpcCtx(), r)
	if err != nil {
		logs.Errorf("list credential scope failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	resp := &pbcs.ListCredentialScopesResp{
		Count:   rp.Count,
		Details: rp.Details,
	}
	return resp, nil

}

// UpdateCredentialScope  update credential scope
func (s *Service) UpdateCredentialScope(ctx context.Context, req *pbcs.UpdateCredentialScopeReq) (*pbcs.UpdateCredentialScopeResp, error) {

	grpcKit := kit.FromGrpcContext(ctx)

	resp := new(pbcs.UpdateCredentialScopeResp)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Credential, Action: meta.Manage}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(grpcKit, res...)
	if err != nil {
		return nil, err
	}

	r := &pbds.UpdateCredentialScopesReq{
		BizId:        req.BizId,
		CredentialId: req.CredentialId,
		ProjectId:    grpcKit.ResolvedProjectID(req.ProjectId),
	}

	// 预先查询该项目的默认环境 ID，避免循环内重复 RPC 调用
	defaultEnvID, err := s.resolveDefaultEnvID(grpcKit, req.BizId, r.ProjectId)
	if err != nil {
		return nil, err
	}

	// 1. 处理新增的 Scope (AddScope)
	for _, spec := range req.AddScope {
		if spec == nil {
			continue
		}

		envID := spec.EnvId
		if envID == 0 {
			envID = defaultEnvID
		}

		// 构造底层的 pbds 结构体
		r.Created = append(r.Created, &pbcrs.CredentialScopeSpec{
			App:   spec.App,
			Scope: spec.Scope,
			EnvId: envID,
		})
	}

	// 2. 处理修改的 Scope (AlterScope)
	for _, spec := range req.AlterScope {
		if spec == nil {
			continue
		}

		envID := spec.EnvId
		if envID == 0 {
			envID = defaultEnvID
		}

		r.Updated = append(r.Updated, &pbcrs.UpdateScopeSpec{
			Id:    spec.Id,
			App:   spec.App,
			Scope: spec.Scope,
			EnvId: envID,
		})
	}

	r.Deleted = append(r.Deleted, req.DelId...)

	_, err = s.client.DS.UpdateCredentialScopes(grpcKit.RpcCtx(), r)
	if err != nil {
		logs.Errorf("update credential scope failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	return resp, nil
}

// resolveDefaultEnvID 按 ProjectId 查询该项目的默认环境 ID。
// 用于 UpdateCredentialScope 中 env_id 未传时的回退场景：
// 路由链 checkOrCreateDefaultProjectEnv → VerifyProjectExists 会覆盖 ProjectID 但不重算 EnvID，
// 直接使用 grpcKit.ResolvedEnvID 可能拿到默认项目的环境 ID，导致非默认项目写入错误的 scope。
func (s *Service) resolveDefaultEnvID(grpcKit *kit.Kit, bizId, projectId uint32) (uint32, error) {
	resp, err := s.client.DS.GetDefaultEnvironment(grpcKit.RpcCtx(), &pbds.GetDefaultEnvironmentReq{
		BizId:     bizId,
		ProjectId: projectId,
	})
	if err != nil {
		logs.Errorf("resolve default env id failed, bizId=%d projectId=%d err=%v rid=%s", bizId, projectId, err, grpcKit.Rid)
		return 0, err
	}
	return resp.Id, nil
}
