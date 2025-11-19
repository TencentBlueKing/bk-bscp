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

package common

import (
	"fmt"

	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// Builder common builder
type Builder struct {
	dao dao.Set
}

// NewBuilder new
func NewBuilder(dao dao.Set) *Builder {
	return &Builder{
		dao: dao,
	}
}

// SetCommonProcessParam 设置
func (builder *Builder) CommonProcessFinalize(task *types.Task, bizID, processID, processInstanceID uint32) error {
	// 从db主动获取进行信息组装payload
	kit := kit.New()
	process, err := builder.dao.Process().GetByID(kit, bizID, processID)
	if err != nil {
		return err
	}
	if process == nil {
		return fmt.Errorf("no process found for biz %d", bizID)
	}

	inst, err := builder.dao.ProcessInstance().GetByID(kit, bizID, processInstanceID)
	if err != nil {
		return err
	}
	if inst == nil {
		return fmt.Errorf("no process instance found for id %d", processInstanceID)
	}
	return task.SetCommonPayload(&common.ProcessPayload{
		SetName:       process.Spec.SetName,
		ModuleName:    process.Spec.ModuleName,
		ServiceName:   process.Spec.ServiceName,
		Environment:   process.Spec.Environment,
		Alias:         process.Spec.Alias,
		InnerIP:       process.Spec.InnerIP,
		AgentID:       process.Attachment.AgentID,
		CcProcessID:   fmt.Sprintf("%d", process.Attachment.CcProcessID),
		HostInstSeq:   inst.Spec.HostInstSeq,
		ModuleInstSeq: inst.Spec.ModuleInstSeq,
		ConfigData:    process.Spec.SourceData,
		CloudID:       int(process.Attachment.CloudID),
	})
}
