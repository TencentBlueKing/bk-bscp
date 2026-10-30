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

// ApplyResource 返回权限申请链接用的资源。
// 与鉴权不同，这里的拓扑用 ancestors 数组表达，而不是 attributes 里的路径字符串。
func (m *Mapped) ApplyResource() []client.ApplyResource {
	if m.ResourceType == "" {
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
	if a == nil {
		return nil, fmt.Errorf("resource attribute is not set")
	}

	// 上层显式要求跳过时不再看资源类型
	if a.Action == meta.SkipAction {
		return skipped(), nil
	}

	switch a.Type {
	case meta.Biz:
		return adaptBiz(a)
	case meta.App:
		return adaptApp(a)
	case meta.Credential:
		return adaptCredential(a)
	case meta.Audit:
		return adaptAudit(a)
	case meta.ProcConfigMgmt:
		return adaptProcConfigMgmt(a)
	case meta.Project:
		return adaptProject(a)
	case meta.Environment:
		return adaptEnvironment(a)
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

func adaptApp(a *meta.ResourceAttribute) (*Mapped, error) {
	switch a.Action {
	case meta.Create:
		// 服务创建按环境授权：语义是"允许在哪个环境下创建服务"，此时还没有 app 实例，
		// 授权维度落在服务将要归属的环境上（ResourceID 即目标环境 ID），
		// 路径为 /biz,{bizID}/project,{projectID}/，按业务或项目维度的授权经继承覆盖。
		return environmentScoped(model.ActionAppCreate, a), nil
	case meta.Find:
		// 查询服务列表属于业务级别操作
		return bizScoped(model.ActionFindBusinessResource, a.BizID), nil
	case meta.View:
		return appScoped(model.ActionAppView, a), nil
	case meta.Update:
		return appScoped(model.ActionAppEdit, a), nil
	case meta.Delete:
		return appScoped(model.ActionAppDelete, a), nil
	case meta.GenerateRelease:
		return appScoped(model.ActionReleaseGenerate, a), nil
	case meta.Publish:
		return appScoped(model.ActionReleasePublish, a), nil
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
func adaptEnvironment(a *meta.ResourceAttribute) (*Mapped, error) {
	switch a.Action {
	case meta.Create:
		return projectScoped(model.ActionEnvironmentCreate, a), nil
	case meta.View:
		return environmentScoped(model.ActionEnvironmentView, a), nil
	case meta.Update:
		return environmentScoped(model.ActionEnvironmentEdit, a), nil
	case meta.Delete:
		return environmentScoped(model.ActionEnvironmentDelete, a), nil
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
// 服务挂在环境下，鉴权需要业务、项目、环境三级路径；任一缺失都会让路径不完整，
// 由 BuildIAMPath 返回空路径，鉴权退化为无权限。
func appScoped(actionID string, a *meta.ResourceAttribute) *Mapped {
	return &Mapped{
		ActionID:      actionID,
		ResourceType:  model.ResourceTypeApp,
		ResourceID:    formatID(a.ResourceID),
		BizID:         formatID(a.BizID),
		ProjectID:     formatOptionalID(a.ProjectID),
		EnvironmentID: formatOptionalID(a.EnvironmentID),
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
func environmentScoped(actionID string, a *meta.ResourceAttribute) *Mapped {
	return &Mapped{
		ActionID:     actionID,
		ResourceType: model.ResourceTypeEnvironment,
		ResourceID:   formatID(a.ResourceID),
		BizID:        formatID(a.BizID),
		ProjectID:    formatOptionalID(a.ProjectID),
	}
}

func formatID(id uint32) string {
	return strconv.FormatUint(uint64(id), 10)
}

// formatOptionalID 与 formatID 相同，但 0 视为"未提供"，返回空串。
// BuildIAMPath 遇到空 ID 会返回空路径，从而让鉴权退化为无权限而不是拼出错误路径。
func formatOptionalID(id uint32) string {
	if id == 0 {
		return ""
	}

	return formatID(id)
}
