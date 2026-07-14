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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/TencentBlueKing/bk-bscp/cmd/tenant-migration/config"
)

// FailedRow records a single row that failed during migration
type FailedRow struct {
	Table    string                 `json:"table"`
	SourceID uint32                 `json:"source_id"`
	NewID    uint32                 `json:"new_id,omitempty"`
	BizID    uint32                 `json:"biz_id"`
	Error    string                 `json:"error"`
	Data     map[string]interface{} `json:"data"`
}

// MySQLMigrator handles MySQL data migration
type MySQLMigrator struct {
	cfg        *config.Config
	sourceDB   *gorm.DB
	targetDB   *gorm.DB
	idMapper   *IDMapper
	failedRows []FailedRow
}

// NewMySQLMigrator creates a new MySQLMigrator instance
func NewMySQLMigrator(cfg *config.Config) (*MySQLMigrator, error) {
	// Connect to source database
	sourceDB, err := gorm.Open(mysql.Open(cfg.Source.MySQL.DSN()),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to source database: %w", err)
	}

	sqlDB, err := sourceDB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get source database handle: %w", err)
	}
	sqlDB.SetMaxOpenConns(int(cfg.Source.MySQL.MaxOpenConn))
	sqlDB.SetMaxIdleConns(int(cfg.Source.MySQL.MaxIdleConn))

	err = sqlDB.Ping()
	if err != nil {
		return nil, fmt.Errorf("source database is unreachable: %w", err)
	}
	log.Println("Source MySQL connection OK")

	// Connect to target database
	targetDB, err := gorm.Open(mysql.Open(cfg.Target.MySQL.DSN()),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to target database: %w", err)
	}

	sqlDB, err = targetDB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get target database handle: %w", err)
	}
	sqlDB.SetMaxOpenConns(int(cfg.Target.MySQL.MaxOpenConn))
	sqlDB.SetMaxIdleConns(int(cfg.Target.MySQL.MaxIdleConn))

	err = sqlDB.Ping()
	if err != nil {
		return nil, fmt.Errorf("target database is unreachable: %w", err)
	}
	log.Println("Target MySQL connection OK")

	return &MySQLMigrator{
		cfg:      cfg,
		sourceDB: sourceDB,
		targetDB: targetDB,
		idMapper: NewIDMapper(),
	}, nil
}

// Close closes database connections
func (m *MySQLMigrator) Close() error {
	if m.sourceDB != nil {
		sqlDB, err := m.sourceDB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
	if m.targetDB != nil {
		sqlDB, err := m.targetDB.DB()
		if err == nil {
			if err := sqlDB.Close(); err != nil {
				return fmt.Errorf("failed to close target database: %w", err)
			}
		}
	}
	return nil
}

// GetIDMapper returns the ID mapper for use by other components (e.g., Vault migrator)
func (m *MySQLMigrator) GetIDMapper() *IDMapper {
	return m.idMapper
}

// Migrate performs the MySQL data migration (insert only, cleanup should be done separately)
func (m *MySQLMigrator) Migrate() ([]TableMigrationResult, error) {
	results := make([]TableMigrationResult, 0)

	// Log biz_id filter info
	if m.cfg.Migration.HasBizFilter() {
		log.Printf("MySQL migration with biz_id filter: %v", m.cfg.Migration.BizIDs)
	}

	// Clear ID mappings from any previous runs
	m.idMapper.ClearAll()

	// Disable foreign key checks on target database
	if err := m.targetDB.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
		return nil, fmt.Errorf("failed to disable foreign key checks: %w", err)
	}
	defer func() {
		if err := m.targetDB.Exec("SET FOREIGN_KEY_CHECKS = 1").Error; err != nil {
			log.Printf("Warning: failed to re-enable foreign key checks: %v", err)
		}
	}()

	// Insert data in dependency order
	log.Println("Inserting data into target database...")
	insertTables := TablesInInsertOrder()
	for _, tableName := range insertTables {
		if m.cfg.ShouldSkipTable(tableName) {
			log.Printf("Skipping table: %s", tableName)
			continue
		}

		result := m.migrateTable(tableName)
		results = append(results, result)

		if !result.Success && !m.cfg.Migration.ContinueOnError {
			return results, fmt.Errorf("migration failed for table %s: %v", tableName, result.Errors)
		}
	}

	return results, nil
}

