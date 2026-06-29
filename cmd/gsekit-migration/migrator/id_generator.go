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

package migrator

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/TencentBlueKing/bk-bscp/cmd/gsekit-migration/config"
)

// IDGenerator allocates new IDs from the BSCP id_generators table
type IDGenerator struct {
	db *gorm.DB
}

// NewIDGenerator creates a new IDGenerator
func NewIDGenerator(db *gorm.DB) *IDGenerator {
	return &IDGenerator{db: db}
}

// NextID allocates the next ID for the given resource (table name)
func (g *IDGenerator) NextID(resource string) (uint32, error) {
	result := g.db.Exec(
		"UPDATE `id_generators` SET `max_id` = `max_id` + 1, `updated_at` = ? WHERE `resource` = ?",
		time.Now(), resource)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to update id_generator for %s: %w", resource, result.Error)
	}

	if result.RowsAffected == 0 {
		// Resource doesn't exist, create it
		if err := g.db.Exec(
			"INSERT INTO `id_generators` (`resource`, `max_id`, `updated_at`) VALUES (?, 1, ?)",
			resource, time.Now()).Error; err != nil {
			return 0, fmt.Errorf("failed to create id_generator for %s: %w", resource, err)
		}
		return 1, nil
	}

	var maxID uint32
	if err := g.db.Raw("SELECT `max_id` FROM `id_generators` WHERE `resource` = ?", resource).
		Scan(&maxID).Error; err != nil {
		return 0, fmt.Errorf("failed to get max_id for %s: %w", resource, err)
	}

	return maxID, nil
}

// BatchNextID allocates count new IDs for the given resource, returns them as a slice
func (g *IDGenerator) BatchNextID(resource string, count int) ([]uint32, error) {
	if count <= 0 {
		return nil, nil
	}

	result := g.db.Exec(
		"UPDATE `id_generators` SET `max_id` = `max_id` + ?, `updated_at` = ? WHERE `resource` = ?",
		count, time.Now(), resource)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to batch update id_generator for %s: %w", resource, result.Error)
	}

	if result.RowsAffected == 0 {
		// Resource doesn't exist, create it
		if err := g.db.Exec(
			"INSERT INTO `id_generators` (`resource`, `max_id`, `updated_at`) VALUES (?, ?, ?)",
			resource, count, time.Now()).Error; err != nil {
			return nil, fmt.Errorf("failed to create id_generator for %s: %w", resource, err)
		}
		ids := make([]uint32, count)
		for i := 0; i < count; i++ {
			ids[i] = uint32(i + 1)
		}
		return ids, nil
	}

	var maxID uint32
	if err := g.db.Raw("SELECT `max_id` FROM `id_generators` WHERE `resource` = ?", resource).
		Scan(&maxID).Error; err != nil {
		return nil, fmt.Errorf("failed to get max_id for %s: %w", resource, err)
	}

	ids := make([]uint32, count)
	startID := maxID - uint32(count) + 1
	for i := 0; i < count; i++ {
		ids[i] = startID + uint32(i)
	}

	return ids, nil
}

// connectDB creates a gorm.DB connection from MySQLConfig
func connectDB(mysqlCfg config.MySQLConfig, name string) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(mysqlCfg.DSN()),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s database: %w", name, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get %s database handle: %w", name, err)
	}
	sqlDB.SetMaxOpenConns(int(mysqlCfg.MaxOpenConn))
	sqlDB.SetMaxIdleConns(int(mysqlCfg.MaxIdleConn))

	log.Printf("Connected to %s database: %s", name, mysqlCfg.Database)
	return db, nil
}
