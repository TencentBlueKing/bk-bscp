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
	"fmt"
	"strconv"

	"github.com/TencentBlueKing/iam-go-sdk"
	bkiam "github.com/TencentBlueKing/iam-go-sdk"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
)

// bizIDAssembleSymbol used to assemble biz_id and resource id's symbol, used in app id generation.
// nolint: unused
const bizIDAssembleSymbol = "-"

var (
	// dummyIAMUser
	dummyIAMUser = bkiam.NewSubject("user", "")
)

// genSkipResource generate iam resource for resource, using skip action.
func genSkipResource(_ *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	return sys.Skip, make([]client.Resource, 0), nil
}

// genBizResource
func genBizResource(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	switch a.Action {
	case meta.FindBusinessResource:
		// create app is related to cmdb business resource
		return sys.BusinessViewResource, []client.Resource{bizRes}, nil
	default:
		return "", nil, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}
}

func genBizIAMResource(a *meta.ResourceAttribute) (*bkiam.Request, error) {
	iamReq := bkiam.Request{
		System:  sys.SystemIDBSCP,
		Subject: dummyIAMUser,
		Resources: []iam.ResourceNode{
			{
				System:    sys.SystemIDCMDB,
				Type:      string(sys.Business),
				ID:        strconv.FormatUint(uint64(a.BizID), 10),
				Attribute: map[string]interface{}{},
			},
		},
	}

	switch a.Action {
	case meta.FindBusinessResource:
		// create app is related to cmdb business resource
		iamReq.Action = bkiam.NewAction(string(sys.BusinessViewResource))
	default:
		return nil, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}
	return &iamReq, nil
}

func genAppIAMResource(a *meta.ResourceAttribute, topology meta.ResourceTopology) (*bkiam.Request, error) {
	// 服务创建按目标环境授权：此时还没有服务实例，资源节点用环境而不是服务。
	// 旧拓扑下 app_create 关联的是业务实例，不能提交环境节点。
	if a.Action == meta.Create {
		action := sys.EnvironmentAppCreate
		if topology.IsLegacy() {
			action = sys.AppCreate
		}

		return &bkiam.Request{
			System:    sys.SystemIDBSCP,
			Subject:   dummyIAMUser,
			Action:    bkiam.NewAction(string(action)),
			Resources: []iam.ResourceNode{appCreateResourceNode(a, topology)},
		}, nil
	}

	iamReq := bkiam.Request{
		System:  sys.SystemIDBSCP,
		Subject: dummyIAMUser,
		Resources: []iam.ResourceNode{
			appResourceNode(a, topology),
		},
	}

	switch a.Action {
	case meta.View:
		iamReq.Action = bkiam.NewAction(string(sys.AppView))
	case meta.Update:
		iamReq.Action = bkiam.NewAction(string(sys.AppEdit))
	case meta.Publish:
		iamReq.Action = bkiam.NewAction(string(sys.ReleasePublish))
	case meta.GenerateRelease:
		iamReq.Action = bkiam.NewAction(string(sys.ReleaseGenerate))
	case meta.Delete:
		iamReq.Action = bkiam.NewAction(string(sys.AppDelete))
	default:
		return nil, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}
	return &iamReq, nil
}

func genCredIAMResource(a *meta.ResourceAttribute) (*bkiam.Request, error) {
	iamReq := bkiam.Request{
		System:  sys.SystemIDBSCP,
		Subject: dummyIAMUser,
		Resources: []iam.ResourceNode{
			{
				System:    sys.SystemIDCMDB,
				Type:      string(sys.Business),
				ID:        strconv.FormatUint(uint64(a.BizID), 10),
				Attribute: map[string]interface{}{},
			},
		},
	}

	switch a.Action {
	case meta.View:
		iamReq.Action = bkiam.NewAction(string(sys.CredentialView))
	case meta.Manage:
		iamReq.Action = bkiam.NewAction(string(sys.CredentialManage))
	default:
		return nil, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}
	return &iamReq, nil
}

func genBizIAMApplication(a *meta.ResourceAttribute) (bkiam.ApplicationAction, error) {
	action := bkiam.ApplicationAction{
		RelatedResourceTypes: []bkiam.ApplicationRelatedResourceType{
			{
				SystemID: sys.SystemIDCMDB,
				Type:     string(sys.Business),
				Instances: []bkiam.ApplicationResourceInstance{
					[]iam.ApplicationResourceNode{
						{
							Type: string(sys.Business),
							ID:   strconv.FormatUint(uint64(a.BizID), 10),
						},
					},
				},
			},
		},
	}

	switch a.Action {
	case meta.FindBusinessResource:
		// create app is related to cmdb business resource
		action.ID = string(sys.BusinessViewResource)
	default:
		return action, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}
	return action, nil
}

