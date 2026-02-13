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
)

// GSEKitConfigInstance represents a row from gsekit_configinstance table
type GSEKitConfigInstance struct {
	ID               int64     `gorm:"column:id;primaryKey"`
	ConfigVersionID  int64     `gorm:"column:config_version_id"`
	ConfigTemplateID int64     `gorm:"column:config_template_id"`
	BkProcessID      int64     `gorm:"column:bk_process_id"`
	InstID           int       `gorm:"column:inst_id"`
	Content          []byte    `gorm:"column:content"`
	Name             string    `gorm:"column:name"`
	Path             string    `gorm:"column:path"`
	IsLatest         bool      `gorm:"column:is_latest"`
	IsReleased       bool      `gorm:"column:is_released"`
	SHA256           string    `gorm:"column:sha256"`
	Expression       string    `gorm:"column:expression"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	CreatedBy        string    `gorm:"column:created_by"`
}

// TableName returns the GSEKit config instance table name
func (GSEKitConfigInstance) TableName() string { return "gsekit_configinstance" }

// latestReleasedQuery is the SQL to find the latest released config instance
// (max id) per (bk_process_id, config_template_id, inst_id) group,
// filtered by is_released=true and bk_process_id belonging to a given biz.
// This matches the GSEKit config delivery page logic (filter_released=True).
const latestReleasedQuery = "SELECT ci.* FROM gsekit_configinstance ci " +
	"INNER JOIN (" +
	"  SELECT MAX(id) AS max_id FROM gsekit_configinstance " +
	"  WHERE is_released = true AND bk_process_id IN (" +
	"    SELECT bk_process_id FROM gsekit_process WHERE bk_biz_id = ?" +
	"  ) GROUP BY bk_process_id, config_template_id, inst_id" +
	") latest ON ci.id = latest.max_id"

// latestReleasedCountQuery counts the number of latest released config instances.
const latestReleasedCountQuery = "SELECT COUNT(*) FROM (" +
	"  SELECT MAX(id) AS max_id FROM gsekit_configinstance " +
	"  WHERE is_released = true AND bk_process_id IN (" +
	"    SELECT bk_process_id FROM gsekit_process WHERE bk_biz_id = ?" +
	"  ) GROUP BY bk_process_id, config_template_id, inst_id" +
	") t"

// migrateConfigInstances migrates config instances from GSEKit to BSCP.
// Only the latest released instance (max id where is_released=true) per
// (bk_process_id, config_template_id, inst_id) is migrated, matching the
// GSEKit config delivery page display logic.
func (m *Migrator) migrateConfigInstances() error {
	log.Println("=== Step 5: Migrating config instances ===")

	batchSize := m.cfg.Migration.BatchSize
	totalMigrated := 0

	for _, bizID := range m.cfg.Migration.BizIDs {
		log.Printf("  Processing config instances for biz %d", bizID)

		var sourceCount int64
		if err := m.sourceDB.Raw(latestReleasedCountQuery, bizID).
			Scan(&sourceCount).Error; err != nil {
			return fmt.Errorf("count latest released config instances for biz %d failed: %w", bizID, err)
		}
		log.Printf("  Found %d latest released config instances in GSEKit for biz %d", sourceCount, bizID)

		if sourceCount == 0 {
			continue
		}

		offset := 0
		for {
			var instances []GSEKitConfigInstance
			if err := m.sourceDB.Raw(latestReleasedQuery+" ORDER BY ci.id LIMIT ? OFFSET ?",
				bizID, batchSize, offset).
				Scan(&instances).Error; err != nil {
				return fmt.Errorf("read latest released config instances for biz %d offset %d failed: %w",
					bizID, offset, err)
			}
			if len(instances) == 0 {
				break
			}

			ids, err := m.idGen.BatchNextID("config_instances", len(instances))
			if err != nil {
				return fmt.Errorf("allocate config_instance IDs failed: %w", err)
			}

			now := time.Now()
			for i, inst := range instances {
				newID := ids[i]

				// Look up mapped config_template_id
				newConfigTemplateID, ok := m.configTemplateIDMap[uint32(inst.ConfigTemplateID)]
				if !ok {
					if m.cfg.Migration.ContinueOnError {
						log.Printf("  Warning: no config_template mapping for %d, skipping instance %d",
							inst.ConfigTemplateID, inst.ID)
						continue
					}
					return fmt.Errorf("no config_template mapping for %d", inst.ConfigTemplateID)
				}

				// Look up mapped config_version_id → template_revision_id
				newConfigVersionID, ok := m.configVersionIDMap[uint32(inst.ConfigVersionID)]
				if !ok {
					if m.cfg.Migration.ContinueOnError {
						log.Printf("  Warning: no config_version mapping for %d, skipping instance %d",
							inst.ConfigVersionID, inst.ID)
						continue
					}
					return fmt.Errorf("no config_version mapping for %d", inst.ConfigVersionID)
				}

				// Decompress content (bz2)
				decompressed, err := decompressBz2(inst.Content)
				if err != nil {
					if m.cfg.Migration.ContinueOnError {
						log.Printf("  Warning: decompress content failed for instance %d: %v", inst.ID, err)
						continue
					}
					return fmt.Errorf("decompress content for instance %d failed: %w", inst.ID, err)
				}

				contentStr := string(decompressed)
				md5Hash := byteMD5(decompressed)

				if err := m.targetDB.Exec(
					"INSERT INTO config_instances (id, biz_id, config_template_id, config_version_id, "+
						"cc_process_id, module_inst_seq, task_id, tenant_id, md5, content, "+
						"creator, reviser, created_at, updated_at) "+
						"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
					newID, bizID, newConfigTemplateID, newConfigVersionID,
					uint32(inst.BkProcessID), inst.InstID, "", m.cfg.Migration.TenantID,
					md5Hash, contentStr,
					"gsekit-migration", "gsekit-migration", now, now,
				).Error; err != nil {
					if m.cfg.Migration.ContinueOnError {
						log.Printf("  Warning: insert config_instance failed for %d: %v", inst.ID, err)
						continue
					}
					return fmt.Errorf("insert config_instance for gsekit_id %d failed: %w", inst.ID, err)
				}
				totalMigrated++
			}

			offset += batchSize
			log.Printf("  Progress: %d config instances migrated for biz %d", totalMigrated, bizID)
		}
	}

	log.Printf("  Total config instances migrated: %d", totalMigrated)
	return nil
}
