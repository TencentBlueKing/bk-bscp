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
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/internal/task/builder/common"
	executorCommon "github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	configExecutor "github.com/TencentBlueKing/bk-bscp/internal/task/executor/config"
	"github.com/TencentBlueKing/bk-bscp/internal/task/step/config"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

const defaultGenerateConfigTimeout = 2 * time.Minute

// GenerateConfigTask task generate config
type GenerateConfigTask struct {
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
	gseConf            cc.GSE
}

// NewConfigGenerateTask xxx
func NewConfigGenerateTask(opts common.ConfigTaskOptions) types.TaskBuilder {
	return &GenerateConfigTask{
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
		gseConf:            cc.G().GSE,
	}
}

// FinalizeTask implements types.TaskBuilder.
func (t *GenerateConfigTask) FinalizeTask(task *types.Task) error {
	payload := executorCommon.BuildConfigTaskPayload(
		t.process,
		t.processInstance,
		t.templateRevision,
		t.configTemplateID,
		t.configTemplateName,
	)
	if err := task.SetCommonPayload(payload); err != nil {
		return err
	}

	// 设置回调，用于任务完成后更新 TaskBatch 状态
	task.SetCallback(string(configExecutor.ConfigGenerateCallbackName))

	return nil
}

func (t *GenerateConfigTask) Steps() ([]*types.Step, error) {
	// 生成配置超时时间处理
	if t.gseConf.GenerateConfigTimeout == 0 {
		t.gseConf.GenerateConfigTimeout = defaultGenerateConfigTimeout
	}
	return []*types.Step{
		config.GenerateConfig(
			t.bizID,
			t.batchID,
			t.configTemplateID,
			t.configTemplateName,
			t.operateType,
			t.operatorUser,
			t.template,
			t.templateRevision,
			t.process,
			t.processInstance,
			t.gseConf.GenerateConfigTimeout,
		),
	}, nil
}

func (t *GenerateConfigTask) TaskInfo() types.TaskInfo {
	taskName := fmt.Sprintf("%s_%s_%s_%d", t.operateType, t.configTemplateName,
		t.process.Spec.Alias, t.processInstance.Spec.ModuleInstSeq)
	return types.TaskInfo{
		TaskName:      taskName,
		TaskType:      common.ConfigGenerateTaskType,
		TaskIndexType: common.TaskIndexType,
		TaskIndex:     fmt.Sprintf("%d", t.batchID),
		Creator:       t.operatorUser,
	}
}
