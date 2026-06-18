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

package migrations

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/cmd/data-service/db-migration/migrator"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

func init() {
	migrator.GetMigrator().AddMigration(&migrator.Migration{
		Version: "20260529100000",
		Name:    "20260529100000_add_project_env_dimension",
		Mode:    migrator.GormMode,
		Up:      mig20260529100000Up,
		Down:    mig20260529100000Down,
	})
}

// ============================================
// Up Migration: 分5步执行
// Step1: 创建 projects / environments / scope_migration_tasks 表 + 注册 id_generators
// Step2: 为每个 biz 创建默认项目和默认环境
// Step3: 为所有需要改动的表添加新字段 (AutoMigrate)
// Step4: 调整索引
// Step5: 分批回填存量数据的 project_id / environment_id（幂等、可续跑）
// ============================================
func mig20260529100000Up(db *gorm.DB) error {
	// 1-4 步：DDL 和基础数据插入，继续使用外层传入的 tx 句柄执行
	if err := stepCreateProjectEnvTables(db); err != nil {
		return fmt.Errorf("step create project/env tables failed: %w", err)
	}

	if err := stepInsertDefaultProjectsAndEnvs(db); err != nil {
		return fmt.Errorf("step insert default projects/envs failed: %w", err)
	}

	if err := stepAddColumns(db); err != nil {
		return fmt.Errorf("step add columns failed: %w", err)
	}

	if err := stepAdjustIndexes(db); err != nil {
		return fmt.Errorf("step adjust indexes failed: %w", err)
	}

	rawSQLDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// 用底层的连接池重新初始化一个全新的 gorm.DB，彻底脱离外层的 tx 事务
	// SkipDefaultTransaction: true 确保单次 Exec 不会产生不必要的额外 Begin/Commit
	standaloneDB, err := gorm.Open(db.Dialector, &gorm.Config{
		SkipDefaultTransaction: true,
		Logger:                 db.Logger, // 保持相同的日志输出
	})
	if err != nil {
		return fmt.Errorf("failed to open standalone gorm connection: %w", err)
	}

	// 将底层连接池赋给新实例
	standaloneDB = standaloneDB.WithContext(db.Statement.Context)
	standaloneDB.ConnPool = rawSQLDB

	// 第 5 步：使用全新的、非事务的 standaloneDB 执行
	if err := stepBackfillDefaultValues(standaloneDB); err != nil {
		return fmt.Errorf("step backfill default values failed: %w", err)
	}

	return nil
}

// Down Migration: 恢复旧索引 -> 删除新增字段 -> 删除默认数据 -> 删表
func mig20260529100000Down(tx *gorm.DB) error {
	// 1. 先恢复旧唯一索引（在删列之前，因为新索引依赖 project_id/environment_id 列）
	if err := stepRestoreIndexes(tx); err != nil {
		return fmt.Errorf("step restore indexes failed: %w", err)
	}

	// 2. 再删除新增的 project_id/environment_id 列（如果有的话）
	if err := stepDropColumns(tx); err != nil {
		return fmt.Errorf("step drop columns failed: %w", err)
	}

	// 3. 删除 environments, projects 表和 scope_migration_tasks 表
	for _, table := range []string{"environments", "projects", "scope_migration_tasks"} {
		if tx.Migrator().HasTable(table) {
			if err := tx.Migrator().DropTable(table); err != nil {
				return fmt.Errorf("drop %s table failed: %w", table, err)
			}
		}
	}

	// 4. 清理 id_generators
	if err := tx.Exec("DELETE FROM id_generators WHERE resource IN ('projects', 'environments')").Error; err != nil {
		return fmt.Errorf("clean id_generators: %w", err)
	}

	return nil
}

