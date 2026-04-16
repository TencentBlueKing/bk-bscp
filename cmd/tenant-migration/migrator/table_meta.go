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

// TableMeta defines the migration metadata for a table
type TableMeta struct {
	Name         string            // Table name
	IDColumn     string            // ID column name (default "id")
	ForeignKeys  map[string]string // Foreign key column -> referenced table name
	JSONArrayFKs map[string]string // JSON array column -> referenced table name (e.g. template_ids -> templates)
	OptionalFKs  map[string]bool   // FK columns that allow soft failure: generate virtual ID when mapping not found
	HasBizID     bool              // Whether the table has biz_id column for filtering
}

// TableMetas defines metadata for all 29 core tables
var TableMetas = map[string]TableMeta{
	// ===== Level 1: Base tables (no foreign key dependencies) =====
	"sharding_bizs": {
		Name:     "sharding_bizs",
		IDColumn: "id",
		HasBizID: true,
	},
	"applications": {
		Name:     "applications",
		IDColumn: "id",
		HasBizID: true,
	},
	"template_spaces": {
		Name:     "template_spaces",
		IDColumn: "id",
		HasBizID: true,
	},
	"groups": {
		Name:     "groups",
		IDColumn: "id",
		HasBizID: true,
	},
	"hooks": {
		Name:     "hooks",
		IDColumn: "id",
		HasBizID: true,
	},
	"credentials": {
		Name:     "credentials",
		IDColumn: "id",
		HasBizID: true,
	},

	// ===== Level 2: Tables with Level 1 dependencies =====
	"config_items": {
		Name:     "config_items",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"app_id": "applications",
		},
	},
	"releases": {
		Name:     "releases",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"app_id": "applications",
		},
	},
	"strategy_sets": {
		Name:     "strategy_sets",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"app_id": "applications",
		},
	},
	"template_sets": {
		Name:     "template_sets",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"template_space_id": "template_spaces",
		},
		JSONArrayFKs: map[string]string{
			"template_ids": "templates",
			"bound_apps":   "applications",
		},
	},
	"templates": {
		Name:     "templates",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"template_space_id": "template_spaces",
		},
	},
	"template_variables": {
		Name:     "template_variables",
		IDColumn: "id",
		HasBizID: true,
	},
	"hook_revisions": {
		Name:     "hook_revisions",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"hook_id": "hooks",
		},
	},
	"credential_scopes": {
		Name:     "credential_scopes",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"credential_id": "credentials",
		},
	},
	"group_app_binds": {
		Name:     "group_app_binds",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"group_id": "groups",
			"app_id":   "applications",
		},
	},

	// ===== Level 3: Tables with Level 2 dependencies =====
	"commits": {
		Name:     "commits",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"app_id":         "applications",
			"config_item_id": "config_items",
			"content_id":     "contents",
		},
		OptionalFKs: map[string]bool{
			"config_item_id": true,
		},
	},
	"contents": {
		Name:     "contents",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"app_id":         "applications",
			"config_item_id": "config_items",
		},
		OptionalFKs: map[string]bool{
			"config_item_id": true,
		},
	},
	"strategies": {
		Name:     "strategies",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"app_id":          "applications",
			"release_id":      "releases",
			"strategy_set_id": "strategy_sets",
		},
	},
	"current_published_strategies": {
		Name:     "current_published_strategies",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"app_id":          "applications",
			"strategy_id":     "strategies",
			"release_id":      "releases",
			"strategy_set_id": "strategy_sets",
		},
	},
	"kvs": {
		Name:     "kvs",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"app_id": "applications",
		},
	},
	"released_config_items": {
		Name:     "released_config_items",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"app_id":         "applications",
			"release_id":     "releases",
			"commit_id":      "commits",
			"config_item_id": "config_items",
			"content_id":     "contents",
		},
		OptionalFKs: map[string]bool{
			"config_item_id": true,
		},
	},
	"released_groups": {
		Name:     "released_groups",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"app_id":      "applications",
			"group_id":    "groups",
			"release_id":  "releases",
			"strategy_id": "strategies",
		},
	},
	"released_hooks": {
		Name:     "released_hooks",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"app_id":           "applications",
			"release_id":       "releases",
			"hook_id":          "hooks",
			"hook_revision_id": "hook_revisions",
		},
	},
	"released_kvs": {
		Name:     "released_kvs",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"app_id":     "applications",
			"release_id": "releases",
		},
	},
	"template_revisions": {
		Name:     "template_revisions",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"template_space_id": "template_spaces",
			"template_id":       "templates",
		},
	},
	"app_template_bindings": {
		Name:     "app_template_bindings",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"app_id": "applications",
		},
		JSONArrayFKs: map[string]string{
			"template_space_ids":    "template_spaces",
			"template_set_ids":      "template_sets",
			"template_ids":          "templates",
			"template_revision_ids": "template_revisions",
			"latest_template_ids":   "templates",
		},
	},
	"app_template_variables": {
		Name:     "app_template_variables",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"app_id": "applications",
		},
	},
	"released_app_templates": {
		Name:     "released_app_templates",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"app_id":               "applications",
			"release_id":           "releases",
			"template_space_id":    "template_spaces",
			"template_set_id":      "template_sets",
			"template_id":          "templates",
			"template_revision_id": "template_revisions",
		},
	},
	"released_app_template_variables": {
		Name:     "released_app_template_variables",
		IDColumn: "id",
		HasBizID: true,
		ForeignKeys: map[string]string{
			"app_id":     "applications",
			"release_id": "releases",
		},
	},
}