// genAppIAMApplication 构造服务相关的申请链接资源。
//
// 申请链接只用于向权限中心提交申请单，不参与鉴权判定，但节点链必须与权限中心注册的
// 实例视图一致：新拓扑下服务需要「业务 → 项目 → 环境 → 服务」、服务创建需要
// 「业务 → 项目 → 环境」，缺少中间层级会被直接拒绝，表现为 1902412 not satisfy instance selection。
// 旧拓扑下服务直接挂在业务下，提交项目/环境节点同样会被拒绝。
// 归属（业务/项目/环境）缺失时拼不出合法链，此时不生成申请项，避免提交 ID 为 0 的非法节点。
func genAppIAMApplication(a *meta.ResourceAttribute, topology meta.ResourceTopology) (
	bkiam.ApplicationAction, error) {

	resourceNodeBiz := iam.ApplicationResourceNode{
		Type: string(sys.Business),
		ID:   strconv.FormatUint(uint64(a.BizID), 10),
	}
	resourceNodeProject := iam.ApplicationResourceNode{
		Type: string(sys.Project),
		ID:   strconv.FormatUint(uint64(a.ProjectID), 10),
	}

	// 没有服务实例时（例如克隆只按业务维度校验 app_edit）无法生成实例级申请项。
	if a.Action != meta.Create && a.ResourceID == 0 {
		return bkiam.ApplicationAction{}, nil
	}

	if topology.IsLegacy() {
		return genLegacyAppIAMApplication(a, resourceNodeBiz)
	}

	switch a.Action {
	case meta.Create:
		// 服务创建按目标环境授权：申请项的资源类型是环境，ResourceID 即目标环境 ID。
		if a.BizID == 0 || a.ProjectID == 0 || a.ResourceID == 0 {
			return bkiam.ApplicationAction{}, nil
		}

		return bkiam.ApplicationAction{
			ID: string(sys.EnvironmentAppCreate),
			RelatedResourceTypes: []bkiam.ApplicationRelatedResourceType{{
				SystemID: sys.SystemIDBSCP,
				Type:     string(sys.Environment),
				Instances: []bkiam.ApplicationResourceInstance{{
					resourceNodeBiz,
					resourceNodeProject,
					{
						Type: string(sys.Environment),
						ID:   strconv.FormatUint(uint64(a.ResourceID), 10),
					},
				}},
			}},
		}, nil
	case meta.View, meta.Update, meta.Delete, meta.Publish, meta.GenerateRelease:
		// 服务本身是申请项的资源类型，需要完整三级祖先。
		if a.BizID == 0 || a.ProjectID == 0 || a.EnvironmentID == 0 {
			return bkiam.ApplicationAction{}, nil
		}

		return bkiam.ApplicationAction{
			ID: string(appApplicationActionID(a.Action)),
			RelatedResourceTypes: []bkiam.ApplicationRelatedResourceType{{
				SystemID: sys.SystemIDBSCP,
				Type:     string(sys.Application),
				Instances: []bkiam.ApplicationResourceInstance{{
					resourceNodeBiz,
					resourceNodeProject,
					{
						Type: string(sys.Environment),
						ID:   strconv.FormatUint(uint64(a.EnvironmentID), 10),
					},
					{
						Type: string(sys.Application),
						ID:   strconv.FormatUint(uint64(a.ResourceID), 10),
					},
				}},
			}},
		}, nil
	default:
		return bkiam.ApplicationAction{}, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}
}

// genLegacyAppIAMApplication 构造旧拓扑（业务 → 服务）下的服务申请项。
// 旧拓扑里没有项目、环境资源类型，节点链只能到业务一层。
func genLegacyAppIAMApplication(a *meta.ResourceAttribute,
	resourceNodeBiz iam.ApplicationResourceNode) (bkiam.ApplicationAction, error) {

	if a.BizID == 0 {
		return bkiam.ApplicationAction{}, nil
	}

	switch a.Action {
	case meta.Create:
		return bkiam.ApplicationAction{
			ID: string(sys.AppCreate),
			RelatedResourceTypes: []bkiam.ApplicationRelatedResourceType{{
				SystemID: sys.SystemIDCMDB,
				Type:     string(sys.Business),
				Instances: []bkiam.ApplicationResourceInstance{{
					resourceNodeBiz,
				}},
			}},
		}, nil
	case meta.View, meta.Update, meta.Delete, meta.Publish, meta.GenerateRelease:
		return bkiam.ApplicationAction{
			ID: string(appApplicationActionID(a.Action)),
			RelatedResourceTypes: []bkiam.ApplicationRelatedResourceType{{
				SystemID: sys.SystemIDBSCP,
				Type:     string(sys.Application),
				Instances: []bkiam.ApplicationResourceInstance{{
					resourceNodeBiz,
					{
						Type: string(sys.Application),
						ID:   strconv.FormatUint(uint64(a.ResourceID), 10),
					},
				}},
			}},
		}, nil
	default:
		return bkiam.ApplicationAction{}, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}
}

// appApplicationActionID 返回服务实例级操作对应的权限中心动作 ID。
func appApplicationActionID(action meta.Action) client.ActionID {
	switch action {
	case meta.Update:
		return sys.AppEdit
	case meta.Delete:
		return sys.AppDelete
	case meta.Publish:
		return sys.ReleasePublish
	case meta.GenerateRelease:
		return sys.ReleaseGenerate
	default:
		return sys.AppView
	}
}

