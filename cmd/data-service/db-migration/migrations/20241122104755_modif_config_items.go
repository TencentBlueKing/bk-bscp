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

	"github.com/TencentBlueKing/bk-bcs/bcs-services/bcs-bscp/cmd/data-service/db-migration/migrator"
)

func init() {
	// add current migration to migrator
	migrator.GetMigrator().AddMigration(&migrator.Migration{
		Version: "20241122104755",
		Name:    "20241122104755_modif_config_items",
		Mode:    migrator.GormMode,
		Up:      mig20241122104755Up,
		Down:    mig20241122104755Down,
	})
}

// mig20241122104755Up for up migration
func mig20241122104755Up(tx *gorm.DB) error {
	// ConfigItems  : config_items
	type ConfigItems struct {
		Charset string `gorm:"column:charset;type:varchar(20);default:'';NOT NULL"`
	}

	// ReleasedConfigItems  : released_config_items
	type ReleasedConfigItems struct {
		Charset string `gorm:"column:charset;type:varchar(20);default:'';NOT NULL"`
	}

	// TemplateRevisions  : template_revisions
	type TemplateRevisions struct {
		Charset string `gorm:"column:charset;type:varchar(20);default:'';NOT NULL"`
	}

	// ReleasedAppTemplates  : released_app_templates
	type ReleasedAppTemplates struct {
		Charset string `gorm:"column:charset;type:varchar(20);default:'';NOT NULL"`
	}

	// ConfigItems add new column
	if !tx.Migrator().HasColumn(&ConfigItems{}, "charset") {
		if err := tx.Migrator().AddColumn(&ConfigItems{}, "charset"); err != nil {
			return err
		}
	}

	// ReleasedConfigItems add new column
	if !tx.Migrator().HasColumn(&ReleasedConfigItems{}, "charset") {
		if err := tx.Migrator().AddColumn(&ReleasedConfigItems{}, "charset"); err != nil {
			return err
		}
	}

	// TemplateRevisions add new column
	if !tx.Migrator().HasColumn(&TemplateRevisions{}, "charset") {
		if err := tx.Migrator().AddColumn(&TemplateRevisions{}, "charset"); err != nil {
			return err
		}
	}

	// ReleasedAppTemplates add new column
	if !tx.Migrator().HasColumn(&ReleasedAppTemplates{}, "charset") {
		if err := tx.Migrator().AddColumn(&ReleasedAppTemplates{}, "charset"); err != nil {
			return err
		}
	}

	return nil
}

// mig20241122104755Down for down migration
func mig20241122104755Down(tx *gorm.DB) error {
	// ConfigItems  : config_items
	type ConfigItems struct {
		Charset string `gorm:"column:charset;type:varchar(20);default:'';NOT NULL"`
	}

	// ReleasedConfigItems  : released_config_items
	type ReleasedConfigItems struct {
		Charset string `gorm:"column:charset;type:varchar(20);default:'';NOT NULL"`
	}

	// TemplateRevisions  : template_revisions
	type TemplateRevisions struct {
		Charset string `gorm:"column:charset;type:varchar(20);default:'';NOT NULL"`
	}

	// ReleasedAppTemplates  : released_app_templates
	type ReleasedAppTemplates struct {
		Charset string `gorm:"column:charset;type:varchar(20);default:'';NOT NULL"`
	}

	// ConfigItems drop column
	if tx.Migrator().HasColumn(&ConfigItems{}, "charset") {
		if err := tx.Migrator().DropColumn(&ConfigItems{}, "charset"); err != nil {
			return err
		}
	}

	// ReleasedConfigItems drop column
	if tx.Migrator().HasColumn(&ReleasedConfigItems{}, "charset") {
		if err := tx.Migrator().DropColumn(&ReleasedConfigItems{}, "charset"); err != nil {
			return err
		}
	}

	// TemplateRevisions drop column
	if tx.Migrator().HasColumn(&TemplateRevisions{}, "charset") {
		if err := tx.Migrator().DropColumn(&TemplateRevisions{}, "charset"); err != nil {
			return err
		}
	}

	// ReleasedAppTemplates drop column
	if tx.Migrator().HasColumn(&ReleasedAppTemplates{}, "charset") {
		if err := tx.Migrator().DropColumn(&ReleasedAppTemplates{}, "charset"); err != nil {
			return err
		}
	}

	return nil
}
