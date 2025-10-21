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

// ProcessInstances defines an process_instances detail information
type ProcessInstance struct {
	ID         uint32                     `json:"id" gorm:"primaryKey"`
	Attachment *ProcessInstanceAttachment `json:"attachment" gorm:"embedded"`
	Spec       *ProcessInstanceSpec       `json:"spec" gorm:"embedded"`
	Revision   *Revision                  `json:"revision" gorm:"embedded"`
}

// TableName is the app's database table name.
func (p *ProcessInstance) TableName() Name {
	return ProcessesTable
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
	LocalInstID     string        `gorm:"column:local_inst_id" json:"local_inst_id"`         // LocalInstID
	InstID          string        `gorm:"column:inst_id" json:"inst_id"`                     // InstID
	Status          ProcessStatus `gorm:"column:status" json:"status"`                       // 进程状态:running,stopped
	ManagedStatus   ManagedStatus `gorm:"column:managed_status" json:"managed_status"`       // 托管状态:managed,unmanaged
	StatusUpdatedAt time.Time     `gorm:"column:status_updated_at" json:"status_updated_at"` // 状态更新时间
}

// ProcessInstanceAttachment xxx
type ProcessInstanceAttachment struct {
	TenantID    string `gorm:"column:tenant_id" json:"tenant_id"`         // 租户ID
	BizID       uint32 `gorm:"column:biz_id" json:"biz_id"`               // 业务ID
	ProcessID   uint32 `gorm:"column:process_id" json:"process_id"`       // 关联的process表ID
	CcProcessID uint32 `gorm:"column:cc_process_id" json:"cc_process_id"` // cc进程ID
}

// ProcessStatus 进程状态
type ProcessStatus string

const (
	// Running 运行中
	Running ProcessStatus = "running"
	// stopped 已停止
	Stopped ProcessStatus = "stopped"
)

// String get string value of process status
func (p ProcessStatus) String() string {
	return string(p)
}

// Validate validate process status is valid or not.
func (p ProcessStatus) Validate() error {
	switch p {
	case Running, Stopped:
		return nil
	default:
		return errors.New("invalid process status")
	}
}

// ManagedStatus 托管状态
type ManagedStatus string

const (
	// Running 运行中
	Managed ManagedStatus = "managed"
	// stopped 已停止
	Unmanaged ManagedStatus = "unmanaged"
)

// String get string value of managed status
func (p ManagedStatus) String() string {
	return string(p)
}

// Validate validate managed status is valid or not.
func (p ManagedStatus) Validate() error {
	switch p {
	case Managed, Unmanaged:
		return nil
	default:
		return errors.New("invalid managed status")
	}
}
