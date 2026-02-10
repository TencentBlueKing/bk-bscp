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
	"context"
	"fmt"
	"log"
	"time"

	vault "github.com/hashicorp/vault/api"
	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/cmd/tenant-migration/config"
)

const (
	// MountPath is the Vault mount path for BSCP KV data
	MountPath = "bk_bscp"
	// kvPath is the path format for unreleased KV data
	kvPath = "biz/%d/apps/%d/kvs/%s"
	// releasedKvPath is the path format for released KV data
	releasedKvPath = "biz/%d/apps/%d/releases/%d/kvs/%s"
)

// VaultMigrator handles Vault KV data migration
type VaultMigrator struct {
	cfg         *config.Config
	sourceVault *vault.Client
	targetVault *vault.Client
	sourceDB    *gorm.DB  // Source MySQL for reading KV records
	targetDB    *gorm.DB  // Target MySQL for reading migrated KV records
	idMapper    *IDMapper // ID mapper from MySQL migration (sourceID -> targetID)
}

// KvRecord represents a KV record from MySQL
type KvRecord struct {
	ID      uint32 `gorm:"column:id"`
	BizID   uint32 `gorm:"column:biz_id"`
	AppID   uint32 `gorm:"column:app_id"`
	Key     string `gorm:"column:key"`
	Version uint32 `gorm:"column:version"`
}

// ReleasedKvRecord represents a released KV record from MySQL
type ReleasedKvRecord struct {
	ID        uint32 `gorm:"column:id"`
	BizID     uint32 `gorm:"column:biz_id"`
	AppID     uint32 `gorm:"column:app_id"`
	ReleaseID uint32 `gorm:"column:release_id"`
	Key       string `gorm:"column:key"`
	Version   uint32 `gorm:"column:version"`
}

// NewVaultMigrator creates a new VaultMigrator instance
func NewVaultMigrator(cfg *config.Config, sourceDB, targetDB *gorm.DB) (*VaultMigrator, error) {
	// Create source Vault client
	sourceConfig := vault.DefaultConfig()
	sourceConfig.Address = cfg.Source.Vault.Address
	sourceClient, err := vault.NewClient(sourceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create source Vault client: %w", err)
	}
	sourceClient.SetToken(cfg.Source.Vault.Token)

	// Create target Vault client
	targetConfig := vault.DefaultConfig()
	targetConfig.Address = cfg.Target.Vault.Address
	targetClient, err := vault.NewClient(targetConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create target Vault client: %w", err)
	}
	targetClient.SetToken(cfg.Target.Vault.Token)

	return &VaultMigrator{
		cfg:         cfg,
		sourceVault: sourceClient,
		targetVault: targetClient,
		sourceDB:    sourceDB,
		targetDB:    targetDB,
		idMapper:    nil, // Will be set before migration
	}, nil
}

// SetIDMapper sets the ID mapper from MySQL migration
// This must be called before Migrate() when using incremental migration
func (m *VaultMigrator) SetIDMapper(mapper *IDMapper) {
	m.idMapper = mapper
}

