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

// Package adaptor maps BSCP's own permission model onto IAM V4 actions and resources.
package adaptor

import (
	"fmt"
	"strconv"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/model"
)

// Options 控制资源映射时启用的权限维度与拓扑形态。
type Options struct {
	// ProjectEnvironmentAuth 是否启用项目与环境维度的权限校验。
	// 关闭时只有这两类资源本身被跳过；服务相关操作仍然校验，
	// 拓扑路径也保持与注册父链一致的完整形态。
	ProjectEnvironmentAuth bool
	// Topology IAM 侧注册的资源拓扑，决定资源路径的层级。
	// 零值按新模型（三级路径）处理，避免调用方漏传时把路径裁成旧模型。
	Topology meta.ResourceTopology
}

// defaultOptions 返回默认开启项目/环境维度校验、按新拓扑映射的选项。
func defaultOptions() Options {
	return Options{ProjectEnvironmentAuth: true, Topology: meta.TopologyHierarchical}
}

// Mapped 是 meta 域模型映射到 IAM V4 之后的结果。
type Mapped struct {
	// Skip 为 true 表示该资源在 BSCP 侧不做权限控制，直接放行，不需要请求权限中心。
	Skip bool
	// ActionID IAM V4 的操作 ID
	ActionID string
	// ResourceType 授权维度，取 model.ResourceTypeBiz、model.ResourceTypeApp、
	// model.ResourceTypeProject 或 model.ResourceTypeEnvironment；为空表示该操作与资源无关
	ResourceType string
	// ResourceID 资源实例 ID
	ResourceID string
	// BizID 所属业务 ID。非根资源都用它生成拓扑路径，让按业务的授权能覆盖到下层资源
	BizID string
	// ProjectID 所属项目 ID。环境与服务用它补出 /biz,{bizID}/project,{projectID}/ 两级路径
	ProjectID string
	// EnvironmentID 所属环境 ID。服务用它补出三级路径的最后一级
	EnvironmentID string
	// legacyBizPath 为 true 时非根资源的拓扑路径只声明业务一层。
	// 只在旧拓扑（业务 → 服务）下使用，新拓扑必须给出完整祖先链。
	legacyBizPath bool
}

// skipped 构造一个直接放行的映射结果。
func skipped() *Mapped {
	return &Mapped{Skip: true}
}

// AuthResource 构建 IAM V4 鉴权接口用的资源。
// auth-by-resources 接口的请求体形如：
//
//	{
//	  "subject": {"type": "user", "id": "admin"},
//	  "action_id": "environment_view",
//	  "resources": [
//	    {"id": "401", "attributes": {"_bk_iam_path_": "/biz,2/project,3/"}}
//	  ]
//	}
//
// attributes 里的 _bk_iam_path_ 声明资源的归属拓扑。非根资源必须带上，
// 否则按上层授权的用户会被判为无权限——权限中心不会反查拓扑。
// 无关资源类型的操作返回 nil，由调用方省略 resource 字段。
func (m *Mapped) AuthResource() *client.AuthResource {
	if m.ResourceType == "" {
		return nil
	}

	if m.ResourceType == model.ResourceTypeBiz {
		// biz 是拓扑根节点，没有祖先，不需要 attributes。
		res := client.NewAuthResource(m.ResourceID, "")

		return &res
	}

	res := client.NewAuthResource(m.ResourceID, m.iamPath())

	return &res
}

// iamPath 按资源类型拼出从根到父级的拓扑路径。
// 项目挂业务，环境挂项目，服务挂环境（三级路径），路径必须与权限中心注册的祖先链一致。
func (m *Mapped) iamPath() string {
	// 旧拓扑下资源直接挂在业务下，只能声明业务一层。
	if m.legacyBizPath {
		return client.BuildIAMPath(model.ResourceTypeBiz, m.BizID)
	}

	switch m.ResourceType {
	case model.ResourceTypeProject:
		return client.BuildIAMPath(model.ResourceTypeBiz, m.BizID)
	case model.ResourceTypeEnvironment:
		return client.BuildIAMPath(model.ResourceTypeBiz, m.BizID,
			model.ResourceTypeProject, m.ProjectID)
	case model.ResourceTypeApp:
		return client.BuildIAMPath(model.ResourceTypeBiz, m.BizID,
			model.ResourceTypeProject, m.ProjectID, model.ResourceTypeEnvironment, m.EnvironmentID)
	default:
		return client.BuildIAMPath(model.ResourceTypeBiz, m.BizID)
	}
}

