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
	"errors"
	"time"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbbase "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
	pbproject "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/project"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// CreateProject implements [pbds.DataServer].
func (s *Service) CreateProject(ctx context.Context, req *pbds.CreateProjectReq) (*pbds.CreateResp, error) {
	kt := kit.FromGrpcContext(ctx)

	id, err := s.dao.Project().Create(kt, &table.Project{
		Spec: &table.ProjectSpec{
			Name: req.GetName(),
			Memo: req.GetMemo(),
		},
		Attachment: &table.ProjectAttachment{
			TenantID: kt.TenantID,
			BizID:    req.GetBizId(),
		},
		Revision: &table.Revision{
			Creator:   kt.User,
			CreatedAt: time.Now().UTC(),
		},
	})
	if err != nil {
		return nil, err
	}

	return &pbds.CreateResp{Id: id}, nil
}

// DeleteProject implements [pbds.DataServer].
func (s *Service) DeleteProject(ctx context.Context, req *pbds.DeleteProjectReq) (*pbbase.EmptyResp, error) {
	kt := kit.FromGrpcContext(ctx)

	project, err := s.dao.Project().Get(kt, req.GetBizId(), req.GetProjectId())
	if err != nil {
		return nil, err
	}

	// 受保护的和系统内置的不允许删除
	if project.Spec.Protected || project.Revision.Creator == table.System {
		return nil, errors.New(i18n.T(kt, "project is protected, cannot be deleted"))
	}

	// 检查是否有关联的环境（属于“级联依赖”导致的无法删除）
	envCount, err := s.dao.Environment().CountByProjectID(kt, req.GetProjectId())
	if err != nil {
		return nil, err
	}

	if envCount > 0 {
		return nil, errors.New(i18n.T(kt, "there are still environments under the project, please delete the environments first"))
	}

	if err = s.dao.Project().Delete(kt, project); err != nil {
		return nil, err
	}

	return &pbbase.EmptyResp{}, nil
}

// GetProject implements [pbds.DataServer].
func (s *Service) GetProject(ctx context.Context, req *pbds.GetProjectReq) (*pbds.GetProjectResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 1. 查询项目详情
	project, err := s.dao.Project().Get(kt, req.GetBizId(), req.GetProjectId())
	if err != nil {
		return nil, err
	}

	// 2. 直接查环境数量
	envCount, err := s.dao.Environment().CountByProjectID(kt, req.GetProjectId())
	if err != nil {
		return nil, err
	}

	// 3. 直接查服务数量
	appCount, err := s.dao.App().CountByProjectID(kt, req.GetProjectId())
	if err != nil {
		return nil, err
	}

	return &pbds.GetProjectResp{
		Id:         project.ID,
		Spec:       pbproject.PbProjectSpec(project.Spec, uint32(envCount), uint32(appCount)),
		Attachment: pbproject.PbProjectAttachment(project.Attachment),
	}, nil
}

// ListProjects implements [pbds.DataServer].
func (s *Service) ListProjects(ctx context.Context, req *pbds.ListProjectsReq) (*pbds.ListProjectsResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 1. 分页查询项目列表
	projects, count, err := s.dao.Project().List(kt, req.GetBizId(), &types.BasePage{
		Start:  req.GetStart(),
		Limit:  uint(req.GetLimit()),
		All:    req.GetAll(),
		Search: req.GetSearchCondition(),
	})
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, "%s: %v", i18n.T(kt, "project list failed"), err)
	}

	// 收集当前页所有的 Project ID
	projectIDs := make([]uint32, 0, len(projects))
	for _, v := range projects {
		projectIDs = append(projectIDs, v.ID)
	}

	// 2. 批量统计环境数量
	envCounts, err := s.dao.Environment().CountByProjectIDs(kt, projectIDs)
	if err != nil {
		return nil, err
	}

	// 3. 批量统计服务数量
	appCounts, err := s.dao.App().CountByProjectIDs(kt, projectIDs)
	if err != nil {
		return nil, err
	}

	return &pbds.ListProjectsResp{
		Count:    uint32(count),
		Projects: pbproject.PbProjects(projects, envCounts, appCounts),
	}, nil
}

// UpdateProject implements [pbds.DataServer].
func (s *Service) UpdateProject(ctx context.Context, req *pbds.UpdateProjectReq) (*pbbase.EmptyResp, error) {
	kt := kit.FromGrpcContext(ctx)

	project, err := s.dao.Project().Get(kt, req.GetBizId(), req.GetProjectId())
	if err != nil {
		return nil, err
	}

	if req.GetName() != "" {
		project.Spec.Name = req.GetName()
	}
	if req.GetMemo() != "" {
		project.Spec.Memo = req.GetMemo()
	}

	project.Revision.Reviser = kt.User
	project.Revision.UpdatedAt = time.Now().UTC()

	if err = s.dao.Project().Update(kt, project); err != nil {
		return nil, err
	}

	return &pbbase.EmptyResp{}, nil
}