// =============================================
// Step1: 创建 projects / environments / scope_migration_tasks 表 + 注册 id_generators
// =============================================
type Project struct {
	ID        uint32    `gorm:"column:id"`
	TenantID  string    `gorm:"column:tenant_id;type:varchar(255);not null;default:'default'"`
	BizID     uint32    `gorm:"column:biz_id;not null"`
	Key       string    `gorm:"column:key;size:64;not null"`
	Name      string    `gorm:"column:name;size:255;not null"`
	Memo      string    `gorm:"column:memo;size:256"`
	Protected bool      `gorm:"column:protected;not null;default:false"`        // 保护标记: true=不允许删除/修改key
	IsDefault *bool     `gorm:"column:is_default;type:tinyint(1);default:null"` // 确保在迁移或未赋值时将其视作数据库的 DEFAULT NULL
	Creator   string    `gorm:"column:creator;size:64;not null"`
	Reviser   string    `gorm:"column:reviser;size:64;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

func (Project) TableName() string { return "projects" }

type Environment struct {
	ID           uint32    `gorm:"column:id"`
	TenantID     string    `gorm:"column:tenant_id;type:varchar(255);not null;default:'default'"`
	BizID        uint32    `gorm:"column:biz_id;not null"`
	ProjectID    uint32    `gorm:"column:project_id;not null"`
	Name         string    `gorm:"column:name;size:255;not null"`
	Type         string    `gorm:"column:type;size:20;not null"` // prod/dev/test/staging
	Memo         string    `gorm:"column:memo;size:256"`
	DisplayOrder int       `gorm:"column:display_order;type:int;not null;default:0"` // 显示顺序
	Protected    bool      `gorm:"column:protected;not null;default:false"`          // 保护标记: true=不允许删除/修改type
	Creator      string    `gorm:"column:creator;size:64;not null"`
	Reviser      string    `gorm:"column:reviser;size:64;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

func (Environment) TableName() string { return "environments" }

// ScopeMigrationTask 记录每张表的回填进度，支持断点续跑和幂等重试。
type ScopeMigrationTask struct {
	TargetTable string    `gorm:"column:table_name;type:varchar(128);not null;primaryKey"`    // 目标表名
	ScopeType   string    `gorm:"column:scope_type;type:varchar(32);not null"`                // project_scope / env_scope / mixed_scope
	LastID      uint64    `gorm:"column:last_id;type:bigint(20) unsigned;not null;default:0"` // 已回填到的最大主键 ID
	Status      string    `gorm:"column:status;type:varchar(32);not null;default:'pending'"`  // pending / running / completed / failed
	UpdatedAt   time.Time `gorm:"column:updated_at;not null"`
	ErrorMsg    string    `gorm:"column:error_msg;type:text"` // 最近一次失败原因
}

func (ScopeMigrationTask) TableName() string { return "scope_migration_tasks" }

const (
	scopeStatusPending   = "pending"
	scopeStatusRunning   = "running"
	scopeStatusCompleted = "completed"
	scopeStatusFailed    = "failed"
)

// =============================================
// Step1: 创建 projects / environments / scope_migration_tasks 表 + 注册 id_generators
// =============================================
func stepCreateProjectEnvTables(tx *gorm.DB) error {
	// --- projects ---
	if err := tx.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").
		AutoMigrate(&Project{}); err != nil {
		return fmt.Errorf("create projects table: %w", err)
	}
	// 唯一索引: key
	if !tx.Migrator().HasIndex("projects", "idx_key") {
		if err := tx.Exec("CREATE UNIQUE INDEX idx_key ON projects (`key`)").Error; err != nil {
			return fmt.Errorf("create unique index on projects: %w", err)
		}
	}
	// 唯一索引：确保一个业务下 is_default = 1 的记录只能有一条
	if !tx.Migrator().HasIndex("projects", "uk_tenantID_bizID_isDefault") {
		sql := "CREATE UNIQUE INDEX uk_tenantID_bizID_isDefault ON projects (tenant_id, biz_id, is_default)"
		if err := tx.Exec(sql).Error; err != nil {
			return fmt.Errorf("create unique index uk_tenantID_bizID_isDefault on projects: %w", err)
		}
	}

	// --- environments ---
	if err := tx.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").
		AutoMigrate(&Environment{}); err != nil {
		return fmt.Errorf("create environments table: %w", err)
	}
	// 唯一索引: tenant_id + biz_id + project_id + name
	if !tx.Migrator().HasIndex("environments", "idx_tenantID_bizID_projectID_name") {
		if errH := tx.Exec("CREATE UNIQUE INDEX idx_tenantID_bizID_projectID_name ON environments (tenant_id, biz_id, project_id, name)").
			Error; errH != nil {
			return fmt.Errorf("create unique index on environments: %w", errH)
		}
	}

	// --- scope_migration_tasks ---
	if err := tx.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").
		AutoMigrate(&ScopeMigrationTask{}); err != nil {
		return fmt.Errorf("create scope_migration_tasks table: %w", err)
	}

	// 注册 id_generators（projects/environments 的 ID 从 id_generators 表分配，与其他业务表一致）
	now := time.Now().Format("2006-01-02 15:04:05")
	if err := tx.Exec("INSERT IGNORE INTO id_generators(resource, max_id, updated_at) VALUES ('projects', 0, ?)", now).Error; err != nil {
		return fmt.Errorf("register projects id_generator: %w", err)
	}
	if err := tx.Exec("INSERT IGNORE INTO id_generators(resource, max_id, updated_at) VALUES ('environments', 0, ?)", now).Error; err != nil {
		return fmt.Errorf("register environments id_generator: %w", err)
	}

	return nil
}

// =============================================
// Step2: 为每个 biz_id 创建默认项目(key=default) 和 默认环境(type=prod)
// =============================================

type BizTenantRecord struct {
	BizID    uint64 `gorm:"column:biz_id"`
	TenantID string `gorm:"column:tenant_id"`
}

func stepInsertDefaultProjectsAndEnvs(tx *gorm.DB) error {
	if err := tx.Exec("SELECT id FROM id_generators WHERE resource IN ('projects', 'environments') FOR UPDATE").Error; err != nil {
		return fmt.Errorf("lock id_generators failed: %w", err)
	}
	// 1. 将所有涉及项目和环境 scope 的表全部纳入统计池中
	allTables := []string{"applications", "hooks", "groups", "credentials", "template_spaces", "template_variables"}

	// 动态构建多表 UNION 的 SQL 语句
	var unionSQLs []string
	for _, table := range allTables {
		unionSQLs = append(unionSQLs, fmt.Sprintf(
			`SELECT DISTINCT biz_id, 
			        CASE WHEN tenant_id IS NULL OR tenant_id = '' THEN 'default' ELSE tenant_id END as tenant_id 
			 FROM %s WHERE biz_id > 0`, quoteTable(table),
		))
	}

	// 最终的外层包裹，做最终的去重
	finalSQL := fmt.Sprintf(
		"SELECT DISTINCT biz_id, tenant_id FROM (%s) AS all_resources",
		strings.Join(unionSQLs, " UNION "),
	)

	var bizTenantList []BizTenantRecord
	if err := tx.Raw(finalSQL).Scan(&bizTenantList).Error; err != nil {
		return fmt.Errorf("query global unique biz/tenant from all tables failed: %w", err)
	}
	if len(bizTenantList) == 0 {
		return nil
	}

	type IDGen struct {
		MaxID uint64 `gorm:"column:max_id"`
	}
	var projGen, envGen IDGen
	if err := tx.Table("id_generators").Set("gorm:query_option", "FOR UPDATE").Where("resource = ?", "projects").First(&projGen).Error; err != nil {
		return fmt.Errorf("query projects id_generator: %w", err)
	}
	if err := tx.Table("id_generators").Set("gorm:query_option", "FOR UPDATE").Where("resource = ?", "environments").First(&envGen).Error; err != nil {
		return fmt.Errorf("query environments id_generator: %w", err)
	}

	nextProjID := projGen.MaxID + 1
	nextEnvID := envGen.MaxID + 1
	now := time.Now()
	systemUser := table.System

	for _, bt := range bizTenantList {
		bizID := uint32(bt.BizID)
		tenantID := bt.TenantID

		// 分配 projID，key 格式: BK-BSCP-XXXXX（主键左侧补零到5位）
		projID := nextProjID
		nextProjID++
		defaultProjectKey := table.GenerateProjectKey(uint32(projID))

		if errI := tx.Exec(
			"INSERT INTO projects (id, tenant_id, biz_id, `key`, name, memo, protected,is_default, creator, reviser, created_at, updated_at)\n"+
				"VALUES (?, ?, ?, ?, ?, '', true, 1, ?, ?, ?, ?)",
			projID, tenantID, bizID, defaultProjectKey, table.DefaultProjectName, systemUser, systemUser, now, now,
		).Error; errI != nil {
			return fmt.Errorf("insert default project for biz %d tenant %s: %w", bizID, tenantID, errI)
		}

		envID := nextEnvID
		nextEnvID++

		if err := tx.Exec(
			`INSERT INTO environments (id, tenant_id, biz_id, project_id, name, `+"`type`"+`, memo, display_order, protected, creator, reviser, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, 'prod', '', 0, true, ?, ?, ?, ?)`,
			envID, tenantID, bizID, projID, table.DefaultEnvName, systemUser, systemUser, now, now,
		).Error; err != nil {
			return fmt.Errorf("insert default environment for biz %d tenant %s: %w", bizID, tenantID, err)
		}
	}

	// 更新 id_generators
	if err := tx.Exec("UPDATE id_generators SET max_id = ?, updated_at = ? WHERE resource = 'projects'", nextProjID-1, now).Error; err != nil {
		return fmt.Errorf("update projects max_id: %w", err)
	}
	if err := tx.Exec("UPDATE id_generators SET max_id = ?, updated_at = ? WHERE resource = 'environments'", nextEnvID-1, now).Error; err != nil {
		return fmt.Errorf("update environments max_id: %w", err)
	}

	return nil
}

// =============================================
// Step3: 为所有需要改动的表添加新字段
//
// 两种作用域:
//   - EnvScope 表: 添加 project_id + environment_id + env_display（仅 applications）
//   - ProjectScope 表: 仅添加 project_id（含 groups/templates/events/audits 等）
// =============================================

// 设计原理：
//
//	GORM Migrator.AddColumn() 需要从 struct 字段的 gorm tag 中读取列的 DDL 约束（类型/长度/NOT NULL/DEFAULT）。
//	因此通用模型必须包含完整的字段定义，仅通过 TableName() 动态切换目标表。
//
// 对应生成的 ALTER TABLE 示例 (以 groups 表为例)：
//
//	ALTER TABLE `groups`
//	  ADD COLUMN `project_id` bigint(20) unsigned NOT NULL DEFAULT 0;
type projectScopeModel struct {
	ProjectID uint32 `gorm:"column:project_id;type:bigint(20) unsigned;not null;default:0"`
}

func newProjectScopeModel() *projectScopeModel {
	return &projectScopeModel{}
}

// EnvScope 列模型：包含 project_id + environment_id (+ applications 专用 env_display)
//
// 对应生成的 ALTER TABLE 示例 (以 applications 表为例)：
// 冗余 environments.name + '-' + environments.type
//
//	ALTER TABLE `applications`
//	  ADD COLUMN `project_id`     bigint(20) unsigned NOT NULL DEFAULT 0,
//	  ADD COLUMN `environment_id` bigint(20) unsigned NOT NULL DEFAULT 0,
//	  ADD COLUMN `env_display`    varchar(280)        NOT NULL DEFAULT '';
type envScopeModel struct {
	ProjectID     uint32 `gorm:"column:project_id;type:bigint(20) unsigned;not null;default:0"`
	EnvironmentID uint32 `gorm:"column:environment_id;type:bigint(20) unsigned;not null;default:0"`
	EnvDisplay    string `gorm:"column:env_display;type:varchar(280);not null;default:''"`
}

func newEnvScopeModel() *envScopeModel {
	return &envScopeModel{}
}

// EnvScope: 添加 project_id + environment_id（applications 额外加 env_display）的表
var envScopeTables = []string{
	"applications",
}

// ProjectScope: 仅添加/回填 project_id 的表
var projectScopeTables = []string{
	"audits", "client_events", "client_querys", "clients",
	"credentials", "events", "groups", "hooks", "template_spaces", "template_variables",
}

func stepAddColumns(tx *gorm.DB) error {
	// ProjectScope: 仅为每张表添加 project_id 列
	// 生成的 SQL 示例: ALTER TABLE `groups` ADD COLUMN `project_id` bigint(20) unsigned NOT NULL DEFAULT 0
	for _, table := range projectScopeTables {
		m := newProjectScopeModel()
		migrator := tx.Table(table).Migrator()
		if !migrator.HasColumn(m, "project_id") {
			if err := migrator.AddColumn(m, "project_id"); err != nil {
				return fmt.Errorf("add project_id to table %s failed: %w", table, err)
			}
		}
	}

	// EnvScope: 添加 project_id + environment_id（applications 额外加 env_display）
	// 生成的 SQL 示例 (applications):
	//   ALTER TABLE `applications`
	//     ADD COLUMN `project_id`     bigint(20) unsigned NOT NULL DEFAULT 0,
	//     ADD COLUMN `environment_id` bigint(20) unsigned NOT NULL DEFAULT 0,
	//     ADD COLUMN `env_display`    varchar(280)        NOT NULL DEFAULT ''
	for _, table := range envScopeTables {
		m := newEnvScopeModel()
		migrator := tx.Table(table).Migrator()
		if !migrator.HasColumn(m, "project_id") {
			if err := migrator.AddColumn(m, "project_id"); err != nil {
				return fmt.Errorf("add project_id to table %s failed: %w", table, err)
			}
		}
		if !migrator.HasColumn(m, "environment_id") {
			if err := migrator.AddColumn(m, "environment_id"); err != nil {
				return fmt.Errorf("add environment_id to table %s failed: %w", table, err)
			}
		}
		// applications 表额外加 env_display 字段（冗余 environments 的 name-type，供前端直接展示）
		if table == "applications" && !migrator.HasColumn(m, "env_display") {
			if err := migrator.AddColumn(m, "env_display"); err != nil {
				return fmt.Errorf("add env_display to table %s failed: %w", table, err)
			}
		}
	}

	return nil
}

// =============================================
// Step4: 调整已有唯一索引（加入 project_id / environment_id）+ 新增二级索引
//
// 原有唯一索引只到 biz 维度（tenant_id + biz_id + name），新增项目/环境维度后：
//   - 不同项目下应允许同名 group/hook/template_space 等 → 唯一键需包含 project_id
//   - 不同环境下应允许同名 application → 唯一键需包含 project_id + environment_id
//
// 处置策略：先删旧唯一索引，再建新唯一索引；同时为 project_id/environment_id 建二级索引。
// =============================================

// indexAdjustment 定义需要调整的唯一索引映射。
type indexAdjustment struct {
	table      string // 目标表名
	oldIdxName string // 旧唯一索引名
	newIdxName string // 新唯一索引名
	newColumns string // 新索引列定义（SQL）
}

// 需要调整唯一索引的表及其新旧索引定义
var projectScopeIndexAdjustments = []indexAdjustment{
	{"groups", "idx_tenantID_bizID_name", "idx_tenantID_bizID_projectID_name",
		"tenant_id, biz_id, project_id, name"},
	{"hooks", "idx_tenantID_bizID_name", "idx_tenantID_bizID_projectID_name",
		"tenant_id, biz_id, project_id, name"},
	{"credentials", "idx_tenantID_bizID_name", "idx_tenantID_bizID_projectID_name",
		"tenant_id, biz_id, project_id, name"},
	{"template_spaces", "idx_tenantID_bizID_name", "idx_tenantID_bizID_projectID_name",
		"tenant_id, biz_id, project_id, name"},
	{"template_variables", "idx_tenantID_bizID_name", "idx_tenantID_bizID_projectID_name",
		"tenant_id, biz_id, project_id, name"},
}

var envScopeIndexAdjustments = []indexAdjustment{
	{"applications", "idx_tenantID_bizID_name", "idx_tenantID_bizID_projectID_envID_name",
		"tenant_id, biz_id, project_id, environment_id, name"},
}

func stepAdjustIndexes(tx *gorm.DB) error {
	// 4a. ProjectScope 表：删旧唯一索引 → 建新唯一索引（含 project_id）
	for _, adj := range projectScopeIndexAdjustments {
		if err := replaceUniqueIndex(tx, adj); err != nil {
			return fmt.Errorf("adjust index on %s: %w", adj.table, err)
		}
	}

	// 4b. EnvScope 表：删旧唯一索引 → 建新唯一索引（含 project_id + environment_id）
	for _, adj := range envScopeIndexAdjustments {
		if err := replaceUniqueIndex(tx, adj); err != nil {
			return fmt.Errorf("adjust index on %s: %w", adj.table, err)
		}
	}

	// 4c. 为所有 ProjectScope 表添加 project_id 二级索引
	for _, table := range projectScopeTables {
		idxName := "idx_project_id"
		if !tx.Migrator().HasIndex(table, idxName) {
			if err := tx.Exec(fmt.Sprintf("CREATE INDEX %s ON %s (project_id)", idxName, quoteTable(table))).Error; err != nil {
				return fmt.Errorf("create index %s on %s: %w", idxName, table, err)
			}
		}
	}

	// 4d. 为 EnvScope 表添加 project_id + environment_id 二级索引
	for _, table := range envScopeTables {
		idxName := "idx_project_id"
		if !tx.Migrator().HasIndex(table, idxName) {
			if err := tx.Exec(fmt.Sprintf("CREATE INDEX %s ON %s (project_id)", idxName, quoteTable(table))).Error; err != nil {
				return fmt.Errorf("create index %s on %s: %w", idxName, table, err)
			}
		}
		idxName = "idx_environment_id"
		if !tx.Migrator().HasIndex(table, idxName) {
			if err := tx.Exec(fmt.Sprintf("CREATE INDEX %s ON %s (environment_id)", idxName, quoteTable(table))).Error; err != nil {
				return fmt.Errorf("create index %s on %s: %w", idxName, table, err)
			}
		}
	}

	return nil
}

// replaceUniqueIndex 删旧唯一索引，建新唯一索引（幂等）。
func replaceUniqueIndex(tx *gorm.DB, adj indexAdjustment) error {
	if tx.Migrator().HasIndex(adj.table, adj.oldIdxName) {
		if err := tx.Migrator().DropIndex(adj.table, adj.oldIdxName); err != nil {
			return fmt.Errorf("drop old index %s: %w", adj.oldIdxName, err)
		}
	}
	if !tx.Migrator().HasIndex(adj.table, adj.newIdxName) {
		if err := tx.Exec(
			fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s)", adj.newIdxName, quoteTable(adj.table), adj.newColumns),
		).Error; err != nil {
			return fmt.Errorf("create new index %s: %w", adj.newIdxName, err)
		}
	}
	return nil
}

