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

// Package auth NOTES
package auth

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	bkiam "github.com/TencentBlueKing/iam-go-sdk"
	"github.com/bluele/gcache"
	"github.com/pkg/errors"

	"github.com/TencentBlueKing/bk-bscp/cmd/auth-server/options"
	"github.com/TencentBlueKing/bk-bscp/internal/space"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sdk/auth"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	adaptorv4 "github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/adaptor"
	authv4 "github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/auth"
	clientv4 "github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/model"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbas "github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// Iam 用于动态创建带租户信息的 IAM 客户端
type Iam struct {
	SystemID  string
	AppCode   string
	AppSecret string
	APIURL    string
}

// WithTenant 构造带指定 TenantID 的 IAM 客户端
func (i *Iam) WithTenant(tenantID string) *bkiam.IAM {
	return bkiam.NewAPIGatewayIAM(
		i.SystemID,
		i.AppCode,
		i.AppSecret,
		i.APIURL,
		bkiam.WithBkTenantID(tenantID),
	)
}

// IAMClientGetter 定义函数来获取带有租户信息的 IAM 客户端
type IAMClientGetter func(tenantID string) *bkiam.IAM

// Auth related operate.
type Auth struct {
	// auth related operate.
	auth auth.Authorizer
	// ds data service's auth related api.
	ds pbds.DataClient
	// disableAuth defines whether iam authorization is disabled
	disableAuth bool
	// disableWriteOpt defines which biz's write operation needs to be disabled
	disableWriteOpt *options.DisableWriteOption
	// iamSettings defines iam settings
	// spaceMgr defines space manager
	spaceMgr *space.Manager
	iamCli   IAMClientGetter
	// v4 权限中心 V4 的鉴权器
	v4 *authv4.Authorizer
	// v4Cli V4 网关客户端，用于生成权限申请链接与创建者授权
	v4Cli v4Gateway
	// appScopes 缓存服务的项目/环境归属，服务鉴权需要它拼三级路径
	appScopes gcache.Cache
	// defaultScopes 缓存按业务解析出的默认项目/环境，用于归属为 0 的历史服务
	defaultScopes gcache.Cache
}

// v4Gateway 是本包用到的 V4 网关能力。
type v4Gateway interface {
	AddAuthorizations(kt *kit.Kit, operator string, items []clientv4.Authorization) error
	GeneratePermApplyURL(kt *kit.Kit, permissions []clientv4.ApplyPermission) (string, error)
}

var _ v4Gateway = (*clientv4.Client)(nil)

// NewAuth new auth.
func NewAuth(auth auth.Authorizer, ds pbds.DataClient, disableAuth bool, iamCli IAMClientGetter,
	disableWriteOpt *options.DisableWriteOption, spaceMgr *space.Manager) (*Auth, error) {

	if auth == nil {
		return nil, errf.New(errf.InvalidParameter, "auth is nil")
	}

	if ds == nil {
		return nil, errf.New(errf.InvalidParameter, "data client is nil")
	}

	if disableWriteOpt == nil {
		return nil, errf.New(errf.InvalidParameter, "disable write operation is nil")
	}

	i := &Auth{
		auth:            auth,
		ds:              ds,
		disableAuth:     disableAuth,
		disableWriteOpt: disableWriteOpt,
		spaceMgr:        spaceMgr,
		iamCli:          iamCli,
		appScopes:       newAppScopeCache(),
		defaultScopes:   newDefaultScopeCache(),
	}

	return i, nil
}

// WithIAMV4 启用权限中心 V4：鉴权、申请链接与创建者授权改走 V4 实现。
func (a *Auth) WithIAMV4(cli *clientv4.Client, cfg authv4.Config) error {
	authorizer, err := authv4.NewAuthorizer(cli, cfg)
	if err != nil {
		return err
	}

	a.v4 = authorizer
	a.v4Cli = cli

	return nil
}

// useV4 判断当前是否走 V4 实现。
func (a *Auth) useV4() bool {
	return a.v4 != nil
}

