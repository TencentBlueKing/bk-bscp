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
		Version: "20251011154255",
		Name:    "20251011154255_modify_template",
		Mode:    migrator.GormMode,
		Up:      mig20251011154255Up,
		Down:    mig20251011154255Down,
	})
}

// mig20251011154255Up for up migration
func mig20251011154255Up(tx *gorm.DB) error {
	// Templates : 配置模版
	type Templates struct {
		TemplateName string `gorm:"column:template_name;type:varchar(255);not null" json:"template_name"`
	}

	// Templates add new column
	if !tx.Migrator().HasColumn(&Templates{}, "template_name") {
		if err := tx.Migrator().AddColumn(&Templates{}, "template_name"); err != nil {
			return err
		}
	}

	return nil
}

// mig20251011154255Down for down migration
func mig20251011154255Down(tx *gorm.DB) error {
	// Templates : 配置模版
	type Templates struct {
		TemplateName string `gorm:"column:template_name;type:varchar(255);not null" json:"template_name"`
	}

	// Templates drop column
	if tx.Migrator().HasColumn(&Templates{}, "template_name") {
		if err := tx.Migrator().DropColumn(&Templates{}, "template_name"); err != nil {
			return err
		}
	}

	return nil
}