func genCredIAMApplication(a *meta.ResourceAttribute) (bkiam.ApplicationAction, error) {
	action := bkiam.ApplicationAction{
		RelatedResourceTypes: []bkiam.ApplicationRelatedResourceType{{
			SystemID: sys.SystemIDCMDB,
			Type:     string(sys.Business),
			Instances: []bkiam.ApplicationResourceInstance{
				[]iam.ApplicationResourceNode{{
					Type: string(sys.Business),
					ID:   strconv.FormatUint(uint64(a.BizID), 10),
				}},
			},
		}},
	}

	switch a.Action {
	case meta.View:
		action.ID = string(sys.CredentialView)
	case meta.Manage:
		action.ID = string(sys.CredentialManage)
	default:
		return action, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}
	return action, nil
}

func genAuditIAMApplication(a *meta.ResourceAttribute) (bkiam.ApplicationAction, error) {
	action := bkiam.ApplicationAction{
		RelatedResourceTypes: []bkiam.ApplicationRelatedResourceType{{
			SystemID: sys.SystemIDCMDB,
			Type:     string(sys.Business),
			Instances: []bkiam.ApplicationResourceInstance{
				[]iam.ApplicationResourceNode{{
					Type: string(sys.Business),
					ID:   strconv.FormatUint(uint64(a.BizID), 10),
				}},
			},
		}},
	}

	switch a.Action {
	case meta.View:
		action.ID = string(sys.AuditView)
	default:
		return action, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}
	return action, nil
}

// genAppResource generate application related iam resource.
//
// 服务的鉴权路径必须与权限中心注册的父链一致：权限中心不反查拓扑，
// 也不接受比父链更短或更长的路径，对不上会让服务级授权全部匹配不到。
// 新拓扑为「业务 → 项目 → 环境」三级，旧拓扑只有业务一级。
// 项目/环境维度校验开关只决定是否单独校验这两类资源，不改变服务自身路径的完整性。
func genAppResource(a *meta.ResourceAttribute, topology meta.ResourceTopology) (
	client.ActionID, []client.Resource, error) {

	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	appRes := client.Resource{
		System: sys.SystemIDBSCP,
		Type:   sys.Application,
		ID:     strconv.FormatUint(uint64(a.ResourceID), 10),
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{appIAMPath(a, topology)},
		},
	}

	switch a.Action {
	case meta.Create:
		// 服务创建在新拓扑下按目标环境授权（ResourceID 即环境 ID），
		// 旧拓扑下 app_create 只关联业务实例，只能按业务维度发起。
		if topology.IsLegacy() {
			return sys.AppCreate, []client.Resource{bizRes}, nil
		}

		return sys.EnvironmentAppCreate, []client.Resource{environmentClientResource(a, topology)}, nil
	case meta.Update:
		// update app is related to bscp application resource
		return sys.AppEdit, []client.Resource{appRes}, nil
	case meta.Delete:
		// delete app is related to bscp application resource
		return sys.AppDelete, []client.Resource{appRes}, nil
	case meta.View:
		// view app is related to bscp application resource
		return sys.AppView, []client.Resource{appRes}, nil
	case meta.GenerateRelease:
		// generate release is related to bscp application resource
		return sys.ReleaseGenerate, []client.Resource{appRes}, nil
	case meta.Publish:
		// publish release is related to bscp application resource
		return sys.ReleasePublish, []client.Resource{appRes}, nil
	case meta.Find:
		// find app is related to cmdb business resource, using view biz action
		return sys.BusinessViewResource, []client.Resource{bizRes}, nil
	default:
		return "", nil, errf.New(errf.InvalidParameter, fmt.Sprintf("unsupported bscp action: %s", a.Action))
	}
}

func genCredResource(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	switch a.Action {
	case meta.View:
		return sys.CredentialView, []client.Resource{bizRes}, nil
	case meta.Manage:
		return sys.CredentialManage, []client.Resource{bizRes}, nil
	default:
		return "", nil, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}
}

// audit permission
func genAuditResource(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	switch a.Action {
	case meta.View:
		return sys.AuditView, []client.Resource{bizRes}, nil
	default:
		return "", nil, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}
}

// genCommitResource generate commit related iam resource.
// nolint: unused
func genCommitResource(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	appRes := client.Resource{
		System: sys.SystemIDBSCP,
		Type:   sys.Application,
		ID: strconv.FormatUint(uint64(a.BizID), 10) + bizIDAssembleSymbol +
			strconv.FormatUint(uint64(a.ResourceID), 10),
		// can be authorized based on business
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{fmt.Sprintf("/%s,%d/", sys.Business, a.BizID)},
		},
	}

	switch a.Action {
	case meta.Create:
		// create commit is related to bscp application resource, using app edit action
		return sys.AppEdit, []client.Resource{appRes}, nil
	case meta.Find:
		// find commit is related to cmdb business resource, using view biz action
		return sys.BusinessViewResource, []client.Resource{bizRes}, nil
	default:
		return "", nil, errf.New(errf.InvalidParameter, fmt.Sprintf("unsupported bscp action: %s", a.Action))
	}
}