// RuntimeCleanupTables returns tables that are skipped during migration but should
// still be cleaned during cleanup. These tables accumulate runtime data (audits,
// events, client records, etc.) in the target environment after migration.
func RuntimeCleanupTables() []string {
	return []string{
		"audits",
		"events",
		"published_strategy_histories",
		"current_released_instances",
		"clients",
		"client_events",
		"client_querys",
		"archived_apps",
		"biz_hosts",
	}
}

// TablesInCleanupOrder returns tables in reverse dependency order (for deletion)
// Delete dependent tables first, then base tables
func TablesInCleanupOrder() []string {
	return []string{
		// Level 3 (delete first - most dependent)
		"released_app_template_variables",
		"released_app_templates",
		"app_template_variables",
		"app_template_bindings",
		"template_revisions",
		"released_kvs",
		"released_hooks",
		"released_groups",
		"released_config_items",
		"kvs",
		"current_published_strategies",
		"strategies",
		"commits",
		"contents",

		// Level 2
		"group_app_binds",
		"credential_scopes",
		"hook_revisions",
		"templates",
		"template_sets",
		"strategy_sets",
		"releases",
		"config_items",

		// Level 1 (delete last - base tables)
		"template_variables",
		"credentials",
		"hooks",
		"groups",
		"template_spaces",
		"applications",
		"sharding_bizs",
	}
}

// TablesInInsertOrder returns tables in dependency order (for insertion)
// Insert base tables first, then dependent tables
func TablesInInsertOrder() []string {
	return []string{
		// Level 1 (insert first - base tables, no foreign keys)
		"sharding_bizs",
		"applications",
		"template_spaces",
		"groups",
		"hooks",
		"credentials",
		"template_variables",

		// Level 2 (depends on Level 1)
		"config_items",
		"releases",
		"strategy_sets",
		"templates",
		"template_sets",
		"hook_revisions",
		"credential_scopes",
		"group_app_binds",

		// Level 3 (depends on Level 1 and Level 2)
		"contents",
		"commits",
		"strategies",
		"current_published_strategies",
		"kvs",
		"released_config_items",
		"released_groups",
		"released_hooks",
		"released_kvs",
		"template_revisions",
		"app_template_bindings",
		"app_template_variables",
		"released_app_templates",
		"released_app_template_variables",
	}
}

// GetTableMeta returns the metadata for a specific table
func GetTableMeta(tableName string) (TableMeta, bool) {
	meta, ok := TableMetas[tableName]
	return meta, ok
}

// HasForeignKeys checks if a table has any foreign key relationships
func HasForeignKeys(tableName string) bool {
	meta, ok := TableMetas[tableName]
	if !ok {
		return false
	}
	return len(meta.ForeignKeys) > 0
}
