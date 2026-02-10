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
	migrator.GetMigrator().AddMigration(&migrator.Migration{
		Version: "20260122100000",
		Name:    "20260122100000_add_tenant_id_to_biz_hosts",
		Mode:    migrator.GormMode,
		Up:      mig20260122100000Up,
		Down:    mig20260122100000Down,
	})
}

// mig20260122100000Up for up migration
func mig20260122100000Up(tx *gorm.DB) error {
	// 1. 添加 tenant_id 列
	if err := tx.Exec("ALTER TABLE biz_hosts ADD COLUMN tenant_id varchar(64) NOT NULL DEFAULT 'default' FIRST").Error; err != nil {
		return err
	}

	// 2. 删除原有主键
	if err := tx.Exec("ALTER TABLE biz_hosts DROP PRIMARY KEY").Error; err != nil {
		return err
	}

	// 3. 创建新的复合主键 (tenant_id, bk_biz_id, bk_host_id)
	if err := tx.Exec("ALTER TABLE biz_hosts ADD PRIMARY KEY (tenant_id, bk_biz_id, bk_host_id)").Error; err != nil {
		return err
	}

	return nil
}

// mig20260122100000Down for down migration
func mig20260122100000Down(tx *gorm.DB) error {
	// 1. 删除主键
	if err := tx.Exec("ALTER TABLE biz_hosts DROP PRIMARY KEY").Error; err != nil {
		return err
	}

	// 2. 恢复原有主键
	if err := tx.Exec("ALTER TABLE biz_hosts ADD PRIMARY KEY (bk_biz_id, bk_host_id)").Error; err != nil {
		return err
	}

	// 3. 删除 tenant_id 列
	if err := tx.Exec("ALTER TABLE biz_hosts DROP COLUMN tenant_id").Error; err != nil {
		return err
	}

	return nil
}
