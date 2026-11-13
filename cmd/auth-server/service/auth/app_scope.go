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

// fillAppScope 以服务自身的真实归属覆盖鉴权路径上的业务、项目与环境。
//
// 服务挂在环境下，鉴权路径是 /biz,{biz}/project,{pid}/environment,{eid}/。
// 调用方（config-server / api-server 的各个 handler）只带 biz 与 app_id，
// 因此由鉴权侧统一回查服务的归属，避免每个 handler 各写一遍。
// 请求里携带的 project_id/environment_id 一律不采信：CheckPermission 这类对外接口的
// resources 由调用方提交，若沿用请求里的归属，把服务 X 的 project/env 填成自己有权限的
// 环境 E 就能拼出 E 的路径拿到 is_allowed=true，与环境侧 applyEnvironmentParent 的
// 「父级一律以回查为准」也不一致。
// 服务查不到归属时开启项目/环境维度校验一律拒绝（业务也置空，避免业务授权经路径继承命中），
// 关闭时退化为业务维度；服务存在但还没绑定项目/环境时保留业务层级，只让业务维度的授权命中。
func (a *Auth) fillAppScope(kt *kit.Kit, resources []*meta.ResourceAttribute) error {
	pending := make([]*meta.ResourceAttribute, 0)
	ids := make([]uint32, 0)
	seen := make(map[uint32]struct{})

	for _, res := range resources {
		// ResourceID 为 0 表示没有服务实例（例如克隆只按业务维度校验 app_edit），
		// 没有可回查的归属，保持调用方声明的业务。
		if res == nil || res.Type != meta.App || !needsAppScope(res.Action) || res.ResourceID == 0 {
			continue
		}

		pending = append(pending, res)
		if _, ok := seen[res.ResourceID]; ok {
			continue
		}
		seen[res.ResourceID] = struct{}{}
		ids = append(ids, res.ResourceID)
	}

	if len(pending) == 0 {
		return nil
	}

	scopes, err := a.lookupAppScopes(kt, ids)
	if err != nil {
		return err
	}

	for _, res := range pending {
		scope, ok := scopes[res.ResourceID]
		if !ok {
			logs.Warnf("app %d has no resolvable scope, drop biz %d/project %d/environment %d declared by the "+
				"request, rid: %s", res.ResourceID, res.BizID, res.ProjectID, res.EnvironmentID, kt.Rid)
			res.ProjectID = 0
			res.EnvironmentID = 0
			// 开启项目/环境维度校验时必须真的拒绝：只清空项目/环境会拼出
			// /biz,{请求的biz}/project,0/environment,0/，业务维度的授权仍经路径继承命中，
			// 等于用一个不存在的服务 ID 换到 is_allowed=true。清空业务后路径变成
			// /biz,0/project,0/environment,0/，任何授权都命中不到。
			// 关闭时没有项目/环境模型，保留调用方声明的业务，退化为业务维度判定。
			if a.projectEnvAuth {
				res.BizID = 0
			}

			continue
		}

		// 业务层级同样以服务自身归属为准：调用方传入的业务 ID 只用于前置的业务访问校验，
		// 不能决定服务鉴权路径，否则传错业务就能用另一个业务的授权访问该服务。
		if res.BizID != 0 && res.BizID != scope.bizID {
			logs.Warnf("app %d belongs to biz %d, but resource declares biz %d, use the app's own scope, rid: %s",
				res.ResourceID, scope.bizID, res.BizID, kt.Rid)
		}

		// 项目/环境只在开启项目/环境维度校验时参与路径，关闭时声明值与实际归属不一致属正常情况。
		if a.projectEnvAuth && ((res.ProjectID != 0 && res.ProjectID != scope.projectID) ||
			(res.EnvironmentID != 0 && res.EnvironmentID != scope.envID)) {
			logs.Warnf("app %d belongs to project %d/environment %d, but resource declares %d/%d, "+
				"use the app's own scope, rid: %s",
				res.ResourceID, scope.projectID, scope.envID, res.ProjectID, res.EnvironmentID, kt.Rid)
		}

		res.BizID = scope.bizID
		res.ProjectID = scope.projectID
		res.EnvironmentID = scope.envID

		// 关闭项目/环境维度校验时服务只按业务一级判定，没有归属是正常情况，不再告警。
		// 开启时归属缺失仍保留业务层级：历史服务还没回填项目/环境，按「业务 → 项目 → 环境」拼出的
		// /biz,{biz}/project,0/environment,0/ 只让业务维度的授权命中，项目/环境维度因 ID 对不上而不命中，
		// 与 adaptor.appScoped 的约定一致。
		if a.projectEnvAuth && (res.ProjectID == 0 || res.EnvironmentID == 0) {
			logs.Warnf("app %d is not bound to a project/environment, only biz-level authorization can match, rid: %s",
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
//
// 关闭项目/环境维度开关时没有项目/环境模型，所有资源都按业务维度处理：
// 只归一化服务的业务归属，项目/环境与服务创建的父级都不回查，见 needsEnvironmentScope。
func (a *Auth) fillResourceScope(kt *kit.Kit, resources []*meta.ResourceAttribute) error {
	if err := a.fillAppScope(kt, resources); err != nil {
		return err
	}

	if err := a.fillEnvironmentScope(kt, resources); err != nil {
		return err
	}

	if !a.projectEnvAuth {
		// 项目资源本身不进入鉴权，补了默认项目也不会被使用。
		return nil
	}

	if err := a.fillProjectScope(kt, resources); err != nil {
		return err
	}

	// 服务归属已在 fillAppScope 回查，项目/环境查看必须跟着服务的真实归属走，
	// 否则调用方可以用自己有权限的项目 ID 放行对其它项目服务的操作。
	alignServiceScopedViews(kt, resources)

	return nil
}

// needsEnvironmentScope 判断资源是否需要回查环境的真实父级（业务与项目）。
//
// 只有开启项目/环境维度校验时才需要：此时项目和环境的资源类型都已注册，
// 服务创建按目标环境授权，路径为 /biz,{biz}/project,{pid}/，项目祖先不能采信调用方提交的 project_id；
// 环境资源本身也按实例校验。
// 关闭开关时没有项目/环境模型，所有资源（含服务创建）都退回业务维度，没有环境实例可回查。
func (a *Auth) needsEnvironmentScope(res *meta.ResourceAttribute) bool {
	if !a.projectEnvAuth || res == nil || res.Action == meta.SkipAction {
		return false
	}

	switch res.Type {
	case meta.Environment:
		return true
	case meta.App:
		// 服务只有创建动作按环境维度鉴权
		return res.Action == meta.Create
	default:
		return false
	}
}

// fillEnvironmentScope 补齐「按项目或环境鉴权」的归属，只在开启项目/环境维度校验时生效。
//
// 覆盖两类资源：在环境下创建服务（app_create，授权维度是目标环境）与环境操作
// （view/update/delete/create）。它们的鉴权路径都需要 project_id 作祖先，缺少环境实例时按默认环境补齐。
// 已有环境实例时父级一律以回查到的真实路径为准，请求里带的业务与项目只当作线索、不参与拼路径。
// 关闭开关时没有项目/环境模型，所有资源都按业务维度处理，这里不做任何回查。
func (a *Auth) fillEnvironmentScope(kt *kit.Kit, resources []*meta.ResourceAttribute) error {
	if !a.projectEnvAuth {
		return nil
	}

	resolveAppCreateTargetEnv(kt, resources)
	resolveEnvironmentInstance(kt, resources)

	envScopes, err := a.lookupEnvironmentScopes(kt, resources)
	if err != nil {
		return err
	}

	for _, res := range resources {
		if !a.needsEnvironmentScope(res) {
			continue
		}

		isEnv := res.Type == meta.Environment
		// 服务创建按目标环境授权，与环境操作共用同一套归属
		isAppCreate := res.Type == meta.App && res.Action == meta.Create

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

// resolveAppCreateTargetEnv 用调用方声明的 environment_id 补齐服务创建的目标环境实例。
//
// 服务创建按目标环境授权，资源实例 ID 即目标环境 ID。config-server 创建服务时两个字段都带，
// 但权限查询（包括前端「创建服务」按钮调用的 /api/v1/auth/iam/permission/check）习惯只用
// environment_id 声明目标环境。若按「未指定目标环境」处理，实例会被补成业务默认环境：
// 用户在默认环境有创建权限、在声明的环境没有权限时仍返回 is_allowed=true，
// 申请链接也会指向默认环境。因此先以 environment_id 作为实例，再按该环境的真实父级拼路径。
func resolveAppCreateTargetEnv(kt *kit.Kit, resources []*meta.ResourceAttribute) {
	for _, res := range resources {
		if res == nil || res.Type != meta.App || res.Action != meta.Create || res.ResourceID != 0 {
			continue
		}

		if res.EnvironmentID == 0 {
			continue
		}

		logs.Infof("app create declares environment %d without resource instance, use it as the target "+
			"environment, rid: %s", res.EnvironmentID, kt.Rid)
		res.ResourceID = res.EnvironmentID
	}
}

// resolveEnvironmentInstance 用调用方声明的 environment_id 补齐环境资源的实例。
//
// 环境按环境实例鉴权，资源实例 ID 即环境 ID。权限查询（包括前端切换环境、环境列表操作调用的
// /api/v1/auth/iam/permission/check）习惯只用 environment_id 声明目标环境，与 app_create
// 一致。若按「未指定实例」处理，实例会被补成业务默认环境：切换任何环境都只校验默认环境，
// 用户在不该访问的环境上也可能被判为有权限，申请链接里的资源名也始终是默认环境（Default）。
// 创建环境时资源实例是所属项目，此时 environment_id 不是实例，保持原有补齐逻辑。
func resolveEnvironmentInstance(kt *kit.Kit, resources []*meta.ResourceAttribute) {
	for _, res := range resources {
		if res == nil || res.Type != meta.Environment || res.Action == meta.Create ||
			res.Action == meta.SkipAction || res.ResourceID != 0 {
			continue
		}

		if res.EnvironmentID == 0 {
			continue
		}

		logs.Infof("environment %s declares environment %d without resource instance, use it as the "+
			"environment instance, rid: %s", res.Action, res.EnvironmentID, kt.Rid)
		res.ResourceID = res.EnvironmentID
	}
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
		if !a.needsEnvironmentScope(res) || res.ResourceID == 0 {
			continue
		}
		// 创建环境时资源实例是所属项目，没有可回查的环境实例。
		if res.Type == meta.Environment && res.Action == meta.Create {
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
	ids := parseIAMPathIDs(path)
	bizID, projectID := ids[string(meta.Biz)], ids[string(meta.Project)]

	return bizID, projectID, bizID != 0 && projectID != 0
}

// parseAppIAMPath 从服务资源的 IAM path 读取真实归属：
// /biz,{bizID}/project,{projectID}/environment,{envID}/。
// 归属缺失时对应字段为 0，由调用方按无权限处理。
func parseAppIAMPath(path string) (appScope, bool) {
	ids := parseIAMPathIDs(path)
	bizID := ids[string(meta.Biz)]
	if bizID == 0 {
		return appScope{}, false
	}

	return appScope{
		bizID:     bizID,
		projectID: ids[string(meta.Project)],
		envID:     ids[string(meta.Environment)],
	}, true
}

// parseIAMPathIDs 把 /biz,1/project,2/environment,3/ 解析成资源类型到 ID 的映射。
func parseIAMPathIDs(path string) map[string]uint32 {
	ids := make(map[string]uint32)
	for _, part := range strings.Split(strings.Trim(path, "/"), "/") {
		typeAndID := strings.SplitN(part, ",", 2)
		if len(typeAndID) != 2 {
			continue
		}

		id, err := strconv.ParseUint(typeAndID[1], 10, 32)
		if err != nil {
			continue
		}
		ids[typeAndID[0]] = uint32(id)
	}

	return ids
}

// needsProjectScope 判断项目维度资源是否需要回查真实业务归属。
//
// 创建与列表不针对具体项目实例；ResourceID 为 0 时没有实例可回查，由默认项目补齐。
func (a *Auth) needsProjectScope(res *meta.ResourceAttribute) bool {
	if res == nil || res.Type != meta.Project || !a.projectEnvAuth {
		return false
	}
	if res.Action == meta.Create || res.Action == meta.Find {
		return false
	}

	return res.ResourceID != 0
}

// fillProjectScope 补齐项目维度鉴权资源的实例与业务归属。
//
// 项目按实例授权，resource_id 与 project_id 指向同一个项目，任一缺失时互补；
// 已有实例时业务祖先一律以回查到的真实路径为准，请求里带的 biz_id 只当作线索：
// 权限中心不反查拓扑，鉴权路径完全由这里给出的祖先决定，沿用请求里的 biz_id
// 会让拥有业务 A 权限的调用方拿业务 B 的项目实例拼出业务 A 的路径。
// 没有实例时（如对外接口不传项目）按业务的默认项目补齐，两者都为空则退化为无权限。
func (a *Auth) fillProjectScope(kt *kit.Kit, resources []*meta.ResourceAttribute) error {
	projectScopes, err := a.lookupProjectScopes(kt, resources)
	if err != nil {
		return err
	}

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
			applyProjectParent(kt, res, projectScopes)

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

// lookupProjectScopes 批量回查项目实例的真实业务归属，避免逐项回查。
func (a *Auth) lookupProjectScopes(kt *kit.Kit, resources []*meta.ResourceAttribute) (map[uint32]uint32, error) {
	projectIDs := make([]uint32, 0)
	seen := make(map[uint32]struct{})
	for _, res := range resources {
		if !a.needsProjectScope(res) {
			continue
		}
		if _, ok := seen[res.ResourceID]; ok {
			continue
		}

		seen[res.ResourceID] = struct{}{}
		projectIDs = append(projectIDs, res.ResourceID)
	}

	scopes := make(map[uint32]uint32, len(projectIDs))
	if len(projectIDs) == 0 {
		return scopes, nil
	}

	ids := make([]string, 0, len(projectIDs))
	for _, id := range projectIDs {
		ids = append(ids, strconv.FormatUint(uint64(id), 10))
	}

	resp, err := a.ds.FetchInstanceInfo(kt.RpcCtx(), &pbds.FetchInstanceInfoReq{
		ResourceType: string(meta.Project),
		Ids:          ids,
	})
	if err != nil {
		logs.Errorf("fetch project scope for iam authorization failed, ids: %v, err: %v, rid: %s",
			ids, err, kt.Rid)

		return nil, err
	}

	for _, detail := range resp.GetDetails() {
		projectID, err := strconv.ParseUint(detail.GetId(), 10, 32)
		if err != nil {
			continue
		}

		for _, path := range detail.GetPath() {
			bizID, ok := parseProjectIAMPath(path)
			if !ok {
				continue
			}

			scopes[uint32(projectID)] = bizID
			break
		}
	}

	return scopes, nil
}

// applyProjectParent 用回查到的真实业务覆盖请求声明的业务。
// 回查不到归属时清空业务：路径拼不出来，鉴权退化为无权限，而不是拿请求里的值冒充祖先。
func applyProjectParent(kt *kit.Kit, res *meta.ResourceAttribute, projectScopes map[uint32]uint32) {
	bizID, ok := projectScopes[res.ResourceID]
	res.ProjectID = res.ResourceID

	if !ok {
		if res.BizID != 0 {
			logs.Warnf("project %d has no resolvable parent, drop biz %d declared by the request, "+
				"authorization will be denied, rid: %s", res.ResourceID, res.BizID, kt.Rid)
		}
		res.BizID = 0

		return
	}

	if res.BizID != 0 && res.BizID != bizID {
		logs.Warnf("project %d belongs to biz %d, but resource declares %d, "+
			"use the project's own parent, rid: %s", res.ResourceID, bizID, res.BizID, kt.Rid)
	}

	res.BizID = bizID
}

// parseProjectIAMPath 从项目资源的 IAM path 读取真实业务祖先：/biz,{bizID}/project,{projectID}/。
func parseProjectIAMPath(path string) (uint32, bool) {
	ids := parseIAMPathIDs(path)
	bizID := ids[string(meta.Biz)]

	return bizID, bizID != 0
}

// alignServiceScopedViews 让同一批次里的项目查看、环境查看以服务的真实归属为准。
//
// 服务内资源的接口要求四项同时通过，但项目、环境两项的实例由调用方传入。
// 沿用调用方的 ID 时，拥有项目 A 查看权限的调用方配上项目 B 的服务 ID，
// 就能用 A 的 project_view 放行对 B 的配置操作。服务归属已经在 fillAppScope 回查，
// 这里按服务的真实项目/环境改写这两项资源，与「父级一律以回查为准」保持一致。
// 同一批次出现归属不同的多个服务时取第一个服务的归属并告警：服务内资源的鉴权主体
// 仍是服务本身，项目/环境维度只做附加收敛，不因批次混合而放开。
func alignServiceScopedViews(kt *kit.Kit, resources []*meta.ResourceAttribute) {
	var scope appScope
	resolved := false

	for _, res := range resources {
		if res == nil || res.Type != meta.App || res.ProjectID == 0 || res.EnvironmentID == 0 {
			continue
		}

		if !resolved {
			scope = appScope{bizID: res.BizID, projectID: res.ProjectID, envID: res.EnvironmentID}
			resolved = true

			continue
		}

		if res.ProjectID != scope.projectID || res.EnvironmentID != scope.envID {
			logs.Warnf("apps in one authorization batch belong to different projects/environments "+
				"(%d/%d and %d/%d), use the first one, rid: %s",
				scope.projectID, scope.envID, res.ProjectID, res.EnvironmentID, kt.Rid)
		}
	}

	if !resolved {
		return
	}

	for _, res := range resources {
		if res == nil || res.Action != meta.View {
			continue
		}

		switch res.Type {
		case meta.Project:
			replaceViewScope(kt, res, scope, scope.projectID)
		case meta.Environment:
			replaceViewScope(kt, res, scope, scope.envID)
		}
	}
}

// replaceViewScope 用服务的真实归属覆盖项目/环境查看资源，并记录调用方传入的差异。
func replaceViewScope(kt *kit.Kit, res *meta.ResourceAttribute, scope appScope, resourceID uint32) {
	if res.ResourceID != resourceID {
		logs.Warnf("%s view resource declares %d, but the app belongs to project %d/environment %d, "+
			"use the app's own scope, rid: %s", res.Type, res.ResourceID, scope.projectID, scope.envID, kt.Rid)
	}

	res.ResourceID = resourceID
	res.ProjectID = scope.projectID
	res.BizID = scope.bizID
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

// lookupAppScopes 批量回查服务的真实归属，命中缓存的条目不再请求 data-service。
// 缓存键必须带租户：多租户下服务 ID 只在本租户内唯一。
// 查不到的服务不出现在返回结果里，由调用方按「归属缺失」处理，避免一个已删除的服务
// 让整批鉴权直接报错。
func (a *Auth) lookupAppScopes(kt *kit.Kit, appIDs []uint32) (map[uint32]appScope, error) {
	scopes := make(map[uint32]appScope, len(appIDs))
	missed := make([]string, 0, len(appIDs))

	for _, appID := range appIDs {
		key := kt.TenantID + "|" + strconv.FormatUint(uint64(appID), 10)
		if a.appScopes != nil {
			if val, err := a.appScopes.GetIFPresent(key); err == nil {
				if scope, ok := val.(appScope); ok {
					scopes[appID] = scope

					continue
				}
			}
		}

		missed = append(missed, strconv.FormatUint(uint64(appID), 10))
	}

	if len(missed) == 0 {
		return scopes, nil
	}

	// 一次批量回查，避免逐服务串行 RPC；服务路径已由 data-service 拼成三级拓扑。
	resp, err := a.ds.FetchInstanceInfo(kt.RpcCtx(), &pbds.FetchInstanceInfoReq{
		ResourceType: string(meta.App),
		Ids:          missed,
	})
	if err != nil {
		logs.Errorf("fetch app scope for iam authorization failed, ids: %v, err: %v, rid: %s",
			missed, err, kt.Rid)

		return nil, err
	}

	for _, detail := range resp.GetDetails() {
		appID, err := strconv.ParseUint(detail.GetId(), 10, 32)
		if err != nil {
			continue
		}

		for _, path := range detail.GetPath() {
			scope, ok := parseAppIAMPath(path)
			if !ok {
				continue
			}

			scopes[uint32(appID)] = scope
			a.cacheAppScope(kt, uint32(appID), scope)

			break
		}
	}

	return scopes, nil
}

// cacheAppScope 缓存服务归属。
// 归属缺失（0 值）不缓存：历史服务还没绑定项目/环境时若把空归属固化下来，
// 整段 TTL 内该服务的鉴权都会按空路径判定为无权限，补上归属后也要等过期才恢复。
func (a *Auth) cacheAppScope(kt *kit.Kit, appID uint32, scope appScope) {
	if a.appScopes == nil || scope.projectID == 0 || scope.envID == 0 {
		return
	}

	key := kt.TenantID + "|" + strconv.FormatUint(uint64(appID), 10)
	_ = a.appScopes.Set(key, scope)
}

// defaultScopeOf 返回业务下的默认项目与环境，结果带缓存。
//
// 这里必须只读：fillResourceScope 在鉴权判定之前执行，而 CheckPermission 等入口的 biz_id
// 完全由调用方提交。若改成补建默认项目/环境（EnsureDefaultProjectEnv），任意登录用户
// 都能在任意业务 ID 下写库。因此查询不到时保持 0，由鉴权路径缺失退化为无权限。
func (a *Auth) defaultScopeOf(kt *kit.Kit, bizID uint32) (appScope, error) {
	key := kt.TenantID + "|" + strconv.FormatUint(uint64(bizID), 10)

	if a.defaultScopes != nil {
		if val, err := a.defaultScopes.GetIFPresent(key); err == nil {
			if scope, ok := val.(appScope); ok {
				return scope, nil
			}
		}
	}

	resp, err := a.ds.GetDefaultProjectEnv(kt.RpcCtx(), &pbds.GetDefaultProjectEnvReq{BizId: bizID})
	if err != nil {
		logs.Errorf("get default project/env for biz %d failed, err: %v, rid: %s", bizID, err, kt.Rid)

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