// AuthorizeBatch authorize resource batch.
func (a *Auth) AuthorizeBatch(ctx context.Context, req *pbas.AuthorizeBatchReq) (*pbas.AuthorizeBatchResp, error) {
	kt := kit.FromGrpcContext(ctx)
	resp := new(pbas.AuthorizeBatchResp)

	if len(req.Resources) == 0 {
		resp.Decisions = make([]*pbas.Decision, 0)
		return resp, nil
	}

	// if write operations are disabled, returns corresponding error
	if err := a.isWriteOperationDisabled(kt, req.Resources); err != nil {
		return nil, err
	}

	// if auth is disabled, returns authorized for all request resources
	// if a.disableAuth {
	// 	resp.Decisions = make([]*pbas.Decision, len(req.Resources))
	// 	for index := range req.Resources {
	// 		resp.Decisions[index] = &pbas.Decision{Authorized: true}
	// 	}
	// 	return resp, nil
	// }

	// 服务挂在环境下，鉴权路径需要项目与环境两级归属。调用方常常只带 biz 与实例 ID，
	// 在这里统一回查或按默认项目/环境补齐，V3 与 V4 共用同一份归属，避免两套路径不一致。
	resources := pbas.ResourceAttributes(req.Resources)
	if err := a.fillResourceScope(kt, resources); err != nil {
		return nil, err
	}

	if a.useV4() {
		return a.authorizeBatchV4(kt, req.GetUser().GetUserName(), resources)
	}

	// parse bscp resource to iam resource
	opts, decisions, err := parseAttributesToBatchOptions(kt, req.User.UserInfo(), resources...)
	if err != nil {
		return nil, err
	}

	// all resources are skipped
	if opts == nil {
		resp.Decisions = pbas.PbDecisions(decisions)
		return resp, nil
	}

	// do authentication
	authDecisions, err := a.auth.AuthorizeBatch(ctx, opts)
	if err != nil {
		logs.Errorf("authorize batch failed, ops: %#v, req: %#v, err: %v, rid: %s", err, opts, req, kt.Rid)
		return nil, err
	}

	index := 0
	decisionLen := len(decisions)
	for _, decision := range authDecisions {
		// skip resources' decisions are already set as authorized
		for index < decisionLen && decisions[index].Authorized {
			index++
		}

		if index >= decisionLen {
			break
		}

		decisions[index].Authorized = decision.Authorized
		index++
	}

	resp.Decisions = pbas.PbDecisions(decisions)
	return resp, nil
}

// authorizeBatchV4 走权限中心 V4 的批量鉴权。
func (a *Auth) authorizeBatchV4(kt *kit.Kit, user string,
	resources []*meta.ResourceAttribute) (*pbas.AuthorizeBatchResp, error) {

	// 此处不校验用户名。feed-server 处理 sidecar 请求时使用应用凭证而非用户身份，
	// 用户名为空且相关资源无需鉴权，用户名是否必需交由 V4 鉴权器在资源映射后判断。
	decisions, err := a.v4.AuthorizeBatch(kt, user, resources...)
	if err != nil {
		logs.Errorf("iam v4 authorize batch failed, user: %s, resource count: %d, err: %v, rid: %s",
			user, len(resources), err, kt.Rid)
		return nil, err
	}

	return &pbas.AuthorizeBatchResp{Decisions: pbas.PbDecisions(decisions)}, nil
}

func (a *Auth) isWriteOperationDisabled(kt *kit.Kit, resources []*pbas.ResourceAttribute) error {
	if !a.disableWriteOpt.IsDisabled {
		return nil
	}

	for _, resource := range resources {
		action := meta.Action(resource.Basic.Action)
		if action == meta.Find || action == meta.SkipAction {
			continue
		}

		if a.disableWriteOpt.IsAll {
			logs.Errorf("all %s operation is disabled, rid: %s", action, kt.Rid)
			return errf.New(errf.Aborted, "bscp server is publishing, wring operation is not allowed")
		}

		bizID := resource.BizId
		if _, exists := a.disableWriteOpt.BizIDMap.Load(bizID); exists {
			logs.Errorf("biz id %d %s operation is disabled, rid: %s", bizID, action, kt.Rid)
			return errf.New(errf.Aborted, "bscp server is publishing, wring operation is not allowed")
		}
	}

	return nil
}

