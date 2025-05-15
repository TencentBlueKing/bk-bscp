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

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/cmd/data-service/db-migration/migrator"
)

// nolint:funlen
var allModels = func() []any {
	// AppTemplateBindings mapped from table <app_template_bindings>
	type AppTemplateBindings struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// AppTemplateVariables mapped from table <app_template_variables>
	type AppTemplateVariables struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// Applications mapped from table <applications>
	type Applications struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// ArchivedApps mapped from table <archived_apps>
	type ArchivedApps struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// Audits mapped from table <audits>
	type Audits struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// ClientEvents mapped from table <client_events>
	type ClientEvents struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// ClientQuerys mapped from table <client_querys>
	type ClientQuerys struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// Clients mapped from table <clients>
	type Clients struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// Commit mapped from table <commits>
	type Commit struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// ConfigItems mapped from table <config_items>
	type ConfigItems struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// Configs mapped from table <configs>
	type Configs struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// Contents mapped from table <contents>
	type Contents struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// CredentialScopes mapped from table <credential_scopes>
	type CredentialScopes struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// Credentials mapped from table <credentials>
	type Credentials struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// CurrentPublishedStrategies mapped from table <current_published_strategies>
	type CurrentPublishedStrategies struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// CurrentReleasedInstances mapped from table <current_released_instances>
	type CurrentReleasedInstances struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// DataSourceContents mapped from table <data_source_contents>
	type DataSourceContents struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// DataSourceInfos mapped from table <data_source_infos>
	type data_source_infos struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// DataSourceMappings mapped from table <data_source_mappings>
	type DataSourceMappings struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// Events mapped from table <events>
	type Events struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// GroupAppBinds mapped from table <group_app_binds>
	type GroupAppBinds struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// Groups mapped from table <groups>
	type Groups struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// HookRevisions mapped from table <hook_revisions>
	type HookRevisions struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// Hooks mapped from table <hooks>
	type Hooks struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// Kvs mapped from table <kvs>
	type Kvs struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// PublishedStrategyHistories mapped from table <published_strategy_histories>
	type PublishedStrategyHistories struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// ReleasedAppTemplateVariables mapped from table <released_app_template_variables>
	type ReleasedAppTemplateVariables struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// ReleasedAppTemplates mapped from table <released_app_templates>
	type ReleasedAppTemplates struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// ReleasedConfigItems mapped from table <released_config_items>
	type ReleasedConfigItems struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// ReleasedGroups mapped from table <released_groups>
	type ReleasedGroups struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// ReleasedHooks mapped from table <released_hooks>
	type ReleasedHooks struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// ReleasedKvs mapped from table <released_kvs>
	type ReleasedKvs struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// ReleasedTableContents mapped from table <released_table_contents>
	type ReleasedTableContents struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// Releases mapped from table <releases>
	type Releases struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// ResourceLocks mapped from table <resource_locks>
	type ResourceLocks struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// ShardingBizs mapped from table <sharding_bizs>
	type ShardingBizs struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// Strategies mapped from table <strategies>
	type Strategies struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// StrategySets mapped from table <strategy_sets>
	type StrategySets struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// TemplateRevisions mapped from table <template_revisions>
	type TemplateRevisions struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// TemplateSets mapped from table <template_sets>
	type TemplateSets struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// TemplateSpaces mapped from table <template_spaces>
	type TemplateSpaces struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// TemplateVariables mapped from table <template_variables>
	type TemplateVariables struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// Templates mapped from table <templates>
	type Templates struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// UserGroupPrivileges mapped from table <user_group_privileges>
	type UserGroupPrivileges struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	// UserPrivileges mapped from table <user_privileges>
	type UserPrivileges struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(255);not null;default:default" json:"tenant_id"`
	}

	return []any{
		&AppTemplateBindings{},
		&AppTemplateVariables{},
		&Applications{},
		&ArchivedApps{},
		&Audits{},
		&ClientEvents{},
		&ClientQuerys{},
		&Clients{},
		&Commit{},
		&ConfigItems{},
		&Configs{},
		&Contents{},
		&CredentialScopes{},
		&Credentials{},
		&CurrentPublishedStrategies{},
		&CurrentReleasedInstances{},
		&DataSourceContents{},
		&data_source_infos{},
		&DataSourceMappings{},
		&Events{},
		&GroupAppBinds{},
		&Groups{},
		&HookRevisions{},
		&Hooks{},
		&Kvs{},
		&PublishedStrategyHistories{},
		&ReleasedAppTemplateVariables{},
		&ReleasedAppTemplates{},
		&ReleasedConfigItems{},
		&ReleasedGroups{},
		&ReleasedHooks{},
		&ReleasedKvs{},
		&ReleasedTableContents{},
		&Releases{},
		&ResourceLocks{},
		&ShardingBizs{},
		&Strategies{},
		&StrategySets{},
		&TemplateRevisions{},
		&TemplateSets{},
		&TemplateSpaces{},
		&TemplateVariables{},
		&Templates{},
		&UserGroupPrivileges{},
		&UserPrivileges{},
	}
}

