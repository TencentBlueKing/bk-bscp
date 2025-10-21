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
		Version: "20250923114014",
		Name:    "20250923114014_add_process",
		Mode:    migrator.GormMode,
		Up:      mig20250923114014Up,
		Down:    mig20250923114014Down,
	})
}

// nolint
// mig20250923114014Up for up migration
func mig20250923114014Up(tx *gorm.DB) error {
	// Process 进程管理主表
	type Process struct {
		ID                uint      `gorm:"type:bigint(1) unsigned not null;primaryKey;autoIncrement:false"`
		TenantID          string    `gorm:"column:tenant_id;type:varchar(255);not null;index:idx_tenantID_bizID_ccProcessID,priority:1;default:default" json:"tenant_id"`
		BizID             uint      `gorm:"column:biz_id;type:bigint unsigned;not null;index:idx_tenantID_bizID_ccProcessID,priority:2;comment:业务ID" json:"biz_id"`        // 业务ID
		CcProcessID       uint      `gorm:"column:cc_process_id;type:bigint;not null;index:idx_tenantID_bizID_ccProcessID,priority:3;comment:cc进程ID" json:"cc_process_id"` // cc进程ID
		SetID             uint      `gorm:"column:set_id;type:bigint;not null;comment:集群ID"`                                                                               // 集群ID
		SetName           string    `gorm:"column:set_name;type:varchar(64);not null;comment:集群" json:"set_name"`                                                          // 集群
		ModuleID          uint      `gorm:"column:module_id;type:bigint;not null;comment:模块ID"`                                                                            // 模块ID
		ModuleName        string    `gorm:"column:module_name;type:varchar(64);not null;comment:模块" json:"module_name"`                                                    // 模块
		ServiceInstanceID uint      `gorm:"column:service_instance_id;type:bigint;not null;comment:服务实例ID"`                                                                // 服务实例ID
		ServiceName       string    `gorm:"column:service_name;type:varchar(128);not null;comment:服务实例名称" json:"service_name"`                                             // 服务实例名称
		HostID            uint      `gorm:"column:host_id;type:bigint;not null;comment:主机ID"`                                                                              // 主机ID
		Environment       string    `gorm:"column:environment;type:varchar(128);not null;comment:环境类型（production/staging等）" json:"environment"`                            // 环境类型（production/staging等）
		Alias_            string    `gorm:"column:alias;type:varchar(128);comment:进程别名" json:"alias"`                                                                      // 进程别名
		InnerIP           string    `gorm:"column:inner_ip;type:varchar(64);not null;comment:内网IP" json:"inner_ip"`                                                        // 内网IP
		CcSyncStatus      string    `gorm:"column:cc_sync_status;type:varchar(64);not null;comment:cc同步状态:synced,deleted,updated" json:"cc_sync_status"`                   // cc同步状态:synced,deleted,updated
		CcSyncUpdatedAt   time.Time `gorm:"column:cc_sync_updated_at;type:timestamp;default:CURRENT_TIMESTAMP;comment:cc同步更新时间" json:"cc_sync_updated_at"`                 // cc同步更新时间
		SourceData        string    `gorm:"column:source_data;type:json;comment:当前同步的数据" json:"source_data"`                                                               // 当前同步的数据
		PrevData          string    `gorm:"column:prev_data;type:json;comment:上一次同步的数据" json:"prev_data"`                                                                  // 上一次同步的数据
		ProcNum           uint      `gorm:"column:proc_num;type:int unsigned;not null;comment:进程数量"`                                                                       // 进程数量

		// Revision is revision info of the resource
		Creator   string    `gorm:"type:varchar(64) not null"`
		Reviser   string    `gorm:"type:varchar(64) not null"`
		CreatedAt time.Time `gorm:"type:datetime(6) not null"`
		UpdatedAt time.Time `gorm:"type:datetime(6) not null"`
	}

	// IDGenerators : ID生成器
	type IDGenerators struct {
		ID        uint      `gorm:"type:bigint(1) unsigned not null;primaryKey"`
		Resource  string    `gorm:"type:varchar(50) not null;uniqueIndex:idx_resource"`
		MaxID     uint      `gorm:"type:bigint(1) unsigned not null"`
		UpdatedAt time.Time `gorm:"type:datetime(6) not null"`
	}

	if err := tx.Set("gorm:table_options", "ENGINE=InnoDB CHARSET=utf8mb4").
		AutoMigrate(&Process{}); err != nil {
		return err
	}

	now := time.Now()
	if result := tx.Create([]IDGenerators{
		{Resource: "process", MaxID: 0, UpdatedAt: now},
	}); result.Error != nil {
		return result.Error
	}

	return nil
}

// mig20250923114014Down for down migration
func mig20250923114014Down(tx *gorm.DB) error {
	// IDGenerators : ID生成器
	type IDGenerators struct {
		ID        uint      `gorm:"type:bigint(1) unsigned not null;primaryKey"`
		Resource  string    `gorm:"type:varchar(50) not null;uniqueIndex:idx_resource"`
		MaxID     uint      `gorm:"type:bigint(1) unsigned not null"`
		UpdatedAt time.Time `gorm:"type:datetime(6) not null"`
	}

	var resources = []string{
		"process",
	}
	if result := tx.Where("resource IN ?", resources).Delete(&IDGenerators{}); result.Error != nil {
		return result.Error
	}

	if err := tx.Migrator().DropTable("process"); err != nil {
		return err
	}

	return nil
}