// parseAttributesToBatchOptions parse auth attributes to authorize batch options
func parseAttributesToBatchOptions(kt *kit.Kit, user *meta.UserInfo, resources ...*meta.ResourceAttribute) (
	*client.AuthBatchOptions, []*meta.Decision, error) {

	authBatchArr := make([]*client.AuthBatch, 0)
	decisions := make([]*meta.Decision, len(resources))
	for index, resource := range resources {
		decisions[index] = &meta.Decision{
			Resource:   resource,
			Authorized: false,
		}

		// this resource should be skipped, do not need to verify in auth center.
		if resource.Action == meta.SkipAction {
			decisions[index].Authorized = true
			logs.V(5).Infof("skip authorization for resource: %+v, rid: %s", resource, kt.Rid)
			continue
		}

		action, iamResources, err := AdaptAuthOptions(resource)
		if err != nil {
			logs.Errorf("adapt bscp resource to iam failed, err: %s, rid: %s", err, kt.Rid)
			return nil, nil, err
		}

		// this resource should be skipped, do not need to verify in auth center.
		if action == sys.Skip {
			decisions[index].Authorized = true
			logs.V(5).Infof("skip authorization for resource: %+v, rid: %s", resource, kt.Rid)
			continue
		}

		authBatchArr = append(authBatchArr, &client.AuthBatch{
			Action:    client.Action{ID: string(action)},
			Resources: iamResources,
		})
	}

	// all resources are skipped
	if len(authBatchArr) == 0 {
		return nil, decisions, nil
	}

	ops := &client.AuthBatchOptions{
		System: sys.SystemIDBSCP,
		Subject: client.Subject{
			Type: "user",
			ID:   user.UserName,
		},
		Batch: authBatchArr,
	}
	return ops, decisions, nil
}

// GetPermissionToApply get iam permission to apply when user has no permission to some resources.
func (a *Auth) GetPermissionToApply(ctx context.Context, req *pbas.GetPermissionToApplyReq) (
	*pbas.GetPermissionToApplyResp, error) {

	kt := kit.FromGrpcContext(ctx)
	resp := new(pbas.GetPermissionToApplyResp)

	resourceAttributes := pbas.ResourceAttributes(req.Resources)
	// 申请链接要回显「业务 → 项目 → 环境」，与鉴权使用同一份归属。
	if err := a.fillResourceScope(kt, resourceAttributes); err != nil {
		return nil, err
	}

	permission, err := a.getPermissionToApply(kt, resourceAttributes)
	if err != nil {
		return nil, err
	}

	url, err := a.genApplyURL(kt, resourceAttributes)
	if err != nil {
		return nil, err
	}
	resp.ApplyUrl = url

	resp.Permission = pbas.PbIamPermission(permission)
	return resp, nil
}

// genApplyURL 生成无权限时的申请链接。
func (a *Auth) genApplyURL(kt *kit.Kit, resources []*meta.ResourceAttribute) (string, error) {
	if a.useV4() {
		return a.genApplyURLV4(kt, resources)
	}

	application, err := AdaptIAMApplicationOptions(resources)
	if err != nil {
		return "", err
	}

	url, err := a.iamCli(kt.TenantID).GetApplyURL(*application)
	if err != nil {
		return "", errors.Wrap(err, "gen apply url")
	}

	return url, nil
}

// genApplyURLV4 用 V4 的接口生成申请链接。
func (a *Auth) genApplyURLV4(kt *kit.Kit, resources []*meta.ResourceAttribute) (string, error) {
	permissions := make([]clientv4.ApplyPermission, 0, len(resources))

	for _, res := range resources {
		mapped, err := adaptorv4.Adapt(res)
		if err != nil {
			return "", err
		}

		// 不做权限控制的资源不需要申请。
		if mapped.Skip {
			continue
		}

		permissions = append(permissions, clientv4.ApplyPermission{
			ActionID:  mapped.ActionID,
			Resources: mapped.ApplyResource(),
		})
	}

	if len(permissions) == 0 {
		return "", nil
	}

	url, err := a.v4Cli.GeneratePermApplyURL(kt, permissions)
	if err != nil {
		return "", errors.Wrap(err, "gen iam v4 apply url")
	}

	return url, nil
}