// migrateTable migrates a single table with new ID allocation and foreign key conversion
// nolint: funlen
func (m *MySQLMigrator) migrateTable(tableName string) TableMigrationResult {
	startTime := time.Now()
	result := TableMigrationResult{
		TableName: tableName,
		Success:   true,
	}

	log.Printf("Migrating table: %s", tableName)

	// Get table metadata
	meta, hasMeta := GetTableMeta(tableName)
	if !hasMeta {
		log.Printf("  Warning: no metadata for table %s, using default settings", tableName)
		meta = TableMeta{Name: tableName, IDColumn: "id", HasBizID: true}
	}

	// Check if table has biz_id column for filtering
	hasBizID := m.hasBizIDColumn(tableName)
	hasBizFilter := m.cfg.Migration.HasBizFilter()

	// Build base query with biz_id filter if applicable
	baseQuery := m.sourceDB.Table(tableName)
	if hasBizID && hasBizFilter {
		baseQuery = baseQuery.Where("biz_id IN ?", m.cfg.Migration.BizIDs)
		log.Printf("  Filtering by biz_id: %v", m.cfg.Migration.BizIDs)
	}

	// Get source count (with filter if applicable)
	var sourceCount int64
	if err := baseQuery.Count(&sourceCount).Error; err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to count source records: %v", err))
		result.Success = false
		result.Duration = time.Since(startTime)
		return result
	}
	result.SourceCount = sourceCount

	if sourceCount == 0 {
		log.Printf("  Table %s is empty (or no matching biz_id), skipping", tableName)
		result.Duration = time.Since(startTime)
		return result
	}

	// Check if table has tenant_id column
	hasTenantID := m.hasTenantIDColumn(tableName)

	// Migrate in batches
	batchSize := m.cfg.Migration.BatchSize
	offset := 0
	migratedCount := int64(0)

	for {
		var rows []map[string]interface{}
		// Rebuild query for each batch (GORM modifies the query object)
		batchQuery := m.sourceDB.Table(tableName)
		if hasBizID && hasBizFilter {
			batchQuery = batchQuery.Where("biz_id IN ?", m.cfg.Migration.BizIDs)
		}
		if err := batchQuery.Order("id").Offset(offset).Limit(batchSize).Find(&rows).Error; err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to read batch at offset %d: %v", offset, err))
			result.Success = false
			break
		}

		if len(rows) == 0 {
			break
		}

		// Process each row
		for _, row := range rows {
			// Get source ID before modifying the row
			sourceID := m.getUint32FromRow(row, meta.IDColumn)

			// Allocate new ID from id_generators
			newID, err := m.getNextID(tableName)
			if err != nil {
				result.ErrorCount++
				errMsg := fmt.Sprintf("failed to get next ID: %v", err)
				result.Errors = append(result.Errors, errMsg)
				m.logFailedRow(tableName, sourceID, 0, row, errMsg)
				if !m.cfg.Migration.ContinueOnError {
					result.Success = false
					break
				}
				continue
			}

			// Store ID mapping (source -> target)
			m.idMapper.Set(tableName, sourceID, newID)

			// Replace ID with new ID
			row[meta.IDColumn] = newID

			// Convert foreign keys using ID mapper
			if err := m.convertForeignKeys(row, meta); err != nil {
				result.ErrorCount++
				errMsg := fmt.Sprintf("failed to convert foreign keys (sourceID=%d): %v", sourceID, err)
				result.Errors = append(result.Errors, errMsg)
				m.logFailedRow(tableName, sourceID, newID, row, errMsg)
				if !m.cfg.Migration.ContinueOnError {
					result.Success = false
					break
				}
				continue
			}

			// Convert JSON array foreign keys (e.g. template_ids in template_sets)
			if err := m.convertJSONArrayFKs(row, meta); err != nil {
				result.ErrorCount++
				errMsg := fmt.Sprintf("failed to convert JSON array FKs (sourceID=%d): %v", sourceID, err)
				result.Errors = append(result.Errors, errMsg)
				m.logFailedRow(tableName, sourceID, newID, row, errMsg)
				if !m.cfg.Migration.ContinueOnError {
					result.Success = false
					break
				}
				continue
			}

			// Convert complex bindings JSON in app_template_bindings
			if tableName == "app_template_bindings" {
				if err := m.convertBindingsJSON(row); err != nil {
					result.ErrorCount++
					errMsg := fmt.Sprintf("failed to convert bindings JSON (sourceID=%d): %v", sourceID, err)
					result.Errors = append(result.Errors, errMsg)
					m.logFailedRow(tableName, sourceID, newID, row, errMsg)
					if !m.cfg.Migration.ContinueOnError {
						result.Success = false
						break
					}
					continue
				}
			}

			// Fill tenant_id if the column exists
			if hasTenantID {
				row["tenant_id"] = m.cfg.Migration.TargetTenantID
			}

			// Handle special cases
			m.handleSpecialCases(tableName, row)

			// Insert into target database
			if err := m.targetDB.Table(tableName).Create(row).Error; err != nil {
				result.ErrorCount++
				errMsg := fmt.Sprintf("failed to insert row (sourceID=%d, newID=%d): %v", sourceID, newID, err)
				result.Errors = append(result.Errors, errMsg)
				m.logFailedRow(tableName, sourceID, newID, row, errMsg)
				if !m.cfg.Migration.ContinueOnError {
					result.Success = false
					break
				}
			} else {
				migratedCount++
			}
		}

		if !result.Success {
			break
		}

		offset += batchSize
		log.Printf("  Progress: %d/%d records migrated", migratedCount, sourceCount)
	}

	result.MigratedCount = migratedCount
	result.Duration = time.Since(startTime)

	if result.Success {
		log.Printf("  Completed: %d records migrated in %v (ID mappings: %d)", migratedCount, result.Duration, m.idMapper.Count(tableName))
	} else {
		log.Printf("  Failed: %d records migrated, %d errors", migratedCount, result.ErrorCount)
	}

	return result
}

