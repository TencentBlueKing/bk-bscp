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
	"encoding/json"
	"errors"
)

// Biz is cmdb biz info.
type Biz struct {
	BizID         int64  `json:"bk_biz_id"`
	BizName       string `json:"bk_biz_name"`
	BizMaintainer string `json:"bk_biz_maintainer"`
}

// HostListReq 查询主机列表请求参数
type HostListReq struct {
	BkBizID  int        `json:"bk_biz_id"`  // 业务ID，必填
	BkObjID  string     `json:"bk_obj_id"`  // 拓扑节点模型ID，如 set/module，不可为 biz/host，必填
	BkInstID int        `json:"bk_inst_id"` // 拓扑节点实例ID，必填
	Fields   []string   `json:"fields"`     // 主机属性列表，必填，用于加速接口请求和减少网络流量
	Page     *PageParam `json:"page"`       // 分页信息，必填
}

type HostListResp struct {
	Count int        `json:"count"` // 总数
	Info  []HostInfo `json:"info"`  // 返回结果
}

// FindHostByTopo xxx
type FindHostByTopo struct {
	BkHostName        string `json:"bk_host_name"`         // 主机名
	BkHostInnerip     string `json:"bk_host_innerip"`      // 内网IP
	BkHostID          int    `json:"bk_host_id"`           // 主机ID
	BkCloudID         int    `json:"bk_cloud_id"`          // 管控区域
	ImportFrom        string `json:"import_from"`          // 主机导入来源,以api方式导入为3
	BkAssetID         string `json:"bk_asset_id"`          // 固资编号
	BkCloudInstID     string `json:"bk_cloud_inst_id"`     // 云主机实例ID
	BkCloudVendor     string `json:"bk_cloud_vendor"`      // 云厂商
	BkCloudHostStatus string `json:"bk_cloud_host_status"` // 云主机状态
	BkComment         string `json:"bk_comment"`           // 备注
	BkCPU             int    `json:"bk_cpu"`               // CPU逻辑核心数
	BkCPUArchitecture string `json:"bk_cpu_architecture"`  // CPU架构
	BkCPUModule       string `json:"bk_cpu_module"`        // CPU型号
	BkDisk            int    `json:"bk_disk"`              // 磁盘容量（GB）
	BkHostOuterip     string `json:"bk_host_outerip"`      // 主机外网IP
	BkHostInneripV6   string `json:"bk_host_innerip_v6"`   // 主机内网IPv6
	BkHostOuteripV6   string `json:"bk_host_outerip_v6"`   // 主机外网IPv6
	BkIspName         string `json:"bk_isp_name"`          // 所属运营商
	BkMac             string `json:"bk_mac"`               // 主机内网MAC地址
	BkMem             int    `json:"bk_mem"`               // 主机内存容量（MB）
	BkOSBit           string `json:"bk_os_bit"`            // 操作系统位数
	BkOSName          string `json:"bk_os_name"`           // 操作系统名称
	BkOSType          string `json:"bk_os_type"`           // 操作系统类型
	BkOSVersion       string `json:"bk_os_version"`        // 操作系统版本
	BkOuterMac        string `json:"bk_outer_mac"`         // 主机外网MAC地址
	BkProvinceName    string `json:"bk_province_name"`     // 所在省份
	BkServiceTerm     int    `json:"bk_service_term"`      // 质保年限
	BkSla             string `json:"bk_sla"`               // SLA级别
	BkSn              string `json:"bk_sn"`                // 设备SN
	BkState           string `json:"bk_state"`             // 当前状态
	BkStateName       string `json:"bk_state_name"`        // 所在国家
	Operator          string `json:"operator"`             // 主要维护人
	BkBakOperator     string `json:"bk_bak_operator"`      // 备份维护人
}

// Data xxx
type Data struct {
	Count int              `json:"count"` // 记录条数
	Info  []FindHostByTopo `json:"info"`  // 主机实际数据
}

// CMDBResponse 通用响应结构
type CMDBResponseData[T any] struct {
	Result     bool   `json:"result"`     // 请求成功与否
	Code       int    `json:"code"`       // 错误编码
	Message    string `json:"message"`    // 错误信息
	Data       T      `json:"data"`       // 泛型，具体数据结构
	Permission any    `json:"permission"` // 权限信息（可以根据需要定义 struct）
}

// HostRelationWatchResponse 主机关系监听响应
type HostRelationWatchResponse = CMDBResponseData[HostRelationWatchData]

// HostWatchData 主机监听数据
type HostWatchData = WatchResourceData[HostDetail]

// HostDetail 主机事件详情
type HostDetail struct {
	BkHostID  *int    `json:"bk_host_id"`  // 主机ID
	BkAgentID *string `json:"bk_agent_id"` // Agent ID
}

// HostWatchResponse 主机监听响应
type HostWatchResponse = CMDBResponseData[HostWatchData]

// HostEvent 主机事件
type HostEvent = Event[HostDetail]

// CMDBListData 表示带数量和列表的 Data
type CMDBListData[T any] struct {
	Count int `json:"count"` // 记录条数
	Info  []T `json:"info"`  // 具体数据列表
}

// SearchBizInstTopoReq xxx
type SearchBizInstTopoReq struct {
	BkBizID int `json:"bk_biz_id"` // 业务ID
}

// SearchBizInstTopo xxx
type SearchBizInstTopo struct {
	BkInstID   int                 `json:"bk_inst_id"`   // 实例ID
	BkInstName string              `json:"bk_inst_name"` // 实例展示名
	BkObjIcon  string              `json:"bk_obj_icon"`  // 模型图标
	BkObjID    string              `json:"bk_obj_id"`    // 模型ID
	BkObjName  string              `json:"bk_obj_name"`  // 模型展示名
	Child      []SearchBizInstTopo `json:"child"`        // 子节点（递归）
	Default    int                 `json:"default"`      // 业务类型 / 集群类型
}

