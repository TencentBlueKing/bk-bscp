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

// Scanner handles asset scanning for source and target databases
type Scanner struct {
	cfg      *config.Config
	sourceDB *gorm.DB
	targetDB *gorm.DB
}

// ScanReport contains the complete scan results
type ScanReport struct {
	StartTime     time.Time
	EndTime       time.Time
	Duration      time.Duration
	SourceSummary DatabaseSummary
	TargetSummary DatabaseSummary
	TableDetails  []TableScanResult
	VaultSummary  *VaultScanSummary
	Comparison    ScanComparison
	FilteredByBiz bool
	BizIDs        []uint32
	ScanMode      string // "all" or "configured"
}

// DatabaseSummary contains summary statistics for a database
type DatabaseSummary struct {
	DatabaseName string
	TableCount   int
	TotalRecords int64
}

// TableScanResult contains scan results for a single table
type TableScanResult struct {
	TableName    string
	SourceCount  int64
	TargetCount  int64
	Difference   int64
	HasBizID     bool
	HasTenantID  bool
	SourceExists bool
	TargetExists bool
}

// VaultScanSummary contains Vault scan summary
type VaultScanSummary struct {
	SourceKvCount         int64
	SourceReleasedKvCount int64
	TargetKvCount         int64
	TargetReleasedKvCount int64
}

// ScanComparison contains comparison statistics
type ScanComparison struct {
	TablesOnlyInSource []string
	TablesOnlyInTarget []string
	TablesWithMoreData []string // Source has more data
	TablesWithLessData []string // Target has more data
	MatchingTables     []string // Same record count
	TotalDifference    int64
}

// NewScanner creates a new Scanner instance
func NewScanner(cfg *config.Config, sourceDB, targetDB *gorm.DB) *Scanner {
	return &Scanner{
		cfg:      cfg,
		sourceDB: sourceDB,
		targetDB: targetDB,
	}
}

// Scan performs asset scanning on source and target databases (configured tables only)
func (s *Scanner) Scan() (*ScanReport, error) {
	return s.ScanConfigured()
}

// ScanConfigured scans only the tables configured for migration
func (s *Scanner) ScanConfigured() (*ScanReport, error) {
	coreTables := s.cfg.GetTablesToMigrate()
	tablesToScan := make([]string, 0, len(coreTables))
	for _, t := range coreTables {
		if !s.cfg.ShouldSkipTable(t) {
			tablesToScan = append(tablesToScan, t)
		}
	}
	return s.scanTables(tablesToScan, "configured")
}

// ScanAll scans all tables in both source and target databases
func (s *Scanner) ScanAll() (*ScanReport, error) {
	// Get all tables from both databases
	sourceTables := s.getAllTables(s.sourceDB, s.cfg.Source.MySQL.Database)
	targetTables := s.getAllTables(s.targetDB, s.cfg.Target.MySQL.Database)

	// Merge tables (union of both)
	tableSet := make(map[string]bool)
	for _, t := range sourceTables {
		tableSet[t] = true
	}
	for _, t := range targetTables {
		tableSet[t] = true
	}

	allTables := make([]string, 0, len(tableSet))
	for t := range tableSet {
		allTables = append(allTables, t)
	}

	// Sort tables for consistent output
	sortStrings(allTables)

	return s.scanTables(allTables, "all")
}

