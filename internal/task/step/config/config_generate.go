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
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"github.com/samber/lo"

	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/config"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	// MaxExecutionTime 最大执行时间
	MaxExecutionTime = 10 * time.Second
)

func GenerateConfig(
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
) *types.Step {
	logs.V(3).Infof("generate config: bizID: %d, configTemplateID: %d, processAlias: %s, moduleInstSeq: %d, operateType: %s",
		bizID, configTemplateID, processAlias, moduleInstSeq, operateType)

	generate := types.NewStep(config.GenerateConfigStepName.String(), config.GenerateConfigStepName.String()).
		SetAlias("generate_config").
		SetMaxExecution(MaxExecutionTime).
		SetMaxTries(0)
	lo.Must0(generate.SetPayload(config.GenerateConfigPayload{
		BizID:              bizID,
		BatchID:            batchID,
		OperateType:        operateType,
		OperatorUser:       operatorUser,
		ConfigTemplateID:   configTemplateID,
		ConfigTemplate:     configTemplate,
		Template:           template,
		TemplateRevision:   templateRevision,
		ProcessInstanceID:  processInstanceID,
		ProcessInstance:    processInstance,
		CcProcessID:        ccProcessID,
		Process:            process,
		ConfigTemplateName: configTemplateName,
		ProcessAlias:       processAlias,
		ModuleInstSeq:      moduleInstSeq,
	}))
	return generate
}