func (a *Auth) getPermissionToApply(kt *kit.Kit, resources []*meta.ResourceAttribute) (*meta.IamPermission, error) {
	permission := new(meta.IamPermission)
	permission.SystemID = sys.SystemIDBSCP
	permission.SystemName = sys.SystemNameBSCP

	// parse bscp auth resource
	resTypeIDsMap, permissionMap, err := a.parseResources(kt, resources)
	if err != nil {
		logs.Errorf("get inst ID and name map failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// get bscp resource name by id, then assign it to corresponding iam auth resource
	instIDNameMap, err := a.getInstIDNameMap(kt, resTypeIDsMap)
	if err != nil {
		return nil, err
	}

	for actionID, permissionTypeMap := range permissionMap {
		action := &meta.IamAction{
			ID:                   string(actionID),
			Name:                 sys.ActionIDNameMap[actionID],
			RelatedResourceTypes: make([]*meta.IamResourceType, 0),
		}

		for rscType := range permissionTypeMap {
			iamResourceType := permissionTypeMap[rscType]

			for idx, resources := range iamResourceType.Instances {
				for idx2, resource := range resources {
					iamResourceType.Instances[idx][idx2].Name = instIDNameMap[resource.ID]
				}
			}

			action.RelatedResourceTypes = append(action.RelatedResourceTypes, iamResourceType)
		}
		permission.Actions = append(permission.Actions, action)
	}

	return permission, nil
}

// parseResources parse bscp auth resource to iam permission resources in organized way
func (a *Auth) parseResources(kt *kit.Kit, resources []*meta.ResourceAttribute) (map[client.TypeID][]string,
	map[client.ActionID]map[client.TypeID]*meta.IamResourceType, error) {

	// resTypeIDsMap maps resource type to resource ids to get resource names.
	resTypeIDsMap := make(map[client.TypeID][]string)
	// permissionMap maps ActionID and TypeID to ResourceInstances
	permissionMap := make(map[client.ActionID]map[client.TypeID]*meta.IamResourceType, 0)

	for _, r := range resources {
		// parse bscp auth resource to iam action id and iam resources
		actionID, resources, err := AdaptAuthOptions(r)
		if err != nil {
			logs.Errorf("adaptor bscp resource to iam failed, err: %s, rid: %s", err, kt.Rid)
			return nil, nil, err
		}

		if _, ok := permissionMap[actionID]; !ok {
			permissionMap[actionID] = make(map[client.TypeID]*meta.IamResourceType, 0)
		}

		// generate iam resource resources by its paths and itself
		for _, res := range resources {
			if len(res.ID) == 0 && res.Attribute == nil {
				continue
			}

			resTypeIDsMap[res.Type] = append(resTypeIDsMap[res.Type], res.ID)

			resource := make([]*meta.IamResourceInstance, 0)
			if res.Attribute != nil {
				// parse bscp auth resource iam path attribute to iam ancestor resources
				iamPath, ok := res.Attribute[client.IamPathKey].([]string)
				if !ok {
					return nil, nil, fmt.Errorf("iam path(%v) is not string array", res.Attribute[client.IamPathKey])
				}

				ancestors, err := a.parseIamPathToAncestors(iamPath)
				if err != nil {
					return nil, nil, err
				}
				resource = append(resource, ancestors...)

				// record ancestor resource ids to get names from them afterwards
				for _, ancestor := range ancestors {
					ancestorType := client.TypeID(ancestor.Type)
					resTypeIDsMap[ancestorType] = append(resTypeIDsMap[ancestorType], ancestor.ID)
				}
			}

			// add iam resource of auth resource to the related iam resources after its ancestors
			resource = append(resource, &meta.IamResourceInstance{
				Type:     string(res.Type),
				TypeName: sys.ResourceTypeIDMap[res.Type],
				ID:       res.ID,
			})

			if permissionMap[actionID][res.Type] == nil {
				permissionMap[actionID][res.Type] = &meta.IamResourceType{
					SystemID:   res.System,
					SystemName: sys.SystemIDNameMap[res.System],
					Type:       string(res.Type),
					TypeName:   sys.ResourceTypeIDMap[res.Type],
					Instances:  make([][]*meta.IamResourceInstance, 0),
				}
			}
			permissionMap[actionID][res.Type].Instances = append(permissionMap[actionID][res.Type].Instances, resource)
		}
	}

	return resTypeIDsMap, permissionMap, nil
}

// parseIamPathToAncestors parse iam path to resource's ancestor resources
func (a *Auth) parseIamPathToAncestors(iamPath []string) ([]*meta.IamResourceInstance, error) {
	resources := make([]*meta.IamResourceInstance, 0)
	for _, path := range iamPath {
		pathItemArr := strings.Split(strings.Trim(path, "/"), "/")
		for _, pathItem := range pathItemArr {
			typeAndID := strings.Split(pathItem, ",")
			if len(typeAndID) != 2 {
				return nil, fmt.Errorf("pathItem %s invalid", pathItem)
			}
			id := typeAndID[1]
			if id == "*" {
				continue
			}
			resources = append(resources, &meta.IamResourceInstance{
				Type:     typeAndID[0],
				TypeName: sys.ResourceTypeIDMap[client.TypeID(typeAndID[0])],
				ID:       id,
			})
		}
	}
	return resources, nil
}

// Note how to get ancestor names? right now it means cc biz name, which is not in bscp
// getInstIDNameMap get resource id to name map by resource ids, groups by resource type
func (a *Auth) getInstIDNameMap(kt *kit.Kit, resTypeIDsMap map[client.TypeID][]string) (map[string]string, error) {

	nameMap := make(map[string]string)
	for resType, ids := range resTypeIDsMap {
		switch resType {
		case sys.Business:
			for _, id := range ids {
				space, err := a.spaceMgr.GetSpaceByUID(kt.Ctx, id)
				if err != nil {
					return nil, err
				}
				nameMap[id] = space.SpaceName
			}
		case sys.Application:
			for _, id := range ids {
				i, err := strconv.Atoi(id)
				if err != nil {
					return nil, err
				}
				app, err := a.ds.GetAppByID(kt.RpcCtx(), &pbds.GetAppByIDReq{AppId: uint32(i)})
				if err != nil {
					return nil, err
				}
				nameMap[id] = app.Spec.Name
			}
		case sys.AppCredential:
			return nil, fmt.Errorf("NOT IMPLEMENTED")
		}
	}
	return nameMap, nil
}

// GrantResourceCreatorAction 把新建资源的权限授予创建者，按运行时配置分发到 V3 或 V4。
func (a *Auth) GrantResourceCreatorAction(ctx context.Context,
	opts *client.GrantResourceCreatorActionOption) error {

	if a.useV4() {
		return a.grantResourceCreatorActionV4(kit.FromGrpcContext(ctx), opts)
	}

	return a.auth.GrantResourceCreatorAction(ctx, opts)
}

// creatorGrantDuration 是创建者授权的有效期。
const creatorGrantDuration = 365 * 24 * time.Hour

// grantResourceCreatorActionV4 用 V4 的 add_authorization 授予创建者权限。
func (a *Auth) grantResourceCreatorActionV4(kt *kit.Kit,
	opts *client.GrantResourceCreatorActionOption) error {

	if opts == nil {
		return errf.New(errf.InvalidParameter, "creator action option is nil")
	}

	// 目前服务和环境需要创建者授权，与 V3 注册的 ResourceCreatorAction 范围一致。
	var roleID, resourceTypeID string
	switch opts.Type {
	case sys.Application:
		roleID, resourceTypeID = model.RoleAppOperator, model.ResourceTypeApp
	case sys.Environment:
		roleID, resourceTypeID = model.RoleEnvironmentOperator, model.ResourceTypeEnvironment
	default:
		return errf.New(errf.InvalidParameter,
			fmt.Sprintf("unsupported resource type for creator grant: %s", opts.Type))
	}

	if opts.Creator == "" {
		return errf.New(errf.InvalidParameter, "creator is not set")
	}

	bizID := bizIDFromAncestors(opts.Ancestors)
	if bizID == "" {
		return errf.New(errf.InvalidParameter, "biz id is missing in ancestors")
	}

	subject := clientv4.NewUserSubject(opts.Creator)
	expiredAt := time.Now().Add(creatorGrantDuration).Unix()

	// 必须拆成两条：一次 add_authorization 仅授予 related_resource_type_id 指定的单个
	// 授权维度。若只授资源维度，角色内 biz 维度的 find_business_resource 不会生效；
	// BSCP 每个业务接口都有业务访问前置校验，该操作缺失时创建者对新资源的操作都会被拒绝。
	items := []clientv4.Authorization{
		{
			Subject:               subject,
			RoleID:                roleID,
			RelatedResourceTypeID: resourceTypeID,
			Resources: []clientv4.ResourceRef{
				{Type: resourceTypeID, ID: opts.ID},
			},
			ExpiredAt: expiredAt,
		},
		{
			Subject:               subject,
			RoleID:                roleID,
			RelatedResourceTypeID: model.ResourceTypeBiz,
			Resources: []clientv4.ResourceRef{
				{Type: model.ResourceTypeBiz, ID: bizID},
			},
			ExpiredAt: expiredAt,
		},
	}

	// 操作人取创建者：该授权由创建资源的行为直接触发，并非管理员代为授权。
	if err := a.v4Cli.AddAuthorizations(kt, opts.Creator, items); err != nil {
		logs.Errorf("iam v4 grant creator action failed, creator: %s, type: %s, id: %s, "+
			"biz: %s, err: %v, rid: %s", opts.Creator, opts.Type, opts.ID, bizID, err, kt.Rid)
		return err
	}

	logs.Infof("iam v4 granted %s to creator %s on %s %s and biz %s, rid: %s",
		roleID, opts.Creator, resourceTypeID, opts.ID, bizID, kt.Rid)

	return nil
}

// bizIDFromAncestors 从祖先列表里取业务 ID。
// V4 的资源类型不带 system 维度，因此只按类型匹配，忽略 Ancestor.System。
func bizIDFromAncestors(ancestors []client.GrantResourceCreatorActionAncestor) string {
	for _, ancestor := range ancestors {
		if ancestor.Type == sys.Business {
			return ancestor.ID
		}
	}

	return ""
}
