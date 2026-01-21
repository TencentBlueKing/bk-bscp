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

// Package table NOTES
package table

import (
	"errors"
	"time"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/enumor"
)

// ProcessStatus 进程状态
type ProcessStatus string

// 托管状态
type ProcessManagedStatus string

const (
	// 运行中
	ProcessStatusRunning ProcessStatus = "running"
	// 部分运行
	ProcessStatusPartlyRunning ProcessStatus = "partly_running"
	// 启动中
	ProcessStatusStarting ProcessStatus = "starting"
	// 重启中
	ProcessStatusRestarting ProcessStatus = "restarting"
	// 停止中
	ProcessStatusStopping ProcessStatus = "stopping"
	// 重载中
	ProcessStatusReloading ProcessStatus = "reloading"
	// 已停止(即未运行)
	ProcessStatusStopped ProcessStatus = "stopped"
)
const (
	// 正执行托管中
	ProcessManagedStatusStarting ProcessManagedStatus = "starting"
	// 正在取消托管中
	ProcessManagedStatusStopping ProcessManagedStatus = "stopping"
	// 托管中
	ProcessManagedStatusManaged ProcessManagedStatus = "managed"
	// 未托管
	ProcessManagedStatusUnmanaged ProcessManagedStatus = "unmanaged"
	// 部分托管中
	ProcessManagedStatusPartlyManaged ProcessManagedStatus = "partly_managed"
)

// ProcessInstances defines an process_instances detail information
type ProcessInstance struct {
	ID         uint32                     `json:"id" gorm:"primaryKey"`
	Attachment *ProcessInstanceAttachment `json:"attachment" gorm:"embedded"`
	Spec       *ProcessInstanceSpec       `json:"spec" gorm:"embedded"`
	Revision   *Revision                  `json:"revision" gorm:"embedded"`
}

// TableName is the app's database table name.
func (p *ProcessInstance) TableName() Name {
	return ProcessInstancesTable
}

// ResID AuditRes interface
func (p *ProcessInstance) ResID() uint32 {
	return p.ID
}

// ResType AuditRes interface
func (p *ProcessInstance) ResType() string {
	return string(enumor.Process)
}

// ProcessInstanceSpec xxx
type ProcessInstanceSpec struct {
	HostInstSeq     uint32               `gorm:"column:host_inst_seq" json:"host_inst_seq"`         // HostInstSeq
	ModuleInstSeq   uint32               `gorm:"column:module_inst_seq" json:"module_inst_seq"`     // ModuleInstSeq
	Status          ProcessStatus        `gorm:"column:status" json:"status"`                       // 进程状态:running,stopped
	ManagedStatus   ProcessManagedStatus `gorm:"column:managed_status" json:"managed_status"`       // 托管状态:managed,unmanaged
	StatusUpdatedAt time.Time            `gorm:"column:status_updated_at" json:"status_updated_at"` // 状态更新时间
}

// ProcessInstanceAttachment xxx
type ProcessInstanceAttachment struct {
	TenantID    string `gorm:"column:tenant_id" json:"tenant_id"`         // 租户ID
	BizID       uint32 `gorm:"column:biz_id" json:"biz_id"`               // 业务ID
	ProcessID   uint32 `gorm:"column:process_id" json:"process_id"`       // 关联的process表ID
	CcProcessID uint32 `gorm:"column:cc_process_id" json:"cc_process_id"` // cc进程ID
}

// String get string value of process status
func (p ProcessStatus) String() string {
	return string(p)
}

// Validate validate process status is valid or not.
func (p ProcessStatus) Validate() error {
	switch p {
	case ProcessStatusRunning, ProcessStatusStopped, ProcessStatusPartlyRunning, ProcessStatusStarting,
		ProcessStatusRestarting, ProcessStatusStopping, ProcessStatusReloading:
		return nil
	default:
		return errors.New("invalid process status")
	}
}

// String get string value of process managed status
func (p ProcessManagedStatus) String() string {
	return string(p)
}

// Validate validate process managed status is valid or not.
func (p ProcessManagedStatus) Validate() error {
	switch p {
	case ProcessManagedStatusStarting, ProcessManagedStatusStopping, ProcessManagedStatusManaged,
		ProcessManagedStatusUnmanaged, ProcessManagedStatusPartlyManaged:
		return nil
	default:
		return errors.New("invalid process managed status")
	}
}

// GetProcessManagedStatusByOpType 根据操作类型获取进程托管状态
func GetProcessManagedStatusByOpType(
	operateType ProcessOperateType,
	originalManagedStatus ProcessManagedStatus,
) ProcessManagedStatus {
	switch operateType {
	case RegisterProcessOperate, StartProcessOperate, RestartProcessOperate, ReloadProcessOperate:
		// 如果进程已经托管，则不进行修改
		if originalManagedStatus == ProcessManagedStatusManaged {
			return ProcessManagedStatusManaged
		}
		// 托管操作/启动操作/重启操作/重载操作 修改进程托管状态为托管中
		return ProcessManagedStatusStarting
	case UnregisterProcessOperate:
		// 取消托管操作 修改进程托管状态为取消托管中
		return ProcessManagedStatusStopping
	case StopProcessOperate, KillProcessOperate:
		// 停止/杀死进程操作，如果进程已经处于取消托管中，则不进行修改
		if originalManagedStatus == ProcessManagedStatusStopping {
			return ProcessManagedStatusManaged
		}
		return ProcessManagedStatusStopping
	case UpdateRegisterProcessOperate:
		// 更新托管操作 修改进程托管状态为托管中
		return ProcessManagedStatusStarting
	default:
		return ""
	}
}

// GetProcessStatusByOpType 根据操作类型获取进程状态
func GetProcessStatusByOpType(operateType ProcessOperateType, originalStatus ProcessStatus, enableProcessRestart bool) ProcessStatus {
	switch operateType {
	case StartProcessOperate:
		return ProcessStatusStarting
	case StopProcessOperate:
		return ProcessStatusStopping
	case RestartProcessOperate:
		return ProcessStatusRestarting
	case ReloadProcessOperate:
		return ProcessStatusReloading
	case KillProcessOperate:
		return ProcessStatusStopping
	case RegisterProcessOperate, UnregisterProcessOperate:
		// 托管/取消托管操作：保留原始进程状态，不修改
		return originalStatus
	case UpdateRegisterProcessOperate:
		// 只更新托管状态
		if !enableProcessRestart {
			return originalStatus
		}
		return ProcessStatusStarting
	default:
		return originalStatus
	}
}
