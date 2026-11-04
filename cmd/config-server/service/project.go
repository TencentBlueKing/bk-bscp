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
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// CreateProject implements [pbcs.ConfigServer].
func (s *Service) CreateProject(ctx context.Context, req *pbcs.CreateProjectReq) (*pbcs.CreateProjectResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Project, Action: meta.Create}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.CreateProject(grpcKit.RpcCtx(), &pbds.CreateProjectReq{
		BizId: req.GetBizId(),
		Name:  req.GetName(),
		Memo:  req.GetMemo(),
	})

	if err != nil {
		return nil, err
	}

	return &pbcs.CreateProjectResp{
		Id: resp.GetId(),
	}, nil
}

// DeleteProject implements [pbcs.ConfigServer].
func (s *Service) DeleteProject(ctx context.Context, req *pbcs.DeleteProjectReq) (*pbcs.DeleteProjectResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Project, Action: meta.Delete, ResourceID: req.ProjectId}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	_, err := s.client.DS.DeleteProject(grpcKit.RpcCtx(), &pbds.DeleteProjectReq{
		BizId:     req.GetBizId(),
		ProjectId: req.GetProjectId(),
	})

	if err != nil {
		return nil, err
	}

	return &pbcs.DeleteProjectResp{}, nil
}

// GetProject implements [pbcs.ConfigServer].
func (s *Service) GetProject(ctx context.Context, req *pbcs.GetProjectReq) (*pbcs.GetProjectResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Project, Action: meta.View, ResourceID: req.ProjectId}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.GetProject(grpcKit.RpcCtx(), &pbds.GetProjectReq{
		BizId:     req.GetBizId(),
		ProjectId: req.GetProjectId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.GetProjectResp{
		Id:         resp.GetId(),
		Spec:       resp.GetSpec(),
		Attachment: resp.GetAttachment(),
	}, nil
}

// ListProjects implements [pbcs.ConfigServer].
func (s *Service) ListProjects(ctx context.Context, req *pbcs.ListProjectsReq) (*pbcs.ListProjectsResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Project, Action: meta.Find}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.ListProjects(grpcKit.RpcCtx(), &pbds.ListProjectsReq{
		BizId:           req.GetBizId(),
		Start:           req.GetStart(),
		Limit:           req.GetLimit(),
		All:             req.GetAll(),
		SearchCondition: req.GetSearchCondition(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ListProjectsResp{
		Count:    resp.GetCount(),
		Projects: resp.GetProjects(),
	}, nil
}

// ListProjectsAnnotation 项目列表的逐项权限注解。
// 列表本身按业务放行，无法逐项拦截；把每个项目的查看/编辑/删除判定带回前端后，
// 前端可以把无权限项目置灰，并在点击时引导申请项目权限，而不是跳过去再报 403。
func ListProjectsAnnotation(ctx context.Context, kt *kit.Kit,
	authorizer iamauth.Authorizer, msg proto.Message) (*webannotation.Annotation, error) {

	resp, ok := msg.(*pbcs.ListProjectsResp)
	if !ok {
		return nil, nil
	}

	perms := make(map[string]webannotation.Perm, len(resp.GetProjects()))
	if len(resp.GetProjects()) == 0 {
		return &webannotation.Annotation{Perms: perms}, nil
	}

	// 每个项目固定查询三个操作，鉴权结果按下标与入参对齐。
	actions := []meta.Action{meta.View, meta.Update, meta.Delete}
	authRes := make([]*meta.ResourceAttribute, 0, len(resp.GetProjects())*len(actions))
	for _, project := range resp.GetProjects() {
		for _, action := range actions {
			authRes = append(authRes, &meta.ResourceAttribute{
				Basic: meta.Basic{Type: meta.Project, Action: action, ResourceID: project.GetId()},
				BizID: project.GetAttachment().GetBizId(),
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

// UpdateProject implements [pbcs.ConfigServer].
func (s *Service) UpdateProject(ctx context.Context, req *pbcs.UpdateProjectReq) (*pbcs.UpdateProjectResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Project, Action: meta.Update, ResourceID: req.ProjectId}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	_, err := s.client.DS.UpdateProject(grpcKit.RpcCtx(), &pbds.UpdateProjectReq{
		BizId:     req.GetBizId(),
		ProjectId: req.GetProjectId(),
		Memo:      req.GetMemo(),
		Name:      req.GetName(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.UpdateProjectResp{}, nil
}

// EnsureDefaultProjectEnv implements [pbcs.ConfigServer].
func (s *Service) EnsureDefaultProjectEnv(ctx context.Context, req *pbcs.EnsureDefaultProjectEnvReq) (*pbcs.EnsureDefaultProjectEnvResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.EnsureDefaultProjectEnv(grpcKit.RpcCtx(), &pbds.EnsureDefaultProjectEnvReq{
		BizId: req.GetBizId(),
	})

	if err != nil {
		return nil, err
	}

	return &pbcs.EnsureDefaultProjectEnvResp{
		ProjectId: resp.GetProjectId(),
		EnvId:     resp.GetEnvId(),
	}, nil
}

// GetProjectByKey implements [pbcs.ConfigServer].
// 按标识查询时项目 ID 尚未确定，只能按业务访问放行；查询结果本身不含敏感配置。
func (s *Service) GetProjectByKey(ctx context.Context, req *pbcs.GetProjectByKeyReq) (*pbcs.GetProjectResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Project, Action: meta.Find}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.GetProjectByKey(grpcKit.RpcCtx(), &pbds.GetProjectByKeyReq{
		BizId:      req.GetBizId(),
		ProjectKey: req.GetProjectKey(),
	})

	if err != nil {
		return nil, err
	}

	return &pbcs.GetProjectResp{
		Id:         resp.GetId(),
		Spec:       resp.GetSpec(),
		Attachment: resp.GetAttachment(),
	}, nil
}
