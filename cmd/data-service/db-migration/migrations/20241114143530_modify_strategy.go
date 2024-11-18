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
		Version: "20241114143530",
		Name:    "20241114143530_modify_strategy",
		Mode:    migrator.GormMode,
		Up:      mig20241114143530Up,
		Down:    mig20241114143530Down,
	})
}

// mig20241114143530Up for up migration
func mig20241114143530Up(tx *gorm.DB) error {
	// Strategies  : strategies
	type Strategies struct {
		RejectReason string `gorm:"type:longtext"`
	}
	// Strategies add new column
	if tx.Migrator().HasColumn(&Strategies{}, "reject_reason") {
		if err := tx.Migrator().AlterColumn(&Strategies{}, "reject_reason"); err != nil {
			return err
		}
	}

	return nil
}

// mig20241114143530Down for down migration
func mig20241114143530Down(tx *gorm.DB) error {
	// Strategies  : strategies
	type Strategies struct {
		RejectReason string `gorm:"column:reject_reason;type:varchar(256);default:NULL"`
	}
	// Strategies add new column
	if tx.Migrator().HasColumn(&Strategies{}, "reject_reason") {
		if err := tx.Migrator().AlterColumn(&Strategies{}, "reject_reason"); err != nil {
			return err
		}
	}

	return nil
}
