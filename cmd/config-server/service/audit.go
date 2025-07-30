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
	"strings"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// ListAudits list audits
func (s *Service) ListAudits(ctx context.Context, req *pbcs.ListAuditsReq) (
	*pbcs.ListAuditsResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.Audit, Action: meta.View, ResourceID: req.BizId}, BizID: req.BizId},
	}

	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	apps, err := s.client.DS.ListAppsRest(grpcKit.RpcCtx(),
		&pbds.ListAppsRestReq{BizId: strconv.Itoa(int(req.GetBizId())), All: true})
	if err != nil {
		return nil, err
	}

	authorizedAppIds := []uint32{}
	authorizedAppMap := map[uint32]bool{}
	authRes := make([]*meta.ResourceAttribute, 0, len(apps.GetDetails()))
	for _, v := range apps.GetDetails() {
		bID, _ := strconv.ParseInt(v.SpaceId, 10, 64)
		authRes = append(authRes, &meta.ResourceAttribute{Basic: meta.Basic{
			Type: meta.App, Action: meta.View, ResourceID: v.Id}, BizID: uint32(bID)},
		)
		authRes = append(authRes, &meta.ResourceAttribute{Basic: meta.Basic{
			Type: meta.App, Action: meta.Update, ResourceID: v.Id}, BizID: uint32(bID)},
		)
		authRes = append(authRes, &meta.ResourceAttribute{Basic: meta.Basic{
			Type: meta.App, Action: meta.Delete, ResourceID: v.Id}, BizID: uint32(bID)},
		)
		authRes = append(authRes, &meta.ResourceAttribute{Basic: meta.Basic{
			Type: meta.App, Action: meta.Publish, ResourceID: v.Id}, BizID: uint32(bID)},
		)
		authRes = append(authRes, &meta.ResourceAttribute{Basic: meta.Basic{
			Type: meta.App, Action: meta.GenerateRelease, ResourceID: v.Id}, BizID: uint32(bID)},
		)
	}
	decisions, _, err := s.authorizer.AuthorizeDecision(grpcKit, authRes...)
	if err != nil {
		return nil, err
	}
	dMap := meta.DecisionsMap(decisions)
	for k, v := range dMap {
		if v && !authorizedAppMap[k.ResourceID] {
			authorizedAppIds = append(authorizedAppIds, k.ResourceID)
			authorizedAppMap[k.ResourceID] = true
		}
	}

	r := &pbds.ListAuditsReq{
		BizId:            req.BizId,
		AppId:            req.AppId,
		StartTime:        req.StartTime,
		EndTime:          req.EndTime,
		Start:            req.Start,
		Limit:            req.Limit,
		All:              req.All,
		Name:             req.Name,
		ResInstance:      req.ResInstance,
		Operator:         req.Operator,
		Id:               req.Id,
		AuthorizedAppIds: authorizedAppIds,
	}
	// 前端组件以逗号分开
	if req.Action != "" {
		r.Action = strings.Split(req.Action, ",")
	}
	if req.Status != "" {
		r.Status = strings.Split(req.Status, ",")
	}
	if req.ResourceType != "" {
		r.ResourceType = strings.Split(req.ResourceType, ",")
	}
	if req.OperateWay != "" {
		r.OperateWay = strings.Split(req.OperateWay, ",")
	}
	rp, err := s.client.DS.ListAudits(grpcKit.RpcCtx(), r)
	if err != nil {
		logs.Errorf("publish failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	resp := &pbcs.ListAuditsResp{
		Count:   rp.Count,
		Details: rp.Details,
	}
	return resp, nil
}
