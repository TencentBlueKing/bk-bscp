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

const (
	// alignReferenceChunkSize 是引用表单条 UPDATE 携带的映射条数上限，
	// 控制 CASE 分支与 IN 列表的长度，避免撞上 max_allowed_packet。
	alignReferenceChunkSize = 200
	// auditResTypeConfigTemplate 是 audits.res_type 中配置模版的取值
	auditResTypeConfigTemplate = "config_template"
	// idGeneratorConfigTemplates 是 id_generators 中配置模版的资源名
	idGeneratorConfigTemplates = "config_templates"
)

// AlignOptions 是 align-template-id 子命令的运行参数。
type AlignOptions struct {
	// Execute 为 false 时只出报告不改数据
	Execute bool
	// OutputPath 是报告 JSON 的落盘路径
	OutputPath string
}

// TemplateIDAligner 把存量 config_templates.id 对齐到 GSEKit 的 config_template_id。
type TemplateIDAligner struct {
	cfg         *config.Config
	sourceDB    *gorm.DB
	targetDB    *gorm.DB
	idGen       *IDGenerator
	opts        AlignOptions
	reserveBase uint32
}

// ReferenceImpact 是三处引用的受影响行数。
type ReferenceImpact struct {
	ConfigInstances int64 `json:"config_instances"`
	TaskBatches     int64 `json:"task_batches"`
	Audits          int64 `json:"audits"`
}

// AlignVerifyResult 是阶段 4 的单项校验结论。
type AlignVerifyResult struct {
	Name    string `json:"name"`
	Pass    bool   `json:"pass"`
	Details string `json:"details,omitempty"`
}

// AlignReport 是对齐报告。执行失败时它同时是人工回滚的依据：
// Moves 里记录了完整的 old_id → final_id 路径，反向执行即可复原。
type AlignReport struct {
	GeneratedAt         time.Time                   `json:"generated_at"`
	DryRun              bool                        `json:"dry_run"`
	Executed            bool                        `json:"executed"`
	ReserveBase         uint32                      `json:"reserve_base"`
	GSEKitAutoIncrement uint32                      `json:"gsekit_auto_increment"`
	IDGeneratorMaxID    uint32                      `json:"id_generator_max_id"`
	Summary             map[AlignClassification]int `json:"summary"`
	Records             []AlignRecord               `json:"records"`
	Blocked             []AlignRecord               `json:"blocked,omitempty"`
	Moves               []MoveStep                  `json:"moves"`
	ReferenceImpact     ReferenceImpact             `json:"reference_impact"`
	VerifyResults       []AlignVerifyResult         `json:"verify_results,omitempty"`
}

// alignPreflight 是阶段 0 采集到的现场快照。
type alignPreflight struct {
	templates           []BSCPConfigTemplateRow
	currentMaxID        uint32
	gsekitAutoIncrement uint32
}

// NewTemplateIDAligner 建立双库连接并读取预留基线。
func NewTemplateIDAligner(cfg *config.Config, opts AlignOptions) (*TemplateIDAligner, error) {
	sourceDB, err := connectDB(cfg.Source.MySQL, "source (GSEKit)")
	if err != nil {
		return nil, err
	}

	targetDB, err := connectDB(cfg.Target.MySQL, "target (BSCP)")
	if err != nil {
		return nil, err
	}

	return &TemplateIDAligner{
		cfg:         cfg,
		sourceDB:    sourceDB,
		targetDB:    targetDB,
		idGen:       NewIDGenerator(targetDB),
		opts:        opts,
		reserveBase: cfg.Migration.ConfigTemplateIDReserveBase,
	}, nil
}

// Close 关闭双库连接。
func (a *TemplateIDAligner) Close() {
	if a.sourceDB != nil {
		if db, err := a.sourceDB.DB(); err == nil {
			db.Close()
		}
	}
	if a.targetDB != nil {
		if db, err := a.targetDB.DB(); err == nil {
			db.Close()
		}
	}
}

