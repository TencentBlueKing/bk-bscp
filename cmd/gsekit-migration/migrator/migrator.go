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

// Package migrator provides the core migration logic for GSEKit to BSCP
package migrator

import (
	// NOCC:gas/crypto(by design)
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/cmd/gsekit-migration/config"
)

// Migrator orchestrates the GSEKit to BSCP data migration
type Migrator struct {
	cfg        *config.Config
	sourceDB   *gorm.DB
	targetDB   *gorm.DB
	idGen      *IDGenerator
	cmdbClient CMDBClient
	uploader   ContentUploader

	// ID mapping tables
	processIDMap        map[uint32]uint32               // ccProcessID → newProcessID
	configTemplateIDMap map[uint32]uint32               // gsekitConfigTemplateID → bscpConfigTemplateID
	configVersionIDMap  map[uint32]uint32               // gsekitConfigVersionID → bscpTemplateRevisionID
	templateIDMap       map[uint32]uint32               // gsekitConfigTemplateID → bscpTemplateID
	templateSpaceMap    map[uint32]*templateSpaceResult // bizID → templateSpaceResult

	// Binding relationship sets, built during config template migration and
	// consumed by config instance migration to skip unbound pairs.
	instanceBindSet    map[templateProcessKey]bool // (configTemplateID, bkProcessID) for INSTANCE-type bindings
	templateBindSet    map[templateProcessKey]bool // (configTemplateID, processTemplateID) for TEMPLATE-type bindings
	processTemplateMap map[int64]int64             // bkProcessID → processTemplateID
}

// MigrationReport contains the overall migration report
type MigrationReport struct {
	Success   bool
	StartTime time.Time
	Duration  time.Duration
	Steps     []StepResult
	Errors    []string
}

// StepResult contains the result of a single migration step
type StepResult struct {
	Name     string
	Success  bool
	Duration time.Duration
	Details  string
}

// NewMigrator creates a new Migrator instance
func NewMigrator(cfg *config.Config) (*Migrator, error) {
	// Connect to source (GSEKit) database
	sourceDB, err := connectDB(cfg.Source.MySQL, "source (GSEKit)")
	if err != nil {
		return nil, err
	}

	// Connect to target (BSCP) database
	targetDB, err := connectDB(cfg.Target.MySQL, "target (BSCP)")
	if err != nil {
		return nil, err
	}

	// Create ID generator
	idGen := NewIDGenerator(targetDB)

	// Create CMDB client
	var cmdbClient CMDBClient
	if cfg.Migration.SkipCMDB {
		log.Println("CMDB: using mock client (skip_cmdb=true)")
		cmdbClient = NewMockCMDBClient()
	} else {
		log.Println("CMDB: using real client")
		cmdbClient = NewRealCMDBClient(&cfg.CMDB)
	}

	// Create content uploader (BKREPO or S3/COS based on config)
	// BKREPO project naming uses tenantID only in multi-tenant mode ({tenantID}.{project}).
	// In single-tenant mode, the BKREPO project name has no tenant prefix, so pass empty tenantID.
	var repoTenantID string
	if cfg.Migration.MultiTenant {
		repoTenantID = cfg.Migration.TenantID
	}
	var uploader ContentUploader
	if cfg.Repository.StorageType != "" {
		uploader = NewContentUploader(&cfg.Repository, repoTenantID)
		log.Printf("Repository: %s uploader initialized", cfg.Repository.StorageType)
	} else {
		log.Println("Repository: no storage_type configured, content will not be uploaded")
	}

	return &Migrator{
		cfg:                 cfg,
		sourceDB:            sourceDB,
		targetDB:            targetDB,
		idGen:               idGen,
		cmdbClient:          cmdbClient,
		uploader:            uploader,
		processIDMap:        make(map[uint32]uint32),
		configTemplateIDMap: make(map[uint32]uint32),
		configVersionIDMap:  make(map[uint32]uint32),
		templateIDMap:       make(map[uint32]uint32),
		templateSpaceMap:    make(map[uint32]*templateSpaceResult),
		instanceBindSet:    make(map[templateProcessKey]bool),
		templateBindSet:    make(map[templateProcessKey]bool),
		processTemplateMap: make(map[int64]int64),
	}, nil
}

