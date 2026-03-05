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
)

// templateSpaceResult stores the created template space info per biz
type templateSpaceResult struct {
	TemplateSpaceID uint32
	TemplateSetID   uint32
}

// migrateTemplateSpaces creates template_spaces and template_sets for each biz
func (m *Migrator) migrateTemplateSpaces() error {
	log.Println("=== Step 1: Creating template spaces and template sets ===")

	for _, bizID := range m.cfg.Migration.BizIDs {
		result, err := m.ensureTemplateSpace(bizID)
		if err != nil {
			return fmt.Errorf("failed to create template space for biz %d: %w", bizID, err)
		}
		m.templateSpaceMap[bizID] = result
		log.Printf("  Biz %d: template_space_id=%d, template_set_id=%d",
			bizID, result.TemplateSpaceID, result.TemplateSetID)
	}

	return nil
}

// ensureTemplateSpace creates or finds an existing template space for a biz
func (m *Migrator) ensureTemplateSpace(bizID uint32) (*templateSpaceResult, error) {
	const spaceName = "config_delivery"
	now := time.Now()
	tenantID := m.cfg.Migration.TenantID
	creator := m.cfg.Migration.Creator
	reviser := m.cfg.Migration.Reviser

	// Check if template_space already exists
	var existingID uint32
	err := m.targetDB.Raw(
		"SELECT id FROM template_spaces WHERE biz_id = ? AND name = ? AND tenant_id = ? LIMIT 1",
		bizID, spaceName, tenantID).Scan(&existingID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("query existing template_space failed: %w", err)
	}

	var spaceID uint32
	if existingID > 0 {
		spaceID = existingID
		log.Printf("  Reusing existing template_space id=%d for biz %d", spaceID, bizID)
	} else {
		// Create new template_space
		spaceID, err = m.idGen.NextID("template_spaces")
		if err != nil {
			return nil, fmt.Errorf("allocate template_space id failed: %w", err)
		}

		err = m.targetDB.Exec(
			"INSERT INTO template_spaces (id, name, memo, biz_id, tenant_id, creator, reviser, created_at, updated_at) "+
				"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			spaceID, spaceName, "GSEKit migration config delivery space",
			bizID, tenantID, creator, reviser, now, now,
		).Error
		if err != nil {
			return nil, fmt.Errorf("insert template_space failed: %w", err)
		}
		log.Printf("  Created template_space id=%d for biz %d", spaceID, bizID)
	}

	// Check if template_set already exists under this space
	var existingSetID uint32
	err = m.targetDB.Raw(
		"SELECT id FROM template_sets WHERE biz_id = ? AND template_space_id = ? AND name = ? AND tenant_id = ? LIMIT 1",
		bizID, spaceID, spaceName, tenantID).Scan(&existingSetID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("query existing template_set failed: %w", err)
	}

	var setID uint32
	if existingSetID > 0 {
		setID = existingSetID
		log.Printf("  Reusing existing template_set id=%d for biz %d", setID, bizID)
	} else {
		setID, err = m.idGen.NextID("template_sets")
		if err != nil {
			return nil, fmt.Errorf("allocate template_set id failed: %w", err)
		}

		if err := m.targetDB.Exec(
			"INSERT INTO template_sets "+
				"(id, name, memo, template_ids, public, bound_apps, "+
				"biz_id, template_space_id, tenant_id, "+
				"creator, reviser, created_at, updated_at) "+
				"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			setID, spaceName, "GSEKit migration default template set",
			"[]", false, "[]",
			bizID, spaceID, tenantID,
			creator, reviser, now, now,
		).Error; err != nil {
			return nil, fmt.Errorf("insert template_set failed: %w", err)
		}
		log.Printf("  Created template_set id=%d for biz %d", setID, bizID)
	}

	return &templateSpaceResult{
		TemplateSpaceID: spaceID,
		TemplateSetID:   setID,
	}, nil
}
