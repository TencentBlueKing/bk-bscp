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
	"errors"
	"fmt"

	bkiam "github.com/TencentBlueKing/iam-go-sdk"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
)

// PermissionOptions 控制资源映射时启用的权限维度。
type PermissionOptions struct {
	// ProjectEnvironmentAuth 是否启用项目与环境维度的权限校验。
	// 关闭时项目、环境资源不参与鉴权；服务创建仍校验，但实例仍用目标环境、
	// 路径只声明业务一层；服务其它操作的鉴权路径同样退化为业务单级。
	ProjectEnvironmentAuth bool
}

// defaultPermissionOptions 返回默认开启项目/环境维度校验的选项。
func defaultPermissionOptions() PermissionOptions {
	return PermissionOptions{ProjectEnvironmentAuth: true}
}

// AdaptAuthOptions convert bscp auth resource to iam action id and resources
func AdaptAuthOptions(a *meta.ResourceAttribute) (client.ActionID, []client.Resource, error) {
	return AdaptAuthOptionsWithOptions(a, defaultPermissionOptions())
}

// AdaptAuthOptionsWithOptions 按指定的权限维度把 bscp 资源映射为 iam 动作与资源。
func AdaptAuthOptionsWithOptions(a *meta.ResourceAttribute, opts PermissionOptions) (
	client.ActionID, []client.Resource, error) {
	if a == nil {
		return "", nil, errf.New(errf.InvalidParameter, "resource attribute is not set")
	}

	// skip actions do not need to relate to resources
	if a.Action == meta.SkipAction {
		return genSkipResource(a)
	}

	// 项目/环境维度校验关闭时，这两类资源不进入权限中心：
	// 调用方已有的业务维度校验就是该场景下的全部约束。
	if !opts.ProjectEnvironmentAuth {
		switch a.Type {
		case meta.Project, meta.Environment:
			return genSkipResource(a)
		}
	}

	switch a.Type {
	case meta.Biz:
		return genBizResource(a)
	case meta.App:
		return genAppResourceWithOptions(a, opts)
	case meta.Credential:
		return genCredResource(a)
	case meta.Audit:
		return genAuditResource(a)
	case meta.Commit, meta.ConfigItem, meta.Content, meta.CRInstance, meta.Release, meta.ReleasedCI, meta.Strategy,
		meta.StrategySet, meta.PSH, meta.Repo, meta.Sidecar:
		return genSkipResource(a)
	case meta.ProcConfigMgmt:
		return genProcConfigMgmtResource(a)
	case meta.Project:
		return genProjectResource(a)
	case meta.Environment:
		return genEnvResource(a)
	// case meta.Commit:
	//	return genCommitResource(a)
	// case meta.ConfigItem:
	//	return genConfigItemResource(a)
	// case meta.Content:
	//	return genContentResource(a)
	// case meta.CRInstance:
	//	return genCRInstanceResource(a)
	// case meta.Release:
	//	return genReleaseRes(a)
	// case meta.ReleasedCI:
	//	return genReleasedCIRes(a)
	// case meta.Strategy:
	//	return genStrategyRes(a)
	// case meta.StrategySet:
	//	return genStrategySetRes(a)
	// case meta.PSH:
	//	return genPSHRes(a)
	// case meta.Repo:
	//	return genRepoRes(a)
	// case meta.Sidecar:
	//	return genSidecarRes(a)
	// case meta.Credential:
	//	return genCredentialRes(a)

	default:
		return "", nil, errf.New(errf.InvalidParameter, fmt.Sprintf("unsupported bscp auth type: %s", a.Type))
	}
}

// AdaptIAMResourceOptions 鉴权, 查看 isAllow 接口使用
func AdaptIAMResourceOptions(a *meta.ResourceAttribute) (*bkiam.Request, error) {
	if a == nil {
		return nil, errors.New("resource attribute is not set")
	}

	switch a.Type {
	case meta.Biz:
		return genBizIAMResource(a)
	case meta.App:
		return genAppIAMResource(a)
	case meta.Credential:
		return genCredIAMResource(a)
	case meta.ProcConfigMgmt:
		return genProcConfigMgmtRes(a)
	case meta.Project:
		return genProjectIAMResource(a)
	case meta.Environment:
		return genEnvIAMResource(a)
	default:
		return nil, fmt.Errorf("unsupported bscp auth type: %s", a.Type)
	}
}

// AdaptIAMApplicationOptions 申请链接, applyURL 接口使用
func AdaptIAMApplicationOptions(as []*meta.ResourceAttribute, opts PermissionOptions) (*bkiam.Application, error) {
	if len(as) == 0 {
		return nil, errors.New("resource attribute is not set")
	}
	actions := make([]bkiam.ApplicationAction, 0)
	for _, a := range as {
		// 关闭项目/环境维度校验时，这两类资源不参与申请链接：申请了也不会被校验。
		if !opts.ProjectEnvironmentAuth && (a.Type == meta.Project || a.Type == meta.Environment) {
			continue
		}

		switch a.Type {
		case meta.Biz:
			action, err := genBizIAMApplication(a)
			if err != nil {
				return nil, err
			}
			actions = append(actions, action)
		case meta.App:
			action, err := genAppIAMApplicationWithOptions(a, opts)
			if err != nil {
				return nil, err
			}
			// 关闭项目/环境维度后服务创建不校验，也就没有可申请的权限。
			if action.ID == "" {
				continue
			}
			actions = append(actions, action)
		case meta.Credential:
			action, err := genCredIAMApplication(a)
			if err != nil {
				return nil, err
			}
			actions = append(actions, action)
		case meta.Audit:
			action, err := genAuditIAMApplication(a)
			if err != nil {
				return nil, err
			}
			actions = append(actions, action)
		case meta.ProcConfigMgmt:
			action, err := genProcConfigMgmtApplication(a)
			if err != nil {
				return nil, err
			}
			actions = append(actions, action)
		case meta.Project:
			action, err := genProjectIAMApplication(a)
			if err != nil {
				return nil, err
			}
			actions = append(actions, action)
		case meta.Environment:
			action, err := genEnvIAMApplication(a)
			if err != nil {
				return nil, err
			}
			actions = append(actions, action)
		default:
			return nil, fmt.Errorf("unsupported bscp auth type: %s", a.Type)
		}
	}
	// 全部资源都被跳过时没有必要生成申请链接，交由调用方返回空链接。
	if len(actions) == 0 {
		return nil, nil
	}

	application := bkiam.NewApplication(sys.SystemIDBSCP, actions)
	return &application, nil
}