// ancestors 按资源类型返回申请链接用的祖先链，顺序从根到父级。
func (m *Mapped) ancestors() []client.ResourceRef {
	// 申请链接的祖先链必须与鉴权路径、注册拓扑保持一致。
	if m.legacyBizPath {
		return []client.ResourceRef{{Type: model.ResourceTypeBiz, ID: m.BizID}}
	}

	switch m.ResourceType {
	case model.ResourceTypeProject:
		return []client.ResourceRef{{Type: model.ResourceTypeBiz, ID: m.BizID}}
	case model.ResourceTypeEnvironment:
		return []client.ResourceRef{
			{Type: model.ResourceTypeBiz, ID: m.BizID},
			{Type: model.ResourceTypeProject, ID: m.ProjectID},
		}
	case model.ResourceTypeApp:
		return []client.ResourceRef{
			{Type: model.ResourceTypeBiz, ID: m.BizID},
			{Type: model.ResourceTypeProject, ID: m.ProjectID},
			{Type: model.ResourceTypeEnvironment, ID: m.EnvironmentID},
		}
	default:
		return []client.ResourceRef{{Type: model.ResourceTypeBiz, ID: m.BizID}}
	}
}

// HasInstance 判断资源是否有可用的实例 ID。
// ID 为 0 表示实例尚未确定（例如调用方未指定目标环境），既不能生成申请项，
// 也没有必要向权限中心申请一个不存在的实例。
func (m *Mapped) HasInstance() bool {
	return m.ResourceType != "" && m.ResourceID != "" && m.ResourceID != "0"
}

// ApplyResource 返回权限申请链接用的资源。
// 与鉴权不同，这里的拓扑用 ancestors 数组表达，而不是 attributes 里的路径字符串。
func (m *Mapped) ApplyResource() []client.ApplyResource {
	if !m.HasInstance() {
		return nil
	}

	if m.ResourceType == model.ResourceTypeBiz {
		return []client.ApplyResource{{ID: m.ResourceID, Type: model.ResourceTypeBiz}}
	}

	// 非根资源在申请链接里用 ancestors 表达归属拓扑：
	// 项目只到业务一层，环境再到项目一层。
	return []client.ApplyResource{{
		ID:        m.ResourceID,
		Type:      m.ResourceType,
		Ancestors: m.ancestors(),
	}}
}

// Adapt 把 meta 包中 BSCP 自定义的资源类型与操作，映射为 IAM V4 注册的操作与资源。
func Adapt(a *meta.ResourceAttribute) (*Mapped, error) {
	return AdaptWithOptions(a, defaultOptions())
}

// AdaptWithOptions 按指定的权限维度把 BSCP 资源与操作映射为 IAM V4 的操作与资源。
func AdaptWithOptions(a *meta.ResourceAttribute, opts Options) (*Mapped, error) {
	if a == nil {
		return nil, fmt.Errorf("resource attribute is not set")
	}

	// 上层显式要求跳过时不再看资源类型
	if a.Action == meta.SkipAction {
		return skipped(), nil
	}

	// 项目/环境维度校验关闭时，这两类资源不进入权限中心：
	// 调用方已有的业务维度校验就是该场景下的全部约束。
	if !opts.ProjectEnvironmentAuth && (a.Type == meta.Project || a.Type == meta.Environment) {
		return skipped(), nil
	}

	switch a.Type {
	case meta.Biz:
		return adaptBiz(a)
	case meta.App:
		return adaptApp(a, opts)
	case meta.Credential:
		return adaptCredential(a)
	case meta.Audit:
		return adaptAudit(a)
	case meta.ProcConfigMgmt:
		return adaptProcConfigMgmt(a)
	case meta.Project:
		return adaptProject(a)
	case meta.Environment:
		return adaptEnvironment(a, opts)
	case meta.Commit, meta.ConfigItem, meta.Content, meta.CRInstance, meta.Release,
		meta.ReleasedCI, meta.Strategy, meta.StrategySet, meta.PSH, meta.Repo, meta.Sidecar:
		return skipped(), nil
	default:
		return nil, fmt.Errorf("unsupported bscp auth type: %s", a.Type)
	}
}

