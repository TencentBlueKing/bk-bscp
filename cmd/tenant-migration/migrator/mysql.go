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

	"github.com/TencentBlueKing/bk-bscp/cmd/tenant-migration/config"
)

// MySQLMigrator handles MySQL data migration
type MySQLMigrator struct {
	cfg      *config.Config
	sourceDB *gorm.DB
	targetDB *gorm.DB
	idMapper *IDMapper
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
		if err := batchQuery.Offset(offset).Limit(batchSize).Find(&rows).Error; err != nil {
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
				result.Errors = append(result.Errors, fmt.Sprintf("failed to get next ID: %v", err))
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
				result.Errors = append(result.Errors, fmt.Sprintf("failed to convert foreign keys (sourceID=%d): %v", sourceID, err))
				if !m.cfg.Migration.ContinueOnError {
					result.Success = false
					break
				}
				continue
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
				result.Errors = append(result.Errors, fmt.Sprintf("failed to insert row (sourceID=%d, newID=%d): %v", sourceID, newID, err))
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
			return fmt.Errorf("foreign key %s=%d references %s, but no mapping found (referenced table may not have been migrated yet)",
				fkColumn, sourceFK, refTable)
		}
		row[fkColumn] = targetFK
	}
	return nil
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
	}

	// KV tables version reset:
	// When migrating to target Vault, the Put operation resets version to 1.
	// Therefore, the version field in MySQL must also be reset to 1.
	if tableName == "kvs" || tableName == "released_kvs" {
		row["version"] = uint32(1)
	}
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

	// Use proper cleanup order
	cleanupTables := TablesInCleanupOrder()

	log.Println("Cleaning up target database...")

	for _, tableName := range cleanupTables {
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

	// Check if table has biz_id column for filtering
	hasBizID := m.hasColumn(tableName, "biz_id", m.cfg.Target.MySQL.Database)
	hasBizFilter := m.cfg.Migration.HasBizFilter()

	// Build base query with biz_id filter if applicable
	baseQuery := m.targetDB.Table(tableName)
	if hasBizID && hasBizFilter {
		baseQuery = baseQuery.Where("biz_id IN ?", m.cfg.Migration.BizIDs)
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
		log.Printf("  Table %s is empty (or no matching biz_id), skipping", tableName)
		return result
	}

	// Delete records
	if hasBizID && hasBizFilter {
		// Delete only records matching the biz_id filter
		if err := m.targetDB.Table(tableName).Where("biz_id IN ?", m.cfg.Migration.BizIDs).Delete(nil).Error; err != nil {
			result.Error = fmt.Sprintf("failed to delete records: %v", err)
			result.Success = false
			return result
		}
		log.Printf("  Deleted %d records from table %s (biz_id filter: %v)", count, tableName, m.cfg.Migration.BizIDs)
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

// TableCleanupResult contains the result of cleaning up a single table
type TableCleanupResult struct {
	TableName    string
	DeletedCount int64
	Error        string
	Success      bool
}
