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
	"time"

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	pushmanager "github.com/TencentBlueKing/bk-bscp/internal/components/push_manager"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/repository"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
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
func NewGenerateConfigExecutor(dao dao.Set, cmdbService bkcmdb.Service, repo repository.Provider,
	pm pushmanager.Service) *GenerateConfigExecutor {
	return &GenerateConfigExecutor{
		Executor: &common.Executor{
			Dao:         dao,
			CMDBService: cmdbService,
			PM:          pm,
		},
		Repo: repo,
	}
}

// SetCMDBService 设置 CMDB 服务（用于获取 CC 拓扑 XML）
func (e *GenerateConfigExecutor) SetCMDBService(cmdbService bkcmdb.Service) {
	e.CMDBService = cmdbService
}

// GenerateConfigPayload generate config payload
type GenerateConfigPayload struct {
	BizID              uint32
	BatchID            uint32
	ConfigTemplateID   uint32
	ConfigTemplateName string
	OperateType        table.ConfigOperateType
	OperatorUser       string
	Template           *table.Template
	TemplateRevision   *table.TemplateRevision
	Process            *table.Process
	ProcessInstance    *table.ProcessInstance
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
	return p.ProcessInstance.Spec.ModuleInstSeq
}

// NeedHelp 是否需要生成 HELP
func (p *GenerateConfigPayload) NeedHelp() bool {
	return false
}

// GenerateConfig generate config
func (e *GenerateConfigExecutor) GenerateConfig(c *istep.Context) error {
	kt := kit.New()

	// 1. 获取 payload
	generatePayload := &GenerateConfigPayload{}
	if err := c.GetPayload(generatePayload); err != nil {
		logs.Errorf("[GenerateConfig STEP]: get payload failed, err=%v", err)
		return err
	}

	// 将渲染结果存储到 CommonPayload 中
	commonPayload := &common.TaskPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		logs.Errorf("[GenerateConfig STEP]: get common payload failed: %d, error: %v",
			generatePayload.TemplateRevision.Attachment.TemplateID, err)
		return fmt.Errorf("[GenerateConfig STEP]: get common payload failed: %w", err)
	}

	if commonPayload.ConfigPayload == nil {
		logs.Errorf("[GenerateConfig STEP]: common payload config payload is nil, template id: %d",
			generatePayload.TemplateRevision.Attachment.TemplateID)
		return fmt.Errorf("common payload config payload is nil, template id: %d",
			generatePayload.TemplateRevision.Attachment.TemplateID)
	}

	kt = kt.GetKitForRepoTmpl(generatePayload.TemplateRevision.Attachment.TemplateSpaceID)
	kt.BizID = generatePayload.BizID

	// 2. 从 TemplateRevision 中获取配置模版
	var configContent string
	// generatePayload.Payload.ConfigPayload.
	if generatePayload.TemplateRevision != nil && generatePayload.TemplateRevision.Spec != nil &&
		generatePayload.TemplateRevision.Spec.ContentSpec != nil {
		// 从仓库中下载实际的配置内容
		signature := generatePayload.TemplateRevision.Spec.ContentSpec.Signature
		if signature != "" && e.Repo != nil {
			// 从仓库下载配置内容
			downloadStart := time.Now()
			body, _, err := e.Repo.Download(kt, signature)
			if err != nil {
				logs.Errorf("[GenerateConfig STEP]: download template config content from repo failed, "+
					"cost:%s", "template id: %d, signature: %s, error: %v", time.Since(downloadStart),
					generatePayload.TemplateRevision.Attachment.TemplateID, signature, err)

				return fmt.Errorf("download template config content from repo failed, "+
					"template id: %d, signature: %s, error: %w",
					generatePayload.TemplateRevision.Attachment.TemplateID, signature, err)
			}

			logs.Infof(
				"[GenerateConfig STEP]: download template success, cost=%s, template_id=%d, signature=%s",
				time.Since(downloadStart),
				generatePayload.TemplateRevision.Attachment.TemplateID,
				signature,
			)

			defer body.Close()

			// 读取内容
			content, err := io.ReadAll(body)
			if err != nil {
				logs.Errorf("[GenerateConfig STEP]: read template config content failed, "+
					"template id: %d, signature: %s, error: %v",
					generatePayload.TemplateRevision.Attachment.TemplateID, signature, err)
				return fmt.Errorf("read template config content failed, "+
					"template id: %d, signature: %s, error: %w",
					generatePayload.TemplateRevision.Attachment.TemplateID, signature, err)
			}

			configContent = string(content)
		}
	}

	// 3. 构建渲染上下文并渲染模板
	var renderedContent string
	if configContent != "" {
		source := &payloadWithTemplate{
			payload:         generatePayload,
			templateContent: configContent,
		}
		renderStart := time.Now()
		contextParams := render.BuildProcessContextParamsFromSource(kt.Ctx, source, e.CMDBService)
		logs.V(3).Infof("build process context params from source, context params: %+v, template id: %d",
			contextParams, generatePayload.TemplateRevision.Attachment.TemplateID)
		// 使用公共方法渲染模板
		var err error
		renderedContent, err = render.Template(configContent, contextParams)
		if err != nil {
			logs.Errorf("[GenerateConfig STEP]: render template failed,cost: %s template id: %d, error: %v", time.Since(renderStart),
				generatePayload.TemplateRevision.Attachment.TemplateID, err)
			return fmt.Errorf("render template failed, template id: %d, error: %w",
				generatePayload.TemplateRevision.Attachment.TemplateID, err)
		}

		logs.Infof("[GenerateConfig STEP]: render template success, cost=%s, template_id=%d", time.Since(renderStart),
			generatePayload.TemplateRevision.Attachment.TemplateID)
	}

	commonPayload.ConfigPayload.ConfigInstanceKey = generateConfigKey(
		generatePayload.ConfigTemplateID,
		generatePayload.Process.Attachment.CcProcessID,
		generatePayload.ProcessInstance.Spec.ModuleInstSeq,
	)
	commonPayload.ConfigPayload.ConfigContent = renderedContent
	commonPayload.ConfigPayload.ConfigContentSignature = tools.SHA256(renderedContent)
	if err := c.SetCommonPayload(commonPayload); err != nil {
		logs.Errorf("[GenerateConfig STEP]: set common payload failed: %d, error: %v",
			generatePayload.TemplateRevision.Attachment.TemplateID, err)

		return fmt.Errorf("[GenerateConfig STEP]: set common payload failed: %w", err)
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

	// 统一推送事件
	e.AfterCallbackNotify(c.Context(), common.CallbackNotify{
		BizID:    payload.BizID,
		BatchID:  payload.BatchID,
		Operator: payload.OperatorUser,
		CbErr:    cbErr,
	})

	return nil
}

// RegisterStepExecutor register step executor
func RegisterGenerateConfigExecutor(e *GenerateConfigExecutor) {
	istep.Register(GenerateConfigStepName, istep.StepExecutorFunc(e.GenerateConfig))
	istep.RegisterCallback(ConfigGenerateCallbackName, istep.CallbackExecutorFunc(e.Callback))
}
