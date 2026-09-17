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
	"math"
	"strings"
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
	creator := m.cfg.Migration.Creator
	reviser := m.cfg.Migration.Reviser
	totalTemplates := 0
	totalRevisions := 0
	cosUploaded := 0
	cosSkipped := 0
	cosFailed := 0
	// bizTemplateIDs: bizID → 该业务下所有迁移生成的 BSCP template ID 列表，
	// 用于最后批量更新 template_sets.template_ids
	bizTemplateIDs := make(map[uint32][]uint32)

	for _, bizID := range m.cfg.Migration.BizIDs {
		log.Printf("  Processing config templates for biz %d", bizID)

		// templateSpaceMap: bizID → 模版空间信息(template_space_id, template_set_id)，
		// 在 Step 1 中创建，此处用于关联模版到对应的模版空间和模版套餐
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

		// bindingMap: GSEKit config_template_id → 该模版的绑定关系列表，
		// 用于迁移时填充 config_templates 的 cc_template_process_ids 和 cc_process_ids
		bindingMap := make(map[int64][]GSEKitConfigTemplateBindingRelationship)
		for _, b := range bindings {
			bindingMap[b.ConfigTemplateID] = append(bindingMap[b.ConfigTemplateID], b)
			// Also populate Migrator-level binding sets for config instance migration
			key := templateProcessKey{configTemplateID: b.ConfigTemplateID, processID: b.ProcessObjectID}
			switch b.ProcessObjectType {
			case "INSTANCE":
				m.instanceBindSet[key] = true
			case "TEMPLATE":
				m.templateBindSet[key] = true
			}
		}

		// Load bk_process_id → process_template_id mappings for this biz,
		// used by config instance migration to resolve TEMPLATE-type bindings.
		var ptMappings []struct {
			BkProcessID       int64 `gorm:"column:bk_process_id"`
			ProcessTemplateID int64 `gorm:"column:process_template_id"`
		}
		if err := m.sourceDB.Raw(
			"SELECT bk_process_id, process_template_id FROM gsekit_process WHERE bk_biz_id = ?",
			bizID).Scan(&ptMappings).Error; err != nil {
			return fmt.Errorf("read process template mappings for biz %d failed: %w", bizID, err)
		}
		for _, pt := range ptMappings {
			m.processTemplateMap[pt.BkProcessID] = pt.ProcessTemplateID
		}

		offset := 0
		for {
			var templates []GSEKitConfigTemplate
			// 分页必须带排序：MySQL 不保证无序分页跨批次的结果稳定，
			// 否则可能重复返回部分行（撞唯一索引后被静默跳过）并遗漏另一部分行
			if err := m.sourceDB.Where("bk_biz_id = ?", bizID).
				Order("config_template_id ASC").
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

				absPath, fileName := normalizePathName(tmpl.AbsPath, tmpl.FileName)
				now := time.Now()
				if err = m.targetDB.Exec(
					"INSERT INTO templates (id, name, path, memo, config_template_name, biz_id, template_space_id, "+
						"tenant_id, creator, reviser, created_at, updated_at) "+
						"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
					templateID, fileName, absPath, "", tmpl.TemplateName,
					bizID, spaceInfo.TemplateSpaceID, m.cfg.Migration.TenantID,
					creator, reviser, now, now,
				).Error; err != nil {
					if m.cfg.Migration.ContinueOnError {
						log.Printf("  Warning: insert template failed for config_template %d: %v", tmpl.ConfigTemplateID, err)
						continue
					}
					return fmt.Errorf("insert template for config_template %d failed: %w", tmpl.ConfigTemplateID, err)
				}
				// templateIDMap: GSEKit config_template_id → BSCP template ID，
				// 仅用于迁移完成后的统计计数（printSummary）
				m.templateIDMap[uint32(tmpl.ConfigTemplateID)] = templateID
				bizTemplateIDs[bizID] = append(bizTemplateIDs[bizID], templateID)

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
					// file_mode mapped from GSEKit line_separator: CRLF → "win", others → "unix"
					// GSEKit file_format is not migrated (it's a syntax highlight hint, not an OS mode)

					privilege := normalizePrivilege(tmpl.Filemode)
					fileMode := mapFileMode(tmpl.LineSeparator)

					if err = m.targetDB.Exec(
						"INSERT INTO template_revisions (id, revision_name, revision_memo, name, path, "+
							"file_type, file_mode, user, user_group, privilege, "+
							"signature, byte_size, md5, charset, "+
							"biz_id, template_space_id, template_id, tenant_id, "+
							"creator, created_at) "+
							"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
						revisionID, fmt.Sprintf("v%d", version.ConfigVersionID), version.Description,
						fileName, absPath,
						"text", fileMode, tmpl.Owner, tmpl.Group, privilege,
						uploadResult.Signature, uploadResult.ByteSize, uploadResult.Md5, "",
						bizID, spaceInfo.TemplateSpaceID, templateID, m.cfg.Migration.TenantID,
						creator, now,
					).Error; err != nil {
						if m.cfg.Migration.ContinueOnError {
							log.Printf("  Warning: insert template_revision failed for version %d: %v", version.ConfigVersionID, err)
							continue
						}
						return fmt.Errorf("insert template_revision for version %d failed: %w", version.ConfigVersionID, err)
					}
					// configVersionIDMap: GSEKit config_version_id → BSCP template_revision ID，
					// 供 migrateConfigInstances 中将配置实例关联到正确的 template_revision
					m.configVersionIDMap[uint32(version.ConfigVersionID)] = revisionID
					totalRevisions++
				}

				// 4. Create config_templates record with binding info
				// 主键直接复用 GSEKit 的 config_template_id：bk-sops 流程里保存的就是这个 ID，
				// 迁移后两侧必须一致，否则插件执行时查不到模版。
				// templates 与 template_revisions 的 ID 仍由 id_generators 分配。
				if tmpl.ConfigTemplateID <= 0 || tmpl.ConfigTemplateID > math.MaxUint32 {
					return fmt.Errorf("gsekit config_template_id %d is out of uint32 range",
						tmpl.ConfigTemplateID)
				}
				configTemplateID := uint32(tmpl.ConfigTemplateID)

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

				if err := m.targetDB.Exec(
					"INSERT INTO config_templates (id, name, highlight_style, biz_id, template_id, "+
						"cc_template_process_ids, cc_process_ids, tenant_id, "+
						"creator, reviser, created_at, updated_at) "+
						"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
					configTemplateID, tmpl.TemplateName, highlightStyle,
					bizID, templateID,
					ccTemplateProcessIDs, ccProcessIDs, m.cfg.Migration.TenantID,
					creator, reviser, now, now,
				).Error; err != nil {
					if m.cfg.Migration.ContinueOnError {
						log.Printf("  Warning: insert config_template failed for %d: %v", tmpl.ConfigTemplateID, err)
						continue
					}
					return fmt.Errorf("insert config_template for %d failed: %w", tmpl.ConfigTemplateID, err)
				}
				// configTemplateIDMap: GSEKit config_template_id → BSCP config_template ID，
				// 供 migrateConfigInstances 中将配置实例关联到正确的 config_template
				m.configTemplateIDMap[uint32(tmpl.ConfigTemplateID)] = configTemplateID
				totalTemplates++
			}

			offset += batchSize
			log.Printf("  Progress: %d config templates, %d revisions migrated for biz %d",
				totalTemplates, totalRevisions, bizID)
		}
	}

	// Update template_sets.template_ids with collected template IDs
	for bizID, templateIDs := range bizTemplateIDs {
		spaceInfo := m.templateSpaceMap[bizID]
		if err := m.targetDB.Exec(
			"UPDATE template_sets SET template_ids = ? WHERE id = ?",
			uint32SliceToJSON(templateIDs), spaceInfo.TemplateSetID,
		).Error; err != nil {
			return fmt.Errorf("update template_sets.template_ids for biz %d failed: %w", bizID, err)
		}
		log.Printf("  Updated template_set %d with %d template_ids for biz %d",
			spaceInfo.TemplateSetID, len(templateIDs), bizID)
	}

	if err := m.raiseConfigTemplateWatermark(); err != nil {
		return err
	}

	logUploadStats(cosUploaded, cosSkipped, cosFailed)
	log.Printf("  Total config templates migrated: %d, revisions: %d", totalTemplates, totalRevisions)
	return nil
}