//
// 核心机制:
//   - 使用 scope_migration_tasks 表记录每张表的回填进度 (last_id + status)
//   - 每张表按主键 id > last_id ORDER BY id LIMIT batchSize 分批 UPDATE
//   - 每批独立事务提交并更新进度表
//   - 失败后 status=failed，下次执行从 last_id 续跑
//   - 已完成的表自动跳过 (status=completed)
//   - 多实例安全: 通过 INSERT IGNORE 初始化任务 + status 字段防并发
// =============================================

const backfillBatchSize = 1000

func quoteTable(name string) string {
	return "`" + name + "`"
}

type batchIDs struct {
	ID uint64 `gorm:"column:id"`
}

func stepBackfillDefaultValues(db *gorm.DB) error {
	// 4a. 初始化所有待回填任务记录（INSERT IGNORE 保证幂等）
	if err := initBackfillTasks(db); err != nil {
		return fmt.Errorf("init backfill tasks failed: %w", err)
	}

	// 4b. 按顺序逐表执行分批回填，每张表独立——单表失败记入 scope_migration_tasks 后继续下一张
	var failedTables []string

	for _, table := range projectScopeTables {
		if err := backfillProjectScopeTable(db, table); err != nil {
			fmt.Printf("[WARN] backfill project_id on %s failed (will retry next run): %v\n", table, err)
			failedTables = append(failedTables, table)
		}
	}

	for _, table := range envScopeTables {
		if err := backfillEnvScopeTable(db, table); err != nil {
			fmt.Printf("[WARN] backfill project_id+env_id on %s failed (will retry next run): %v\n", table, err)
			failedTables = append(failedTables, table)
		}
	}

	if len(failedTables) > 0 {
		return fmt.Errorf("%d table(s) backfill failed (retryable via scope_migration_tasks): %v",
			len(failedTables), failedTables)
	}

	return nil
}

