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

// Task defines an task detail information
type Task struct {
	ID         uint32          `json:"id" gorm:"primaryKey"`
	Attachment *TaskAttachment `json:"attachment" gorm:"embedded"`
	Spec       *TaskSpec       `json:"spec" gorm:"embedded"`
	Revision   *Revision       `json:"revision" gorm:"embedded"`
}

// TableName is the app's database table name.
func (t *Task) TableName() string {
	return "task"
}

// ResID AuditRes interface
func (t *Task) ResID() uint32 {
	return t.ID
}

// ResType AuditRes interface
func (p *Task) ResType() string {
	return string(enumor.Task)
}

// TaskSpec xxx
type TaskSpec struct {
	TaskName   string    `gorm:"column:task_name" json:"task_name"`     // 任务名称，例如配置下发、上线版本
	TaskType   string    `gorm:"column:task_type" json:"task_type"`     // 任务类型，例如 配置任务、进程任务
	Action     string    `gorm:"column:action" json:"action"`           // 动作，如 生成、启动、停止、下发
	TargetType string    `gorm:"column:target_type" json:"target_type"` // 目标对象类型，如 某个进程的操作
	TargetID   string    `gorm:"column:target_id" json:"target_id"`     // 目标对象ID（如配置文件ID、进程ID）
	EnvType    string    `gorm:"column:env_type" json:"env_type"`       // 环境类型，如 dev、test、prod
	Operator   string    `gorm:"column:operator" json:"operator"`       // 操作人
	Status     string    `gorm:"column:status" json:"status"`           // 任务状态：pending、running、success、failed、canceled
	Retries    int       `gorm:"column:retries" json:"retries"`         // 已重试次数
	MaxRetries int       `gorm:"column:max_retries" json:"max_retries"` // 最大重试次数
	Payload    string    `gorm:"column:payload" json:"payload"`         // 任务参数，存储执行所需的上下文
	TaskResult string    `gorm:"column:task_result" json:"task_result"` // 任务执行结果
	StartedAt  time.Time `gorm:"column:started_at" json:"started_at"`   // 开始时间
	FinishedAt time.Time `gorm:"column:finished_at" json:"finished_at"` // 结束时间
}

// TaskAttachment xxx
type TaskAttachment struct {
	TenantID     string `gorm:"column:tenant_id" json:"tenant_id"`           // 租户ID
	BizID        uint32 `gorm:"column:biz_id" json:"biz_id"`                 // 业务ID
	ParentTaskID uint32 `gorm:"column:parent_task_id" json:"parent_task_id"` // 父任务ID，用于任务拆分 / 子任务场景
}