func init() {
	// add current migration to migrator
	migrator.GetMigrator().AddMigration(&migrator.Migration{
		Version: "20250427100034",
		Name:    "20250427100034_add_tenantID_column",
		Mode:    migrator.GormMode,
		Up:      mig20250427100034Up,
		Down:    mig20250427100034Down,
	})
}

// mig20250427100034Up for up migration
func mig20250427100034Up(tx *gorm.DB) error {
	// Step 1: 删除旧索引
	if err := dropOldIndexes(tx); err != nil {
		return err
	}

	// Step 2: 添加 TenantID 字段
	if err := addTenantIDColumn(tx); err != nil {
		return err
	}

	// Step 3: 添加新索引
	if err := createNewIndexes(tx); err != nil {
		return err
	}

	return nil
}

// mig20250427100034Down for down migration
func mig20250427100034Down(tx *gorm.DB) error {
	// Step 1: 删除新索引
	if err := dropNewIndexes(tx); err != nil {
		return err
	}

	// Step 2: 删除 TenantID 字段
	if err := dropTenantIDColumn(tx); err != nil {
		return err
	}

	// Step 3: 恢复旧索引
	if err := oldIndexesToRestore(tx); err != nil {
		return err
	}

	return nil
}

// 删除旧索引
func dropOldIndexes(tx *gorm.DB) error {
	indexesToDrop := map[string][]string{
		"app_template_bindings":           {"idx_bizID_appID"},
		"app_template_variables":          {"idx_bizID_appID"},
		"applications":                    {"idx_bizID_name"},
		"archived_apps":                   {"idx_bizID_appID"},
		"audits":                          {"idx_bizID_appID_createdAt"},
		"client_querys":                   {"idx_bizID_appID_creator"},
		"commits":                         {"idx_bizID_appID_cfgID"},
		"config_items":                    {"idx_bizID_appID_name"},
		"contents":                        {"idx_bizID_appID_cfgID"},
		"credentials":                     {"idx_bizID_name"},
		"current_published_strategies":    {"idx_strategyID", "idx_bizID_appID", "idx_bizID_releaseID"},
		"current_released_instances":      {"idx_appID_uid", "idx_bizID_appID"},
		"events":                          {"idx_resource_bizID"},
		"group_app_binds":                 {"idx_groupID_appID_bizID"},
		"groups":                          {"idx_bizID_name", "idx_bizID"},
		"hook_revisions":                  {"idx_bizID_revisionName"},
		"hooks":                           {"idx_bizID_name"},
		"kvs":                             {"idx_bizID_appID_key_kvState"},
		"published_strategy_histories":    {"idx_bizID_appID_setID_strategyID", "idx_bizID_appID_setID_namespace"},
		"released_app_template_variables": {"idx_releaseID", "idx_bizID_appID"},
		"released_app_templates":          {"idx_bizID_appID_relID"},
		"released_config_items":           {"idx_bizID_appID_relID"},
		"released_groups":                 {"idx_groupID_appID_bizID"},
		"released_hooks":                  {"idx_appID_releaseID_hookType"},
		"released_kvs":                    {"relID_key", "idx_bizID_appID_ID"},
		"releases":                        {"idx_bizID_appID_name", "idx_bizID_appID"},
		"resource_locks":                  {"idx_bizID_resType_resKey"},
		"strategies":                      {"idx_bizID_appID"},
		"strategy_sets":                   {"idx_appID_name", "idx_bizID_id", "idx_bizID_appID"},
		"template_revisions":              {"idx_bizID_tempID_revName"},
		"template_sets":                   {"idx_bizID_tempSpaID_name"},
		"template_spaces":                 {"idx_bizID_name"},
		"template_variables":              {"idx_bizID_name"},
		"templates":                       {"idx_bizID_tempSpaID_name_path"},
	}

	for tableName, indexes := range indexesToDrop {
		for _, index := range indexes {
			// 如果索引不存在，跳过；否则返回错误
			if !tx.Migrator().HasIndex(tableName, index) {
				continue
			}
			if err := tx.Migrator().DropIndex(tableName, index); err != nil {
				return fmt.Errorf("failed to drop index %s on table %s: %w", index, tableName, err)
			}
		}
	}
	return nil
}

