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
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/cmd/tenant-migration/config"
)

// Validator handles data validation after migration
type Validator struct {
	cfg      *config.Config
	sourceDB *gorm.DB
	targetDB *gorm.DB
}

// ValidationReport contains the validation results
type ValidationReport struct {
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
	TableResults []TableValidationResult
	Errors       []string
	Success      bool
}

// TableValidationResult contains the validation result for a single table
type TableValidationResult struct {
	TableName   string
	SourceCount int64
	TargetCount int64
	Match       bool
	TenantIDSet bool
	Errors      []string
}

// NewValidator creates a new Validator instance
func NewValidator(cfg *config.Config, sourceDB, targetDB *gorm.DB) *Validator {
	return &Validator{
		cfg:      cfg,
		sourceDB: sourceDB,
		targetDB: targetDB,
	}
}

// Validate performs validation on migrated data
func (v *Validator) Validate() (*ValidationReport, error) {
	startTime := time.Now()
	report := &ValidationReport{
		StartTime: startTime,
		Success:   true,
	}

	coreTables := config.CoreTables()

	log.Println("Starting data validation...")
	if v.cfg.Migration.HasBizFilter() {
		log.Printf("Validation with biz_id filter: %v", v.cfg.Migration.BizIDs)
	}

	for _, tableName := range coreTables {
		if v.cfg.ShouldSkipTable(tableName) {
			continue
		}

		result := v.validateTable(tableName)
		report.TableResults = append(report.TableResults, result)

		if !result.Match || !result.TenantIDSet {
			report.Success = false
		}
	}

	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(report.StartTime)

	log.Printf("Validation completed in %v", report.Duration)
	return report, nil
}

// validateTable validates a single table
func (v *Validator) validateTable(tableName string) TableValidationResult {
	result := TableValidationResult{
		TableName: tableName,
		Match:     true,
	}

	// Check if table has biz_id column for filtering
	hasBizIDSource := v.hasBizIDColumn(tableName, v.sourceDB, v.cfg.Source.MySQL.Database)
	hasBizIDTarget := v.hasBizIDColumn(tableName, v.targetDB, v.cfg.Target.MySQL.Database)
	hasBizFilter := v.cfg.Migration.HasBizFilter()

	// Build source query with biz_id filter if applicable
	sourceQuery := v.sourceDB.Table(tableName)
	if hasBizIDSource && hasBizFilter {
		sourceQuery = sourceQuery.Where("biz_id IN ?", v.cfg.Migration.BizIDs)
	}

	// Get source count
	var sourceCount int64
	if err := sourceQuery.Count(&sourceCount).Error; err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to count source records: %v", err))
		result.Match = false
		return result
	}
	result.SourceCount = sourceCount

	// Build target query with biz_id filter if applicable
	targetQuery := v.targetDB.Table(tableName)
	if hasBizIDTarget && hasBizFilter {
		targetQuery = targetQuery.Where("biz_id IN ?", v.cfg.Migration.BizIDs)
	}

	// Get target count
	var targetCount int64
	if err := targetQuery.Count(&targetCount).Error; err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to count target records: %v", err))
		result.Match = false
		return result
	}
	result.TargetCount = targetCount

	// Check if counts match
	if sourceCount != targetCount {
		result.Match = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("count mismatch: source=%d, target=%d", sourceCount, targetCount))
	}

	// Check if tenant_id is set correctly (for tables that have it)
	if v.hasTenantIDColumn(tableName) {
		var nullCount int64
		// Build query with biz_id filter if applicable
		if hasBizIDTarget && hasBizFilter {
			query := fmt.Sprintf("SELECT COUNT(*) FROM `%s` WHERE (tenant_id IS NULL OR tenant_id = '') AND biz_id IN ?", tableName)
			if err := v.targetDB.Raw(query, v.cfg.Migration.BizIDs).Scan(&nullCount).Error; err != nil {
				result.Errors = append(result.Errors,
					fmt.Sprintf("failed to check tenant_id: %v", err))
			}
		} else {
			query := fmt.Sprintf("SELECT COUNT(*) FROM `%s` WHERE tenant_id IS NULL OR tenant_id = ''", tableName)
			if err := v.targetDB.Raw(query).Scan(&nullCount).Error; err != nil {
				result.Errors = append(result.Errors,
					fmt.Sprintf("failed to check tenant_id: %v", err))
			}
		}

		if nullCount > 0 {
			result.TenantIDSet = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("%d records have NULL or empty tenant_id", nullCount))
		} else {
			result.TenantIDSet = true
		}

		// Verify tenant_id value matches configuration
		var mismatchCount int64
		if hasBizIDTarget && hasBizFilter {
			query := fmt.Sprintf("SELECT COUNT(*) FROM `%s` WHERE tenant_id != ? AND biz_id IN ?", tableName)
			if err := v.targetDB.Raw(query, v.cfg.Migration.TargetTenantID, v.cfg.Migration.BizIDs).Scan(&mismatchCount).Error; err != nil {
				result.Errors = append(result.Errors,
					fmt.Sprintf("failed to verify tenant_id value: %v", err))
			}
		} else {
			query := fmt.Sprintf("SELECT COUNT(*) FROM `%s` WHERE tenant_id != ?", tableName)
			if err := v.targetDB.Raw(query, v.cfg.Migration.TargetTenantID).Scan(&mismatchCount).Error; err != nil {
				result.Errors = append(result.Errors,
					fmt.Sprintf("failed to verify tenant_id value: %v", err))
			}
		}

		if mismatchCount > 0 {
			result.Errors = append(result.Errors,
				fmt.Sprintf("%d records have incorrect tenant_id (expected: %s)",
					mismatchCount, v.cfg.Migration.TargetTenantID))
		}
	} else {
		result.TenantIDSet = true // N/A for tables without tenant_id
	}

	log.Printf("  Table %s: source=%d, target=%d, match=%v, tenant_id=%v",
		tableName, result.SourceCount, result.TargetCount, result.Match, result.TenantIDSet)

	return result
}