// initBackfillTasks 向 scope_migration_tasks 插入所有待回填表的初始记录。
func initBackfillTasks(tx *gorm.DB) error {
	now := time.Now()

	for _, t := range projectScopeTables {
		err := tx.Exec(
			fmt.Sprintf(`INSERT IGNORE INTO scope_migration_tasks (table_name, scope_type, last_id, status, updated_at) VALUES (?, 'project_scope', 0, '%s', ?)`,
				scopeStatusPending), t, now,
		).Error
		if err != nil {
			return fmt.Errorf("init task for %s: %w", t, err)
		}
	}

	for _, t := range envScopeTables {
		err := tx.Exec(
			fmt.Sprintf(`INSERT IGNORE INTO scope_migration_tasks (table_name, scope_type, last_id, status, updated_at) VALUES (?, 'env_scope', 0, '%s', ?)`,
				scopeStatusPending), t, now,
		).Error
		if err != nil {
			return fmt.Errorf("init task for %s: %w", t, err)
		}
	}

	return nil
}

// updateTaskStatus 更新任务进度。
func updateTaskStatus(tx *gorm.DB, tableName string, lastID uint64, status, errorMsg string) error {
	now := time.Now()
	result := tx.Exec(
		`UPDATE scope_migration_tasks
		 SET last_id = ?, status = ?, error_msg = ?, updated_at = ?
		 WHERE table_name = ?`,
		lastID, status, errorMsg, now, tableName,
	)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("task row not found for table %s (possible race)", tableName)
	}
	return nil
}

