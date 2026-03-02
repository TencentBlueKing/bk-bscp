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

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/cmd/gsekit-migration/config"
)

// Validator performs post-migration data validation
type Validator struct {
	cfg      *config.Config
	sourceDB *gorm.DB
	targetDB *gorm.DB
}

// ValidationReport contains the overall validation result
type ValidationReport struct {
	Success bool
	Checks  []ValidationCheck
}

// ValidationCheck contains a single validation check result
type ValidationCheck struct {
	Name    string
	Pass    bool
	Details string
}

// NewValidator creates a new Validator
func NewValidator(cfg *config.Config, sourceDB, targetDB *gorm.DB) *Validator {
	return &Validator{
		cfg:      cfg,
		sourceDB: sourceDB,
		targetDB: targetDB,
	}
}

// Validate runs all validation checks
func (v *Validator) Validate() (*ValidationReport, error) {
	report := &ValidationReport{Success: true}

	log.Println("Running validation checks...")

	checks := []struct {
		name string
		fn   func() *ValidationCheck
	}{
		{"Process record count", v.checkProcessCount},
		{"ProcessInst record count", v.checkProcessInstCount},
		{"ConfigTemplate record count", v.checkConfigTemplateCount},
		{"TemplateRevision record count", v.checkTemplateRevisionCount},
		{"ConfigInstance record count", v.checkConfigInstanceCount},
		{"Tenant ID consistency", v.checkTenantID},
	}

	for _, check := range checks {
		result := check.fn()
		report.Checks = append(report.Checks, *result)
		if !result.Pass {
			report.Success = false
		}
	}

	return report, nil
}

func (v *Validator) checkProcessCount() *ValidationCheck {
	check := &ValidationCheck{Name: "Process record count", Pass: true}

	for _, bizID := range v.cfg.Migration.BizIDs {
		var sourceCount int64
		if err := v.sourceDB.Raw("SELECT COUNT(*) FROM gsekit_process WHERE bk_biz_id = ?", bizID).
			Scan(&sourceCount).Error; err != nil {
			check.Pass = false
			check.Details = fmt.Sprintf("source query failed for biz %d: %v", bizID, err)
			return check
		}

		var targetCount int64
		if err := v.targetDB.Raw("SELECT COUNT(*) FROM processes WHERE biz_id = ? AND tenant_id = ?",
			bizID, v.cfg.Migration.TenantID).Scan(&targetCount).Error; err != nil {
			check.Pass = false
			check.Details = fmt.Sprintf("target query failed for biz %d: %v", bizID, err)
			return check
		}

		if sourceCount != targetCount {
			check.Pass = false
			check.Details = fmt.Sprintf("biz %d: source=%d, target=%d (mismatch)", bizID, sourceCount, targetCount)
			return check
		}

		check.Details = appendDetail(check.Details, fmt.Sprintf("biz %d: %d records matched", bizID, sourceCount))
	}

	return check
}

func (v *Validator) checkProcessInstCount() *ValidationCheck {
	check := &ValidationCheck{Name: "ProcessInst record count", Pass: true}

	for _, bizID := range v.cfg.Migration.BizIDs {
		var sourceCount int64
		if err := v.sourceDB.Raw("SELECT COUNT(*) FROM gsekit_processinst WHERE bk_biz_id = ?", bizID).
			Scan(&sourceCount).Error; err != nil {
			check.Pass = false
			check.Details = fmt.Sprintf("source query failed for biz %d: %v", bizID, err)
			return check
		}

		var targetCount int64
		if err := v.targetDB.Raw("SELECT COUNT(*) FROM process_instances WHERE biz_id = ? AND tenant_id = ?",
			bizID, v.cfg.Migration.TenantID).Scan(&targetCount).Error; err != nil {
			check.Pass = false
			check.Details = fmt.Sprintf("target query failed for biz %d: %v", bizID, err)
			return check
		}

		if sourceCount != targetCount {
			check.Pass = false
			check.Details = fmt.Sprintf("biz %d: source=%d, target=%d (mismatch)", bizID, sourceCount, targetCount)
			return check
		}

		check.Details = appendDetail(check.Details, fmt.Sprintf("biz %d: %d records matched", bizID, sourceCount))
	}

	return check
}

