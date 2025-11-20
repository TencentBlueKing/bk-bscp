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

package gse

import (
	"fmt"
	"sync"

	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/render"
)

var (
	// defaultRenderer is a singleton Renderer instance reused across multiple calls
	defaultRenderer     *render.Renderer
	defaultRendererOnce sync.Once
	defaultRendererErr  error
)

// getDefaultRenderer returns a singleton Renderer instance
// It initializes the renderer on first call and reuses it for subsequent calls
func getDefaultRenderer() (*render.Renderer, error) {
	defaultRendererOnce.Do(func() {
		defaultRenderer, defaultRendererErr = render.NewRenderer()
		if defaultRendererErr != nil {
			logs.Errorf("failed to initialize default renderer: %+v", defaultRendererErr)
		}
	})
	return defaultRenderer, defaultRendererErr
}

// BuildProcessOperateParams 构建 ProcessOperate 的参数
type BuildProcessOperateParams struct {
	BizID         uint32            // 业务ID
	Alias         string            // 进程别名
	FuncName      string            // 进程二进制文件名
	HostInstSeq   uint32            // 主机级别的自增ID
	ModuleInstSeq uint32            // 模块级别的自增ID
	SetName       string            // 集群名称（用于模板渲染）
	ModuleName    string            // 模块名称（用于模板渲染）
	AgentID       []string          // Agent ID列表
	GseOpType     gse.OpType        // GSE操作类型
	ProcessInfo   table.ProcessInfo // 进程配置信息
}

// BuildProcessOperate 构建 GSE ProcessOperate 对象
// 所有操作类型都建议传入全量参数
// 渲染需要完全兼容：https://github.com/TencentBlueKing/bk-process-config-manager/blob/V1.0.X/apps/gsekit/pipeline_plugins/components/collections/gse.py#L327
func BuildProcessOperate(params BuildProcessOperateParams) (*gse.ProcessOperate, error) {
	// 验证必填参数
	if err := validateBuildProcessOperateParams(params); err != nil {
		logs.Errorf("validate build process operate params failed, err: %+v", err)
		return nil, err
	}
	// 构建模板渲染的上下文
	renderContext := buildRenderContext(params)

	// 获取单例渲染器（复用实例，避免重复创建和验证）
	renderer, err := getDefaultRenderer()
	if err != nil {
		logs.Errorf("build process operate failed, err: %+v", err)
		return nil, err
	}

	// 渲染需要模板化的字段
	workPath, err := renderField(renderer, params.ProcessInfo.WorkPath, renderContext)
	if err != nil {
		logs.Errorf("render work path failed, err: %+v", err)
		return nil, err
	}
	pidFile, err := renderField(renderer, params.ProcessInfo.PidFile, renderContext)
	if err != nil {
		logs.Errorf("render pid file failed, err: %+v", err)
		return nil, err
	}
	startCmd, err := renderField(renderer, params.ProcessInfo.StartCmd, renderContext)
	if err != nil {
		logs.Errorf("render start cmd failed, err: %+v", err)
		return nil, err
	}
	stopCmd, err := renderField(renderer, params.ProcessInfo.StopCmd, renderContext)
	if err != nil {
		logs.Errorf("render stop cmd failed, err: %+v", err)
		return nil, err
	}
	restartCmd, err := renderField(renderer, params.ProcessInfo.RestartCmd, renderContext)
	if err != nil {
		logs.Errorf("render restart cmd failed, err: %+v", err)
		return nil, err
	}
	reloadCmd, err := renderField(renderer, params.ProcessInfo.ReloadCmd, renderContext)
	if err != nil {
		logs.Errorf("render reload cmd failed, err: %+v", err)
		return nil, err
	}
	killCmd, err := renderField(renderer, params.ProcessInfo.FaceStopCmd, renderContext)
	if err != nil {
		logs.Errorf("render kill cmd failed, err: %+v", err)
		return nil, err
	}

	// 构建基础的 ProcessOperate 对象
	processOperate := &gse.ProcessOperate{
		Meta: gse.ProcessMeta{
			Namespace: gse.BuildNamespace(params.BizID),
			Name:      gse.BuildProcessName(params.Alias, params.HostInstSeq),
		},
		AgentIDList: params.AgentID,
		OpType:      params.GseOpType,
		Spec: gse.ProcessSpec{
			Identity: gse.ProcessIdentity{
				ProcName:  params.FuncName,
				SetupPath: workPath,
				PidPath:   pidFile,
				User:      params.ProcessInfo.User,
			},
			Control: gse.ProcessControl{
				StartCmd:   startCmd,
				StopCmd:    stopCmd,
				RestartCmd: restartCmd,
				ReloadCmd:  reloadCmd,
				KillCmd:    killCmd,
			},
			Resource: gse.ProcessResource{
				CPU: DefaultCPULimit,
				Mem: DefaultMemLimit,
			},
			MonitorPolicy: gse.ProcessMonitorPolicy{
				AutoType:       gse.AutoTypePersistent,
				StartCheckSecs: DefaultStartCheckSecs,
				OpTimeout:      params.ProcessInfo.Timeout,
			},
		},
	}
	return processOperate, nil
}

func validateBuildProcessOperateParams(params BuildProcessOperateParams) error {
	if params.BizID == 0 {
		return fmt.Errorf("bizID is required")
	}
	if params.Alias == "" {
		return fmt.Errorf("alias is required")
	}

	// 验证 hostInstSeq 和 moduleInstSeq 必须大于 0
	if params.HostInstSeq == 0 {
		return fmt.Errorf("hostInstSeq is required")
	}
	if params.ModuleInstSeq == 0 {
		return fmt.Errorf("moduleInstSeq is required")
	}
	return nil
}

// buildRenderContext 构建模板渲染的上下文
// 参考 Python 代码中的 context 结构
func buildRenderContext(params BuildProcessOperateParams) map[string]interface{} {
	moduleInstSeq := params.ModuleInstSeq
	hostInstSeq := params.HostInstSeq

	context := map[string]interface{}{
		// bscp 字段
		"module_inst_seq":   moduleInstSeq,
		"module_inst_seq_0": moduleInstSeq - 1,
		"host_inst_seq":     hostInstSeq,
		"host_inst_seq_0":   hostInstSeq - 1,
		// [gsekit]新版本字段(key不能修改，需要兼容原来定义，)
		"inst_id":         moduleInstSeq,
		"inst_id_0":       moduleInstSeq - 1,
		"local_inst_id":   hostInstSeq,
		"local_inst_id0":  hostInstSeq - 1,
		"bk_set_name":     params.SetName,
		"bk_module_name":  params.ModuleName,
		"bk_process_name": params.Alias,
		// [gsekit]兼容老版本字段
		"InstID":       moduleInstSeq,
		"InstID0":      moduleInstSeq - 1,
		"LocalInstID":  hostInstSeq,
		"LocalInstID0": hostInstSeq - 1,
		"SetName":      params.SetName,
		"ModuleName":   params.ModuleName,
		"FuncID":       params.Alias,
	}

	return context
}

// renderField 渲染单个字段
func renderField(renderer *render.Renderer, template string, context map[string]interface{}) (string, error) {
	if template == "" {
		logs.Warnf("template is empty")
		return "", nil
	}

	result, err := renderer.Render(template, context)
	if err != nil {
		logs.Errorf("render field failed, template: %s, context: %+v, err: %+v", template, context, err)
		return "", err
	}

	return result, nil
}