// backfillProjectScopeTable 对 ProjectScope 表进行分批回填 project_id。
func backfillProjectScopeTable(db *gorm.DB, tableName string) error {
	var task ScopeMigrationTask
	if err := db.Where("table_name = ?", tableName).First(&task).Error; err != nil {
		return fmt.Errorf("query task for %s: %w", tableName, err)
	}
	if task.Status == scopeStatusCompleted {
		return nil
	}

	if err := updateTaskStatus(db, tableName, task.LastID, scopeStatusRunning, ""); err != nil {
		return err
	}

	for {
		var ids []batchIDs
		// 1. 单纯通过 ID 分批，确保能扫完整个表，防止因为某批次未更新而中断
		if err := db.Raw(fmt.Sprintf(`SELECT id FROM %s WHERE id > ? ORDER BY id LIMIT %d`,
			quoteTable(tableName), backfillBatchSize), task.LastID).Scan(&ids).Error; err != nil {
			return fmt.Errorf("query batch ids for %s: %w", tableName, err)
		}

		if len(ids) == 0 {
			break
		}

		idList := make([]uint64, len(ids))
		for i, row := range ids {
			idList[i] = row.ID
		}
		maxIDInBatch := idList[len(idList)-1]

		// 2. 使用 CASE WHEN 将 NULL 和空字符串 '' 统一收拢为 'default'
		sql := fmt.Sprintf(`
			UPDATE %s t
			INNER JOIN projects p ON p.biz_id = t.biz_id 
				AND CASE WHEN p.tenant_id IS NULL OR p.tenant_id = '' THEN 'default' ELSE p.tenant_id END 
				  = CASE WHEN t.tenant_id IS NULL OR t.tenant_id = '' THEN 'default' ELSE t.tenant_id END
				AND p.key = CONCAT('BK-BSCP-', LPAD(p.id, 5, '0'))
			SET t.project_id = p.id
			WHERE t.id IN (?) AND (t.project_id = 0 OR t.project_id IS NULL)`, quoteTable(tableName))

		result := db.Exec(sql, idList)
		if result.Error != nil {
			_ = updateTaskStatus(db, tableName, task.LastID, scopeStatusFailed, result.Error.Error())
			return fmt.Errorf("batch update on %s at last_id=%d: %w", tableName, task.LastID, result.Error)
		}

		// 3. 无论当前批次有没有实际更新到行（affected == 0），
		// 只要这一批次的 ID 处理过了，游标 task.LastID 就要强行向前推进，否则会死循环！
		if err := updateTaskStatus(db, tableName, maxIDInBatch, scopeStatusRunning, ""); err != nil {
			return err
		}
		task.LastID = maxIDInBatch
	}

	return updateTaskStatus(db, tableName, task.LastID, scopeStatusCompleted, "")
}

