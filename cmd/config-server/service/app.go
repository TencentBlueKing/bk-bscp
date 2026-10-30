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

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	iamauth "github.com/TencentBlueKing/bk-bscp/internal/iam/auth"
	"github.com/TencentBlueKing/bk-bscp/internal/rest/view/webannotation"
	"github.com/TencentBlueKing/bk-bscp/internal/space"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbas "github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbapp "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/app"
	pbcommit "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/commit"
	pbci "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/config-item"
	pbcontent "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/content"
	pbkv "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/kv"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// appCreateAuthResources 构造创建/克隆服务的鉴权资源。
// 服务落在具体环境里，创建权限按目标环境授予（app_create @ environment）：授权页面因此
// 需要先选项目、再选环境，项目/业务维度的授权经拓扑路径继承覆盖到该环境。
// 创建后会把该服务的运维权限自动授予创建者，因此创建权限只覆盖被授权的环境，
// 不会因为一条业务维度的授权就扩散到业务下的全部项目与环境。
func appCreateAuthResources(bizID, projectID, envID uint32) []*meta.ResourceAttribute {
	return []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: bizID},
		{
			// ResourceID 是目标环境 ID：创建时尚无服务实例，授权维度落在服务将要归属的环境上。
			Basic:         meta.Basic{Type: meta.App, Action: meta.Create, ResourceID: envID},
			BizID:         bizID,
			ProjectID:     projectID,
			EnvironmentID: envID,
		},
	}
}

// appListAuthResources 服务列表的鉴权资源：只校验业务访问，不按环境维度拦截。
// 列表按上层放行并返回全部服务，环境与服务的逐项权限由 ListAppsAnnotation
// 带回前端置灰并引导申请。ListAppsRest 与 ListAppsBySpaceRest 共用这一处，
// 避免两个入口的规则再次分叉。
func appListAuthResources(bizID uint32) []*meta.ResourceAttribute {
	return []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: bizID},
	}
}

// CreateApp create app with options
func (s *Service) CreateApp(ctx context.Context, req *pbcs.CreateAppReq) (*pbcs.CreateAppResp, error) {
	kt := kit.FromGrpcContext(ctx)

	if err := req.Validate(kt); err != nil {
		return nil, err
	}

	projectID := kt.ResolvedProjectID(req.GetProjectId())
	envID := kt.ResolvedEnvID(req.GetEnvId())

	if err := s.authorizer.Authorize(kt, appCreateAuthResources(req.BizId, projectID, envID)...); err != nil {
		return nil, err
	}

	r := &pbds.CreateAppReq{
		BizId: req.BizId,
		Spec: &pbapp.AppSpec{
			Name:        req.Name,
			ConfigType:  req.ConfigType,
			Memo:        req.Memo,
			Alias:       req.Alias,
			DataType:    req.DataType,
			IsApprove:   req.IsApprove,
			ApproveType: req.ApproveType,
			Approver:    req.Approver,
		},
		ProjectId: projectID,
		EnvId:     envID,
	}
	rp, err := s.client.DS.CreateApp(kt.RpcCtx(), r)
	if err != nil {
		logs.Errorf("create app failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	if err := s.authorizer.GrantResourceCreatorAction(kt, &client.GrantResourceCreatorActionOption{
		System:  sys.SystemIDBSCP,
		Type:    sys.Application,
		ID:      strconv.Itoa(int(rp.Id)),
		Name:    req.Name,
		Creator: kt.User,
		// 声明服务所属的业务、项目与环境，权限中心据此回显三级拓扑
		Ancestors: []client.GrantResourceCreatorActionAncestor{
			{
				System: sys.SystemIDCMDB,
				Type:   sys.Business,
				ID:     strconv.Itoa(int(req.BizId)),
			},
			{
				System: sys.SystemIDBSCP,
				Type:   sys.Project,
				ID:     strconv.Itoa(int(projectID)),
			},
			{
				System: sys.SystemIDBSCP,
				Type:   sys.Environment,
				ID:     strconv.Itoa(int(envID)),
			},
		},
	}); err != nil {
		logs.Errorf("grant app creator action failed, err: %v, rid: %s", err, kt.Rid)
	}

	resp := &pbcs.CreateAppResp{Id: rp.Id}
	return resp, nil
}

