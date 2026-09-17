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
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/cmd/gsekit-migration/config"
)

const precheckAlertReasonNoGSEKitMatch = "no GSEKit template with same (biz_id, name)"

// PrecheckAlignOptions 是 precheck-align-template-id 的运行参数。
type PrecheckAlignOptions struct {
	OutputPath string
}

// TemplateIDPrechecker 只读确认迁移产物能否按名字在 GSEKit 命中。
// 与 align-template-id 无耦合，不改任何数据。
type TemplateIDPrechecker struct {
	cfg      *config.Config
	sourceDB *gorm.DB
	targetDB *gorm.DB
	opts     PrecheckAlignOptions
}

// PrecheckAlignSummary 是前置校验汇总。
type PrecheckAlignSummary struct {
	Scanned int `json:"scanned"`
	OK      int `json:"ok"`
	Alert   int `json:"alert"`
}

// PrecheckAlignOK 是一条名字命中的迁移产物。
type PrecheckAlignOK struct {
	BSCPID                 uint32 `json:"bscp_id"`
	BizID                  uint32 `json:"biz_id"`
	Name                   string `json:"name"`
	GSEKitConfigTemplateID uint32 `json:"gsekit_config_template_id"`
}

// PrecheckAlignAlert 是一条未能在 GSEKit 按名命中的迁移产物。
type PrecheckAlignAlert struct {
	BSCPID  uint32 `json:"bscp_id"`
	BizID   uint32 `json:"biz_id"`
	Name    string `json:"name"`
	Creator string `json:"creator"`
	Reason  string `json:"reason"`
}

// PrecheckAlignReport 是前置校验报告。
type PrecheckAlignReport struct {
	GeneratedAt      time.Time            `json:"generated_at"`
	MigrationCreator string               `json:"migration_creator"`
	ExcludedBizIDs   []uint32             `json:"excluded_biz_ids"`
	Summary          PrecheckAlignSummary `json:"summary"`
	Alerts           []PrecheckAlignAlert `json:"alerts"`
	OKs              []PrecheckAlignOK    `json:"oks"`
}

// NewTemplateIDPrechecker 建立双库只读连接。
func NewTemplateIDPrechecker(cfg *config.Config, opts PrecheckAlignOptions) (*TemplateIDPrechecker, error) {
	sourceDB, err := connectDB(cfg.Source.MySQL, "source (GSEKit)")
	if err != nil {
		return nil, err
	}

	targetDB, err := connectDB(cfg.Target.MySQL, "target (BSCP)")
	if err != nil {
		if db, dbErr := sourceDB.DB(); dbErr == nil {
			_ = db.Close()
		}
		return nil, err
	}

	return &TemplateIDPrechecker{
		cfg:      cfg,
		sourceDB: sourceDB,
		targetDB: targetDB,
		opts:     opts,
	}, nil
}

// Close 关闭双库连接。
func (p *TemplateIDPrechecker) Close() {
	if p.sourceDB != nil {
		if db, err := p.sourceDB.DB(); err == nil {
			_ = db.Close()
		}
	}
	if p.targetDB != nil {
		if db, err := p.targetDB.DB(); err == nil {
			_ = db.Close()
		}
	}
}

// Run 采集双侧数据、建报告并落盘。有 ALERT 时返回 error（供 CLI 非 0 退出），报告仍会写出。
func (p *TemplateIDPrechecker) Run() (*PrecheckAlignReport, error) {
	if p.cfg.Migration.Creator == "" {
		return nil, fmt.Errorf("migration.creator is empty, cannot identify migration artifacts")
	}

	var templates []BSCPConfigTemplateRow
	if err := p.targetDB.Raw(
		"SELECT id, biz_id, name, creator FROM config_templates ORDER BY id ASC").
		Scan(&templates).Error; err != nil {
		return nil, fmt.Errorf("read config_templates failed: %w", err)
	}

	byBizName, err := p.loadGSEKitNameIndex()
	if err != nil {
		return nil, err
	}

	report := buildPrecheckAlignReport(templates, byBizName, p.cfg.Migration.Creator, p.cfg.Migration.NativeBizID)
	if err := p.writeReport(report); err != nil {
		return report, err
	}

	if report.Summary.Alert > 0 {
		return report, fmt.Errorf("%d migration artifact(s) have no GSEKit name match; see %s",
			report.Summary.Alert, p.opts.OutputPath)
	}
	return report, nil
}