// Close closes database connections
func (m *Migrator) Close() {
	if m.sourceDB != nil {
		sqlDB, err := m.sourceDB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
	if m.targetDB != nil {
		sqlDB, err := m.targetDB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

// isBizMigrated checks whether a biz has already been migrated by looking for
// the migration-specific template space (name="config_delivery") in the target DB.
// This is reliable because template_spaces is always the first thing created
// during migration and is cleaned up by the cleanup tool.
func (m *Migrator) isBizMigrated(bizID uint32) (bool, error) {
	var count int64
	if err := m.targetDB.Raw(
		"SELECT COUNT(*) FROM template_spaces WHERE biz_id = ? AND name = ? AND tenant_id = ?",
		bizID, "config_delivery", m.cfg.Migration.TenantID).Scan(&count).Error; err != nil {
		return false, fmt.Errorf("query template_spaces for biz %d: %w", bizID, err)
	}
	return count > 0, nil
}

// checkAlreadyMigrated checks all biz IDs and returns an error listing
// which ones have already been migrated, so the user can re-check the command.
func (m *Migrator) checkAlreadyMigrated() error {
	var migrated []uint32
	for _, bizID := range m.cfg.Migration.BizIDs {
		found, err := m.isBizMigrated(bizID)
		if err != nil {
			return fmt.Errorf("failed to check migration status for biz %d: %w", bizID, err)
		}
		if found {
			migrated = append(migrated, bizID)
		}
	}
	if len(migrated) > 0 {
		return fmt.Errorf("the following biz_ids have already been migrated: %v\n"+
			"please remove them from biz_ids or run cleanup first, then retry", migrated)
	}
	return nil
}

// Run executes the full migration pipeline
func (m *Migrator) Run() (*MigrationReport, error) {
	report := &MigrationReport{
		Success:   true,
		StartTime: time.Now(),
	}

	log.Printf("Starting GSEKit to BSCP migration for biz_ids: %v", m.cfg.Migration.BizIDs)
	log.Printf("Multi-tenant: %v, Tenant ID: %q, Batch Size: %d",
		m.cfg.Migration.MultiTenant, m.cfg.Migration.TenantID, m.cfg.Migration.BatchSize)

	// Check for already-migrated biz IDs; abort immediately if any found
	if err := m.checkAlreadyMigrated(); err != nil {
		report.Success = false
		report.Errors = append(report.Errors, err.Error())
		report.Duration = time.Since(report.StartTime)
		return report, err
	}

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Create template spaces", m.migrateTemplateSpaces},
		{"Migrate processes", m.migrateProcesses},
		{"Migrate process instances", m.migrateProcessInstances},
		{"Migrate config templates", m.migrateConfigTemplates},
		{"Migrate config instances", m.migrateConfigInstances},
	}

	for _, step := range steps {
		stepStart := time.Now()
		err := step.fn()
		result := StepResult{
			Name:     step.name,
			Success:  err == nil,
			Duration: time.Since(stepStart),
		}
		if err != nil {
			result.Details = err.Error()
			report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", step.name, err))
			report.Success = false

			if !m.cfg.Migration.ContinueOnError {
				report.Steps = append(report.Steps, result)
				report.Duration = time.Since(report.StartTime)
				return report, err
			}
		}
		report.Steps = append(report.Steps, result)
	}

	report.Duration = time.Since(report.StartTime)
	log.Printf("Migration completed in %v", report.Duration)
	return report, nil
}

// Validate runs the validation checks
func (m *Migrator) Validate() (*ValidationReport, error) {
	v := NewValidator(m.cfg, m.sourceDB, m.targetDB)
	return v.Validate()
}

// Cleanup runs the cleanup operation
func (m *Migrator) Cleanup() (*CleanupReport, error) {
	c := NewCleaner(m.cfg, m.targetDB)
	return c.Cleanup()
}

// PrintReport prints the migration report
func (m *Migrator) PrintReport(report *MigrationReport) {
	fmt.Println("\n========== Migration Report ==========")
	fmt.Printf("Status: %s\n", boolToStatus(report.Success))
	fmt.Printf("Duration: %v\n", report.Duration)
	fmt.Printf("Biz IDs: %v\n", m.cfg.Migration.BizIDs)
	fmt.Println("\nSteps:")
	for _, step := range report.Steps {
		status := "OK"
		if !step.Success {
			status = "FAILED"
		}
		fmt.Printf("  [%s] %s (%v)\n", status, step.Name, step.Duration)
		if step.Details != "" {
			fmt.Printf("         Error: %s\n", step.Details)
		}
	}
	if len(report.Errors) > 0 {
		fmt.Printf("\nErrors (%d):\n", len(report.Errors))
		for _, err := range report.Errors {
			fmt.Printf("  - %s\n", err)
		}
	}

	// Print ID mapping stats
	fmt.Printf("\nID Mappings:\n")
	fmt.Printf("  Processes: %d\n", len(m.processIDMap))
	fmt.Printf("  Config Templates: %d\n", len(m.configTemplateIDMap))
	fmt.Printf("  Config Versions: %d\n", len(m.configVersionIDMap))
	fmt.Printf("  Templates: %d\n", len(m.templateIDMap))
	fmt.Println("=======================================")
}

// PrintValidationReport prints the validation report
func (m *Migrator) PrintValidationReport(report *ValidationReport) {
	fmt.Println("\n========== Validation Report ==========")
	fmt.Printf("Status: %s\n", boolToStatus(report.Success))
	fmt.Println("\nChecks:")
	for _, check := range report.Checks {
		status := "PASS"
		if !check.Pass {
			status = "FAIL"
		}
		fmt.Printf("  [%s] %s\n", status, check.Name)
		if check.Details != "" {
			fmt.Printf("         %s\n", check.Details)
		}
	}
	fmt.Println("=======================================")
}

// PrintCleanupReport prints the cleanup report
func (m *Migrator) PrintCleanupReport(report *CleanupReport) {
	fmt.Println("\n========== Cleanup Report ==========")
	fmt.Printf("Status: %s\n", boolToStatus(report.Success))
	fmt.Printf("Duration: %v\n", report.Duration)
	fmt.Println("\nTables:")
	for _, tr := range report.Tables {
		status := "OK"
		if !tr.Success {
			status = "FAILED"
		}
		fmt.Printf("  [%s] %s: %d records deleted\n", status, tr.TableName, tr.DeletedCount)
		if tr.Error != "" {
			fmt.Printf("         Error: %s\n", tr.Error)
		}
	}
	fmt.Println("====================================")
}

func boolToStatus(b bool) string {
	if b {
		return "SUCCESS"
	}
	return "FAILED"
}

// byteSHA256 computes SHA256 hex string for byte data
func byteSHA256(data []byte) string {
	hash := sha256.New()
	hash.Write(data) // nolint
	return fmt.Sprintf("%x", hash.Sum(nil))
}

// byteMD5 computes MD5 hex string for byte data
func byteMD5(data []byte) string {
	hash := md5.New() // nolint
	hash.Write(data)  // nolint
	return fmt.Sprintf("%x", hash.Sum(nil))
}