// backfillEnvScopeTable 对 EnvScope 表进行分批回填 project_id + environment_id (+ env_display for applications)。
func backfillEnvScopeTable(db *gorm.DB, tableName string) error {
	var task ScopeMigrationTask
	if err := db.Where("table_name = ?", tableName).First(&task).Error; err != nil {
		return fmt.Errorf("query task for %s: %w", tableName, err)
	}
	if task.Status == scopeStatusCompleted {
		return nil
	}

	if err := updateTaskStatus(db, tableName, task.LastID, scopeStatusRunning, ""); err != nil {
		return err
	}

	for {
		var ids []batchIDs
		// 1. 使用 ID 驱动分批，确保能扫描全表
		if err := db.Raw(fmt.Sprintf(
			`SELECT id FROM %s WHERE id > ? ORDER BY id LIMIT %d`,
			quoteTable(tableName), backfillBatchSize,
		), task.LastID).Scan(&ids).Error; err != nil {
			return fmt.Errorf("query batch ids for %s: %w", tableName, err)
		}
		if len(ids) == 0 {
			break
		}

		idList := make([]uint64, len(ids))
		for i, row := range ids {
			idList[i] = row.ID
		}
		maxIDInBatch := idList[len(idList)-1]

		// 根据目标表选择不同的回填 SQL
		var sql string
		if tableName == "applications" {
			// 2. applications 表：使用 CASE WHEN 统一收拢 NULL 和 ''，并在 WHERE 中过滤未回填数据
			sql = fmt.Sprintf(`
				UPDATE %s t
				INNER JOIN projects p ON p.biz_id = t.biz_id 
					AND CASE WHEN p.tenant_id IS NULL OR p.tenant_id = '' THEN 'default' ELSE p.tenant_id END 
					  = CASE WHEN t.tenant_id IS NULL OR t.tenant_id = '' THEN 'default' ELSE t.tenant_id END
					AND p.key = CONCAT('BK-BSCP-', LPAD(p.id, 5, '0'))
				INNER JOIN environments e ON e.project_id = p.id
					AND CASE WHEN e.tenant_id IS NULL OR e.tenant_id = '' THEN 'default' ELSE e.tenant_id END 
					  = CASE WHEN p.tenant_id IS NULL OR p.tenant_id = '' THEN 'default' ELSE p.tenant_id END
					AND e.type = 'prod'
					AND e.name = 'default'
				SET t.project_id = p.id, t.environment_id = e.id, t.env_display = CONCAT(e.name, '-', e.type)
				WHERE t.id IN (?) 
				  AND ((t.project_id = 0 OR t.project_id IS NULL) OR (t.environment_id = 0 OR t.environment_id IS NULL))`, quoteTable(tableName))
		} else {
			// 3. 其他 EnvScope 表：同样做 CASE WHEN 兼容
			sql = fmt.Sprintf(`
				UPDATE %s t
				INNER JOIN projects p ON p.biz_id = t.biz_id 
					AND CASE WHEN p.tenant_id IS NULL OR p.tenant_id = '' THEN 'default' ELSE p.tenant_id END 
					  = CASE WHEN t.tenant_id IS NULL OR t.tenant_id = '' THEN 'default' ELSE t.tenant_id END
					AND p.key = CONCAT('BK-BSCP-', LPAD(p.id, 5, '0'))
				INNER JOIN environments e ON e.project_id = p.id
					AND CASE WHEN e.tenant_id IS NULL OR e.tenant_id = '' THEN 'default' ELSE e.tenant_id END 
					  = CASE WHEN p.tenant_id IS NULL OR p.tenant_id = '' THEN 'default' ELSE p.tenant_id END
					AND e.type = 'prod'
					AND e.name = 'default'
				SET t.project_id = p.id, t.environment_id = e.id
				WHERE t.id IN (?) 
				  AND ((t.project_id = 0 OR t.project_id IS NULL) OR (t.environment_id = 0 OR t.environment_id IS NULL))`, quoteTable(tableName))
		}

		result := db.Exec(sql, idList)
		if result.Error != nil {
			_ = updateTaskStatus(db, tableName, task.LastID, scopeStatusFailed, result.Error.Error())
			return fmt.Errorf("batch update on %s at last_id=%d: %w", tableName, task.LastID, result.Error)
		}

		// 4. 无论当前批次有没有实际更新到行（affected == 0），
		// 无论当前批次是否更新成功，LastID 游标必须强制向前推，防止因遇到不符合更新条件的数据而死锁或中断。
		if err := updateTaskStatus(db, tableName, maxIDInBatch, scopeStatusRunning, ""); err != nil {
			return err
		}
		task.LastID = maxIDInBatch
	}

	return updateTaskStatus(db, tableName, task.LastID, scopeStatusCompleted, "")
}