// hasTenantIDColumn checks if a table has tenant_id column
func (v *Validator) hasTenantIDColumn(tableName string) bool {
	return v.hasColumn(tableName, "tenant_id", v.cfg.Target.MySQL.Database, v.targetDB)
}

// hasBizIDColumn checks if a table has biz_id column
func (v *Validator) hasBizIDColumn(tableName string, db *gorm.DB, database string) bool {
	return v.hasColumn(tableName, "biz_id", database, db)
}

// hasColumn checks if a table has a specific column
func (v *Validator) hasColumn(tableName, columnName, database string, db *gorm.DB) bool {
	var count int64
	query := `SELECT COUNT(*) FROM information_schema.columns 
			  WHERE table_schema = ? AND table_name = ? AND column_name = ?`
	if err := db.Raw(query, database, tableName, columnName).Scan(&count).Error; err != nil {
		return false
	}
	return count > 0
}

// PrintReport prints the validation report to stdout
func (v *Validator) PrintReport(report *ValidationReport) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("VALIDATION REPORT")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Start Time:  %s\n", report.StartTime.Format(time.RFC3339))
	fmt.Printf("End Time:    %s\n", report.EndTime.Format(time.RFC3339))
	fmt.Printf("Duration:    %v\n", report.Duration)
	fmt.Printf("Status:      %s\n", boolToStatus(report.Success))
	fmt.Println()

	fmt.Println("Table Validation Results:")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("%-30s %10s %10s %8s %8s\n", "Table", "Source", "Target", "Match", "TenantID")
	fmt.Println(strings.Repeat("-", 60))

	for _, tr := range report.TableResults {
		matchStr := "YES"
		if !tr.Match {
			matchStr = "NO"
		}
		tenantStr := "YES"
		if !tr.TenantIDSet {
			tenantStr = "NO"
		}
		fmt.Printf("%-30s %10d %10d %8s %8s\n",
			tr.TableName, tr.SourceCount, tr.TargetCount, matchStr, tenantStr)
	}
	fmt.Println()

	// Print errors if any
	hasErrors := false
	for _, tr := range report.TableResults {
		if len(tr.Errors) > 0 {
			if !hasErrors {
				fmt.Println("Validation Errors:")
				fmt.Println(strings.Repeat("-", 60))
				hasErrors = true
			}
			fmt.Printf("Table: %s\n", tr.TableName)
			for _, err := range tr.Errors {
				fmt.Printf("  - %s\n", err)
			}
		}
	}

	if hasErrors {
		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 60))
}
