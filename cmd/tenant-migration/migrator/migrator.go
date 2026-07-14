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

// Package migrator provides the core migration logic for tenant migration
package migrator

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/TencentBlueKing/bk-bscp/cmd/tenant-migration/config"
)

// Migrator is the main migration controller
type Migrator struct {
	cfg           *config.Config
	mysqlMigrator *MySQLMigrator
	vaultMigrator *VaultMigrator
	validator     *Validator
	scanner       *Scanner
}

// MigrationReport contains the migration results
type MigrationReport struct {
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
	TotalTables  int
	TableResults []TableMigrationResult
	VaultResults *VaultMigrationResult
	Errors       []string
	Success      bool
}

// TableMigrationResult contains the result of migrating a single table
type TableMigrationResult struct {
	TableName     string
	SourceCount   int64
	MigratedCount int64
	SkippedCount  int64
	ErrorCount    int64
	Duration      time.Duration
	Errors        []string
	Success       bool
}

// VaultMigrationResult contains the result of migrating Vault data
type VaultMigrationResult struct {
	KvCount         int64
	ReleasedKvCount int64
	MigratedKvs     int64
	MigratedRKvs    int64
	Duration        time.Duration
	Errors          []string
	Success         bool
}

// NewMigrator creates a new Migrator instance
func NewMigrator(cfg *config.Config) (*Migrator, error) {
	mysqlMigrator, err := NewMySQLMigrator(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create MySQL migrator: %w", err)
	}

	var vaultMigrator *VaultMigrator
	if cfg.Source.Vault.Address != "" && cfg.Target.Vault.Address != "" {
		vaultMigrator, err = NewVaultMigrator(cfg, mysqlMigrator.sourceDB, mysqlMigrator.targetDB)
		if err != nil {
			return nil, fmt.Errorf("failed to create Vault migrator: %w", err)
		}
	}

	validator := NewValidator(cfg, mysqlMigrator.sourceDB, mysqlMigrator.targetDB)
	scanner := NewScanner(cfg, mysqlMigrator.sourceDB, mysqlMigrator.targetDB, vaultMigrator)

	return &Migrator{
		cfg:           cfg,
		mysqlMigrator: mysqlMigrator,
		vaultMigrator: vaultMigrator,
		validator:     validator,
		scanner:       scanner,
	}, nil
}

// Close closes all connections
func (m *Migrator) Close() error {
	if m.mysqlMigrator != nil {
		if err := m.mysqlMigrator.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Run runs the full migration (MySQL + Vault)
func (m *Migrator) Run() (*MigrationReport, error) {
	report := &MigrationReport{
		StartTime: time.Now(),
		Success:   true,
	}

	log.Println("Starting full migration (MySQL + Vault)...")
	log.Println("Note: Run 'cleanup' command first if you need to clear existing data for the target business")

	// MySQL migration
	log.Println("MySQL migration...")
	tableResults, err := m.mysqlMigrator.Migrate()
	report.TableResults = tableResults
	report.TotalTables = len(tableResults)

	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("MySQL migration error: %s", err.Error()))
		report.Success = false
	}

	for _, tr := range tableResults {
		if !tr.Success {
			report.Success = false
		}
	}

	// Vault migration (if configured)
	if m.vaultMigrator != nil {
		log.Println("Vault migration...")

		// Pass ID mapper from MySQL migration to Vault migration
		// This enables Vault paths to be updated with new app_id and release_id
		m.vaultMigrator.SetIDMapper(m.mysqlMigrator.GetIDMapper())

		vaultResult, err := m.vaultMigrator.Migrate()
		report.VaultResults = vaultResult

		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("Vault migration error: %s", err.Error()))
			report.Success = false
		} else if !vaultResult.Success {
			report.Success = false
		}
	} else {
		log.Println("Vault migration skipped (not configured)")
	}

	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(report.StartTime)

	log.Printf("Full migration completed in %v\n", report.Duration)
	return report, nil
}

// Validate runs validation on migrated data
func (m *Migrator) Validate() (*ValidationReport, error) {
	return m.validator.Validate()
}

// Scan performs asset scanning on source and target databases (configured tables only)
func (m *Migrator) Scan() (*ScanReport, error) {
	return m.scanner.Scan()
}

// ScanAll performs asset scanning on all tables in source and target databases
func (m *Migrator) ScanAll() (*ScanReport, error) {
	return m.scanner.ScanAll()
}