// Run 依次执行体检、建映射、报告，在 --execute 且无待人工确认项时继续执行与校验。
func (a *TemplateIDAligner) Run() (*AlignReport, error) {
	// todo：增加锁表的操作，避免在执行过程中有新模版创建
	pre, err := a.preflight()
	if err != nil {
		return nil, err
	}

	records, err := a.buildMapping(pre)
	if err != nil {
		return nil, err
	}

	blocked := make([]AlignRecord, 0)
	for _, r := range records {
		// 存在迁移产物未查询到 gsekit 模版，需要人工决策
		if needsManualDecision(r.Classification) {
			blocked = append(blocked, r)
		}
	}

	// dry-run 与被阻塞的场景都不能动 id_generators，用模拟 ID 把终值和引用影响算出来即可
	need := tempIDsNeeded(records)
	willExecute := a.opts.Execute && len(blocked) == 0

	// 预分配腾空所需的连续高位 ID
	var tempIDs []uint32
	if willExecute {
		if tempIDs, err = a.allocateHighIDs(need); err != nil {
			return nil, err
		}
	} else {
		tempIDs = simulateHighIDs(pre.currentMaxID, a.reserveBase, need)
	}

	// 规划搬迁步骤
	steps, err := planAlignment(records, tempIDs)
	if err != nil {
		return nil, err
	}

	report, err := a.buildReport(pre, records, blocked, steps)
	if err != nil {
		return nil, err
	}

	if !willExecute {
		if err := a.writeReport(report); err != nil {
			return report, err
		}
		if len(blocked) > 0 && a.opts.Execute {
			return report, fmt.Errorf("%d template(s) need manual decision, refusing to execute; see %s",
				len(blocked), a.opts.OutputPath)
		}
		return report, nil
	}

	if err := a.execute(steps); err != nil {
		// 报告先落盘再返回错误，否则人工回滚会失去依据
		if writeErr := a.writeReport(report); writeErr != nil {
			log.Printf("  Warning: write report failed: %v", writeErr)
		}
		return report, err
	}
	report.Executed = true

	report.VerifyResults = a.verify(pre, steps)
	if err := a.writeReport(report); err != nil {
		return report, err
	}

	for _, v := range report.VerifyResults {
		if !v.Pass {
			return report, fmt.Errorf("post-alignment verification failed: %s (%s); "+
				"use the moves section in %s to roll back manually", v.Name, v.Details, a.opts.OutputPath)
		}
	}

	return report, nil
}

// preflight 采集执行时点的现场：GSEKit 水位、BSCP 全量模版、id_generators 水位，
// 并确认没有进行中的任务。快照必须在同一时点采集，不接受历史结论。
func (a *TemplateIDAligner) preflight() (*alignPreflight, error) {
	autoIncr, err := a.readGSEKitTemplateAutoIncrement()
	if err != nil {
		return nil, err
	}
	if autoIncr >= a.reserveBase {
		return nil, fmt.Errorf("gsekit_configtemplate AUTO_INCREMENT %d has reached the reserve base %d, "+
			"raise migration.config_template_id_reserve_base before aligning", autoIncr, a.reserveBase)
	}

	var templates []BSCPConfigTemplateRow
	if err := a.targetDB.Raw(
		"SELECT id, biz_id, name, creator " +
			"FROM config_templates ORDER BY id ASC").Scan(&templates).Error; err != nil {
		return nil, fmt.Errorf("read config_templates failed: %w", err)
	}
	if len(templates) == 0 {
		return nil, fmt.Errorf("config_templates is empty, nothing to align")
	}

	var maxID uint32
	if err := a.targetDB.Raw(
		"SELECT max_id FROM id_generators WHERE resource = ?",
		idGeneratorConfigTemplates).Scan(&maxID).Error; err != nil {
		return nil, fmt.Errorf("read id_generators for config_templates failed: %w", err)
	}

	// todo：补充 sql 检查数据库是否有脏数据
	if err := a.checkRunningTasks(); err != nil {
		return nil, err
	}

	log.Printf("  Preflight: %d config_templates, id_generators max_id=%d, "+
		"gsekit AUTO_INCREMENT=%d, reserve base=%d", len(templates), maxID, autoIncr, a.reserveBase)

	return &alignPreflight{
		templates:           templates,
		currentMaxID:        maxID,
		gsekitAutoIncrement: autoIncr,
	}, nil
}

