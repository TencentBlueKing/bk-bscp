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
		Version: "20250923114027",
		Name:    "20250923114027_add_process_instance",
		Mode:    migrator.GormMode,
		Up:      mig20250923114027Up,
		Down:    mig20250923114027Down,
	})
}

// nolint
// mig20250923114027Up for up migration
func mig20250923114027Up(tx *gorm.DB) error {
	// ProcessInstances进程实例表
	type ProcessInstances struct {
		ID              uint      `gorm:"type:bigint(1) unsigned not null;primaryKey;autoIncrement:false"`
		TenantID        string    `gorm:"column:tenant_id;type:varchar(255);not null;index:idx_tenantID_bizID_ccProcessID_processID,priority:1;default:default" json:"tenant_id"`
		BizID           uint      `gorm:"column:biz_id;type:bigint unsigned;not null;index:idx_tenantID_bizID_ccProcessID_processID,priority:2;comment:业务ID" json:"biz_id"` // 业务ID
		CcProcessID     uint      `gorm:"column:cc_process_id;type:bigint;not null;index:idx_tenantID_bizID_ccProcessID_processID,priority:3;comment:cc进程ID" json:"cc_process_id"`
		ProcessID       uint      `gorm:"column:process_id;type:bigint;not null;index:idx_tenantID_bizID_ccProcessID_processID,priority:4;comment:关联的process表ID" json:"process_id"` // 关联的process表ID
		LocalInstID     string    `gorm:"column:local_inst_id;type:varchar(64);not null;comment:LocalInstID" json:"local_inst_id"`                                                  // LocalInstID
		InstID          string    `gorm:"column:inst_id;type:varchar(64);not null;comment:InstID" json:"inst_id"`                                                                   // InstID
		Status          string    `gorm:"column:status;type:varchar(64);not null;comment:进程状态:running,stopped" json:"status"`                                                       // 进程状态:running,stopped
		ManagedStatus   string    `gorm:"column:managed_status;type:varchar(64);not null;comment:托管状态:managed,unmanaged" json:"managed_status"`                                     // 托管状态:managed,unmanaged
		StatusUpdatedAt time.Time `gorm:"column:status_updated_at;type:timestamp;default:CURRENT_TIMESTAMP;comment:状态更新时间" json:"status_updated_at"`                                // 状态更新时间

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
		AutoMigrate(&ProcessInstances{}); err != nil {
		return err
	}

	now := time.Now()
	if result := tx.Create([]IDGenerators{
		{Resource: "process_instance", MaxID: 0, UpdatedAt: now},
	}); result.Error != nil {
		return result.Error
	}

	return nil
}

// mig20250923114027Down for down migration
func mig20250923114027Down(tx *gorm.DB) error {
	// IDGenerators : ID生成器
	type IDGenerators struct {
		ID        uint      `gorm:"type:bigint(1) unsigned not null;primaryKey"`
		Resource  string    `gorm:"type:varchar(50) not null;uniqueIndex:idx_resource"`
		MaxID     uint      `gorm:"type:bigint(1) unsigned not null"`
		UpdatedAt time.Time `gorm:"type:datetime(6) not null"`
	}

	var resources = []string{
		"process_instances",
	}
	if result := tx.Where("resource IN ?", resources).Delete(&IDGenerators{}); result.Error != nil {
		return result.Error
	}

	if err := tx.Migrator().DropTable("process_instances"); err != nil {
		return err
	}

	return nil
}
