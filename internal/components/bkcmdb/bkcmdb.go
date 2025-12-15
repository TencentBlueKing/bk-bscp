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
	"fmt"

	"github.com/go-resty/resty/v2"

	"github.com/TencentBlueKing/bk-bscp/internal/components"
	"github.com/TencentBlueKing/bk-bscp/internal/components/bkuser"
	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/cmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/types"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

var (
	searchBusiness                   = "%s/api/v3/biz/search/bk_supplier_account"
	findHostByTopo                   = "%s/api/v3/findmany/hosts/by_topo/biz/%d"
	searchBizInstTopo                = "%s/api/v3/find/topoinst/biz/%d"
	getServiceTemplate               = "%s/api/v3/find/proc/service_template/%d"
	listServiceTemplate              = "%s/api/v3/findmany/proc/service_template"
	getProcTemplate                  = "%s/api/v3/find/proc/proc_template/id/%d"
	listProcTemplate                 = "%s/api/v3/findmany/proc/proc_template"
	listProcessInstance              = "%s/api/v3/findmany/proc/process_instance"
	findHostBySetTemplate            = "%s/api/v3/findmany/hosts/by_set_templates/biz/%d"
	listSetTemplate                  = "%s/api/v3/findmany/topo/set_template/bk_biz_id/%d"
	listProcessDetailByIds           = "%s/api/v3/findmany/proc/process_instance/detail/biz/%d"
	listServiceInstanceBySetTemplate = "%s/api/v3/findmany/proc/service/" +
		"set_template/list_service_instance/biz/%d"
	findModuleWithRelation  = "%s/api/v3/findmany/module/with_relation/biz/%d"
	searchBusinessByAccount = "%s/api/v3/biz/search/%s"

	findModuleBatch     = "%s/api/v3/findmany/module/bk_biz_id/%d"
	listServiceInstance = "%s/api/v3/findmany/proc/service_instance"

	findSetBatch         = "%s/api/v3/findmany/set/bk_biz_id/%d"
	searchSet            = "%s/api/v3/set/search/%s/%d"
	searchModule         = "%s/api/v3/module/search/%s/%d/%d"
	findHostTopoRelation = "%s/api/v3/host/topo/relation/read"
	listBizHosts         = "%s/api/v3/hosts/app/%d/list_hosts"
	watchResource        = "%s/api/v3/event/watch/resource/%s"
	findHostBizRelations = "%s/api/v3/hosts/modules/read"
	findTopoBrief        = "%s/api/v3/cache/find/cache/topo/brief/biz/%d"
	searchObjectAttr     = "%s/api/v3/find/objectattr"
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
	// 组装网关认证信息
	gwAuthOptions := []components.GWAuthOption{}
	withBkUsername := components.WithBkUsername(bkcmdb.BkUserName)

	// 多租户模式，带上租户ID
	if cc.G().FeatureFlags.EnableMultiTenantMode {
		admin, err := bkuser.GetTenantBKAdmin(ctx)
		if err != nil {
			return fmt.Errorf("get tenant admin failed: %w", err)
		}
		withBkUsername = components.WithBkUsername(admin.BkUsername)
	}
	gwAuthOptions = append(gwAuthOptions, withBkUsername)

	authHeader := components.MakeBKAPIGWAuthHeader(
		bkcmdb.AppCode,
		bkcmdb.AppSecret,
		gwAuthOptions...,
	)

	// 构造请求
	request := components.GetClient().SetDebug(false).R().
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

	// 统一反序列化结果，自动处理外层包装和错误码验证
	if err := components.UnmarshalBKResult(resp, result); err != nil {
		logs.Errorf("unmarshal bk result failed, err: %v, resp: %v", err, resp.Body())
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

	// UnmarshalBKResult 会自动处理外层的 data 包装
	if err := bkcmdb.doRequest(ctx, POST, url, req, result); err != nil {
		return nil, err
	}

	logs.Infof("search business result: count=%d, info_len=%d", result.Count, len(result.Info))
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
func (bkcmdb *CMDBService) FindHostByTopo(ctx context.Context, req HostListReq) (
	*HostListResp, error) {
	url := fmt.Sprintf(findHostByTopo, bkcmdb.Host, req.BkBizID)

	resp := new(HostListResp)
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// SearchBizInstTopo 查询业务实例拓扑
func (bkcmdb *CMDBService) SearchBizInstTopo(ctx context.Context, req *BizTopoReq) (
	[]*BizTopoNode, error) {
	url := fmt.Sprintf(searchBizInstTopo, bkcmdb.Host, req.BkBizID)

	var nodes []*BizTopoNode
	if err := bkcmdb.doRequest(ctx, POST, url, req, &nodes); err != nil {
		return nil, err
	}

	return nodes, nil
}

// GetServiceTemplate 获取服务模板
func (bkcmdb *CMDBService) GetServiceTemplate(ctx context.Context, req ServiceTemplateReq) (
	*ServiceTemplate, error) {
	url := fmt.Sprintf(getServiceTemplate, bkcmdb.Host, req.ServiceTemplateID)
	resp := new(ServiceTemplate)
	if err := bkcmdb.doRequest(ctx, GET, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// ListServiceTemplate 服务模板列表查询
func (bkcmdb *CMDBService) ListServiceTemplate(ctx context.Context, req *ListServiceTemplateReq) (
	*ServiceTemplateListResp, error) {
	url := fmt.Sprintf(listServiceTemplate, bkcmdb.Host)
	resp := new(ServiceTemplateListResp)
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// GetProcTemplate 获取进程模板
func (bkcmdb *CMDBService) GetProcTemplate(ctx context.Context, req GetProcTemplateReq) (
	*ProcTemplate, error) {
	url := fmt.Sprintf(getProcTemplate, bkcmdb.Host, req.ProcessTemplateID)
	resp := new(ProcTemplate)
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// ListProcTemplate 查询进程模板列表
func (bkcmdb *CMDBService) ListProcTemplate(ctx context.Context, req *ListProcTemplateReq) (
	*ListProcTemplateResp, error) {
	url := fmt.Sprintf(listProcTemplate, bkcmdb.Host)

	resp := new(ListProcTemplateResp)
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// ListProcessInstance 查询进程实例列表
func (bkcmdb *CMDBService) ListProcessInstance(ctx context.Context, req *ListProcessInstanceReq) (
	[]*ListProcessInstance, error) {
	url := fmt.Sprintf(listProcessInstance, bkcmdb.Host)

	var resp []*ListProcessInstance
	if err := bkcmdb.doRequest(ctx, POST, url, req, &resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// FindHostBySetTemplate 查询集群模板下的主机
func (bkcmdb *CMDBService) FindHostBySetTemplate(ctx context.Context, req FindHostBySetTemplateReq) (
	*FindHostBySetTemplateResp, error) {
	url := fmt.Sprintf(findHostBySetTemplate, bkcmdb.Host, req.BkBizID)

	result := new(FindHostBySetTemplateResp)
	if err := bkcmdb.doRequest(ctx, POST, url, req, result); err != nil {
		return nil, err
	}

	logs.Infof("search business result: count=%d, info_len=%d", result.Count, len(result.Info))
	return result, nil
}

// ListSetTemplate 查询集群模板
func (bkcmdb *CMDBService) ListSetTemplate(ctx context.Context, req ListSetTemplateReq) (
	*ListSetTemplateResp, error) {
	url := fmt.Sprintf(listSetTemplate, bkcmdb.Host, req.BkBizID)

	resp := new(ListSetTemplateResp)
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// ListProcessDetailByIds 查询某业务下进程ID对应的进程详情
func (bkcmdb *CMDBService) ListProcessDetailByIds(ctx context.Context, req ProcessReq) (
	[]*ProcessInfo, error) {
	url := fmt.Sprintf(listProcessDetailByIds, bkcmdb.Host, req.BkBizID)

	resp := new([]*ProcessInfo)
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}
	return *resp, nil
}

// ListServiceInstanceBySetTemplate 通过集群模版查询关联的服务实例列表
func (bkcmdb *CMDBService) ListServiceInstanceBySetTemplate(ctx context.Context, req ServiceInstanceReq) (
	[]ServiceInstanceInfo, error) {
	url := fmt.Sprintf(listServiceInstanceBySetTemplate, bkcmdb.Host, req.BkBizID)

	var resp []ServiceInstanceInfo
	if err := bkcmdb.doRequest(ctx, POST, url, req, &resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// FindModuleBatch 批量查询某业务的模块详情
func (bkcmdb *CMDBService) FindModuleBatch(ctx context.Context, req *ModuleReq) (
	[]*ModuleInfo, error) {
	url := fmt.Sprintf(findModuleBatch, bkcmdb.Host, req.BkBizID)

	var resp []*ModuleInfo
	if err := bkcmdb.doRequest(ctx, POST, url, req, &resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// ListServiceInstance 查询服务实例列表
func (bkcmdb *CMDBService) ListServiceInstance(ctx context.Context, req *ServiceInstanceListReq) (
	*ServiceInstanceResp, error) {
	url := fmt.Sprintf(listServiceInstance, bkcmdb.Host)

	resp := new(ServiceInstanceResp)
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	logs.Infof("list service instance result: count=%d, info_len=%d", resp.Count, len(resp.Info))
	return resp, nil
}

// FindSetBatch 批量查询某业务的集群详情
func (bkcmdb *CMDBService) FindSetBatch(ctx context.Context, req SetListReq) ([]SetInfo, error) {
	url := fmt.Sprintf(findSetBatch, bkcmdb.Host, req.BkBizID)

	var setInfos []SetInfo
	if err := bkcmdb.doRequest(ctx, POST, url, req, &setInfos); err != nil {
		return nil, err
	}

	return setInfos, nil
}

// FindHostTopoRelation 获取主机与拓扑的关系
func (bkcmdb *CMDBService) FindHostTopoRelation(ctx context.Context, req *HostTopoReq) (
	*HostTopoInfoResp, error) {
	url := fmt.Sprintf(findHostTopoRelation, bkcmdb.Host)

	resp := new(HostTopoInfoResp)
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// FindModuleWithRelation 根据条件查询业务下的模块
func (bkcmdb *CMDBService) FindModuleWithRelation(ctx context.Context, req ModuleListReq) (
	*ModuleListResp, error) {
	url := fmt.Sprintf(findModuleWithRelation, bkcmdb.Host, req.BkBizID)

	resp := new(ModuleListResp)
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// SearchSet 查询集群
func (bkcmdb *CMDBService) SearchSet(ctx context.Context, req SearchSetReq) (*Sets, error) {
	url := fmt.Sprintf(searchSet, bkcmdb.Host, req.BkSupplierAccount, req.BkBizID)

	resp := new(Sets)
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// SearchBusinessByAccount 查询业务
func (bkcmdb *CMDBService) SearchBusinessByAccount(ctx context.Context, req SearchSetReq) (*Business, error) {
	url := fmt.Sprintf(searchBusinessByAccount, bkcmdb.Host, req.BkSupplierAccount)

	resp := new(Business)
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// SearchModule 查询模块
func (bkcmdb *CMDBService) SearchModule(ctx context.Context, req SearchModuleReq) (*ModuleListResp, error) {
	url := fmt.Sprintf(searchModule, bkcmdb.Host, req.BkSupplierAccount, req.BkBizID, req.BkSetID)

	resp := new(ModuleListResp)
	if err := bkcmdb.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// ListBizHosts query hosts under biz
func (bkcmdb *CMDBService) ListBizHosts(ctx context.Context, req *ListBizHostsRequest) (
	*CMDBListData[HostInfo], error) {
	url := fmt.Sprintf(listBizHosts, bkcmdb.Host, req.BkBizID)

	result := new(CMDBListData[HostInfo])
	// UnmarshalBKResult 会自动处理外层包装和错误验证
	if err := bkcmdb.doRequest(ctx, POST, url, req, result); err != nil {
		return nil, err
	}

	return result, nil
}

// WatchHostResource watch host resource change
func (bkcmdb *CMDBService) WatchHostResource(ctx context.Context, req *WatchResourceRequest) (
	*WatchResourceData[HostDetail], error) {
	if req.BkResource == "" {
		return nil, fmt.Errorf("resource type is required")
	}

	url := fmt.Sprintf(watchResource, bkcmdb.Host, req.BkResource)

	result := new(WatchResourceData[HostDetail])
	// UnmarshalBKResult 会自动处理外层包装和错误验证
	if err := bkcmdb.doRequest(ctx, POST, url, req, result); err != nil {
		return nil, err
	}

	return result, nil
}

// WatchHostRelationResource watch host relation resource change
func (bkcmdb *CMDBService) WatchHostRelationResource(ctx context.Context, req *WatchResourceRequest) (
	*WatchResourceData[HostRelationDetail], error) {
	if req.BkResource == "" {
		return nil, fmt.Errorf("resource type is required")
	}

	url := fmt.Sprintf(watchResource, bkcmdb.Host, req.BkResource)

	result := new(WatchResourceData[HostRelationDetail])
	// UnmarshalBKResult 会自动处理外层包装和错误验证
	if err := bkcmdb.doRequest(ctx, POST, url, req, result); err != nil {
		return nil, err
	}

	return result, nil
}

// FindHostBizRelations query host biz relation information
func (bkcmdb *CMDBService) FindHostBizRelations(ctx context.Context, req *FindHostBizRelationsRequest) (
	[]HostBizRelation, error) {
	if req.BkBizID == 0 {
		return nil, fmt.Errorf("bk_biz_id is required")
	}
	if len(req.BkHostID) == 0 {
		return nil, fmt.Errorf("bk_host_id list is required")
	}

	url := fmt.Sprintf(findHostBizRelations, bkcmdb.Host)

	var result []HostBizRelation
	// UnmarshalBKResult 会自动处理外层包装和错误验证
	if err := bkcmdb.doRequest(ctx, POST, url, req, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// ResourceWatch 监听资源变化事件
func (bkcmdb *CMDBService) ResourceWatch(ctx context.Context, req *WatchResourceRequest) (*WatchData, error) {
	url := fmt.Sprintf(watchResource, bkcmdb.Host, req.BkResource)
	result := new(WatchData)
	if err := bkcmdb.doRequest(ctx, POST, url, req, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// FindTopoBrief 查询业务拓扑简要信息（缓存接口）
func (bkcmdb *CMDBService) FindTopoBrief(ctx context.Context, bizID int) (*TopoBriefResp, error) {
	url := fmt.Sprintf(findTopoBrief, bkcmdb.Host, bizID)

	// UnmarshalBKResult 会自动处理外层的 data 包装
	resp := new(TopoBriefResp)
	if err := bkcmdb.doRequest(ctx, GET, url, nil, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// SearchObjectAttr 查询对象属性
func (bkcmdb *CMDBService) SearchObjectAttr(ctx context.Context, req SearchObjectAttrReq) ([]ObjectAttrInfo, error) {
	url := fmt.Sprintf(searchObjectAttr, bkcmdb.Host)

	var resp []ObjectAttrInfo
	if err := bkcmdb.doRequest(ctx, POST, url, req, &resp); err != nil {
		return nil, err
	}

	return resp, nil
}