// UpdateApp update app with options
func (s *Service) UpdateApp(ctx context.Context, req *pbcs.UpdateAppReq) (*pbapp.App, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.App, Action: meta.Update, ResourceID: req.GetAppId()}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(grpcKit, res...)
	if err != nil {
		return nil, err
	}

	r := &pbds.UpdateAppReq{
		AppId: req.GetAppId(),
		BizId: req.BizId,
		Spec: &pbapp.AppSpec{
			Name:        req.Name,
			Memo:        req.Memo,
			Alias:       req.Alias,
			DataType:    req.DataType,
			IsApprove:   req.IsApprove,
			ApproveType: req.ApproveType,
			Approver:    req.Approver,
		},
		ProjectId: grpcKit.ResolvedProjectID(req.GetProjectId()),
		EnvId:     grpcKit.ResolvedEnvID(req.GetEnvId()),
	}
	app, err := s.client.DS.UpdateApp(grpcKit.RpcCtx(), r)
	if err != nil {
		logs.Errorf("update app failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	return app, nil
}

// DeleteApp delete app with options
func (s *Service) DeleteApp(ctx context.Context, req *pbcs.DeleteAppReq) (*pbcs.DeleteAppResp, error) {
	kt := kit.FromGrpcContext(ctx)
	resp := new(pbcs.DeleteAppResp)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.App, Action: meta.Delete, ResourceID: req.GetAppId()}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(kt, res...)
	if err != nil {
		return nil, err
	}

	r := &pbds.DeleteAppReq{
		AppId:     req.GetAppId(),
		BizId:     req.BizId,
		ProjectId: kt.ResolvedProjectID(req.GetProjectId()),
		EnvId:     kt.ResolvedEnvID(req.GetEnvId()),
	}
	_, err = s.client.DS.DeleteApp(kt.RpcCtx(), r)
	if err != nil {
		logs.Errorf("delete app failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return resp, nil
}

// GetApp get app with app id
func (s *Service) GetApp(ctx context.Context, req *pbcs.GetAppReq) (*pbapp.App, error) {
	kt := kit.FromGrpcContext(ctx)
	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.App, Action: meta.View, ResourceID: req.AppId}, BizID: req.BizId},
	}
	err := s.authorizer.Authorize(kt, res...)
	if err != nil {
		return nil, err
	}

	r := &pbds.GetAppReq{
		BizId:     req.BizId,
		AppId:     req.AppId,
		ProjectId: kt.ResolvedProjectID(req.GetProjectId()),
		EnvId:     kt.ResolvedEnvID(req.GetEnvId()),
	}
	rp, err := s.client.DS.GetApp(kt.RpcCtx(), r)
	if err != nil {
		logs.Errorf("list apps failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return rp, nil
}

// GetAppByName get app by app name
func (s *Service) GetAppByName(ctx context.Context, req *pbcs.GetAppByNameReq) (*pbapp.App, error) {
	kt := kit.FromGrpcContext(ctx)

	r := &pbds.GetAppByNameReq{
		BizId:     req.BizId,
		AppName:   req.AppName,
		ProjectId: kt.ResolvedProjectID(req.GetProjectId()),
		EnvId:     kt.ResolvedEnvID(req.GetEnvId()),
	}
	rp, err := s.client.DS.GetAppByName(kt.RpcCtx(), r)
	if err != nil {
		logs.Errorf("list apps failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
		{Basic: meta.Basic{Type: meta.App, Action: meta.View, ResourceID: rp.Id}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(kt, res...); err != nil {
		return nil, err
	}

	return rp, nil
}

// ListAppsRest list apps with rest filter
func (s *Service) ListAppsRest(ctx context.Context, req *pbcs.ListAppsRestReq) (*pbcs.ListAppsResp, error) {
	kt := kit.FromGrpcContext(ctx)

	err := s.authorizer.Authorize(kt, appListAuthResources(req.BizId)...)
	if err != nil {
		return nil, err
	}

	userSpaceResp, err := s.client.AS.ListUserSpace(kt.RpcCtx(), &pbas.ListUserSpaceReq{})
	if err != nil {
		return nil, err
	}

	if len(userSpaceResp.GetItems()) == 0 {
		return nil, errors.New("use have no spaces")
	}

	spaceMap := map[string]*pbas.Space{}
	// TODO: 该接口如果有用到后续单独加个方法
	// spaceIdList := []string{}
	for _, s := range userSpaceResp.GetItems() {
		spaceMap[s.SpaceId] = s
		// spaceIdList = append(spaceIdList, s.SpaceId)
	}

	r := &pbds.ListAppsRestReq{
		BizId:     req.BizId,
		Start:     req.Start,
		Limit:     req.Limit,
		Search:    req.Search,
		All:       req.All,
		ProjectId: kt.ResolvedProjectID(req.GetProjectId()),
	}
	rp, err := s.client.DS.ListAppsRest(kt.RpcCtx(), r)
	if err != nil {
		logs.Errorf("list apps failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 只填写当前页的space
	for _, app := range rp.Details {
		id := strconv.Itoa(int(app.BizId))
		sp, ok := spaceMap[id]
		if !ok {
			app.SpaceId = id
			app.SpaceName = ""
			app.SpaceTypeId = ""
			app.SpaceTypeName = ""
		} else {
			app.SpaceId = id
			app.SpaceName = sp.SpaceName
			app.SpaceTypeId = sp.SpaceTypeId
			app.SpaceTypeName = sp.SpaceTypeName
		}
	}

	resp := &pbcs.ListAppsResp{
		Count:   rp.Count,
		Details: rp.Details,
	}
	return resp, nil
}

// ListAppsBySpaceRest list apps with rest filter
// 与 ListAppsRest 同一条规则（见 appListAuthResources）：只校验业务访问，
// 不按环境维度拦截，未授权的服务由 ListAppsAnnotation 置灰并引导申请。
func (s *Service) ListAppsBySpaceRest(ctx context.Context, req *pbcs.ListAppsBySpaceRestReq) (*pbcs.ListAppsResp, error) {
	kt := kit.FromGrpcContext(ctx)

	err := s.authorizer.Authorize(kt, appListAuthResources(req.BizId)...)
	if err != nil {
		return nil, err
	}

	r := &pbds.ListAppsRestReq{
		BizId:      req.BizId,
		Start:      req.Start,
		Limit:      req.Limit,
		All:        req.All,
		TopIds:     req.TopIds,
		ConfigType: req.ConfigType,
		Search:     req.Search,
		ProjectId:  kt.ResolvedProjectID(req.GetProjectId()),
		EnvId:      kt.ResolvedEnvID(req.GetEnvId()),
	}
	rp, err := s.client.DS.ListAppsRest(kt.RpcCtx(), r)
	if err != nil {
		logs.Errorf("list apps failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	spaceUidMap := map[string]struct{}{}
	for _, app := range rp.Details {
		uid := space.BuildSpaceUid(space.BK_CMDB, strconv.Itoa(int(app.BizId)))
		spaceUidMap[uid] = struct{}{}

	}
	querySpaceReq := &pbas.QuerySpaceReq{SpaceUid: []string{}}
	for spaceUid := range spaceUidMap {
		querySpaceReq.SpaceUid = append(querySpaceReq.SpaceUid, spaceUid)
	}

	logs.V(5).Infof("query space, req: %+v, kt: %+v", querySpaceReq, kt)
	querySpaceResp, err := s.client.AS.QuerySpace(kt.RpcCtx(), querySpaceReq)
	if err != nil {
		return nil, errors.Wrap(err, "QuerySpace")
	}

	spaceMap := map[string]*pbas.Space{}
	for _, s := range querySpaceResp.GetItems() {
		spaceMap[s.SpaceId] = s
	}

	// 只填写当前页的space
	for _, app := range rp.Details {
		id := strconv.Itoa(int(app.BizId))
		sp, ok := spaceMap[id]
		if !ok {
			app.SpaceId = id
			app.SpaceName = ""
			app.SpaceTypeId = ""
			app.SpaceTypeName = ""
		} else {
			app.SpaceId = id
			app.SpaceName = sp.SpaceName
			app.SpaceTypeId = sp.SpaceTypeId
			app.SpaceTypeName = sp.SpaceTypeName
		}
	}

	resp := &pbcs.ListAppsResp{
		Count:         rp.Count,
		Details:       rp.Details,
		KvAppsCount:   rp.KvAppsCount,
		FileAppsCount: rp.FileAppsCount,
	}

	return resp, nil
}

// ListAppsAnnotation list apps permission annotations
func ListAppsAnnotation(ctx context.Context, kt *kit.Kit,
	authorizer iamauth.Authorizer, msg proto.Message) (*webannotation.Annotation, error) {

	resp, ok := msg.(*pbcs.ListAppsResp)
	if !ok {
		return nil, nil
	}

	perms := map[string]webannotation.Perm{}
	authRes := make([]*meta.ResourceAttribute, 0, len(resp.Details)*5)
	for _, v := range resp.Details {
		bID := uint64(v.GetBizId())
		if bID == 0 {
			bID, _ = strconv.ParseUint(v.SpaceId, 10, 32)
		}
		authRes = append(authRes, &meta.ResourceAttribute{Basic: meta.Basic{
			Type: meta.App, Action: meta.View, ResourceID: v.Id}, BizID: uint32(bID),
			ProjectID: v.GetProjectId(), EnvironmentID: v.GetEnvId()},
		)
		authRes = append(authRes, &meta.ResourceAttribute{Basic: meta.Basic{
			Type: meta.App, Action: meta.Update, ResourceID: v.Id}, BizID: uint32(bID),
			ProjectID: v.GetProjectId(), EnvironmentID: v.GetEnvId()},
		)
		authRes = append(authRes, &meta.ResourceAttribute{Basic: meta.Basic{
			Type: meta.App, Action: meta.Delete, ResourceID: v.Id}, BizID: uint32(bID),
			ProjectID: v.GetProjectId(), EnvironmentID: v.GetEnvId()},
		)
		authRes = append(authRes, &meta.ResourceAttribute{Basic: meta.Basic{
			Type: meta.App, Action: meta.Publish, ResourceID: v.Id}, BizID: uint32(bID),
			ProjectID: v.GetProjectId(), EnvironmentID: v.GetEnvId()},
		)
		authRes = append(authRes, &meta.ResourceAttribute{Basic: meta.Basic{
			Type: meta.App, Action: meta.GenerateRelease, ResourceID: v.Id}, BizID: uint32(bID),
			ProjectID: v.GetProjectId(), EnvironmentID: v.GetEnvId()},
		)
	}

	decisions, _, err := authorizer.AuthorizeDecision(kt, authRes...)
	if err != nil {
		return nil, err
	}

	// 必须按下标对齐，不能用资源属性做 map key：鉴权侧会给服务资源补齐
	// project_id / environment_id 再回传，补齐后的属性与这里的入参不再相等，
	// 用 map 查会一律取到 false，导致有权限的服务也被禁用。
	for idx, res := range authRes {
		if idx >= len(decisions) || decisions[idx] == nil {
			continue
		}

		id := strconv.Itoa(int(res.ResourceID))
		if _, ok := perms[id]; !ok {
			perms[id] = webannotation.Perm{}
		}
		perms[id][string(res.Action)] = decisions[idx].Authorized
	}

	return &webannotation.Annotation{Perms: perms}, nil
}

// CloneApp clones an application service
func (s *Service) CloneApp(ctx context.Context, req *pbcs.CloneAppReq) (*pbcs.CreateAppResp, error) {
	kt := kit.FromGrpcContext(ctx)

	projectID := kt.ResolvedProjectID(req.GetProjectId())
	envID := kt.ResolvedEnvID(req.GetEnvId())

	// 克隆等同于在目标环境里新建服务，与创建服务使用同一套校验。
	if err := s.authorizer.Authorize(kt, appCreateAuthResources(req.BizId, projectID, envID)...); err != nil {
		return nil, err
	}

	configItems := make([]*pbci.ConfigItem, 0)
	for _, v := range req.GetConfigItems() {
		configItems = append(configItems, &pbci.ConfigItem{
			Spec: &pbci.ConfigItemSpec{
				Name:     v.Name,
				Path:     v.Path,
				FileType: v.FileType,
				FileMode: v.FileMode,
				Memo:     v.Memo,
				Permission: &pbci.FilePermission{
					User:      v.User,
					UserGroup: v.UserGroup,
					Privilege: v.Privilege,
				},
				Charset: v.Charset,
			},
			CommitSpec: &pbcommit.CommitSpec{
				Content: &pbcontent.ContentSpec{
					Signature: v.Sign,
					ByteSize:  v.ByteSize,
					Md5:       v.Md5,
				},
				Memo: v.Memo,
			},
		})
	}

	bindings := make([]*pbds.CloneAppReq_TemplateBinding, 0)
	for _, v := range req.GetBindings() {
		bindings = append(bindings, &pbds.CloneAppReq_TemplateBinding{
			TemplateSpaceId: v.TemplateSpaceId,
			TemplateBinding: v.GetTemplateBinding(),
		})
	}

	kvItems := make([]*pbkv.Kv, 0)
	for _, v := range req.GetKvItems() {
		kvItems = append(kvItems, &pbkv.Kv{
			Spec: &pbkv.KvSpec{
				Key:                       v.Key,
				KvType:                    v.KvType,
				Value:                     v.Value,
				Memo:                      v.Memo,
				SecretType:                v.SecretType,
				SecretHidden:              v.SecretHidden,
				CertificateExpirationDate: v.CertificateExpirationDate,
			},
		})
	}

	resp, err := s.client.DS.CloneApp(kt.RpcCtx(), &pbds.CloneAppReq{
		BizId:       req.GetBizId(),
		Name:        req.GetName(),
		Alias:       req.GetAlias(),
		ConfigType:  req.GetConfigType(),
		Memo:        req.GetMemo(),
		IsApprove:   req.GetIsApprove(),
		ApproveType: req.GetApproveType(),
		Approver:    req.GetApprover(),
		ConfigItems: configItems,
		KvItems:     kvItems,
		Variables:   req.GetVariables(),
		Bindings:    bindings,
		PreHookId:   req.GetPreHookId(),
		PostHookId:  req.GetPostHookId(),
		DataType:    req.GetDataType(),
		ProjectId:   kt.ResolvedProjectID(req.GetProjectId()),
		EnvId:       kt.ResolvedEnvID(req.GetEnvId()),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.CreateAppResp{
		Id: resp.GetId(),
	}, nil
}