// getUint32FromRow extracts a uint32 value from a row map
func (m *MySQLMigrator) getUint32FromRow(row map[string]interface{}, column string) uint32 {
	val, ok := row[column]
	if !ok || val == nil {
		return 0
	}

	switch v := val.(type) {
	case uint32:
		return v
	case uint64:
		return uint32(v)
	case int64:
		return uint32(v)
	case int:
		return uint32(v)
	case float64:
		return uint32(v)
	default:
		return 0
	}
}

// getNextID allocates the next ID from id_generators table
func (m *MySQLMigrator) getNextID(tableName string) (uint32, error) {
	// Use resource name (table name is usually the resource name)
	resource := tableName

	// Update max_id and get the new value atomically
	result := m.targetDB.Exec(
		"UPDATE `id_generators` SET `max_id` = `max_id` + 1, `updated_at` = ? WHERE `resource` = ?",
		time.Now(), resource)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to update id_generator: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		// Resource doesn't exist, create it
		if err := m.targetDB.Exec(
			"INSERT INTO `id_generators` (`resource`, `max_id`, `updated_at`) VALUES (?, 1, ?)",
			resource, time.Now()).Error; err != nil {
			return 0, fmt.Errorf("failed to create id_generator: %w", err)
		}
		return 1, nil
	}

	// Get the new max_id
	var maxID uint32
	if err := m.targetDB.Raw("SELECT `max_id` FROM `id_generators` WHERE `resource` = ?", resource).
		Scan(&maxID).Error; err != nil {
		return 0, fmt.Errorf("failed to get max_id: %w", err)
	}

	return maxID, nil
}

