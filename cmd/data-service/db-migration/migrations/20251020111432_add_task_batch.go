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
		Version: "20251020111432",
		Name:    "20251020111432_add_task_batch",
		Mode:    migrator.GormMode,
		Up:      mig20251020111432Up,
		Down:    mig20251020111432Down,
	})
}

// mig20251020111432Up for up migration
// nolint
func mig20251020111432Up(tx *gorm.DB) error {
	// TaskBatch : 任务主表
	type TaskBatch struct {
		ID         uint       `gorm:"type:bigint(1) unsigned not null;primaryKey;autoIncrement:false"`
		TenantID   string     `gorm:"column:tenant_id;type:varchar(255);not null;index:idx_tenantID_bizID_ccProcessID,priority:1;default:default" json:"tenant_id"`
		BizID      uint       `gorm:"column:biz_id;type:bigint unsigned;not null;index:idx_tenantID_bizID_ccProcessID,priority:2;comment:业务ID" json:"biz_id"` // 业务ID
		TaskObject string     `gorm:"type:varchar(250) not null;comment:任务对象" json:"task_object"`                                                             // 任务对象
		TaskAction string     `gorm:"type:varchar(250) not null;comment:任务动作" json:"task_action"`                                                             // 任务动作
		TaskData   string     `gorm:"type:longtext not null;comment:任务数据" json:"task_data"`                                                                   // 任务数据
		Status     string     `gorm:"type:varchar(250) not null;comment:任务状态" json:"status"`                                                                  // 任务状态
		StartAt    *time.Time `gorm:"type:timestamp not null;comment:任务开始时间" json:"start_at"`                                                                 // 任务开始时间
		EndAt      *time.Time `gorm:"type:timestamp null;comment:任务结束时间" json:"end_at"`                                                                       // 任务结束时间

		// Revision is revision info of the resource
		Creator   string    `gorm:"type:varchar(64) not null" json:"creator"`
		Reviser   string    `gorm:"type:varchar(64) not null" json:"reviser"`
		CreatedAt time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
		UpdatedAt time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	}

	if err := tx.Set("gorm:table_options", "ENGINE=InnoDB CHARSET=utf8mb4").
		AutoMigrate(&TaskBatch{}); err != nil {
		return err
	}

	now := time.Now()
	if result := tx.Create([]IDGenerators{
		{Resource: "task_batch", MaxID: 0, UpdatedAt: now},
	}); result.Error != nil {
		return result.Error
	}

	return nil
}

// mig20251020111432Down for down migration
func mig20251020111432Down(tx *gorm.DB) error {
	var resources = []string{
		"task_batch",
	}
	if result := tx.Where("resource IN ?", resources).Delete(&IDGenerators{}); result.Error != nil {
		return result.Error
	}

	if err := tx.Migrator().DropTable("task_batch"); err != nil {
		return err
	}

	return nil
}