// 恢复旧索引
func oldIndexesToRestore(tx *gorm.DB) error {
	oldIndexesToRestore := []struct {
		table     string
		indexName string
		column    string
		unique    bool
	}{
		{"app_template_bindings", "idx_bizID_appID", "biz_id,app_id", true},
		{"app_template_variables", "idx_bizID_appID", "biz_id,app_id", true},
		{"applications", "idx_bizID_name", "biz_id,name", true},
		{"archived_apps", "idx_bizID_appID", "biz_id,app_id", true},
		{"audits", "idx_bizID_appID_createdAt", "biz_id,app_id,created_at", false},
		{"client_querys", "idx_bizID_appID_creator", "biz_id,app_id,creator", false},
		{"commits", "idx_bizID_appID_cfgID", "biz_id,app_id,config_item_id", false},
		{"config_items", "idx_bizID_appID_name", "biz_id,app_id,path,name", true},
		{"contents", "idx_bizID_appID_cfgID", "biz_id,app_id,config_item_id", false},
		{"credentials", "idx_bizID_name", "biz_id,name", true},
		{"current_published_strategies", "idx_strategyID", "strategy_id", true},
		{"current_published_strategies", "idx_bizID_appID", "biz_id,app_id", false},
		{"current_published_strategies", "idx_bizID_releaseID", "biz_id,release_id", false},
		{"current_released_instances", "idx_appID_uid", "app_id,uid", true},
		{"current_released_instances", "idx_bizID_appID", "biz_id,app_id", false},
		{"events", "idx_resource_bizID", "resource,biz_id", false},
		{"group_app_binds", "idx_groupID_appID_bizID", "group_id,biz_id,app_id", false},
		{"groups", "idx_bizID_name", "biz_id,name", true},
		{"groups", "idx_bizID", "biz_id", false},
		{"hook_revisions", "idx_bizID_revisionName", "hook_id,name", true},
		{"hooks", "idx_bizID_name", "biz_id,name", true},
		{"kvs", "idx_bizID_appID_key_kvState", "`key`,kv_state,biz_id,app_id", true},
		{"published_strategy_histories", "idx_bizID_appID_setID_strategyID",
			"biz_id,app_id,strategy_set_id,strategy_id", false},
		{"published_strategy_histories", "idx_bizID_appID_setID_namespace",
			"biz_id,app_id,strategy_set_id,namespace", false},
		{"released_app_template_variables", "idx_releaseID", "release_id", true},
		{"released_app_template_variables", "idx_bizID_appID", "biz_id,app_id", false},
		{"released_app_templates", "idx_bizID_appID_relID", "biz_id,app_id,release_id", false},
		{"released_config_items", "idx_bizID_appID_relID", "biz_id,app_id,release_id", false},
		{"released_groups", "idx_groupID_appID_bizID", "group_id,biz_id,app_id", false},
		{"released_hooks", "idx_appID_releaseID_hookType", "app_id,release_id,hook_type", true},
		{"released_kvs", "relID_key", "`key`,release_id", true},
		{"released_kvs", "idx_bizID_appID_ID", "biz_id,app_id,release_id", false},
		{"releases", "idx_bizID_appID_name", "biz_id,app_id,name", true},
		{"releases", "idx_bizID_appID", "biz_id,app_id", false},
		{"resource_locks", "idx_bizID_resType_resKey", "biz_id,res_type,res_key", true},
		{"strategies", "idx_bizID_appID", "biz_id,app_id", false},
		{"strategy_sets", "idx_appID_name", "app_id,name", true},
		{"strategy_sets", "idx_bizID_id", "biz_id,id", true},
		{"strategy_sets", "idx_bizID_appID", "biz_id,app_id", false},
		{"template_revisions", "idx_bizID_tempID_revName", "biz_id,template_id,revision_name", true},
		{"template_sets", "idx_bizID_tempSpaID_name", "biz_id,template_space_id,name", true},
		{"template_spaces", "idx_bizID_name", "biz_id,name", true},
		{"template_variables", "idx_bizID_name", "biz_id,name", true},
		{"templates", "idx_bizID_tempSpaID_name_path", "biz_id,template_space_id,name,path", true},
	}

	for _, idx := range oldIndexesToRestore {
		// 索引存在跳过
		if tx.Migrator().HasIndex(idx.table, idx.indexName) {
			continue
		}
		if err := tx.Migrator().CreateIndex(idx.table, idx.indexName); err != nil {
			if err := tx.Exec(fmt.Sprintf("CREATE %s INDEX %s ON `%s` (%s)",
				func() string {
					if idx.unique {
						return "UNIQUE"
					}
					return ""
				}(), idx.indexName, idx.table, idx.column)).Error; err != nil {
				return fmt.Errorf("failed to create index %s on table %s: %w", idx.indexName, idx.table, err)
			}
		}
	}

	return nil
}

