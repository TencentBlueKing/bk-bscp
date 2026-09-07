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

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/cmd/gsekit-migration/config"
)

// Cleaner handles cleanup of migrated data
type Cleaner struct {
	cfg      *config.Config
	targetDB *gorm.DB
}

// CleanupReport contains the cleanup results
type CleanupReport struct {
	Success  bool
	Duration time.Duration
	Tables   []TableCleanupResult
}

// TableCleanupResult contains cleanup result for a single table
type TableCleanupResult struct {
	TableName    string
	DeletedCount int64
	Success      bool
	Error        string
}

// NewCleaner creates a new Cleaner
func NewCleaner(cfg *config.Config, targetDB *gorm.DB) *Cleaner {
	return &Cleaner{
		cfg:      cfg,
		targetDB: targetDB,
	}
}

// Cleanup deletes migrated data in reverse dependency order
func (c *Cleaner) Cleanup() (*CleanupReport, error) {
	startTime := time.Now()
	report := &CleanupReport{Success: true}

	log.Printf("Cleaning up migrated data for biz_ids: %v", c.cfg.Migration.BizIDs)

	// First, find the template_space IDs created by migration for later use
	templateSpaceIDs := make([]uint32, 0)
	for _, bizID := range c.cfg.Migration.BizIDs {
		var spaceIDs []uint32
		if err := c.targetDB.Raw(
			"SELECT id FROM template_spaces WHERE biz_id = ? AND name = ? AND tenant_id = ?",
			bizID, "config_delivery", c.cfg.Migration.TenantID).Scan(&spaceIDs).Error; err != nil {
			log.Printf("  Warning: query template_space for biz %d failed: %v", bizID, err)
			continue
		}
		templateSpaceIDs = append(templateSpaceIDs, spaceIDs...)
	}

	// Delete in reverse dependency order
	type cleanupStep struct {
		tableName string
		query     string
		args      []interface{}
	}

	steps := []cleanupStep{
		{
			tableName: "config_instances",
			query:     "DELETE FROM config_instances WHERE biz_id IN ? AND tenant_id = ?",
			args:      []interface{}{c.cfg.Migration.BizIDs, c.cfg.Migration.TenantID},
		},
		{
			tableName: "config_templates",
			query:     "DELETE FROM config_templates WHERE biz_id IN ? AND tenant_id = ?",
			args:      []interface{}{c.cfg.Migration.BizIDs, c.cfg.Migration.TenantID},
		},
	}

	// Template revisions, templates, template_sets - filter by template_space_id
	if len(templateSpaceIDs) > 0 {
		steps = append(steps,
			cleanupStep{
				tableName: "template_revisions",
				query:     "DELETE FROM template_revisions WHERE biz_id IN ? AND template_space_id IN ? AND tenant_id = ?",
				args:      []interface{}{c.cfg.Migration.BizIDs, templateSpaceIDs, c.cfg.Migration.TenantID},
			},
			cleanupStep{
				tableName: "templates",
				query:     "DELETE FROM templates WHERE biz_id IN ? AND template_space_id IN ? AND tenant_id = ?",
				args:      []interface{}{c.cfg.Migration.BizIDs, templateSpaceIDs, c.cfg.Migration.TenantID},
			},
			cleanupStep{
				tableName: "template_sets",
				query:     "DELETE FROM template_sets WHERE biz_id IN ? AND template_space_id IN ? AND tenant_id = ?",
				args:      []interface{}{c.cfg.Migration.BizIDs, templateSpaceIDs, c.cfg.Migration.TenantID},
			},
		)
	}

	steps = append(steps,
		cleanupStep{
			tableName: "template_spaces",
			query:     "DELETE FROM template_spaces WHERE biz_id IN ? AND name = ? AND tenant_id = ?",
			args:      []interface{}{c.cfg.Migration.BizIDs, "config_delivery", c.cfg.Migration.TenantID},
		},
		cleanupStep{
			tableName: "process_instances",
			query:     "DELETE FROM process_instances WHERE biz_id IN ? AND tenant_id = ?",
			args:      []interface{}{c.cfg.Migration.BizIDs, c.cfg.Migration.TenantID},
		},
		cleanupStep{
			tableName: "processes",
			query:     "DELETE FROM processes WHERE biz_id IN ? AND tenant_id = ?",
			args:      []interface{}{c.cfg.Migration.BizIDs, c.cfg.Migration.TenantID},
		},
	)

	for _, step := range steps {
		result := c.cleanupTable(step.tableName, step.query, step.args)
		report.Tables = append(report.Tables, result)
		if !result.Success {
			report.Success = false
		}
	}

	report.Duration = time.Since(startTime)
	log.Printf("Cleanup completed in %v", report.Duration)
	return report, nil
}

func (c *Cleaner) cleanupTable(tableName, query string, args []interface{}) TableCleanupResult {
	result := TableCleanupResult{
		TableName: tableName,
		Success:   true,
	}

	execResult := c.targetDB.Exec(query, args...)
	if execResult.Error != nil {
		result.Error = fmt.Sprintf("delete failed: %v", execResult.Error)
		result.Success = false
		log.Printf("  [FAIL] %s: %s", tableName, result.Error)
		return result
	}

	result.DeletedCount = execResult.RowsAffected
	log.Printf("  [OK] %s: %d records deleted", tableName, result.DeletedCount)
	return result
}
