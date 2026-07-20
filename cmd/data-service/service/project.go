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
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbbase "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
	pbenvironment "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/environment"
	pbproject "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/project"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// EnsureDefaultProjectEnv implements [pbds.ConfigServer].
func (s *Service) EnsureDefaultProjectEnv(ctx context.Context, req *pbds.EnsureDefaultProjectEnvReq) (
	*pbds.EnsureDefaultProjectEnvResp, error) {
	kt := kit.FromGrpcContext(ctx)
	bizID := req.GetBizId()
	if bizID == 0 {
		return nil, errors.New(i18n.T(kt, "invalid biz_id"))
	}

	var projectID, envID uint32
	// 1. 尝试获取已存在的默认项目
	project, err := s.dao.Project().GetDefaultProject(kt, bizID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 2. 尝试获取已存在的默认环境
	var env *table.Environment
	if project != nil {
		projectID = project.ID
		env, err = s.dao.Environment().GetDefaultEnvironment(kt, bizID, projectID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if env != nil {
			envID = env.ID
		}
	}

	// 3. 如果项目和环境都已经存在，直接返回
	if project != nil && env != nil {
		return &pbds.EnsureDefaultProjectEnvResp{
			ProjectId: projectID,
			EnvId:     envID,
		}, nil
	}

	// 4. 开启事务进行创建（按需创建项目和/或环境）
	tx := s.dao.GenQuery().Begin()
	committed := false
	defer func() {
		if !committed {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kt.Rid)
			}
		}
	}()

	createdAt := time.Now().UTC()
	// 4.1 如果项目不存在，创建项目
	if project == nil {
		newProject := &table.Project{
			Spec: &table.ProjectSpec{
				Name:      table.DefaultProjectName,
				Protected: true,
				IsDefault: sql.NullBool{
					Bool:  true,
					Valid: true,
				},
			},
			Attachment: &table.ProjectAttachment{
				TenantID: kt.TenantID,
				BizID:    bizID,
			},
			Revision: &table.Revision{Creator: table.System, CreatedAt: createdAt},
		}

		err = s.dao.Project().CreateIfNotExistWithTx(kt, tx, newProject)
		if err != nil {
			return nil, fmt.Errorf("create default project failed: %w", err)
		}

		projectID = newProject.ID
		existingProj, qErr := s.dao.Project().GetDefaultProject(kt, bizID)
		if qErr != nil && !errors.Is(qErr, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("query default project after create failed: %w", qErr)
		}
		if existingProj != nil {
			projectID = existingProj.ID
		}
	}

	// 4.2 如果环境不存在，创建环境
	if env == nil {
		newEnv := &table.Environment{
			Spec: &table.EnvironmentSpec{
				Name:      table.DefaultEnvName,
				Type:      table.EnvironmentTypeProd,
				Protected: true,
			},
			Attachment: &table.EnvironmentAttachment{
				TenantID:  kt.TenantID,
				BizID:     bizID,
				ProjectID: projectID,
			},
			Revision: &table.Revision{Creator: table.System, CreatedAt: createdAt},
		}
		err = s.dao.Environment().CreateIfNotExistWithTx(kt, tx, newEnv)
		if err != nil {
			return nil, fmt.Errorf("ensure default env failed: %w", err)
		}

		envID = newEnv.ID
		existingEnv, qErr := s.dao.Environment().GetDefaultEnvironment(kt, bizID, projectID)
		if qErr != nil && !errors.Is(qErr, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("query default env after create failed: %w", qErr)
		}
		if existingEnv != nil {
			envID = existingEnv.ID
		}
	}

	// 5. 提交事务
	if e := tx.Commit(); e != nil {
		logs.Errorf("commit transaction failed, err: %v, rid: %s", e, kt.Rid)
		return nil, e
	}
	committed = true

	// 6. 返回结果
	return &pbds.EnsureDefaultProjectEnvResp{
		ProjectId: projectID,
		EnvId:     envID,
	}, nil
}

// GetDefaultEnvironment 根据业务和项目查询默认环境完整信息。
func (s *Service) GetDefaultEnvironment(ctx context.Context, req *pbds.GetDefaultEnvironmentReq) (
	*pbds.GetDefaultEnvironmentResp, error) {
	kt := kit.FromGrpcContext(ctx)
	bizID := req.GetBizId()
	projectID := req.GetProjectId()

	env, err := s.dao.Environment().GetDefaultEnvironment(kt, bizID, projectID)
	if err != nil {
		logs.Errorf("get default environment failed, bizId=%d projectId=%d err=%v rid:%s", bizID, projectID, err, kt.Rid)
		return nil, err
	}
	if env == nil {
		return &pbds.GetDefaultEnvironmentResp{}, nil
	}

	return &pbds.GetDefaultEnvironmentResp{
		Id:         env.ID,
		Spec:       pbenvironment.PbEnvironmentSpec(env.Spec, 0),
		Attachment: pbenvironment.PbEnvironmentAttachment(env.Attachment),
	}, nil
}

// CreateProject implements [pbds.DataServer].
func (s *Service) CreateProject(ctx context.Context, req *pbds.CreateProjectReq) (*pbds.CreateResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 1. 开启事物
	tx := s.dao.GenQuery().Begin()
	committed := false
	defer func() {
		if !committed {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kt.Rid)
			}
		}
	}()

	// 2. 在事务中创建项目
	projectID, err := s.dao.Project().CreateWithTx(kt, tx, &table.Project{
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

	// 3. 在同一事务中自动创建默认生产环境
	_, err = s.dao.Environment().CreateWithTx(kt, tx, &table.Environment{
		Spec: &table.EnvironmentSpec{
			Name: table.DefaultEnvName,
			Type: table.EnvironmentTypeProd,
			Memo: "Default production environment automatically created.",
		},
		Attachment: &table.EnvironmentAttachment{
			TenantID:  kt.TenantID,
			BizID:     req.GetBizId(),
			ProjectID: projectID,
		},
		Revision: &table.Revision{
			Creator:   table.System, // 系统自动创建，不记录创建人
			CreatedAt: time.Now().UTC(),
		},
	})
	if err != nil {
		return nil, err
	}

	// 4. 提交事务
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true

	return &pbds.CreateResp{Id: projectID}, nil
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
	envs, _, err := s.dao.Environment().List(kt, req.GetBizId(), req.GetProjectId(), &types.BasePage{All: true})
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, "%s: %v", i18n.T(kt, "environment list failed"), err)
	}
	envIDs := []uint32{}
	for _, env := range envs {
		envIDs = append(envIDs, env.ID)
	}

	appCounts, err := s.dao.App().CountByEnvIDs(kt, envIDs)
	if err != nil {
		return nil, err
	}

	if len(appCounts) > 0 {
		return nil, errors.New(i18n.T(kt, "there are still active resources in the environments under this project, cannot be deleted"))
	}

	// 开启事物
	tx := s.dao.GenQuery().Begin()
	committed := false
	defer func() {
		if !committed {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kt.Rid)
			}
		}
	}()

	if err = s.dao.Project().DeleteWithTx(kt, tx, project); err != nil {
		return nil, err
	}

	if err = s.dao.Environment().DeleteByProjectIDWithTx(kt, tx, req.GetBizId(), req.GetProjectId()); err != nil {
		return nil, err
	}

	if e := tx.Commit(); e != nil {
		logs.Errorf("commit transaction failed, err: %v, rid: %s", e, kt.Rid)
		return nil, e
	}
	committed = true

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