// GetServiceTemplateReq xxx
type GetServiceTemplateReq struct {
	ServiceTemplateId int `json:"service_template_id"` // 服务模板ID
}

// ServiceTemplate 服务模板信息
type GetServiceTemplate struct {
	BkBizID           int      `json:"bk_biz_id"`           // 业务ID
	ID                int      `json:"id"`                  // 服务模板ID
	Name              []string `json:"name"`                // 服务模板名称（数组）
	ServiceCategoryID int      `json:"service_category_id"` // 服务分类ID
	Creator           string   `json:"creator"`             // 创建者
	Modifier          string   `json:"modifier"`            // 最后修改人员
	CreateTime        string   `json:"create_time"`         // 创建时间
	LastTime          string   `json:"last_time"`           // 更新时间
	BkSupplierAccount string   `json:"bk_supplier_account"` // 开发商账号
	HostApplyEnabled  bool     `json:"host_apply_enabled"`  // 是否启用主机属性自动应用
}

// ListServiceTemplateReq xxx
type ListServiceTemplateReq struct {
	BkBizID            int        `json:"bk_biz_id"`                      // 业务ID（必选）
	ServiceCategoryID  int        `json:"service_category_id,omitempty"`  // 服务分类ID（可选）
	Search             string     `json:"search,omitempty"`               // 按服务模板名查询，默认为空
	IsExact            bool       `json:"is_exact,omitempty"`             // 是否精确匹配（搭配 search 使用）
	ServiceTemplateIDs []int      `json:"service_template_ids,omitempty"` // 服务模板ID列表
	Page               *PageParam `json:"page"`                           // 分页参数（必选）
}

// ListServiceTemplate xxx
type ListServiceTemplate struct {
	BkBizID           int      `json:"bk_biz_id"`
	ID                int      `json:"id"`
	Name              []string `json:"name"` // name 是 array
	ServiceCategoryID int      `json:"service_category_id"`
	Creator           string   `json:"creator"`
	Modifier          string   `json:"modifier"`
	CreateTime        string   `json:"create_time"`
	LastTime          string   `json:"last_time"`
	BkSupplierAccount string   `json:"bk_supplier_account"`
	HostApplyEnabled  bool     `json:"host_apply_enabled"`
}

// GetProcTemplateReq xxx
type GetProcTemplateReq struct {
	BkBizID           int `json:"bk_biz_id"`
	ProcessTemplateID int `json:"process_template_id"`
}

// ProcTemplate 进程模板数据
type ProcTemplate struct {
	ID                int                      `json:"id"`
	BkProcessName     string                   `json:"bk_process_name"`
	BkBizID           int                      `json:"bk_biz_id"`
	ServiceTemplateID int                      `json:"service_template_id"`
	Property          map[string]PropertyField `json:"property"`
	Creator           string                   `json:"creator"`
	Modifier          string                   `json:"modifier"`
	CreateTime        string                   `json:"create_time"`
	LastTime          string                   `json:"last_time"`
	BkSupplierAccount string                   `json:"bk_supplier_account"`
}

// Property xxx
type Property struct {
	AutoStart         bool     `json:"auto_start"`
	BkBizID           int      `json:"bk_biz_id"`
	BkFuncID          string   `json:"bk_func_id"`
	BkFuncName        string   `json:"bk_func_name"`
	BkProcessID       int      `json:"bk_process_id"`
	BkProcessName     string   `json:"bk_process_name"`
	BkStartParamRegex string   `json:"bk_start_param_regex"`
	BkSupplierAccount string   `json:"bk_supplier_account"`
	CreateTime        string   `json:"create_time"`
	Description       string   `json:"description"`
	FaceStopCmd       string   `json:"face_stop_cmd"`
	LastTime          string   `json:"last_time"`
	PidFile           string   `json:"pid_file"`
	Priority          int      `json:"priority"`
	ProcNum           int      `json:"proc_num"`
	ReloadCmd         string   `json:"reload_cmd"`
	RestartCmd        string   `json:"restart_cmd"`
	StartCmd          string   `json:"start_cmd"`
	StopCmd           string   `json:"stop_cmd"`
	Timeout           int      `json:"timeout"`
	User              string   `json:"user"`
	WorkPath          string   `json:"work_path"`
	BindInfo          BindInfo `json:"bind_info"`
}

// HostInfo 主机信息
type HostInfo struct {
	BkHostName        string `json:"bk_host_name"`         // 主机名
	BkHostInnerIP     string `json:"bk_host_innerip"`      // 内网IP
	BkHostID          int    `json:"bk_host_id"`           // 主机ID
	BkAgentID         string `json:"bk_agent_id"`          // Agent ID
	BkCloudID         int    `json:"bk_cloud_id"`          // 管控区域
	ImportFrom        string `json:"import_from"`          // 主机导入来源, API方式导入为3
	BkAssetID         string `json:"bk_asset_id"`          // 固资编号
	BkCloudInstID     string `json:"bk_cloud_inst_id"`     // 云主机实例ID
	BkCloudVendor     string `json:"bk_cloud_vendor"`      // 云厂商
	BkCloudHostStatus string `json:"bk_cloud_host_status"` // 云主机状态
	BkComment         string `json:"bk_comment"`           // 备注
	BkCPU             int    `json:"bk_cpu"`               // CPU逻辑核心数
	BkCPUArchitecture string `json:"bk_cpu_architecture"`  // CPU架构
	BkCPUModule       string `json:"bk_cpu_module"`        // CPU型号
	BkDisk            int    `json:"bk_disk"`              // 磁盘容量（GB）
	BkHostOuterIP     string `json:"bk_host_outerip"`      // 主机外网IP
	BkHostInnerIPV6   string `json:"bk_host_innerip_v6"`   // 主机内网IPv6
	BkHostOuterIPV6   string `json:"bk_host_outerip_v6"`   // 主机外网IPv6
	BkISPName         string `json:"bk_isp_name"`          // 所属运营商
	BkMAC             string `json:"bk_mac"`               // 主机内网MAC地址
	BkMem             int    `json:"bk_mem"`               // 主机内存容量（MB）
	BkOSBit           string `json:"bk_os_bit"`            // 操作系统位数
	BkOSName          string `json:"bk_os_name"`           // 操作系统名称
	BkOSType          string `json:"bk_os_type"`           // 操作系统类型
	BkOSVersion       string `json:"bk_os_version"`        // 操作系统版本
	BkOuterMAC        string `json:"bk_outer_mac"`         // 主机外网MAC地址
	BkProvinceName    string `json:"bk_province_name"`     // 所在省份
	BkServiceTerm     int    `json:"bk_service_term"`      // 质保年限
	BkSLA             string `json:"bk_sla"`               // SLA级别
	BkSN              string `json:"bk_sn"`                // 设备SN
	BkState           string `json:"bk_state"`             // 当前状态
	BkStateName       string `json:"bk_state_name"`        // 所在国家
	Operator          string `json:"operator"`             // 主要维护人
	BkBakOperator     string `json:"bk_bak_operator"`      // 备份维护人
}

