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

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
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
	// 如果上面 project 没找到，这里 project.ID 是 0，必然也找不到环境，属于正常现象
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
			Revision: &table.Revision{
				Creator:   table.System,
				CreatedAt: createdAt,
			},
		}

		err = s.dao.Project().CreateIfNotExistWithTx(kt, tx, newProject)
		if err != nil {
			return nil, fmt.Errorf("create default project failed: %w", err)
		}

		// OnConflict 时新分配的 ID 不会回填为已存在的真实主键，
		// 必须在同一事务内重新查询以获取正确的 ID。
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
			Revision: &table.Revision{
				Creator:   table.System,
				CreatedAt: createdAt,
			},
		}
		err = s.dao.Environment().CreateIfNotExistWithTx(kt, tx, newEnv)
		if err != nil {
			return nil, fmt.Errorf("ensure default env failed: %w", err)
		}

		// OnConflict 时新分配的 ID 不会回填为已存在的真实主键，
		// 必须在同一事务内重新查询以获取正确的 ID。
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