// convertForeignKeys converts foreign key values using ID mapper
// Returns an error if a foreign key reference cannot be found in the mapper
func (m *MySQLMigrator) convertForeignKeys(row map[string]interface{}, meta TableMeta) error {
	if len(meta.ForeignKeys) == 0 {
		return nil
	}

	for fkColumn, refTable := range meta.ForeignKeys {
		sourceFK := m.getUint32FromRow(row, fkColumn)
		if sourceFK == 0 {
			// Foreign key is null or zero, skip
			continue
		}

		// Look up the target ID from the mapper
		targetFK := m.idMapper.Get(refTable, sourceFK)
		if targetFK == 0 {
			if meta.OptionalFKs[fkColumn] {
				virtualID, err := m.getOrCreateVirtualID(refTable, sourceFK)
				if err != nil {
					return fmt.Errorf("failed to generate virtual ID for optional FK %s=%d: %w",
						fkColumn, sourceFK, err)
				}
				row[fkColumn] = virtualID
				continue
			}
			return fmt.Errorf("foreign key %s=%d references %s, but no mapping found (referenced table may not have been migrated yet)",
				fkColumn, sourceFK, refTable)
		}
		row[fkColumn] = targetFK
	}
	return nil
}

// getOrCreateVirtualID generates a virtual ID for a deleted record.
// If a virtual ID was already generated for this source ID, it returns the same one.
func (m *MySQLMigrator) getOrCreateVirtualID(refTable string, sourceID uint32) (uint32, error) {
	if existingID := m.idMapper.Get(refTable, sourceID); existingID != 0 {
		return existingID, nil
	}

	newID, err := m.getNextID(refTable)
	if err != nil {
		return 0, err
	}

	m.idMapper.Set(refTable, sourceID, newID)
	return newID, nil
}

// convertJSONArrayFKs converts ID values inside JSON array columns using ID mapper
func (m *MySQLMigrator) convertJSONArrayFKs(row map[string]interface{}, meta TableMeta) error {
	if len(meta.JSONArrayFKs) == 0 {
		return nil
	}

	for jsonCol, refTable := range meta.JSONArrayFKs {
		rawVal, ok := row[jsonCol]
		if !ok || rawVal == nil {
			continue
		}

		sourceIDs, err := parseJSONUint32Array(rawVal)
		if err != nil {
			return fmt.Errorf("failed to parse JSON array column %s: %w", jsonCol, err)
		}

		if len(sourceIDs) == 0 {
			continue
		}

		targetIDs := make([]uint32, 0, len(sourceIDs))
		for _, sid := range sourceIDs {
			if sid == 0 {
				targetIDs = append(targetIDs, 0)
				continue
			}
			tid := m.idMapper.Get(refTable, sid)
			if tid == 0 {
				return fmt.Errorf("JSON array column %s: source ID %d references %s, but no mapping found",
					jsonCol, sid, refTable)
			}
			targetIDs = append(targetIDs, tid)
		}

		data, err := json.Marshal(targetIDs)
		if err != nil {
			return fmt.Errorf("failed to marshal converted IDs for column %s: %w", jsonCol, err)
		}
		row[jsonCol] = string(data)
	}
	return nil
}