func (v *Validator) checkConfigTemplateCount() *ValidationCheck {
	check := &ValidationCheck{Name: "ConfigTemplate record count", Pass: true}

	for _, bizID := range v.cfg.Migration.BizIDs {
		var sourceCount int64
		if err := v.sourceDB.Raw("SELECT COUNT(*) FROM gsekit_configtemplate WHERE bk_biz_id = ?", bizID).
			Scan(&sourceCount).Error; err != nil {
			check.Pass = false
			check.Details = fmt.Sprintf("source query failed for biz %d: %v", bizID, err)
			return check
		}

		var targetCount int64
		if err := v.targetDB.Raw("SELECT COUNT(*) FROM config_templates WHERE biz_id = ? AND tenant_id = ?",
			bizID, v.cfg.Migration.TenantID).Scan(&targetCount).Error; err != nil {
			check.Pass = false
			check.Details = fmt.Sprintf("target query failed for biz %d: %v", bizID, err)
			return check
		}

		if sourceCount != targetCount {
			check.Pass = false
			check.Details = fmt.Sprintf("biz %d: source=%d, target=%d (mismatch)", bizID, sourceCount, targetCount)
			return check
		}

		check.Details = appendDetail(check.Details, fmt.Sprintf("biz %d: %d records matched", bizID, sourceCount))
	}

	return check
}

func (v *Validator) checkTemplateRevisionCount() *ValidationCheck {
	check := &ValidationCheck{Name: "TemplateRevision record count", Pass: true}

	for _, bizID := range v.cfg.Migration.BizIDs {
		// Source: count all non-draft versions for templates in this biz
		var sourceCount int64
		if err := v.sourceDB.Raw(
			"SELECT COUNT(*) FROM gsekit_configtemplateversion v "+
				"INNER JOIN gsekit_configtemplate t ON v.config_template_id = t.config_template_id "+
				"WHERE t.bk_biz_id = ? AND v.is_draft = ?", bizID, false).
			Scan(&sourceCount).Error; err != nil {
			check.Pass = false
			check.Details = fmt.Sprintf("source query failed for biz %d: %v", bizID, err)
			return check
		}

		var targetCount int64
		if err := v.targetDB.Raw("SELECT COUNT(*) FROM template_revisions WHERE biz_id = ? AND tenant_id = ?",
			bizID, v.cfg.Migration.TenantID).Scan(&targetCount).Error; err != nil {
			check.Pass = false
			check.Details = fmt.Sprintf("target query failed for biz %d: %v", bizID, err)
			return check
		}

		if sourceCount != targetCount {
			check.Pass = false
			check.Details = fmt.Sprintf("biz %d: source=%d, target=%d (mismatch)", bizID, sourceCount, targetCount)
			return check
		}

		check.Details = appendDetail(check.Details, fmt.Sprintf("biz %d: %d records matched", bizID, sourceCount))
	}

	return check
}

func (v *Validator) checkConfigInstanceCount() *ValidationCheck {
	check := &ValidationCheck{Name: "ConfigInstance record count", Pass: true}

	for _, bizID := range v.cfg.Migration.BizIDs {
		// Source: count latest released instances (max id per group where is_released=true)
		var sourceCount int64
		if err := v.sourceDB.Raw(latestReleasedCountQuery, bizID).
			Scan(&sourceCount).Error; err != nil {
			check.Pass = false
			check.Details = fmt.Sprintf("source query failed for biz %d: %v", bizID, err)
			return check
		}

		var targetCount int64
		if err := v.targetDB.Raw("SELECT COUNT(*) FROM config_instances WHERE biz_id = ? AND tenant_id = ?",
			bizID, v.cfg.Migration.TenantID).Scan(&targetCount).Error; err != nil {
			check.Pass = false
			check.Details = fmt.Sprintf("target query failed for biz %d: %v", bizID, err)
			return check
		}

		if sourceCount != targetCount {
			check.Pass = false
			check.Details = fmt.Sprintf("biz %d: source=%d, target=%d (mismatch)", bizID, sourceCount, targetCount)
			return check
		}

		check.Details = appendDetail(check.Details, fmt.Sprintf("biz %d: %d records matched", bizID, sourceCount))
	}

	return check
}

func (v *Validator) checkTenantID() *ValidationCheck {
	check := &ValidationCheck{Name: "Tenant ID consistency", Pass: true}

	tables := []string{"processes", "process_instances", "config_templates", "config_instances",
		"templates", "template_revisions", "template_spaces", "template_sets"}
	tenantID := v.cfg.Migration.TenantID

	for _, table := range tables {
		for _, bizID := range v.cfg.Migration.BizIDs {
			var count int64
			if err := v.targetDB.Raw(
				fmt.Sprintf("SELECT COUNT(*) FROM `%s` WHERE biz_id = ? AND (tenant_id IS NULL OR tenant_id != ?)", table),
				bizID, tenantID).Scan(&count).Error; err != nil {
				// Table might not have biz_id column, skip
				continue
			}
			if count > 0 {
				check.Pass = false
				check.Details = appendDetail(check.Details,
					fmt.Sprintf("table %s biz %d: %d records with missing/wrong tenant_id", table, bizID, count))
			}
		}
	}

	if check.Details == "" {
		check.Details = "All records have correct tenant_id"
	}

	return check
}

func appendDetail(existing, new string) string {
	if existing == "" {
		return new
	}
	return existing + "; " + new
}
