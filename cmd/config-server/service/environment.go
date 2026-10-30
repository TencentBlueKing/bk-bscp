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
	"strconv"

	"google.golang.org/protobuf/proto"

	iamauth "github.com/TencentBlueKing/bk-bscp/internal/iam/auth"
	"github.com/TencentBlueKing/bk-bscp/internal/rest/view/webannotation"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbenvironment "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/environment"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// CreateEnvironment implements [pbcs.ConfigServer].
func (s *Service) CreateEnvironment(ctx context.Context, req *pbcs.CreateEnvironmentReq) (*pbcs.CreateEnvironmentResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{
			Basic:     meta.Basic{Type: meta.Environment, Action: meta.Create, ResourceID: req.ProjectId},
			BizID:     req.BizId,
			ProjectID: req.ProjectId,
		},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.CreateEnvironment(grpcKit.RpcCtx(), &pbds.CreateEnvironmentReq{
		BizId:     req.GetBizId(),
		ProjectId: req.GetProjectId(),
		Name:      req.GetName(),
		Type:      req.GetType(),
		Memo:      req.GetMemo(),
	})

	if err != nil {
		return nil, err
	}

	// 环境创建者自动获得该环境的运维权限：环境仍需按项目/环境维度分别授权，
	// 因此这里同样拆成环境与业务两条，业务维度用于放行所有业务接口的前置校验。
	if err := s.authorizer.GrantResourceCreatorAction(grpcKit, &client.GrantResourceCreatorActionOption{
		System:  sys.SystemIDBSCP,
		Type:    sys.Environment,
		ID:      strconv.Itoa(int(resp.GetId())),
		Name:    req.GetName(),
		Creator: grpcKit.User,
		Ancestors: []client.GrantResourceCreatorActionAncestor{
			{
				System: sys.SystemIDCMDB,
				Type:   sys.Business,
				ID:     strconv.Itoa(int(req.GetBizId())),
			},
			{
				System: sys.SystemIDBSCP,
				Type:   sys.Project,
				ID:     strconv.Itoa(int(req.GetProjectId())),
			},
		},
	}); err != nil {
		logs.Errorf("grant environment creator action failed, err: %v, rid: %s", err, grpcKit.Rid)
	}

	return &pbcs.CreateEnvironmentResp{
		Id: resp.GetId(),
	}, nil
}