// scanTables scans a list of tables and returns the report
func (s *Scanner) scanTables(tables []string, mode string) (*ScanReport, error) {
	startTime := time.Now()
	report := &ScanReport{
		StartTime:     startTime,
		FilteredByBiz: s.cfg.Migration.HasBizFilter(),
		BizIDs:        s.cfg.Migration.BizIDs,
		ScanMode:      mode,
	}

	log.Printf("Starting asset scan (mode: %s)...", mode)
	if report.FilteredByBiz {
		log.Printf("Scanning with biz_id filter: %v", report.BizIDs)
	}

	// Set database names
	report.SourceSummary.DatabaseName = s.cfg.Source.MySQL.Database
	report.TargetSummary.DatabaseName = s.cfg.Target.MySQL.Database

	log.Printf("Scanning %d tables...", len(tables))

	// Scan each table
	for _, tableName := range tables {
		result := s.scanTable(tableName)
		report.TableDetails = append(report.TableDetails, result)

		// Update summaries
		if result.SourceExists {
			report.SourceSummary.TableCount++
			report.SourceSummary.TotalRecords += result.SourceCount
		}
		if result.TargetExists {
			report.TargetSummary.TableCount++
			report.TargetSummary.TotalRecords += result.TargetCount
		}

		// Build comparison
		s.updateComparison(&report.Comparison, result)
	}

	// Scan Vault if configured
	if s.cfg.Source.Vault.Address != "" || s.cfg.Target.Vault.Address != "" {
		report.VaultSummary = s.scanVaultFromDB()
	}

	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(report.StartTime)

	log.Printf("Asset scan completed in %v", report.Duration)
	return report, nil
}

// getAllTables retrieves all table names from a database
func (s *Scanner) getAllTables(db *gorm.DB, database string) []string {
	var tables []string
	query := `SELECT table_name FROM information_schema.tables 
			  WHERE table_schema = ? AND table_type = 'BASE TABLE'
			  ORDER BY table_name`
	if err := db.Raw(query, database).Scan(&tables).Error; err != nil {
		log.Printf("Warning: failed to get tables from database %s: %v", database, err)
		return nil
	}
	return tables
}

// sortStrings sorts a string slice in place
func sortStrings(s []string) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// scanTable scans a single table in both source and target databases
func (s *Scanner) scanTable(tableName string) TableScanResult {
	result := TableScanResult{
		TableName: tableName,
	}

	// Check if table has biz_id and tenant_id columns
	result.HasBizID = s.hasBizIDColumn(tableName, s.sourceDB, s.cfg.Source.MySQL.Database)
	result.HasTenantID = s.hasTenantIDColumn(tableName, s.targetDB, s.cfg.Target.MySQL.Database)

	hasBizFilter := s.cfg.Migration.HasBizFilter()

	// Check source table existence and count
	result.SourceExists = s.tableExists(tableName, s.sourceDB, s.cfg.Source.MySQL.Database)
	if result.SourceExists {
		sourceQuery := s.sourceDB.Table(tableName)
		if result.HasBizID && hasBizFilter {
			sourceQuery = sourceQuery.Where("biz_id IN ?", s.cfg.Migration.BizIDs)
		}
		if err := sourceQuery.Count(&result.SourceCount).Error; err != nil {
			log.Printf("  Warning: failed to count source table %s: %v", tableName, err)
		}
	}

	// Check target table existence and count
	result.TargetExists = s.tableExists(tableName, s.targetDB, s.cfg.Target.MySQL.Database)
	if result.TargetExists {
		targetQuery := s.targetDB.Table(tableName)
		// For target, also check biz_id column existence in target DB
		hasBizIDTarget := s.hasBizIDColumn(tableName, s.targetDB, s.cfg.Target.MySQL.Database)
		if hasBizIDTarget && hasBizFilter {
			targetQuery = targetQuery.Where("biz_id IN ?", s.cfg.Migration.BizIDs)
		}
		if err := targetQuery.Count(&result.TargetCount).Error; err != nil {
			log.Printf("  Warning: failed to count target table %s: %v", tableName, err)
		}
	}

	result.Difference = result.SourceCount - result.TargetCount

	log.Printf("  Table %s: source=%d, target=%d, diff=%d",
		tableName, result.SourceCount, result.TargetCount, result.Difference)

	return result
}