// genConfigItemResource generate config item related iam resource.
// nolint: unused
func genConfigItemResource(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	appRes := client.Resource{
		System: sys.SystemIDBSCP,
		Type:   sys.Application,
		ID: strconv.FormatUint(uint64(a.BizID), 10) + bizIDAssembleSymbol +
			strconv.FormatUint(uint64(a.ResourceID), 10),
		// can be authorized based on business
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{fmt.Sprintf("/%s,%d/", sys.Business, a.BizID)},
		},
	}

	switch a.Action {
	case meta.Create:
		// create config item is related to bscp application resource, using app edit action
		return sys.AppEdit, []client.Resource{appRes}, nil
	case meta.Update:
		// update config item is related to bscp application resource, using app edit action
		return sys.AppEdit, []client.Resource{appRes}, nil
	case meta.Delete:
		// delete config item is related to bscp application resource, using app edit action
		return sys.AppEdit, []client.Resource{appRes}, nil
	case meta.Find:
		// find config item is related to cmdb business resource, using view biz action
		return sys.BusinessViewResource, []client.Resource{bizRes}, nil
	default:
		return "", nil, errf.New(errf.InvalidParameter, fmt.Sprintf("unsupported bscp action: %s", a.Action))
	}
}

// genContentResource generate content related iam resource.
// nolint: unused
func genContentResource(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	appRes := client.Resource{
		System: sys.SystemIDBSCP,
		Type:   sys.Application,
		ID: strconv.FormatUint(uint64(a.BizID), 10) + bizIDAssembleSymbol +
			strconv.FormatUint(uint64(a.ResourceID), 10),
		// can be authorized based on business
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{fmt.Sprintf("/%s,%d/", sys.Business, a.BizID)},
		},
	}

	switch a.Action {
	case meta.Create:
		// create content is related to bscp application resource, using app edit action
		return sys.AppEdit, []client.Resource{appRes}, nil
	case meta.Find:
		// find content is related to cmdb business resource, using view biz action
		return sys.BusinessViewResource, []client.Resource{bizRes}, nil
	case meta.Upload:
		// upload content is related to bscp application resource, using app edit action
		return sys.AppEdit, []client.Resource{appRes}, nil
	case meta.Download:
		// download content is related to bscp application resource, using app edit action
		return sys.AppEdit, []client.Resource{appRes}, nil
	default:
		return "", nil, errf.New(errf.InvalidParameter, fmt.Sprintf("unsupported bscp action: %s", a.Action))
	}
}

// genCRInstanceResource generate current released instance related iam resource.
// nolint: unused
func genCRInstanceResource(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	appRes := client.Resource{
		System: sys.SystemIDBSCP,
		Type:   sys.Application,
		ID: strconv.FormatUint(uint64(a.BizID), 10) + bizIDAssembleSymbol +
			strconv.FormatUint(uint64(a.ResourceID), 10),
		// can be authorized based on business
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{fmt.Sprintf("/%s,%d/", sys.Business, a.BizID)},
		},
	}

	switch a.Action {
	case meta.Create:
		// create current released instance is related to bscp application resource, using app edit action
		return sys.AppEdit, []client.Resource{appRes}, nil
	case meta.Delete:
		// delete current released instance is related to bscp application resource, using app edit action
		return sys.AppEdit, []client.Resource{appRes}, nil
	case meta.Find:
		// find current released instance is related to cmdb business resource, using view biz action
		return sys.BusinessViewResource, []client.Resource{bizRes}, nil
	default:
		return "", nil, errf.New(errf.InvalidParameter, fmt.Sprintf("unsupported bscp action: %s", a.Action))
	}
}

// genReleaseRes generate release related iam resource.
// nolint: unused
func genReleaseRes(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	appRes := client.Resource{
		System: sys.SystemIDBSCP,
		Type:   sys.Application,
		ID: strconv.FormatUint(uint64(a.BizID), 10) + bizIDAssembleSymbol +
			strconv.FormatUint(uint64(a.ResourceID), 10),
		// can be authorized based on business
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{fmt.Sprintf("/%s,%d/", sys.Business, a.BizID)},
		},
	}

	switch a.Action {
	case meta.Create:
		// create current released instance is related to bscp application resource, using config item packing action
		return sys.ReleaseGenerate, []client.Resource{appRes}, nil
	case meta.Find:
		// find release is related to cmdb business resource, using view biz action
		return sys.BusinessViewResource, []client.Resource{bizRes}, nil
	default:
		return "", nil, errf.New(errf.InvalidParameter, fmt.Sprintf("unsupported bscp action: %s", a.Action))
	}
}

// genReleasedCIRes generate released config item related iam resource.
// nolint: unused
func genReleasedCIRes(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	switch a.Action {
	case meta.Find:
		// find released config item is related to cmdb business resource, using view biz action
		return sys.BusinessViewResource, []client.Resource{bizRes}, nil
	default:
		return "", nil, errf.New(errf.InvalidParameter, fmt.Sprintf("unsupported bscp action: %s", a.Action))
	}
}