const (
	// configTemplateIDConflictChunkSize 限制单次 IN 查询的 ID 数量。
	configTemplateIDConflictChunkSize = 1000
	// maxConfigTemplateIDConflictDetails 是报错时列出的冲突行上限。
	maxConfigTemplateIDConflictDetails = 20
)

// existingConfigTemplate 是目标库 config_templates 里与待写入 ID 冲突的一行。
type existingConfigTemplate struct {
	ID       uint32 `gorm:"column:id"`
	BizID    uint32 `gorm:"column:biz_id"`
	Name     string `gorm:"column:name"`
	Creator  string `gorm:"column:creator"`
	TenantID string `gorm:"column:tenant_id"`
}

// checkConfigTemplateIDConflict 确认本批 GSEKit 模版 ID 在目标库 config_templates
// 中尚未被占用。主键直接复用 GSEKit ID 后，残留的自建模版或未对齐的旧数据都会
// 让 INSERT 撞主键；必须在写入前显式失败，而不是落到 continue_on_error 的静默跳过。
// 恢复办法是先跑 align-template-id 把自建模版腾出 GSEKit 区间，或 cleanup 残留数据。
func (m *Migrator) checkConfigTemplateIDConflict() error {
	var sourceIDs []uint32
	if err := m.sourceDB.Raw(
		"SELECT config_template_id FROM gsekit_configtemplate WHERE bk_biz_id IN ?",
		m.cfg.Migration.BizIDs).Scan(&sourceIDs).Error; err != nil {
		return fmt.Errorf("read gsekit config_template_id for conflict check failed: %w", err)
	}
	if len(sourceIDs) == 0 {
		return nil
	}

	conflicts, err := m.loadExistingConfigTemplatesByIDs(sourceIDs)
	if err != nil {
		return err
	}

	if err := configTemplateIDConflictError(conflicts); err != nil {
		return err
	}

	log.Printf("  Config template id conflict check passed: %d gsekit ids, none occupied in BSCP",
		len(sourceIDs))
	return nil
}