func adaptBiz(a *meta.ResourceAttribute) (*Mapped, error) {
	if a.Action != meta.FindBusinessResource {
		return nil, fmt.Errorf("unsupported bscp action for biz: %s", a.Action)
	}

	return bizScoped(model.ActionFindBusinessResource, a.BizID), nil
}

// adaptApp 映射服务相关操作。
//
// 服务的拓扑路径必须与权限中心注册的父链一致：新拓扑为「业务 → 项目 → 环境」三级，
// 旧拓扑只有业务一级。项目/环境维度校验开关只决定这两类资源本身是否校验，
// 不会改变服务路径——路径与父链不符时，权限中心无法用它匹配任何服务级授权。
func adaptApp(a *meta.ResourceAttribute, opts Options) (*Mapped, error) {
	switch a.Action {
	case meta.Create:
		// 服务创建按环境授权：语义是"允许在哪个环境下创建服务"，此时还没有 app 实例，
		// 授权维度落在服务将要归属的环境上（ResourceID 即目标环境 ID），
		// 路径为 /biz,{bizID}/project,{projectID}/，按业务或项目维度的授权经继承覆盖。
		// 旧拓扑下 app_create 只关联业务实例，只能按业务维度发起。
		if opts.Topology.IsLegacy() {
			return bizScoped(model.ActionAppCreate, a.BizID), nil
		}

		return environmentScoped(model.ActionAppCreate, a, opts.Topology), nil
	case meta.Find:
		// 查询服务列表属于业务级别操作
		return bizScoped(model.ActionFindBusinessResource, a.BizID), nil
	case meta.View:
		return appScoped(model.ActionAppView, a, opts.Topology), nil
	case meta.Update:
		return appScoped(model.ActionAppEdit, a, opts.Topology), nil
	case meta.Delete:
		return appScoped(model.ActionAppDelete, a, opts.Topology), nil
	case meta.GenerateRelease:
		return appScoped(model.ActionReleaseGenerate, a, opts.Topology), nil
	case meta.Publish:
		return appScoped(model.ActionReleasePublish, a, opts.Topology), nil
	default:
		return nil, fmt.Errorf("unsupported bscp action for app: %s", a.Action)
	}
}

// adaptProject 项目按项目实例授权，实现项目级隔离：
// 授予某个项目的权限只覆盖该项目；按业务授权的用户仍覆盖业务下全部项目（路径继承）。
// 创建时项目实例还不存在，只能按业务授权，语义是"允许在哪个业务下创建项目"。
// 列表没有确定的项目实例，按业务访问放行，与服务的列表语义一致。
func adaptProject(a *meta.ResourceAttribute) (*Mapped, error) {
	switch a.Action {
	case meta.Create:
		return bizScoped(model.ActionProjectCreate, a.BizID), nil
	case meta.Find:
		return bizScoped(model.ActionFindBusinessResource, a.BizID), nil
	case meta.View:
		return projectScoped(model.ActionProjectView, a), nil
	case meta.Update:
		return projectScoped(model.ActionProjectEdit, a), nil
	case meta.Delete:
		return projectScoped(model.ActionProjectDelete, a), nil
	default:
		return nil, fmt.Errorf("unsupported bscp action for project: %s", a.Action)
	}
}

// adaptEnvironment 环境是独立授权资源，按环境实例鉴权：
// 授予某个环境的权限只覆盖该环境；项目或业务维度的授权经拓扑路径覆盖其下全部环境。
// 创建时环境实例还不存在，按所属项目授权。
func adaptEnvironment(a *meta.ResourceAttribute, opts Options) (*Mapped, error) {
	switch a.Action {
	case meta.Create:
		return projectScoped(model.ActionEnvironmentCreate, a), nil
	case meta.View:
		return environmentScoped(model.ActionEnvironmentView, a, opts.Topology), nil
	case meta.Update:
		return environmentScoped(model.ActionEnvironmentEdit, a, opts.Topology), nil
	case meta.Delete:
		return environmentScoped(model.ActionEnvironmentDelete, a, opts.Topology), nil
	default:
		return nil, fmt.Errorf("unsupported bscp action for environment: %s", a.Action)
	}
}

