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

package render

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/processor/cmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// ProcessContextParams 用于构建进程模板渲染上下文的参数
// 完全参考 Python 代码中的 get_process_context 函数
// https://github.com/TencentBlueKing/bk-process-config-manager/blob/a03a937ecea681f8d3f58a3f05cff6ed83ba4f7c/apps/gsekit/configfile/handlers/config_version.py#L268
// nolint
type ProcessContextParams struct {
	// 进程实例序列号（对应 Python 中的 inst_id 和 local_inst_id）
	ModuleInstSeq int // 模块级别的自增ID（对应 ModuleInstSeq）
	HostInstSeq   int // 主机级别的自增ID（对应 HostInstSeq）

	// 进程基本信息（对应 Python process_info）
	SetName     string // bk_set_name
	ModuleName  string // bk_module_name
	ServiceName string // service_instance name
	ProcessName string // bk_process_name
	ProcessID   int    // bk_process_id
	FuncName    string // bk_func_name (对应 FuncName)
	WorkPath    string // work_path (对应 WorkPath)
	PidFile     string // pid_file (对应 PidFile)

	// 主机信息
	HostInnerIP string // bk_host_innerip
	CloudID     int    // bk_cloud_id

	// 可选字段
	// CcXML 是包含整个业务拓扑结构的 XML 数据（可选）
	// 结构：Business(根节点) -> Set(集群) -> Module(模块) -> Host(主机)
	// 应该包含业务下所有的 Set、Module 和 Host 信息，以便模板可以通过 XPath 查询任意节点
	// 例如：cc.findall('.//Host') 可以查询所有主机
	//      cc.findall('.//Set[@SetName="xxx"]') 可以查询指定集群
	// Python 脚本会自动解析 cc_xml 为 cc 对象（lxml Element），
	// 并根据 bk_set_name/bk_module_name/bk_host_innerip/bk_cloud_id 自动构建 this 对象
	// （包含 this.cc_set, this.cc_module, this.cc_host, this.attrib）
	// 参考：render/python/main.py 中的 build_cc_context 函数
	// 示例结构：
	//   <?xml version="1.0" encoding="UTF-8"?>
	//   <Business Name="biz_name">
	//     <Set SetName="set-A" ...>
	//       <Module ModuleName="module-X" ...>
	//         <Host InnerIP="10.0.0.1" bk_cloud_id="0" ... />
	//       </Module>
	//     </Set>
	//   </Business>
	CcXML           string                 // CC XML 数据（可选）
	GlobalVariables map[string]interface{} // 全局变量（可选，biz_global_variables）
	WithHelp        bool                   // 是否生成 HELP（如果模板中包含 ${HELP}，则设置为 true）
}

