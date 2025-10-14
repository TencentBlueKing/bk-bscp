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
	"time"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/enumor"
)

// Process defines an Process detail information
type Process struct {
	ID         uint32             `json:"id" gorm:"primaryKey"`
	Attachment *ProcessAttachment `json:"attachment" gorm:"embedded"`
	Spec       *ProcessSpec       `json:"spec" gorm:"embedded"`
	Revision   *Revision          `json:"revision" gorm:"embedded"`
}

// TableName is the app's database table name.
func (p *Process) TableName() string {
	return "process"
}

// ResID AuditRes interface
func (p *Process) ResID() uint32 {
	return p.ID
}

// ResType AuditRes interface
func (p *Process) ResType() string {
	return string(enumor.Process)
}

// ProcessSpec xxx
type ProcessSpec struct {
	SetName         string    `gorm:"column:set_name" json:"set_name"`                     // 集群
	ModuleName      string    `gorm:"column:module_name" json:"module_name"`               // 模块
	ServiceName     string    `gorm:"column:service_name" json:"service_name"`             // 服务实例名称
	Environment     string    `gorm:"column:environment" json:"environment"`               // 环境类型(production/staging等)
	Alias           string    `gorm:"column:alias" json:"alias"`                           // 进程别名
	InnerIP         string    `gorm:"column:inner_ip" json:"inner_ip"`                     // 内网IP
	CcSyncStatus    string    `gorm:"column:cc_sync_statu" json:"cc_sync_status"`          // cc同步状态:synced,deleted,updated
	CcSyncUpdatedAt time.Time `gorm:"column:cc_sync_updated_at" json:"cc_sync_updated_at"` // cc同步更新时间
	SourceData      string    `gorm:"column:source_data" json:"source_data"`               // 源数据，用于和CC对比
}

// ProcessAttachment xxx
type ProcessAttachment struct {
	TenantID    string `gorm:"column:tenant_id" json:"tenant_id"`         // 租户ID
	BizID       uint32 `gorm:"column:biz_id" json:"biz_id"`               // 业务ID
	CcProcessID uint32 `gorm:"column:cc_process_id" json:"cc_process_id"` // cc进程ID
}