// readGSEKitTemplateAutoIncrement 读 GSEKit 模版表的 AUTO_INCREMENT。
// GSEKit 是硬删除且主键不回收，MAX(config_template_id) 会低于真实水位，
// 预留基线必须画在 AUTO_INCREMENT 之上。
func (a *TemplateIDAligner) readGSEKitTemplateAutoIncrement() (uint32, error) {
	var autoIncr *uint32
	if err := a.sourceDB.Raw(
		"SELECT AUTO_INCREMENT FROM information_schema.TABLES " +
			"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'gsekit_configtemplate'").
		Scan(&autoIncr).Error; err != nil {
		return 0, fmt.Errorf("read gsekit_configtemplate AUTO_INCREMENT failed: %w", err)
	}
	if autoIncr == nil {
		return 0, fmt.Errorf("gsekit_configtemplate AUTO_INCREMENT is unavailable")
	}
	return *autoIncr, nil
}

// checkRunningTasks 拒绝在有进行中任务时搬迁：那些任务持有的模版 ID 会在执行过程中失效。
func (a *TemplateIDAligner) checkRunningTasks() error {
	var running int64
	if err := a.targetDB.Raw(
		"SELECT COUNT(*) FROM task_batches WHERE status = ?", "running").Scan(&running).Error; err != nil {
		return fmt.Errorf("count running task_batches failed: %w", err)
	}
	if running > 0 {
		return fmt.Errorf("%d task_batches are still running, wait for them to finish before aligning", running)
	}
	return nil
}

// bizNameKey 是 (业务, 模版名)，两侧唯一索引保证它是天然键：
// GSEKit 侧 unique_together (bk_biz_id, template_name)，BSCP 侧 uniqueIndex (biz_id, name)。
type bizNameKey struct {
	bizID uint32
	name  string
}

// buildMapping 为每条 BSCP 记录求目标 GSEKit ID。
func (a *TemplateIDAligner) buildMapping(pre *alignPreflight) ([]AlignRecord, error) {
	byBizName, err := a.loadGSEKitTemplates()
	if err != nil {
		return nil, err
	}

	records := make([]AlignRecord, 0, len(pre.templates))
	// 遍历 BSCP 模版，根据名字匹配结果给记录分类，并返回自动决策出的 GSEKit ID。
	for _, bscpTemplate := range pre.templates {
		// 查询 GSEKit 模版表，获取 (业务, 模版名) 对应的 config_template_id
		gsekitConfigTemplateID := byBizName[bizNameKey{bizID: bscpTemplate.BizID, name: bscpTemplate.Name}]
		class, autoID := classifyTemplate(bscpTemplate, gsekitConfigTemplateID, a.cfg.Migration.Creator,
			a.cfg.Migration.IsNativeBiz(bscpTemplate.BizID))
		records = append(records, a.decide(bscpTemplate, class, autoID))
	}

	return records, nil
}

// decide 把分类结论转成一条报告记录，落定终值与决策来源。
func (a *TemplateIDAligner) decide(row BSCPConfigTemplateRow, class AlignClassification,
	autoID uint32) AlignRecord {

	rec := AlignRecord{
		BSCPID:         row.ID,
		BizID:          row.BizID,
		Name:           row.Name,
		Classification: class,
		GSEKitIDByName: autoID,
	}

	switch class {
	case ClassMatchedName:
		if autoID == row.ID {
			// 已经对齐到 GSEKit 的模版不必再动，直接复用原 ID
			rec.FinalNewID = row.ID
			rec.DecisionSource = decisionNoop
			rec.Note = "already aligned"
			break
		}
		rec.FinalNewID = autoID
		rec.DecisionSource = decisionName

	case ClassUnmatchedNative, ClassForcedNative:
		// 已经在预留区的自建模版不必再动，省掉一次无谓搬迁和引用改写
		// 实际上这种情况不可能出现，目前数据库存在 ID 大于预留基线的自建模版，所以必然需要重新分配
		if row.ID >= a.reserveBase {
			rec.FinalNewID = row.ID
			rec.DecisionSource = decisionNoop
			rec.Note = "BSCP-native template already outside the GSEKit range"
			break
		}
		// 终值留给 planAlignment 分配的高位 ID
		rec.DecisionSource = decisionEvacuate
		rec.Note = "BSCP-native template, evacuated to the reserved range"
		if class == ClassForcedNative {
			rec.Note = fmt.Sprintf("biz %d is configured as native_biz_id, evacuated to the reserved range without name matching",
				row.BizID)
		}

	case ClassUnmatchedUnknown:
		rec.DecisionSource = decisionBlocked
		rec.Note = "looks like a migration artifact but has no GSEKit match, needs manual decision"
	}

	return rec
}

