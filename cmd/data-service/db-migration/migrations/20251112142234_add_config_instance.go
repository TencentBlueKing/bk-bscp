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
		Version: "20251112142234",
		Name:    "20251112142234_add_config_instance",
		Mode:    migrator.GormMode,
		Up:      mig20251112142234Up,
		Down:    mig20251112142234Down,
	})
}

// nolint
// mig20251112142234Up for up migration
func mig20251112142234Up(tx *gorm.DB) error {
	// ConfigInstance 配置实例表
	type ConfigInstance struct {
		ID uint `gorm:"type:bigint(1) unsigned not null;primaryKey;autoIncrement:false"`
		// Attachment is attachment info of the resource
		BizID            uint   `gorm:"column:biz_id;type:bigint unsigned;not null;comment:业务ID"`
		ConfigTemplateID uint   `gorm:"column:config_template_id;type:bigint unsigned;not null;comment:配置模板ID"`
		ConfigVersionID  uint   `gorm:"column:config_version_id;type:bigint unsigned;comment:配置模板版本ID"`
		CcProcessID      uint   `gorm:"column:cc_process_id;type:bigint unsigned;not null;comment:cc进程ID"`
		ModuleInstSeq    uint   `gorm:"column:module_inst_seq;type:bigint unsigned;not null;comment:模块下的进程实例序列号"`
		GenerateTaskID   string `gorm:"column:task_id;type:varchar(255);not null;comment:配置生成任务ID"`
		TenantID         string `gorm:"column:tenant_id;type:varchar(255);not null;default:default;comment:租户ID"`
		Md5              string `gorm:"column:md5;type:varchar(64);not null;comment:配置内容MD5"`
		Content          string `gorm:"column:content;type:longtext;comment:文件内容"`

		// Revision is revision info of the resource
		Creator   string    `gorm:"type:varchar(64) not null" json:"creator"`
		Reviser   string    `gorm:"type:varchar(64) not null" json:"reviser"`
		CreatedAt time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
		UpdatedAt time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	}

	// IDGenerators : ID生成器
	type IDGenerators struct {
		ID        uint      `gorm:"type:bigint(1) unsigned not null;primaryKey"`
		Resource  string    `gorm:"type:varchar(50) not null;uniqueIndex:idx_resource"`
		MaxID     uint      `gorm:"type:bigint(1) unsigned not null"`
		UpdatedAt time.Time `gorm:"type:datetime(6) not null"`
	}

	if err := tx.Set("gorm:table_options", "ENGINE=InnoDB CHARSET=utf8mb4").
		AutoMigrate(&ConfigInstance{}); err != nil {
		return err
	}

	now := time.Now()
	if result := tx.Create([]IDGenerators{
		{Resource: "config_instances", MaxID: 0, UpdatedAt: now},
	}); result.Error != nil {
		return result.Error
	}

	return nil
}

// mig20251112142234Down for down migration
func mig20251112142234Down(tx *gorm.DB) error {
	// IDGenerators : ID生成器
	type IDGenerators struct {
		ID        uint      `gorm:"type:bigint(1) unsigned not null;primaryKey"`
		Resource  string    `gorm:"type:varchar(50) not null;uniqueIndex:idx_resource"`
		MaxID     uint      `gorm:"type:bigint(1) unsigned not null"`
		UpdatedAt time.Time `gorm:"type:datetime(6) not null"`
	}

	var resources = []string{
		"config_instances",
	}
	if result := tx.Where("resource IN ?", resources).Delete(&IDGenerators{}); result.Error != nil {
		return result.Error
	}

	if err := tx.Migrator().DropTable("config_instances"); err != nil {
		return err
	}

	return nil
}