// PrintScanReport prints the scan report to stdout
func (m *Migrator) PrintScanReport(report *ScanReport) {
	m.scanner.PrintReport(report)
}

// FullCleanupResult contains the result of full cleanup operation (MySQL + Vault)
type FullCleanupResult struct {
	MySQLResult *CleanupResult
	VaultResult *VaultCleanupResult
	Duration    time.Duration
	Success     bool
}

// Cleanup clears migrated data from target database and Vault.
// If biz_id filter is configured (--biz-ids), only clears data for those businesses.
// Vault cleanup runs FIRST because it reads target DB records to construct correct Vault paths;
// MySQL cleanup runs SECOND so that target DB records are still available for Vault path lookup.
func (m *Migrator) Cleanup() (*FullCleanupResult, error) {
	startTime := time.Now()
	result := &FullCleanupResult{
		Success: true,
	}

	log.Println("Starting full cleanup (Vault + MySQL)...")

	// Step 1: Cleanup Vault FIRST (reads target DB to get mapped IDs for Vault paths)
	if m.vaultMigrator != nil {
		log.Println("Step 1/2: Vault cleanup...")
		vaultResult, err := m.vaultMigrator.CleanupTarget()
		result.VaultResult = vaultResult
		if err != nil {
			result.Success = false
			log.Printf("Vault cleanup failed: %v", err)
		} else if !vaultResult.Success {
			result.Success = false
		}
	} else {
		log.Println("Step 1/2: Vault cleanup skipped (not configured)")
	}

	// Step 2: Cleanup MySQL SECOND (after Vault has read the records it needs)
	log.Println("Step 2/2: MySQL cleanup...")
	mysqlResult, err := m.mysqlMigrator.CleanupTarget()
	result.MySQLResult = mysqlResult
	if err != nil {
		result.Success = false
		log.Printf("MySQL cleanup failed: %v", err)
	} else if !mysqlResult.Success {
		result.Success = false
	}

	result.Duration = time.Since(startTime)
	log.Printf("Full cleanup completed in %v", result.Duration)

	return result, nil
}

// PrintCleanupReport prints the cleanup report to stdout
func (m *Migrator) PrintCleanupReport(result *FullCleanupResult) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("CLEANUP REPORT")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Duration:    %v\n", result.Duration)
	fmt.Printf("Status:      %s\n", boolToStatus(result.Success))
	fmt.Println()

	// MySQL cleanup results
	if result.MySQLResult != nil && len(result.MySQLResult.TableResults) > 0 {
		fmt.Println("MySQL Cleanup Results:")
		fmt.Println(strings.Repeat("-", 60))
		fmt.Printf("%-40s %12s %8s\n", "Table", "Deleted", "Status")
		fmt.Println(strings.Repeat("-", 60))

		var totalDeleted int64
		for _, tr := range result.MySQLResult.TableResults {
			fmt.Printf("%-40s %12d %8s\n",
				tr.TableName, tr.DeletedCount, boolToStatus(tr.Success))
			totalDeleted += tr.DeletedCount
		}
		fmt.Println(strings.Repeat("-", 60))
		fmt.Printf("%-40s %12d\n", "Total", totalDeleted)
		fmt.Println()
	}

	// Vault cleanup results
	if result.VaultResult != nil {
		fmt.Println("Vault Cleanup Results:")
		fmt.Println(strings.Repeat("-", 60))
		fmt.Printf("KV Records:          %d deleted\n", result.VaultResult.DeletedKvs)
		fmt.Printf("Released KV Records: %d deleted\n", result.VaultResult.DeletedRKvs)
		fmt.Printf("Status:              %s\n", boolToStatus(result.VaultResult.Success))
		fmt.Println()
	}

	// Print MySQL errors if any
	hasErrors := false
	if result.MySQLResult != nil {
		for _, tr := range result.MySQLResult.TableResults {
			if tr.Error != "" {
				if !hasErrors {
					fmt.Println("Errors:")
					fmt.Println(strings.Repeat("-", 60))
					hasErrors = true
				}
				fmt.Printf("  MySQL Table %s: %s\n", tr.TableName, tr.Error)
			}
		}
	}

	// Print Vault errors if any
	if result.VaultResult != nil && len(result.VaultResult.Errors) > 0 {
		if !hasErrors {
			fmt.Println("Errors:")
			fmt.Println(strings.Repeat("-", 60))
			hasErrors = true
		}
		for _, err := range result.VaultResult.Errors {
			fmt.Printf("  Vault: %s\n", err)
		}
	}

	if hasErrors {
		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 60))
}