type indexField struct {
	Column string // 列名
	Prefix int    // 前缀长度，0 表示不加
}

type indexDef struct {
	Table     string
	IndexName string
	Fields    []indexField
	Unique    bool
}

// 创建新索引
// nolint:funlen
func createNewIndexes(tx *gorm.DB) error {
	var indexes = []indexDef{
		{
			Table:     "app_template_bindings",
			IndexName: "idx_tenantID_bizID_appID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}},
			Unique:    true,
		},
		{
			Table:     "app_template_variables",
			IndexName: "idx_tenantID_bizID_appID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}},
			Unique:    true,
		},
		{
			Table:     "applications",
			IndexName: "idx_tenantID_bizID_name",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"name", 0}},
			Unique:    true,
		},
		{
			Table:     "archived_apps",
			IndexName: "idx_tenantID_bizID_appID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}},
			Unique:    true,
		},
		{
			Table:     "audits",
			IndexName: "idx_tenantID_bizID_appID_createdAt",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}, {"created_at", 0}},
			Unique:    false,
		},
		{
			Table:     "client_querys",
			IndexName: "idx_tenantID_bizID_appID_creator",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}, {"creator", 0}},
			Unique:    false,
		},
		{
			Table:     "commits",
			IndexName: "idx_tenantID_bizID_appID_cfgID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}, {"config_item_id", 0}},
			Unique:    false,
		},
		{
			Table:     "config_items",
			IndexName: "idx_tenantID_bizID_appID_name",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}, {"path", 100}, {"name", 100}},
			Unique:    true,
		},
		{
			Table:     "contents",
			IndexName: "idx_tenantID_bizID_appID_cfgID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}, {"config_item_id", 0}},
			Unique:    false,
		},
		{
			Table:     "credentials",
			IndexName: "idx_tenantID_bizID_name",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"name", 0}},
			Unique:    true,
		},
		{
			Table:     "current_published_strategies",
			IndexName: "idx_tenantID_strategyID",
			Fields:    []indexField{{"tenant_id", 0}, {"strategy_id", 0}},
			Unique:    true,
		},
		{
			Table:     "current_published_strategies",
			IndexName: "idx_tenantID_bizID_appID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}},
			Unique:    false,
		},
		{
			Table:     "current_published_strategies",
			IndexName: "idx_tenantID_bizID_releaseID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"release_id", 0}},
			Unique:    false,
		},
		{
			Table:     "current_released_instances",
			IndexName: "idx_tenantID_appID_uid",
			Fields:    []indexField{{"tenant_id", 0}, {"app_id", 0}, {"uid", 0}},
			Unique:    true,
		},
		{
			Table:     "current_released_instances",
			IndexName: "idx_tenantID_bizID_appID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}},
			Unique:    false,
		},
		{
			Table:     "events",
			IndexName: "idx_tenantID_resource_bizID",
			Fields:    []indexField{{"tenant_id", 0}, {"resource", 0}, {"biz_id", 0}},
			Unique:    false,
		},
		{
			Table:     "group_app_binds",
			IndexName: "idx_tenantID_groupID_appID_bizID",
			Fields:    []indexField{{"tenant_id", 0}, {"group_id", 0}, {"biz_id", 0}, {"app_id", 0}},
			Unique:    false,
		},
		{
			Table:     "groups",
			IndexName: "idx_tenantID_bizID_name",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"name", 0}},
			Unique:    true,
		},
		{
			Table:     "groups",
			IndexName: "idx_tenantID_bizID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}},
			Unique:    false,
		},
		{
			Table:     "hook_revisions",
			IndexName: "idx_tenantID_bizID_revisionName",
			Fields:    []indexField{{"tenant_id", 0}, {"hook_id", 0}, {"name", 0}},
			Unique:    true,
		},
		{
			Table:     "hooks",
			IndexName: "idx_tenantID_bizID_name",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"name", 0}},
			Unique:    true,
		},
		{
			Table:     "kvs",
			IndexName: "idx_tenantID_bizID_appID_key_kvState",
			Fields:    []indexField{{"tenant_id", 0}, {"key", 0}, {"kv_state", 0}, {"biz_id", 0}, {"app_id", 0}},
			Unique:    true,
		},
		{
			Table:     "published_strategy_histories",
			IndexName: "idx_tenantID_bizID_appID_setID_strategyID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}, {"strategy_set_id", 0}, {"strategy_id", 0}},
			Unique:    false,
		},
		{
			Table:     "published_strategy_histories",
			IndexName: "idx_tenantID_bizID_appID_setID_namespace",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}, {"strategy_set_id", 0}, {"namespace", 0}},
			Unique:    false,
		},
		{
			Table:     "released_app_template_variables",
			IndexName: "idx_tenantID_releaseID",
			Fields:    []indexField{{"tenant_id", 0}, {"release_id", 0}},
			Unique:    true,
		},
		{
			Table:     "released_app_template_variables",
			IndexName: "idx_tenantID_bizID_appID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}},
			Unique:    false,
		},
		{
			Table:     "released_app_templates",
			IndexName: "idx_tenantID_bizID_appID_relID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}, {"release_id", 0}},
			Unique:    false,
		},
		{
			Table:     "released_config_items",
			IndexName: "idx_tenantID_bizID_appID_relID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}, {"release_id", 0}},
			Unique:    false,
		},
		{
			Table:     "released_groups",
			IndexName: "idx_tenantID_groupID_appID_bizID",
			Fields:    []indexField{{"tenant_id", 0}, {"group_id", 0}, {"biz_id", 0}, {"app_id", 0}},
			Unique:    false,
		},
		{
			Table:     "released_hooks",
			IndexName: "idx_tenantID_appID_releaseID_hookType",
			Fields:    []indexField{{"tenant_id", 0}, {"app_id", 0}, {"release_id", 0}, {"hook_type", 0}},
			Unique:    true,
		},
		{
			Table:     "released_kvs",
			IndexName: "tenantID_relID_key",
			Fields:    []indexField{{"tenant_id", 0}, {"key", 0}, {"release_id", 0}},
			Unique:    true,
		},
		{
			Table:     "released_kvs",
			IndexName: "idx_tenantID_bizID_appID_ID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}, {"release_id", 0}},
			Unique:    false,
		},
		{
			Table:     "releases",
			IndexName: "idx_tenantID_bizID_appID_name",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}, {"name", 0}},
			Unique:    true,
		},
		{
			Table:     "releases",
			IndexName: "idx_tenantID_bizID_appID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}},
			Unique:    false,
		},
		{
			Table:     "resource_locks",
			IndexName: "idx_tenantID_bizID_resType_resKey",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"res_type", 0}, {"res_key", 100}},
			Unique:    true,
		},
		{
			Table:     "strategies",
			IndexName: "idx_tenantID_bizID_appID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}},
			Unique:    false,
		},
		{
			Table:     "strategy_sets",
			IndexName: "idx_tenantID_appID_name",
			Fields:    []indexField{{"tenant_id", 0}, {"app_id", 0}, {"name", 0}},
			Unique:    true,
		},
		{
			Table:     "strategy_sets",
			IndexName: "idx_tenantID_bizID_id",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"id", 0}},
			Unique:    true,
		},
		{
			Table:     "strategy_sets",
			IndexName: "idx_tenantID_bizID_appID",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"app_id", 0}},
			Unique:    false,
		},
		{
			Table:     "template_revisions",
			IndexName: "idx_tenantID_bizID_tempID_revName",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"template_id", 0}, {"revision_name", 0}},
			Unique:    true,
		},
		{
			Table:     "template_sets",
			IndexName: "idx_tenantID_bizID_tempSpaID_name",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"template_space_id", 0}, {"name", 0}},
			Unique:    true,
		},
		{
			Table:     "template_spaces",
			IndexName: "idx_tenantID_bizID_name",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"name", 0}},
			Unique:    true,
		},
		{
			Table:     "template_variables",
			IndexName: "idx_tenantID_bizID_name",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"name", 0}},
			Unique:    true,
		},
		{
			Table:     "templates",
			IndexName: "idx_tenantID_bizID_tempSpaID_name_path",
			Fields:    []indexField{{"tenant_id", 0}, {"biz_id", 0}, {"template_space_id", 0}, {"name", 100}, {"path", 100}},
			Unique:    true,
		},
	}

	for _, idx := range indexes {
		// 索引存在跳过
		if tx.Migrator().HasIndex(idx.Table, idx.IndexName) {
			continue
		}
		if err := tx.Migrator().CreateIndex(idx.Table, idx.IndexName); err != nil {
			// 构建列字符串，带前缀
			columns := buildIndexColumns(idx.Fields)
			if err := tx.Exec(fmt.Sprintf("CREATE %s INDEX %s ON `%s` (%s)",
				func() string {
					if idx.Unique {
						return "UNIQUE "
					}
					return ""
				}(), idx.IndexName, idx.Table, columns)).Error; err != nil {
				return fmt.Errorf("failed to create index %s on table %s: %w", idx.IndexName, idx.Table, err)
			}
		}
	}
	return nil
}

