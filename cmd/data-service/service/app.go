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
	"strings"
	"time"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/cmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbapp "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/app"
	pbbase "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
	pbci "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/config-item"
	pbkv "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/kv"
	pbtv "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/template-variable"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
	"github.com/TencentBlueKing/bk-bscp/pkg/version"
)

// CreateApp create application.
func (s *Service) CreateApp(ctx context.Context, req *pbds.CreateAppReq) (*pbds.CreateResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// validate biz exist when user is not for test
	if !strings.HasPrefix(kt.User, constant.BKUserForTestPrefix) {
		if err := s.validateBizExist(kt, req.BizId); err != nil {
			logs.Errorf("validate biz exist failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
	}

	if _, err := s.dao.App().GetByName(kt, req.BizId, req.Spec.Name); err == nil {
		return nil, errf.Errorf(errf.InvalidRequest, i18n.T(kt, "app name %s already exists", req.Spec.Name))
	}

	if _, err := s.dao.App().GetByAlias(kt, req.BizId, req.Spec.Alias); err == nil {
		return nil, errf.Errorf(errf.InvalidRequest, i18n.T(kt, "app alias %s already exists", req.Spec.Alias))
	}

	app := &table.App{
		BizID: req.BizId,
		Spec:  req.Spec.AppSpec(),
		Revision: &table.Revision{
			Creator: kt.User,
			Reviser: kt.User,
		},
	}

	id, err := s.dao.App().Create(kt, app)
	if err != nil {
		logs.Errorf("create app failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	resp := &pbds.CreateResp{Id: id}
	return resp, nil
}

// UpdateApp update application.
func (s *Service) UpdateApp(ctx context.Context, req *pbds.UpdateAppReq) (*pbapp.App, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	old, err := s.dao.App().GetByAlias(grpcKit, req.BizId, req.Spec.Alias)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logs.Errorf("get app failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, errf.Errorf(errf.DBOpFailed, i18n.T(grpcKit, "get app failed, err: %v", err))
	}
	if !errors.Is(gorm.ErrRecordNotFound, err) && old.ID != req.Id {
		return nil, errf.Errorf(errf.InvalidRequest, "app alias %s already exists", req.Spec.Alias)
	}

	app, err := s.dao.App().Get(grpcKit, req.BizId, req.Id)
	if err != nil {
		logs.Errorf("get app failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, errf.Errorf(errf.DBOpFailed, i18n.T(grpcKit, "get app failed, err: %v", err))
	}
	if app.Spec.ConfigType == table.KV {
		if e := s.checkUpdateAppDataType(grpcKit, req, app); e != nil {
			return nil, e
		}
	}

	app = &table.App{
		ID:    req.Id,
		BizID: req.BizId,
		Spec:  req.Spec.AppSpec(),
		Revision: &table.Revision{
			Reviser: grpcKit.User,
		},
	}
	if err = s.dao.App().Update(grpcKit, app); err != nil {
		logs.Errorf("update app failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	app, err = s.dao.App().Get(grpcKit, req.BizId, req.Id)
	if err != nil {
		logs.Errorf("updating the app was successful, but retrieving the app failed, err: %v, rid: %s",
			err, grpcKit.Rid)
		return nil, err
	}

	return pbapp.PbApp(app), nil
}

func (s *Service) checkUpdateAppDataType(kt *kit.Kit, req *pbds.UpdateAppReq, app *table.App) error {

	if app.Spec.DataType == table.DataType(req.Spec.DataType) {
		return nil
	}

	if req.Spec.DataType == string(table.KvAny) {
		return nil
	}

	// 获取所有的kv
	kvState := []string{
		string(table.KvStateAdd),
		string(table.KvStateRevise),
		string(table.KvStateUnchange),
		string(table.KvStateDelete),
	}
	kvList, err := s.dao.Kv().ListAllByAppID(kt, app.ID, req.BizId, kvState)
	if err != nil {
		return err
	}
	if len(kvList) == 0 {
		return nil
	}

	for _, kv := range kvList {
		kvType, _, err := s.getKv(kt, req.BizId, kv.Attachment.AppID, kv.Spec.Version, kv.Spec.Key)
		if err != nil {
			return err
		}

		if string(kvType) != req.Spec.DataType {
			return errf.Errorf(errf.InvalidArgument, i18n.T(kt, "the specified type does not match the actual configuration"))
		}
	}

	return nil
}

// DeleteApp delete application.
func (s *Service) DeleteApp(ctx context.Context, req *pbds.DeleteAppReq) (*pbbase.EmptyResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	app := &table.App{
		ID:    req.Id,
		BizID: req.BizId,
	}

	tx := s.dao.GenQuery().Begin()

	// Use defer to ensure transaction is properly handled
	committed := false
	defer func() {
		if !committed {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, grpcKit.Rid)
			}
		}
	}()

	// 1. delete app related resources
	if err := s.deleteAppRelatedResources(grpcKit, req, tx); err != nil {
		logs.Errorf("delete app related resources failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(grpcKit, "delete app related resources failed, err: %v", err))
	}

	// 2. delete app
	if err := s.dao.App().DeleteWithTx(grpcKit, tx, app); err != nil {
		logs.Errorf("delete app failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(grpcKit, "delete app failed, err: %v", err))
	}

	if err := tx.Commit(); err != nil {
		logs.Errorf("commit transaction failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(grpcKit, "delete app failed, err: %v", err))
	}
	committed = true

	return new(pbbase.EmptyResp), nil
}

func (s *Service) deleteAppRelatedResources(grpcKit *kit.Kit, req *pbds.DeleteAppReq, tx *gen.QueryTx) error {
	// delete app template binding
	if err := s.dao.AppTemplateBinding().DeleteByAppIDWithTx(grpcKit, tx, req.GetBizId(), req.Id); err != nil {
		logs.Errorf("delete app template binding failed, err: %v, rid: %s", err, grpcKit.Rid)
		return err
	}

	// delete group app binding
	if err := s.dao.GroupAppBind().BatchDeleteByAppIDWithTx(grpcKit, tx, req.Id, req.BizId); err != nil {
		logs.Errorf("delete group app binding failed, err: %v, rid: %s", err, grpcKit.Rid)
		return err
	}

	// delete released group
	if err := s.dao.ReleasedGroup().BatchDeleteByAppIDWithTx(grpcKit, tx, req.Id, req.BizId); err != nil {
		logs.Errorf("delete group app binding failed, err: %v, rid: %s", err, grpcKit.Rid)
		return err
	}

	// delete app template binding
	if err := s.dao.ReleasedAppTemplate().BatchDeleteByAppIDWithTx(grpcKit, tx, req.Id, req.BizId); err != nil {
		logs.Errorf("delete released app template failed, err: %v, rid: %s", err, grpcKit.Rid)
		return err
	}

	// delete released app template binding
	if err := s.dao.ReleasedAppTemplate().BatchDeleteByAppIDWithTx(grpcKit, tx, req.Id, req.BizId); err != nil {
		logs.Errorf("delete released app template failed, err: %v, rid: %s", err, grpcKit.Rid)
		return err
	}

	// delete released app template variables
	if err := s.dao.ReleasedAppTemplateVariable().BatchDeleteByAppIDWithTx(grpcKit, tx, req.Id, req.BizId); err != nil {
		logs.Errorf("delete released app template variables failed, err: %v, rid: %s", err, grpcKit.Rid)
		return err
	}

	// delete released hook
	if err := s.dao.ReleasedHook().DeleteByAppIDWithTx(grpcKit, tx, req.Id, req.BizId); err != nil {
		logs.Errorf("delete released hooks failed, err: %v, rid: %s", err, grpcKit.Rid)
		return err
	}

	// delete related credential scopes and update credentials
	if err := s.updateRelatedCredentials(grpcKit, tx, req.Id, req.BizId); err != nil {
		return err
	}

	return nil
}

// updateRelatedCredentials delete related credential scopes and update credentials to emit event.
func (s *Service) updateRelatedCredentials(grpcKit *kit.Kit, tx *gen.QueryTx, appID, bizID uint32) error {
	app, err := s.dao.App().Get(grpcKit, bizID, appID)
	if err != nil {
		logs.Errorf("get app failed, err: %v, rid: %s", err, grpcKit.Rid)
		return err
	}
	matchedScopeIDs := make([]uint32, 0)
	matchedCredentialIDs := make([]uint32, 0)
	// delete related credential scopes
	scopes, err := s.dao.CredentialScope().ListAll(grpcKit, bizID)
	if err != nil {
		return err
	}
	for _, scope := range scopes {
		appName, _, _ := scope.Spec.CredentialScope.Split()
		if appName == app.Spec.Name {
			matchedScopeIDs = append(matchedScopeIDs, scope.ID)
			matchedCredentialIDs = append(matchedCredentialIDs, scope.Attachment.CredentialId)
		}
	}
	if e := s.dao.CredentialScope().BatchDeleteWithTx(grpcKit, tx, bizID, matchedScopeIDs); e != nil {
		logs.Errorf("delete credential scopes failed, err: %v, rid: %s", e, grpcKit.Rid)
		return e
	}
	// update credentials
	matchedCredentialIDs = tools.RemoveDuplicates(matchedCredentialIDs)
	for _, credentialID := range matchedCredentialIDs {
		if e := s.dao.Credential().UpdateRevisionWithTx(grpcKit, tx, bizID, credentialID); e != nil {
			logs.Errorf("update credential revision failed, err: %v, rid: %s", e, grpcKit.Rid)
			return e
		}
	}

	return nil
}

// GetApp get apps by app id.
func (s *Service) GetApp(ctx context.Context, req *pbds.GetAppReq) (*pbapp.App, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	app, err := s.dao.App().Get(grpcKit, req.BizId, req.AppId)
	if err != nil {
		logs.Errorf("get app failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	return pbapp.PbApp(app), nil
}

// GetAppByID get apps by only by app id.
func (s *Service) GetAppByID(ctx context.Context, req *pbds.GetAppByIDReq) (*pbapp.App, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	app, err := s.dao.App().GetByID(grpcKit, req.GetAppId())
	if err != nil {
		logs.Errorf("get app by id failed, err: %v, rid: %s", err, grpcKit.Rid)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errf.Errorf(errf.AppNotExists, i18n.T(grpcKit, "app %d not found", req.AppId))
		}
		return nil, errors.Wrapf(err, "query app by id %d", req.GetAppId())
	}

	return pbapp.PbApp(app), nil
}

// GetAppByName get app by app name.
func (s *Service) GetAppByName(ctx context.Context, req *pbds.GetAppByNameReq) (*pbapp.App, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	app, err := s.dao.App().GetByName(grpcKit, req.GetBizId(), req.GetAppName())
	if err != nil {
		logs.Errorf("get app by name failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, errors.Wrapf(err, "query app by name %s failed", req.GetAppName())
	}

	return pbapp.PbApp(app), nil
}

// ListAppsRest list apps by query condition.
func (s *Service) ListAppsRest(ctx context.Context, req *pbds.ListAppsRestReq) (*pbds.ListAppsResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 默认分页
	limit := uint(req.Limit)
	if limit == 0 {
		limit = 50
	}

	// StrToUint32Slice the comma separated string goes to uint32 slice
	topIds, _ := tools.StrToUint32Slice(req.TopIds)
	opt := &types.BasePage{
		Start:  req.Start,
		Limit:  limit,
		All:    req.All,
		TopIds: topIds,
	}
	if err := opt.Validate(types.DefaultPageOption); err != nil {
		return nil, err
	}

	bizList, err := tools.GetUint32List(req.BizId)
	if err != nil {
		return nil, err
	}
	if len(bizList) == 0 {
		return nil, fmt.Errorf("bizList is empty")
	}

	details, count, err := s.dao.App().List(kt, bizList, req.Search, req.ConfigType, req.Operator, opt)
	if err != nil {
		logs.Errorf("list apps failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	kvAppsCount, fileAppsCount, err := s.dao.App().CountApps(kt, bizList, req.Operator, req.Search)
	if err != nil {
		logs.Errorf("count apps failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	resp := &pbds.ListAppsResp{
		Count:         uint32(count),
		Details:       pbapp.PbApps(details),
		KvAppsCount:   uint32(kvAppsCount),
		FileAppsCount: uint32(fileAppsCount),
	}
	return resp, nil
}

// ListAppsByIDs list apps by query condition.
func (s *Service) ListAppsByIDs(ctx context.Context, req *pbds.ListAppsByIDsReq) (*pbds.ListAppsByIDsResp, error) {
	kt := kit.FromGrpcContext(ctx)

	if len(req.Ids) == 0 {
		return nil, fmt.Errorf("app ids is empty")
	}

	details, err := s.dao.App().ListAppsByIDs(kt, req.Ids)
	if err != nil {
		logs.Errorf("list apps failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	resp := &pbds.ListAppsByIDsResp{
		Details: pbapp.PbApps(details),
	}
	return resp, nil
}

// validateBizExist validate if biz exists in cmdb before create app.
func (s *Service) validateBizExist(kt *kit.Kit, bizID uint32) error {
	// if build version is debug mode, not need to validate biz exist in cmdb.
	if version.Debug() {
		return nil
	}

	searchBizParams := &cmdb.SearchBizParams{
		Fields: []string{"bk_biz_id"},
		Page:   cmdb.BasePage{Limit: 1},
		BizPropertyFilter: &cmdb.QueryFilter{
			Rule: cmdb.CombinedRule{
				Condition: cmdb.ConditionAnd,
				Rules: []cmdb.Rule{
					cmdb.AtomRule{
						Field:    cmdb.BizIDField,
						Operator: cmdb.OperatorEqual,
						Value:    bizID,
					}},
			}},
	}

	bizResp, err := s.esb.Cmdb().SearchBusiness(kt.Ctx, searchBizParams)
	if err != nil {
		return errf.Errorf(errf.InvalidRequest, i18n.T(kt, "business query failed, err: %v", err))
	}

	if bizResp.Count == 0 {
		return errf.Errorf(errf.RelatedResNotExist, i18n.T(kt, "app related biz %d is not exist", bizID))
	}

	return nil
}

// BatchUpdateLastConsumedTime 批量更新最后一次拉取时间
func (s *Service) BatchUpdateLastConsumedTime(ctx context.Context, req *pbds.BatchUpdateLastConsumedTimeReq) (
	*pbds.BatchUpdateLastConsumedTimeResp, error) {
	kit := kit.FromGrpcContext(ctx)

	err := s.dao.App().BatchUpdateLastConsumedTime(kit, req.GetAppIds())
	if err != nil {
		return nil, err
	}

	return &pbds.BatchUpdateLastConsumedTimeResp{}, nil
}

// CloneApp clones an application service.
func (s *Service) CloneApp(ctx context.Context, req *pbds.CloneAppReq) (*pbds.CreateResp, error) {
	kit := kit.FromGrpcContext(ctx)

	// 1. 创建服务相关数据
	// 2. 导入相关配置数据
	// 3. 如果是文件配置需导入模板和变量
	// 4. 导入脚本
	now := time.Now().UTC()
	tx := s.dao.GenQuery().Begin()
	committed := false
	defer func() {
		if !committed {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kit.Rid)
			}
		}
	}()

	if !strings.HasPrefix(kit.User, constant.BKUserForTestPrefix) {
		if err := s.validateBizExist(kit, req.BizId); err != nil {
			logs.Errorf("validate biz exist failed, err: %v, rid: %s", err, kit.Rid)
			return nil, err
		}
	}

	if _, err := s.dao.App().GetByName(kit, req.BizId, req.GetName()); err == nil {
		return nil, errf.Errorf(errf.InvalidRequest, i18n.T(kit, "app name %s already exists", req.GetName()))
	}

	if _, err := s.dao.App().GetByAlias(kit, req.BizId, req.GetAlias()); err == nil {
		return nil, errf.Errorf(errf.InvalidRequest, i18n.T(kit, "app alias %s already exists", req.GetAlias()))
	}

	app := &table.App{
		BizID: req.BizId,
		Spec: &table.AppSpec{
			Name:        req.GetName(),
			ConfigType:  table.ConfigType(req.GetConfigType()),
			Memo:        req.GetMemo(),
			Alias:       req.GetAlias(),
			DataType:    table.DataType(req.GetDataType()),
			ApproveType: table.ApproveType(req.GetApproveType()),
			IsApprove:   req.GetIsApprove(),
			Approver:    req.GetApprover(),
		},
		Revision: &table.Revision{
			Creator: kit.User,
		},
	}

	appID, err := s.dao.App().CreateWithTx(kit, tx, app)
	if err != nil {
		return nil, err
	}

	if len(req.GetConfigItems()) != 0 {
		err := s.createConfigItems(kit, tx, req.GetBizId(), appID, req.GetPreHookId(), req.GetPostHookId(),
			req.GetConfigItems(), req.GetVariables(), req.GetBindings(), now)
		if err != nil {
			return nil, err
		}
	}

	if len(req.GetKvItems()) != 0 {
		err := s.createKvItems(kit, tx, req.BizId, appID, req.GetKvItems(), now)
		if err != nil {
			return nil, err
		}
	}

	if e := tx.Commit(); e != nil {
		logs.Errorf("commit transaction failed, err: %v, rid: %s", e, kit.Rid)
		return nil, e
	}

	committed = true

	return &pbds.CreateResp{
		Id: appID,
	}, nil
}

// createConfigItems creates file configuration items
// nolint:funlen
func (s *Service) createConfigItems(kit *kit.Kit, tx *gen.QueryTx, bizID, appID, preHookId, postHookId uint32,
	configItems []*pbci.ConfigItem, variables []*pbtv.TemplateVariableSpec,
	bindings []*pbds.CloneAppReq_TemplateBinding, now time.Time) error {

	items := make([]*pbds.BatchUpsertConfigItemsReq_ConfigItem, 0)
	for _, v := range configItems {
		items = append(items, &pbds.BatchUpsertConfigItemsReq_ConfigItem{
			ConfigItemAttachment: &pbci.ConfigItemAttachment{
				BizId: bizID, AppId: appID,
			},
			ConfigItemSpec: v.GetSpec(), ContentSpec: v.GetCommitSpec().GetContent(),
		})
	}

	_, err := s.doBatchCreateConfigItems(kit, tx, items, now, bizID, appID)
	if err != nil {
		logs.Errorf("batch create config items failed, err: %v, rid: %s", err, kit.Rid)
		return err
	}

	// 2. 新增模板变量
	variableMap := make([]*table.TemplateVariableSpec, 0)
	for _, vars := range variables {
		variableMap = append(variableMap, vars.TemplateVariableSpec())
	}
	appVar := &table.AppTemplateVariable{
		Spec: &table.AppTemplateVariableSpec{
			Variables: variableMap,
		},
		Attachment: &table.AppTemplateVariableAttachment{
			BizID: bizID, AppID: appID,
		},
		Revision: &table.Revision{
			Creator: kit.User, CreatedAt: now, Reviser: kit.User,
		},
	}
	if appVar.Spec.Variables != nil {
		if errT := s.dao.AppTemplateVariable().UpsertWithTx(kit, tx, appVar); errT != nil {
			logs.Errorf("batch create template variable failed, err: %v, rid: %s", errT, kit.Rid)
			return errT
		}
	}

	// 3. 关联模板套餐
	templateSpaceIDs := make([]uint32, 0, len(bindings))
	templateSetIDs := make([]uint32, 0, len(bindings))
	templateIDs := make([]uint32, 0)
	latestTemplateIDs := make([]uint32, 0)
	templateRevisionIDs := make([]uint32, 0)
	templateRevisions := make(table.TemplateBindings, 0, len(bindings))
	for _, v := range bindings {
		templateSpaceIDs = append(templateSpaceIDs, v.GetTemplateSpaceId())
		templateSetIDs = append(templateSetIDs, v.GetTemplateBinding().GetTemplateSetId())
		tb := v.GetTemplateBinding()
		revs := make([]*table.TemplateRevisionBinding, 0, len(tb.GetTemplateRevisions()))
		for _, b := range tb.GetTemplateRevisions() {
			templateIDs = append(templateIDs, b.GetTemplateId())
			templateRevisionIDs = append(templateRevisionIDs, b.GetTemplateRevisionId())
			if b.GetIsLatest() {
				latestTemplateIDs = append(latestTemplateIDs, b.GetTemplateId())
			}
			revs = append(revs, &table.TemplateRevisionBinding{
				TemplateID:         b.GetTemplateId(),
				TemplateRevisionID: b.GetTemplateRevisionId(),
				IsLatest:           b.GetIsLatest(),
			})
		}

		templateRevisions = append(templateRevisions, &table.TemplateBinding{
			TemplateSetID: tb.GetTemplateSetId(), TemplateRevisions: revs,
		})
	}

	atb := &table.AppTemplateBinding{
		Spec: &table.AppTemplateBindingSpec{
			TemplateSpaceIDs:    templateSpaceIDs,
			TemplateSetIDs:      templateSetIDs,
			TemplateIDs:         templateIDs,
			TemplateRevisionIDs: templateRevisionIDs,
			LatestTemplateIDs:   latestTemplateIDs,
			Bindings:            templateRevisions,
		},
		Attachment: &table.AppTemplateBindingAttachment{
			BizID: bizID, AppID: appID,
		},
		Revision: &table.Revision{
			Creator: kit.User, CreatedAt: now,
		},
	}

	if len(atb.Spec.Bindings) != 0 {
		_, err = s.dao.AppTemplateBinding().CreateWithTx(kit, tx, atb)
		if err != nil {
			logs.Errorf("batch create binding template failed, err: %v, rid: %s", err, kit.Rid)
			return err
		}
	}
	// 4. 引用脚本
	if err := s.referenceHook(kit, tx, bizID, appID, preHookId, postHookId); err != nil {
		logs.Errorf("reference hook failed, err: %v, rid: %s", err, kit.Rid)
		return err
	}

	return nil
}

// createKvItems creates KV configuration items and stores them in vault
func (s *Service) createKvItems(kit *kit.Kit, tx *gen.QueryTx, bizID, appID uint32,
	kvs []*pbkv.Kv, now time.Time) error {

	items := make([]*pbds.BatchUpsertKvsReq_Kv, 0)
	toCreate := make([]*table.Kv, 0)
	for _, v := range kvs {
		toCreate = append(toCreate, &table.Kv{
			KvState: table.KvStateAdd,
			Spec:    v.GetSpec().KvSpec(),
			Attachment: &table.KvAttachment{
				BizID: bizID,
				AppID: appID,
			},
			Revision: &table.Revision{
				Creator:   kit.User,
				CreatedAt: now,
			},
			ContentSpec: &table.ContentSpec{
				Signature: tools.SHA256(v.Spec.Value),
				Md5:       tools.MD5(v.Spec.Value),
				ByteSize:  uint64(len(v.Spec.Value)),
			},
		})

		items = append(items, &pbds.BatchUpsertKvsReq_Kv{
			KvAttachment: v.GetAttachment(),
			KvSpec:       v.GetSpec(),
		})
	}

	// 1. 新增或者编辑vault中的kv
	versionMap, err := s.doBatchUpsertVault(kit, &pbds.BatchUpsertKvsReq{
		BizId:      bizID,
		AppId:      appID,
		Kvs:        items,
		ReplaceAll: false,
	})
	if err != nil {
		return err
	}

	for _, v := range toCreate {
		version, exists := versionMap[v.Spec.Key]
		if !exists {
			return errors.New(i18n.T(kit, "save kv failed"))
		}
		v.Spec.Version = uint32(version)
	}

	// 2. 创建kv
	if err := s.dao.Kv().BatchCreateWithTx(kit, tx, toCreate); err != nil {
		logs.Errorf("batch create kv failed, err: %v, rid: %s", err, kit.Rid)
		return errf.Errorf(errf.DBOpFailed, i18n.T(kit, "batch create of KV config failed, err: %v", err))
	}

	return nil
}

func (s *Service) referenceHook(kit *kit.Kit, tx *gen.QueryTx, bizID, appID, preHookId, postHookId uint32) error {
	preHook := &table.ReleasedHook{
		AppID: appID,
		BizID: bizID,
		// ReleasedID 0 for editing release
		ReleaseID: 0,
		HookID:    preHookId,
		HookType:  table.PreHook,
	}
	postHook := &table.ReleasedHook{
		AppID: appID,
		BizID: bizID,
		// ReleasedID 0 for editing release
		ReleaseID: 0,
		HookID:    postHookId,
		HookType:  table.PostHook,
	}

	if preHookId > 0 {
		hook, err := s.getReleasedHook(kit, preHook)
		if err != nil {
			logs.Errorf("no released releases of the pre-hook, err: %v, rid: %s", err, kit.Rid)
			return errors.New("no released releases of the pre-hook")
		}

		if err = s.dao.ReleasedHook().UpsertWithTx(kit, tx, hook); err != nil {
			logs.Errorf("upsert pre-hook failed, err: %v, rid: %s", err, kit.Rid)
			return err
		}
	}

	if postHookId > 0 {
		hook, err := s.getReleasedHook(kit, postHook)
		if err != nil {
			logs.Errorf("get post-hook failed, err: %v, rid: %s", err, kit.Rid)
			return err
		}

		if err = s.dao.ReleasedHook().UpsertWithTx(kit, tx, hook); err != nil {
			logs.Errorf("upsert post-hook failed, err: %v, rid: %s", err, kit.Rid)
			return err
		}
	}

	return nil
}
