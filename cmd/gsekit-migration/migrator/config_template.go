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
	"bytes"
	"compress/bzip2"
	"context"
	"fmt"
	"io"
	"log"
	"time"
)

// GSEKitConfigTemplate represents a row from gsekit_configtemplate table
type GSEKitConfigTemplate struct {
	ConfigTemplateID int64     `gorm:"column:config_template_id;primaryKey"`
	BkBizID          int64     `gorm:"column:bk_biz_id"`
	TemplateName     string    `gorm:"column:template_name"`
	FileName         string    `gorm:"column:file_name"`
	AbsPath          string    `gorm:"column:abs_path"`
	Owner            string    `gorm:"column:owner"`
	Group            string    `gorm:"column:group"`
	Filemode         string    `gorm:"column:filemode"`
	LineSeparator    string    `gorm:"column:line_separator"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	CreatedBy        string    `gorm:"column:created_by"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
	UpdatedBy        string    `gorm:"column:updated_by"`
}

// TableName returns the GSEKit config template table name
func (GSEKitConfigTemplate) TableName() string { return "gsekit_configtemplate" }

// GSEKitConfigTemplateVersion represents a row from gsekit_configtemplateversion table
type GSEKitConfigTemplateVersion struct {
	ConfigVersionID  int64     `gorm:"column:config_version_id;primaryKey"`
	ConfigTemplateID int64     `gorm:"column:config_template_id"`
	Description      string    `gorm:"column:description"`
	Content          []byte    `gorm:"column:content"`
	IsDraft          bool      `gorm:"column:is_draft"`
	IsActive         bool      `gorm:"column:is_active"`
	FileFormat       *string   `gorm:"column:file_format"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	CreatedBy        string    `gorm:"column:created_by"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
	UpdatedBy        string    `gorm:"column:updated_by"`
}

// TableName returns the GSEKit config template version table name
func (GSEKitConfigTemplateVersion) TableName() string { return "gsekit_configtemplateversion" }

// GSEKitConfigTemplateBindingRelationship represents a binding relationship
type GSEKitConfigTemplateBindingRelationship struct {
	ID                int64  `gorm:"column:id;primaryKey"`
	BkBizID           int64  `gorm:"column:bk_biz_id"`
	ConfigTemplateID  int64  `gorm:"column:config_template_id"`
	ProcessObjectType string `gorm:"column:process_object_type"`
	ProcessObjectID   int64  `gorm:"column:process_object_id"`
}

// TableName returns the binding relationship table name
func (GSEKitConfigTemplateBindingRelationship) TableName() string {
	return "gsekit_configtemplatebindingrelationship"
}

