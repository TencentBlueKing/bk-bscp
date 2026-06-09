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

	"gorm.io/gorm"

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

	if req.GetKey() != "" {
		project, err := s.dao.Project().GetByKey(kt, req.GetBizId(), req.GetKey())
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if project != nil && project.Spec.Key == req.GetKey() {
			return nil, errf.Errorf(errf.InvalidParameter, "%s", i18n.T(kt, "project key %s already exists", req.GetKey()))
		}
	}

	id, err := s.dao.Project().Create(kt, &table.Project{
		Spec: &table.ProjectSpec{
			Name:      req.GetName(),
			Key:       req.GetKey(),
			Memo:      req.GetMemo(),
			Protected: req.GetProtected(),
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

	if project.Spec.Protected {
		return nil, errors.New(i18n.T(kt, "project is protected, cannot be deleted"))
	}

	if err = s.dao.Project().Delete(kt, project); err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kt, "project deletion failed: %v", err))
	}

	return &pbbase.EmptyResp{}, nil
}

// GetProject implements [pbds.DataServer].
func (s *Service) GetProject(ctx context.Context, req *pbds.GetProjectReq) (*pbds.GetProjectResp, error) {
	kt := kit.FromGrpcContext(ctx)

	project, err := s.dao.Project().Get(kt, req.GetBizId(), req.GetProjectId())
	if err != nil {
		return nil, err
	}

	return &pbds.GetProjectResp{
		Id:         project.ID,
		Spec:       pbproject.PbProjectSpec(project.Spec),
		Attachment: pbproject.PbProjectAttachment(project.Attachment),
	}, nil
}

// ListProjects implements [pbds.DataServer].
func (s *Service) ListProjects(ctx context.Context, req *pbds.ListProjectsReq) (*pbds.ListProjectsResp, error) {
	kt := kit.FromGrpcContext(ctx)

	projects, count, err := s.dao.Project().List(kt, req.GetBizId(), &types.BasePage{
		Start:  req.GetStart(),
		Limit:  uint(req.GetLimit()),
		All:    req.GetAll(),
		Search: req.GetSearchCondition(),
	})
	if err != nil {
		return nil, err
	}

	return &pbds.ListProjectsResp{
		Count:    uint32(count),
		Projects: pbproject.PbProjects(projects),
	}, nil
}

// UpdateProject implements [pbds.DataServer].
func (s *Service) UpdateProject(ctx context.Context, req *pbds.UpdateProjectReq) (*pbbase.EmptyResp, error) {
	kt := kit.FromGrpcContext(ctx)

	project, err := s.dao.Project().Get(kt, req.GetBizId(), req.GetProjectId())
	if err != nil {
		return nil, err
	}

	project.Spec.Memo = req.GetMemo()
	project.Spec.Protected = req.GetProtected()
	project.Revision.Reviser = kt.User
	project.Revision.UpdatedAt = time.Now().UTC()

	if err = s.dao.Project().Update(kt, project); err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kt, "project update failed: %v", err))
	}

	return &pbbase.EmptyResp{}, nil
}