// genStrategyRes generate strategy related iam resource.
// nolint: unused
func genStrategyRes(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	appRes := client.Resource{
		System: sys.SystemIDBSCP,
		Type:   sys.Application,
		ID: strconv.FormatUint(uint64(a.BizID), 10) + bizIDAssembleSymbol +
			strconv.FormatUint(uint64(a.ResourceID), 10),
		// can be authorized based on business
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{fmt.Sprintf("/%s,%d/", sys.Business, a.BizID)},
		},
	}

	switch a.Action {
	case meta.Create:
		// create strategy is related to bscp application resource, using strategy create action
		return sys.StrategyCreate, []client.Resource{appRes}, nil
	case meta.Update:
		// update strategy is related to bscp application resource, using strategy edit action
		return sys.StrategyEdit, []client.Resource{appRes}, nil
	case meta.Delete:
		// delete strategy is related to bscp application resource, using strategy delete action
		return sys.StrategyDelete, []client.Resource{appRes}, nil
	case meta.Publish:
		// publish strategy is related to bscp application resource, using strategy publish action
		return sys.ReleasePublish, []client.Resource{appRes}, nil
	case meta.FinishPublish:
		// finish publish strategy is related to bscp application resource, using strategy finish publish action
		return sys.ConfigItemFinishPublish, []client.Resource{appRes}, nil
	case meta.Find:
		// find strategy is related to cmdb business resource, using view biz action
		return sys.BusinessViewResource, []client.Resource{bizRes}, nil
	default:
		return "", nil, errf.New(errf.InvalidParameter, fmt.Sprintf("unsupported bscp action: %s", a.Action))
	}
}

// genStrategySetRes generate strategy set related iam resource.
// nolint: unused
func genStrategySetRes(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	appRes := client.Resource{
		System: sys.SystemIDBSCP,
		Type:   sys.Application,
		ID: strconv.FormatUint(uint64(a.BizID), 10) + bizIDAssembleSymbol +
			strconv.FormatUint(uint64(a.ResourceID), 10),
		// can be authorized based on business
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{fmt.Sprintf("/%s,%d/", sys.Business, a.BizID)},
		},
	}

	switch a.Action {
	case meta.Create:
		// create strategy set is related to bscp application resource, using strategy set create action
		return sys.StrategySetCreate, []client.Resource{appRes}, nil
	case meta.Update:
		// update strategy set is related to bscp application resource, using strategy set edit action
		return sys.StrategySetEdit, []client.Resource{appRes}, nil
	case meta.Delete:
		// delete strategy set is related to bscp application resource, using strategy set delete action
		return sys.StrategySetDelete, []client.Resource{appRes}, nil
	case meta.Find:
		// find strategy set is related to cmdb business resource, using view biz action
		return sys.BusinessViewResource, []client.Resource{bizRes}, nil
	default:
		return "", nil, errf.New(errf.InvalidParameter, fmt.Sprintf("unsupported bscp action: %s", a.Action))
	}
}

// genPSHRes generate published strategy history related iam resource.
// nolint: unused
func genPSHRes(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	switch a.Action {
	case meta.Find:
		// find published strategy history is related to cmdb business resource, using task history view action
		return sys.TaskHistoryView, []client.Resource{bizRes}, nil
	default:
		return "", nil, errf.New(errf.InvalidParameter, fmt.Sprintf("unsupported bscp action: %s", a.Action))
	}
}

// genRepoRes generate repo related iam resource.
// nolint: unused
func genRepoRes(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	switch a.Action {
	case meta.Find:
		// find repo resource is related to cmdb business resource, using view biz action
		return sys.BusinessViewResource, []client.Resource{bizRes}, nil
	default:
		return "", nil, errf.New(errf.InvalidParameter, fmt.Sprintf("unsupported bscp action: %s", a.Action))
	}
}

// genSidecarRes generate sidecar related iam resource.
// nolint: unused
func genSidecarRes(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	switch a.Action {
	case meta.Access:
		// request from sidecar is related to business view resource
		return sys.BusinessViewResource, []client.Resource{bizRes}, nil
	default:
		return "", nil, errf.New(errf.InvalidParameter, fmt.Sprintf("unsupported bscp action: %s", a.Action))
	}
}

// genCredentialRes generate application credential related iam resource.
// nolint: unused
func genCredentialRes(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	switch a.Action {
	case meta.Access:
		// request from credential is related to business view resource
		return sys.CredentialView, []client.Resource{bizRes}, nil
	case meta.Manage:
		// manage credential is related to bscp application resource
		return sys.CredentialManage, []client.Resource{bizRes}, nil

	default:
		return "", nil, errf.New(errf.InvalidParameter, fmt.Sprintf("unsupported bscp action: %s", a.Action))
	}
}

func genProcConfigMgmtRes(a *meta.ResourceAttribute) (*bkiam.Request, error) {
	iamReq := bkiam.Request{
		System:  sys.SystemIDBSCP,
		Subject: dummyIAMUser,
		Resources: []iam.ResourceNode{
			{
				System:    sys.SystemIDCMDB,
				Type:      string(sys.Business),
				ID:        strconv.FormatUint(uint64(a.BizID), 10),
				Attribute: map[string]interface{}{},
			},
		},
	}

	switch a.Action {
	case meta.View:
		iamReq.Action = bkiam.NewAction(string(sys.ProcConfigMgmtView))
	case meta.Create:
		iamReq.Action = bkiam.NewAction(string(sys.ConfigTemplateCreate))
	case meta.Update:
		iamReq.Action = bkiam.NewAction(string(sys.ConfigTemplateEdit))
	case meta.Delete:
		iamReq.Action = bkiam.NewAction(string(sys.ConfigTemplateDelete))
	case meta.GenerateConfig:
		iamReq.Action = bkiam.NewAction(string(sys.ConfigGenerate))
	case meta.ReleaseConfig:
		iamReq.Action = bkiam.NewAction(string(sys.ConfigRelease))
	case meta.ProcessOperate:
		iamReq.Action = bkiam.NewAction(string(sys.ProcessOperate))
	default:
		return nil, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}

	return &iamReq, nil
}

