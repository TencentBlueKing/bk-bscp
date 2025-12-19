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

	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/task/builder/common"
	executorCommon "github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	configExecutor "github.com/TencentBlueKing/bk-bscp/internal/task/executor/config"
	"github.com/TencentBlueKing/bk-bscp/internal/task/step/config"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

// GenerateConfigTask task generate config
type GenerateConfigTask struct {
	*common.Builder
	bizID   uint32
	batchID uint32

	// 任务类型
	operateType table.ConfigOperateType
	// 操作人
	operatorUser string

	// 预定义渲染模版可能需要的字段
	configTemplateID  uint32
	configTemplate    *table.ConfigTemplate
	template          *table.Template
	templateRevision  *table.TemplateRevision
	processInstanceID uint32
	processInstance   *table.ProcessInstance
	ccProcessID       uint32
	process           *table.Process

	configTemplateName string
	processAlias       string
	moduleInstSeq      uint32
}

func NewConfigGenerateTask(
	dao dao.Set,
	bizID uint32,
	batchID uint32,
	operateType table.ConfigOperateType,
	operatorUser string,
	// 预定义渲染模版可能需要的字段
	configTemplateID uint32,
	configTemplate *table.ConfigTemplate,
	template *table.Template,
	templateRevision *table.TemplateRevision,
	processInstanceID uint32,
	processInstance *table.ProcessInstance,
	ccProcessID uint32,
	process *table.Process,
	configTemplateName string,
	processAlias string,
	moduleInstSeq uint32,
) types.TaskBuilder {
	return &GenerateConfigTask{
		Builder:            common.NewBuilder(dao),
		bizID:              bizID,
		batchID:            batchID,
		operateType:        operateType,
		operatorUser:       operatorUser,
		configTemplateID:   configTemplateID,
		configTemplate:     configTemplate,
		template:           template,
		templateRevision:   templateRevision,
		processInstanceID:  processInstanceID,
		processInstance:    processInstance,
		ccProcessID:        ccProcessID,
		process:            process,
		configTemplateName: configTemplateName,
		processAlias:       processAlias,
		moduleInstSeq:      moduleInstSeq,
	}
}

// FinalizeTask implements types.TaskBuilder.
func (t *GenerateConfigTask) FinalizeTask(task *types.Task) error {
	key := fmt.Sprintf("%d-%d-%d", t.configTemplateID, t.ccProcessID, t.moduleInstSeq)
	// 设置 CommonPayload
	if err := task.SetCommonPayload(&executorCommon.TaskPayload{
		ProcessPayload: &executorCommon.ProcessPayload{
			SetName:       t.process.Spec.SetName,
			ModuleName:    t.process.Spec.ModuleName,
			ServiceName:   t.process.Spec.ServiceName,
			Environment:   t.process.Spec.Environment,
			Alias:         t.process.Spec.Alias,
			FuncName:      t.process.Spec.FuncName,
			InnerIP:       t.process.Spec.InnerIP,
			AgentID:       t.process.Attachment.AgentID,
			CcProcessID:   t.process.Attachment.CcProcessID,
			HostInstSeq:   t.processInstance.Spec.HostInstSeq,
			ModuleInstSeq: t.processInstance.Spec.ModuleInstSeq,
			ConfigData:    t.process.Spec.SourceData,
			CloudID:       int(t.process.Attachment.CloudID),
		},
		ConfigPayload: &executorCommon.ConfigPayload{
			ConfigTemplateID:        t.configTemplateID,
			ConfigTemplateVersionID: t.templateRevision.ID,
			ConfigTemplateName:      t.configTemplateName,
			ConfigFileName:          t.templateRevision.Spec.Name,
			ConfigFilePath:          t.templateRevision.Spec.Path,
			ConfigFileOwner:         t.templateRevision.Spec.Permission.User,
			ConfigFileGroup:         t.templateRevision.Spec.Permission.UserGroup,
			ConfigFilePermission:    t.templateRevision.Spec.Permission.Privilege,
			ConfigInstanceKey:       key,
			ConfigContent:           "",
		},
	}); err != nil {
		return err
	}

	// 设置回调，用于任务完成后更新 TaskBatch 状态
	task.SetCallback(string(configExecutor.ConfigGenerateCallbackName))

	return nil
}

func (t *GenerateConfigTask) Steps() ([]*types.Step, error) {
	return []*types.Step{
		config.GenerateConfig(
			t.bizID,
			t.batchID,
			t.operateType,
			t.operatorUser,
			t.configTemplateID,
			t.configTemplate,
			t.template,
			t.templateRevision,
			t.processInstanceID,
			t.processInstance,
			t.ccProcessID,
			t.process,
			t.configTemplateName,
			t.processAlias,
			t.moduleInstSeq,
		),
	}, nil
}

func (t *GenerateConfigTask) TaskInfo() types.TaskInfo {
	taskName := fmt.Sprintf("%s_%s_%s_%d", t.operateType, t.configTemplateName, t.processAlias, t.moduleInstSeq)
	return types.TaskInfo{
		TaskName:      taskName,
		TaskType:      common.ConfigGenerateTaskType,
		TaskIndexType: common.TaskIndexType,
		TaskIndex:     fmt.Sprintf("%d", t.batchID),
		Creator:       t.operatorUser,
	}
}
