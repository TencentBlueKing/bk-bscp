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
	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/cmd/data-service/db-migration/migrator"
)

func init() {
	// add current migration to migrator
	migrator.GetMigrator().AddMigration(&migrator.Migration{
		Version: "20260408152615",
		Name:    "20260408152615_modify_process",
		Mode:    migrator.GormMode,
		Up:      mig20260408152615Up,
		Down:    mig20260408152615Down,
	})
}

// mig20260408152615Up for up migration
func mig20260408152615Up(tx *gorm.DB) error {
	// Process 进程管理主表
	type Process struct {
		AgentStatus string `gorm:"column:agent_status;type:varchar(64);default:NULL;comment:agent状态(normal、abnormal)" json:"agent_status"`
	}

	// Process Kvs add new column
	if !tx.Migrator().HasColumn(&Process{}, "agent_status") {
		if err := tx.Migrator().AddColumn(&Process{}, "agent_status"); err != nil {
			return err
		}
	}

	return nil
}

// mig20260408152615Down for down migration
func mig20260408152615Down(tx *gorm.DB) error {
	// Process 进程管理主表
	type Process struct {
		AgentStatus string `gorm:"column:agent_status;type:varchar(64);default:NULL;comment:agent状态(normal、abnormal)" json:"agent_status"`
	}

	if tx.Migrator().HasColumn(&Process{}, "agent_status") {
		if err := tx.Migrator().DropColumn(&Process{}, "agent_status"); err != nil {
			return err
		}
	}

	return nil
}