// CMDBResponse 通用响应结构
type CMDBResponse struct {
	Result     bool            `json:"result"`  // 请求成功与否
	Code       int             `json:"code"`    // 错误编码
	Message    string          `json:"message"` // 错误信息
	Data       json.RawMessage `json:"data"`
	Permission any             `json:"permission"` // 权限信息（可以根据需要定义 struct）
}

// Decode 把 Data 部分解码到目标结构里
func (r *CMDBResponse) Decode(v any) error {
	if len(r.Data) == 0 {
		return nil
	}
	return json.Unmarshal(r.Data, v)
}

// Encode 把目标结构编码回 Data 部分
func (r *CMDBResponse) Encode(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	r.Data = b
	return nil
}

// BizTopoReq 查询业务实例拓扑请求参数
type BizTopoReq struct {
	BkBizID int `json:"bk_biz_id"` // 业务ID，必填
}

// BizTopoNode 业务拓扑节点信息
type BizTopoNode struct {
	BkInstID   int            `json:"bk_inst_id"`   // 实例ID
	BkInstName string         `json:"bk_inst_name"` // 实例展示名
	BkObjIcon  string         `json:"bk_obj_icon"`  // 模型图标
	BkObjID    string         `json:"bk_obj_id"`    // 模型ID
	BkObjName  string         `json:"bk_obj_name"`  // 模型展示名
	Child      []*BizTopoNode `json:"child"`        // 子节点（递归）
	Default    int            `json:"default"`      // 业务类型 / 集群类型
}

// ServiceTemplateReq 请求参数
type ServiceTemplateReq struct {
	ServiceTemplateID int `json:"service_template_id"` // 服务模板ID
}

// ServiceTemplate 服务模板信息
type ServiceTemplate struct {
	BkBizID           int    `json:"bk_biz_id"`           // 业务ID
	ID                int    `json:"id"`                  // 服务模板ID
	Name              string `json:"name"`                // 服务模板名称
	ServiceCategoryID int    `json:"service_category_id"` // 服务分类ID
	Creator           string `json:"creator"`             // 创建者
	Modifier          string `json:"modifier"`            // 最后修改人员
	CreateTime        string `json:"create_time"`         // 创建时间
	LastTime          string `json:"last_time"`           // 更新时间
	BkSupplierAccount string `json:"bk_supplier_account"` // 开发商账号
	HostApplyEnabled  bool   `json:"host_apply_enabled"`  // 是否启用主机属性自动应用
}

// PageParam 公共分页参数
type PageParam struct {
	Start int    `json:"start"`          // 记录开始位置，默认0
	Limit int    `json:"limit"`          // 每页限制条数，最大500
	Sort  string `json:"sort,omitempty"` // 排序字段
}

// ServiceTemplateListResp 响应结果
type ServiceTemplateListResp struct {
	Count int                `json:"count"` // 总数
	Info  []*ServiceTemplate `json:"info"`  // 返回结果
}

// PropertyField 通用属性字段
type PropertyField struct {
	Value          any  `json:"value"`
	AsDefaultValue bool `json:"as_default_value"`
}

// ListProcTemplateReq xxx
type ListProcTemplateReq struct {
	BkBizID           int `json:"bk_biz_id"`           // 业务ID
	ServiceTemplateID int `json:"service_template_id"` // 服务模板ID，service_template_id和process_template_ids至少传一个
	ProcessTemplateID int `json:"process_template_id"` // 进程模板ID数组，最多200个，service_template_id和process_template_ids至少传一个
}

type ListProcTemplateResp struct {
	Count int             `json:"count"` // 总数
	Info  []*ProcTemplate `json:"info"`  // 返回结果
}

// ListProcessInstanceReq xxx
type ListProcessInstanceReq struct {
	BkBizID           int `json:"bk_biz_id"`           // 业务ID
	ServiceInstanceID int `json:"service_instance_id"` // 服务实例ID
}

// ListProcessInstance 进程数据
type ListProcessInstance struct {
	Property *ProcessInfo `json:"property"` // 进程属性信息
	Relation *Relation    `json:"relation"` // 进程与服务实例的关联信息
}

// BindInfo 绑定信息
type BindInfo struct {
	Enable        bool   `json:"enable"`          // 端口是否启用
	IP            string `json:"ip"`              // 绑定的ip
	Port          string `json:"port"`            // 绑定的端口
	Protocol      string `json:"protocol"`        // 使用的协议
	TemplateRowID int    `json:"template_row_id"` // 实例化使用的模板行索引，进程内唯一
	RowID         int    `json:"row_id"`          // 模板行索引，进程内唯一
}

// Relation 进程与服务实例关联信息
type Relation struct {
	BkBizID           int    `json:"bk_biz_id"`           // 业务id
	BkProcessID       int    `json:"bk_process_id"`       // 进程id
	ServiceInstanceID int    `json:"service_instance_id"` // 服务实例id
	ProcessTemplateID int    `json:"process_template_id"` // 进程模版id
	BkHostID          int    `json:"bk_host_id"`          // 主机id
	BkSupplierAccount string `json:"bk_supplier_account"` // 开发商账号
}