// loadGSEKitTemplates 全量加载 GSEKit 模版建 (业务, 名字) → config_template_id 索引。
func (a *TemplateIDAligner) loadGSEKitTemplates() (map[bizNameKey]uint32, error) {
	var rows []struct {
		ConfigTemplateID uint32 `gorm:"column:config_template_id"`
		BkBizID          uint32 `gorm:"column:bk_biz_id"`
		TemplateName     string `gorm:"column:template_name"`
	}
	if err := a.sourceDB.Raw(
		"SELECT config_template_id, bk_biz_id, template_name FROM gsekit_configtemplate").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("read gsekit_configtemplate failed: %w", err)
	}

	byBizName := make(map[bizNameKey]uint32, len(rows))
	for _, r := range rows {
		byBizName[bizNameKey{bizID: r.BkBizID, name: r.TemplateName}] = r.ConfigTemplateID
	}

	log.Printf("  Loaded %d GSEKit config templates", len(rows))
	return byBizName, nil
}

// allocateHighIDs 预分配腾空所需的连续高位 ID。
// 刻意放在事务外先提交：水位单调递增，即使后续事务回滚，白白消耗掉的 ID 段也不影响正确性。
func (a *TemplateIDAligner) allocateHighIDs(count int) ([]uint32, error) {
	// 先把水位顶到预留基线之上，此后任何并发新建拿到的 ID 都落不进 GSEKit 区间
	if err := a.idGen.EnsureAtLeast(idGeneratorConfigTemplates, a.reserveBase); err != nil {
		return nil, err
	}

	if count == 0 {
		return nil, nil
	}

	// 预分配腾空所需的连续高位 ID
	ids, err := a.idGen.BatchNextID(idGeneratorConfigTemplates, count)
	if err != nil {
		return nil, fmt.Errorf("allocate %d temporary ids failed: %w", count, err)
	}
	log.Printf("  Allocated %d temporary ids: %d..%d", len(ids), ids[0], ids[len(ids)-1])
	return ids, nil
}

// buildReport 汇总分类统计与引用影响。
func (a *TemplateIDAligner) buildReport(pre *alignPreflight, records, blocked []AlignRecord,
	steps []MoveStep) (*AlignReport, error) {

	summary := make(map[AlignClassification]int)
	for _, r := range records {
		summary[r.Classification]++
	}

	impact, err := a.estimateReferenceImpact(finalMapping(steps))
	if err != nil {
		return nil, err
	}

	return &AlignReport{
		GeneratedAt:         time.Now(),
		DryRun:              !a.opts.Execute,
		ReserveBase:         a.reserveBase,
		GSEKitAutoIncrement: pre.gsekitAutoIncrement,
		IDGeneratorMaxID:    pre.currentMaxID,
		Summary:             summary,
		Records:             records,
		Blocked:             blocked,
		Moves:               steps,
		ReferenceImpact:     impact,
	}, nil
}

// estimateReferenceImpact 统计三处引用各有多少行会被改写。
func (a *TemplateIDAligner) estimateReferenceImpact(pairs []IDPair) (ReferenceImpact, error) {
	var impact ReferenceImpact
	if len(pairs) == 0 {
		return impact, nil
	}

	oldIDs := pairSourceIDs(pairs)

	if err := a.targetDB.Raw(
		"SELECT COUNT(*) FROM config_instances WHERE config_template_id IN ?",
		oldIDs).Scan(&impact.ConfigInstances).Error; err != nil {
		return impact, fmt.Errorf("count affected config_instances failed: %w", err)
	}

	if err := a.targetDB.Raw(
		"SELECT COUNT(*) FROM audits WHERE res_type = ? AND res_id IN ?",
		auditResTypeConfigTemplate, oldIDs).Scan(&impact.Audits).Error; err != nil {
		return impact, fmt.Errorf("count affected audits failed: %w", err)
	}

	mapping := make(map[uint32]uint32, len(pairs))
	for _, p := range pairs {
		mapping[p.From] = p.To
	}
	batches, err := a.loadTaskBatchesToRewrite(mapping)
	if err != nil {
		return impact, err
	}
	impact.TaskBatches = int64(len(batches))

	return impact, nil
}