// PrintReport prints a concise migration summary to stdout.
// If there are failed records, full details are written to a JSON log file.
func (m *Migrator) PrintReport(report *MigrationReport) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("MIGRATION REPORT")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("Start Time:  %s\n", report.StartTime.Format(time.RFC3339))
	fmt.Printf("End Time:    %s\n", report.EndTime.Format(time.RFC3339))
	fmt.Printf("Duration:    %v\n", report.Duration)
	fmt.Printf("Status:      %s\n", boolToStatus(report.Success))
	fmt.Println()

	if len(report.TableResults) > 0 {
		fmt.Println("MySQL Migration Results:")
		fmt.Println(strings.Repeat("-", 70))
		fmt.Printf("%-35s %10s %10s %8s %8s\n", "Table", "Source", "Migrated", "Failed", "Status")
		fmt.Println(strings.Repeat("-", 70))

		for _, tr := range report.TableResults {
			fmt.Printf("%-35s %10d %10d %8d %8s\n",
				tr.TableName, tr.SourceCount, tr.MigratedCount,
				tr.ErrorCount, boolToStatus(tr.Success))
		}
		fmt.Println()
	}

	if report.VaultResults != nil {
		fmt.Println("Vault Migration Results:")
		fmt.Println(strings.Repeat("-", 70))
		fmt.Printf("KV Records:          %d migrated of %d\n",
			report.VaultResults.MigratedKvs, report.VaultResults.KvCount)
		fmt.Printf("Released KV Records: %d migrated of %d\n",
			report.VaultResults.MigratedRKvs, report.VaultResults.ReleasedKvCount)
		fmt.Printf("Status:              %s\n", boolToStatus(report.VaultResults.Success))
		fmt.Println()
	}

	if len(report.Errors) > 0 {
		fmt.Println("Errors:")
		fmt.Println(strings.Repeat("-", 70))
		for _, err := range report.Errors {
			fmt.Printf("  - %s\n", err)
		}
		fmt.Println()
	}

	// Write failed rows to log file if any
	if m.mysqlMigrator.HasFailedRows() {
		logDir := m.buildLogDir()
		logFile, err := m.mysqlMigrator.WriteFailedRowsLog(logDir)
		if err != nil {
			log.Printf("Warning: failed to write error log: %v", err)
		} else if logFile != "" {
			fmt.Printf("  >> Failed records detail saved to: %s\n\n", logFile)
		}
	}

	fmt.Println(strings.Repeat("=", 70))
}

// ExistingDataItem describes a table with existing data in the target database
type ExistingDataItem struct {
	Table string
	Count int64
}

// CheckTargetData checks if the target database already has data for the configured biz_ids.
// Returns a list of tables with existing record counts (empty if no data found).
func (m *Migrator) CheckTargetData() []ExistingDataItem {
	if !m.cfg.Migration.HasBizFilter() {
		return nil
	}

	var result []ExistingDataItem
	coreLevel1Tables := []string{"applications", "template_spaces", "groups", "hooks", "credentials"}

	for _, table := range coreLevel1Tables {
		var count int64
		err := m.mysqlMigrator.targetDB.Table(table).
			Where("biz_id IN ?", m.cfg.Migration.BizIDs).
			Count(&count).Error
		if err != nil {
			log.Printf("Warning: failed to check target table %s: %v", table, err)
			continue
		}
		if count > 0 {
			result = append(result, ExistingDataItem{Table: table, Count: count})
		}
	}
	return result
}

// buildLogDir returns the log directory path based on biz_ids filter.
// e.g. logs/biz_100_200/ or logs/all/
func (m *Migrator) buildLogDir() string {
	var bizDir string
	if m.cfg.Migration.HasBizFilter() {
		parts := make([]string, 0, len(m.cfg.Migration.BizIDs))
		for _, id := range m.cfg.Migration.BizIDs {
			parts = append(parts, fmt.Sprintf("%d", id))
		}
		bizDir = "biz_" + strings.Join(parts, "_")
	} else {
		bizDir = "all"
	}
	return fmt.Sprintf("logs/%s", bizDir)
}

func boolToStatus(success bool) string {
	if success {
		return "SUCCESS"
	}
	return "FAILED"
}