// BuildProcessContext 构建进程模板渲染的上下文
// 完全按照 Python 代码中的 get_process_context 函数构建，保证变量名和顺序完全一致
// https://github.com/TencentBlueKing/bk-process-config-manager/blob/a03a937ecea681f8d3f58a3f05cff6ed83ba4f7c/apps/gsekit/configfile/handlers/config_version.py#L268
// nolint
func BuildProcessContext(params ProcessContextParams) map[string]interface{} {
	instID := params.ModuleInstSeq
	localInstID := params.HostInstSeq

	// 构建 Scope: "{bk_set_name}.{bk_module_name}.{service_instance_name}.{bk_process_name}.{bk_process_id}"
	scope := ""
	if params.SetName != "" && params.ModuleName != "" && params.ServiceName != "" && params.ProcessName != "" {
		scope = fmt.Sprintf("%s.%s.%s.%s.%d", params.SetName, params.ModuleName, params.ServiceName, params.ProcessName, params.ProcessID)
	}

	// 完全按照 Python 代码的顺序和变量名构建 context
	context := map[string]interface{}{
		// Python 代码中的原始字段（按顺序）
		"Scope":           scope,
		"FuncID":          params.ProcessName, // Python: bk_process_name
		"InstID":          instID,
		"InstID0":         instID - 1,
		"ModuleInstSeq":   params.ModuleInstSeq,
		"HostInstSeq":     params.HostInstSeq,
		"LocalInstID":     localInstID,
		"LocalInstID0":    localInstID - 1,
		"bk_set_name":     params.SetName,
		"bk_module_name":  params.ModuleName,
		"bk_host_innerip": params.HostInnerIP,
		"bk_cloud_id":     params.CloudID,
		"bk_process_id":   params.ProcessID,
		"bk_process_name": params.ProcessName,
		"FuncName":        params.FuncName,    // Python: process_info["process"]["bk_func_name"]
		"ProcName":        params.ProcessName, // Python: process_info["process"]["bk_process_name"]
		"WorkPath":        params.WorkPath,    // Python: process_info["process"]["work_path"]
		"PidFile":         params.PidFile,     // Python: process_info["process"]["pid_file"]
	}

	// 添加 CC XML（如果提供）
	// Python 脚本会自动解析 cc_xml 为 cc 对象（lxml Element）
	// 并根据 context 中的 bk_set_name/bk_module_name/bk_host_innerip/bk_cloud_id
	// 自动构建 this 对象（包含 this.cc_set, this.cc_module, this.cc_host, this.attrib）
	// 详见 render/python/main.py 中的 build_cc_context 函数
	if params.CcXML != "" {
		context["cc_xml"] = params.CcXML
	}

	// 添加全局变量（如果提供）
	// Python 代码中会从 biz_global_variables 补充内置字段
	// 参考 Python 代码逻辑：
	//   - biz_global_variables 按对象类型分组：{ "set": [...], "module": [...], "host": [...] }
	//   - 在 Python 脚本中会从 this.cc_set.attrib、this.cc_module.attrib、this.cc_host.attrib 中提取属性值
	//   - 补充内置字段：for bk_obj_id, bk_obj_variables in biz_global_variables.items():
	//                    for variable in bk_obj_variables:
	//                        bk_property_id = variable["bk_property_id"]
	//                        context[bk_property_id] = getattr(this_context, f"cc_{bk_obj_id}").attrib.get(bk_property_id)
	if len(params.GlobalVariables) > 0 {
		// 将 biz_global_variables 添加到 context 中，供 Python 脚本使用
		// Python 脚本会在 build_cc_context 中处理 biz_global_variables
		if bizGlobalVars, ok := params.GlobalVariables["biz_global_variables"]; ok {
			context["biz_global_variables"] = bizGlobalVars
		} else {
			// 如果没有 biz_global_variables key，直接将整个 GlobalVariables 作为 biz_global_variables
			// 这样可以兼容不同的传递方式
			context["biz_global_variables"] = params.GlobalVariables
		}

		// 同时将其他全局变量也添加到 context 中（跳过 biz_global_variables 和 global_variables）
		for k, v := range params.GlobalVariables {
			if k != "global_variables" && k != "biz_global_variables" {
				context[k] = v
			}
		}
	}

	// Python 代码中最后会设置：context["global_variables"] = context
	// 这允许模板中通过 global_variables 访问所有变量
	// 注意：为了避免 JSON 编码时的循环引用，我们在 Go 端不设置 global_variables
	// 而是在 Python 端设置（在 build_cc_context 之后），这样可以避免 JSON 编码问题
	// Python 脚本会在接收到 context 后设置 context["global_variables"] = context

	// 如果 with_help 为 true，生成 HELP（参考 Python 代码第 359-361 行）
	// Python 代码：if with_help: context["HELP"] = mako_render(HELP_TEMPLATE, context)
	// 注意：HELP 的生成需要在 Python 脚本中完成，因为需要使用 MakoSandbox
	// 这里只设置一个标记，Python 脚本会检查并生成 HELP
	if params.WithHelp {
		// HELP 的生成在 Python 脚本中完成，这里只标记需要生成
		// Python 脚本会在 build_cc_context 之后检查 context 中是否有 with_help 标记
		context["_with_help"] = true
	}

	return context
}

// Template 渲染模板的便捷函数
// 自动构建 context 并执行渲染
func Template(template string, params ProcessContextParams) (string, error) {
	if template == "" {
		return "", nil
	}

	// 获取渲染器
	renderer, err := GetDefaultRenderer()
	if err != nil {
		logs.Errorf("get default renderer failed, error: %v", err)
		return "", fmt.Errorf("get default renderer failed: %w", err)
	}

	// 构建 context
	context := BuildProcessContext(params)

	// 执行渲染
	return renderer.Render(template, context)
}

// ProcessInfoSource 进程信息源接口
// 用于从不同来源（task payload、preview request 等）获取进程信息
type ProcessInfoSource interface {
	// GetProcess 获取 Process 对象（可以为 nil）
	GetProcess() *table.Process
	// GetProcessInstance 获取 ProcessInstance 对象（可以为 nil）
	GetProcessInstance() *table.ProcessInstance
	// GetModuleInstSeq 获取模块实例序列号（如果提供，优先使用）
	GetModuleInstSeq() uint32
	// NeedHelp 是否需要生成 HELP
	NeedHelp() bool
}

