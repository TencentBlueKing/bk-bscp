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

// Package bkcmdb provides bkcmdb client.
package bkcmdb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-resty/resty/v2"

	"github.com/TencentBlueKing/bk-bscp/internal/components"
	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/cmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/types"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// nolint: unused
var (
	searchBusiness                   = "%s/api/bk-cmdb/prod/api/v3/biz/search/bk_supplier_account"
	findHostByTopo                   = "%s/api/bk-cmdb/prod/api/v3/findmany/hosts/by_topo/biz/%d"
	searchBizInstTopo                = "%s/api/bk-cmdb/prod/api/v3/find/topoinst/biz/%d"
	getServiceTemplate               = "%s/api/bk-cmdb/prod/api/v3/find/proc/service_template/%d"
	listServiceTemplate              = "%s/api/bk-cmdb/prod/api/v3/findmany/proc/service_template"
	getProcTemplate                  = "%s/api/bk-cmdb/prod/api/v3/find/proc/proc_template/id/%d"
	listProcTemplate                 = "%s/api/bk-cmdb/prod/api/v3/findmany/proc/proc_template"
	listProcessInstance              = "%s/api/bk-cmdb/prod/api/v3/findmany/proc/process_instance"
	findHostBySetTemplate            = "%s/api/bk-cmdb/prod/api/v3/findmany/hosts/by_set_templates/biz/%d"
	listSetTemplate                  = "%s/api/bk-cmdb/prod/api/v3/findmany/topo/set_template/bk_biz_id/%d"
	listProcessDetailByIds           = "%s/api/bk-cmdb/prod/api/v3/findmany/proc/process_instance/detail/biz/%d"
	listServiceInstanceBySetTemplate = "%s/api/bk-cmdb/prod/api/v3/findmany/proc/service/" +
		"set_template/list_service_instance/biz/%d"
	findModuleBatch     = "%s/api/bk-cmdb/prod/api/v3/findmany/module/bk_biz_id/%d"
	listServiceInstance = "%s/api/bk-cmdb/prod/api/v3/findmany/proc/service_instance"

	findSetBatch         = "%s/api/bk-cmdb/prod/api/v3/findmany/set/bk_biz_id/%d"
	searchSet            = "%s/api/bk-cmdb/prod/api/v3/set/search/%s/%d"
	searchModule         = "%s/api/bk-cmdb/prod/api/v3/module/search/%s/%d/%d"
	findHostTopoRelation = "%s/api/bk-cmdb/prod/api/v3/host/topo/relation/read"
	listBizHosts         = "%s/prod/api/v3/hosts/app/%d/list_hosts"
	watchResource        = "%s/prod/api/v3/event/watch/resource/%s"
	findHostBizRelations = "%s/prod/api/v3/hosts/modules/read"
)

type HTTPMethod string

const (
	GET  HTTPMethod = "GET"
	POST HTTPMethod = "POST"
)

// CMDBService bkcmdb client
type CMDBService struct {
	*cc.CMDBConfig
}

func (bkcmdb *CMDBService) doRequest(ctx context.Context, method HTTPMethod, url string, body any, result any) error {
	authHeader := components.MakeBKAPIGWAuthHeader(
		bkcmdb.AppCode,
		bkcmdb.AppSecret,
		components.WithBkUsername(bkcmdb.BkUserName),
	)

	// 构造请求
	request := components.GetClient().R().
		SetContext(ctx).
		SetHeader("X-Bkapi-Authorization", authHeader).
		SetBody(body)

	// 执行请求
	var resp *resty.Response
	var err error

	switch method {
	case GET:
		resp, err = request.Get(url)
	case POST:
		resp, err = request.Post(url)
	default:
		return fmt.Errorf("%s method not defined", method)
	}

	if err != nil {
		return err
	}

	// 统一反序列化结果
	// if err := components.UnmarshalBKResult(resp, result); err != nil {
	// 	logs.Errorf("unmarshal bk result failed, err: %v", err)
	// 	return err
	// }
	if err := json.Unmarshal(resp.Body(), result); err != nil {
		logs.Errorf("unmarshal cmdb response failed, err: %v", err)
		return err
	}
	return nil
}

