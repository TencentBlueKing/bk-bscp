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

package iamv4

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"

	"github.com/TencentBlueKing/bk-bscp/internal/space"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/model"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// spaceLister 提供业务实例数据。抽成接口是为了让回调逻辑能脱离 CMDB 单测。
type spaceLister interface {
	// AllSpaces 返回当前租户的全量业务，内部按租户缓存
	AllSpaces(ctx context.Context) []*space.Space
	// QuerySpace 按业务 ID 批量查询
	QuerySpace(ctx context.Context, bizIDs []string) ([]*space.Space, error)
}

// appLister 提供 IAM 资源实例数据（服务、项目等），由 data-service 的 gRPC client 实现。
type appLister interface {
	ListInstances(ctx context.Context, in *pbds.ListInstancesReq, opts ...grpc.CallOption) (
		*pbds.ListInstancesResp, error)
	FetchInstanceInfo(ctx context.Context, in *pbds.FetchInstanceInfoReq, opts ...grpc.CallOption) (
		*pbds.FetchInstanceInfoResp, error)
}

var (
	_ spaceLister = new(space.Manager)
	_ appLister   = pbds.DataClient(nil)
)

// IAM 实现 V4 的资源实例回调。
type IAM struct {
	apps appLister
	bizs spaceLister
}

// NewIAM 构造 V4 回调处理器。
func NewIAM(apps appLister, bizs spaceLister) (*IAM, error) {
	if apps == nil {
		return nil, errors.New("app lister is nil")
	}

	if bizs == nil {
		return nil, errors.New("space lister is nil")
	}

	return &IAM{apps: apps, bizs: bizs}, nil
}

// PullResource 是回调的统一入口，按 method 与 type 分发。
// 返回值直接作为响应体的 data 字段，调用方负责包裹成 {"data": ...}。
func (i *IAM) PullResource(kt *kit.Kit, req *PullResourceReq) (any, error) {
	switch req.Method {
	case MethodListInstance:
		return i.listInstance(kt, req)
	case MethodFetchInstanceInfo:
		return i.fetchInstanceInfo(kt, req)
	default:
		return nil, fmt.Errorf("unsupported callback method: %s", req.Method)
	}
}

func (i *IAM) listInstance(kt *kit.Kit, req *PullResourceReq) (*ListInstanceResult, error) {
	switch req.Type {
	case model.ResourceTypeBiz:
		return i.listBizInstances(kt, req.Filter, req.Page)
	case model.ResourceTypeApp:
		return i.listAppInstances(kt, req.Filter, req.Page)
	case model.ResourceTypeProject:
		return i.listProjectInstances(kt, req.Filter, req.Page)
	case model.ResourceTypeEnvironment:
		return i.listEnvironmentInstances(kt, req.Filter, req.Page)
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", req.Type)
	}
}

func (i *IAM) fetchInstanceInfo(kt *kit.Kit, req *PullResourceReq) ([]instanceInfo, error) {
	if len(req.Filter.IDs) == 0 {
		return []instanceInfo{}, nil
	}

	selector := newAttrSelector(req.Requires)

	switch req.Type {
	case model.ResourceTypeBiz:
		return i.fetchBizInstanceInfo(kt, req.Filter.IDs, selector)
	case model.ResourceTypeApp:
		return i.fetchAppInstanceInfo(kt, req.Filter.IDs, selector)
	case model.ResourceTypeProject:
		return i.fetchProjectInstanceInfo(kt, req.Filter.IDs, selector)
	case model.ResourceTypeEnvironment:
		return i.fetchEnvironmentInstanceInfo(kt, req.Filter.IDs, selector)
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", req.Type)
	}
}

// matchKeyword 按 display_name 与 id 做包含式匹配。
// 协议只要求支持 display_name，这里连 id 一起匹配以便用户直接输入业务/服务 ID 搜索。
func matchKeyword(keyword, id, displayName string) bool {
	if keyword == "" {
		return true
	}

	return contains(displayName, keyword) || contains(id, keyword)
}
