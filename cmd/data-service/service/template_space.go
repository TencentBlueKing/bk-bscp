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
	"fmt"

	"github.com/TencentBlueKing/bk-bscp/internal/search"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbbase "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
	pbtr "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/template-revision"
	pbts "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/template-space"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// CreateTemplateSpace create template space.
func (s *Service) CreateTemplateSpace(ctx context.Context, req *pbds.CreateTemplateSpaceReq) (*pbds.CreateResp, error) {
	kt := kit.FromGrpcContext(ctx)

	if _, err := s.dao.TemplateSpace().GetByUniqueKey(kt, req.Attachment.BizId, req.Spec.Name); err == nil {
		return nil, fmt.Errorf("template space's same name %s already exists", req.Spec.Name)
	}

	templateSpace := &table.TemplateSpace{
		Spec:       req.Spec.TemplateSpaceSpec(),
		Attachment: req.Attachment.TemplateSpaceAttachment(),
		Revision: &table.Revision{
			Creator: kt.User,
			Reviser: kt.User,
		},
	}
	id, err := s.dao.TemplateSpace().Create(kt, templateSpace)
	if err != nil {
		logs.Errorf("create template space failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	resp := &pbds.CreateResp{Id: id}
	return resp, nil
}

// ListTemplateSpaces list template space.
func (s *Service) ListTemplateSpaces(ctx context.Context,
	req *pbds.ListTemplateSpacesReq) (*pbds.ListTemplateSpacesResp, error) {

	kt := kit.FromGrpcContext(ctx)

	opt := &types.BasePage{Start: req.Start, Limit: uint(req.Limit), All: req.All}
	if err := opt.Validate(types.DefaultPageOption); err != nil {
		return nil, err
	}

	searcher, err := search.NewSearcher(req.SearchFields, req.SearchValue, search.TemplateSpace)
	if err != nil {
		return nil, err
	}

	details, count, err := s.dao.TemplateSpace().List(kt, req.BizId, searcher, opt)
	if err != nil {
		logs.Errorf("list template spaces failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	resp := &pbds.ListTemplateSpacesResp{
		Count:   uint32(count),
		Details: pbts.PbTemplateSpaces(details),
	}
	return resp, nil
}

// UpdateTemplateSpace update template space.
func (s *Service) UpdateTemplateSpace(ctx context.Context,
	req *pbds.UpdateTemplateSpaceReq) (*pbbase.EmptyResp, error) {

	kt := kit.FromGrpcContext(ctx)

	templateSpace := &table.TemplateSpace{
		ID:         req.Id,
		Spec:       req.Spec.TemplateSpaceSpec(),
		Attachment: req.Attachment.TemplateSpaceAttachment(),
		Revision: &table.Revision{
			Reviser: kt.User,
		},
	}
	if err := s.dao.TemplateSpace().Update(kt, templateSpace); err != nil {
		logs.Errorf("update template space failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return new(pbbase.EmptyResp), nil
}

// DeleteTemplateSpace delete template space.
func (s *Service) DeleteTemplateSpace(ctx context.Context,
	req *pbds.DeleteTemplateSpaceReq) (*pbbase.EmptyResp, error) {

	kt := kit.FromGrpcContext(ctx)

	if err := s.dao.Validator().ValidateTmplSpaceNoSubRes(kt, req.Id); err != nil {
		return nil, err
	}

	templateSpace := &table.TemplateSpace{
		ID:         req.Id,
		Attachment: req.Attachment.TemplateSpaceAttachment(),
	}
	if err := s.dao.TemplateSpace().Delete(kt, templateSpace); err != nil {
		logs.Errorf("delete template space failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return new(pbbase.EmptyResp), nil
}

// GetAllBizsOfTmplSpaces get all biz ids of template spaces
func (s *Service) GetAllBizsOfTmplSpaces(ctx context.Context, req *pbbase.EmptyReq) (
	*pbds.GetAllBizsOfTmplSpacesResp, error) {
	kt := kit.FromGrpcContext(ctx)

	bizIDs, err := s.dao.TemplateSpace().GetAllBizs(kt)
	if err != nil {
		logs.Errorf("get all bizs of template space failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	resp := &pbds.GetAllBizsOfTmplSpacesResp{BizIds: bizIDs}
	return resp, nil
}

// CreateDefaultTmplSpace create default template space
func (s *Service) CreateDefaultTmplSpace(ctx context.Context, req *pbds.CreateDefaultTmplSpaceReq) (
	*pbds.CreateResp, error) {
	kt := kit.FromGrpcContext(ctx)

	id, err := s.dao.TemplateSpace().CreateDefault(kt, req.BizId)
	if err != nil {
		logs.Errorf("create default template space failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	resp := &pbds.CreateResp{Id: id}
	return resp, nil
}

// ListTmplSpacesByIDs list template space by ids.
func (s *Service) ListTmplSpacesByIDs(ctx context.Context, req *pbds.ListTmplSpacesByIDsReq) (*pbds.
	ListTmplSpacesByIDsResp, error) {
	kt := kit.FromGrpcContext(ctx)

	if err := s.dao.Validator().ValidateTmplSpacesExist(kt, req.Ids); err != nil {
		return nil, err
	}

	details, err := s.dao.TemplateSpace().ListByIDs(kt, req.Ids)
	if err != nil {
		logs.Errorf("list template spaces failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	resp := &pbds.ListTmplSpacesByIDsResp{
		Details: pbts.PbTemplateSpaces(details),
	}
	return resp, nil
}

// GetLatestTemplateVersionsInSpace implements pbds.DataServer.
func (s *Service) GetLatestTemplateVersionsInSpace(ctx context.Context, req *pbds.GetLatestTemplateVersionsInSpaceReq) (
	*pbds.GetLatestTemplateVersionsInSpaceResp, error) {
	kit := kit.FromGrpcContext(ctx)

	// 1. 获取空间名
	templateSpace, err := s.dao.TemplateSpace().Get(kit, req.BizId, req.TemplateSpaceId)
	if err != nil {
		return nil, err
	}
	templateSets := make([]*table.TemplateSet, 0)
	// 2. 模板套餐不是0表示获取某个套餐
	if req.TemplateId != 0 {
		templateSet, errT := s.dao.TemplateSet().GetByTemplateSetByID(kit, req.BizId, req.TemplateId)
		if errT != nil {
			return nil, errT
		}
		templateSets = append(templateSets, templateSet)
	} else {
		// 获取空间下的所有套餐
		templateSets, _, err = s.dao.TemplateSet().List(kit, req.BizId, req.TemplateSpaceId, nil, &types.BasePage{All: true})
		if err != nil {
			return nil, err
		}
	}

	// 3. 通过套餐获取模板
	templateIds := []uint32{}
	for _, v := range templateSets {
		templateIds = append(templateIds, v.Spec.TemplateIDs...)
	}

	// 去重
	templateIds = tools.RemoveDuplicates(templateIds)

	// 4. 获取最新的模板文件
	templateRevision, err := s.dao.TemplateRevision().ListLatestRevisionsGroupByTemplateIds(kit, templateIds)
	if err != nil {
		return nil, err
	}

	templateRevisionMap := make(map[uint32]*table.TemplateRevision, 0)

	for _, v := range templateRevision {
		templateRevisionMap[v.Attachment.TemplateID] = v
	}

	items := make([]*pbds.GetLatestTemplateVersionsInSpaceResp_TemplateSetSpec, 0)
	for _, set := range templateSets {
		revisions := make([]*pbtr.TemplateRevisionSpec, 0)

		for _, v := range set.Spec.TemplateIDs {
			revision := templateRevisionMap[v]
			revisions = append(revisions, pbtr.PbTemplateRevision(revision, "").Spec)
		}
		items = append(items, &pbds.GetLatestTemplateVersionsInSpaceResp_TemplateSetSpec{
			Name:             set.Spec.Name,
			TemplateRevision: revisions,
		})
	}

	return &pbds.GetLatestTemplateVersionsInSpaceResp{
		TemplateSpace: &pbts.TemplateSpaceSpec{
			Name: templateSpace.Spec.Name,
			Memo: templateSpace.Spec.Memo,
		},
		TemplateSet: items,
	}, nil
}
