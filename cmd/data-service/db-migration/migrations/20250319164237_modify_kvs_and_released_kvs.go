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
		Version: "20250319164237",
		Name:    "20250319164237_modify_kvs_and_released_kvs",
		Mode:    migrator.GormMode,
		Up:      mig20250319164237Up,
		Down:    mig20250319164237Down,
	})
}

// mig20250319164237Up for up migration
func mig20250319164237Up(tx *gorm.DB) error {
	// Kv mapped from table <kvs>
	type Kv struct {
		ManagedTableID   uint32  `gorm:"column:managed_table_id;type:bigint unsigned;not null" json:"managed_table_id"`
		ExternalSourceID uint32  `gorm:"column:external_source_id;type:bigint unsigned;not null" json:"external_source_id"`
		FilterCondition  *string `gorm:"column:filter_condition;type:json" json:"filter_condition"`
		FilterFields     *string `gorm:"column:filter_fields;type:json" json:"filter_fields"`
	}

	// ReleasedKv mapped from table <released_kvs>
	type ReleasedKv struct {
		ManagedTableID   uint32  `gorm:"column:managed_table_id;type:bigint unsigned;not null" json:"managed_table_id"`
		ExternalSourceID uint32  `gorm:"column:external_source_id;type:bigint unsigned;not null" json:"external_source_id"`
		FilterCondition  *string `gorm:"column:filter_condition;type:json" json:"filter_condition"`
		FilterFields     *string `gorm:"column:filter_fields;type:json" json:"filter_fields"`
	}

	// Kv drop column
	if tx.Migrator().HasColumn(&Kv{}, "managed_table_id") {
		if err := tx.Migrator().DropColumn(&Kv{}, "managed_table_id"); err != nil {
			return err
		}
	}
	// Kv drop column
	if tx.Migrator().HasColumn(&Kv{}, "external_source_id") {
		if err := tx.Migrator().DropColumn(&Kv{}, "external_source_id"); err != nil {
			return err
		}
	}
	// Kv drop column
	if tx.Migrator().HasColumn(&Kv{}, "filter_condition") {
		if err := tx.Migrator().DropColumn(&Kv{}, "filter_condition"); err != nil {
			return err
		}
	}
	// Kv drop column
	if tx.Migrator().HasColumn(&Kv{}, "filter_fields") {
		if err := tx.Migrator().DropColumn(&Kv{}, "filter_fields"); err != nil {
			return err
		}
	}

	// ReleasedKv drop column
	if tx.Migrator().HasColumn(&ReleasedKv{}, "managed_table_id") {
		if err := tx.Migrator().DropColumn(&ReleasedKv{}, "managed_table_id"); err != nil {
			return err
		}
	}
	// ReleasedKv drop column
	if tx.Migrator().HasColumn(&ReleasedKv{}, "external_source_id") {
		if err := tx.Migrator().DropColumn(&ReleasedKv{}, "external_source_id"); err != nil {
			return err
		}
	}
	// ReleasedKv drop column
	if tx.Migrator().HasColumn(&ReleasedKv{}, "filter_condition") {
		if err := tx.Migrator().DropColumn(&ReleasedKv{}, "filter_condition"); err != nil {
			return err
		}
	}
	// Kv drop column
	if tx.Migrator().HasColumn(&ReleasedKv{}, "filter_fields") {
		if err := tx.Migrator().DropColumn(&ReleasedKv{}, "filter_fields"); err != nil {
			return err
		}
	}

	return nil
}

// mig20250319164237Down for down migration
func mig20250319164237Down(tx *gorm.DB) error {
	if err := tx.Migrator().DropTable("model_example"); err != nil {
		return err
	}

	return nil
}