// =============================================
// Restore Indexes (Down): 恢复旧唯一索引 + 删除二级索引
// =============================================

// indexRestoreDef 定义需要恢复的旧唯一索引。
type indexRestoreDef struct {
	table      string
	newIdxName string // 本迁移创建的新唯一索引（要删的）
	oldIdxName string // 旧唯一索引名（要恢复的）
	oldColumns string // 旧索引列定义
}

var projectScopeIndexRestores = []indexRestoreDef{
	{"groups", "idx_tenantID_bizID_projectID_name", "idx_tenantID_bizID_name",
		"tenant_id, biz_id, name"},
	{"hooks", "idx_tenantID_bizID_projectID_name", "idx_tenantID_bizID_name",
		"tenant_id, biz_id, name"},
	{"credentials", "idx_tenantID_bizID_projectID_name", "idx_tenantID_bizID_name",
		"tenant_id, biz_id, name"},
	{"template_spaces", "idx_tenantID_bizID_projectID_name", "idx_tenantID_bizID_name",
		"tenant_id, biz_id, name"},
	{"template_variables", "idx_tenantID_bizID_projectID_name", "idx_tenantID_bizID_name",
		"tenant_id, biz_id, name"},
}

var envScopeIndexRestores = []indexRestoreDef{
	{"applications", "idx_tenantID_bizID_projectID_envID_name", "idx_tenantID_bizID_name",
		"tenant_id, biz_id, name"},
}