// migrateConfigTemplates migrates config templates from GSEKit to BSCP
// nolint:gocyclo,funlen
func (m *Migrator) migrateConfigTemplates() error {
	log.Println("=== Step 4: Migrating config templates ===")

	ctx := context.Background()
	batchSize := m.cfg.Migration.BatchSize
	totalTemplates := 0
	totalRevisions := 0
	cosUploaded := 0
	cosSkipped := 0
	cosFailed := 0

	for _, bizID := range m.cfg.Migration.BizIDs {
		log.Printf("  Processing config templates for biz %d", bizID)

		spaceInfo, ok := m.templateSpaceMap[bizID]
		if !ok {
			return fmt.Errorf("no template space found for biz %d", bizID)
		}

		// Count source records
		var sourceCount int64
		if err := m.sourceDB.Model(&GSEKitConfigTemplate{}).Where("bk_biz_id = ?", bizID).Count(&sourceCount).Error; err != nil {
			return fmt.Errorf("count gsekit_configtemplate for biz %d failed: %w", bizID, err)
		}
		log.Printf("  Found %d config templates in GSEKit for biz %d", sourceCount, bizID)

		if sourceCount == 0 {
			continue
		}

		// Read all binding relationships for this biz upfront
		var bindings []GSEKitConfigTemplateBindingRelationship
		if err := m.sourceDB.Where("bk_biz_id = ?", bizID).Find(&bindings).Error; err != nil {
			return fmt.Errorf("read binding relationships for biz %d failed: %w", bizID, err)
		}

		// Group bindings by config_template_id
		bindingMap := make(map[int64][]GSEKitConfigTemplateBindingRelationship)
		for _, b := range bindings {
			bindingMap[b.ConfigTemplateID] = append(bindingMap[b.ConfigTemplateID], b)
		}

		offset := 0
		for {
			var templates []GSEKitConfigTemplate
			if err := m.sourceDB.Where("bk_biz_id = ?", bizID).
				Offset(offset).Limit(batchSize).
				Find(&templates).Error; err != nil {
				return fmt.Errorf("read gsekit_configtemplate batch for biz %d offset %d failed: %w", bizID, offset, err)
			}
			if len(templates) == 0 {
				break
			}

			for _, tmpl := range templates {
				// 1. Get ALL non-draft versions for this template (preserve full history)
				var versions []GSEKitConfigTemplateVersion
				if err := m.sourceDB.Where("config_template_id = ? AND is_draft = ?",
					tmpl.ConfigTemplateID, false).
					Order("config_version_id ASC").
					Find(&versions).Error; err != nil {
					if m.cfg.Migration.ContinueOnError {
						log.Printf("  Warning: query versions for config_template %d failed: %v", tmpl.ConfigTemplateID, err)
						continue
					}
					return fmt.Errorf("query versions for config_template %d failed: %w", tmpl.ConfigTemplateID, err)
				}

				if len(versions) == 0 {
					log.Printf("  Info: config_template %d has no published versions, skipping", tmpl.ConfigTemplateID)
					continue
				}

				// 2. Create templates record (one per ConfigTemplate)
				templateID, err := m.idGen.NextID("templates")
				if err != nil {
					return fmt.Errorf("allocate template id failed: %w", err)
				}

				now := time.Now()
				if err = m.targetDB.Exec(
					"INSERT INTO templates (id, name, path, memo, biz_id, template_space_id, tenant_id, "+
						"creator, reviser, created_at, updated_at) "+
						"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
					templateID, tmpl.FileName, tmpl.AbsPath, "",
					bizID, spaceInfo.TemplateSpaceID, m.cfg.Migration.TenantID,
					"gsekit-migration", "gsekit-migration", now, now,
				).Error; err != nil {
					if m.cfg.Migration.ContinueOnError {
						log.Printf("  Warning: insert template failed for config_template %d: %v", tmpl.ConfigTemplateID, err)
						continue
					}
					return fmt.Errorf("insert template for config_template %d failed: %w", tmpl.ConfigTemplateID, err)
				}
				m.templateIDMap[uint32(tmpl.ConfigTemplateID)] = templateID

				// 3. Create template_revisions for EACH non-draft version
				for _, version := range versions {
					// Template version content is stored as plain text, no decompression needed
					content := version.Content

					// Upload to repository
					var uploadResult *UploadResult
					if m.uploader != nil {
						uploadResult, err = m.uploader.Upload(ctx, bizID, content)
						if err != nil {
							cosFailed++
							if m.cfg.Migration.ContinueOnError {
								log.Printf("  Warning: upload failed for version %d: %v", version.ConfigVersionID, err)
								continue
							}
							return fmt.Errorf("upload for version %d failed: %w", version.ConfigVersionID, err)
						}
						cosUploaded++
					} else {
						// No uploader configured, compute hashes but skip upload
						uploadResult = computeContentHashes(content)
						cosSkipped++
					}

					var revisionID uint32
					revisionID, err = m.idGen.NextID("template_revisions")
					if err != nil {
						return fmt.Errorf("allocate template_revision id failed: %w", err)
					}

					// file_type fixed to "text" (all GSEKit config templates are text)
					// file_mode fixed to "unix" (GSEKit abs_path is Unix-style, and BSCP "win" mode
					// doesn't support permission validation)
					// GSEKit file_format is not migrated (it's a syntax highlight hint, not an OS mode)

					privilege := normalizePrivilege(tmpl.Filemode)

					if err = m.targetDB.Exec(
						"INSERT INTO template_revisions (id, revision_name, revision_memo, name, path, "+
							"file_type, file_mode, user, user_group, privilege, "+
							"signature, byte_size, md5, charset, "+
							"biz_id, template_space_id, template_id, tenant_id, "+
							"creator, created_at) "+
							"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
						revisionID, fmt.Sprintf("v%d", version.ConfigVersionID), version.Description,
						tmpl.FileName, tmpl.AbsPath,
						"text", "unix", tmpl.Owner, tmpl.Group, privilege,
						uploadResult.Signature, uploadResult.ByteSize, uploadResult.Md5, "UTF-8",
						bizID, spaceInfo.TemplateSpaceID, templateID, m.cfg.Migration.TenantID,
						"gsekit-migration", now,
					).Error; err != nil {
						if m.cfg.Migration.ContinueOnError {
							log.Printf("  Warning: insert template_revision failed for version %d: %v", version.ConfigVersionID, err)
							continue
						}
						return fmt.Errorf("insert template_revision for version %d failed: %w", version.ConfigVersionID, err)
					}
					m.configVersionIDMap[uint32(version.ConfigVersionID)] = revisionID
					totalRevisions++
				}

				// 4. Create config_templates record with binding info
				configTemplateID, err := m.idGen.NextID("config_templates")
				if err != nil {
					return fmt.Errorf("allocate config_template id failed: %w", err)
				}

				// Determine highlight_style from active version's file_format
				highlightStyle := mapHighlightStyle(versions)

				// Process binding relationships
				ccTemplateProcessIDs := "[]"
				ccProcessIDs := "[]"
				if rels, ok := bindingMap[tmpl.ConfigTemplateID]; ok {
					templateProcIDs := make([]uint32, 0)
					instanceProcIDs := make([]uint32, 0)
					for _, rel := range rels {
						switch rel.ProcessObjectType {
						case "TEMPLATE":
							templateProcIDs = append(templateProcIDs, uint32(rel.ProcessObjectID))
						case "INSTANCE":
							instanceProcIDs = append(instanceProcIDs, uint32(rel.ProcessObjectID))
						}
					}
					if len(templateProcIDs) > 0 {
						ccTemplateProcessIDs = uint32SliceToJSON(templateProcIDs)
					}
					if len(instanceProcIDs) > 0 {
						ccProcessIDs = uint32SliceToJSON(instanceProcIDs)
					}
				}

				if err = m.targetDB.Exec(
					"INSERT INTO config_templates (id, name, highlight_style, biz_id, template_id, "+
						"cc_template_process_ids, cc_process_ids, tenant_id, "+
						"creator, reviser, created_at, updated_at) "+
						"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
					configTemplateID, tmpl.TemplateName, highlightStyle,
					bizID, templateID,
					ccTemplateProcessIDs, ccProcessIDs, m.cfg.Migration.TenantID,
					"gsekit-migration", "gsekit-migration", now, now,
				).Error; err != nil {
					if m.cfg.Migration.ContinueOnError {
						log.Printf("  Warning: insert config_template failed for %d: %v", tmpl.ConfigTemplateID, err)
						continue
					}
					return fmt.Errorf("insert config_template for %d failed: %w", tmpl.ConfigTemplateID, err)
				}
				m.configTemplateIDMap[uint32(tmpl.ConfigTemplateID)] = configTemplateID
				totalTemplates++
			}

			offset += batchSize
			log.Printf("  Progress: %d config templates, %d revisions migrated for biz %d",
				totalTemplates, totalRevisions, bizID)
		}
	}

	logUploadStats(cosUploaded, cosSkipped, cosFailed)
	log.Printf("  Total config templates migrated: %d, revisions: %d", totalTemplates, totalRevisions)
	return nil
}

