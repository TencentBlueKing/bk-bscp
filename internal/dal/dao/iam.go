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

package dao

import (
	"fmt"
	"strconv"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/orm"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/sharding"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// IAM only used to iam to pull resource callback.
type IAM interface {
	// ListInstances list instances with options.
	ListInstances(kt *kit.Kit, opts *types.ListInstancesOption) (*types.ListInstanceDetails, error)
	// FetchInstanceInfo fetch instance info with options.
	FetchInstanceInfo(kt *kit.Kit, opts *types.FetchInstanceInfoOption) (*types.FetchInstanceInfoDetails, error)
}

var _ IAM = new(iamDao)

type iamDao struct {
	orm  orm.Interface
	genQ *gen.Query
	sd   *sharding.Sharding
}

// ListInstances list instances with options.
func (r *iamDao) ListInstances(kt *kit.Kit, opts *types.ListInstancesOption) (
	*types.ListInstanceDetails, error) {

	if opts == nil {
		return nil, errf.New(errf.InvalidParameter, "list instances options is null")
	}

	// enable unlimited query, because this is iam pull resource callback.
	po := &types.PageOption{MaxLimit: client.BkIAMMaxPageSize}
	if err := opts.Validate(po); err != nil {
		return nil, err
	}

	var (
		count   int64
		details []*types.InstanceResource
	)
	switch opts.ResourceType {
	case meta.App.String():
		// 服务挂在环境下，权限中心展开资源树时会按环境拉取可选服务；
		// 但业务/项目层级的展示配置仍会按上层拉取，需要一并支持，否则授权页面直接报错。
		m := r.genQ.App
		q := m.WithContext(kt.Ctx).
			Select(m.ID.As("id"), m.Name.As("name"))
		switch opts.ParentType {
		case meta.Biz.String():
			bizID, err := strconv.ParseUint(opts.ParentID, 10, 32)
			if err != nil {
				return nil, err
			}
			q = q.Where(m.BizID.Eq(uint32(bizID)))
		case meta.Project.String():
			projectID, err := strconv.ParseUint(opts.ParentID, 10, 32)
			if err != nil {
				return nil, err
			}
			q = q.Where(m.ProjID.Eq(uint32(projectID)))
		case meta.Environment.String():
			envID, err := strconv.ParseUint(opts.ParentID, 10, 32)
			if err != nil {
				return nil, err
			}
			// 服务必须挂在具体环境下，environment_id 不存在 0 值约定，按环境精确匹配。
			q = q.Where(m.EnvID.Eq(uint32(envID)))
		default:
			return nil, fmt.Errorf("unexpected parent type for app: %s", opts.ParentType)
		}
		if opts.Keyword != "" {
			pattern := "%" + opts.Keyword + "%"
			kw := m.WithContext(kt.Ctx).Where(m.Name.Like(pattern))
			if id, parseErr := strconv.ParseUint(opts.Keyword, 10, 32); parseErr == nil {
				kw = kw.Or(m.ID.Eq(uint32(id)))
			}
			q = q.Where(kw)
		}
		var err error
		count, err = q.ScanByPage(&details, opts.Page.Offset(), opts.Page.LimitInt())
		if err != nil {
			return nil, err
		}
	case meta.Project.String():
		// 项目挂在业务下，权限中心按业务维度拉取可选项目，因此 parent 必须是业务。
		bizID, err := strconv.ParseUint(opts.ParentID, 10, 32)
		if err != nil {
			return nil, err
		}
		m := r.genQ.Project
		q := m.WithContext(kt.Ctx).
			Select(m.ID.As("id"), m.Name.As("name")).Where(m.BizID.Eq(uint32(bizID)))
		if opts.Keyword != "" {
			pattern := "%" + opts.Keyword + "%"
			kw := m.WithContext(kt.Ctx).Where(m.Name.Like(pattern))
			if id, parseErr := strconv.ParseUint(opts.Keyword, 10, 32); parseErr == nil {
				kw = kw.Or(m.ID.Eq(uint32(id)))
			}
			q = q.Where(kw)
		}
		count, err = q.ScanByPage(&details, opts.Page.Offset(), opts.Page.LimitInt())
		if err != nil {
			return nil, err
		}
	case meta.Environment.String():
		// 环境挂在项目下，权限中心按项目维度拉取可选环境，因此 parent 必须是项目。
		projectID, err := strconv.ParseUint(opts.ParentID, 10, 32)
		if err != nil {
			return nil, err
		}
		m := r.genQ.Environment
		q := m.WithContext(kt.Ctx).
			Select(m.ID.As("id"), m.Name.As("name")).Where(m.ProjectID.Eq(uint32(projectID)))
		if opts.Keyword != "" {
			pattern := "%" + opts.Keyword + "%"
			kw := m.WithContext(kt.Ctx).Where(m.Name.Like(pattern))
			if id, parseErr := strconv.ParseUint(opts.Keyword, 10, 32); parseErr == nil {
				kw = kw.Or(m.ID.Eq(uint32(id)))
			}
			q = q.Where(kw)
		}
		count, err = q.ScanByPage(&details, opts.Page.Offset(), opts.Page.LimitInt())
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid resource type %s", opts.ResourceType)
	}

	return &types.ListInstanceDetails{Count: uint32(count), Details: details}, nil
}

// FetchInstanceInfo fetch instance info with options.
func (r *iamDao) FetchInstanceInfo(kt *kit.Kit, opts *types.FetchInstanceInfoOption) (
	*types.FetchInstanceInfoDetails, error) {

	if opts == nil {
		return nil, errf.New(errf.InvalidParameter, "fetch instance info options is null")
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	var details []*types.InstanceInfo
	switch opts.ResourceType {
	case meta.App.String():
		ids, err := tools.StringSliceToUint32Slice(opts.IDs)
		if err != nil {
			return nil, err
		}
		m := r.genQ.App
		apps, err := m.WithContext(kt.Ctx).Where(m.ID.In(ids...)).Find()
		if err != nil {
			return nil, err
		}
		for _, app := range apps {
			// 服务的 _bk_iam_path_ 是 /biz,{bizID}/project,{projectID}/environment,{envID}/：
			// 归属直接取自服务自身，按业务、项目或环境维度的授权据此覆盖到该服务。
			detail := &types.InstanceInfo{
				ID:          strconv.Itoa(int(app.ID)),
				DisplayName: app.Spec.Name,
				Approver:    []string{},
				Path: []string{fmt.Sprintf("/%s,%d/%s,%d/%s,%d/",
					sys.Business, app.BizID, sys.Project, app.ProjID, sys.Environment, app.EnvID)},
			}
			if app.Revision.Creator != "" {
				detail.Approver = append(detail.Approver, app.Revision.Creator)
			}
			if app.Revision.Reviser != "" {
				detail.Approver = append(detail.Approver, app.Revision.Reviser)
			}
			details = append(details, detail)
		}
	case meta.Project.String():
		// 项目的 _bk_iam_path_ 是 /biz,{bizID}/，按业务维度的授权据此覆盖该项目。
		ids, err := tools.StringSliceToUint32Slice(opts.IDs)
		if err != nil {
			return nil, err
		}
		m := r.genQ.Project
		projects, err := m.WithContext(kt.Ctx).Where(m.ID.In(ids...)).Find()
		if err != nil {
			return nil, err
		}
		for _, project := range projects {
			detail := &types.InstanceInfo{
				ID:          strconv.Itoa(int(project.ID)),
				DisplayName: project.Spec.Name,
				Approver:    []string{},
				Path:        []string{fmt.Sprintf("/%s,%d/", sys.Business, project.Attachment.BizID)},
			}
			if project.Revision.Creator != "" {
				detail.Approver = append(detail.Approver, project.Revision.Creator)
			}
			if project.Revision.Reviser != "" {
				detail.Approver = append(detail.Approver, project.Revision.Reviser)
			}
			details = append(details, detail)
		}
	case meta.Environment.String():
		// 环境的 _bk_iam_path_ 是 /biz,{bizID}/project,{projectID}/：
		// 按业务或项目维度的授权据此覆盖该项目下的环境。
		ids, err := tools.StringSliceToUint32Slice(opts.IDs)
		if err != nil {
			return nil, err
		}
		m := r.genQ.Environment
		envs, err := m.WithContext(kt.Ctx).Where(m.ID.In(ids...)).Find()
		if err != nil {
			return nil, err
		}
		for _, env := range envs {
			detail := &types.InstanceInfo{
				ID:          strconv.Itoa(int(env.ID)),
				DisplayName: env.Spec.Name,
				Approver:    []string{},
				Path: []string{fmt.Sprintf("/%s,%d/%s,%d/",
					sys.Business, env.Attachment.BizID, sys.Project, env.Attachment.ProjectID)},
			}
			if env.Revision.Creator != "" {
				detail.Approver = append(detail.Approver, env.Revision.Creator)
			}
			if env.Revision.Reviser != "" {
				detail.Approver = append(detail.Approver, env.Revision.Reviser)
			}
			details = append(details, detail)
		}
	default:
		return nil, fmt.Errorf("invalid resource type %s", opts.ResourceType)
	}

	return &types.FetchInstanceInfoDetails{Details: details}, nil
}
