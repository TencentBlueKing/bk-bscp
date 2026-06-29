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
	pbenvironment "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/environment"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// CreateEnvironment implements [pbds.DataServer].
func (s *Service) CreateEnvironment(ctx context.Context, req *pbds.CreateEnvironmentReq) (*pbds.CreateResp, error) {
	kt := kit.FromGrpcContext(ctx)

	environment, err := s.dao.Environment().GetByName(kt, req.GetBizId(), req.GetProjectId(), req.GetName())
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errf.Errorf(errf.DBOpFailed, "%s: %v", i18n.T(kt, "get environment by name failed"), err)
	}

	if environment != nil && environment.Spec.Name == req.GetName() {
		return nil, errf.Errorf(errf.InvalidParameter, "%s", i18n.T(kt, "environment name %s already exists", req.GetName()))
	}

	id, err := s.dao.Environment().Create(kt, &table.Environment{
		Spec: &table.EnvironmentSpec{
			Name: req.GetName(),
			Type: table.EnvironmentType(req.GetType()),
			Memo: req.GetMemo(),
		},
		Attachment: &table.EnvironmentAttachment{
			TenantID:  kt.TenantID,
			BizID:     req.GetBizId(),
			ProjectID: req.GetProjectId(),
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

// DeleteEnvironment implements [pbds.DataServer].
func (s *Service) DeleteEnvironment(ctx context.Context, req *pbds.DeleteEnvironmentReq) (*pbbase.EmptyResp, error) {
	kt := kit.FromGrpcContext(ctx)

	environment, err := s.dao.Environment().Get(kt, req.GetBizId(), req.GetProjectId(), req.GetEnvId())
	if err != nil {
		return nil, err
	}

	// 受保护的和系统内置的不允许删除
	if environment.Spec.Protected || environment.Revision.Creator == table.System {
		return nil, errors.New(i18n.T(kt, "environment is protected, cannot be deleted"))
	}

	// 3. 如果是生产环境(Prod)下的默认环境(Default)，不允许删除
	if environment.Spec.Type == table.EnvironmentTypeProd && environment.Spec.Name == table.DefaultEnvName {
		return nil, errors.New(i18n.T(kt, "the default environment under the production space cannot be deleted"))
	}

	// 检查是否有关联的服务（属于“级联依赖”导致的无法删除）
	appCount, err := s.dao.App().CountByEnvID(kt, req.GetEnvId())
	if err != nil {
		return nil, err
	}

	if appCount > 0 {
		return nil, errors.New(i18n.T(kt, "there are still app under the environments, please delete the app first"))
	}

	if err = s.dao.Environment().Delete(kt, environment); err != nil {
		return nil, err
	}

	return &pbbase.EmptyResp{}, nil
}

// GetEnvironment implements [pbds.DataServer].
func (s *Service) GetEnvironment(ctx context.Context, req *pbds.GetEnvironmentReq) (*pbds.GetEnvironmentResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 1. 查询环境详情
	environment, err := s.dao.Environment().Get(kt, req.GetBizId(), req.GetProjectId(), req.GetEnvId())
	if err != nil {
		return nil, err
	}

	// 2. 直接查询该环境下的服务总数
	appCount, err := s.dao.App().CountByEnvID(kt, req.GetEnvId())
	if err != nil {
		return nil, err
	}

	return &pbds.GetEnvironmentResp{
		Id:         environment.ID,
		Spec:       pbenvironment.PbEnvironmentSpec(environment.Spec, uint32(appCount)),
		Attachment: pbenvironment.PbEnvironmentAttachment(environment.Attachment),
	}, nil
}

// ListEnvironments implements [pbds.DataServer].
func (s *Service) ListEnvironments(ctx context.Context, req *pbds.ListEnvironmentsReq) (*pbds.ListEnvironmentsResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 1. 分页查询环境列表
	environments, _, err := s.dao.Environment().List(kt, req.GetBizId(), req.GetProjectId(), &types.BasePage{
		Start:  req.GetStart(),
		Limit:  uint(req.GetLimit()),
		All:    req.GetAll(),
		Search: req.GetSearchCondition(),
	})
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, "%s: %v", i18n.T(kt, "environment list failed"), err)
	}

	// 2. 收集当前页所有的环境 ID
	envIDs := make([]uint32, 0, len(environments))
	for _, env := range environments {
		envIDs = append(envIDs, env.ID)
	}

	// 3. 批量统计每个环境下的服务数量
	appCounts, err := s.dao.App().CountByEnvIDs(kt, envIDs)
	if err != nil {
		return nil, err
	}

	// 4. 初始化四个分组切片，防止返回 nil
	prodEnvs := make([]*pbenvironment.Environment, 0)
	stagingEnvs := make([]*pbenvironment.Environment, 0)
	testEnvs := make([]*pbenvironment.Environment, 0)
	devEnvs := make([]*pbenvironment.Environment, 0)

	// 5. 循环装配并按 type 分组
	for _, env := range environments {
		// 获取当前环境的服务总数
		appCount := appCounts[env.ID]

		// 转换为 pb 结构体
		pbEnv := &pbenvironment.Environment{
			Id:         env.ID,
			Spec:       pbenvironment.PbEnvironmentSpec(env.Spec, appCount),
			Attachment: pbenvironment.PbEnvironmentAttachment(env.Attachment),
			Revision:   pbbase.PbRevision(env.Revision),
		}

		// 根据 type 分流到不同的切片中
		switch env.Spec.Type {
		case table.EnvironmentTypeProd:
			prodEnvs = append(prodEnvs, pbEnv)
		case table.EnvironmentTypeStaging:
			stagingEnvs = append(stagingEnvs, pbEnv)
		case table.EnvironmentTypeTest:
			testEnvs = append(testEnvs, pbEnv)
		case table.EnvironmentTypeDev:
			devEnvs = append(devEnvs, pbEnv)
		default:
			// 如果有未知的类型,默认放进开发环境中
			devEnvs = append(devEnvs, pbEnv)
		}
	}

	return &pbds.ListEnvironmentsResp{
		ProdEnvironments:    prodEnvs,
		StagingEnvironments: stagingEnvs,
		TestEnvironments:    testEnvs,
		DevEnvironments:     devEnvs,
	}, nil
}

// UpdateEnvironment implements [pbds.DataServer].
func (s *Service) UpdateEnvironment(ctx context.Context, req *pbds.UpdateEnvironmentReq) (*pbbase.EmptyResp, error) {
	kt := kit.FromGrpcContext(ctx)

	environment, err := s.dao.Environment().Get(kt, req.GetBizId(), req.GetProjectId(), req.GetEnvId())
	if err != nil {
		return nil, err
	}

	environment.Spec.Memo = req.GetMemo()
	environment.Revision.Reviser = kt.User
	environment.Revision.UpdatedAt = time.Now().UTC()

	if err = s.dao.Environment().Update(kt, environment); err != nil {
		return nil, err
	}

	return &pbbase.EmptyResp{}, nil
}