// scanVaultFromDB scans Vault KV counts from database records
func (s *Scanner) scanVaultFromDB() *VaultScanSummary {
	summary := &VaultScanSummary{}
	hasBizFilter := s.cfg.Migration.HasBizFilter()

	// Count source KVs
	sourceKvQuery := s.sourceDB.Table("kvs")
	if hasBizFilter {
		sourceKvQuery = sourceKvQuery.Where("biz_id IN ?", s.cfg.Migration.BizIDs)
	}
	if err := sourceKvQuery.Count(&summary.SourceKvCount).Error; err != nil {
		log.Printf("  Warning: failed to count source kvs: %v", err)
	}

	// Count source released KVs
	sourceRKvQuery := s.sourceDB.Table("released_kvs")
	if hasBizFilter {
		sourceRKvQuery = sourceRKvQuery.Where("biz_id IN ?", s.cfg.Migration.BizIDs)
	}
	if err := sourceRKvQuery.Count(&summary.SourceReleasedKvCount).Error; err != nil {
		log.Printf("  Warning: failed to count source released_kvs: %v", err)
	}

	// Count target KVs
	targetKvQuery := s.targetDB.Table("kvs")
	if hasBizFilter {
		targetKvQuery = targetKvQuery.Where("biz_id IN ?", s.cfg.Migration.BizIDs)
	}
	if err := targetKvQuery.Count(&summary.TargetKvCount).Error; err != nil {
		log.Printf("  Warning: failed to count target kvs: %v", err)
	}

	// Count target released KVs
	targetRKvQuery := s.targetDB.Table("released_kvs")
	if hasBizFilter {
		targetRKvQuery = targetRKvQuery.Where("biz_id IN ?", s.cfg.Migration.BizIDs)
	}
	if err := targetRKvQuery.Count(&summary.TargetReleasedKvCount).Error; err != nil {
		log.Printf("  Warning: failed to count target released_kvs: %v", err)
	}

	log.Printf("  Vault KVs - Source: %d unreleased, %d released | Target: %d unreleased, %d released",
		summary.SourceKvCount, summary.SourceReleasedKvCount,
		summary.TargetKvCount, summary.TargetReleasedKvCount)

	return summary
}

// updateComparison updates the comparison statistics
func (s *Scanner) updateComparison(comp *ScanComparison, result TableScanResult) {
	if result.SourceExists && !result.TargetExists {
		comp.TablesOnlyInSource = append(comp.TablesOnlyInSource, result.TableName)
	} else if !result.SourceExists && result.TargetExists {
		comp.TablesOnlyInTarget = append(comp.TablesOnlyInTarget, result.TableName)
	} else if result.SourceExists && result.TargetExists {
		if result.Difference > 0 {
			comp.TablesWithMoreData = append(comp.TablesWithMoreData, result.TableName)
		} else if result.Difference < 0 {
			comp.TablesWithLessData = append(comp.TablesWithLessData, result.TableName)
		} else {
			comp.MatchingTables = append(comp.MatchingTables, result.TableName)
		}
	}
	comp.TotalDifference += result.Difference
}

// tableExists checks if a table exists in the database
func (s *Scanner) tableExists(tableName string, db *gorm.DB, database string) bool {
	var count int64
	query := `SELECT COUNT(*) FROM information_schema.tables 
			  WHERE table_schema = ? AND table_name = ?`
	if err := db.Raw(query, database, tableName).Scan(&count).Error; err != nil {
		return false
	}
	return count > 0
}

// hasBizIDColumn checks if a table has biz_id column
func (s *Scanner) hasBizIDColumn(tableName string, db *gorm.DB, database string) bool {
	return s.hasColumn(tableName, "biz_id", db, database)
}

// hasTenantIDColumn checks if a table has tenant_id column
func (s *Scanner) hasTenantIDColumn(tableName string, db *gorm.DB, database string) bool {
	return s.hasColumn(tableName, "tenant_id", db, database)
}

// hasColumn checks if a table has a specific column
func (s *Scanner) hasColumn(tableName, columnName string, db *gorm.DB, database string) bool {
	var count int64
	query := `SELECT COUNT(*) FROM information_schema.columns 
			  WHERE table_schema = ? AND table_name = ? AND column_name = ?`
	if err := db.Raw(query, database, tableName, columnName).Scan(&count).Error; err != nil {
		return false
	}
	return count > 0
}