func buildIndexColumns(fields []indexField) string {
	parts := make([]string, len(fields))
	for i, f := range fields {
		if f.Prefix > 0 {
			parts[i] = fmt.Sprintf("`%s`(%d)", f.Column, f.Prefix)
		} else {
			parts[i] = fmt.Sprintf("`%s`", f.Column)
		}
	}
	return strings.Join(parts, ", ")
}

// 删除新索引
func dropNewIndexes(tx *gorm.DB) error {
	indexesToDrop := map[string][]string{
		"app_template_bindings":  {"idx_tenantID_bizID_appID"},
		"app_template_variables": {"idx_tenantID_bizID_appID"},
		"applications":           {"idx_tenantID_bizID_name"},
		"archived_apps":          {"idx_tenantID_bizID_appID"},
		"audits":                 {"idx_tenantID_bizID_appID_createdAt"},
		"client_querys":          {"idx_tenantID_bizID_appID_creator"},
		"commits":                {"idx_tenantID_bizID_appID_cfgID"},
		"config_items":           {"idx_tenantID_bizID_appID_name"},
		"contents":               {"idx_tenantID_bizID_appID_cfgID"},
		"credentials":            {"idx_tenantID_bizID_name"},
		"current_published_strategies": {"idx_tenantID_strategyID", "idx_tenantID_bizID_appID",
			"idx_tenantID_bizID_releaseID"},
		"current_released_instances": {"idx_tenantID_appID_uid", "idx_tenantID_bizID_appID"},
		"events":                     {"idx_tenantID_bizID_appID"},
		"group_app_binds":            {"idx_tenantID_groupID_appID_bizID"},
		"groups":                     {"idx_tenantID_bizID_name", "idx_tenantID_bizID"},
		"hook_revisions":             {"idx_tenantID_bizID_revisionName"},
		"hooks":                      {"idx_tenantID_bizID_name"},
		"kvs":                        {"idx_tenantID_bizID_appID_key_kvState"},
		"published_strategy_histories": {"idx_tenantID_bizID_appID_setID_strategyID",
			"idx_tenantID_bizID_appID_setID_namespace"},
		"released_app_template_variables": {"idx_tenantID_releaseID", "idx_tenantID_bizID_appID"},
		"released_app_templates":          {"idx_tenantID_bizID_appID_relID"},
		"released_config_items":           {"idx_tenantID_bizID_appID_relID"},
		"released_groups":                 {"idx_tenantID_groupID_appID_bizID"},
		"released_hooks":                  {"idx_tenantID_appID_releaseID_hookType"},
		"released_kvs":                    {"tenantID_relID_key", "idx_tenantID_bizID_appID_ID"},
		"releases":                        {"idx_tenantID_bizID_appID_name", "idx_tenantID_bizID_appID"},
		"resource_locks":                  {"idx_tenantID_bizID_resType_resKey"},
		"strategies":                      {"idx_tenantID_bizID_appID"},
		"strategy_sets": {"idx_tenantID_appID_name", "idx_tenantID_bizID_id",
			"idx_tenantID_bizID_appID"},
		"template_revisions": {"idx_tenantID_bizID_tempID_revName"},
		"template_sets":      {"idx_tenantID_bizID_tempSpaID_name"},
		"template_spaces":    {"idx_tenantID_bizID_name"},
		"template_variables": {"idx_tenantID_bizID_name"},
		"templates":          {"idx_tenantID_bizID_tempSpaID_name_path"},
	}

	for tableName, indexes := range indexesToDrop {
		for _, index := range indexes {
			// 如果索引不存在，跳过；否则返回错误
			if !tx.Migrator().HasIndex(tableName, index) {
				continue
			}
			if err := tx.Migrator().DropIndex(tableName, index); err != nil {
				return fmt.Errorf("failed to drop index %s on table %s: %w", index, tableName, err)
			}
		}
	}

	return nil
}

// 新增 TenantID 字段
func addTenantIDColumn(tx *gorm.DB) error {
	for _, model := range allModels() {
		if !tx.Migrator().HasColumn(model, "tenant_id") {
			if err := tx.Migrator().AddColumn(model, "tenant_id"); err != nil {
				return err
			}
		}
	}

	return nil
}

// 删除 TenantID 字段
func dropTenantIDColumn(tx *gorm.DB) error {
	for _, model := range allModels() {
		if tx.Migrator().HasColumn(model, "tenant_id") {
			if err := tx.Migrator().DropColumn(model, "tenant_id"); err != nil {
				return err
			}
		}
	}

	return nil
}