// BuildProcessContextParamsFromSource 从 ProcessInfoSource 构建渲染上下文参数
// 这是公共函数，用于统一处理从不同来源获取的进程信息
func BuildProcessContextParamsFromSource(
	ctx context.Context,
	source ProcessInfoSource,
	cmdbService bkcmdb.Service,
) ProcessContextParams {
	var (
		moduleInstSeq int
		hostInstSeq   int
		setName       string
		moduleName    string
		serviceName   string
		processName   string
		processID     int
		funcName      string
		workPath      string
		pidFile       string
		hostInnerIP   string
		cloudID       int
	)

	process := source.GetProcess()
	processInstance := source.GetProcessInstance()

	// 从 ProcessInstance 获取序列号（对应 Python 中的 inst_id 和 local_inst_id）
	if processInstance != nil && processInstance.Spec != nil {
		moduleInstSeq = int(processInstance.Spec.ModuleInstSeq)
		hostInstSeq = int(processInstance.Spec.HostInstSeq)
	}

	// 如果 source 中提供了 ModuleInstSeq，优先使用
	if source.GetModuleInstSeq() > 0 {
		moduleInstSeq = int(source.GetModuleInstSeq())
	}

	// 从 Process 获取进程信息
	if process != nil {
		if process.Spec != nil {
			setName = process.Spec.SetName
			moduleName = process.Spec.ModuleName
			serviceName = process.Spec.ServiceName
			processName = process.Spec.Alias
			funcName = process.Spec.FuncName
			hostInnerIP = process.Spec.InnerIP

			// 从 SourceData 中解析 ProcessInfo 获取 WorkPath 和 PidFile
			if process.Spec.SourceData != "" {
				var processInfo table.ProcessInfo
				if err := json.Unmarshal([]byte(process.Spec.SourceData), &processInfo); err == nil {
					workPath = processInfo.WorkPath
					pidFile = processInfo.PidFile
				} else {
					logs.Warnf("unmarshal process info failed, process id: %d, error: %v",
						process.Attachment.CcProcessID, err)
				}
			}
		}
		if process.Attachment != nil {
			processID = int(process.Attachment.CcProcessID)
			cloudID = int(process.Attachment.CloudID)
		}
	}
	var bizID uint32
	if process != nil && process.Attachment != nil {
		bizID = process.Attachment.BizID
	}
	// 获取 CC XML 和全局变量（参考 Python 代码中的 cache_topo_tree_attr 方法）
	var ccXML string
	var globalVars map[string]interface{}
	if cmdbService != nil && bizID > 0 {
		// 创建 CC 拓扑 XML 服务
		topoService := cmdb.NewCCTopoXMLService(int(bizID), cmdbService)

		// 获取进程所属 Set 的环境类型（bk_set_env），用于过滤拓扑
		var setEnv string
		if process != nil && process.Spec != nil {
			setEnv = process.Spec.Environment
		}

		// 获取拓扑 XML
		xmlStr, err := topoService.GetTopoTreeXML(ctx, setEnv)
		if err != nil {
			// 如果获取失败，记录警告但不中断渲染流程
			logs.Warnf("get cc topo xml failed, biz id: %d, error: %v", bizID, err)
		} else {
			ccXML = xmlStr
		}

		// 获取全局变量（biz_global_variables）
		globalVarsMap, err := topoService.GetBizGlobalVariablesMap(ctx)
		if err != nil {
			// 如果获取失败，记录警告但不中断渲染流程
			logs.Warnf("get biz global variables failed, biz id: %d, error: %v", bizID, err)
			globalVars = make(map[string]interface{})
		} else {
			// 将 biz_global_variables 包装在 map 中，供 render/context.go 使用
			globalVars = map[string]interface{}{
				"biz_global_variables": globalVarsMap,
			}
		}
	} else {
		// 如果没有 CMDB 服务，初始化为空 map
		globalVars = make(map[string]interface{})
	}

	return ProcessContextParams{
		ModuleInstSeq:   moduleInstSeq,
		HostInstSeq:     hostInstSeq,
		SetName:         setName,
		ModuleName:      moduleName,
		ServiceName:     serviceName,
		ProcessName:     processName,
		ProcessID:       processID,
		FuncName:        funcName,
		WorkPath:        workPath,
		PidFile:         pidFile,
		HostInnerIP:     hostInnerIP,
		CloudID:         cloudID,
		CcXML:           ccXML,
		GlobalVariables: globalVars,
		WithHelp:        source.NeedHelp(),
	}
}
