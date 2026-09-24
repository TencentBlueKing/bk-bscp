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

// collectTargetIDsQuery collects the IDs of the latest released config instance
// (max id where is_released=true) per (bk_process_id, config_template_id, inst_id)
// group for a given biz. Only IDs are returned so the expensive GROUP BY + MAX(id)
// aggregation runs once, and full rows are fetched later via primary-key lookups.
const collectTargetIDsQuery = "SELECT MAX(id) AS max_id FROM gsekit_configinstance " +
	"WHERE is_released = true AND bk_process_id IN (" +
	"  SELECT bk_process_id FROM gsekit_process WHERE bk_biz_id = ?" +
	") GROUP BY bk_process_id, config_template_id, inst_id " +
	"ORDER BY max_id"

// templateProcessKey is a composite key for checking whether a config template
// is still bound to a specific process (directly or via process template).
type templateProcessKey struct {
	configTemplateID int64
	processID        int64
}

// isProcessTemplateBound checks whether a config instance's (config_template_id,
// bk_process_id) pair still has a valid binding, either directly (INSTANCE type)
// or via the process's process_template_id (TEMPLATE type).
// The binding sets are pre-built during config template migration.
func (m *Migrator) isProcessTemplateBound(configTemplateID, bkProcessID int64) bool {
	if m.instanceBindSet[templateProcessKey{configTemplateID, bkProcessID}] {
		return true
	}
	ptID := m.processTemplateMap[bkProcessID]
	return ptID != 0 && m.templateBindSet[templateProcessKey{configTemplateID, ptID}]
}

// migrateConfigInstances migrates config instances from GSEKit to BSCP.
// Only the latest released instance (max id where is_released=true) per
// (bk_process_id, config_template_id, inst_id) is migrated, matching the
// GSEKit config delivery page display logic.
//
// The migration uses a two-phase approach to avoid OFFSET deep-pagination:
//   - Phase 1: collect all target IDs via a single GROUP BY aggregation
//   - Phase 2: fetch full rows in batches using primary-key IN lookups
//
// Instances whose config template has been deleted or unbound from the process
// are silently skipped.
func (m *Migrator) migrateConfigInstances() error {
	log.Println("=== Step 5: Migrating config instances ===")

	batchSize := m.cfg.Migration.BatchSize
	totalMigrated := 0
	creator := m.cfg.Migration.Creator
	reviser := m.cfg.Migration.Reviser

	for _, bizID := range m.cfg.Migration.BizIDs {
		log.Printf("  Processing config instances for biz %d", bizID)

		// Phase 1: collect all target IDs with a single aggregation query
		var targetIDs []int64
		if err := m.sourceDB.Raw(collectTargetIDsQuery, bizID).
			Scan(&targetIDs).Error; err != nil {
			return fmt.Errorf("collect target config instance IDs for biz %d failed: %w", bizID, err)
		}
		log.Printf("  Found %d latest released config instances in GSEKit for biz %d", len(targetIDs), bizID)

		if len(targetIDs) == 0 {
			continue
		}

		// Phase 2: fetch full rows in batches using WHERE id IN (...)
		for batchStart := 0; batchStart < len(targetIDs); batchStart += batchSize {
			batchEnd := batchStart + batchSize
			if batchEnd > len(targetIDs) {
				batchEnd = len(targetIDs)
			}
			batchIDs := targetIDs[batchStart:batchEnd]

			var instances []GSEKitConfigInstance
			if err := m.sourceDB.Where("id IN ?", batchIDs).
				Find(&instances).Error; err != nil {
				return fmt.Errorf("fetch config instances by IDs for biz %d failed: %w", bizID, err)
			}

			ids, err := m.idGen.BatchNextID("config_instances", len(instances))
			if err != nil {
				return fmt.Errorf("allocate config_instance IDs failed: %w", err)
			}

			now := time.Now()
			for i, inst := range instances {
				newID := ids[i]

				// Look up mapped config_template_id.
				// The source template may have been deleted in GSEKit (the config
				// instance table acts as a history table), so a missing mapping is
				// expected and the record is silently skipped.
				newConfigTemplateID, ok := m.configTemplateIDMap[uint32(inst.ConfigTemplateID)]
				if !ok {
					continue
				}

				// Look up mapped config_version_id → template_revision_id.
				// Same as above: the version may belong to a deleted template.
				newConfigVersionID, ok := m.configVersionIDMap[uint32(inst.ConfigVersionID)]
				if !ok {
					continue
				}

				// Skip instances whose process-template binding has been removed
				if !m.isProcessTemplateBound(inst.ConfigTemplateID, inst.BkProcessID) {
					continue
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
					creator, reviser, now, now,
				).Error; err != nil {
					if m.cfg.Migration.ContinueOnError {
						log.Printf("  Warning: insert config_instance failed for %d: %v", inst.ID, err)
						continue
					}
					return fmt.Errorf("insert config_instance for gsekit_id %d failed: %w", inst.ID, err)
				}
				totalMigrated++
			}

			log.Printf("  Progress: %d/%d config instances migrated for biz %d",
				totalMigrated, len(targetIDs), bizID)
		}
	}

	log.Printf("  Total config instances migrated: %d", totalMigrated)
	return nil
}