// convertBindingsJSON converts IDs inside the complex bindings JSON column of app_template_bindings.
// The structure is: [{"template_set_id": N, "template_revisions": [{"template_id": N, "template_revision_id": N, "is_latest": bool}]}]
func (m *MySQLMigrator) convertBindingsJSON(row map[string]interface{}) error {
	rawVal, ok := row["bindings"]
	if !ok || rawVal == nil {
		return nil
	}

	rawBytes, err := toJSONBytes(rawVal)
	if err != nil {
		return fmt.Errorf("failed to read bindings column: %w", err)
	}

	var bindings []map[string]interface{}
	if unmarshalErr := json.Unmarshal(rawBytes, &bindings); unmarshalErr != nil {
		return fmt.Errorf("failed to parse bindings JSON: %w", unmarshalErr)
	}

	for i, binding := range bindings {
		if tsID, ok := binding["template_set_id"]; ok && tsID != nil {
			sourceID := jsonNumberToUint32(tsID)
			if sourceID != 0 {
				targetID := m.idMapper.Get("template_sets", sourceID)
				if targetID == 0 {
					return fmt.Errorf("bindings[%d].template_set_id=%d: no mapping found in template_sets", i, sourceID)
				}
				binding["template_set_id"] = targetID
			}
		}

		rawRevisions, ok := binding["template_revisions"]
		if !ok || rawRevisions == nil {
			continue
		}

		revisions, ok := rawRevisions.([]interface{})
		if !ok {
			continue
		}

		for j, rawRev := range revisions {
			rev, ok := rawRev.(map[string]interface{})
			if !ok {
				continue
			}

			if tmplID, ok := rev["template_id"]; ok && tmplID != nil {
				sourceID := jsonNumberToUint32(tmplID)
				if sourceID != 0 {
					targetID := m.idMapper.Get("templates", sourceID)
					if targetID == 0 {
						return fmt.Errorf("bindings[%d].template_revisions[%d].template_id=%d: no mapping found in templates",
							i, j, sourceID)
					}
					rev["template_id"] = targetID
				}
			}

			if trID, ok := rev["template_revision_id"]; ok && trID != nil {
				sourceID := jsonNumberToUint32(trID)
				if sourceID != 0 {
					targetID := m.idMapper.Get("template_revisions", sourceID)
					if targetID == 0 {
						return fmt.Errorf(
							"bindings[%d].template_revisions[%d].template_revision_id=%d: no mapping found in template_revisions",
							i, j, sourceID)
					}
					rev["template_revision_id"] = targetID
				}
			}
		}
	}

	data, err := json.Marshal(bindings)
	if err != nil {
		return fmt.Errorf("failed to marshal converted bindings: %w", err)
	}
	row["bindings"] = string(data)
	return nil
}

// parseJSONUint32Array parses a raw DB value ([]byte or string) as a JSON array of uint32
func parseJSONUint32Array(val interface{}) ([]uint32, error) {
	rawBytes, err := toJSONBytes(val)
	if err != nil {
		return nil, err
	}

	var numbers []json.Number
	if err := json.Unmarshal(rawBytes, &numbers); err != nil {
		var floats []float64
		if err2 := json.Unmarshal(rawBytes, &floats); err2 != nil {
			return nil, fmt.Errorf("cannot parse as number array: %w", err)
		}
		result := make([]uint32, len(floats))
		for i, f := range floats {
			result[i] = uint32(f)
		}
		return result, nil
	}

	result := make([]uint32, len(numbers))
	for i, n := range numbers {
		v, err := n.Int64()
		if err != nil {
			return nil, fmt.Errorf("invalid number at index %d: %w", i, err)
		}
		result[i] = uint32(v)
	}
	return result, nil
}

// toJSONBytes converts a raw DB value to JSON bytes
func toJSONBytes(val interface{}) ([]byte, error) {
	switch v := val.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return nil, fmt.Errorf("unsupported type %T for JSON conversion", val)
		}
		return data, nil
	}
}

// jsonNumberToUint32 converts a JSON-deserialized number value to uint32
func jsonNumberToUint32(val interface{}) uint32 {
	switch v := val.(type) {
	case float64:
		return uint32(v)
	case json.Number:
		n, _ := v.Int64()
		return uint32(n)
	case int64:
		return uint32(v)
	case uint32:
		return v
	case uint64:
		return uint32(v)
	default:
		return 0
	}
}

// hasTenantIDColumn checks if a table has tenant_id column
func (m *MySQLMigrator) hasTenantIDColumn(tableName string) bool {
	return m.hasColumn(tableName, "tenant_id", m.cfg.Target.MySQL.Database)
}