// SearchBusiness 组件化的函数
func (bkcmdb *CMDBService) SearchBusiness(ctx context.Context, params *cmdb.SearchBizParams) (
	*cmdb.SearchBizResult, error) {
	// bk_supplier_account 是无效参数, 占位用
	url := fmt.Sprintf(searchBusiness, bkcmdb.Host)

	type esbSearchBizParams struct {
		*types.CommParams
		*cmdb.SearchBizParams
	}

	req := &esbSearchBizParams{SearchBizParams: params}
	result := new(cmdb.SearchBizResult)

	if err := bkcmdb.doRequest(ctx, POST, url, req, result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListAllBusiness 获取所有业务列表
func (bkcmdb *CMDBService) ListAllBusiness(ctx context.Context) (*cmdb.SearchBizResult, error) {
	params := &cmdb.SearchBizParams{}
	bizRes, err := bkcmdb.SearchBusiness(ctx, params)
	if err != nil {
		logs.Errorf("search business failed, err: %v", err)
		return nil, err
	}

	return bizRes, nil
}

// FindHostByTopo 查询拓扑节点下的主机
func (bkcmdb *CMDBService) FindHostByTopo(ctx context.Context, req FindHostByTopoReq) (
	*CMDBResponse[CMDBListData[FindHostByTopo]], error) {
	url := fmt.Sprintf(findHostByTopo, bkcmdb.Host, req.BkBizID)

	resp := new(CMDBResponse[CMDBListData[FindHostByTopo]])
	if err := bkcmdb.doRequest(ctx, GET, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// SearchBizInstTopo 查询业务实例拓扑
func (bkcmdb *CMDBService) SearchBizInstTopo(ctx context.Context, req SearchBizInstTopoReq) (
	*CMDBResponse[SearchBizInstTopo], error) {
	url := fmt.Sprintf(searchBizInstTopo, bkcmdb.Host, req.BkBizID)

	resp := new(CMDBResponse[SearchBizInstTopo])
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// GetServiceTemplate 获取服务模板
func (bkcmdb *CMDBService) GetServiceTemplate(ctx context.Context, req GetServiceTemplateReq) (
	*CMDBResponse[GetServiceTemplate], error) {
	url := fmt.Sprintf(getServiceTemplate, bkcmdb.Host, req.ServiceTemplateId)
	resp := new(CMDBResponse[GetServiceTemplate])
	if err := bkcmdb.doRequest(ctx, GET, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// ListServiceTemplate 服务模板列表查询
func (bkcmdb *CMDBService) ListServiceTemplate(ctx context.Context, req ListServiceTemplateReq) (
	*CMDBResponse[CMDBListData[ListServiceTemplate]], error) {
	url := fmt.Sprintf(listServiceTemplate, bkcmdb.Host)
	resp := new(CMDBResponse[CMDBListData[ListServiceTemplate]])
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// GetProcTemplate 获取进程模板
func (bkcmdb *CMDBService) GetProcTemplate(ctx context.Context, req GetProcTemplateReq) (
	*CMDBResponse[ProcTemplate], error) {
	url := fmt.Sprintf(getProcTemplate, bkcmdb.Host, req.ProcessTemplateID)
	resp := new(CMDBResponse[ProcTemplate])
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// ListProcTemplate 查询进程模板列表
func (bkcmdb *CMDBService) ListProcTemplate(ctx context.Context, req ListProcTemplateReq) (
	*CMDBResponse[CMDBListData[ProcTemplate]], error) {
	url := fmt.Sprintf(listProcTemplate, bkcmdb.Host)

	resp := new(CMDBResponse[CMDBListData[ProcTemplate]])
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// ListProcessInstance 查询进程实例列表
func (bkcmdb *CMDBService) ListProcessInstance(ctx context.Context, req ListProcessInstanceReq) (
	*CMDBResponse[CMDBListData[ListProcessInstance]], error) {
	url := fmt.Sprintf(listProcessInstance, bkcmdb.Host)

	resp := new(CMDBResponse[CMDBListData[ListProcessInstance]])
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// FindHostBySetTemplate 查询集群模板下的主机
func (bkcmdb *CMDBService) FindHostBySetTemplate(ctx context.Context, req FindHostBySetTemplateReq) (
	*CMDBResponse[CMDBListData[HostInfo]], error) {
	url := fmt.Sprintf(findHostBySetTemplate, bkcmdb.Host)

	resp := new(CMDBResponse[CMDBListData[HostInfo]])
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// ListSetTemplate 查询集群模板
func (bkcmdb *CMDBService) ListSetTemplate(ctx context.Context, req ListSetTemplateReq) (
	*CMDBResponse[CMDBListData[ClusterTemplateInfo]], error) {
	url := fmt.Sprintf(listSetTemplate, bkcmdb.Host, req.BkBizID)

	resp := new(CMDBResponse[CMDBListData[ClusterTemplateInfo]])
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// ListProcessDetailByIds 查询某业务下进程ID对应的进程详情
func (bkcmdb *CMDBService) ListProcessDetailByIds(ctx context.Context, req ProcessRequest) (
	*CMDBResponse[CMDBListData[ProcessInfo]], error) {
	url := fmt.Sprintf(listProcessDetailByIds, bkcmdb.Host, req.BkBizID)

	resp := new(CMDBResponse[CMDBListData[ProcessInfo]])
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// ListServiceInstanceBySetTemplate 通过集群模版查询关联的服务实例列表
func (bkcmdb *CMDBService) ListServiceInstanceBySetTemplate(ctx context.Context, req ServiceInstanceRequest) (
	*CMDBResponse[CMDBListData[ServiceInstanceInfo]], error) {
	url := fmt.Sprintf(listServiceInstanceBySetTemplate, bkcmdb.Host, req.BkBizID)

	resp := new(CMDBResponse[CMDBListData[ServiceInstanceInfo]])
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// FindModuleBatch 批量查询某业务的模块详情
func (bkcmdb *CMDBService) FindModuleBatch(ctx context.Context, req ServiceInstanceRequest) (
	*CMDBResponse[[]ModuleInfo], error) {
	url := fmt.Sprintf(listServiceInstanceBySetTemplate, bkcmdb.Host, req.BkBizID)

	resp := new(CMDBResponse[[]ModuleInfo])
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// ListServiceInstance 查询服务实例列表
func (bkcmdb *CMDBService) ListServiceInstance(ctx context.Context) {

}

// FindSetBatch 批量查询某业务的集群详情
func (bkcmdb *CMDBService) FindSetBatch(ctx context.Context) {

}

// SearchSet 查询集群
func (bkcmdb *CMDBService) SearchSet(ctx context.Context) {

}

// SearchModule 查询模块
func (bkcmdb *CMDBService) SearchModule(ctx context.Context) {

}

// FindHostTopoRelation  获取主机与拓扑的关系
func (bkcmdb *CMDBService) FindHostTopoRelation(ctx context.Context) {

}

// ListBizHosts query hosts under biz
func (bkcmdb *CMDBService) ListBizHosts(ctx context.Context, req *ListBizHostsRequest) (
	*CMDBResponse[CMDBListData[HostInfo]], error) {
	url := fmt.Sprintf(listBizHosts, bkcmdb.Host, req.BkBizID)

	resp := new(CMDBResponse[CMDBListData[HostInfo]])
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// WatchHostResource watch host resource change
func (bkcmdb *CMDBService) WatchHostResource(ctx context.Context, req *WatchResourceRequest) (
	*HostWatchResponse, error) {
	if req.BkResource == "" {
		return nil, fmt.Errorf("resource type is required")
	}

	url := fmt.Sprintf(watchResource, bkcmdb.Host, req.BkResource)

	resp := new(HostWatchResponse)
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// WatchHostRelationResource watch host relation resource change
func (bkcmdb *CMDBService) WatchHostRelationResource(ctx context.Context, req *WatchResourceRequest) (
	*HostRelationWatchResponse, error) {
	if req.BkResource == "" {
		return nil, fmt.Errorf("resource type is required")
	}

	url := fmt.Sprintf(watchResource, bkcmdb.Host, req.BkResource)

	resp := new(HostRelationWatchResponse)
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// FindHostBizRelations query host biz relation information
func (bkcmdb *CMDBService) FindHostBizRelations(ctx context.Context, req *FindHostBizRelationsRequest) (
	*FindHostBizRelationsResponse, error) {
	if req.BkBizID == 0 {
		return nil, fmt.Errorf("bk_biz_id is required")
	}
	if len(req.BkHostID) == 0 {
		return nil, fmt.Errorf("bk_host_id list is required")
	}

	url := fmt.Sprintf(findHostBizRelations, bkcmdb.Host)

	resp := new(FindHostBizRelationsResponse)
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}
