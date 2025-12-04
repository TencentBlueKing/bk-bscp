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

package config

import (
	"fmt"
	"io"
	"strings"

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/repository"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/render"
)

const (
	// GenerateConfigStepName generate config step name
	GenerateConfigStepName istep.StepName = "GenerateConfig"
	// ConfigGenerateCallbackName 配置生成回调名称
	ConfigGenerateCallbackName istep.CallbackName = "ConfigGenerateCallback"
)

// ConfigExecutor config step executor
type GenerateConfigExecutor struct {
	*common.Executor
	Repo repository.Provider // 仓库服务，用于下载配置内容
}

// NewConfigExecutor new config executor
func NewGenerateConfigExecutor(dao dao.Set, repo repository.Provider) *GenerateConfigExecutor {
	return &GenerateConfigExecutor{
		Executor: &common.Executor{
			Dao: dao,
		},
		Repo: repo,
	}
}

// SetCMDBService 设置 CMDB 服务（用于获取 CC 拓扑 XML）
func (e *GenerateConfigExecutor) SetCMDBService(cmdbService bkcmdb.Service) {
	e.Executor.CMDBService = cmdbService
}

// GenerateConfigPayload generate config payload
type GenerateConfigPayload struct {
	BizID   uint32
	BatchID uint32

	// 任务类型
	OperateType table.ConfigOperateType

	// 操作人
	OperatorUser string

	// 预定义渲染模版可能需要的字段
	ConfigTemplateID  uint32
	ConfigTemplate    *table.ConfigTemplate
	Template          *table.Template
	TemplateRevision  *table.TemplateRevision
	ProcessInstanceID uint32
	ProcessInstance   *table.ProcessInstance
	CcProcessID       uint32
	Process           *table.Process

	ConfigTemplateName string
	ProcessAlias       string
	ModuleInstSeq      uint32
}

// GetProcess 获取 Process
func (p *GenerateConfigPayload) GetProcess() *table.Process {
	return p.Process
}

// GetProcessInstance 获取进程实例
func (p *GenerateConfigPayload) GetProcessInstance() *table.ProcessInstance {
	return p.ProcessInstance
}

// GetModuleInstSeq 获取模块实例序列号
func (p *GenerateConfigPayload) GetModuleInstSeq() uint32 {
	return p.ModuleInstSeq
}

// NeedHelp 是否需要生成 HELP
func (p *GenerateConfigPayload) NeedHelp() bool {
	return false
}

// GenerateConfig generate config
func (e *GenerateConfigExecutor) GenerateConfig(c *istep.Context) error {
	// 1. 获取 payload
	payload := &GenerateConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		return err
	}

	// 2. 从 TemplateRevision 中获取配置模版
	var configContent string
	if payload.TemplateRevision != nil && payload.TemplateRevision.Spec != nil &&
		payload.TemplateRevision.Spec.ContentSpec != nil {
		// 从仓库中下载实际的配置内容
		signature := payload.TemplateRevision.Spec.ContentSpec.Signature
		if signature != "" && e.Repo != nil {
			kt := kit.New()
			// 为模板空间创建 kit
			k := kt.GetKitForRepoTmpl(payload.TemplateRevision.Attachment.TemplateSpaceID)

			// 从仓库下载配置内容
			body, _, err := e.Repo.Download(k, signature)
			if err != nil {
				return fmt.Errorf("download template config content from repo failed, "+
					"template id: %d, signature: %s, error: %w",
					payload.TemplateRevision.Attachment.TemplateID, signature, err)
			}
			defer body.Close()

			// 读取内容
			content, err := io.ReadAll(body)
			if err != nil {
				return fmt.Errorf("read template config content failed, "+
					"template id: %d, signature: %s, error: %w",
					payload.TemplateRevision.Attachment.TemplateID, signature, err)
			}

			configContent = string(content)
		}
	}

	// 3. 构建渲染上下文并渲染模板
	var renderedContent string
	if configContent != "" {
		source := &payloadWithTemplate{
			payload:         payload,
			templateContent: configContent,
		}
		contextParams := render.BuildProcessContextParamsFromSource(c.Context(), source, e.Executor.CMDBService)
		logs.V(3).Infof("build process context params from source, context params: %+v, template id: %d",
			contextParams, payload.TemplateRevision.Attachment.TemplateID)
		// 使用公共方法渲染模板
		var err error
		renderedContent, err = render.Template(configContent, contextParams)
		if err != nil {
			logs.Errorf("render template failed, template id: %d, error: %v",
				payload.TemplateRevision.Attachment.TemplateID, err)
			return fmt.Errorf("render template failed, template id: %d, error: %w",
				payload.TemplateRevision.Attachment.TemplateID, err)
		}
	}

	// 将渲染结果存储到 CommonPayload 中
	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[Finalize STEP]: get common payload failed: %w", err)
	}
	commonPayload.ConfigPayload.ConfigInstanceKey = generateConfigKey(
		payload.ConfigTemplateID,
		payload.CcProcessID,
		payload.ModuleInstSeq,
	)
	commonPayload.ConfigPayload.ConfigContent = renderedContent
	if err := c.SetCommonPayload(commonPayload); err != nil {
		return fmt.Errorf("[Finalize STEP]: set common payload failed: %w", err)
	}

	return nil
}

// payloadWithTemplate 包装 GenerateConfigPayload 和模板内容，实现 ProcessInfoSource 接口
type payloadWithTemplate struct {
	payload         *GenerateConfigPayload
	templateContent string
}

func (p *payloadWithTemplate) GetProcess() *table.Process {
	return p.payload.GetProcess()
}

func (p *payloadWithTemplate) GetProcessInstance() *table.ProcessInstance {
	return p.payload.GetProcessInstance()
}

func (p *payloadWithTemplate) GetModuleInstSeq() uint32 {
	return p.payload.GetModuleInstSeq()
}

func (p *payloadWithTemplate) NeedHelp() bool {
	return strings.Contains(p.templateContent, "${HELP}")
}

// generateConfigKey 生成配置的key
// 格式: 配置模版ID-ccProcessID-模块下进程实例序列号
func generateConfigKey(configTemplateID, ccProcessID, moduleInstSeq uint32) string {
	return fmt.Sprintf("%d-%d-%d", configTemplateID, ccProcessID, moduleInstSeq)
}

// Callback 配置生成回调方法
// cbErr: 如果为 nil 表示任务成功，否则表示任务失败
func (e *GenerateConfigExecutor) Callback(c *istep.Context, cbErr error) error {
	payload := &GenerateConfigPayload{}
	if err := c.GetPayload(payload); err != nil {
		return fmt.Errorf("[ConfigGenerateCallback]: get payload failed: %w", err)
	}

	// 更新 TaskBatch 状态
	isSuccess := cbErr == nil
	if err := e.Dao.TaskBatch().IncrementCompletedCount(kit.New(), payload.BatchID, isSuccess); err != nil {
		return fmt.Errorf("[ConfigGenerateCallback]: increment completed count failed, batchID: %d, err: %w",
			payload.BatchID, err)
	}

	return nil
}

// RegisterExecutor register executor
func RegisterExecutor(e *GenerateConfigExecutor) {
	istep.Register(GenerateConfigStepName, istep.StepExecutorFunc(e.GenerateConfig))
	istep.RegisterCallback(ConfigGenerateCallbackName, istep.CallbackExecutorFunc(e.Callback))
}