// FindHostBySetTemplateReq xxx
type FindHostBySetTemplateReq struct {
	BkBizID          int        `json:"bk_biz_id"`            // 业务ID，必填
	BkSetTemplateIDs []int      `json:"bk_set_template_ids"`  // 集群模板ID列表，最多可填500个，必填
	BkSetIDs         []int      `json:"bk_set_ids,omitempty"` // 集群ID列表，最多可填500个，可选
	Fields           []string   `json:"fields"`               // 主机属性列表，控制返回结果的模块信息里有哪些字段，必填
	Page             *PageParam `json:"page"`                 // 分页信息，必填
}

type FindHostBySetTemplateResp struct {
	Count int        `json:"count"` // 总数
	Info  []HostInfo `json:"info"`  // 返回结果
}

// ListSetTemplateReq xxx
type ListSetTemplateReq struct {
	BkBizID        int        `json:"bk_biz_id"`                  // 业务ID，必填
	SetTemplateIDs []int      `json:"set_template_ids,omitempty"` // 集群模板ID数组，可选
	Page           *PageParam `json:"page,omitempty"`             // 分页信息，可选
}

type ListSetTemplateResp struct {
	Count int                   `json:"count"` // 总数
	Info  []ClusterTemplateInfo `json:"info"`  // 返回结果
}

// ClusterTemplateInfo 集群模板信息
type ClusterTemplateInfo struct {
	ID                int      `json:"id"`                  // 集群模板ID
	Name              []string `json:"name"`                // 集群模板名称
	BkBizID           int      `json:"bk_biz_id"`           // 业务ID
	Creator           string   `json:"creator"`             // 创建者
	Modifier          string   `json:"modifier"`            // 最后修改人员
	CreateTime        string   `json:"create_time"`         // 创建时间
	LastTime          string   `json:"last_time"`           // 更新时间
	BkSupplierAccount string   `json:"bk_supplier_account"` // 开发商账号
}

// ProcessRequest 查询进程请求参数
type ProcessReq struct {
	BkBizID      int      `json:"bk_biz_id"`        // 进程所在的业务ID，必填
	BkProcessIDs []int    `json:"bk_process_ids"`   // 进程ID列表，最多传500个，必填
	Fields       []string `json:"fields,omitempty"` // 进程属性列表，可选，控制返回结果字段，加速请求
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	AutoStart         bool        `json:"auto_start"`           // 是否自动拉起
	BkBizID           int         `json:"bk_biz_id"`            // 业务id
	BkFuncName        string      `json:"bk_func_name"`         // 进程名称
	BkProcessID       int         `json:"bk_process_id"`        // 进程id
	BkProcessName     string      `json:"bk_process_name"`      // 进程别名
	BkStartParamRegex string      `json:"bk_start_param_regex"` // 进程启动参数
	BkSupplierAccount string      `json:"bk_supplier_account"`  // 开发商账号
	CreateTime        string      `json:"create_time"`          // 创建时间
	Description       string      `json:"description"`          // 描述
	FaceStopCmd       string      `json:"face_stop_cmd"`        // 强制停止命令
	LastTime          string      `json:"last_time"`            // 更新时间
	PidFile           string      `json:"pid_file"`             // PID文件路径
	Priority          int         `json:"priority"`             // 启动优先级
	ProcNum           int         `json:"proc_num"`             // 启动数量
	ReloadCmd         string      `json:"reload_cmd"`           // 进程重载命令
	RestartCmd        string      `json:"restart_cmd"`          // 重启命令
	StartCmd          string      `json:"start_cmd"`            // 启动命令
	StopCmd           string      `json:"stop_cmd"`             // 停止命令
	Timeout           int         `json:"timeout"`              // 操作超时时长
	User              string      `json:"user"`                 // 启动用户
	WorkPath          string      `json:"work_path"`            // 工作路径
	BkCreatedAt       string      `json:"bk_created_at"`        // 创建时间
	BkCreatedBy       string      `json:"bk_created_by"`        // 创建人
	BkUpdatedAt       string      `json:"bk_updated_at"`        // 更新时间
	BkUpdatedBy       string      `json:"bk_updated_by"`        // 更新人
	BindInfo          []*BindInfo `json:"bind_info"`            // 绑定信息列表
	ServiceInstanceID int         `json:"service_instance_id"`  // 服务实例ID
	BkStartCheckSecs  int         `json:"bk_start_check_secs"`  // 启动等待时长
}

// ServiceInstanceRequest 查询服务实例请求参数
type ServiceInstanceReq struct {
	BkBizID       int        `json:"bk_biz_id"`       // 业务id，必填
	SetTemplateID int        `json:"set_template_id"` // 集群模版ID，必填
	Page          *PageParam `json:"page"`            // 分页参数，必填
}

type ServiceInstanceResp struct {
	Count int                    `json:"count"` // 总数
	Info  []*ServiceInstanceInfo `json:"info"`  // 返回结果
}

// ServiceInstanceInfo 服务实例信息
type ServiceInstanceInfo struct {
	ID                int                   `json:"id"`                  // 服务实例ID
	Name              string                `json:"name"`                // 服务实例名称
	BkBizID           int                   `json:"bk_biz_id"`           // 业务id
	BkModuleID        int                   `json:"bk_module_id"`        // 模型id
	ServiceTemplateID int                   `json:"service_template_id"` // 服务模版ID
	Labels            map[string]string     `json:"labels"`              // 标签信息
	BkHostID          int                   `json:"bk_host_id"`          // 主机id
	Creator           string                `json:"creator"`             // 本条数据创建者
	Modifier          string                `json:"modifier"`            // 本条数据的最后修改人员
	CreateTime        string                `json:"create_time"`         // 创建时间
	LastTime          string                `json:"last_time"`           // 更新时间
	BkSupplierAccount string                `json:"bk_supplier_account"` // 开发商账号
	ProcessInstances  []ListProcessInstance `json:"process_instances"`   // 进程实例列表
}