// hasBizIDColumn checks if a table has biz_id column
func (m *MySQLMigrator) hasBizIDColumn(tableName string) bool {
	return m.hasColumn(tableName, "biz_id", m.cfg.Source.MySQL.Database)
}

// hasColumn checks if a table has a specific column
func (m *MySQLMigrator) hasColumn(tableName, columnName, database string) bool {
	var count int64
	query := `SELECT COUNT(*) FROM information_schema.columns 
			  WHERE table_schema = ? AND table_name = ? AND column_name = ?`
	db := m.sourceDB
	if database == m.cfg.Target.MySQL.Database {
		db = m.targetDB
	}
	if err := db.Raw(query, database, tableName, columnName).Scan(&count).Error; err != nil {
		log.Printf("Warning: failed to check %s column for table %s: %v", columnName, tableName, err)
		return false
	}
	return count > 0
}

// detectBizIDColumn returns the biz ID column name for a table in the target database.
// Most tables use "biz_id", but some (e.g. biz_hosts) use "bk_biz_id".
// Returns empty string if the table doesn't exist or has no biz ID column.
func (m *MySQLMigrator) detectBizIDColumn(tableName string) string {
	if !m.tableExists(tableName, m.cfg.Target.MySQL.Database) {
		return ""
	}
	if m.hasColumn(tableName, "biz_id", m.cfg.Target.MySQL.Database) {
		return "biz_id"
	}
	if m.hasColumn(tableName, "bk_biz_id", m.cfg.Target.MySQL.Database) {
		return "bk_biz_id"
	}
	return ""
}

// tableExists checks if a table exists in the specified database
func (m *MySQLMigrator) tableExists(tableName, database string) bool {
	var count int64
	query := `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = ?`
	db := m.sourceDB
	if database == m.cfg.Target.MySQL.Database {
		db = m.targetDB
	}
	if err := db.Raw(query, database, tableName).Scan(&count).Error; err != nil {
		return false
	}
	return count > 0
}

// handleSpecialCases handles special type conversions and transformations
func (m *MySQLMigrator) handleSpecialCases(tableName string, row map[string]interface{}) {
	if tableName == "strategies" {
		// itsm_ticket_state_id: int -> string
		if stateID, ok := row["itsm_ticket_state_id"]; ok && stateID != nil {
			switch v := stateID.(type) {
			case int, int32, int64, uint, uint32, uint64:
				row["itsm_ticket_state_id"] = fmt.Sprintf("%v", v)
			case float64:
				row["itsm_ticket_state_id"] = fmt.Sprintf("%.0f", v)
			}
		}

		m.clearItsmFields(row)
	}

	// KV tables version reset:
	// When migrating to target Vault, the Put operation resets version to 1.
	// Therefore, the version field in MySQL must also be reset to 1.
	if tableName == "kvs" || tableName == "released_kvs" {
		row["version"] = uint32(1)
	}
}

// clearItsmFields handles ITSM ticket fields for strategies during cross-environment migration.
//
// Only pending strategies need processing. Completed strategies (already_publish, rejected_approval,
// etc.) are left untouched because:
//   - The UI does not display ITSM fields for these statuses (approveStatus = -1).
//   - Preserving the original approval record is harmless and keeps audit traceability.
//
// Pending strategies (pending_approval / pending_publish) are set to "revoked_publish" because:
//  1. The source ITSM ticket SN, callback URL, and workflow config do not work in the target env.
//  2. Keeping "pending_approval" without a real ITSM ticket would block new publishes
//     (SubmitPublishApprove rejects when an existing strategy is pending).
//  3. The UI would show a confusing "pending" spinner with no approval link.
//  4. "revoked_publish" is transient in the UI (disappears on page refresh) and does not
//     block new publish operations, allowing users to re-submit in the new environment.
func (m *MySQLMigrator) clearItsmFields(row map[string]interface{}) {
	publishStatus, _ := row["publish_status"].(string)
	if publishStatus != "pending_approval" && publishStatus != "pending_publish" {
		return
	}

	row["publish_status"] = "revoked_publish"
	row["approver_progress"] = ""
	row["itsm_ticket_type"] = ""
	row["itsm_ticket_url"] = ""
	row["itsm_ticket_sn"] = ""
	row["itsm_ticket_status"] = ""
	row["itsm_ticket_state_id"] = ""
}