func stepRestoreIndexes(tx *gorm.DB) error {
	for _, r := range projectScopeIndexRestores {
		if err := restoreUniqueIndex(tx, r); err != nil {
			return fmt.Errorf("restore index on %s: %w", r.table, err)
		}
	}
	for _, r := range envScopeIndexRestores {
		if err := restoreUniqueIndex(tx, r); err != nil {
			return fmt.Errorf("restore index on %s: %w", r.table, err)
		}
	}

	// 删除二级索引
	for _, table := range projectScopeTables {
		dropIndexIfExists(tx, table, "idx_project_id")
	}
	for _, table := range envScopeTables {
		dropIndexIfExists(tx, table, "idx_project_id")
		dropIndexIfExists(tx, table, "idx_environment_id")
	}

	return nil
}

// restoreUniqueIndex 删新索引，恢复旧唯一索引（幂等）。
func restoreUniqueIndex(tx *gorm.DB, r indexRestoreDef) error {
	if tx.Migrator().HasIndex(r.table, r.newIdxName) {
		if err := tx.Migrator().DropIndex(r.table, r.newIdxName); err != nil {
			return fmt.Errorf("drop new index %s: %w", r.newIdxName, err)
		}
	}
	if !tx.Migrator().HasIndex(r.table, r.oldIdxName) {
		if err := tx.Exec(
			fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s)", r.oldIdxName, quoteTable(r.table), r.oldColumns),
		).Error; err != nil {
			return fmt.Errorf("restore old index %s: %w", r.oldIdxName, err)
		}
	}
	return nil
}

func dropIndexIfExists(tx *gorm.DB, table, idxName string) {
	if tx.Migrator().HasIndex(table, idxName) {
		_ = tx.Migrator().DropIndex(table, idxName)
	}
}

// =============================================
// Drop Columns (Down)
// =============================================
func stepDropColumns(tx *gorm.DB) error {
	// EnvScope 表: 删除 project_id + environment_id（applications 额外删除 env_display）
	// 生成的 SQL 示例 (applications):
	//   ALTER TABLE `applications` DROP COLUMN `project_id`, DROP COLUMN `environment_id`, DROP COLUMN `env_display`
	for _, table := range envScopeTables {
		m := newEnvScopeModel()
		migrator := tx.Table(table).Migrator()
		cols := []string{"project_id", "environment_id"}
		if table == "applications" {
			cols = append(cols, "env_display")
		}
		for _, col := range cols {
			if migrator.HasColumn(m, col) {
				if err := migrator.DropColumn(m, col); err != nil {
					return fmt.Errorf("drop column %s from %s: %w", col, table, err)
				}
			}
		}
	}

	// ProjectScope 表: 仅删除 project_id
	// 生成的 SQL 示例: ALTER TABLE `groups` DROP COLUMN `project_id`
	for _, table := range projectScopeTables {
		m := newProjectScopeModel()
		migrator := tx.Table(table).Migrator()
		if migrator.HasColumn(m, "project_id") {
			if err := migrator.DropColumn(m, "project_id"); err != nil {
				return fmt.Errorf("drop column project_id from %s: %w", table, err)
			}
		}
	}

	return nil
}
