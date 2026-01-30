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
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// 定义任务类型常量
const (
	// ProcessOperateTaskType 进程操作任务类型
	ProcessOperateTaskType = "process_operate"

	// SyncCMDBGSETaskType 同步cmdb和gse任务类型
	SyncCMDBGSETaskType = "sync_cmdb_gse"

	// ProcessStateSyncType 同步 gse 进程和托管状态
	ProcessStateSyncType = "process_state_sync"
)

// 定义任务索引类型常量
const (
	// TaskIndexType 任务批次索引类型
	TaskIndexType = "task_batch"
	// BizIDTaskIndexType 业务ID索引类型
	BizIDTaskIndexType = "biz_id"
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
	return task.SetCommonPayload(&common.TaskPayload{
		ProcessPayload: &common.ProcessPayload{
			SetName:       process.Spec.SetName,
			ModuleName:    process.Spec.ModuleName,
			ServiceName:   process.Spec.ServiceName,
			Environment:   process.Spec.Environment,
			Alias:         process.Spec.Alias,
			FuncName:      process.Spec.FuncName,
			InnerIP:       process.Spec.InnerIP,
			AgentID:       process.Attachment.AgentID,
			CcProcessID:   process.Attachment.CcProcessID,
			HostInstSeq:   inst.Spec.HostInstSeq,
			ModuleInstSeq: inst.Spec.ModuleInstSeq,
			ConfigData:    process.Spec.SourceData,
			CloudID:       int(process.Attachment.CloudID),
		},
	})
}

// ConfigTaskOptions 配置生成和配置检查共用
type ConfigTaskOptions struct {
	Dao                dao.Set
	BizID              uint32
	BatchID            uint32
	ConfigTemplateID   uint32
	ConfigTemplateName string
	OperatorUser       string
	OperateType        table.ConfigOperateType
	Template           *table.Template
	TemplateRevision   *table.TemplateRevision
	Process            *table.Process
	ProcessInstance    *table.ProcessInstance
}