func genProcConfigMgmtApplication(a *meta.ResourceAttribute) (bkiam.ApplicationAction, error) {
	action := bkiam.ApplicationAction{
		RelatedResourceTypes: []bkiam.ApplicationRelatedResourceType{{
			SystemID: sys.SystemIDCMDB,
			Type:     string(sys.Business),
			Instances: []bkiam.ApplicationResourceInstance{
				[]iam.ApplicationResourceNode{{
					Type: string(sys.Business),
					ID:   strconv.FormatUint(uint64(a.BizID), 10),
				}},
			},
		}},
	}

	switch a.Action {
	case meta.View:
		action.ID = string(sys.ProcConfigMgmtView)
	case meta.ProcessOperate:
		action.ID = string(sys.ProcessOperate)
	case meta.ReleaseConfig:
		action.ID = string(sys.ConfigRelease)
	case meta.GenerateConfig:
		action.ID = string(sys.ConfigGenerate)
	case meta.Create:
		action.ID = string(sys.ConfigTemplateCreate)
	case meta.Update:
		action.ID = string(sys.ConfigTemplateEdit)
	case meta.Delete:
		action.ID = string(sys.ConfigTemplateDelete)
	default:
		return action, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}
	return action, nil
}

func genProcConfigMgmtResource(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	switch a.Action {
	case meta.View:
		return sys.ProcConfigMgmtView, []client.Resource{bizRes}, nil
	case meta.ProcessOperate:
		return sys.ProcessOperate, []client.Resource{bizRes}, nil
	case meta.ReleaseConfig:
		return sys.ConfigRelease, []client.Resource{bizRes}, nil
	case meta.GenerateConfig:
		return sys.ConfigGenerate, []client.Resource{bizRes}, nil
	case meta.Create:
		return sys.ConfigTemplateCreate, []client.Resource{bizRes}, nil
	case meta.Update:
		return sys.ConfigTemplateEdit, []client.Resource{bizRes}, nil
	case meta.Delete:
		return sys.ConfigTemplateDelete, []client.Resource{bizRes}, nil
	default:
		return "", nil, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}
}

// projectIAMPath 返回项目实例的拓扑路径，声明它所属的业务，
// 按业务维度的授权据此覆盖业务下全部项目。
func projectIAMPath(a *meta.ResourceAttribute) string {
	return bizOnlyIAMPath(a.BizID)
}

// bizOnlyIAMPath 返回只声明业务一层的拓扑路径，用于根资源（项目）的父级声明。
func bizOnlyIAMPath(bizID uint32) string {
	return fmt.Sprintf("/%s,%d/", sys.Business, bizID)
}

// environmentIAMPath 返回环境实例的拓扑路径。
// 新拓扑声明「业务 → 项目」两级，按业务或项目维度的授权据此覆盖项目下的全部环境；
// 旧拓扑没有项目资源类型，只声明业务一层。
func environmentIAMPath(a *meta.ResourceAttribute, topology meta.ResourceTopology) string {
	if topology.IsLegacy() {
		return bizOnlyIAMPath(a.BizID)
	}

	return fmt.Sprintf("/%s,%d/%s,%d/", sys.Business, a.BizID, sys.Project, a.ProjectID)
}

// appIAMPath 返回服务实例的拓扑路径。
// 新拓扑声明「业务 → 项目 → 环境」三级，按业务、项目或环境维度的授权据此覆盖环境下的全部服务；
// 旧拓扑下服务直接挂在业务下，只声明业务一层。
func appIAMPath(a *meta.ResourceAttribute, topology meta.ResourceTopology) string {
	if topology.IsLegacy() {
		return bizOnlyIAMPath(a.BizID)
	}

	return fmt.Sprintf("/%s,%d/%s,%d/%s,%d/", sys.Business, a.BizID,
		sys.Project, a.ProjectID, sys.Environment, a.EnvironmentID)
}

// projectResourceNode 构造项目实例节点。
func projectResourceNode(a *meta.ResourceAttribute) iam.ResourceNode {
	return iam.ResourceNode{
		System: sys.SystemIDBSCP,
		Type:   string(sys.Project),
		ID:     strconv.FormatUint(uint64(a.ResourceID), 10),
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{projectIAMPath(a)},
		},
	}
}

// environmentResourceNode 构造环境实例节点。
func environmentResourceNode(a *meta.ResourceAttribute, topology meta.ResourceTopology) iam.ResourceNode {
	return iam.ResourceNode{
		System: sys.SystemIDBSCP,
		Type:   string(sys.Environment),
		ID:     strconv.FormatUint(uint64(a.ResourceID), 10),
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{environmentIAMPath(a, topology)},
		},
	}
}

// appResourceNode 构造服务实例节点。
func appResourceNode(a *meta.ResourceAttribute, topology meta.ResourceTopology) iam.ResourceNode {
	return iam.ResourceNode{
		System: sys.SystemIDBSCP,
		Type:   string(sys.Application),
		ID:     strconv.FormatUint(uint64(a.ResourceID), 10),
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{appIAMPath(a, topology)},
		},
	}
}