// DeleteEnvironment implements [pbcs.ConfigServer].
func (s *Service) DeleteEnvironment(ctx context.Context, req *pbcs.DeleteEnvironmentReq) (*pbcs.DeleteEnvironmentResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{
			Basic:     meta.Basic{Type: meta.Environment, Action: meta.Delete, ResourceID: req.EnvId},
			BizID:     req.BizId,
			ProjectID: req.ProjectId,
		},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	_, err := s.client.DS.DeleteEnvironment(grpcKit.RpcCtx(), &pbds.DeleteEnvironmentReq{
		BizId:     req.GetBizId(),
		ProjectId: req.GetProjectId(),
		EnvId:     req.GetEnvId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.DeleteEnvironmentResp{}, nil
}

// GetEnvironment implements [pbcs.ConfigServer].
func (s *Service) GetEnvironment(ctx context.Context, req *pbcs.GetEnvironmentReq) (*pbcs.GetEnvironmentResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{
			Basic:     meta.Basic{Type: meta.Environment, Action: meta.View, ResourceID: req.EnvId},
			BizID:     req.BizId,
			ProjectID: req.ProjectId,
		},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.GetEnvironment(grpcKit.RpcCtx(), &pbds.GetEnvironmentReq{
		BizId:     req.GetBizId(),
		ProjectId: req.GetProjectId(),
		EnvId:     req.GetEnvId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.GetEnvironmentResp{
		Id:         resp.GetId(),
		Spec:       resp.GetSpec(),
		Attachment: resp.GetAttachment(),
	}, nil
}

// ListEnvironments implements [pbcs.ConfigServer].
// 只校验项目可见性，逐环境的权限判定交给 ListEnvironmentsAnnotation：
// 无权限的环境也要返回给前端置灰展示，用户才知道环境存在、也才能点击申请环境权限；
// 直接过滤会把环境整体隐藏，用户既看不到也没有申请入口。
func (s *Service) ListEnvironments(ctx context.Context, req *pbcs.ListEnvironmentsReq) (*pbcs.ListEnvironmentsResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{
			Basic: meta.Basic{Type: meta.Project, Action: meta.View, ResourceID: req.ProjectId},
			BizID: req.BizId,
		},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.ListEnvironments(grpcKit.RpcCtx(), &pbds.ListEnvironmentsReq{
		BizId:           req.GetBizId(),
		ProjectId:       req.GetProjectId(),
		Start:           req.GetStart(),
		Limit:           req.GetLimit(),
		All:             req.GetAll(),
		SearchCondition: req.GetSearchCondition(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ListEnvironmentsResp{
		ProdEnvironments:    resp.GetProdEnvironments(),
		StagingEnvironments: resp.GetStagingEnvironments(),
		TestEnvironments:    resp.GetTestEnvironments(),
		DevEnvironments:     resp.GetDevEnvironments(),
	}, nil
}

// ListEnvironmentsAnnotation 环境列表的逐项权限注解。
// 环境按环境实例授权，列表里每个环境都要给出查看/编辑/删除的判定结果，
// 前端据此把无权限环境置灰，并在点击时引导申请权限。
func ListEnvironmentsAnnotation(ctx context.Context, kt *kit.Kit,
	authorizer iamauth.Authorizer, msg proto.Message) (*webannotation.Annotation, error) {

	resp, ok := msg.(*pbcs.ListEnvironmentsResp)
	if !ok {
		return nil, nil
	}

	envs := make([]*pbenvironment.Environment, 0,
		len(resp.GetProdEnvironments())+len(resp.GetStagingEnvironments())+
			len(resp.GetTestEnvironments())+len(resp.GetDevEnvironments()))
	envs = append(envs, resp.GetProdEnvironments()...)
	envs = append(envs, resp.GetStagingEnvironments()...)
	envs = append(envs, resp.GetTestEnvironments()...)
	envs = append(envs, resp.GetDevEnvironments()...)

	perms := make(map[string]webannotation.Perm, len(envs))
	if len(envs) == 0 {
		return &webannotation.Annotation{Perms: perms}, nil
	}

	// 每个环境固定查询三个操作，鉴权结果按下标与入参对齐。
	actions := []meta.Action{meta.View, meta.Update, meta.Delete}
	authRes := make([]*meta.ResourceAttribute, 0, len(envs)*len(actions))
	for _, env := range envs {
		attachment := env.GetAttachment()
		for _, action := range actions {
			authRes = append(authRes, &meta.ResourceAttribute{
				Basic:     meta.Basic{Type: meta.Environment, Action: action, ResourceID: env.GetId()},
				BizID:     attachment.GetBizId(),
				ProjectID: attachment.GetProjectId(),
			})
		}
	}

	decisions, _, err := authorizer.AuthorizeDecision(kt, authRes...)
	if err != nil {
		return nil, err
	}

	for idx, res := range authRes {
		if idx >= len(decisions) || decisions[idx] == nil {
			continue
		}

		id := strconv.Itoa(int(res.ResourceID))
		if _, exist := perms[id]; !exist {
			perms[id] = webannotation.Perm{}
		}
		perms[id][string(res.Action)] = decisions[idx].Authorized
	}

	return &webannotation.Annotation{Perms: perms}, nil
}

// UpdateEnvironment implements [pbcs.ConfigServer].
func (s *Service) UpdateEnvironment(ctx context.Context, req *pbcs.UpdateEnvironmentReq) (*pbcs.UpdateEnvironmentResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{
			Basic:     meta.Basic{Type: meta.Environment, Action: meta.Update, ResourceID: req.EnvId},
			BizID:     req.BizId,
			ProjectID: req.ProjectId,
		},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	_, err := s.client.DS.UpdateEnvironment(grpcKit.RpcCtx(), &pbds.UpdateEnvironmentReq{
		BizId:     req.GetBizId(),
		ProjectId: req.GetProjectId(),
		EnvId:     req.GetEnvId(),
		Memo:      req.GetMemo(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.UpdateEnvironmentResp{}, nil
}

// GetEnvironmentByName implements [pbcs.ConfigServer].
// 按名称查询时环境 ID 还未确定：先用项目查看权限把查询限定在项目内，
// 解析出环境后再按环境实例校验查看权限，避免以"按名查询"为入口绕过环境级鉴权。
func (s *Service) GetEnvironmentByName(ctx context.Context, req *pbcs.GetEnvironmentByNameReq) (*pbcs.GetEnvironmentResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{
			Basic: meta.Basic{Type: meta.Project, Action: meta.View, ResourceID: req.ProjectId},
			BizID: req.BizId,
		},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.GetEnvironmentByName(grpcKit.RpcCtx(), &pbds.GetEnvironmentByNameReq{
		BizId:     req.GetBizId(),
		ProjectId: req.GetProjectId(),
		EnvName:   req.GetEnvName(),
	})
	if err != nil {
		return nil, err
	}

	res = []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{
			Basic:     meta.Basic{Type: meta.Environment, Action: meta.View, ResourceID: resp.GetId()},
			BizID:     req.BizId,
			ProjectID: req.ProjectId,
		},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	return &pbcs.GetEnvironmentResp{
		Id:         resp.GetId(),
		Spec:       resp.GetSpec(),
		Attachment: resp.GetAttachment(),
	}, nil
}