// loadExistingConfigTemplatesByIDs 按主键查出目标库里已存在的 config_templates。
func (m *Migrator) loadExistingConfigTemplatesByIDs(ids []uint32) ([]existingConfigTemplate, error) {
	conflicts := make([]existingConfigTemplate, 0)
	for start := 0; start < len(ids); start += configTemplateIDConflictChunkSize {
		end := start + configTemplateIDConflictChunkSize
		if end > len(ids) {
			end = len(ids)
		}

		var chunk []existingConfigTemplate
		if err := m.targetDB.Raw(
			"SELECT id, biz_id, name, creator, tenant_id FROM config_templates WHERE id IN ?",
			ids[start:end]).Scan(&chunk).Error; err != nil {
			return nil, fmt.Errorf("query config_templates for id conflict failed: %w", err)
		}
		conflicts = append(conflicts, chunk...)
	}
	return conflicts, nil
}

// configTemplateIDConflictError 把冲突行整理成硬失败错误。无冲突时返回 nil。
func configTemplateIDConflictError(conflicts []existingConfigTemplate) error {
	if len(conflicts) == 0 {
		return nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "config_templates primary key conflict: %d GSEKit id(s) already exist in BSCP:\n",
		len(conflicts))

	shown := len(conflicts)
	if shown > maxConfigTemplateIDConflictDetails {
		shown = maxConfigTemplateIDConflictDetails
	}
	for _, c := range conflicts[:shown] {
		fmt.Fprintf(&b, "  id=%d biz_id=%d name=%q creator=%q tenant_id=%q\n",
			c.ID, c.BizID, c.Name, c.Creator, c.TenantID)
	}
	if len(conflicts) > maxConfigTemplateIDConflictDetails {
		fmt.Fprintf(&b, "  ... and %d more\n", len(conflicts)-maxConfigTemplateIDConflictDetails)
	}
	b.WriteString("run align-template-id to evacuate native templates from the GSEKit id range, " +
		"or cleanup leftover data, then retry")
	return fmt.Errorf("%s", b.String())
}