// bizResourceNode 构造业务实例节点，用于旧拓扑下的服务创建。
func bizResourceNode(bizID uint32) iam.ResourceNode {
	return iam.ResourceNode{
		System:    sys.SystemIDCMDB,
		Type:      string(sys.Business),
		ID:        strconv.FormatUint(uint64(bizID), 10),
		Attribute: map[string]interface{}{},
	}
}

// appCreateResourceNode 返回服务创建动作的资源节点：
// 新拓扑按目标环境授权，旧拓扑按业务授权（app_create 只关联业务实例）。
func appCreateResourceNode(a *meta.ResourceAttribute, topology meta.ResourceTopology) iam.ResourceNode {
	if topology.IsLegacy() {
		return bizResourceNode(a.BizID)
	}

	return environmentResourceNode(a, topology)
}

// projectClientResource 构造项目实例资源。
func projectClientResource(a *meta.ResourceAttribute) client.Resource {
	return client.Resource{
		System: sys.SystemIDBSCP,
		Type:   sys.Project,
		ID:     strconv.FormatUint(uint64(a.ResourceID), 10),
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{projectIAMPath(a)},
		},
	}
}

// environmentClientResource 构造环境实例资源。
func environmentClientResource(a *meta.ResourceAttribute, topology meta.ResourceTopology) client.Resource {
	return client.Resource{
		System: sys.SystemIDBSCP,
		Type:   sys.Environment,
		ID:     strconv.FormatUint(uint64(a.ResourceID), 10),
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{environmentIAMPath(a, topology)},
		},
	}
}

// genProjectResource 项目按项目实例隔离：创建按业务授权（此时项目实例还不存在），
// 查看/编辑/删除按项目实例授权，列表按业务访问放行。
func genProjectResource(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	switch a.Action {
	case meta.Create:
		return sys.ProjectCreate, []client.Resource{bizRes}, nil
	case meta.Find:
		return sys.BusinessViewResource, []client.Resource{bizRes}, nil
	case meta.View:
		return sys.ProjectView, []client.Resource{projectClientResource(a)}, nil
	case meta.Update:
		return sys.ProjectEdit, []client.Resource{projectClientResource(a)}, nil
	case meta.Delete:
		return sys.ProjectDelete, []client.Resource{projectClientResource(a)}, nil
	default:
		return "", nil, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}
}

func genProjectIAMResource(a *meta.ResourceAttribute) (*bkiam.Request, error) {
	iamReq := bkiam.Request{
		System:  sys.SystemIDBSCP,
		Subject: dummyIAMUser,
		Resources: []iam.ResourceNode{
			{
				System:    sys.SystemIDCMDB,
				Type:      string(sys.Business),
				ID:        strconv.FormatUint(uint64(a.BizID), 10),
				Attribute: map[string]interface{}{},
			},
		},
	}

	switch a.Action {
	case meta.Create:
		iamReq.Action = bkiam.NewAction(string(sys.ProjectCreate))
	case meta.Find:
		iamReq.Action = bkiam.NewAction(string(sys.BusinessViewResource))
	case meta.View:
		iamReq.Action = bkiam.NewAction(string(sys.ProjectView))
		iamReq.Resources = []iam.ResourceNode{projectResourceNode(a)}
	case meta.Update:
		iamReq.Action = bkiam.NewAction(string(sys.ProjectEdit))
		iamReq.Resources = []iam.ResourceNode{projectResourceNode(a)}
	case meta.Delete:
		iamReq.Action = bkiam.NewAction(string(sys.ProjectDelete))
		iamReq.Resources = []iam.ResourceNode{projectResourceNode(a)}
	default:
		return nil, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}

	return &iamReq, nil
}

func genProjectIAMApplication(a *meta.ResourceAttribute) (bkiam.ApplicationAction, error) {
	action := bkiam.ApplicationAction{
		RelatedResourceTypes: []bkiam.ApplicationRelatedResourceType{{
			SystemID: sys.SystemIDCMDB,
			Type:     string(sys.Business),
			Instances: []bkiam.ApplicationResourceInstance{
				[]iam.ApplicationResourceNode{{
					Type: string(sys.Business),
					ID:   strconv.FormatUint(uint64(a.BizID), 10),
				}},
			},
		}},
	}

	switch a.Action {
	case meta.Create:
		action.ID = string(sys.ProjectCreate)
	case meta.Find:
		// 列表按业务访问放行，申请链接指向业务即可。
		action.ID = string(sys.BusinessViewResource)
	case meta.View:
		action.ID = string(sys.ProjectView)
		action.RelatedResourceTypes = projectApplicationResource(a)
	case meta.Update:
		action.ID = string(sys.ProjectEdit)
		action.RelatedResourceTypes = projectApplicationResource(a)
	case meta.Delete:
		action.ID = string(sys.ProjectDelete)
		action.RelatedResourceTypes = projectApplicationResource(a)
	default:
		return action, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}

	return action, nil
}