// ModuleRequest 查询模块信息请求
type ModuleReq struct {
	BkBizID int      `json:"bk_biz_id"` // 业务ID (必选)
	BkIDs   []int    `json:"bk_ids"`    // 模块实例ID列表 (必选, 最多500个)
	Fields  []string `json:"fields"`    // 模块属性列表 (必选)
}

// ModuleInfo 模块信息
type ModuleInfo struct {
	BkModuleID        int    `json:"bk_module_id"`        // 模块ID
	BkModuleName      string `json:"bk_module_name"`      // 模块名称
	Default           int    `json:"default"`             // 模块类型
	CreateTime        string `json:"create_time"`         // 创建时间
	BkSetID           int    `json:"bk_set_id"`           // 集群ID
	BkBakOperator     string `json:"bk_bak_operator"`     // 备份维护人
	BkBizID           int    `json:"bk_biz_id"`           // 业务ID
	BkModuleType      string `json:"bk_module_type"`      // 模块类型
	BkParentID        int    `json:"bk_parent_id"`        // 父节点ID
	BkSupplierAccount string `json:"bk_supplier_account"` // 开发商账号
	LastTime          string `json:"last_time"`           // 更新时间
	HostApplyEnabled  bool   `json:"host_apply_enabled"`  // 是否启用主机属性自动应用
	Operator          string `json:"operator"`            // 主要维护人
	ServiceCategoryID int    `json:"service_category_id"` // 服务分类ID
	ServiceTemplateID int    `json:"service_template_id"` // 服务模板ID
	SetTemplateID     int    `json:"set_template_id"`     // 集群模板ID
	BkCreatedAt       string `json:"bk_created_at"`       // 创建时间
	BkUpdatedAt       string `json:"bk_updated_at"`       // 更新时间
	BkCreatedBy       string `json:"bk_created_by"`       // 创建人
}

// TopoBriefBizInfo 拓扑简要信息中的业务信息
type TopoBriefBizInfo struct {
	ID                int    `json:"id"`                  // 业务ID
	Name              string `json:"nm"`                  // 业务名称
	Default           int    `json:"dft"`                 // 默认标识
	BkSupplierAccount string `json:"bk_supplier_account"` // 开发商账号
}

// TopoBriefNode 拓扑简要节点（Set 或 Module）
type TopoBriefNode struct {
	Obj     string           `json:"obj"` // 对象类型: "set" 或 "module"
	ID      int              `json:"id"`  // 实例ID
	Name    string           `json:"nm"`  // 名称
	Default int              `json:"dft"` // 默认标识
	Nodes   []*TopoBriefNode `json:"nds"` // 子节点（Module 的 nds 为 null）
}

// TopoBriefResp 拓扑简要信息响应
type TopoBriefResp struct {
	Biz   TopoBriefBizInfo `json:"biz"`  // 业务信息
	Idle  []*TopoBriefNode `json:"idle"` // 空闲机池节点列表
	Nodes []*TopoBriefNode `json:"nds"`  // 业务节点列表
}

// SearchObjectAttrReq 查询对象属性请求参数
type SearchObjectAttrReq struct {
	BkObjID string `json:"bk_obj_id"` // 对象模型ID，如 "set", "module", "host"
	BkBizID int    `json:"bk_biz_id"` // 业务ID
}

// ObjectAttrOption 对象属性选项（用于枚举类型）
type ObjectAttrOption struct {
	ID        string `json:"id"`         // 选项ID
	IsDefault bool   `json:"is_default"` // 是否默认值
	Name      string `json:"name"`       // 选项名称
	Type      string `json:"type"`       // 选项类型
}

// ObjectAttrInfo 对象属性信息
type ObjectAttrInfo struct {
	BkBizID             int         `json:"bk_biz_id"`              // 业务ID（0表示系统属性）
	ID                  int         `json:"id"`                     // 属性ID
	BkSupplierAccount   string      `json:"bk_supplier_account"`    // 开发商账号
	BkObjID             string      `json:"bk_obj_id"`              // 对象模型ID
	BkPropertyID        string      `json:"bk_property_id"`         // 属性ID
	BkPropertyName      string      `json:"bk_property_name"`       // 属性名称
	BkPropertyGroup     string      `json:"bk_property_group"`      // 属性分组
	BkPropertyIndex     int         `json:"bk_property_index"`      // 属性索引
	Unit                string      `json:"unit"`                   // 单位
	Placeholder         string      `json:"placeholder"`            // 占位符
	Editable            bool        `json:"editable"`               // 是否可编辑
	IsPre               bool        `json:"ispre"`                  // 是否预定义
	IsRequired          bool        `json:"isrequired"`             // 是否必填
	IsReadonly          bool        `json:"isreadonly"`             // 是否只读
	IsOnly              bool        `json:"isonly"`                 // 是否唯一
	BkIsSystem          bool        `json:"bk_issystem"`            // 是否系统属性
	BkIsAPI             bool        `json:"bk_isapi"`               // 是否API属性
	BkPropertyType      string      `json:"bk_property_type"`       // 属性类型
	Option              interface{} `json:"option"`                 // 选项（可能是字符串或数组）
	IsMultiple          *bool       `json:"ismultiple,omitempty"`   // 是否多选（可选字段）
	Description         string      `json:"description"`            // 描述
	BkTemplateID        int         `json:"bk_template_id"`         // 模板ID
	Creator             string      `json:"creator"`                // 创建者
	CreateTime          string      `json:"create_time"`            // 创建时间
	LastTime            string      `json:"last_time"`              // 更新时间
	BkPropertyGroupName string      `json:"bk_property_group_name"` // 属性分组名称
}

