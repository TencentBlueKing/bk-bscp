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
	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/cmd/data-service/db-migration/migrator"
)

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
// nolint:funlen
func mig20250427100034Up(tx *gorm.DB) error {

	// AppTemplateBindings mapped from table <app_template_bindings>
	type AppTemplateBindings struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// AppTemplateVariables mapped from table <app_template_variables>
	type AppTemplateVariables struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Applications mapped from table <applications>
	type Applications struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ArchivedApps mapped from table <archived_apps>
	type ArchivedApps struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Audits mapped from table <audits>
	type Audits struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ClientEvents mapped from table <client_events>
	type ClientEvents struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ClientQuerys mapped from table <client_querys>
	type ClientQuerys struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Clients mapped from table <clients>
	type Clients struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Commit mapped from table <commits>
	type Commit struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ConfigItems mapped from table <config_items>
	type ConfigItems struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Configs mapped from table <configs>
	type Configs struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Contents mapped from table <contents>
	type Contents struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// CredentialScopes mapped from table <credential_scopes>
	type CredentialScopes struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Credentials mapped from table <credentials>
	type Credentials struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// CurrentPublishedStrategies mapped from table <current_published_strategies>
	type CurrentPublishedStrategies struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// CurrentReleasedInstances mapped from table <current_released_instances>
	type CurrentReleasedInstances struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// DataSourceContents mapped from table <data_source_contents>
	type DataSourceContents struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// DataSourceInfos mapped from table <data_source_infos>
	type data_source_infos struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// DataSourceMappings mapped from table <data_source_mappings>
	type DataSourceMappings struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Events mapped from table <events>
	type Events struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// GroupAppBinds mapped from table <group_app_binds>
	type GroupAppBinds struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Groups mapped from table <groups>
	type Groups struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// HookRevisions mapped from table <hook_revisions>
	type HookRevisions struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Hooks mapped from table <hooks>
	type Hooks struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Kvs mapped from table <kvs>
	type Kvs struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// PublishedStrategyHistories mapped from table <published_strategy_histories>
	type PublishedStrategyHistories struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ReleasedAppTemplateVariables mapped from table <released_app_template_variables>
	type ReleasedAppTemplateVariables struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ReleasedAppTemplates mapped from table <released_app_templates>
	type ReleasedAppTemplates struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ReleasedConfigItems mapped from table <released_config_items>
	type ReleasedConfigItems struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ReleasedGroups mapped from table <released_groups>
	type ReleasedGroups struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ReleasedHooks mapped from table <released_hooks>
	type ReleasedHooks struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ReleasedKvs mapped from table <released_kvs>
	type ReleasedKvs struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ReleasedTableContents mapped from table <released_table_contents>
	type ReleasedTableContents struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Releases mapped from table <releases>
	type Releases struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ResourceLocks mapped from table <resource_locks>
	type ResourceLocks struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ShardingBizs mapped from table <sharding_bizs>
	type ShardingBizs struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Strategies mapped from table <strategies>
	type Strategies struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// StrategySets mapped from table <strategy_sets>
	type StrategySets struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// TemplateRevisions mapped from table <template_revisions>
	type TemplateRevisions struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// TemplateSets mapped from table <template_sets>
	type TemplateSets struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// TemplateSpaces mapped from table <template_spaces>
	type TemplateSpaces struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// TemplateVariables mapped from table <template_variables>
	type TemplateVariables struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Templates mapped from table <templates>
	type Templates struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// UserGroupPrivileges mapped from table <user_group_privileges>
	type UserGroupPrivileges struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// UserPrivileges mapped from table <user_privileges>
	type UserPrivileges struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	models := []interface{}{
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

	for _, model := range models {
		if !tx.Migrator().HasColumn(model, "tenant_id") {
			if err := tx.Migrator().AddColumn(model, "tenant_id"); err != nil {
				return err
			}
		}
	}

	return nil
}

// mig20250427100034Down for down migration
// nolint:funlen
func mig20250427100034Down(tx *gorm.DB) error {

	// AppTemplateBindings mapped from table <app_template_bindings>
	type AppTemplateBindings struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// AppTemplateVariables mapped from table <app_template_variables>
	type AppTemplateVariables struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Applications mapped from table <applications>
	type Applications struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ArchivedApps mapped from table <archived_apps>
	type ArchivedApps struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Audits mapped from table <audits>
	type Audits struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ClientEvents mapped from table <client_events>
	type ClientEvents struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ClientQuerys mapped from table <client_querys>
	type ClientQuerys struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Clients mapped from table <clients>
	type Clients struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Commit mapped from table <commits>
	type Commit struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ConfigItems mapped from table <config_items>
	type ConfigItems struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Configs mapped from table <configs>
	type Configs struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Contents mapped from table <contents>
	type Contents struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// CredentialScopes mapped from table <credential_scopes>
	type CredentialScopes struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Credentials mapped from table <credentials>
	type Credentials struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// CurrentPublishedStrategies mapped from table <current_published_strategies>
	type CurrentPublishedStrategies struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// CurrentReleasedInstances mapped from table <current_released_instances>
	type CurrentReleasedInstances struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// DataSourceContents mapped from table <data_source_contents>
	type DataSourceContents struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// DataSourceInfos mapped from table <data_source_infos>
	type data_source_infos struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// DataSourceMappings mapped from table <data_source_mappings>
	type DataSourceMappings struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Events mapped from table <events>
	type Events struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// GroupAppBinds mapped from table <group_app_binds>
	type GroupAppBinds struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Groups mapped from table <groups>
	type Groups struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// HookRevisions mapped from table <hook_revisions>
	type HookRevisions struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Hooks mapped from table <hooks>
	type Hooks struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Kvs mapped from table <kvs>
	type Kvs struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// PublishedStrategyHistories mapped from table <published_strategy_histories>
	type PublishedStrategyHistories struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ReleasedAppTemplateVariables mapped from table <released_app_template_variables>
	type ReleasedAppTemplateVariables struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ReleasedAppTemplates mapped from table <released_app_templates>
	type ReleasedAppTemplates struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ReleasedConfigItems mapped from table <released_config_items>
	type ReleasedConfigItems struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ReleasedGroups mapped from table <released_groups>
	type ReleasedGroups struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ReleasedHooks mapped from table <released_hooks>
	type ReleasedHooks struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ReleasedKvs mapped from table <released_kvs>
	type ReleasedKvs struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ReleasedTableContents mapped from table <released_table_contents>
	type ReleasedTableContents struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Releases mapped from table <releases>
	type Releases struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ResourceLocks mapped from table <resource_locks>
	type ResourceLocks struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// ShardingBizs mapped from table <sharding_bizs>
	type ShardingBizs struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Strategies mapped from table <strategies>
	type Strategies struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// StrategySets mapped from table <strategy_sets>
	type StrategySets struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// TemplateRevisions mapped from table <template_revisions>
	type TemplateRevisions struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// TemplateSets mapped from table <template_sets>
	type TemplateSets struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// TemplateSpaces mapped from table <template_spaces>
	type TemplateSpaces struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// TemplateVariables mapped from table <template_variables>
	type TemplateVariables struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// Templates mapped from table <templates>
	type Templates struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// UserGroupPrivileges mapped from table <user_group_privileges>
	type UserGroupPrivileges struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	// UserPrivileges mapped from table <user_privileges>
	type UserPrivileges struct {
		TenantID string `gorm:"column:tenant_id;type:varchar(256);not null;default:default" json:"tenant_id"`
	}

	models := []interface{}{
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

	for _, model := range models {
		if tx.Migrator().HasColumn(model, "tenant_id") {
			if err := tx.Migrator().DropColumn(model, "tenant_id"); err != nil {
				return err
			}
		}
	}

	return nil
}