// GetSourceDB returns the source database connection
func (m *MySQLMigrator) GetSourceDB() *gorm.DB {
	return m.sourceDB
}

// GetTargetDB returns the target database connection
func (m *MySQLMigrator) GetTargetDB() *gorm.DB {
	return m.targetDB
}

// CleanupTarget clears all migrated data from target database
// If biz_id filter is configured, only clears data for those businesses
func (m *MySQLMigrator) CleanupTarget() (*CleanupResult, error) {
	startTime := time.Now()
	result := &CleanupResult{
		Success: true,
	}

	// Log biz_id filter info
	if m.cfg.Migration.HasBizFilter() {
		log.Printf("MySQL cleanup with biz_id filter: %v", m.cfg.Migration.BizIDs)
	}

	// Disable foreign key checks
	if err := m.targetDB.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
		return nil, fmt.Errorf("failed to disable foreign key checks: %w", err)
	}
	defer func() {
		if err := m.targetDB.Exec("SET FOREIGN_KEY_CHECKS = 1").Error; err != nil {
			log.Printf("Warning: failed to re-enable foreign key checks: %v", err)
		}
	}()

	log.Println("Cleaning up target database...")

	// Step 1: Clean runtime tables (audits, events, clients, etc.)
	// These are not migrated but accumulate data in the target env and must be cleaned per biz_id.
	log.Println("  Cleaning runtime tables...")
	for _, tableName := range RuntimeCleanupTables() {
		tableResult := m.cleanupTable(tableName)
		result.TableResults = append(result.TableResults, tableResult)

		if !tableResult.Success {
			result.Success = false
			if !m.cfg.Migration.ContinueOnError {
				break
			}
		}
	}

	// Step 2: Clean core migration tables in reverse dependency order
	if result.Success || m.cfg.Migration.ContinueOnError {
		log.Println("  Cleaning core tables...")
		for _, tableName := range TablesInCleanupOrder() {
			if m.cfg.ShouldSkipTable(tableName) {
				continue
			}

			tableResult := m.cleanupTable(tableName)
			result.TableResults = append(result.TableResults, tableResult)

			if !tableResult.Success {
				result.Success = false
				if !m.cfg.Migration.ContinueOnError {
					break
				}
			}
		}
	}

	result.Duration = time.Since(startTime)
	log.Printf("Cleanup completed in %v", result.Duration)

	return result, nil
}