// ServiceInstanceListReq 查询服务实例请求参数
type ServiceInstanceListReq struct {
	BkBizID    int        `json:"bk_biz_id"`              // 业务id，必填
	BkModuleID int        `json:"bk_module_id,omitempty"` // 模块ID，可选
	BkHostIDs  []int      `json:"bk_host_ids,omitempty"`  // 主机id列表，最多支持1000个，可选
	Selectors  []Selector `json:"selectors,omitempty"`    // label过滤功能，可选
	Page       *PageParam `json:"page,omitempty"`         // 分页参数，可选
	SearchKey  string     `json:"search_key,omitempty"`   // 名字过滤参数（模糊搜索），可选
}

// Selector 标签过滤条件
type Selector struct {
	Key      string   `json:"key"`              // 标签键
	Operator string   `json:"operator"`         // 操作符 (=, !=, exists, !, in, notin)
	Values   []string `json:"values,omitempty"` // 值列表（适用于 in, notin）
}

// SetListReq 查询集群实例请求参数
type SetListReq struct {
	BkBizID int      `json:"bk_biz_id"` // 业务ID，必填
	BkIDs   []int    `json:"bk_ids"`    // 集群实例ID列表 (bk_set_id)，最多500个，必填
	Fields  []string `json:"fields"`    // 集群属性字段列表，必填
}

// Business 业务数据
type Sets struct {
	Count int       `json:"count"` // 记录条数
	Info  []SetInfo `json:"info"`  // 集群数据
}

// SetInfo 集群信息
type SetInfo struct {
	BkSetName          string   `json:"bk_set_name"`          // 集群名称
	Default            int      `json:"default"`              // 0-普通集群，1-内置模块集合
	BkBizID            int      `json:"bk_biz_id"`            // 业务ID
	BkCapacity         int      `json:"bk_capacity"`          // 设计容量
	BkParentID         int      `json:"bk_parent_id"`         // 父节点ID
	BkSetID            int      `json:"bk_set_id"`            // 集群ID
	BkServiceStatus    string   `json:"bk_service_status"`    // 服务状态: 1-开放, 2-关闭
	BkSetDesc          string   `json:"bk_set_desc"`          // 集群描述
	BkSetEnv           string   `json:"bk_set_env"`           // 环境类型: 1-测试, 2-体验, 3-正式
	CreateTime         string   `json:"create_time"`          // 创建时间
	LastTime           string   `json:"last_time"`            // 更新时间
	BkSupplierAccount  string   `json:"bk_supplier_account"`  // 开发商账号
	Description        string   `json:"description"`          // 数据描述信息
	SetTemplateVersion []string `json:"set_template_version"` // 集群模板的当前版本
	SetTemplateID      int      `json:"set_template_id"`      // 集群模板ID
	BkCreatedAt        string   `json:"bk_created_at"`        // 创建时间
	BkUpdatedAt        string   `json:"bk_updated_at"`        // 更新时间
}

// HostTopoReq 查询业务下主机与集群/模块绑定关系请求参数
type HostTopoReq struct {
	BkBizID     int        `json:"bk_biz_id"`               // 业务ID，必填
	BkSetIDs    []int      `json:"bk_set_ids,omitempty"`    // 集群ID列表，最多200条
	BkModuleIDs []int      `json:"bk_module_ids,omitempty"` // 模块ID列表，最多500条
	BkHostIDs   []int      `json:"bk_host_ids,omitempty"`   // 主机ID列表，最多500条
	Page        *PageParam `json:"page"`                    // 分页信息，必填
}

type HostTopoInfoResp struct {
	Count int             `json:"count"` // 总数
	Data  []*HostTopoInfo `json:"data"`  // 返回结果
}

// HostTopoInfo 主机与拓扑绑定信息
type HostTopoInfo struct {
	BkBizID           int    `json:"bk_biz_id"`           // 业务ID
	BkSetID           int    `json:"bk_set_id"`           // 集群ID
	BkModuleID        int    `json:"bk_module_id"`        // 模块ID
	BkHostID          int    `json:"bk_host_id"`          // 主机ID
	BkSupplierAccount string `json:"bk_supplier_account"` // 开发商账号
}

// ModuleListReq 查询模块请求参数
type ModuleListReq struct {
	BkBizID              int        `json:"bk_biz_id"`                         // 业务ID，必填
	BkSetIDs             []int      `json:"bk_set_ids,omitempty"`              // 集群ID列表，可选，最多200个
	BkServiceTemplateIDs []int      `json:"bk_service_template_ids,omitempty"` // 服务模板ID列表，可选
	Fields               []string   `json:"fields"`                            // 模块属性列表，必填
	Page                 *PageParam `json:"page"`                              // 分页信息，必填
}

// ModuleListResp 查询模块响应参数
type ModuleListResp struct {
	Count int          `json:"count"` // 总数
	Info  []ModuleInfo `json:"info"`  // 模块信息列表
}

// SearchSetReq 查询集群请求参数
type SearchSetReq struct {
	BkSupplierAccount string         `json:"bk_supplier_account,omitempty"` // 开发商账号
	BkBizID           int            `json:"bk_biz_id"`                     // 业务ID, 如果查询业务改字段没有
	Fields            []string       `json:"fields"`                        // 查询字段
	Condition         map[string]any `json:"condition,omitempty"`           // 查询条件（不推荐使用）
	Filter            *Filter        `json:"filter,omitempty"`              // 查询集群列表时的参数
	BizPropertyFilter *Filter        `json:"biz_property_filter,omitempty"` // 查询业务列表时的参数
	TimeCondition     *TimeFilter    `json:"time_condition,omitempty"`      // 按时间查询模型实例的条件
	Page              *PageParam     `json:"page"`                          // 分页参数
}

// Filter 属性组合过滤条件
type Filter struct {
	Condition string `json:"condition"` // 规则操作符 (and / or)
	Rules     []Rule `json:"rules"`     // 过滤规则集合
}

