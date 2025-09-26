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

package migrations

import (
	"time"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/cmd/data-service/db-migration/migrator"
)

func init() {
	// add current migration to migrator
	migrator.GetMigrator().AddMigration(&migrator.Migration{
		Version: "20250923114042",
		Name:    "20250923114042_add_task",
		Mode:    migrator.GormMode,
		Up:      mig20250923114042Up,
		Down:    mig20250923114042Down,
	})
}

// mig20250923114042Up for up migration
func mig20250923114042Up(tx *gorm.DB) error {
	// Tasks 任务表
	type Tasks struct {
		ID           uint       `gorm:"column:id;type:bigint;primaryKey;autoIncrement:true;comment:主键ID" json:"id"` // 主键ID
		TenantID     string     `gorm:"column:tenant_id;type:varchar(255);not null;index:idx_tenantID_bizID_parent_taskID,priority:1;default:default" json:"tenant_id"`
		BizID        uint       `gorm:"column:biz_id;type:bigint unsigned;not null;index:idx_tenantID_bizID_parent_taskID,priority:2;comment:业务ID" json:"biz_id"`                               // 业务ID
		ParentTaskID uint       `gorm:"column:parent_task_id;type:bigint;index:idx_tenantID_bizID_parent_taskID,priority:3;comment:父任务ID，用于任务拆分 / 子任务场景" json:"parent_task_id"`                 // 父任务ID，用于任务拆分 / 子任务场景
		TaskName     string     `gorm:"column:task_name;type:varchar(128);not null;comment:任务名称，例如配置下发、上线版本" json:"task_name"`                                                                  // 任务名称，例如配置下发、上线版本
		TaskType     string     `gorm:"column:task_type;type:varchar(64);not null;index:idx_task_type,priority:1;comment:任务类型，例如 配置任务、进程任务" json:"task_type"`                                   // 任务类型，例如 配置任务、进程任务
		Action       string     `gorm:"column:action;type:varchar(64);not null;index:idx_action,priority:1;comment:动作，如 生成、启动、停止、下发" json:"action"`                                             // 动作，如 生成、启动、停止、下发
		TargetType   string     `gorm:"column:target_type;type:varchar(64);not null;index:idx_target,priority:1;comment:目标对象类型，如 某个进程的操作" json:"target_type"`                                   // 目标对象类型，如 某个进程的操作
		TargetID     string     `gorm:"column:target_id;type:varchar(128);not null;index:idx_target,priority:2;comment:目标对象ID（如配置文件ID、进程ID）" json:"target_id"`                                  // 目标对象ID（如配置文件ID、进程ID）
		EnvType      string     `gorm:"column:env_type;type:varchar(32);not null;comment:环境类型，如 dev、test、prod" json:"env_type"`                                                                 // 环境类型，如 dev、test、prod
		Operator     string     `gorm:"column:operator;type:varchar(64);not null;comment:操作人" json:"operator"`                                                                                  // 操作人
		Status       string     `gorm:"column:status;type:varchar(32);not null;index:idx_status,priority:1;default:pending;comment:任务状态：pending、running、success、failed、canceled" json:"status"` // 任务状态：pending、running、success、failed、canceled
		Retries      int32      `gorm:"column:retries;type:int;not null;comment:已重试次数" json:"retries"`                                                                                          // 已重试次数
		MaxRetries   int32      `gorm:"column:max_retries;type:int;not null;default:3;comment:最大重试次数" json:"max_retries"`                                                                       // 最大重试次数
		Payload      string     `gorm:"column:payload;type:json;comment:任务参数，存储执行所需的上下文" json:"payload"`                                                                                        // 任务参数，存储执行所需的上下文
		TaskResult   string     `gorm:"column:task_result;type:json;comment:任务执行结果" json:"task_result"`                                                                                         // 任务执行结果
		CreatedAt    *time.Time `gorm:"column:created_at;type:timestamp;index:idx_created_at,priority:1;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`                              // 创建时间
		UpdatedAt    *time.Time `gorm:"column:updated_at;type:timestamp;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`                                                              // 更新时间
		StartedAt    *time.Time `gorm:"column:started_at;type:timestamp;comment:开始时间" json:"started_at"`                                                                                        // 开始时间
		FinishedAt   *time.Time `gorm:"column:finished_at;type:timestamp;comment:结束时间" json:"finished_at"`                                                                                      // 结束时间
	}

	if err := tx.Set("gorm:table_options", "ENGINE=InnoDB CHARSET=utf8mb4").
		AutoMigrate(&Tasks{}); err != nil {
		return err
	}

	now := time.Now()
	if result := tx.Create([]IDGenerators{
		{Resource: "tasks", MaxID: 0, UpdatedAt: now},
	}); result.Error != nil {
		return result.Error
	}

	return nil
}

// mig20250923114042Down for down migration
func mig20250923114042Down(tx *gorm.DB) error {
	// IDGenerators : ID生成器
	type IDGenerators struct {
		ID        uint      `gorm:"type:bigint(1) unsigned not null;primaryKey"`
		Resource  string    `gorm:"type:varchar(50) not null;uniqueIndex:idx_resource"`
		MaxID     uint      `gorm:"type:bigint(1) unsigned not null"`
		UpdatedAt time.Time `gorm:"type:datetime(6) not null"`
	}

	var resources = []string{
		"tasks",
	}
	if result := tx.Where("resource IN ?", resources).Delete(&IDGenerators{}); result.Error != nil {
		return result.Error
	}

	if err := tx.Migrator().DropTable("tasks"); err != nil {
		return err
	}

	return nil
}