// cleanupTable deletes all records from a single table
// If biz_id filter is configured, only delete records for those businesses
func (m *MySQLMigrator) cleanupTable(tableName string) TableCleanupResult {
	result := TableCleanupResult{
		TableName: tableName,
		Success:   true,
	}

	if !m.tableExists(tableName, m.cfg.Target.MySQL.Database) {
		log.Printf("  Table %s does not exist, skipping", tableName)
		return result
	}

	// Detect the biz ID column name: most tables use "biz_id", biz_hosts uses "bk_biz_id"
	bizIDColumn := m.detectBizIDColumn(tableName)
	hasBizFilter := m.cfg.Migration.HasBizFilter()

	// Build base query with biz_id filter if applicable
	baseQuery := m.targetDB.Table(tableName)
	if bizIDColumn != "" && hasBizFilter {
		baseQuery = baseQuery.Where(bizIDColumn+" IN ?", m.cfg.Migration.BizIDs)
	}

	// Count records before deletion
	var count int64
	if err := baseQuery.Count(&count).Error; err != nil {
		result.Error = fmt.Sprintf("failed to count records: %v", err)
		result.Success = false
		return result
	}
	result.DeletedCount = count

	if count == 0 {
		log.Printf("  Table %s is empty (or no matching records), skipping", tableName)
		return result
	}

	// Delete records
	if bizIDColumn != "" && hasBizFilter {
		if err := m.targetDB.Table(tableName).Where(bizIDColumn+" IN ?", m.cfg.Migration.BizIDs).
			Delete(nil).Error; err != nil {
			result.Error = fmt.Sprintf("failed to delete records: %v", err)
			result.Success = false
			return result
		}
		log.Printf("  Deleted %d records from table %s (%s filter: %v)",
			count, tableName, bizIDColumn, m.cfg.Migration.BizIDs)
	} else {
		// Delete all records using TRUNCATE for better performance
		// Use backticks to handle reserved keywords like 'groups'
		if err := m.targetDB.Exec(fmt.Sprintf("TRUNCATE TABLE `%s`", tableName)).Error; err != nil {
			// If TRUNCATE fails (e.g., due to foreign keys), try DELETE
			if err := m.targetDB.Exec(fmt.Sprintf("DELETE FROM `%s`", tableName)).Error; err != nil {
				result.Error = fmt.Sprintf("failed to delete records: %v", err)
				result.Success = false
				return result
			}
		}
		log.Printf("  Deleted %d records from table %s", count, tableName)
	}

	return result
}

// CleanupResult contains the result of cleanup operation
type CleanupResult struct {
	TableResults []TableCleanupResult
	Duration     time.Duration
	Success      bool
}

// logFailedRow collects a failed row for later file output
func (m *MySQLMigrator) logFailedRow(tableName string, sourceID, newID uint32, row map[string]interface{}, errMsg string) {
	bizID := m.getUint32FromRow(row, "biz_id")

	rowCopy := make(map[string]interface{}, len(row))
	for k, v := range row {
		rowCopy[k] = v
	}

	m.failedRows = append(m.failedRows, FailedRow{
		Table:    tableName,
		SourceID: sourceID,
		NewID:    newID,
		BizID:    bizID,
		Error:    errMsg,
		Data:     rowCopy,
	})
}

// HasFailedRows returns true if there are any collected failed rows
func (m *MySQLMigrator) HasFailedRows() bool {
	return len(m.failedRows) > 0
}

// WriteFailedRowsLog writes all collected failed rows to a JSON log file, grouped by biz_id.
// logDir specifies the directory (e.g. "logs/biz_100_200"), file is named migrate_<timestamp>.json.
func (m *MySQLMigrator) WriteFailedRowsLog(logDir string) (string, error) {
	if len(m.failedRows) == 0 {
		return "", nil
	}

	grouped := make(map[uint32][]FailedRow)
	for _, r := range m.failedRows {
		grouped[r.BizID] = append(grouped[r.BizID], r)
	}

	type bizFailedRows struct {
		BizID      uint32      `json:"biz_id"`
		Count      int         `json:"count"`
		FailedRows []FailedRow `json:"failed_rows"`
	}

	output := struct {
		Timestamp   string          `json:"timestamp"`
		TotalFailed int             `json:"total_failed"`
		Businesses  []bizFailedRows `json:"businesses"`
	}{
		Timestamp:   time.Now().Format(time.RFC3339),
		TotalFailed: len(m.failedRows),
	}

	for bizID, rows := range grouped {
		output.Businesses = append(output.Businesses, bizFailedRows{
			BizID:      bizID,
			Count:      len(rows),
			FailedRows: rows,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal failed rows: %w", err)
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create log directory %s: %w", logDir, err)
	}

	filename := fmt.Sprintf("migrate_%s.json", time.Now().Format("20060102_150405"))
	fullPath := logDir + "/" + filename
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write failed rows log: %w", err)
	}

	return fullPath, nil
}

// TableCleanupResult contains the result of cleaning up a single table
type TableCleanupResult struct {
	TableName    string
	DeletedCount int64
	Error        string
	Success      bool
}
