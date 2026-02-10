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

package table

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/enumor"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// TaskObject task object
type TaskObject string
type TaskAction string

// TaskBatchStatus task batch status
type TaskBatchStatus string

const (
	// TaskObjectProcess 任务对象：进程
	TaskObjectProcess TaskObject = "process"
	// TaskObjectConfigFile 任务对象：配置文件
	TaskObjectConfigFile TaskObject = "config_file"

	// TaskActionStart 任务动作：启动
	TaskActionStart TaskAction = "start"
	// TaskActionStop 任务动作：停止
	TaskActionStop TaskAction = "stop"
	// TaskActionQueryStatus 任务动作：状态查询
	TaskActionQueryStatus TaskAction = "query_status"
	// TaskActionRegister 任务动作：托管
	TaskActionRegister TaskAction = "register"
	// TaskActionUnregister 任务动作：取消托管
	TaskActionUnregister TaskAction = "unregister"
	// TaskActionRestart 任务动作：重启
	TaskActionRestart TaskAction = "restart"
	// TaskActionReload 任务动作：重载
	TaskActionReload TaskAction = "reload"
	// TaskActionKill 任务动作：强制停止
	TaskActionKill TaskAction = "kill"
	// TaskActionConfigPublish 任务动作：配置下发
	TaskActionConfigPublish TaskAction = "config_publish"
	// TaskActionConfigGenerate 任务动作：配置生成
	TaskActionConfigGenerate TaskAction = "config_generate"
	// TaskActionConfigCheck 任务动作：配置检查
	TaskActionConfigCheck TaskAction = "config_check"

	// TaskBatchStatusRunning 任务状态：执行中
	TaskBatchStatusRunning TaskBatchStatus = "running"
	// TaskBatchStatusFailed 任务状态：失败
	TaskBatchStatusFailed TaskBatchStatus = "failed"
	// TaskBatchStatusSucceed 任务状态：成功
	TaskBatchStatusSucceed TaskBatchStatus = "succeed"
	// TaskBatchStatusPartlyFailed 任务状态：部分失败
	TaskBatchStatusPartlyFailed TaskBatchStatus = "partly_failed"
)

// TaskObjectChoice 任务对象查询选项
type TaskObjectChoice struct {
	ID   string
	Name string
}

// TaskActionChoice 任务动作查询选项
type TaskActionChoice struct {
	ID   string
	Name string
}

// TaskBatchStatusChoice 任务状态查询选项
type TaskBatchStatusChoice struct {
	ID   string
	Name string
}

// GetTaskObjectChoices 获取所有任务对象查询选项
func GetTaskObjectChoices() []TaskObjectChoice {
	return []TaskObjectChoice{
		{ID: string(TaskObjectConfigFile), Name: "配置文件"},
		{ID: string(TaskObjectProcess), Name: "进程"},
	}
}

// GetTaskActionChoices 获取所有任务动作查询选项
func GetTaskActionChoices() []TaskActionChoice {
	return []TaskActionChoice{
		{ID: string(TaskActionStart), Name: "启动"},
		{ID: string(TaskActionStop), Name: "停止"},
		{ID: string(TaskActionRestart), Name: "重启"},
		{ID: string(TaskActionReload), Name: "重载"},
		{ID: string(TaskActionKill), Name: "强制停止"},
		{ID: string(TaskActionRegister), Name: "托管"},
		{ID: string(TaskActionUnregister), Name: "取消托管"},
		{ID: string(TaskActionConfigPublish), Name: "配置下发"},
		{ID: string(TaskActionConfigGenerate), Name: "配置生成"},
		{ID: string(TaskActionConfigCheck), Name: "配置检查"},
	}
}

// GetTaskBatchStatusChoices 获取所有任务状态查询选项
func GetTaskBatchStatusChoices() []TaskBatchStatusChoice {
	return []TaskBatchStatusChoice{
		{ID: string(TaskBatchStatusRunning), Name: "正在执行"},
		{ID: string(TaskBatchStatusSucceed), Name: "执行成功"},
		{ID: string(TaskBatchStatusFailed), Name: "执行失败"},
		{ID: string(TaskBatchStatusPartlyFailed), Name: "部分失败"},
	}
}

// TaskBatch task batch
type TaskBatch struct {
	ID         uint32               `json:"id" gorm:"primaryKey"`
	Attachment *TaskBatchAttachment `json:"attachment" gorm:"embedded"`
	Spec       *TaskBatchSpec       `json:"spec" gorm:"embedded"`
	Revision   *Revision            `json:"revision" gorm:"embedded"`
}

