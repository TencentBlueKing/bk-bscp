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
		return nil, err
	}
	if environment != nil && environment.Spec.Name == req.GetName() {
		return nil, errf.Errorf(errf.InvalidParameter, "%s", i18n.T(kt, "environment name %s already exists", req.GetName()))
	}

	id, err := s.dao.Environment().Create(kt, &table.Environment{
		Spec: &table.EnvironmentSpec{
			Name:      req.GetName(),
			Type:      table.EnvironmentType(req.GetType()),
			Memo:      req.GetMemo(),
			Protected: req.GetProtected(),
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

	if environment.Spec.Protected {
		return nil, errors.New(i18n.T(kt, "environment is protected, cannot be deleted"))
	}

	if err = s.dao.Environment().Delete(kt, environment); err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kt, "environment deletion failed: %v", err))
	}

	return &pbbase.EmptyResp{}, nil
}

// GetEnvironment implements [pbds.DataServer].
func (s *Service) GetEnvironment(ctx context.Context, req *pbds.GetEnvironmentReq) (*pbds.GetEnvironmentResp, error) {
	kt := kit.FromGrpcContext(ctx)

	environment, err := s.dao.Environment().Get(kt, req.GetBizId(), req.GetProjectId(), req.GetEnvId())
	if err != nil {
		return nil, err
	}

	return &pbds.GetEnvironmentResp{
		Id:         environment.ID,
		Spec:       pbenvironment.PbEnvironmentSpec(environment.Spec),
		Attachment: pbenvironment.PbEnvironmentAttachment(environment.Attachment),
	}, nil
}

// ListEnvironments implements [pbds.DataServer].
func (s *Service) ListEnvironments(ctx context.Context, req *pbds.ListEnvironmentsReq) (*pbds.ListEnvironmentsResp, error) {
	kt := kit.FromGrpcContext(ctx)

	environments, count, err := s.dao.Environment().List(kt, req.GetBizId(), req.GetProjectId(), &types.BasePage{
		Start:  req.GetStart(),
		Limit:  uint(req.GetLimit()),
		All:    req.GetAll(),
		Search: req.GetSearchCondition(),
	})
	if err != nil {
		return nil, err
	}

	return &pbds.ListEnvironmentsResp{
		Count:        uint32(count),
		Environments: pbenvironment.PbEnvironments(environments),
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
	environment.Spec.Protected = req.GetProtected()
	environment.Revision.Reviser = kt.User
	environment.Revision.UpdatedAt = time.Now().UTC()

	if err = s.dao.Environment().Update(kt, environment); err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kt, "environment update failed: %v", err))
	}

	return &pbbase.EmptyResp{}, nil
}