// projectApplicationResource 构造申请链接用的项目资源，祖先链只到业务一层。
func projectApplicationResource(a *meta.ResourceAttribute) []bkiam.ApplicationRelatedResourceType {
	return []bkiam.ApplicationRelatedResourceType{{
		SystemID: sys.SystemIDBSCP,
		Type:     string(sys.Project),
		Instances: []bkiam.ApplicationResourceInstance{
			[]iam.ApplicationResourceNode{
				{
					Type: string(sys.Business),
					ID:   strconv.FormatUint(uint64(a.BizID), 10),
				},
				{
					Type: string(sys.Project),
					ID:   strconv.FormatUint(uint64(a.ResourceID), 10),
				},
			},
		},
	}}
}

// genEnvResource 环境按环境实例隔离：创建按所属项目授权（此时环境实例还不存在），
// 查看/编辑/删除按环境实例授权。
func genEnvResource(a *meta.ResourceAttribute, topology meta.ResourceTopology) (
	client.ActionID, []client.Resource, error) {

	// 创建环境时还没有环境实例，鉴权落在所属项目上。
	projectRes := client.Resource{
		System: sys.SystemIDBSCP,
		Type:   sys.Project,
		ID:     strconv.FormatUint(uint64(a.ProjectID), 10),
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{projectIAMPath(a)},
		},
	}
	envRes := environmentClientResource(a, topology)

	switch a.Action {
	case meta.Create:
		return sys.EnvironmentCreate, []client.Resource{projectRes}, nil
	case meta.View:
		return sys.EnvironmentView, []client.Resource{envRes}, nil
	case meta.Update:
		return sys.EnvironmentEdit, []client.Resource{envRes}, nil
	case meta.Delete:
		return sys.EnvironmentDelete, []client.Resource{envRes}, nil
	default:
		return "", nil, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}
}

func genEnvIAMResource(a *meta.ResourceAttribute, topology meta.ResourceTopology) (*bkiam.Request, error) {
	iamReq := bkiam.Request{
		System:  sys.SystemIDBSCP,
		Subject: dummyIAMUser,
	}

	switch a.Action {
	case meta.Create:
		// 创建按所属项目授权，项目实例节点用 ProjectID 而不是 ResourceID。
		iamReq.Action = bkiam.NewAction(string(sys.EnvironmentCreate))
		iamReq.Resources = []iam.ResourceNode{{
			System: sys.SystemIDBSCP,
			Type:   string(sys.Project),
			ID:     strconv.FormatUint(uint64(a.ProjectID), 10),
			Attribute: map[string]interface{}{
				client.IamPathKey: []string{projectIAMPath(a)},
			},
		}}
	case meta.View:
		iamReq.Action = bkiam.NewAction(string(sys.EnvironmentView))
		iamReq.Resources = []iam.ResourceNode{environmentResourceNode(a, topology)}
	case meta.Update:
		iamReq.Action = bkiam.NewAction(string(sys.EnvironmentEdit))
		iamReq.Resources = []iam.ResourceNode{environmentResourceNode(a, topology)}
	case meta.Delete:
		iamReq.Action = bkiam.NewAction(string(sys.EnvironmentDelete))
		iamReq.Resources = []iam.ResourceNode{environmentResourceNode(a, topology)}
	default:
		return nil, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}

	return &iamReq, nil
}

func genEnvIAMApplication(a *meta.ResourceAttribute) (bkiam.ApplicationAction, error) {
	action := bkiam.ApplicationAction{}

	switch a.Action {
	case meta.Create:
		action.ID = string(sys.EnvironmentCreate)
		action.RelatedResourceTypes = []bkiam.ApplicationRelatedResourceType{{
			SystemID: sys.SystemIDBSCP,
			Type:     string(sys.Project),
			Instances: []bkiam.ApplicationResourceInstance{
				[]iam.ApplicationResourceNode{
					{
						Type: string(sys.Business),
						ID:   strconv.FormatUint(uint64(a.BizID), 10),
					},
					{
						Type: string(sys.Project),
						ID:   strconv.FormatUint(uint64(a.ProjectID), 10),
					},
				},
			},
		}}
	case meta.View:
		action.ID = string(sys.EnvironmentView)
		action.RelatedResourceTypes = environmentApplicationResource(a)
	case meta.Update:
		action.ID = string(sys.EnvironmentEdit)
		action.RelatedResourceTypes = environmentApplicationResource(a)
	case meta.Delete:
		action.ID = string(sys.EnvironmentDelete)
		action.RelatedResourceTypes = environmentApplicationResource(a)
	default:
		return action, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}

	return action, nil
}

// environmentApplicationResource 构造申请链接用的环境资源，祖先链为「业务 → 项目」。
func environmentApplicationResource(a *meta.ResourceAttribute) []bkiam.ApplicationRelatedResourceType {
	return []bkiam.ApplicationRelatedResourceType{{
		SystemID: sys.SystemIDBSCP,
		Type:     string(sys.Environment),
		Instances: []bkiam.ApplicationResourceInstance{
			[]iam.ApplicationResourceNode{
				{
					Type: string(sys.Business),
					ID:   strconv.FormatUint(uint64(a.BizID), 10),
				},
				{
					Type: string(sys.Project),
					ID:   strconv.FormatUint(uint64(a.ProjectID), 10),
				},
				{
					Type: string(sys.Environment),
					ID:   strconv.FormatUint(uint64(a.ResourceID), 10),
				},
			},
		},
	}}
}
