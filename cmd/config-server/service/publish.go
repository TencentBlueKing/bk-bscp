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
	"fmt"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/runtime/selector"
)

// Publish publish a strategy
func (s *Service) Publish(ctx context.Context, req *pbcs.PublishReq) (
	*pbcs.PublishResp, error) {

	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.App, Action: meta.Publish, ResourceID: req.AppId}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(grpcKit, res...)
	if err != nil {
		return nil, err
	}

	r := &pbds.PublishReq{
		BizId:           req.BizId,
		AppId:           req.AppId,
		ReleaseId:       req.ReleaseId,
		Memo:            req.Memo,
		All:             req.All,
		GrayPublishMode: req.GrayPublishMode,
		Default:         req.Default,
		Groups:          req.Groups,
		Labels:          req.Labels,
		GroupName:       req.GroupName,
	}
	rp, err := s.client.DS.Publish(grpcKit.RpcCtx(), r)
	if err != nil {
		logs.Errorf("publish failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	resp := &pbcs.PublishResp{
		Id:              rp.PublishedStrategyHistoryId,
		HaveCredentials: rp.HaveCredentials,
		HavePull:        rp.HavePull,
	}
	return resp, nil
}

// SubmitPublishApprove submit publish a strategy
func (s *Service) SubmitPublishApprove(ctx context.Context, req *pbcs.SubmitPublishApproveReq) (
	*pbcs.PublishResp, error) {

	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.App, Action: meta.Publish, ResourceID: req.AppId}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(grpcKit, res...)
	if err != nil {
		return nil, err
	}
	if err = s.validateGrayPercentGroups(grpcKit, req.Groups); err != nil {
		logs.Errorf("validate gray percent groups failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	r := &pbds.SubmitPublishApproveReq{
		BizId:           req.BizId,
		AppId:           req.AppId,
		ReleaseId:       req.ReleaseId,
		Memo:            req.Memo,
		All:             req.All,
		GrayPublishMode: req.GrayPublishMode,
		Default:         req.Default,
		Groups:          req.Groups,
		Labels:          req.Labels,
		GroupName:       req.GroupName,
		PublishType:     req.PublishType,
		PublishTime:     req.PublishTime,
		IsCompare:       req.IsCompare,
	}
	rp, err := s.client.DS.SubmitPublishApprove(grpcKit.RpcCtx(), r)
	if err != nil {
		logs.Errorf("publish failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	resp := &pbcs.PublishResp{
		Id:              rp.PublishedStrategyHistoryId,
		HaveCredentials: rp.HaveCredentials,
		HavePull:        rp.HavePull,
	}
	return resp, nil
}

// validateGrayPercentGroups 校验灰度比例分组
// nolint:funlen
func (s *Service) validateGrayPercentGroups(grpcKit *kit.Kit, groups []uint32) error {
	if len(groups) == 0 {
		return nil
	}

	// 过滤ID为0（默认分组）的group
	validGroupIDs := make([]uint32, 0)
	for _, groupID := range groups {
		if groupID != 0 {
			validGroupIDs = append(validGroupIDs, groupID)
		}
	}

	// 如果过滤后没有有效的group，直接返回
	if len(validGroupIDs) == 0 {
		return nil
	}

	// 获取所有分组的详细信息
	groupDetails := make([]*table.Group, 0)
	for _, groupID := range validGroupIDs {
		r := &pbds.GetGroupByIDReq{
			BizId:   grpcKit.BizID,
			GroupId: groupID,
		}
		group, err := s.client.DS.GetGroupByID(grpcKit.RpcCtx(), r)
		if err != nil {
			logs.Errorf("get group by id %d failed, err: %v, rid: %s", groupID, err, grpcKit.Rid)
			return err
		}

		// 将pb格式转换为table格式
		tableGroup, err := group.Group()
		if err != nil {
			return fmt.Errorf("convert group pb to table failed, err: %v", err)
		}
		groupDetails = append(groupDetails, tableGroup)
	}

	// 检查是否存在gray_percent分组
	var hasGrayPercentGroup bool
	var grayPercentGroups []*table.Group
	var nonGrayPercentGroups []*table.Group

	for _, group := range groupDetails {
		if group.Spec == nil || group.Spec.Selector == nil {
			nonGrayPercentGroups = append(nonGrayPercentGroups, group)
			continue
		}

		// 检查LabelsAnd中是否包含gray_percent
		hasGrayPercent := false
		for _, element := range group.Spec.Selector.LabelsAnd {
			if element.Key == table.GrayPercentKey {
				hasGrayPercent = true
				break
			}
		}

		// 检查LabelsOr中是否包含gray_percent
		if !hasGrayPercent {
			for _, element := range group.Spec.Selector.LabelsOr {
				if element.Key == table.GrayPercentKey {
					hasGrayPercent = true
					break
				}
			}
		}

		if hasGrayPercent {
			hasGrayPercentGroup = true
			grayPercentGroups = append(grayPercentGroups, group)
		} else {
			nonGrayPercentGroups = append(nonGrayPercentGroups, group)
		}
	}

	// 如果存在gray_percent分组，则所有分组都必须包含gray_percent
	if hasGrayPercentGroup && len(nonGrayPercentGroups) > 0 {
		return errors.New(i18n.T(grpcKit, "if gray_percent groups exist, all groups must contain gray_percent label"))
	}

	// 如果存在gray_percent分组，验证非gray_percent标签的一致性
	if hasGrayPercentGroup && len(grayPercentGroups) > 1 {
		// 获取第一个分组的非gray_percent标签作为基准
		baseNonGrayLabels := s.extractNonGrayPercentLabels(grayPercentGroups[0])

		// 验证其他分组的非gray_percent标签与基准一致
		for i := 1; i < len(grayPercentGroups); i++ {
			currentNonGrayLabels := s.extractNonGrayPercentLabels(grayPercentGroups[i])
			if !s.compareLabels(baseNonGrayLabels, currentNonGrayLabels) {
				return errors.New(i18n.T(grpcKit, "non-gray_percent labels must be consistent across all gray_percent groups"))
			}
		}
	}

	return nil
}

// Approve publish approve
func (s *Service) Approve(ctx context.Context, req *pbcs.ApproveReq) (*pbcs.ApproveResp, error) {

	grpcKit := kit.FromGrpcContext(ctx)

	// 审批通过和驳回无需权限
	// 撤销和上线使用上线版本权限
	if req.PublishStatus == string(table.RevokedPublish) || req.PublishStatus == string(table.AlreadyPublish) {
		res := []*meta.ResourceAttribute{
			{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
			{Basic: meta.Basic{Type: meta.App, Action: meta.Publish, ResourceID: req.AppId}, BizID: req.BizId},
		}
		err := s.authorizer.Authorize(grpcKit, res...)
		if err != nil {
			return nil, err
		}
	}

	r := &pbds.ApproveReq{
		BizId:         req.BizId,
		AppId:         req.AppId,
		ReleaseId:     req.ReleaseId,
		PublishStatus: req.PublishStatus,
		Reason:        req.Reason,
	}
	rp, err := s.client.DS.Approve(grpcKit.RpcCtx(), r)
	if err != nil {
		logs.Errorf("approve failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	resp := &pbcs.ApproveResp{
		HaveCredentials: rp.HaveCredentials,
		Code:            0,
		HavePull:        rp.HavePull,
		Message:         rp.Message,
	}
	return resp, nil
}

// GenerateReleaseAndPublish generate release and publish
func (s *Service) GenerateReleaseAndPublish(ctx context.Context, req *pbcs.GenerateReleaseAndPublishReq) (
	*pbcs.PublishResp, error) {

	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.App, Action: meta.GenerateRelease, ResourceID: req.AppId}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.App, Action: meta.Publish, ResourceID: req.AppId}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(grpcKit, res...)
	if err != nil {
		return nil, err
	}

	// 创建版本前验证非模板配置和模板配置是否存在冲突
	ci, err := s.ListConfigItems(grpcKit.RpcCtx(), &pbcs.ListConfigItemsReq{
		BizId: req.BizId,
		AppId: req.AppId,
		All:   true,
	})
	if err != nil {
		return nil, err
	}
	if ci.ConflictNumber > 0 {
		logs.Errorf("generate release and publish failed there is a file conflict, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, errors.New("generate release and publish failed there is a file conflict")
	}

	r := &pbds.GenerateReleaseAndPublishReq{
		BizId:           req.BizId,
		AppId:           req.AppId,
		ReleaseName:     req.ReleaseName,
		ReleaseMemo:     req.ReleaseMemo,
		Variables:       req.Variables,
		All:             req.All,
		GrayPublishMode: req.GrayPublishMode,
		Groups:          req.Groups,
		Labels:          req.Labels,
		GroupName:       req.GroupName,
	}
	rp, err := s.client.DS.GenerateReleaseAndPublish(grpcKit.RpcCtx(), r)
	if err != nil {
		logs.Errorf("generate release and publish failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	resp := &pbcs.PublishResp{
		Id: rp.PublishedStrategyHistoryId,
	}
	return resp, nil
}

// extractNonGrayPercentLabels 提取分组中除gray_percent之外的标签
func (s *Service) extractNonGrayPercentLabels(group *table.Group) []selector.Element {
	var nonGrayLabels []selector.Element

	if group.Spec == nil || group.Spec.Selector == nil {
		return nonGrayLabels
	}

	// 处理LabelsAnd中的非gray_percent标签
	for _, element := range group.Spec.Selector.LabelsAnd {
		if element.Key != table.GrayPercentKey {
			nonGrayLabels = append(nonGrayLabels, element)
		}
	}

	// 处理LabelsOr中的非gray_percent标签
	for _, element := range group.Spec.Selector.LabelsOr {
		if element.Key != table.GrayPercentKey {
			nonGrayLabels = append(nonGrayLabels, element)
		}
	}

	return nonGrayLabels
}

// compareLabels 比较两个标签列表是否一致（忽略顺序）
func (s *Service) compareLabels(labels1, labels2 []selector.Element) bool {
	if len(labels1) != len(labels2) {
		return false
	}

	// 为每个labels1中的元素在labels2中寻找匹配项
	for _, elem1 := range labels1 {
		found := false
		for _, elem2 := range labels2 {
			if elem1.Equal(&elem2) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