func adaptCredential(a *meta.ResourceAttribute) (*Mapped, error) {
	switch a.Action {
	case meta.View:
		return bizScoped(model.ActionAppCredentialView, a.BizID), nil
	case meta.Manage:
		return bizScoped(model.ActionAppCredentialManage, a.BizID), nil
	default:
		return nil, fmt.Errorf("unsupported bscp action for credential: %s", a.Action)
	}
}

func adaptAudit(a *meta.ResourceAttribute) (*Mapped, error) {
	if a.Action != meta.View {
		return nil, fmt.Errorf("unsupported bscp action for audit: %s", a.Action)
	}

	return bizScoped(model.ActionAuditView, a.BizID), nil
}

func adaptProcConfigMgmt(a *meta.ResourceAttribute) (*Mapped, error) {
	switch a.Action {
	case meta.View:
		return bizScoped(model.ActionProcConfigMgmtView, a.BizID), nil
	case meta.ProcessOperate:
		return bizScoped(model.ActionProcessOperate, a.BizID), nil
	case meta.Create:
		return bizScoped(model.ActionConfigTemplateCreate, a.BizID), nil
	case meta.Update:
		return bizScoped(model.ActionConfigTemplateEdit, a.BizID), nil
	case meta.Delete:
		return bizScoped(model.ActionConfigTemplateDelete, a.BizID), nil
	case meta.GenerateConfig:
		return bizScoped(model.ActionConfigGenerate, a.BizID), nil
	case meta.ReleaseConfig:
		return bizScoped(model.ActionConfigRelease, a.BizID), nil
	default:
		return nil, fmt.Errorf("unsupported bscp action for process config management: %s", a.Action)
	}
}

// bizScoped 构造按业务维度鉴权的映射。
func bizScoped(actionID string, bizID uint32) *Mapped {
	id := formatID(bizID)

	return &Mapped{
		ActionID:     actionID,
		ResourceType: model.ResourceTypeBiz,
		ResourceID:   id,
		BizID:        id,
	}
}

// appScoped 构造按服务维度鉴权的映射。
// 新拓扑下服务挂在环境下，鉴权路径为三级：/biz,{biz}/project,{project}/environment,{env}/；
// 旧拓扑下服务直接挂业务，路径只有 /biz,{biz}/。
// 归属为 0 时按 0 拼接（不省略层级），与 V3 的 appIAMPath 完全一致：
// 历史服务还没回填项目/环境时，业务维度的授权仍能通过路径继承覆盖它，
// 项目/环境维度的授权则因为 ID 对不上而自然不命中。
func appScoped(actionID string, a *meta.ResourceAttribute, topology meta.ResourceTopology) *Mapped {
	return &Mapped{
		ActionID:      actionID,
		ResourceType:  model.ResourceTypeApp,
		ResourceID:    formatID(a.ResourceID),
		BizID:         formatID(a.BizID),
		ProjectID:     formatID(a.ProjectID),
		EnvironmentID: formatID(a.EnvironmentID),
		legacyBizPath: topology.IsLegacy(),
	}
}

// projectScoped 构造按项目维度鉴权的映射。
func projectScoped(actionID string, a *meta.ResourceAttribute) *Mapped {
	return &Mapped{
		ActionID:     actionID,
		ResourceType: model.ResourceTypeProject,
		ResourceID:   formatID(a.ResourceID),
		BizID:        formatID(a.BizID),
	}
}

// environmentScoped 构造按环境维度鉴权的映射。
// 新拓扑下环境挂项目，路径为 /biz,{biz}/project,{project}/；
// 旧拓扑没有环境资源类型，只声明业务一层。
// 与 V3 的 environmentIAMPath 一致：项目归属为 0 时照样拼成 /biz,{biz}/project,0/，
// 业务维度的授权仍可覆盖该环境，项目维度的授权因为 ID 对不上而不命中。
func environmentScoped(actionID string, a *meta.ResourceAttribute,
	topology meta.ResourceTopology) *Mapped {

	return &Mapped{
		ActionID:      actionID,
		ResourceType:  model.ResourceTypeEnvironment,
		ResourceID:    formatID(a.ResourceID),
		BizID:         formatID(a.BizID),
		ProjectID:     formatID(a.ProjectID),
		legacyBizPath: topology.IsLegacy(),
	}
}

func formatID(id uint32) string {
	return strconv.FormatUint(uint64(id), 10)
}