// Rule 单条过滤规则
type Rule struct {
	Field    string      `json:"field"`           // 字段名
	Operator string      `json:"operator"`        // 操作符
	Value    interface{} `json:"value,omitempty"` // 操作数，不同操作符对应不同格式
}

// TimeFilter 按时间查询条件
type TimeFilter struct {
	Oper  string         `json:"oper"`  // 操作符，目前仅支持 "and"
	Rules []TimeRuleItem `json:"rules"` // 时间范围规则
}

// TimeRuleItem 时间查询规则
type TimeRuleItem struct {
	Field string `json:"field"` // 模型字段名
	Start string `json:"start"` // 起始时间 yyyy-MM-dd hh:mm:ss
	End   string `json:"end"`   // 结束时间 yyyy-MM-dd hh:mm:ss
}

// Business 业务数据
type Business struct {
	Count int            `json:"count"` // 记录条数
	Info  []BusinessInfo `json:"info"`  // 业务实际数据
}

// BusinessInfo 单个业务信息
type BusinessInfo struct {
	BkBizID           int    `json:"bk_biz_id"`           // 业务ID
	BkBizName         string `json:"bk_biz_name"`         // 业务名
	BkBizMaintainer   string `json:"bk_biz_maintainer"`   // 运维人员
	BkBizProductor    string `json:"bk_biz_productor"`    // 产品人员
	BkBizDeveloper    string `json:"bk_biz_developer"`    // 开发人员
	BkBizTester       string `json:"bk_biz_tester"`       // 测试人员
	TimeZone          string `json:"time_zone"`           // 时区
	Language          string `json:"language"`            // 语言 (1=中文, 2=英文)
	BkSupplierAccount string `json:"bk_supplier_account"` // 开发商账号
	CreateTime        string `json:"create_time"`         // 创建时间
	LastTime          string `json:"last_time"`           // 更新时间
	Default           int    `json:"default"`             // 业务类型
	Operator          string `json:"operator"`            // 主要维护人
	LifeCycle         string `json:"life_cycle"`          // 生命周期
	BkCreatedAt       string `json:"bk_created_at"`       // 创建时间
	BkUpdatedAt       string `json:"bk_updated_at"`       // 更新时间
	BkCreatedBy       string `json:"bk_created_by"`       // 创建人
}

// SearchModuleReq 查询模块请求参数
type SearchModuleReq struct {
	BkSupplierAccount string         `json:"bk_supplier_account,omitempty"` // 开发商账号
	BkBizID           int            `json:"bk_biz_id"`                     // 业务ID, 必填
	BkSetID           int            `json:"bk_set_id"`                     // 集群ID, 可选
	Fields            []string       `json:"fields"`                        // 查询字段
	Condition         map[string]any `json:"condition,omitempty"`           // 查询条件（不推荐使用）
	Filter            *Filter        `json:"filter,omitempty"`              // 属性组合查询条件
	Page              *PageParam     `json:"page"`                          // 分页参数
}

// ListBizHostsRequest 查询业务主机请求参数
type ListBizHostsRequest struct {
	BkBizID            int                 `json:"bk_biz_id"`            // 业务ID
	Page               PageParam           `json:"page"`                 // 分页参数
	Fields             []string            `json:"fields"`               // 需要返回的字段列表
	HostPropertyFilter *HostPropertyFilter `json:"host_property_filter"` // 主机属性过滤条件
}

// HostPropertyFilter 主机属性过滤条件
type HostPropertyFilter struct {
	Condition string             `json:"condition"` // 逻辑条件，如 "AND", "OR"
	Rules     []HostPropertyRule `json:"rules"`     // 过滤规则列表
}

// HostPropertyRule 主机属性过滤规则
type HostPropertyRule struct {
	Field    string      `json:"field"`    // 字段名
	Operator string      `json:"operator"` // 操作符
	Value    interface{} `json:"value"`    // 值
}

// HostPropertyOperator 主机属性操作符常量
const (
	HostPropertyOperatorEqual = "equal"
)

// HostPropertyCondition 主机属性条件常量
const (
	HostPropertyConditionAnd = "AND"
)

// WatchResourceRequest 监听资源请求参数
type WatchResourceRequest struct {
	BkResource        string      `json:"bk_resource"`         // 资源类型，如 "host", "host_relation"
	BkEventTypes      []string    `json:"bk_event_types"`      // 事件类型，如 ["create", "update", "delete"]
	BkFields          []string    `json:"bk_fields"`           // 需要返回的字段列表
	BkStartFrom       *int64      `json:"bk_start_from"`       // 监听事件的起始时间（该值为unix time的秒数，且仅支持监听3小时内的事件）
	BkCursor          string      `json:"bk_cursor"`           // 监听事件的游标
	BkSupplierAccount string      `json:"bk_supplier_account"` // 供应商账户
	BkFilter          interface{} `json:"bk_filter"`           // 过滤条件
}

// WatchResourceData 监听资源的数据结构
type WatchResourceData[T any] struct {
	BkWatched bool       `json:"bk_watched"` // 是否监听到了事件，true：监听到了事件；false:未监听到事件
	BkEvents  []Event[T] `json:"bk_events"`  // 事件列表
}

// HostRelationWatchData 主机关系监听数据
type HostRelationWatchData = WatchResourceData[HostRelationDetail]

// Event 事件信息
type Event[T any] struct {
	BkCursor    string `json:"bk_cursor"`     // 游标
	BkResource  string `json:"bk_resource"`   // 资源类型
	BkEventType string `json:"bk_event_type"` // 事件类型
	BkDetail    *T     `json:"bk_detail"`     // 事件详情，未监听到事件时未nil
}

// HostRelationDetail 主机关系事件详情
type HostRelationDetail struct {
	BkBizID  *int `json:"bk_biz_id"`  // 业务ID
	BkHostID *int `json:"bk_host_id"` // 主机ID
}

// HostRelationEvent 主机关系事件
type HostRelationEvent = Event[HostRelationDetail]

