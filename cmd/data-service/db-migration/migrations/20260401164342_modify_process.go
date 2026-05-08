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
		Version: "20260401164342",
		Name:    "20260401164342_modify_process",
		Mode:    migrator.GormMode,
		Up:      mig20260401164342Up,
		Down:    mig20260401164342Down,
	})
}

// mig20260401164342Up for up migration
func mig20260401164342Up(tx *gorm.DB) error {
	// Process 进程管理主表
	type Process struct {
		OsType string `gorm:"column:os_type;type:varchar(64);not null;comment:系统类型(linux:1,win:2)" json:"os_type"` // 系统类型(linux:1,win:2)
	}

	// Process Kvs add new column
	if !tx.Migrator().HasColumn(&Process{}, "os_type") {
		if err := tx.Migrator().AddColumn(&Process{}, "os_type"); err != nil {
			return err
		}
	}

	return nil
}

// mig20260401164342Down for down migration
func mig20260401164342Down(tx *gorm.DB) error {
	// Process 进程管理主表
	type Process struct {
		OsType string `gorm:"column:os_type;type:varchar(64);not null;comment:系统类型(linux:1,win:2)" json:"os_type"` // 系统类型(linux:1,win:2)
	}

	if tx.Migrator().HasColumn(&Process{}, "os_type") {
		if err := tx.Migrator().DropColumn(&Process{}, "os_type"); err != nil {
			return err
		}
	}

	return nil
}