// taskBatchRewrite 是一条待改写的 task_batches 记录。
type taskBatchRewrite struct {
	ID       uint32
	TaskData string
}

// loadTaskBatchesToRewrite 找出 task_data 里含被搬迁模版 ID 的批次。
// task_data 是 longtext 存的 JSON，只能在 Go 侧解析改写，不用 SQL 的 JSON 函数，
// 以免依赖具体 MySQL 版本。先用 LIKE 粗筛，避免把整表拉进内存。
func (a *TemplateIDAligner) loadTaskBatchesToRewrite(mapping map[uint32]uint32) ([]taskBatchRewrite, error) {
	var rows []struct {
		ID       uint32 `gorm:"column:id"`
		TaskData string `gorm:"column:task_data"`
	}
	if err := a.targetDB.Raw(
		"SELECT id, task_data FROM task_batches WHERE task_data LIKE ? ORDER BY id ASC",
		"%config_template_ids%").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("read task_batches failed: %w", err)
	}

	out := make([]taskBatchRewrite, 0)
	for _, r := range rows {
		rewritten, changed, err := rewriteTaskData(r.TaskData, mapping)
		if err != nil {
			return nil, fmt.Errorf("task_batches %d: %w", r.ID, err)
		}
		if changed {
			out = append(out, taskBatchRewrite{ID: r.ID, TaskData: rewritten})
		}
	}

	return out, nil
}

// writeReport 把报告写成 JSON。
func (a *TemplateIDAligner) writeReport(report *AlignReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal align report failed: %w", err)
	}
	if err := os.WriteFile(a.opts.OutputPath, data, 0o644); err != nil {
		return fmt.Errorf("write align report to %s failed: %w", a.opts.OutputPath, err)
	}
	log.Printf("  Report written to %s", a.opts.OutputPath)
	return nil
}

// PrintAlignReport 打印报告摘要。
func PrintAlignReport(report *AlignReport) {
	mode := "DRY-RUN"
	if report.Executed {
		mode = "EXECUTED"
	} else if !report.DryRun {
		mode = "NOT EXECUTED"
	}

	fmt.Println("\n========== Config Template ID Alignment ==========")
	fmt.Printf("Mode: %s\n", mode)
	fmt.Printf("Reserve base: %d (gsekit AUTO_INCREMENT: %d, id_generators max_id: %d)\n",
		report.ReserveBase, report.GSEKitAutoIncrement, report.IDGeneratorMaxID)

	fmt.Println("\nClassification:")
	classes := make([]string, 0, len(report.Summary))
	for c := range report.Summary {
		classes = append(classes, string(c))
	}
	sort.Strings(classes)
	for _, c := range classes {
		fmt.Printf("  %-18s %d\n", c, report.Summary[AlignClassification(c)])
	}

	fmt.Printf("\nTemplates to move: %d\n", len(report.Moves))
	fmt.Printf("Reference rows affected: config_instances=%d, task_batches=%d, audits=%d\n",
		report.ReferenceImpact.ConfigInstances, report.ReferenceImpact.TaskBatches,
		report.ReferenceImpact.Audits)

	if len(report.Blocked) > 0 {
		fmt.Printf("\nNeed manual decision (%d):\n", len(report.Blocked))
		for _, r := range report.Blocked {
			fmt.Printf("  biz %d, bscp id %d, name %q\n", r.BizID, r.BSCPID, r.Name)
		}
	}

	if len(report.VerifyResults) > 0 {
		fmt.Println("\nVerification:")
		for _, v := range report.VerifyResults {
			status := "PASS"
			if !v.Pass {
				status = "FAIL"
			}
			fmt.Printf("  [%s] %s\n", status, v.Name)
			if v.Details != "" {
				fmt.Printf("         %s\n", v.Details)
			}
		}
	}

	fmt.Println("=================================================")
}
