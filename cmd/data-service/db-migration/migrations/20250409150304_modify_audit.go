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
		Version: "20250409150304",
		Name:    "20250409150304_modify_audit",
		Mode:    migrator.GormMode,
		Up:      mig20250409150304Up,
		Down:    mig20250409150304Down,
	})
}

// mig20250409150304Up for up migration
func mig20250409150304Up(tx *gorm.DB) error {
	// Audits: audits
	type Audits struct {
		ResInstance string `gorm:"type:text"`
	}
	// Strategies add new column
	if tx.Migrator().HasColumn(&Audits{}, "res_instance") {
		if err := tx.Migrator().AlterColumn(&Audits{}, "res_instance"); err != nil {
			return err
		}
	}

	return nil
}

// mig20250409150304Down for down migration
func mig20250409150304Down(tx *gorm.DB) error {
	// Audits: audits
	type Audits struct {
		ResInstance string `gorm:"type:text"`
	}
	// Strategies add new column
	if tx.Migrator().HasColumn(&Audits{}, "res_instance") {
		if err := tx.Migrator().AlterColumn(&Audits{}, "res_instance"); err != nil {
			return err
		}
	}

	return nil
}
