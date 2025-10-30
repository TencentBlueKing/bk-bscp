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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/enumor"
)

// ProcessOperateType 操作类型
type ProcessOperateType string

const (
	// RegisterOperate 托管操作
	RegisterProcessOperate ProcessOperateType = "register"
	// UnregisterOperate 取消托管操作
	UnregisterProcessOperate ProcessOperateType = "unregister"
	// StartOperate 启动操作
	StartProcessOperate ProcessOperateType = "start"
	// StopOperate 停止操作
	StopProcessOperate ProcessOperateType = "stop"
	// RestartOperate 重启操作
	RestartProcessOperate ProcessOperateType = "restart"
	// ReloadOperate 重载操作
	ReloadProcessOperate ProcessOperateType = "reload"
	// KillOperate 强制停止操作
	KillProcessOperate ProcessOperateType = "kill"
)

// Process defines an Process detail information
type Process struct {
	ID         uint32             `json:"id" gorm:"primaryKey"`
	Attachment *ProcessAttachment `json:"attachment" gorm:"embedded"`
	Spec       *ProcessSpec       `json:"spec" gorm:"embedded"`
	Revision   *Revision          `json:"revision" gorm:"embedded"`
}

// TableName is the app's database table name.
func (p *Process) TableName() Name {
	return ProcessesTable
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
	SetName         string       `gorm:"column:set_name" json:"set_name"`                     // 集群
	ModuleName      string       `gorm:"column:module_name" json:"module_name"`               // 模块
	ServiceName     string       `gorm:"column:service_name" json:"service_name"`             // 服务实例名称
	Environment     string       `gorm:"column:environment" json:"environment"`               // 环境类型(production/staging等)
	Alias           string       `gorm:"column:alias" json:"alias"`                           // 进程别名
	InnerIP         string       `gorm:"column:inner_ip" json:"inner_ip"`                     // 内网IP
	CcSyncStatus    CCSyncStatus `gorm:"column:cc_sync_status" json:"cc_sync_status"`         // cc同步状态:synced,deleted,updated
	CcSyncUpdatedAt time.Time    `gorm:"column:cc_sync_updated_at" json:"cc_sync_updated_at"` // cc同步更新时间
	SourceData      string       `gorm:"column:source_data" json:"source_data"`               // 本次同步的数据
	PrevData        string       `gorm:"column:prev_data" json:"prev_data"`                   // 上一次同步的数据
	ProcNum         uint         `gorm:"column:proc_num" json:"proc_num"`                     // 进程数量
}

func (p ProcessInfo) Value() (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal process info failed: %w", err)
	}
	str := string(b)
	if str == "" {
		str = "{}"
	}

	return str, nil
}

// Scan 实现 sql.Scanner 接口 —— 从数据库读取 JSON 并反序列化为结构体
func (p *ProcessInfo) Scan(value any) error {
	if value == nil {
		*p = ProcessInfo{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("ProcessInfo should be a []byte, got %T", value)
	}

	if err := json.Unmarshal(bytes, p); err != nil {
		return fmt.Errorf("unmarshal ProcessInfo failed: %w", err)
	}
	return nil
}

// ProcessInfo xxx
type ProcessInfo struct {
	BkStartParamRegex string `json:"bk_start_param_regex"` // 进程启动参数
	WorkPath          string `json:"work_path"`            // 工作路径
	PidFile           string `json:"pid_file"`             // PID文件路径
	User              string `json:"user"`                 // 启动用户
	ReloadCmd         string `json:"reload_cmd"`           // 重载命令
	RestartCmd        string `json:"restart_cmd"`          // 重启命令
	StartCmd          string `json:"start_cmd"`            // 启动命令
	StopCmd           string `json:"stop_cmd"`             // 停止命令
	FaceStopCmd       string `json:"face_stop_cmd"`        // 强制停止命令
	Timeout           int    `json:"timeout"`              // 操作超时时长
}

// ProcessAttachment xxx
type ProcessAttachment struct {
	TenantID          string `gorm:"column:tenant_id" json:"tenant_id"`                     // 租户ID
	BizID             uint32 `gorm:"column:biz_id" json:"biz_id"`                           // 业务ID
	CcProcessID       uint32 `gorm:"column:cc_process_id" json:"cc_process_id"`             // cc进程ID
	SetID             uint32 `gorm:"column:set_id" json:"set_id"`                           // 集群ID
	ModuleID          uint32 `gorm:"column:module_id" json:"module_id"`                     // 模块ID
	ServiceInstanceID uint32 `gorm:"column:service_instance_id" json:"service_instance_id"` // 服务实例
	HostID            uint32 `gorm:"column:host_id" json:"host_id"`                         // 主机ID
	CloudID           uint32 `gorm:"column:cloud_id" json:"cloud_id"`                       // 管控区域
	AgentID           string `gorm:"column:agent_id" json:"agent_id"`
}

// CCSyncStatus cc同步状态
type CCSyncStatus string

const (
	// Sync 同步中
	Sync CCSyncStatus = "sync"
	// Synced 已同步
	Synced CCSyncStatus = "synced"
	// Deleted 已删除
	Deleted CCSyncStatus = "deleted"
	// Updated 已修改
	Updated CCSyncStatus = "updated"
)

// String get string value of cc sync status
func (p CCSyncStatus) String() string {
	return string(p)
}

// Validate validate cc sync status is valid or not.
func (p CCSyncStatus) Validate() error {
	switch p {
	case Synced, Deleted, Updated:
		return nil
	default:
		return errors.New("invalid cc sync status")
	}
}