// Migrate performs the Vault KV data migration with ID mapping support
func (m *VaultMigrator) Migrate() (*VaultMigrationResult, error) {
	startTime := time.Now()
	result := &VaultMigrationResult{
		Success: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Log migration mode
	if m.idMapper != nil && m.idMapper.Count("applications") > 0 {
		log.Println("Vault migration with ID mapping enabled (incremental migration mode)")
	} else {
		log.Println("Vault migration without ID mapping (direct path migration mode)")
	}

	// Migrate unreleased KV data
	log.Println("Migrating unreleased KV data...")
	kvCount, err := m.getKvRecordsCount()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to count KV records from source: %v", err))
		result.Success = false
		result.Duration = time.Since(startTime)
		return result, err
	}
	result.KvCount = kvCount

	batchSize := m.cfg.Migration.BatchSize
	var kvRecords []KvRecord
	for offset := 0; ; offset += batchSize {
		kvRecords, err = m.getKvRecordsBatch(offset, batchSize)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to get KV records batch at offset %d: %v", offset, err))
			result.Success = false
			result.Duration = time.Since(startTime)
			return result, err
		}

		if len(kvRecords) == 0 {
			break
		}

		for _, kv := range kvRecords {
			if migrateErr := m.migrateKv(ctx, kv); migrateErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to migrate KV %d: %v", kv.ID, migrateErr))
				if !m.cfg.Migration.ContinueOnError {
					result.Success = false
					result.Duration = time.Since(startTime)
					return result, migrateErr
				}
			} else {
				result.MigratedKvs++
			}
		}

		log.Printf("  KV progress: %d/%d migrated", result.MigratedKvs, result.KvCount)
	}

	// Migrate released KV data
	log.Println("Migrating released KV data...")
	releasedKvCount, err := m.getReleasedKvRecordsCount()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to count released KV records from source: %v", err))
		result.Success = false
		result.Duration = time.Since(startTime)
		return result, err
	}
	result.ReleasedKvCount = releasedKvCount

	var releasedKvRecords []ReleasedKvRecord
	for offset := 0; ; offset += batchSize {
		releasedKvRecords, err = m.getReleasedKvRecordsBatch(offset, batchSize)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to get released KV records batch at offset %d: %v", offset, err))
			result.Success = false
			result.Duration = time.Since(startTime)
			return result, err
		}

		if len(releasedKvRecords) == 0 {
			break
		}

		for _, rkv := range releasedKvRecords {
			if err := m.migrateReleasedKv(ctx, rkv); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to migrate released KV %d: %v", rkv.ID, err))
				if !m.cfg.Migration.ContinueOnError {
					result.Success = false
					result.Duration = time.Since(startTime)
					return result, err
				}
			} else {
				result.MigratedRKvs++
			}
		}

		log.Printf("  Released KV progress: %d/%d migrated", result.MigratedRKvs, result.ReleasedKvCount)
	}

	result.Duration = time.Since(startTime)
	log.Printf("Vault migration completed: %d KVs, %d released KVs in %v",
		result.MigratedKvs, result.MigratedRKvs, result.Duration)

	return result, nil
}

