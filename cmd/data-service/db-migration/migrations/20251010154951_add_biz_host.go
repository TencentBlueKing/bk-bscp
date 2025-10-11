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
		Version: "20251010154951",
		Name:    "20251010154951_add_biz_host",
		Mode:    migrator.GormMode,
		Up:      mig20251010154951Up,
		Down:    mig20251010154951Down,
	})
}

// mig20251010154951Up for up migration
func mig20251010154951Up(tx *gorm.DB) error {
	// BizHost : 业务主机关系表
	type BizHost struct {
		BizID         int       `gorm:"type:bigint(1) unsigned not null;column:bk_biz_id;primaryKey;autoIncrement:false"`
		HostID        int       `gorm:"type:bigint(1) unsigned not null;column:bk_host_id;primaryKey;autoIncrement:false"`
		AgentID       string    `gorm:"type:varchar(256);column:bk_agent_id"`
		BKHostInnerIP string    `gorm:"type:varchar(256);column:bk_host_innerip"`
		LastUpdated   time.Time `gorm:"type:datetime(6) not null;column:last_updated;autoUpdateTime"`
	}

	if err := tx.Set("gorm:table_options", "ENGINE=InnoDB CHARSET=utf8mb4").
		AutoMigrate(&BizHost{}); err != nil {
		return err
	}

	return nil
}

// mig20251010154951Down for down migration
func mig20251010154951Down(tx *gorm.DB) error {
	if err := tx.Migrator().DropTable("biz_hosts"); err != nil {
		return err
	}

	return nil
}
