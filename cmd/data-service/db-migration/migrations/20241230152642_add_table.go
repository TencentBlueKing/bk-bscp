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
		Version: "20241230152642",
		Name:    "20241230152642_add_table",
		Mode:    migrator.GormMode,
		Up:      mig20241230152642Up,
		Down:    mig20241230152642Down,
	})
}

// mig20241230152642Up for up migration
func mig20241230152642Up(tx *gorm.DB) error {
	// DataSourceInfo mapped from table <data_source_infos>
	type DataSourceInfo struct {
		ID         uint32    `gorm:"column:id;type:bigint unsigned;primaryKey" json:"id"`
		BizID      uint32    `gorm:"column:biz_id;type:bigint unsigned;not null;uniqueIndex:idx_bizID_name,priority:1" json:"biz_id"`
		Name       string    `gorm:"column:name;type:varchar(255);not null;uniqueIndex:idx_bizID_name,priority:2" json:"name"`
		Memo       *string   `gorm:"column:memo;type:varchar(255)" json:"memo"`
		SourceType string    `gorm:"column:source_type;type:varchar(20);not null" json:"source_type"`
		Dsn        string    `gorm:"column:dsn;type:varchar(800);not null" json:"dsn"`
		Creator    string    `gorm:"column:creator;type:varchar(64);not null" json:"creator"`
		Reviser    string    `gorm:"column:reviser;type:varchar(64);not null" json:"reviser"`
		CreatedAt  time.Time `gorm:"column:created_at;type:datetime(6);not null" json:"created_at"`
		UpdatedAt  time.Time `gorm:"column:updated_at;type:datetime(6);not null" json:"updated_at"`
	}

	// DataSourceMapping mapped from table <data_source_mappings>
	type DataSourceMapping struct {
		ID               uint32    `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement:true" json:"id"`
		BizID            uint32    `gorm:"column:biz_id;type:bigint unsigned;not null;uniqueIndex:idx_bizID_dsID_tName,priority:1" json:"biz_id"`
		DataSourceInfoID uint32    `gorm:"column:data_source_info_id;type:bigint;not null;uniqueIndex:idx_bizID_dsID_tName,priority:2" json:"data_source_info_id"`
		DatabasesName    string    `gorm:"column:databases_name;type:varchar(255);not null" json:"databases_name"`
		TableName_       string    `gorm:"column:table_name;type:varchar(255);not null;uniqueIndex:idx_bizID_dsID_tName,priority:3" json:"table_name"`
		TableMemo        *string   `gorm:"column:table_memo;type:varchar(255)" json:"table_memo"`
		VisibleRange     *string   `gorm:"column:visible_range;type:json" json:"visible_range"`
		Columns          *string   `gorm:"column:columns;type:json" json:"columns"`
		Creator          string    `gorm:"column:creator;type:varchar(64);not null" json:"creator"`
		Reviser          string    `gorm:"column:reviser;type:varchar(64);not null" json:"reviser"`
		CreatedAt        time.Time `gorm:"column:created_at;type:datetime(6);not null" json:"created_at"`
		UpdatedAt        time.Time `gorm:"column:updated_at;type:datetime(6);not null" json:"updated_at"`
	}

	// DataSourceContent mapped from table <data_source_contents>
	type DataSourceContent struct {
		ID                  uint32    `gorm:"column:id;type:bigint unsigned;primaryKey" json:"id"`
		DataSourceMappingID uint32    `gorm:"column:data_source_mapping_id;type:bigint;not null;index:data_source_mapping_id,priority:1" json:"data_source_mapping_id"`
		Content             *string   `gorm:"column:content;type:json" json:"content"`
		Status              string    `gorm:"column:status;type:varchar(64);not null" json:"status"`
		Creator             string    `gorm:"column:creator;type:varchar(64);not null" json:"creator"`
		Reviser             string    `gorm:"column:reviser;type:varchar(64);not null" json:"reviser"`
		CreatedAt           time.Time `gorm:"column:created_at;type:datetime(6);not null" json:"created_at"`
		UpdatedAt           time.Time `gorm:"column:updated_at;type:datetime(6);not null" json:"updated_at"`
	}

	// ReleasedTableContent mapped from table <released_table_contents>
	type ReleasedTableContent struct {
		ID          int64     `gorm:"column:id;type:bigint unsigned;primaryKey" json:"id"`
		BizID       int64     `gorm:"column:biz_id;type:bigint unsigned;not null;index:idx_bizID_appID_releaseKvID,priority:1" json:"biz_id"`
		AppID       int64     `gorm:"column:app_id;type:bigint unsigned;not null;index:idx_bizID_appID_releaseKvID,priority:2" json:"app_id"`
		ReleaseKvID int64     `gorm:"column:release_kv_id;type:bigint unsigned;not null;index:idx_bizID_appID_releaseKvID,priority:3" json:"release_kv_id"`
		Content     string    `gorm:"column:content;type:json;not null" json:"content"`
		Creator     string    `gorm:"column:creator;type:varchar(64);not null" json:"creator"`
		Reviser     string    `gorm:"column:reviser;type:varchar(64);not null" json:"reviser"`
		CreatedAt   time.Time `gorm:"column:created_at;type:datetime(6);not null" json:"created_at"`
		UpdatedAt   time.Time `gorm:"column:updated_at;type:datetime(6);not null" json:"updated_at"`
	}

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

	// IDGenerators : ID生成器
	type IDGenerators struct {
		ID        uint      `gorm:"type:bigint(1) unsigned not null;primaryKey"`
		Resource  string    `gorm:"type:varchar(50) not null;uniqueIndex:idx_resource"`
		MaxID     uint      `gorm:"type:bigint(1) unsigned not null"`
		UpdatedAt time.Time `gorm:"type:datetime(6) not null"`
	}

	if err := tx.Set("gorm:table_options", "ENGINE=InnoDB CHARSET=utf8mb4").
		AutoMigrate(&DataSourceInfo{}, &DataSourceMapping{}, &DataSourceContent{}, &ReleasedTableContent{}); err != nil {
		return err
	}

	now := time.Now()

	if result := tx.Create([]IDGenerators{
		{Resource: "data_source_infos", MaxID: 0, UpdatedAt: now},
		{Resource: "data_source_mappings", MaxID: 0, UpdatedAt: now},
		{Resource: "data_source_contents", MaxID: 0, UpdatedAt: now},
		{Resource: "released_table_contents", MaxID: 0, UpdatedAt: now},
	}); result.Error != nil {
		return result.Error
	}

	// Kv add new column
	if !tx.Migrator().HasColumn(&Kv{}, "managed_table_id") {
		if err := tx.Migrator().AddColumn(&Kv{}, "managed_table_id"); err != nil {
			return err
		}
	}
	// Kv add new column
	if !tx.Migrator().HasColumn(&Kv{}, "external_source_id") {
		if err := tx.Migrator().AddColumn(&Kv{}, "external_source_id"); err != nil {
			return err
		}
	}
	// Kv add new column
	if !tx.Migrator().HasColumn(&Kv{}, "filter_condition") {
		if err := tx.Migrator().AddColumn(&Kv{}, "filter_condition"); err != nil {
			return err
		}
	}
	// Kv add new column
	if !tx.Migrator().HasColumn(&Kv{}, "filter_fields") {
		if err := tx.Migrator().AddColumn(&Kv{}, "filter_fields"); err != nil {
			return err
		}
	}

	// ReleasedKv add new column
	if !tx.Migrator().HasColumn(&ReleasedKv{}, "managed_table_id") {
		if err := tx.Migrator().AddColumn(&ReleasedKv{}, "managed_table_id"); err != nil {
			return err
		}
	}
	// ReleasedKv add new column
	if !tx.Migrator().HasColumn(&ReleasedKv{}, "external_source_id") {
		if err := tx.Migrator().AddColumn(&ReleasedKv{}, "external_source_id"); err != nil {
			return err
		}
	}
	// ReleasedKv add new column
	if !tx.Migrator().HasColumn(&ReleasedKv{}, "filter_condition") {
		if err := tx.Migrator().AddColumn(&ReleasedKv{}, "filter_condition"); err != nil {
			return err
		}
	}
	// Kv add new column
	if !tx.Migrator().HasColumn(&ReleasedKv{}, "filter_fields") {
		if err := tx.Migrator().AddColumn(&ReleasedKv{}, "filter_fields"); err != nil {
			return err
		}
	}

	return nil
}

// mig20241230152642Down for down migration
func mig20241230152642Down(tx *gorm.DB) error {
	// IDGenerators : ID生成器
	type IDGenerators struct {
		ID        uint      `gorm:"type:bigint(1) unsigned not null;primaryKey"`
		Resource  string    `gorm:"type:varchar(50) not null;uniqueIndex:idx_resource"`
		MaxID     uint      `gorm:"type:bigint(1) unsigned not null"`
		UpdatedAt time.Time `gorm:"type:datetime(6) not null"`
	}

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

	if err := tx.Migrator().
		DropTable("data_source_infos", "data_source_mappings", "data_source_contents", "released_table_contents"); err != nil {
		return err
	}

	var resources = []string{
		"data_source_infos",
		"data_source_mappings",
		"data_source_contents",
		"released_table_contents",
	}

	if result := tx.Where("resource IN ?", resources).Delete(&IDGenerators{}); result.Error != nil {
		return result.Error
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
