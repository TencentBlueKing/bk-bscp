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

	"github.com/TencentBlueKing/bk-bscp/internal/task/builder/common"
	executorCommon "github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	configExecutor "github.com/TencentBlueKing/bk-bscp/internal/task/executor/config"
	configStep "github.com/TencentBlueKing/bk-bscp/internal/task/step/config"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

// checkConfigTask creates a config check task builder.
func NewCheckConfigTask(opts common.ConfigTaskOptions) types.TaskBuilder {
	return &checkConfigTask{
		Builder:            common.NewBuilder(opts.Dao),
		bizID:              opts.BizID,
		batchID:            opts.BatchID,
		configTemplateID:   opts.ConfigTemplateID,
		configTemplateName: opts.ConfigTemplateName,
		operateType:        opts.OperateType,
		operatorUser:       opts.OperatorUser,
		template:           opts.Template,
		templateRevision:   opts.TemplateRevision,
		process:            opts.Process,
		processInstance:    opts.ProcessInstance,
	}
}

type checkConfigTask struct {
	*common.Builder
	bizID              uint32
	batchID            uint32
	configTemplateID   uint32
	configTemplateName string
	operateType        table.ConfigOperateType
	operatorUser       string
	template           *table.Template
	templateRevision   *table.TemplateRevision
	process            *table.Process
	processInstance    *table.ProcessInstance
}

// FinalizeTask implements [types.TaskBuilder].
func (c *checkConfigTask) FinalizeTask(t *types.Task) error {
	payload := executorCommon.BuildConfigTaskPayload(
		c.process,
		c.processInstance,
		c.templateRevision,
		c.configTemplateID,
		c.configTemplateName,
	)

	if err := t.SetCommonPayload(payload); err != nil {
		return err
	}

	// 设置回调，用于任务完成后更新 TaskBatch 状态
	t.SetCallback(string(configExecutor.CheckConfigCallbackName))

	return nil
}

// Steps implements [types.TaskBuilder].
func (c *checkConfigTask) Steps() ([]*types.Step, error) {
	return []*types.Step{
		// Step 1：计算 & 比对 md5
		configStep.CheckConfigMD5(
			c.bizID,
			c.batchID,
			c.configTemplateID,
			c.configTemplateName,
			c.operateType,
			c.operatorUser,
			c.template,
			c.templateRevision,
			c.process,
			c.processInstance,
		),
		// Step 2：仅在 md5 不一致时获取文件内容
		configStep.FetchConfigContent(c.bizID,
			c.batchID,
			c.configTemplateID,
			c.configTemplateName,
			c.operateType,
			c.operatorUser,
			c.template,
			c.templateRevision,
			c.process,
			c.processInstance),
	}, nil
}

// TaskInfo implements [types.TaskBuilder].
func (c *checkConfigTask) TaskInfo() types.TaskInfo {
	// 使用配置实例 key 作为任务名称的一部分
	taskName := fmt.Sprintf("%s_%s_%s_%d", c.operateType, c.configTemplateName,
		c.process.Spec.Alias, c.processInstance.Spec.ModuleInstSeq)
	return types.TaskInfo{
		TaskName:      taskName,
		TaskType:      string(table.TaskActionConfigCheck),
		TaskIndexType: common.TaskIndexType,
		TaskIndex:     fmt.Sprintf("%d", c.batchID),
		Creator:       c.operatorUser,
	}
}
