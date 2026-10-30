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

package auth

import (
	"strconv"
	"strings"
	"time"

	"github.com/bluele/gcache"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

const (
	// appScopeCacheSize 是服务归属缓存的条目上限。
	appScopeCacheSize = 4096
	// appScopeCacheTTL 是服务归属缓存的时长。服务调整所属项目/环境是低频操作，
	// 付出一次短 TTL 的延迟生效即可，换来鉴权热路径上不再逐次回查 data-service。
	appScopeCacheTTL = time.Minute
)

// appScope 是服务在资源拓扑中的归属：业务、项目与所属环境。
type appScope struct {
	bizID     uint32
	projectID uint32
	envID     uint32
}

// newAppScopeCache 构造服务归属缓存。
func newAppScopeCache() gcache.Cache {
	return gcache.New(appScopeCacheSize).LRU().Expiration(appScopeCacheTTL).Build()
}

// newDefaultScopeCache 构造默认项目/环境缓存。
func newDefaultScopeCache() gcache.Cache {
	return gcache.New(appScopeCacheSize).LRU().Expiration(appScopeCacheTTL).Build()
}

// fillAppScope 为服务维度的资源补齐项目与环境归属。
//
// 服务挂在环境下，鉴权路径是 /biz,{biz}/project,{pid}/environment,{eid}/。
// 调用方（config-server / api-server 的各个 handler）只带 biz 与 app_id，
// 因此由鉴权侧统一回查服务的归属，避免每个 handler 各写一遍、也避免依赖客户端传入的 ID。
// 服务必须挂在具体项目与环境下，归属缺失时保持为空，由鉴权路径缺失退化为无权限，防止静默越权。
func (a *Auth) fillAppScope(kt *kit.Kit, resources []*meta.ResourceAttribute) error {
	pending := make([]*meta.ResourceAttribute, 0)
	ids := make(map[uint32]struct{})

	for _, res := range resources {
		if res == nil || res.Type != meta.App || !needsAppScope(res.Action) {
			continue
		}
		if res.ProjectID != 0 && res.EnvironmentID != 0 {
			continue
		}

		pending = append(pending, res)
		ids[res.ResourceID] = struct{}{}
	}

	if len(pending) == 0 {
		return nil
	}

	scopes := make(map[uint32]appScope, len(ids))
	for id := range ids {
		scope, err := a.appScopeOf(kt, id)
		if err != nil {
			return err
		}
		scopes[id] = scope
	}

	for _, res := range pending {
		scope := scopes[res.ResourceID]

		// 业务层级以服务自身归属为准：调用方传入的业务 ID 只用于前置的业务访问校验，
		// 不能决定服务鉴权路径，否则传错业务就能用另一个业务的授权访问该服务。
		if scope.bizID != 0 && res.BizID != 0 && res.BizID != scope.bizID {
			logs.Warnf("app %d belongs to biz %d, but resource declares biz %d, use the app's own biz, rid: %s",
				res.ResourceID, scope.bizID, res.BizID, kt.Rid)
		}
		if scope.bizID != 0 {
			res.BizID = scope.bizID
		}

		if res.ProjectID == 0 {
			res.ProjectID = scope.projectID
		}
		if res.EnvironmentID == 0 {
			res.EnvironmentID = scope.envID
		}
		if res.ProjectID == 0 || res.EnvironmentID == 0 {
			logs.Warnf("app %d is not bound to a project/environment, authorization will be denied, rid: %s",
				res.ResourceID, kt.Rid)
		}
	}

	return nil
}

// fillResourceScope 为项目、环境与服务维度的资源补齐项目/环境归属。
//
// 权限检查这类不带业务上下文的调用方常常不传归属（0 值），若直接拼路径会得到空串，
// 权限中心按无拓扑处理，结果就是"已经授权了却一直判无权限"。
// 因此统一在鉴权入口解析实例的真实归属，未指定实例时再补默认归属，供 V3/V4 共用。
func (a *Auth) fillResourceScope(kt *kit.Kit, resources []*meta.ResourceAttribute) error {
	if err := a.fillAppScope(kt, resources); err != nil {
		return err
	}

	if err := a.fillEnvironmentScope(kt, resources); err != nil {
		return err
	}

	return a.fillProjectScope(kt, resources)
}

// fillEnvironmentScope 补齐「按项目或环境鉴权」的归属。
//
// 覆盖两类资源：环境操作（view/update/delete/create）与在环境下创建服务（app_create，
// 授权维度是目标环境）。它们的鉴权路径都需要 project_id 作祖先，缺少环境实例时按默认环境补齐。
// 已有环境实例时父级一律以回查到的真实路径为准，请求里带的业务与项目只当作线索、不参与拼路径。
func (a *Auth) fillEnvironmentScope(kt *kit.Kit, resources []*meta.ResourceAttribute) error {
	envScopes, err := a.lookupEnvironmentScopes(kt, resources)
	if err != nil {
		return err
	}

	for _, res := range resources {
		if res == nil {
			continue
		}

		isEnv := res.Type == meta.Environment
		// 服务创建按目标环境授权，与环境操作共用同一套归属
		isAppCreate := res.Type == meta.App && res.Action == meta.Create
		if !isEnv && !isAppCreate {
			continue
		}

		hasInstance := (!isEnv || res.Action != meta.Create) && res.ResourceID != 0
		if hasInstance {
			applyEnvironmentParent(kt, res, envScopes)
		} else if err := a.fillMissingEnvironmentScope(kt, res); err != nil {
			return err
		}

		// 服务创建按环境维度鉴权，环境 ID 即资源实例 ID
		if isAppCreate && res.EnvironmentID == 0 {
			res.EnvironmentID = res.ResourceID
		}
	}

	return nil
}

// applyEnvironmentParent 用回查到的真实父级覆盖请求声明的业务与项目。
//
// 权限中心不反查拓扑，鉴权路径完全由这里给出的祖先决定，沿用请求里的 project_id
// 会让拥有项目 A 权限的调用方拿项目 B 的环境 ID 拼出项目 A 的路径。
// 回查不到父级时清空归属：路径拼不出来，鉴权退化为无权限，而不是拿请求里的值冒充祖先。
func applyEnvironmentParent(kt *kit.Kit, res *meta.ResourceAttribute, envScopes map[uint32]appScope) {
	scope, ok := envScopes[res.ResourceID]
	if !ok {
		if res.BizID != 0 || res.ProjectID != 0 {
			logs.Warnf("environment %d has no resolvable parent, drop biz %d/project %d declared by the request, "+
				"authorization will be denied, rid: %s", res.ResourceID, res.BizID, res.ProjectID, kt.Rid)
		}

		res.BizID = 0
		res.ProjectID = 0

		return
	}

	if (res.BizID != 0 && res.BizID != scope.bizID) ||
		(res.ProjectID != 0 && res.ProjectID != scope.projectID) {
		logs.Warnf("environment %d belongs to biz %d/project %d, but resource declares %d/%d, "+
			"use the environment's own parents, rid: %s",
			res.ResourceID, scope.bizID, scope.projectID, res.BizID, res.ProjectID, kt.Rid)
	}

	res.BizID = scope.bizID
	res.ProjectID = scope.projectID
}

// fillMissingEnvironmentScope 处理没有可回查实例的情况：创建环境时资源实例就是所属项目，
// 项目或环境缺失时按业务默认归属补齐。没有业务 ID 就查不到默认归属，保持为空，
// 由鉴权路径缺失退化为无权限。
func (a *Auth) fillMissingEnvironmentScope(kt *kit.Kit, res *meta.ResourceAttribute) error {
	isEnvCreate := res.Type == meta.Environment && res.Action == meta.Create
	// 创建环境时尚无环境实例，资源实例就是所属项目，两个字段互为等价
	if isEnvCreate {
		if res.ProjectID == 0 {
			res.ProjectID = res.ResourceID
		}
		if res.ResourceID == 0 {
			res.ResourceID = res.ProjectID
		}
	}

	if (res.ProjectID != 0 && res.ResourceID != 0) || res.BizID == 0 {
		return nil
	}

	scope, err := a.defaultScopeOf(kt, res.BizID)
	if err != nil {
		return err
	}

	if res.ProjectID == 0 {
		res.ProjectID = scope.projectID
	}
	if res.ResourceID == 0 {
		res.ResourceID = scope.envID
	}

	if res.ProjectID == 0 || res.ResourceID == 0 {
		logs.Warnf("resource %s/%s has no project/environment scope, authorization will be denied, rid: %s",
			res.Type, res.Action, kt.Rid)
	}

	// 创建环境的资源实例始终是所属项目，环境 ID 在这里没有意义
	if isEnvCreate && res.ProjectID != 0 {
		res.ResourceID = res.ProjectID
	}

	return nil
}

// lookupEnvironmentScopes 批量回查环境实例的真实父级，避免逐项回查。
func (a *Auth) lookupEnvironmentScopes(kt *kit.Kit, resources []*meta.ResourceAttribute) (map[uint32]appScope, error) {
	// 已存在环境的父项目必须取自该环境的真实路径，不能用业务默认项目替代，也不能信请求里的 project_id：
	// 权限中心不反查拓扑，路径完全由调用方给出的祖先决定，因此业务与项目都已传入时同样要回查。
	envIDs := make([]uint32, 0)
	seenEnvIDs := make(map[uint32]struct{})
	for _, res := range resources {
		if res == nil || res.Action == meta.SkipAction {
			continue
		}
		isExistingEnvironment := res.Type == meta.Environment && res.Action != meta.Create
		isAppCreate := res.Type == meta.App && res.Action == meta.Create
		if (!isExistingEnvironment && !isAppCreate) || res.ResourceID == 0 {
			continue
		}
		if _, ok := seenEnvIDs[res.ResourceID]; ok {
			continue
		}
		seenEnvIDs[res.ResourceID] = struct{}{}
		envIDs = append(envIDs, res.ResourceID)
	}

	envScopes := make(map[uint32]appScope, len(envIDs))
	if len(envIDs) > 0 {
		ids := make([]string, 0, len(envIDs))
		for _, id := range envIDs {
			ids = append(ids, strconv.FormatUint(uint64(id), 10))
		}
		resp, err := a.ds.FetchInstanceInfo(kt.RpcCtx(), &pbds.FetchInstanceInfoReq{
			ResourceType: string(meta.Environment),
			Ids:          ids,
		})
		if err != nil {
			logs.Errorf("fetch environment scope for iam authorization failed, ids: %v, err: %v, rid: %s",
				ids, err, kt.Rid)
			return nil, err
		}
		for _, detail := range resp.GetDetails() {
			envID, err := strconv.ParseUint(detail.GetId(), 10, 32)
			if err != nil {
				continue
			}
			for _, path := range detail.GetPath() {
				bizID, projectID, ok := parseEnvironmentIAMPath(path)
				if !ok {
					continue
				}
				envScopes[uint32(envID)] = appScope{bizID: bizID, projectID: projectID, envID: uint32(envID)}
				break
			}
		}
	}
	return envScopes, nil
}

// parseEnvironmentIAMPath 从环境资源的 IAM path 读取真实业务与项目祖先。
func parseEnvironmentIAMPath(path string) (uint32, uint32, bool) {
	var bizID, projectID uint32
	for _, part := range strings.Split(strings.Trim(path, "/"), "/") {
		typeAndID := strings.SplitN(part, ",", 2)
		if len(typeAndID) != 2 {
			continue
		}
		id, err := strconv.ParseUint(typeAndID[1], 10, 32)
		if err != nil {
			continue
		}
		switch typeAndID[0] {
		case string(meta.Biz):
			bizID = uint32(id)
		case string(meta.Project):
			projectID = uint32(id)
		}
	}
	return bizID, projectID, bizID != 0 && projectID != 0
}

// fillProjectScope 让项目维度鉴权资源的实例 ID 与项目 ID 保持一致。
//
// 项目按实例授权，resource_id 与 project_id 指向同一个项目，任一缺失时互补；
// 两者都为空时按默认项目补齐。创建与列表不针对具体项目实例，不在此处理。
func (a *Auth) fillProjectScope(kt *kit.Kit, resources []*meta.ResourceAttribute) error {
	for _, res := range resources {
		if res == nil || res.Type != meta.Project {
			continue
		}

		if res.Action == meta.Create || res.Action == meta.Find {
			continue
		}

		if res.ProjectID == 0 {
			res.ProjectID = res.ResourceID
		}
		if res.ResourceID == 0 {
			res.ResourceID = res.ProjectID
		}
		if res.ResourceID != 0 {
			continue
		}

		if res.BizID == 0 {
			continue
		}

		scope, err := a.defaultScopeOf(kt, res.BizID)
		if err != nil {
			return err
		}

		res.ProjectID = scope.projectID
		res.ResourceID = scope.projectID
		if res.ProjectID == 0 {
			logs.Warnf("project resource has no project scope, authorization will be denied, rid: %s", kt.Rid)
		}
	}

	return nil
}

// needsAppScope 判断该操作是否按服务实例鉴权。
// 服务列表按业务维度授权，不针对具体实例，不需要归属信息；
// 服务创建按目标环境鉴权，由 fillEnvironmentScope 处理。
func needsAppScope(action meta.Action) bool {
	switch action {
	case meta.View, meta.Update, meta.Delete, meta.GenerateRelease, meta.Publish:
		return true
	default:
		return false
	}
}

// appScopeOf 返回服务的项目与环境归属，结果带缓存。
// 缓存键必须带租户：多租户下服务 ID 只在本租户内唯一。
func (a *Auth) appScopeOf(kt *kit.Kit, appID uint32) (appScope, error) {
	key := kt.TenantID + "|" + strconv.FormatUint(uint64(appID), 10)

	if a.appScopes != nil {
		if val, err := a.appScopes.GetIFPresent(key); err == nil {
			if scope, ok := val.(appScope); ok {
				return scope, nil
			}
		}
	}

	app, err := a.ds.GetAppByID(kt.RpcCtx(), &pbds.GetAppByIDReq{AppId: appID})
	if err != nil {
		logs.Errorf("get app %d for iam authorization failed, err: %v, rid: %s", appID, err, kt.Rid)

		return appScope{}, err
	}

	scope := appScope{
		bizID:     app.GetBizId(),
		projectID: app.GetProjectId(),
		envID:     app.GetEnvId(),
	}

	// 归属缺失（0 值）不缓存：历史服务还没绑定项目/环境时若把空归属固化下来，
	// 整段 TTL 内该服务的鉴权都会按空路径判定为无权限，补上归属后也要等过期才恢复。
	if a.appScopes != nil && scope.projectID != 0 && scope.envID != 0 {
		_ = a.appScopes.Set(key, scope)
	}

	return scope, nil
}

// defaultScopeOf 返回业务下的默认项目与环境，结果带缓存。
// 复用 EnsureDefaultProjectEnv：它和请求链上给 kt 赋默认值是同一个来源，
// 默认项目/环境缺失时按平台既有语义补齐。
func (a *Auth) defaultScopeOf(kt *kit.Kit, bizID uint32) (appScope, error) {
	key := kt.TenantID + "|" + strconv.FormatUint(uint64(bizID), 10)

	if a.defaultScopes != nil {
		if val, err := a.defaultScopes.GetIFPresent(key); err == nil {
			if scope, ok := val.(appScope); ok {
				return scope, nil
			}
		}
	}

	resp, err := a.ds.EnsureDefaultProjectEnv(kt.RpcCtx(), &pbds.EnsureDefaultProjectEnvReq{BizId: bizID})
	if err != nil {
		logs.Errorf("ensure default project/env for biz %d failed, err: %v, rid: %s", bizID, err, kt.Rid)

		return appScope{}, err
	}

	scope := appScope{bizID: bizID, projectID: resp.GetProjectId(), envID: resp.GetEnvId()}
	// 归属缺失（0 值）不缓存：一次异常若被固化，整段 TTL 内该业务的项目/环境维度鉴权
	// 都会按空路径判定为无权限，表现为批量 403。
	if a.defaultScopes != nil && scope.projectID != 0 && scope.envID != 0 {
		_ = a.defaultScopes.Set(key, scope)
	}

	return scope, nil
}