// loadGSEKitNameIndex 全量加载 GSEKit (biz_id, name) → config_template_id。
func (p *TemplateIDPrechecker) loadGSEKitNameIndex() (map[bizNameKey]uint32, error) {
	var rows []struct {
		ConfigTemplateID uint32 `gorm:"column:config_template_id"`
		BkBizID          uint32 `gorm:"column:bk_biz_id"`
		TemplateName     string `gorm:"column:template_name"`
	}
	if err := p.sourceDB.Raw(
		"SELECT config_template_id, bk_biz_id, template_name FROM gsekit_configtemplate").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("read gsekit_configtemplate failed: %w", err)
	}

	byBizName := make(map[bizNameKey]uint32, len(rows))
	for _, r := range rows {
		byBizName[bizNameKey{bizID: r.BkBizID, name: r.TemplateName}] = r.ConfigTemplateID
	}
	log.Printf("  Loaded %d GSEKit config templates for precheck", len(rows))
	return byBizName, nil
}

func (p *TemplateIDPrechecker) writeReport(report *PrecheckAlignReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal precheck report failed: %w", err)
	}
	if err := os.WriteFile(p.opts.OutputPath, data, 0o644); err != nil {
		return fmt.Errorf("write precheck report to %s failed: %w", p.opts.OutputPath, err)
	}
	log.Printf("  Precheck report written to %s", p.opts.OutputPath)
	return nil
}

// isPrecheckCandidate 判断是否纳入前置校验：迁移账号创建，且不是自建业务。
func isPrecheckCandidate(row BSCPConfigTemplateRow, migrationCreator string, nativeBizID uint32) bool {
	if migrationCreator == "" || row.Creator != migrationCreator {
		return false
	}
	return nativeBizID == 0 || row.BizID != nativeBizID
}

// buildPrecheckAlignReport 对候选记录做名字匹配并汇总。
func buildPrecheckAlignReport(templates []BSCPConfigTemplateRow, byBizName map[bizNameKey]uint32,
	migrationCreator string, nativeBizID uint32) *PrecheckAlignReport {

	excluded := make([]uint32, 0)
	if nativeBizID != 0 {
		excluded = []uint32{nativeBizID}
	}

	report := &PrecheckAlignReport{
		GeneratedAt:      time.Now(),
		MigrationCreator: migrationCreator,
		ExcludedBizIDs:   excluded,
		Alerts:           make([]PrecheckAlignAlert, 0),
		OKs:              make([]PrecheckAlignOK, 0),
	}

	for _, row := range templates {
		if !isPrecheckCandidate(row, migrationCreator, nativeBizID) {
			continue
		}
		report.Summary.Scanned++

		gsekitID := byBizName[bizNameKey{bizID: row.BizID, name: row.Name}]
		if gsekitID == 0 {
			report.Summary.Alert++
			report.Alerts = append(report.Alerts, PrecheckAlignAlert{
				BSCPID:  row.ID,
				BizID:   row.BizID,
				Name:    row.Name,
				Creator: row.Creator,
				Reason:  precheckAlertReasonNoGSEKitMatch,
			})
			continue
		}

		report.Summary.OK++
		report.OKs = append(report.OKs, PrecheckAlignOK{
			BSCPID:                 row.ID,
			BizID:                  row.BizID,
			Name:                   row.Name,
			GSEKitConfigTemplateID: gsekitID,
		})
	}

	sort.Slice(report.Alerts, func(i, j int) bool {
		if report.Alerts[i].BizID != report.Alerts[j].BizID {
			return report.Alerts[i].BizID < report.Alerts[j].BizID
		}
		return report.Alerts[i].BSCPID < report.Alerts[j].BSCPID
	})
	sort.Slice(report.OKs, func(i, j int) bool {
		if report.OKs[i].BizID != report.OKs[j].BizID {
			return report.OKs[i].BizID < report.OKs[j].BizID
		}
		return report.OKs[i].BSCPID < report.OKs[j].BSCPID
	})

	return report
}

// PrintPrecheckAlignReport 打印前置校验摘要。
func PrintPrecheckAlignReport(report *PrecheckAlignReport) {
	fmt.Println("\n========== Precheck: Align Template ID ==========")
	fmt.Printf("Migration creator: %s\n", report.MigrationCreator)
	fmt.Printf("Excluded biz_ids: %v\n", report.ExcludedBizIDs)
	fmt.Printf("Scanned: %d  OK: %d  ALERT: %d\n",
		report.Summary.Scanned, report.Summary.OK, report.Summary.Alert)

	if len(report.Alerts) > 0 {
		fmt.Printf("\nAlerts (%d):\n", len(report.Alerts))
		for _, a := range report.Alerts {
			fmt.Printf("  biz %d, bscp id %d, name %q, creator %s\n",
				a.BizID, a.BSCPID, a.Name, a.Creator)
			fmt.Printf("    reason: %s\n", a.Reason)
		}
	} else {
		fmt.Println("\nNo alerts: every scanned migration artifact matches a GSEKit template by name.")
	}
	fmt.Println("=================================================")
}