// getKvRecordsBatch retrieves a batch of KV records from source MySQL with pagination
// If biz_id filter is configured, only returns records for those businesses
func (m *VaultMigrator) getKvRecordsBatch(offset, limit int) ([]KvRecord, error) {
	var records []KvRecord
	query := m.sourceDB.Table("kvs")
	if m.cfg.Migration.HasBizFilter() {
		query = query.Where("biz_id IN ?", m.cfg.Migration.BizIDs)
	}
	if err := query.Offset(offset).Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// getKvRecordsCount returns the total count of KV records from source MySQL
// If biz_id filter is configured, only counts records for those businesses
func (m *VaultMigrator) getKvRecordsCount() (int64, error) {
	var count int64
	query := m.sourceDB.Table("kvs")
	if m.cfg.Migration.HasBizFilter() {
		query = query.Where("biz_id IN ?", m.cfg.Migration.BizIDs)
	}
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// getReleasedKvRecordsBatch retrieves a batch of released KV records from source MySQL with pagination
// If biz_id filter is configured, only returns records for those businesses
func (m *VaultMigrator) getReleasedKvRecordsBatch(offset, limit int) ([]ReleasedKvRecord, error) {
	var records []ReleasedKvRecord
	query := m.sourceDB.Table("released_kvs")
	if m.cfg.Migration.HasBizFilter() {
		query = query.Where("biz_id IN ?", m.cfg.Migration.BizIDs)
	}
	if err := query.Offset(offset).Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// getReleasedKvRecordsCount returns the total count of released KV records from source MySQL
// If biz_id filter is configured, only counts records for those businesses
func (m *VaultMigrator) getReleasedKvRecordsCount() (int64, error) {
	var count int64
	query := m.sourceDB.Table("released_kvs")
	if m.cfg.Migration.HasBizFilter() {
		query = query.Where("biz_id IN ?", m.cfg.Migration.BizIDs)
	}
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// getTargetAppID returns the target app_id using ID mapper, or source app_id if no mapping
func (m *VaultMigrator) getTargetAppID(sourceAppID uint32) uint32 {
	if m.idMapper == nil {
		return sourceAppID
	}
	if targetID := m.idMapper.Get("applications", sourceAppID); targetID != 0 {
		return targetID
	}
	return sourceAppID
}

// getTargetReleaseID returns the target release_id using ID mapper, or source release_id if no mapping
func (m *VaultMigrator) getTargetReleaseID(sourceReleaseID uint32) uint32 {
	if m.idMapper == nil {
		return sourceReleaseID
	}
	if targetID := m.idMapper.Get("releases", sourceReleaseID); targetID != 0 {
		return targetID
	}
	return sourceReleaseID
}

// getSourceKvPath returns the source Vault path for a KV record
func (m *VaultMigrator) getSourceKvPath(kv KvRecord) string {
	return fmt.Sprintf(kvPath, kv.BizID, kv.AppID, kv.Key)
}

// getTargetKvPath returns the target Vault path for a KV record (with ID mapping)
func (m *VaultMigrator) getTargetKvPath(kv KvRecord) string {
	targetAppID := m.getTargetAppID(kv.AppID)
	return fmt.Sprintf(kvPath, kv.BizID, targetAppID, kv.Key)
}

// getSourceReleasedKvPath returns the source Vault path for a released KV record
func (m *VaultMigrator) getSourceReleasedKvPath(rkv ReleasedKvRecord) string {
	return fmt.Sprintf(releasedKvPath, rkv.BizID, rkv.AppID, rkv.ReleaseID, rkv.Key)
}

// getTargetReleasedKvPath returns the target Vault path for a released KV record (with ID mapping)
func (m *VaultMigrator) getTargetReleasedKvPath(rkv ReleasedKvRecord) string {
	targetAppID := m.getTargetAppID(rkv.AppID)
	targetReleaseID := m.getTargetReleaseID(rkv.ReleaseID)
	return fmt.Sprintf(releasedKvPath, rkv.BizID, targetAppID, targetReleaseID, rkv.Key)
}

// migrateKv migrates a single unreleased KV from source to target Vault
// Uses ID mapping to update the target path if available
func (m *VaultMigrator) migrateKv(ctx context.Context, kv KvRecord) error {
	sourcePath := m.getSourceKvPath(kv)
	targetPath := m.getTargetKvPath(kv)

	// Read from source Vault
	secret, err := m.sourceVault.KVv2(MountPath).GetVersion(ctx, sourcePath, int(kv.Version))
	if err != nil {
		return fmt.Errorf("failed to read from source Vault path %s: %w", sourcePath, err)
	}

	if secret == nil || secret.Data == nil {
		log.Printf("  Warning: KV %s has no data, skipping", sourcePath)
		return nil
	}

	// Write to target Vault with potentially new path
	_, err = m.targetVault.KVv2(MountPath).Put(ctx, targetPath, secret.Data)
	if err != nil {
		return fmt.Errorf("failed to write to target Vault path %s: %w", targetPath, err)
	}

	return nil
}

// migrateReleasedKv migrates a single released KV from source to target Vault
// Uses ID mapping to update the target path if available
func (m *VaultMigrator) migrateReleasedKv(ctx context.Context, rkv ReleasedKvRecord) error {
	sourcePath := m.getSourceReleasedKvPath(rkv)
	targetPath := m.getTargetReleasedKvPath(rkv)

	// Read from source Vault
	secret, err := m.sourceVault.KVv2(MountPath).GetVersion(ctx, sourcePath, int(rkv.Version))
	if err != nil {
		return fmt.Errorf("failed to read from source Vault path %s: %w", sourcePath, err)
	}

	if secret == nil || secret.Data == nil {
		log.Printf("  Warning: Released KV %s has no data, skipping", sourcePath)
		return nil
	}

	// Write to target Vault with potentially new path
	_, err = m.targetVault.KVv2(MountPath).Put(ctx, targetPath, secret.Data)
	if err != nil {
		return fmt.Errorf("failed to write to target Vault path %s: %w", targetPath, err)
	}

	return nil
}

// VaultCleanupResult contains the result of Vault cleanup operation
type VaultCleanupResult struct {
	DeletedKvs  int64
	DeletedRKvs int64
	Duration    time.Duration
	Errors      []string
	Success     bool
}

// CleanupTarget deletes all migrated KV data from target Vault
// Uses source database to get KV records (since target DB may not have data yet)
// If biz_id filter is configured, only deletes KVs for those businesses
func (m *VaultMigrator) CleanupTarget() (*VaultCleanupResult, error) {
	startTime := time.Now()
	result := &VaultCleanupResult{
		Success: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Log biz_id filter info
	if m.cfg.Migration.HasBizFilter() {
		log.Printf("Vault cleanup with biz_id filter: %v", m.cfg.Migration.BizIDs)
	}

	// Step 1: Delete unreleased KV data (use source DB to get records)
	log.Println("Cleaning up unreleased KV data from target Vault...")
	batchSize := m.cfg.Migration.BatchSize
	for offset := 0; ; offset += batchSize {
		kvRecords, err := m.getKvRecordsBatch(offset, batchSize)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to get KV records batch at offset %d: %v", offset, err))
			result.Success = false
			result.Duration = time.Since(startTime)
			return result, err
		}

		if len(kvRecords) == 0 {
			break
		}

		for _, kv := range kvRecords {
			if deleteErr := m.deleteKv(ctx, kv); deleteErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to delete KV %d: %v", kv.ID, deleteErr))
				if !m.cfg.Migration.ContinueOnError {
					result.Success = false
					result.Duration = time.Since(startTime)
					return result, deleteErr
				}
			} else {
				result.DeletedKvs++
			}
		}
	}
	log.Printf("  Deleted %d unreleased KVs", result.DeletedKvs)

	// Step 2: Delete released KV data (use source DB to get records)
	log.Println("Cleaning up released KV data from target Vault...")
	for offset := 0; ; offset += batchSize {
		releasedKvRecords, err := m.getReleasedKvRecordsBatch(offset, batchSize)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to get released KV records batch at offset %d: %v", offset, err))
			result.Success = false
			result.Duration = time.Since(startTime)
			return result, err
		}

		if len(releasedKvRecords) == 0 {
			break
		}

		for _, rkv := range releasedKvRecords {
			if err := m.deleteReleasedKv(ctx, rkv); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to delete released KV %d: %v", rkv.ID, err))
				if !m.cfg.Migration.ContinueOnError {
					result.Success = false
					result.Duration = time.Since(startTime)
					return result, err
				}
			} else {
				result.DeletedRKvs++
			}
		}
	}
	log.Printf("  Deleted %d released KVs", result.DeletedRKvs)

	result.Duration = time.Since(startTime)
	log.Printf("Vault cleanup completed: %d KVs, %d released KVs deleted in %v",
		result.DeletedKvs, result.DeletedRKvs, result.Duration)

	return result, nil
}

// deleteKv deletes a single unreleased KV from target Vault
// Uses ID mapping for the target path
func (m *VaultMigrator) deleteKv(ctx context.Context, kv KvRecord) error {
	targetPath := m.getTargetKvPath(kv)

	// Delete all versions and metadata
	err := m.targetVault.KVv2(MountPath).DeleteMetadata(ctx, targetPath)
	if err != nil {
		return fmt.Errorf("failed to delete from target Vault path %s: %w", targetPath, err)
	}

	return nil
}

// deleteReleasedKv deletes a single released KV from target Vault
// Uses ID mapping for the target path
func (m *VaultMigrator) deleteReleasedKv(ctx context.Context, rkv ReleasedKvRecord) error {
	targetPath := m.getTargetReleasedKvPath(rkv)

	// Delete all versions and metadata
	err := m.targetVault.KVv2(MountPath).DeleteMetadata(ctx, targetPath)
	if err != nil {
		return fmt.Errorf("failed to delete from target Vault path %s: %w", targetPath, err)
	}

	return nil
}