// decompressBz2 decompresses bz2 compressed data.
// If the data is not bz2 compressed (no "BZh" magic header), the original data is returned as-is.
// If the data has a bz2 header but decompression fails, an error is returned.
func decompressBz2(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}

	// BZ2 magic bytes: "BZh" (0x42, 0x5A, 0x68)
	if len(data) < 3 || data[0] != 0x42 || data[1] != 0x5A || data[2] != 0x68 {
		// Not bz2 compressed, return original data as plain text
		return data, nil
	}

	reader := bzip2.NewReader(bytes.NewReader(data))
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("bz2 decompression failed: %w", err)
	}

	return decompressed, nil
}

// mapHighlightStyle determines highlight_style for config_templates from the active version's file_format.
// GSEKit file_format has 4 known values: "python", "yaml", "json", "javascript",
// which map 1:1 to BSCP config_templates.highlight_style.
// If file_format is absent or not one of these 4, defaults to "python".
func mapHighlightStyle(versions []GSEKitConfigTemplateVersion) string {
	// Prefer the active version's file_format; fall back to the last version
	var fileFormat string
	for i := len(versions) - 1; i >= 0; i-- {
		if versions[i].IsActive && versions[i].FileFormat != nil && *versions[i].FileFormat != "" {
			fileFormat = *versions[i].FileFormat
			break
		}
	}
	if fileFormat == "" {
		// Fall back to the last version's file_format
		for i := len(versions) - 1; i >= 0; i-- {
			if versions[i].FileFormat != nil && *versions[i].FileFormat != "" {
				fileFormat = *versions[i].FileFormat
				break
			}
		}
	}

	switch fileFormat {
	case "python", "yaml", "json", "javascript":
		return fileFormat
	default:
		return "python"
	}
}

// normalizePrivilege ensures privilege is in 3-digit format
func normalizePrivilege(mode string) string {
	if len(mode) == 3 {
		return mode
	}
	if len(mode) == 4 {
		// Remove leading 0 (e.g., "0755" -> "755")
		return mode[1:]
	}
	if mode == "" {
		return "644"
	}
	return mode
}

// uint32SliceToJSON converts a uint32 slice to JSON array string
func uint32SliceToJSON(ids []uint32) string {
	if len(ids) == 0 {
		return "[]"
	}
	result := "["
	for i, id := range ids {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%d", id)
	}
	result += "]"
	return result
}