// checkConfigTemplateIDReserveBase 确认本批业务的 GSEKit 模版 ID 都在预留基线之下。
// 迁移把 config_template_id 直接当作 BSCP 主键写入，一旦某个 ID 触到基线就会撞进
// BSCP 自建模版的地盘。这种情况必须显式失败，而不是静默覆盖别人的记录。
// 恢复办法是调大 migration.config_template_id_reserve_base 并重跑 align-template-id。
func (m *Migrator) checkConfigTemplateIDReserveBase() error {
	reserveBase := m.cfg.Migration.ConfigTemplateIDReserveBase

	var maxID uint32
	if err := m.sourceDB.Raw(
		"SELECT COALESCE(MAX(config_template_id), 0) FROM gsekit_configtemplate WHERE bk_biz_id IN ?",
		m.cfg.Migration.BizIDs).Scan(&maxID).Error; err != nil {
		return fmt.Errorf("read max gsekit config_template_id failed: %w", err)
	}

	if maxID >= reserveBase {
		return fmt.Errorf("gsekit config_template_id reaches %d which is at or beyond the reserve base %d, "+
			"raise migration.config_template_id_reserve_base and re-run align-template-id before migrating",
			maxID, reserveBase)
	}

	log.Printf("  Config template id reserve base: %d, max gsekit id in this batch: %d", reserveBase, maxID)
	return nil
}

// raiseConfigTemplateWatermark 把 id_generators 的 config_templates 水位抬到预留基线之上。
// 主键改为复用 GSEKit ID 后不再经过 id_generators，水位必须单独抬高，
// 否则后续 BSCP 自建模版会分配到已被迁移数据占用的低位 ID。
// 护栏已保证本批最大 GSEKit ID 低于基线，抬到基线即足够；EnsureAtLeast 内部只增不减。
func (m *Migrator) raiseConfigTemplateWatermark() error {
	reserveBase := m.cfg.Migration.ConfigTemplateIDReserveBase
	if err := m.idGen.EnsureAtLeast("config_templates", reserveBase); err != nil {
		return err
	}
	log.Printf("  Raised id_generators watermark for config_templates to at least %d", reserveBase)
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

// mapFileMode maps GSEKit line_separator to BSCP file_mode.
// GSEKit line_separator values: "CR" (MacOs), "LF" (Unix), "CRLF" (Windows).
// BSCP file_mode values: "unix", "win".
func mapFileMode(lineSeparator string) string {
	if lineSeparator == "CRLF" {
		return "win"
	}
	return "unix"
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

// normalizePathName ensures that path ends with exactly one separator and name
// does not start with one, so BSCP's direct concatenation (path + name)
// produces the correct full path. GSEKit allows users to freely edit abs_path
// and file_name independently, so any combination is possible:
//
//	"/etc/nginx"  + "nginx.conf"  → "/etc/nginx/" + "nginx.conf"   (separator added)
//	"/etc/nginx/" + "/nginx.conf" → "/etc/nginx/" + "nginx.conf"   (duplicate removed)
//	"/etc/nginx"  + "/nginx.conf" → "/etc/nginx/" + "nginx.conf"   (both fixed)
//	"C:\nginx"    + "\n.conf"     → "C:\nginx\"   + "n.conf"       (Windows handled)
func normalizePathName(dirPath, fileName string) (string, string) {
	isWin := strings.ContainsRune(dirPath, '\\')
	sep := "/"
	if isWin {
		sep = "\\"
	}

	if dirPath == "" {
		dirPath = sep
	} else if !strings.HasSuffix(dirPath, sep) {
		dirPath += sep
	}

	fileName = strings.TrimLeft(fileName, "/\\")

	return dirPath, fileName
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