// TableName is the app's database table name.
func (t *TaskBatch) TableName() string {
	return "task_batches"
}

// ResID AuditRes interface
func (t *TaskBatch) ResID() uint32 {
	return t.ID
}

// ResType AuditRes interface
func (t *TaskBatch) ResType() string {
	return string(enumor.Task)
}

func (t *TaskBatch) ValidateCreate() error {
	if t.ID > 0 {
		return errors.New("id should not be set")
	}

	if t.Spec == nil {
		return errors.New("spec not set")
	}

	if err := t.Spec.ValidateCreate(); err != nil {
		return err
	}

	if t.Attachment == nil {
		return errors.New("attachment not set")
	}

	if err := t.Attachment.Validate(); err != nil {
		return err
	}

	if t.Revision == nil {
		return errors.New("revision not set")
	}

	if err := t.Revision.ValidateCreate(); err != nil {
		return err
	}
	return nil
}

// OperateRange 操作范围
type OperateRange struct {
	SetNames     []string `json:"set_names"`      // 集群名称列表
	ModuleNames  []string `json:"module_names"`   // 模块名称列表
	ServiceNames []string `json:"service_names"`  // 服务实例名称列表
	ProcessAlias []string `json:"process_alias"`  // 进程别名列表
	CCProcessID  []uint32 `json:"cc_process_ids"` // cc进程ID列表
}

// TaskData 任务数据接口
type TaskData interface {
	String() string
}

// TaskExecutionData 任务执行数据，包含任务执行时需要的环境、操作范围等信息
type TaskExecutionData struct {
	Environment       string       `json:"environment"`
	OperateRange      OperateRange `json:"operate_range"`
	ConfigTemplateIDs []uint32     `json:"config_template_ids,omitempty"` // 配置模板ID列表（用于配置下发任务）
}

func (t *TaskExecutionData) String() string {
	// json marshal 会忽略零值字段
	b, err := json.Marshal(t)
	if err != nil {
		logs.Errorf("marshal task execution data failed, err: %v", err)
		return ""
	}
	return string(b)
}

// TaskBatchSpec xxx
type TaskBatchSpec struct {
	TaskObject TaskObject      `json:"task_object" gorm:"column:task_object"`
	TaskAction TaskAction      `json:"task_action" gorm:"column:task_action"`
	TaskData   string          `json:"task_data" gorm:"column:task_data"` // 任务数据，主要目的是方便这个表更通用
	Status     TaskBatchStatus `json:"status" gorm:"column:status"`
	StartAt    *time.Time      `json:"start_at" gorm:"column:start_at"`
	EndAt      *time.Time      `json:"end_at" gorm:"column:end_at"`

	// 任务计数字段，用于 Callback 机制更新批次状态
	TotalCount     uint32 `json:"total_count" gorm:"column:total_count"`         // 总任务数
	CompletedCount uint32 `json:"completed_count" gorm:"column:completed_count"` // 已完成任务数
	SuccessCount   uint32 `json:"success_count" gorm:"column:success_count"`     // 成功任务数
	FailedCount    uint32 `json:"failed_count" gorm:"column:failed_count"`       // 失败任务数

	ExtraData string `json:"extra_data" gorm:"column:extra_data"` // 额外扩展数据
}

// 辅助方法：设置任务数据
func (t *TaskBatchSpec) SetTaskData(data TaskData) {
	t.TaskData = data.String()
}

// GetTaskExecutionData 获取任务执行数据
func (t *TaskBatchSpec) GetTaskExecutionData() (*TaskExecutionData, error) {
	var data TaskExecutionData
	if err := json.Unmarshal([]byte(t.TaskData), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (t *TaskBatchSpec) ValidateCreate() error {
	if t.TaskObject == "" {
		return errors.New("task_object not set")
	}

	if t.TaskData == "" {
		return errors.New("task_data not set")
	}

	if t.Status == "" {
		return errors.New("status not set")
	}

	return nil
}

// TaskBatchAttachment xxx
type TaskBatchAttachment struct {
	TenantID string `gorm:"column:tenant_id" json:"tenant_id"` // 租户ID
	BizID    uint32 `gorm:"column:biz_id" json:"biz_id"`       // 业务ID
}

func (t *TaskBatchAttachment) Validate() error {
	if t.BizID == 0 {
		return errors.New("biz_id not set")
	}

	return nil
}
