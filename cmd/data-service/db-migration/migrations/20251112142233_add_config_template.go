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
		Version: "20251112142233",
		Name:    "20251112142233_add_config_template",
		Mode:    migrator.GormMode,
		Up:      mig20251112142233Up,
		Down:    mig20251112142233Down,
	})
}

// nolint
// mig20251112142233Up for up migration
func mig20251112142233Up(tx *gorm.DB) error {
	// ConfigTemplate 配置模板表
	type ConfigTemplate struct {
		ID uint `gorm:"type:bigint(1) unsigned not null;primaryKey;autoIncrement:false"`

		// Spec is specifics of the resource defined with user
		Name string `gorm:"type:varchar(255) not null;uniqueIndex:idx_bizID_name,priority:2"`

		// Attachment is attachment info of the resource
		BizID                uint   `gorm:"column:biz_id;type:bigint unsigned;not null;comment:业务ID"`
		TemplateID           uint   `gorm:"column:template_id;type:bigint unsigned;not null;comment:关联的BSCP templates表的ID"`
		CcTemplateProcessIDs string `gorm:"column:cc_template_process_ids;type:json not null;comment:关联的cc服务模版下的模板进程ID列表"`
		CcProcessIDs         string `gorm:"column:cc_process_ids;type:json not null;comment:关联的cc中未通过服务模板创建的进程实例ID列表"`
		TenantID             string `gorm:"column:tenant_id;type:varchar(255);not null;default:default"`

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
		AutoMigrate(&ConfigTemplate{}); err != nil {
		return err
	}

	now := time.Now()
	if result := tx.Create([]IDGenerators{
		{Resource: "config_templates", MaxID: 0, UpdatedAt: now},
	}); result.Error != nil {
		return result.Error
	}

	return nil
}

// mig20251112142233Down for down migration
func mig20251112142233Down(tx *gorm.DB) error {
	// IDGenerators : ID生成器
	type IDGenerators struct {
		ID        uint      `gorm:"type:bigint(1) unsigned not null;primaryKey"`
		Resource  string    `gorm:"type:varchar(50) not null;uniqueIndex:idx_resource"`
		MaxID     uint      `gorm:"type:bigint(1) unsigned not null"`
		UpdatedAt time.Time `gorm:"type:datetime(6) not null"`
	}

	var resources = []string{
		"config_templates",
	}
	if result := tx.Where("resource IN ?", resources).Delete(&IDGenerators{}); result.Error != nil {
		return result.Error
	}

	if err := tx.Migrator().DropTable("config_templates"); err != nil {
		return err
	}

	return nil
}