// PrintReport prints the scan report to stdout
func (s *Scanner) PrintReport(report *ScanReport) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("ASSET SCAN REPORT")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Scan Time:   %s\n", report.StartTime.Format(time.RFC3339))
	fmt.Printf("Duration:    %v\n", report.Duration)
	scanModeDesc := "configured tables"
	if report.ScanMode == "all" {
		scanModeDesc = "all tables in database"
	}
	fmt.Printf("Scan Mode:   %s\n", scanModeDesc)
	if report.FilteredByBiz {
		fmt.Printf("Biz Filter:  %v\n", report.BizIDs)
	}
	fmt.Println()

	// Database summaries
	fmt.Println("DATABASE SUMMARY")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("%-20s %-30s %-30s\n", "", "Source", "Target")
	fmt.Printf("%-20s %-30s %-30s\n", "Database", report.SourceSummary.DatabaseName, report.TargetSummary.DatabaseName)
	fmt.Printf("%-20s %-30d %-30d\n", "Tables", report.SourceSummary.TableCount, report.TargetSummary.TableCount)
	fmt.Printf("%-20s %-30d %-30d\n", "Total Records", report.SourceSummary.TotalRecords, report.TargetSummary.TotalRecords)
	fmt.Println()

	// Table details
	fmt.Println("TABLE DETAILS")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("%-35s %12s %12s %12s %8s\n", "Table", "Source", "Target", "Difference", "Status")
	fmt.Println(strings.Repeat("-", 80))

	for _, t := range report.TableDetails {
		status := "✓"
		if t.Difference != 0 {
			status = "≠"
		}
		if !t.SourceExists {
			status = "S-"
		}
		if !t.TargetExists {
			status = "T-"
		}

		diffStr := fmt.Sprintf("%+d", t.Difference)
		if t.Difference == 0 {
			diffStr = "0"
		}

		fmt.Printf("%-35s %12d %12d %12s %8s\n",
			t.TableName, t.SourceCount, t.TargetCount, diffStr, status)
	}
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("%-35s %12d %12d %+12d\n",
		"TOTAL", report.SourceSummary.TotalRecords, report.TargetSummary.TotalRecords, report.Comparison.TotalDifference)
	fmt.Println()

	// Vault summary
	if report.VaultSummary != nil {
		fmt.Println("VAULT KV SUMMARY")
		fmt.Println(strings.Repeat("-", 80))
		fmt.Printf("%-35s %12s %12s\n", "", "Source", "Target")
		fmt.Printf("%-35s %12d %12d\n", "Unreleased KVs (kvs table)",
			report.VaultSummary.SourceKvCount, report.VaultSummary.TargetKvCount)
		fmt.Printf("%-35s %12d %12d\n", "Released KVs (released_kvs table)",
			report.VaultSummary.SourceReleasedKvCount, report.VaultSummary.TargetReleasedKvCount)
		fmt.Println()
	}

	// Comparison summary
	fmt.Println("COMPARISON SUMMARY")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("Tables with matching records:    %d\n", len(report.Comparison.MatchingTables))
	fmt.Printf("Tables with more data in source: %d\n", len(report.Comparison.TablesWithMoreData))
	fmt.Printf("Tables with more data in target: %d\n", len(report.Comparison.TablesWithLessData))

	if len(report.Comparison.TablesOnlyInSource) > 0 {
		fmt.Printf("Tables only in source:           %s\n", strings.Join(report.Comparison.TablesOnlyInSource, ", "))
	}
	if len(report.Comparison.TablesOnlyInTarget) > 0 {
		fmt.Printf("Tables only in target:           %s\n", strings.Join(report.Comparison.TablesOnlyInTarget, ", "))
	}

	fmt.Println()

	// Status legend
	fmt.Println("STATUS LEGEND")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Println("  ✓  = Record counts match")
	fmt.Println("  ≠  = Record counts differ")
	fmt.Println("  S- = Table missing in source")
	fmt.Println("  T- = Table missing in target")
	fmt.Println()

	fmt.Println(strings.Repeat("=", 80))
}
