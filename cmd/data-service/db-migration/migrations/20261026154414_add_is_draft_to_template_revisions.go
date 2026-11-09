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
		Version: "20261026154414",
		Name:    "20261026154414_add_is_draft_to_template_revisions",
		Mode:    migrator.GormMode,
		Up:      mig20261026154414Up,
		Down:    mig20261026154414Down,
	})
}

// mig20261026154414Up for up migration
func mig20261026154414Up(tx *gorm.DB) error {
	// 新增草稿标识，默认非草稿，存量版本全部视为正式版本
	if !tx.Migrator().HasColumn("template_revisions", "is_draft") {
		if err := tx.Exec("ALTER TABLE `template_revisions` ADD COLUMN `is_draft` tinyint(1) NOT NULL DEFAULT 0 " +
			"COMMENT '是否为草稿版本'").Error; err != nil {
			return err
		}
	}

	return nil
}

// mig20261026154414Down for down migration
func mig20261026154414Down(tx *gorm.DB) error {
	if tx.Migrator().HasColumn("template_revisions", "is_draft") {
		if err := tx.Exec("ALTER TABLE `template_revisions` DROP COLUMN `is_draft`").Error; err != nil {
			return err
		}
	}

	return nil
}
