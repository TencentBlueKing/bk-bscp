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

	// TaskBatchStatusRunning 任务状态：执行中
	TaskBatchStatusRunning TaskBatchStatus = "running"
	// TaskBatchStatusFailed 任务状态：失败
	TaskBatchStatusFailed TaskBatchStatus = "failed"
	// TaskBatchStatusSucceed 任务状态：成功
	TaskBatchStatusSucceed TaskBatchStatus = "succeed"
	// TaskBatchStatusPartlyFailed 任务状态：部分失败
	TaskBatchStatusPartlyFailed TaskBatchStatus = "partly_failed"
)

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
	SetNames     []string `json:"set_names"`        // 集群ID列表
	ModuleNames  []string `json:"module_names"`     // 模块ID列表
	ServiceNames []string `json:"service_names"`    // 服务实例ID列表
	ProcessAlias []string `json:"process_alias"`    // 进程别名列表
	CCProcessID  []uint32 `json:"cc_process_names"` // cc进程ID列表
}

// TaskData 任务数据接口
type TaskData interface {
	String() string
}

// ProcessTaskData 进程组合任务数据
type ProcessTaskData struct {
	Environment  string       `json:"environment"`
	OperateRange OperateRange `json:"operate_range"`
}

func (p *ProcessTaskData) String() string {
	// json marshal 会忽略零值字段
	b, err := json.Marshal(p)
	if err != nil {
		logs.Errorf("marshal process task data failed, err: %v", err)
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
}

// 辅助方法：设置任务数据
func (t *TaskBatchSpec) SetTaskData(data TaskData) {
	t.TaskData = data.String()
}

// 辅助方法：获取进程任务数据
func (t *TaskBatchSpec) GetProcessTaskData() (*ProcessTaskData, error) {
	var data ProcessTaskData
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
