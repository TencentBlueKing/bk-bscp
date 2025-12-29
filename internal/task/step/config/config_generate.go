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

// GenerateConfig xxx
func GenerateConfig(bizID, batchID, configTemplateID uint32, configTemplateName string, operateType table.ConfigOperateType,
	operatorUser string, template *table.Template, templateRevision *table.TemplateRevision, process *table.Process,
	processInstance *table.ProcessInstance, generateConfigTimeout time.Duration) *types.Step {
	logs.V(3).Infof("generate config: bizID: %d, configTemplateID: %d, processAlias: %s, moduleInstSeq: %d, operateType: %s",
		bizID, configTemplateID, process.Spec.Alias, processInstance.Spec.ModuleInstSeq, operateType)

	generate := types.NewStep(config.GenerateConfigStepName.String(), config.GenerateConfigStepName.String()).
		SetAlias("generate_config").
		SetMaxExecution(generateConfigTimeout).
		SetMaxTries(0)
	lo.Must0(generate.SetPayload(config.GenerateConfigPayload{
		BizID:              bizID,
		BatchID:            batchID,
		ConfigTemplateID:   configTemplateID,
		ConfigTemplateName: configTemplateName,
		OperateType:        operateType,
		OperatorUser:       operatorUser,
		Template:           template,
		TemplateRevision:   templateRevision,
		Process:            process,
		ProcessInstance:    processInstance,
	}))
	return generate
}
