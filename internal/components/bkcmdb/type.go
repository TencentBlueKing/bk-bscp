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

// Biz is cmdb biz info.
type Biz struct {
	BizID         int64  `json:"bk_biz_id"`
	BizName       string `json:"bk_biz_name"`
	BizMaintainer string `json:"bk_biz_maintainer"`
}

// FindHostByTopoReq xxx
type FindHostByTopoReq struct {
	BkBizID  int       `json:"bk_biz_id"`  // 业务ID
	BkObjID  string    `json:"bk_obj_id"`  // 拓扑节点模型ID，拓扑节点模型ID，如集群（set）、模块（module）或其他topo节点上的模型，不可为biz、host
	BkInstID int       `json:"bk_inst_id"` // 拓扑节点实例ID
	Fields   []string  `json:"fields"`     // 主机属性列表，控制返回结果的主机里有哪些字段，能够加速接口请求和减少网络流量传输
	Page     PageParam `json:"page"`       // 分页信息
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
type CMDBResponse[T any] struct {
	Result     bool   `json:"result"`     // 请求成功与否
	Code       int    `json:"code"`       // 错误编码
	Message    string `json:"message"`    // 错误信息
	Data       T      `json:"data"`       // 泛型，具体数据结构
	Permission any    `json:"permission"` // 权限信息（可以根据需要定义 struct）
}

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

// PageParam 公共分页参数
type PageParam struct {
	Start int    `json:"start"`          // 记录开始位置，默认0
	Limit int    `json:"limit"`          // 每页限制条数，最大500
	Sort  string `json:"sort,omitempty"` // 排序字段
}

// ListServiceTemplateReq xxx
type ListServiceTemplateReq struct {
	BkBizID            int       `json:"bk_biz_id"`                      // 业务ID（必选）
	ServiceCategoryID  int       `json:"service_category_id,omitempty"`  // 服务分类ID（可选）
	Search             string    `json:"search,omitempty"`               // 按服务模板名查询，默认为空
	IsExact            bool      `json:"is_exact,omitempty"`             // 是否精确匹配（搭配 search 使用）
	ServiceTemplateIDs []int     `json:"service_template_ids,omitempty"` // 服务模板ID列表
	Page               PageParam `json:"page"`                           // 分页参数（必选）
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

// ProcTemplate xxx
type ProcTemplate struct {
	ID                int      `json:"id"`
	BkProcessName     string   `json:"bk_process_name"`
	BkBizID           int      `json:"bk_biz_id"`
	ServiceTemplateID int      `json:"service_template_id"`
	Property          Property `json:"property"`
	Creator           string   `json:"creator"`
	Modifier          string   `json:"modifier"`
	CreateTime        string   `json:"create_time"`
	LastTime          string   `json:"last_time"`
	BkSupplierAccount string   `json:"bk_supplier_account"`
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

// ListProcTemplateReq xxx
type ListProcTemplateReq struct {
	BkBizID           int `json:"bk_biz_id"`           // 业务ID
	ServiceTemplateID int `json:"service_template_id"` // 服务实例ID
	ProcessTemplateID int `json:"process_template_id"`
}

// ListProcessInstanceReq xxx
type ListProcessInstanceReq struct {
	BkBizID           int `json:"bk_biz_id"`           // 业务ID
	ServiceTemplateID int `json:"service_template_id"` // 服务实例ID
}

// ListProcessInstance 进程数据
type ListProcessInstance struct {
	Property ProcessInfo `json:"property"` // 进程属性信息
	Relation Relation    `json:"relation"` // 进程与服务实例的关联信息
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
	BkBizID              int       `json:"bk_biz_id"`               // 业务ID，必填
	BkServiceTemplateIDs []int     `json:"bk_service_template_ids"` // 集群模板ID列表，最多可填500个，必填
	BkSetIDs             []int     `json:"bk_set_ids,omitempty"`    // 集群ID列表，最多可填500个，可选
	Fields               []string  `json:"fields"`                  // 主机属性列表，控制返回结果的模块信息里有哪些字段，必填
	Page                 PageParam `json:"page"`                    // 分页信息，必填
}

// HostInfo 主机信息
type HostInfo struct {
	BkHostName        string `json:"bk_host_name"`         // 主机名
	BkHostInnerIP     string `json:"bk_host_innerip"`      // 内网IP
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

// ListSetTemplateReq xxx
type ListSetTemplateReq struct {
	BkBizID        int       `json:"bk_biz_id"`                  // 业务ID，必填
	SetTemplateIDs []int     `json:"set_template_ids,omitempty"` // 集群模板ID数组，可选
	Page           PageParam `json:"page,omitempty"`             // 分页信息，可选
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
type ProcessRequest struct {
	BkBizID      int      `json:"bk_biz_id"`        // 进程所在的业务ID，必填
	BkProcessIDs []int    `json:"bk_process_ids"`   // 进程ID列表，最多传500个，必填
	Fields       []string `json:"fields,omitempty"` // 进程属性列表，可选，控制返回结果字段，加速请求
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	AutoStart         bool       `json:"auto_start"`           // 是否自动拉起
	BkBizID           int        `json:"bk_biz_id"`            // 业务id
	BkFuncName        string     `json:"bk_func_name"`         // 进程名称
	BkProcessID       int        `json:"bk_process_id"`        // 进程id
	BkProcessName     string     `json:"bk_process_name"`      // 进程别名
	BkStartParamRegex string     `json:"bk_start_param_regex"` // 进程启动参数
	BkSupplierAccount string     `json:"bk_supplier_account"`  // 开发商账号
	CreateTime        string     `json:"create_time"`          // 创建时间
	Description       string     `json:"description"`          // 描述
	FaceStopCmd       string     `json:"face_stop_cmd"`        // 强制停止命令
	LastTime          string     `json:"last_time"`            // 更新时间
	PidFile           string     `json:"pid_file"`             // PID文件路径
	Priority          int        `json:"priority"`             // 启动优先级
	ProcNum           int        `json:"proc_num"`             // 启动数量
	ReloadCmd         string     `json:"reload_cmd"`           // 进程重载命令
	RestartCmd        string     `json:"restart_cmd"`          // 重启命令
	StartCmd          string     `json:"start_cmd"`            // 启动命令
	StopCmd           string     `json:"stop_cmd"`             // 停止命令
	Timeout           int        `json:"timeout"`              // 操作超时时长
	User              string     `json:"user"`                 // 启动用户
	WorkPath          string     `json:"work_path"`            // 工作路径
	BkCreatedAt       string     `json:"bk_created_at"`        // 创建时间
	BkCreatedBy       string     `json:"bk_created_by"`        // 创建人
	BkUpdatedAt       string     `json:"bk_updated_at"`        // 更新时间
	BkUpdatedBy       string     `json:"bk_updated_by"`        // 更新人
	BindInfo          []BindInfo `json:"bind_info"`            // 绑定信息列表
}

// ServiceInstanceRequest 查询服务实例请求参数
type ServiceInstanceRequest struct {
	BkBizID       int       `json:"bk_biz_id"`       // 业务id，必填
	SetTemplateID int       `json:"set_template_id"` // 集群模版ID，必填
	Page          PageParam `json:"page"`            // 分页参数，必填
}

// ServiceInstanceInfo 服务实例信息
type ServiceInstanceInfo struct {
	ID                int               `json:"id"`                  // 服务实例ID
	Name              string            `json:"name"`                // 服务实例名称
	BkBizID           int               `json:"bk_biz_id"`           // 业务id
	BkModuleID        int               `json:"bk_module_id"`        // 模型id
	ServiceTemplateID int               `json:"service_template_id"` // 服务模版ID
	Labels            map[string]string `json:"labels"`              // 标签信息
	BkHostID          int               `json:"bk_host_id"`          // 主机id
	Creator           string            `json:"creator"`             // 本条数据创建者
	Modifier          string            `json:"modifier"`            // 本条数据的最后修改人员
	CreateTime        string            `json:"create_time"`         // 创建时间
	LastTime          string            `json:"last_time"`           // 更新时间
	BkSupplierAccount string            `json:"bk_supplier_account"` // 开发商账号
}

// ModuleRequest 查询模块信息请求
type ModuleRequest struct {
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
