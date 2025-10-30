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

	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/client"
	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/cmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

// Service xxx
type Service interface {
	// SearchBusiness 通用查询
	SearchBusiness(ctx context.Context, params *cmdb.SearchBizParams) (*cmdb.SearchBizResult, error)
	// ListAllBusiness 读取全部业务列表
	ListAllBusiness(ctx context.Context) (*cmdb.SearchBizResult, error)
	// 查询拓扑节点下的主机
	FindHostByTopo(ctx context.Context, req HostListReq) (
		*CMDBResponse, error)
	// SearchBizInstTopo 查询业务实例拓扑
	SearchBizInstTopo(ctx context.Context, req BizTopoReq) (
		*CMDBResponse, error)
	// GetServiceTemplate 获取服务模板
	GetServiceTemplate(ctx context.Context, req ServiceTemplateReq) (
		*CMDBResponse, error)
	// ListServiceTemplate 服务模板列表查询
	ListServiceTemplate(ctx context.Context, req ListServiceTemplateReq) (
		*CMDBResponse, error)
	// GetProcTemplate 获取进程模板
	GetProcTemplate(ctx context.Context, req GetProcTemplateReq) (
		*CMDBResponse, error)
	// ListProcTemplate 查询进程模板列表
	ListProcTemplate(ctx context.Context, req ListProcTemplateReq) (
		*CMDBResponse, error)
	// ListProcessInstance 查询进程实例列表
	ListProcessInstance(ctx context.Context, req ListProcessInstanceReq) (
		[]*ListProcessInstance, error)
	// FindHostBySetTemplate 查询集群模板下的主机
	FindHostBySetTemplate(ctx context.Context, req FindHostBySetTemplateReq) (
		*FindHostBySetTemplateResp, error)
	// ListSetTemplate 查询集群模板
	ListSetTemplate(ctx context.Context, req ListSetTemplateReq) (
		*CMDBResponse, error)
	// ListProcessDetailByIds 查询某业务下进程ID对应的进程详情
	ListProcessDetailByIds(ctx context.Context, req ProcessReq) (
		*CMDBResponse, error)
	// ListServiceInstanceBySetTemplate 通过集群模版查询关联的服务实例列表
	ListServiceInstanceBySetTemplate(ctx context.Context, req ServiceInstanceReq) (
		*CMDBResponse, error)
	// FindModuleBatch 批量查询某业务的模块详情
	FindModuleBatch(ctx context.Context, req ModuleReq) (
		*CMDBResponse, error)
	// ListServiceInstance 查询服务实例列表
	ListServiceInstance(ctx context.Context, req ServiceInstanceListReq) (
		*ServiceInstanceResp, error)
	// FindSetBatch 批量查询某业务的集群详情
	FindSetBatch(ctx context.Context, req SetListReq) (*CMDBResponse, error)
	// FindHostTopoRelation 获取主机与拓扑的关系
	FindHostTopoRelation(ctx context.Context, req HostTopoReq) (
		*CMDBResponse, error)
	// FindModuleWithRelation 根据条件查询业务下的模块
	FindModuleWithRelation(ctx context.Context, req ModuleListReq) (
		*CMDBResponse, error)
	// SearchSet 查询集群
	SearchSet(ctx context.Context, req SearchSetReq) (*Sets, error)
	// SearchBusinessByAccount 查询业务
	SearchBusinessByAccount(ctx context.Context, req SearchSetReq) (*Business, error)
	// SearchModule 查询模块
	SearchModule(ctx context.Context, req SearchModuleReq) (*ModuleListResp, error)
	// ResourceWatch 监听资源变化事件
	ResourceWatch(ctx context.Context, req *WatchResourceRequest) (*WatchData, error)
	ListBizHosts(ctx context.Context, req *ListBizHostsRequest) (*CMDBListData[HostInfo], error)
	// WatchHostResource 监听主机资源变化
	WatchHostResource(ctx context.Context, req *WatchResourceRequest) (*WatchResourceData[HostDetail], error)
	// WatchHostRelationResource 监听主机关系资源变化
	WatchHostRelationResource(ctx context.Context, req *WatchResourceRequest) (*WatchResourceData[HostRelationDetail], error)
	// FindHostBizRelations 查询主机业务关系信息
	FindHostBizRelations(ctx context.Context, req *FindHostBizRelationsRequest) ([]HostBizRelation, error)
}

// New cmdb service
func New(cfg *cc.CMDBConfig, esbClient client.Client) (Service, error) {
	return &CMDBService{cfg}, nil
}