// FindHostBizRelationsRequest 查询主机业务关系请求参数
type FindHostBizRelationsRequest struct {
	BkBizID  int   `json:"bk_biz_id"`  // 业务ID
	BkHostID []int `json:"bk_host_id"` // 主机ID列表
}

// HostBizRelation 主机业务关系信息
type HostBizRelation struct {
	BkBizID           int    `json:"bk_biz_id"`           // 业务ID
	BkHostID          int    `json:"bk_host_id"`          // 主机ID
	BkModuleID        int    `json:"bk_module_id"`        // 模块ID
	BkSetID           int    `json:"bk_set_id"`           // 集群ID
	BkSupplierAccount string `json:"bk_supplier_account"` // 开发商账号
}

// FindHostBizRelationsResponse 查询主机业务关系响应
type FindHostBizRelationsResponse = CMDBResponseData[[]HostBizRelation]

// WatchData 定义了监听到的事件数据详情
type WatchData struct {
	BkWatched bool         `json:"bk_watched"` // 是否监听到了事件，true：监听到了事件；false：未监听到事件
	BkEvents  []BkEventObj `json:"bk_events"`  // 监听到的事件详情列表（最大长度为200）
}

// BkEventObj 定义了单条监听到的事件
type BkEventObj struct {
	BkCursor    string          `json:"bk_cursor"`     // 当前资源事件的游标值，可用于获取下一个事件
	BkResource  ResourceType    `json:"bk_resource"`   // 事件对应的资源类型，例如 host、host_relation、biz_set_relation 等
	BkEventType EventType       `json:"bk_event_type"` // 事件类型：create（新增）、update（更新）、delete（删除）
	BkDetail    json.RawMessage `json:"bk_detail"`     // 事件对应资源的详情数据（结构随资源类型不同而不同）
}

// Decode 把 Data 部分解码到目标结构里
func (r *BkEventObj) Decode(v any) error {
	if len(r.BkDetail) == 0 {
		return nil
	}
	return json.Unmarshal(r.BkDetail, v)
}

// Encode 把目标结构编码回 Data 部分
func (r *BkEventObj) Encode(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	r.BkDetail = b
	return nil
}

// EventType 表示 CMDB 事件类型
type EventType string

const (
	// EventCreate 表示资源新增事件
	EventCreate EventType = "create"
	// EventUpdate 表示资源更新事件
	EventUpdate EventType = "update"
	// EventDelete 表示资源删除事件
	EventDelete EventType = "delete"
)

// String 返回事件类型字符串
func (e EventType) String() string {
	return string(e)
}

// Validate 校验事件类型是否合法
func (e EventType) Validate() error {
	switch e {
	case EventCreate, EventUpdate, EventDelete:
		return nil
	default:
		return errors.New("invalid CMDB event type")
	}
}

// ResourceType 表示 CMDB 可监听的资源类型
type ResourceType string

const (
	// 主机详情事件
	ResourceHost ResourceType = "host"
	// 主机关系事件
	ResourceHostRelation ResourceType = "host_relation"
	// 业务详情事件
	ResourceBiz ResourceType = "biz"
	// 集群详情事件
	ResourceSet ResourceType = "set"
	// 模块详情事件
	ResourceModule ResourceType = "module"
	// 进程详情事件
	ResourceProcess ResourceType = "process"
	// 通用模型实例事件
	ResourceObjectInstance ResourceType = "object_instance"
	// 主线模型实例事件
	ResourceMainlineInstance ResourceType = "mainline_instance"
	// 业务集事件
	ResourceBizSet ResourceType = "biz_set"
	// 业务集与业务关系事件
	ResourceBizSetRelation ResourceType = "biz_set_relation"
	// 管控区域事件
	ResourcePlat ResourceType = "plat"
	// 项目事件
	ResourceProject ResourceType = "project"
)

// String 返回资源类型字符串
func (r ResourceType) String() string {
	return string(r)
}

// Validate 校验资源类型是否合法
func (r ResourceType) Validate() error {
	switch r {
	case ResourceHost,
		ResourceHostRelation,
		ResourceBiz,
		ResourceSet,
		ResourceModule,
		ResourceProcess,
		ResourceObjectInstance,
		ResourceMainlineInstance,
		ResourceBizSet,
		ResourceBizSetRelation,
		ResourcePlat,
		ResourceProject:
		return nil
	default:
		return errors.New("invalid CMDB resource type")
	}
}

// ListServiceInstanceRequest 查询服务实例请求参数
type ListServiceInstanceReq struct {
	BizID              int64      `json:"bk_biz_id"`            // 业务 ID
	Page               PageParam  `json:"page"`                 // 分页信息
	ModuleID           int64      `json:"bk_module_id"`         // 模块 ID
	HostIDs            []int64    `json:"bk_host_list"`         // 主机 ID 列表
	ServiceInstanceIDs []int64    `json:"service_instance_ids"` // 服务实例 ID 列表
	Selectors          []Selector `json:"selectors"`            // 查询条件选择器
}

// ListServiceInstanceResponse 查询服务实例响应
type ListServiceInstanceResp struct {
	Count int64                 `json:"count"` // 总数量
	Info  []ServiceInstanceInfo `json:"info"`  // 服务实例列表
}

// ListHostByServiceTemplateRequest 按服务模板、模块维度查询主机列表请求
type ListHostByServiceTemplateReq struct {
	BizID              int64     `json:"bk_biz_id"`               // 业务 ID
	ServiceTemplateIDs []int64   `json:"bk_service_template_ids"` // 服务模板 ID 列表
	ModuleIDs          []int64   `json:"bk_module_ids"`           // 模块 ID 列表
	Fields             []string  `json:"fields"`                  // 返回字段列表
	Page               PageParam `json:"page"`                    // 分页参数
}

// ListHostByServiceTemplateResponse 查询主机列表响应
type ListHostByServiceTemplateResp struct {
	Count int        `json:"count"` // 总数
	Info  []HostInfo `json:"info"`  // 返回结果
}
