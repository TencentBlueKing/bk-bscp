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

func genAppIAMResource(a *meta.ResourceAttribute) (*bkiam.Request, error) {
	// 服务创建按目标环境授权：此时还没有服务实例，资源节点用环境而不是服务。
	if a.Action == meta.Create {
		return &bkiam.Request{
			System:    sys.SystemIDBSCP,
			Subject:   dummyIAMUser,
			Action:    bkiam.NewAction(string(sys.AppCreate)),
			Resources: []iam.ResourceNode{environmentResourceNode(a)},
		}, nil
	}

	iamReq := bkiam.Request{
		System:  sys.SystemIDBSCP,
		Subject: dummyIAMUser,
		Resources: []iam.ResourceNode{
			appResourceNode(a),
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

func genAppIAMApplication(a *meta.ResourceAttribute) (bkiam.ApplicationAction, error) {
	action := bkiam.ApplicationAction{}
	resourceNodes := []iam.ApplicationResourceNode{}
	resourceNodeBiz := iam.ApplicationResourceNode{
		Type: string(sys.Business),
		ID:   strconv.FormatUint(uint64(a.BizID), 10),
	}
	resourceNodeProject := iam.ApplicationResourceNode{
		Type: string(sys.Project),
		ID:   strconv.FormatUint(uint64(a.ProjectID), 10),
	}
	resourceNodeEnv := iam.ApplicationResourceNode{
		Type: string(sys.Environment),
		ID:   strconv.FormatUint(uint64(a.EnvironmentID), 10),
	}
	resourceNodeApp := iam.ApplicationResourceNode{
		Type: string(sys.Application),
		ID:   strconv.FormatUint(uint64(a.ResourceID), 10),
	}

	switch a.Action {
	case meta.Create:
		// 服务创建按目标环境授权，申请链接的拓扑是「业务 → 项目 → 环境」。
		action.ID = string(sys.AppCreate)
		resourceNodes = append(resourceNodes, resourceNodeBiz, resourceNodeProject, resourceNodeEnv)
		action.RelatedResourceTypes = []bkiam.ApplicationRelatedResourceType{
			{
				SystemID:  sys.SystemIDBSCP,
				Type:      string(sys.Environment),
				Instances: []bkiam.ApplicationResourceInstance{resourceNodes},
			},
		}
		return action, nil
	case meta.View:
		action.ID = string(sys.AppView)
		resourceNodes = append(resourceNodes, resourceNodeBiz, resourceNodeProject, resourceNodeEnv, resourceNodeApp)
	case meta.Update:
		action.ID = string(sys.AppEdit)
		resourceNodes = append(resourceNodes, resourceNodeBiz, resourceNodeProject, resourceNodeEnv, resourceNodeApp)
	case meta.Delete:
		action.ID = string(sys.AppDelete)
		resourceNodes = append(resourceNodes, resourceNodeBiz, resourceNodeProject, resourceNodeEnv, resourceNodeApp)
	case meta.Publish:
		action.ID = string(sys.ReleasePublish)
		resourceNodes = append(resourceNodes, resourceNodeBiz, resourceNodeProject, resourceNodeEnv, resourceNodeApp)
	case meta.GenerateRelease:
		action.ID = string(sys.ReleaseGenerate)
		resourceNodes = append(resourceNodes, resourceNodeBiz, resourceNodeProject, resourceNodeEnv, resourceNodeApp)
	default:
		return action, fmt.Errorf("unsupported bscp action: %s", a.Action)
	}
	action.RelatedResourceTypes = []bkiam.ApplicationRelatedResourceType{
		{
			SystemID:  sys.SystemIDBSCP,
			Type:      string(sys.Application),
			Instances: []bkiam.ApplicationResourceInstance{resourceNodes},
		},
	}
	return action, nil
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
func genAppResource(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	bizRes := client.Resource{
		System: sys.SystemIDCMDB,
		Type:   sys.Business,
		ID:     strconv.FormatUint(uint64(a.BizID), 10),
	}

	// 服务挂在环境下，鉴权路径为 业务 → 项目 → 环境 三级，
	// 按业务、项目或环境维度的授权都能据此覆盖到该服务。
	appRes := client.Resource{
		System: sys.SystemIDBSCP,
		Type:   sys.Application,
		ID:     strconv.FormatUint(uint64(a.ResourceID), 10),
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{appIAMPath(a)},
		},
	}

	switch a.Action {
	case meta.Create:
		// 服务创建按目标环境授权：授权维度落在服务将要归属的环境上（ResourceID 即环境 ID），
		// 路径为 /biz,{bizID}/project,{projectID}/，业务/项目维度的授权经继承覆盖。
		return sys.AppCreate, []client.Resource{environmentClientResource(a)}, nil
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
	return fmt.Sprintf("/%s,%d/", sys.Business, a.BizID)
}

// environmentIAMPath 返回环境实例的拓扑路径，声明「业务 → 项目」两级拓扑，
// 按业务或项目维度的授权据此覆盖项目下的全部环境。
func environmentIAMPath(a *meta.ResourceAttribute) string {
	return fmt.Sprintf("/%s,%d/%s,%d/", sys.Business, a.BizID, sys.Project, a.ProjectID)
}

// appIAMPath 返回服务实例的拓扑路径，声明「业务 → 项目 → 环境」三级拓扑，
// 按业务、项目或环境维度的授权据此覆盖环境下的全部服务。
func appIAMPath(a *meta.ResourceAttribute) string {
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
func environmentResourceNode(a *meta.ResourceAttribute) iam.ResourceNode {
	return iam.ResourceNode{
		System: sys.SystemIDBSCP,
		Type:   string(sys.Environment),
		ID:     strconv.FormatUint(uint64(a.ResourceID), 10),
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{environmentIAMPath(a)},
		},
	}
}

// appResourceNode 构造服务实例节点。
func appResourceNode(a *meta.ResourceAttribute) iam.ResourceNode {
	return iam.ResourceNode{
		System: sys.SystemIDBSCP,
		Type:   string(sys.Application),
		ID:     strconv.FormatUint(uint64(a.ResourceID), 10),
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{appIAMPath(a)},
		},
	}
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
func environmentClientResource(a *meta.ResourceAttribute) client.Resource {
	return client.Resource{
		System: sys.SystemIDBSCP,
		Type:   sys.Environment,
		ID:     strconv.FormatUint(uint64(a.ResourceID), 10),
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{environmentIAMPath(a)},
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
func genEnvResource(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	// 创建环境时还没有环境实例，鉴权落在所属项目上。
	projectRes := client.Resource{
		System: sys.SystemIDBSCP,
		Type:   sys.Project,
		ID:     strconv.FormatUint(uint64(a.ProjectID), 10),
		Attribute: map[string]interface{}{
			client.IamPathKey: []string{projectIAMPath(a)},
		},
	}
	envRes := environmentClientResource(a)

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

func genEnvIAMResource(a *meta.ResourceAttribute) (*bkiam.Request, error) {
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
		iamReq.Resources = []iam.ResourceNode{environmentResourceNode(a)}
	case meta.Update:
		iamReq.Action = bkiam.NewAction(string(sys.EnvironmentEdit))
		iamReq.Resources = []iam.ResourceNode{environmentResourceNode(a)}
	case meta.Delete:
		iamReq.Action = bkiam.NewAction(string(sys.EnvironmentDelete))
		iamReq.Resources = []iam.ResourceNode{environmentResourceNode(a)}
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
