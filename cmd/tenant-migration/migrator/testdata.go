//go:build testdata

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
	"strings"
	"sync/atomic"
	"time"

	vault "github.com/openbao/openbao/api/v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/TencentBlueKing/bk-bscp/cmd/tenant-migration/config"
)

// Hardcoded test data generation constants
const (
	// Number of applications per business
	appsPerBiz = 10
	// Number of config items per application
	configsPerApp = 20
	// Number of releases per application
	releasesPerApp = 5
	// Number of KV configs per application
	kvsPerApp = 10
	// Number of groups per business
	groupsPerBiz = 5
	// Number of hooks per business
	hooksPerBiz = 3
	// Number of credentials per business
	credentialsPerBiz = 2
	// Number of template spaces per business
	templateSpacesPerBiz = 2
	// Number of templates per template space
	templatesPerSpace = 5
)

// testDataBizIDs is the list of business IDs to generate data for
var testDataBizIDs = []uint32{1001, 1002, 1003}

// TestDataGenerator handles test data generation
type TestDataGenerator struct {
	cfg         *config.Config
	db          *gorm.DB
	vaultClient *vault.Client

	// ID counters for each table
	idCounters map[string]*uint32

	// Generated data tracking for relationships
	apps           []generatedApp
	groups         []generatedGroup
	hooks          []generatedHook
	hookRevisions  []generatedHookRevision
	credentials    []generatedCredential
	templateSpaces []generatedTemplateSpace
	templates      []generatedTemplate
	templateSets   []generatedTemplateSet
	configItems    []generatedConfigItem
	releases       []generatedRelease
	strategySets   []generatedStrategySet
	strategies     []generatedStrategy
	contents       []generatedContent
	commits        []generatedCommit
	kvs            []generatedKv
	templateRevs   []generatedTemplateRevision
	templateVars   []generatedTemplateVariable
	appTplBindings []generatedAppTemplateBinding
	appTplVars     []generatedAppTemplateVariable
}

// Generated data types for tracking relationships
type generatedApp struct {
	ID    uint32
	BizID uint32
	Name  string
}

type generatedGroup struct {
	ID    uint32
	BizID uint32
	UID   string
}

type generatedHook struct {
	ID    uint32
	BizID uint32
}

type generatedHookRevision struct {
	ID     uint32
	BizID  uint32
	HookID uint32
}

type generatedCredential struct {
	ID    uint32
	BizID uint32
}

type generatedTemplateSpace struct {
	ID    uint32
	BizID uint32
}

type generatedTemplate struct {
	ID              uint32
	BizID           uint32
	TemplateSpaceID uint32
}

type generatedTemplateSet struct {
	ID              uint32
	BizID           uint32
	TemplateSpaceID uint32
}

type generatedConfigItem struct {
	ID    uint32
	BizID uint32
	AppID uint32
}

type generatedRelease struct {
	ID    uint32
	BizID uint32
	AppID uint32
}

type generatedStrategySet struct {
	ID    uint32
	BizID uint32
	AppID uint32
}

type generatedStrategy struct {
	ID            uint32
	BizID         uint32
	AppID         uint32
	ReleaseID     uint32
	StrategySetID uint32
}

type generatedContent struct {
	ID           uint32
	BizID        uint32
	AppID        uint32
	ConfigItemID uint32
}

type generatedCommit struct {
	ID           uint32
	BizID        uint32
	AppID        uint32
	ConfigItemID uint32
	ContentID    uint32
}

type generatedKv struct {
	ID    uint32
	BizID uint32
	AppID uint32
	Key   string
}

type generatedTemplateRevision struct {
	ID              uint32
	BizID           uint32
	TemplateSpaceID uint32
	TemplateID      uint32
}

type generatedTemplateVariable struct {
	ID    uint32
	BizID uint32
}

type generatedAppTemplateBinding struct {
	ID    uint32
	BizID uint32
	AppID uint32
}

type generatedAppTemplateVariable struct {
	ID    uint32
	BizID uint32
	AppID uint32
}

// TestDataReport contains the result of test data generation
type TestDataReport struct {
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
	TableResults map[string]int64
	VaultKvs     int64
	VaultRKvs    int64
	Errors       []string
	Success      bool
}

// NewTestDataGenerator creates a new TestDataGenerator instance
func NewTestDataGenerator(cfg *config.Config) (*TestDataGenerator, error) {
	// Connect to source database
	db, err := gorm.Open(mysql.Open(cfg.Source.MySQL.DSN()),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to source database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database handle: %w", err)
	}
	sqlDB.SetMaxOpenConns(int(cfg.Source.MySQL.MaxOpenConn))
	sqlDB.SetMaxIdleConns(int(cfg.Source.MySQL.MaxIdleConn))

	// Create Vault client if configured
	var vaultClient *vault.Client
	if cfg.Source.Vault.Address != "" {
		vaultConfig := vault.DefaultConfig()
		vaultConfig.Address = cfg.Source.Vault.Address
		vaultClient, err = vault.NewClient(vaultConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Vault client: %w", err)
		}
		vaultClient.SetToken(cfg.Source.Vault.Token)
	}

	return &TestDataGenerator{
		cfg:         cfg,
		db:          db,
		vaultClient: vaultClient,
		idCounters:  make(map[string]*uint32),
	}, nil
}

// Close closes database connections
func (g *TestDataGenerator) Close() error {
	if g.db != nil {
		sqlDB, err := g.db.DB()
		if err == nil {
			return sqlDB.Close()
		}
	}
	return nil
}

// Generate generates test data
func (g *TestDataGenerator) Generate() (*TestDataReport, error) {
	report := &TestDataReport{
		StartTime:    time.Now(),
		Success:      true,
		TableResults: make(map[string]int64),
	}

	log.Println("Starting test data generation...")
	log.Printf("Configuration: %d businesses, %d apps/biz, %d configs/app, %d releases/app, %d kvs/app",
		len(testDataBizIDs), appsPerBiz, configsPerApp, releasesPerApp, kvsPerApp)

	// Initialize ID counters
	g.initIDCounters()

	// Disable foreign key checks
	if err := g.db.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
		return nil, fmt.Errorf("failed to disable foreign key checks: %w", err)
	}
	defer func() {
		if err := g.db.Exec("SET FOREIGN_KEY_CHECKS = 1").Error; err != nil {
			log.Printf("Warning: failed to re-enable foreign key checks: %v", err)
		}
	}()

	// Generate data in dependency order
	generators := []struct {
		name string
		fn   func() (int64, error)
	}{
		{"sharding_bizs", g.generateShardingBizs},
		{"applications", g.generateApplications},
		{"template_spaces", g.generateTemplateSpaces},
		{"groups", g.generateGroups},
		{"hooks", g.generateHooks},
		{"credentials", g.generateCredentials},
		{"config_items", g.generateConfigItems},
		{"releases", g.generateReleases},
		{"strategy_sets", g.generateStrategySets},
		{"template_sets", g.generateTemplateSets},
		{"templates", g.generateTemplates},
		{"template_variables", g.generateTemplateVariables},
		{"hook_revisions", g.generateHookRevisions},
		{"credential_scopes", g.generateCredentialScopes},
		{"group_app_binds", g.generateGroupAppBinds},
		{"contents", g.generateContents},
		{"commits", g.generateCommits},
		{"strategies", g.generateStrategies},
		{"current_published_strategies", g.generateCurrentPublishedStrategies},
		{"kvs", g.generateKvs},
		{"released_config_items", g.generateReleasedConfigItems},
		{"released_groups", g.generateReleasedGroups},
		{"released_hooks", g.generateReleasedHooks},
		{"released_kvs", g.generateReleasedKvs},
		{"template_revisions", g.generateTemplateRevisions},
		{"app_template_bindings", g.generateAppTemplateBindings},
		{"app_template_variables", g.generateAppTemplateVariables},
		{"released_app_templates", g.generateReleasedAppTemplates},
		{"released_app_template_variables", g.generateReleasedAppTemplateVariables},
	}

	for _, gen := range generators {
		log.Printf("Generating %s...", gen.name)
		count, err := gen.fn()
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", gen.name, err))
			report.Success = false
			log.Printf("  Error: %v", err)
		} else {
			report.TableResults[gen.name] = count
			log.Printf("  Generated %d records", count)
		}
	}

	// Generate Vault data if configured
	if g.vaultClient != nil {
		log.Println("Generating Vault KV data...")
		kvCount, rkvCount, err := g.generateVaultData()
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("vault: %v", err))
			report.Success = false
		} else {
			report.VaultKvs = kvCount
			report.VaultRKvs = rkvCount
			log.Printf("  Generated %d KVs, %d released KVs", kvCount, rkvCount)
		}
	}

	// Update id_generators table
	if err := g.updateIDGenerators(); err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("id_generators: %v", err))
		log.Printf("Warning: failed to update id_generators: %v", err)
	}

	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(report.StartTime)

	log.Printf("Test data generation completed in %v", report.Duration)
	return report, nil
}

// initIDCounters initializes ID counters starting from 0
// Assumes the database is empty
func (g *TestDataGenerator) initIDCounters() {
	resources := []string{
		"applications", "config_items", "commits", "contents", "releases",
		"released_config_items", "strategies", "strategy_sets", "current_published_strategies",
		"groups", "group_app_binds", "released_groups", "hooks", "hook_revisions",
		"released_hooks", "credentials", "credential_scopes", "template_spaces",
		"templates", "template_revisions", "template_sets", "template_variables",
		"app_template_bindings", "app_template_variables", "released_app_templates",
		"released_app_template_variables", "kvs", "released_kvs", "sharding_bizs",
	}

	for _, resource := range resources {
		var initialID uint32 = 0
		g.idCounters[resource] = &initialID
	}
}

// nextID returns the next ID for a resource
func (g *TestDataGenerator) nextID(resource string) uint32 {
	counter, ok := g.idCounters[resource]
	if !ok {
		var initialValue uint32 = 100000
		g.idCounters[resource] = &initialValue
		counter = &initialValue
	}
	return atomic.AddUint32(counter, 1)
}

// currentMaxID returns the current max ID for a resource
func (g *TestDataGenerator) currentMaxID(resource string) uint32 {
	counter, ok := g.idCounters[resource]
	if !ok {
		return 0
	}
	return atomic.LoadUint32(counter)
}

// generateShardingBizs generates sharding_bizs records
func (g *TestDataGenerator) generateShardingBizs() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, bizID := range testDataBizIDs {
		// Check if already exists
		var existing int64
		if err := g.db.Table("sharding_bizs").Where("biz_id = ?", bizID).Count(&existing).Error; err != nil {
			return count, err
		}
		if existing > 0 {
			continue
		}

		id := g.nextID("sharding_bizs")
		if err := g.db.Exec(`INSERT INTO sharding_bizs (id, memo, biz_id, sharding_db_id, creator, reviser, created_at, updated_at) 
			VALUES (?, ?, ?, 0, ?, ?, ?, ?)`,
			id, fmt.Sprintf("testdata biz %d", bizID), bizID, creator, creator, now, now).Error; err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

// generateApplications generates applications records
func (g *TestDataGenerator) generateApplications() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, bizID := range testDataBizIDs {
		for i := 0; i < appsPerBiz; i++ {
			id := g.nextID("applications")
			name := fmt.Sprintf("test-app-%d-%d", bizID, i+1)

			if err := g.db.Exec(`INSERT INTO applications (id, name, config_type, memo, alias, data_type, biz_id, creator, reviser, created_at, updated_at) 
				VALUES (?, ?, 'file', ?, ?, '', ?, ?, ?, ?, ?)`,
				id, name, fmt.Sprintf("test app %d for biz %d", i+1, bizID), name, bizID, creator, creator, now, now).Error; err != nil {
				return count, err
			}

			g.apps = append(g.apps, generatedApp{ID: id, BizID: bizID, Name: name})
			count++
		}
	}

	return count, nil
}

// generateTemplateSpaces generates template_spaces records
func (g *TestDataGenerator) generateTemplateSpaces() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, bizID := range testDataBizIDs {
		for i := 0; i < templateSpacesPerBiz; i++ {
			id := g.nextID("template_spaces")

			if err := g.db.Exec(`INSERT INTO template_spaces (id, name, memo, biz_id, creator, reviser, created_at, updated_at) 
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				id, fmt.Sprintf("test-tpl-space-%d-%d", bizID, i+1),
				fmt.Sprintf("test template space %d", i+1), bizID, creator, creator, now, now).Error; err != nil {
				return count, err
			}

			g.templateSpaces = append(g.templateSpaces, generatedTemplateSpace{ID: id, BizID: bizID})
			count++
		}
	}

	return count, nil
}

// generateGroups generates groups records
func (g *TestDataGenerator) generateGroups() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, bizID := range testDataBizIDs {
		for i := 0; i < groupsPerBiz; i++ {
			id := g.nextID("groups")
			uid := fmt.Sprintf("group-uid-%d-%d", bizID, i+1)

			if err := g.db.Exec(`INSERT INTO `+"`groups`"+` (id, name, mode, public, selector, uid, biz_id, creator, reviser, created_at, updated_at) 
				VALUES (?, ?, 'custom', true, '{"labels_and":[{"key":"env","op":"eq","value":"test"}]}', ?, ?, ?, ?, ?, ?)`,
				id, fmt.Sprintf("test-group-%d-%d", bizID, i+1), uid, bizID, creator, creator, now, now).Error; err != nil {
				return count, err
			}

			g.groups = append(g.groups, generatedGroup{ID: id, BizID: bizID, UID: uid})
			count++
		}
	}

	return count, nil
}

// generateHooks generates hooks records
func (g *TestDataGenerator) generateHooks() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, bizID := range testDataBizIDs {
		for i := 0; i < hooksPerBiz; i++ {
			id := g.nextID("hooks")

			if err := g.db.Exec(`INSERT INTO hooks (id, name, memo, type, tags, biz_id, creator, reviser, created_at, updated_at) 
				VALUES (?, ?, ?, 'shell', '["pre"]', ?, ?, ?, ?, ?)`,
				id, fmt.Sprintf("test-hook-%d-%d", bizID, i+1),
				fmt.Sprintf("test hook %d", i+1), bizID, creator, creator, now, now).Error; err != nil {
				return count, err
			}

			g.hooks = append(g.hooks, generatedHook{ID: id, BizID: bizID})
			count++
		}
	}

	return count, nil
}

// generateCredentials generates credentials records
func (g *TestDataGenerator) generateCredentials() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, bizID := range testDataBizIDs {
		for i := 0; i < credentialsPerBiz; i++ {
			id := g.nextID("credentials")

			if err := g.db.Exec(`INSERT INTO credentials (id, name, biz_id, credential_type, enc_credential, enc_algorithm, memo, enable, creator, reviser, created_at, updated_at, expired_at) 
				VALUES (?, ?, ?, 'bearer', ?, 'aes', ?, 1, ?, ?, ?, ?, ?)`,
				id, fmt.Sprintf("test-cred-%d-%d", bizID, i+1), bizID,
				fmt.Sprintf("encrypted_token_%d_%d", bizID, i+1),
				fmt.Sprintf("test credential %d", i+1),
				creator, creator, now, now, now.Add(365*24*time.Hour)).Error; err != nil {
				return count, err
			}

			g.credentials = append(g.credentials, generatedCredential{ID: id, BizID: bizID})
			count++
		}
	}

	return count, nil
}

// generateConfigItems generates config_items records
func (g *TestDataGenerator) generateConfigItems() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, app := range g.apps {
		for i := 0; i < configsPerApp; i++ {
			id := g.nextID("config_items")

			if err := g.db.Exec(`INSERT INTO config_items (id, name, path, file_type, file_mode, memo, user, user_group, privilege, charset, biz_id, app_id, creator, reviser, created_at, updated_at) 
				VALUES (?, ?, ?, 'yaml', 'unix', ?, 'root', 'root', '644', '', ?, ?, ?, ?, ?, ?)`,
				id, fmt.Sprintf("config-%d.yaml", i+1), fmt.Sprintf("/etc/app/%d", i+1),
				fmt.Sprintf("config item %d", i+1), app.BizID, app.ID, creator, creator, now, now).Error; err != nil {
				return count, err
			}

			g.configItems = append(g.configItems, generatedConfigItem{ID: id, BizID: app.BizID, AppID: app.ID})
			count++
		}
	}

	return count, nil
}

// generateReleases generates releases records
func (g *TestDataGenerator) generateReleases() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, app := range g.apps {
		for i := 0; i < releasesPerApp; i++ {
			id := g.nextID("releases")

			if err := g.db.Exec(`INSERT INTO releases (id, name, memo, deprecated, publish_num, fully_released, biz_id, app_id, creator, created_at) 
				VALUES (?, ?, ?, false, 1, true, ?, ?, ?, ?)`,
				id, fmt.Sprintf("v%d.0.0", i+1), fmt.Sprintf("release %d", i+1),
				app.BizID, app.ID, creator, now).Error; err != nil {
				return count, err
			}

			g.releases = append(g.releases, generatedRelease{ID: id, BizID: app.BizID, AppID: app.ID})
			count++
		}
	}

	return count, nil
}

// generateStrategySets generates strategy_sets records
func (g *TestDataGenerator) generateStrategySets() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, app := range g.apps {
		id := g.nextID("strategy_sets")

		if err := g.db.Exec(`INSERT INTO strategy_sets (id, name, mode, status, memo, biz_id, app_id, creator, reviser, created_at, updated_at) 
			VALUES (?, ?, 'normal', 'enabled', ?, ?, ?, ?, ?, ?, ?)`,
			id, fmt.Sprintf("strategy-set-%d", app.ID), fmt.Sprintf("strategy set for app %d", app.ID),
			app.BizID, app.ID, creator, creator, now, now).Error; err != nil {
			return count, err
		}

		g.strategySets = append(g.strategySets, generatedStrategySet{ID: id, BizID: app.BizID, AppID: app.ID})
		count++
	}

	return count, nil
}

// generateTemplateSets generates template_sets records
func (g *TestDataGenerator) generateTemplateSets() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, ts := range g.templateSpaces {
		id := g.nextID("template_sets")

		if err := g.db.Exec(`INSERT INTO template_sets (id, name, memo, template_ids, public, bound_apps, biz_id, template_space_id, creator, reviser, created_at, updated_at) 
			VALUES (?, ?, ?, '[]', true, '[]', ?, ?, ?, ?, ?, ?)`,
			id, fmt.Sprintf("test-tpl-set-%d", ts.ID), fmt.Sprintf("template set for space %d", ts.ID),
			ts.BizID, ts.ID, creator, creator, now, now).Error; err != nil {
			return count, err
		}

		g.templateSets = append(g.templateSets, generatedTemplateSet{ID: id, BizID: ts.BizID, TemplateSpaceID: ts.ID})
		count++
	}

	return count, nil
}

// generateTemplates generates templates records
func (g *TestDataGenerator) generateTemplates() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, ts := range g.templateSpaces {
		for i := 0; i < templatesPerSpace; i++ {
			id := g.nextID("templates")

			if err := g.db.Exec(`INSERT INTO templates (id, name, path, memo, biz_id, template_space_id, creator, reviser, created_at, updated_at) 
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				id, fmt.Sprintf("template-%d.conf", i+1), fmt.Sprintf("/etc/templates/%d", i+1),
				fmt.Sprintf("template %d", i+1), ts.BizID, ts.ID, creator, creator, now, now).Error; err != nil {
				return count, err
			}

			g.templates = append(g.templates, generatedTemplate{ID: id, BizID: ts.BizID, TemplateSpaceID: ts.ID})
			count++
		}
	}

	return count, nil
}

// generateTemplateVariables generates template_variables records
func (g *TestDataGenerator) generateTemplateVariables() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, bizID := range testDataBizIDs {
		for i := 0; i < 5; i++ { // 5 variables per biz
			id := g.nextID("template_variables")

			if err := g.db.Exec(`INSERT INTO template_variables (id, name, type, default_val, memo, biz_id, creator, reviser, created_at, updated_at) 
				VALUES (?, ?, 'string', ?, ?, ?, ?, ?, ?, ?)`,
				id, fmt.Sprintf("VAR_%d_%d", bizID, i+1), fmt.Sprintf("default_%d", i+1),
				fmt.Sprintf("variable %d", i+1), bizID, creator, creator, now, now).Error; err != nil {
				return count, err
			}

			g.templateVars = append(g.templateVars, generatedTemplateVariable{ID: id, BizID: bizID})
			count++
		}
	}

	return count, nil
}

// generateHookRevisions generates hook_revisions records
func (g *TestDataGenerator) generateHookRevisions() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, hook := range g.hooks {
		id := g.nextID("hook_revisions")

		if err := g.db.Exec(`INSERT INTO hook_revisions (id, name, memo, state, content, biz_id, hook_id, creator, reviser, created_at, updated_at) 
			VALUES (?, ?, ?, 'not_deployed', '#!/bin/bash\necho "hook script"', ?, ?, ?, ?, ?, ?)`,
			id, "v1.0.0", "hook revision", hook.BizID, hook.ID, creator, creator, now, now).Error; err != nil {
			return count, err
		}

		g.hookRevisions = append(g.hookRevisions, generatedHookRevision{ID: id, BizID: hook.BizID, HookID: hook.ID})
		count++
	}

	return count, nil
}

// generateCredentialScopes generates credential_scopes records
func (g *TestDataGenerator) generateCredentialScopes() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, cred := range g.credentials {
		id := g.nextID("credential_scopes")

		if err := g.db.Exec(`INSERT INTO credential_scopes (id, biz_id, credential_id, credential_scope, creator, reviser, updated_at, created_at, expired_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, cred.BizID, cred.ID, fmt.Sprintf("app:test-app-%d-*", cred.BizID),
			creator, creator, now, now, now.Add(365*24*time.Hour)).Error; err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

// generateGroupAppBinds generates group_app_binds records
func (g *TestDataGenerator) generateGroupAppBinds() (int64, error) {
	var count int64

	// Bind first group of each biz to first app of that biz
	bizGroupMap := make(map[uint32]uint32)
	for _, grp := range g.groups {
		if _, exists := bizGroupMap[grp.BizID]; !exists {
			bizGroupMap[grp.BizID] = grp.ID
		}
	}

	for _, app := range g.apps {
		groupID, exists := bizGroupMap[app.BizID]
		if !exists {
			continue
		}

		id := g.nextID("group_app_binds")
		if err := g.db.Exec(`INSERT INTO group_app_binds (id, group_id, app_id, biz_id) 
			VALUES (?, ?, ?, ?)`, id, groupID, app.ID, app.BizID).Error; err != nil {
			return count, err
		}
		count++
		// Only bind one per biz
		delete(bizGroupMap, app.BizID)
	}

	return count, nil
}

// generateContents generates contents records
func (g *TestDataGenerator) generateContents() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, ci := range g.configItems {
		id := g.nextID("contents")

		if err := g.db.Exec(`INSERT INTO contents (id, signature, byte_size, md5, biz_id, app_id, config_item_id, creator, created_at) 
			VALUES (?, ?, 1024, ?, ?, ?, ?, ?, ?)`,
			id, fmt.Sprintf("sha256_sig_%d", id), fmt.Sprintf("md5_%d", id),
			ci.BizID, ci.AppID, ci.ID, creator, now).Error; err != nil {
			return count, err
		}

		g.contents = append(g.contents, generatedContent{ID: id, BizID: ci.BizID, AppID: ci.AppID, ConfigItemID: ci.ID})
		count++
	}

	return count, nil
}

// generateCommits generates commits records
func (g *TestDataGenerator) generateCommits() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, content := range g.contents {
		id := g.nextID("commits")

		if err := g.db.Exec(`INSERT INTO commits (id, content_id, signature, byte_size, md5, memo, biz_id, app_id, config_item_id, creator, created_at) 
			VALUES (?, ?, ?, 1024, ?, 'commit', ?, ?, ?, ?, ?)`,
			id, content.ID, fmt.Sprintf("sha256_sig_%d", content.ID), fmt.Sprintf("md5_%d", content.ID),
			content.BizID, content.AppID, content.ConfigItemID, creator, now).Error; err != nil {
			return count, err
		}

		g.commits = append(g.commits, generatedCommit{
			ID: id, BizID: content.BizID, AppID: content.AppID,
			ConfigItemID: content.ConfigItemID, ContentID: content.ID,
		})
		count++
	}

	return count, nil
}

// generateStrategies generates strategies records
func (g *TestDataGenerator) generateStrategies() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	// Create strategy for each release
	releaseIdx := 0
	for _, ss := range g.strategySets {
		// Find releases for this app
		for ; releaseIdx < len(g.releases) && g.releases[releaseIdx].AppID == ss.AppID; releaseIdx++ {
			release := g.releases[releaseIdx]
			id := g.nextID("strategies")

			if err := g.db.Exec(`INSERT INTO strategies (id, name, release_id, as_default, scope, namespace, memo, pub_state, biz_id, app_id, strategy_set_id, itsm_ticket_state_id, creator, reviser, created_at, updated_at) 
				VALUES (?, ?, ?, true, null, '', ?, 'published', ?, ?, ?, '1', ?, ?, ?, ?)`,
				id, fmt.Sprintf("strategy-%d", id), release.ID,
				fmt.Sprintf("strategy for release %d", release.ID),
				ss.BizID, ss.AppID, ss.ID, creator, creator, now, now).Error; err != nil {
				return count, err
			}

			g.strategies = append(g.strategies, generatedStrategy{
				ID: id, BizID: ss.BizID, AppID: ss.AppID,
				ReleaseID: release.ID, StrategySetID: ss.ID,
			})
			count++
		}
	}

	return count, nil
}

// generateCurrentPublishedStrategies generates current_published_strategies records
func (g *TestDataGenerator) generateCurrentPublishedStrategies() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, strategy := range g.strategies {
		id := g.nextID("current_published_strategies")

		if err := g.db.Exec(`INSERT INTO current_published_strategies (id, name, release_id, as_default, scope, mode, namespace, memo, pub_state, biz_id, app_id, strategy_set_id, strategy_id, creator, created_at) 
			VALUES (?, ?, ?, true, null, 'normal', '', ?, 'published', ?, ?, ?, ?, ?, ?)`,
			id, fmt.Sprintf("cps-%d", id), strategy.ReleaseID,
			fmt.Sprintf("current published strategy %d", id),
			strategy.BizID, strategy.AppID, strategy.StrategySetID, strategy.ID, creator, now).Error; err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

// generateKvs generates kvs records
func (g *TestDataGenerator) generateKvs() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, app := range g.apps {
		for i := 0; i < kvsPerApp; i++ {
			id := g.nextID("kvs")
			key := fmt.Sprintf("kv_key_%d_%d", app.ID, i+1)

			// Test scenario: version = 5 to simulate real production data with multiple edits
			// Vault will also have 5 versions written to match this
			if err := g.db.Exec(`INSERT INTO kvs (id, `+"`key`"+`, version, kv_type, kv_state, signature, md5, byte_size, memo, biz_id, app_id, creator, reviser, created_at, updated_at) 
				VALUES (?, ?, 5, 'string', 'add', ?, ?, 64, ?, ?, ?, ?, ?, ?, ?)`,
				id, key, fmt.Sprintf("kv_sha256_%d", id), fmt.Sprintf("kv_md5_%d", id),
				fmt.Sprintf("kv %d", i+1), app.BizID, app.ID, creator, creator, now, now).Error; err != nil {
				return count, err
			}

			g.kvs = append(g.kvs, generatedKv{ID: id, BizID: app.BizID, AppID: app.ID, Key: key})
			count++
		}
	}

	return count, nil
}

// generateReleasedConfigItems generates released_config_items records
func (g *TestDataGenerator) generateReleasedConfigItems() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	// Map commits by config item ID
	commitMap := make(map[uint32]generatedCommit)
	for _, c := range g.commits {
		commitMap[c.ConfigItemID] = c
	}

	// Map config items by app ID
	ciByApp := make(map[uint32][]generatedConfigItem)
	for _, ci := range g.configItems {
		ciByApp[ci.AppID] = append(ciByApp[ci.AppID], ci)
	}

	for _, release := range g.releases {
		configItems := ciByApp[release.AppID]
		for _, ci := range configItems {
			commit, exists := commitMap[ci.ID]
			if !exists {
				continue
			}

			id := g.nextID("released_config_items")
			if err := g.db.Exec(`INSERT INTO released_config_items (id, commit_id, release_id, biz_id, app_id, config_item_id, content_id, signature, byte_size, md5, origin_signature, origin_byte_size, name, path, file_type, file_mode, charset, memo, user, user_group, privilege, creator, reviser, created_at, updated_at) 
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1024, ?, ?, 1024, ?, ?, 'yaml', 'unix', '', 'released config', 'root', 'root', '644', ?, ?, ?, ?)`,
				id, commit.ID, release.ID, ci.BizID, ci.AppID, ci.ID, commit.ContentID,
				fmt.Sprintf("sha256_sig_%d", commit.ContentID), fmt.Sprintf("md5_%d", commit.ContentID),
				fmt.Sprintf("sha256_origin_%d", commit.ContentID),
				fmt.Sprintf("config-%d.yaml", ci.ID%100+1), fmt.Sprintf("/etc/app/%d", ci.ID%100+1),
				creator, creator, now, now).Error; err != nil {
				return count, err
			}
			count++
		}
	}

	return count, nil
}

// generateReleasedGroups generates released_groups records
func (g *TestDataGenerator) generateReleasedGroups() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	// Map groups by biz ID
	groupByBiz := make(map[uint32]generatedGroup)
	for _, grp := range g.groups {
		if _, exists := groupByBiz[grp.BizID]; !exists {
			groupByBiz[grp.BizID] = grp
		}
	}

	for _, strategy := range g.strategies {
		group, exists := groupByBiz[strategy.BizID]
		if !exists {
			continue
		}

		id := g.nextID("released_groups")
		if err := g.db.Exec(`INSERT INTO released_groups (id, group_id, app_id, release_id, strategy_id, mode, selector, uid, edited, biz_id, reviser, updated_at) 
			VALUES (?, ?, ?, ?, ?, 'custom', '{"labels_and":[{"key":"env","op":"eq","value":"test"}]}', ?, false, ?, ?, ?)`,
			id, group.ID, strategy.AppID, strategy.ReleaseID, strategy.ID, group.UID, strategy.BizID, creator, now).Error; err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

// generateReleasedHooks generates released_hooks records
func (g *TestDataGenerator) generateReleasedHooks() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	// Map hook revisions by biz ID
	hookRevByBiz := make(map[uint32]generatedHookRevision)
	for _, hr := range g.hookRevisions {
		if _, exists := hookRevByBiz[hr.BizID]; !exists {
			hookRevByBiz[hr.BizID] = hr
		}
	}

	for _, release := range g.releases {
		hr, exists := hookRevByBiz[release.BizID]
		if !exists {
			continue
		}

		id := g.nextID("released_hooks")
		if err := g.db.Exec(`INSERT INTO released_hooks (id, app_id, release_id, hook_type, hook_id, hook_revision_id, hook_name, hook_revision_name, content, script_type, biz_id, reviser, updated_at) 
			VALUES (?, ?, ?, 'pre', ?, ?, 'test-hook', 'v1.0.0', '#!/bin/bash\necho "hook"', 'shell', ?, ?, ?)`,
			id, release.AppID, release.ID, hr.HookID, hr.ID, release.BizID, creator, now).Error; err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

// generateReleasedKvs generates released_kvs records
func (g *TestDataGenerator) generateReleasedKvs() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	// Map kvs by app ID
	kvsByApp := make(map[uint32][]generatedKv)
	for _, kv := range g.kvs {
		kvsByApp[kv.AppID] = append(kvsByApp[kv.AppID], kv)
	}

	for _, release := range g.releases {
		kvs := kvsByApp[release.AppID]
		for _, kv := range kvs {
			id := g.nextID("released_kvs")
			if err := g.db.Exec(`INSERT INTO released_kvs (id, `+"`key`"+`, version, release_id, kv_type, signature, md5, byte_size, memo, biz_id, app_id, creator, reviser, created_at, updated_at) 
				VALUES (?, ?, 1, ?, 'string', ?, ?, 64, 'released kv', ?, ?, ?, ?, ?, ?)`,
				id, kv.Key, release.ID, fmt.Sprintf("rkv_sha256_%d", id), fmt.Sprintf("rkv_md5_%d", id),
				kv.BizID, kv.AppID, creator, creator, now, now).Error; err != nil {
				return count, err
			}
			count++
		}
	}

	return count, nil
}

// generateTemplateRevisions generates template_revisions records
func (g *TestDataGenerator) generateTemplateRevisions() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, tpl := range g.templates {
		id := g.nextID("template_revisions")
		if err := g.db.Exec(`INSERT INTO template_revisions (id, revision_name, revision_memo, name, path, file_type, file_mode, charset, user, user_group, privilege, signature, byte_size, md5, biz_id, template_space_id, template_id, creator, created_at) 
			VALUES (?, 'v1.0.0', 'revision', ?, ?, 'text', 'unix', '', 'root', 'root', '644', ?, 2048, ?, ?, ?, ?, ?, ?)`,
			id, fmt.Sprintf("template-%d.conf", tpl.ID%100+1), fmt.Sprintf("/etc/templates/%d", tpl.ID%100+1),
			fmt.Sprintf("sha256_tpl_%d", id), fmt.Sprintf("tpl_md5_%d", id),
			tpl.BizID, tpl.TemplateSpaceID, tpl.ID, creator, now).Error; err != nil {
			return count, err
		}
		g.templateRevs = append(g.templateRevs, generatedTemplateRevision{
			ID: id, BizID: tpl.BizID, TemplateSpaceID: tpl.TemplateSpaceID, TemplateID: tpl.ID,
		})
		count++
	}

	return count, nil
}

// generateAppTemplateBindings generates app_template_bindings records
func (g *TestDataGenerator) generateAppTemplateBindings() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, app := range g.apps {
		id := g.nextID("app_template_bindings")
		if err := g.db.Exec(`INSERT INTO app_template_bindings (id, template_space_ids, template_set_ids, template_ids, template_revision_ids, latest_template_ids, bindings, biz_id, app_id, creator, reviser, created_at, updated_at) 
			VALUES (?, '[]', '[]', '[]', '[]', '[]', '[]', ?, ?, ?, ?, ?, ?)`,
			id, app.BizID, app.ID, creator, creator, now, now).Error; err != nil {
			return count, err
		}
		g.appTplBindings = append(g.appTplBindings, generatedAppTemplateBinding{ID: id, BizID: app.BizID, AppID: app.ID})
		count++
	}

	return count, nil
}

// generateAppTemplateVariables generates app_template_variables records
func (g *TestDataGenerator) generateAppTemplateVariables() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, app := range g.apps {
		id := g.nextID("app_template_variables")
		if err := g.db.Exec(`INSERT INTO app_template_variables (id, variables, biz_id, app_id, creator, reviser, created_at, updated_at) 
			VALUES (?, '[]', ?, ?, ?, ?, ?, ?)`,
			id, app.BizID, app.ID, creator, creator, now, now).Error; err != nil {
			return count, err
		}
		g.appTplVars = append(g.appTplVars, generatedAppTemplateVariable{ID: id, BizID: app.BizID, AppID: app.ID})
		count++
	}

	return count, nil
}

// generateReleasedAppTemplates generates released_app_templates records
func (g *TestDataGenerator) generateReleasedAppTemplates() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	// Map template revisions by template space ID
	tplRevBySpace := make(map[uint32]generatedTemplateRevision)
	for _, tr := range g.templateRevs {
		if _, exists := tplRevBySpace[tr.TemplateSpaceID]; !exists {
			tplRevBySpace[tr.TemplateSpaceID] = tr
		}
	}

	// Map template sets by biz ID
	tplSetByBiz := make(map[uint32]generatedTemplateSet)
	for _, ts := range g.templateSets {
		if _, exists := tplSetByBiz[ts.BizID]; !exists {
			tplSetByBiz[ts.BizID] = ts
		}
	}

	for _, release := range g.releases {
		tplSet, exists := tplSetByBiz[release.BizID]
		if !exists {
			continue
		}
		tplRev, exists := tplRevBySpace[tplSet.TemplateSpaceID]
		if !exists {
			continue
		}

		id := g.nextID("released_app_templates")
		if err := g.db.Exec(`INSERT INTO released_app_templates (id, release_id, template_space_id, template_space_name, template_set_id, template_set_name, template_id, name, path, template_revision_id, is_latest, template_revision_name, template_revision_memo, file_type, file_mode, charset, user, user_group, privilege, signature, byte_size, md5, origin_signature, origin_byte_size, biz_id, app_id, creator, reviser, created_at, updated_at) 
			VALUES (?, ?, ?, 'test-template-space', ?, 'test-template-set', ?, 'template.conf', '/etc/templates', ?, true, 'v1.0.0', 'released', 'text', 'unix', '', 'root', 'root', '644', ?, 2048, ?, ?, 2048, ?, ?, ?, ?, ?, ?)`,
			id, release.ID, tplRev.TemplateSpaceID, tplSet.ID, tplRev.TemplateID, tplRev.ID,
			fmt.Sprintf("sha256_rat_%d", id), fmt.Sprintf("rat_md5_%d", id), fmt.Sprintf("sha256_origin_%d", id),
			release.BizID, release.AppID, creator, creator, now, now).Error; err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

// generateReleasedAppTemplateVariables generates released_app_template_variables records
func (g *TestDataGenerator) generateReleasedAppTemplateVariables() (int64, error) {
	now := time.Now()
	creator := "testdata_gen"
	var count int64

	for _, release := range g.releases {
		id := g.nextID("released_app_template_variables")
		if err := g.db.Exec(`INSERT INTO released_app_template_variables (id, release_id, variables, biz_id, app_id, creator, created_at) 
			VALUES (?, ?, '[]', ?, ?, ?, ?)`,
			id, release.ID, release.BizID, release.AppID, creator, now).Error; err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

// vaultKvVersionCount is the number of versions to create for each KV in Vault
// This should match the version value in generateKvs (currently 5)
const vaultKvVersionCount = 5

// generateVaultData generates Vault KV data
// For unreleased KVs: writes 5 times to create version 5 (matching DB version field)
// For released KVs: writes 1 time (version 1) as released KVs are immutable snapshots
func (g *TestDataGenerator) generateVaultData() (int64, int64, error) {
	if g.vaultClient == nil {
		return 0, 0, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	var kvCount, rkvCount int64

	// Generate unreleased KV data with multiple versions
	// Write 5 times to simulate edits and create version 5
	log.Printf("  Writing %d versions for each KV to simulate edits...", vaultKvVersionCount)
	for _, kv := range g.kvs {
		path := fmt.Sprintf("biz/%d/apps/%d/kvs/%s", kv.BizID, kv.AppID, kv.Key)

		// Write multiple times to create version history
		for v := 1; v <= vaultKvVersionCount; v++ {
			data := map[string]interface{}{
				"value": fmt.Sprintf("test_value_%d_v%d", kv.ID, v),
			}

			_, err := g.vaultClient.KVv2(MountPath).Put(ctx, path, data)
			if err != nil {
				return kvCount, rkvCount, fmt.Errorf("failed to write KV %s version %d: %w", path, v, err)
			}
		}
		kvCount++
	}

	// Generate released KV data (single version - released KVs are immutable)
	kvsByApp := make(map[uint32][]generatedKv)
	for _, kv := range g.kvs {
		kvsByApp[kv.AppID] = append(kvsByApp[kv.AppID], kv)
	}

	for _, release := range g.releases {
		kvs := kvsByApp[release.AppID]
		for _, kv := range kvs {
			path := fmt.Sprintf("biz/%d/apps/%d/releases/%d/kvs/%s", kv.BizID, kv.AppID, release.ID, kv.Key)
			data := map[string]interface{}{
				"value": fmt.Sprintf("released_value_%d_%d", kv.ID, release.ID),
			}

			_, err := g.vaultClient.KVv2(MountPath).Put(ctx, path, data)
			if err != nil {
				return kvCount, rkvCount, fmt.Errorf("failed to write released KV %s: %w", path, err)
			}
			rkvCount++
		}
	}

	return kvCount, rkvCount, nil
}

// updateIDGenerators updates the id_generators table
func (g *TestDataGenerator) updateIDGenerators() error {
	for resource, counter := range g.idCounters {
		maxID := atomic.LoadUint32(counter)
		if maxID == 0 {
			continue
		}

		// Check if record exists
		var count int64
		if err := g.db.Table("id_generators").Where("resource = ?", resource).Count(&count).Error; err != nil {
			return err
		}

		if count > 0 {
			if err := g.db.Exec(`UPDATE id_generators SET max_id = GREATEST(max_id, ?) WHERE resource = ?`,
				maxID, resource).Error; err != nil {
				return err
			}
		} else {
			if err := g.db.Exec(`INSERT INTO id_generators (resource, max_id, updated_at) VALUES (?, ?, ?)`,
				resource, maxID, time.Now()).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

// Clean removes all data from test tables using TRUNCATE
// Note: Vault data must be cleaned BEFORE MySQL data because cleanVaultData()
// reads from MySQL kvs/released_kvs tables to construct Vault paths
func (g *TestDataGenerator) Clean() (*TestDataReport, error) {
	report := &TestDataReport{
		StartTime:    time.Now(),
		Success:      true,
		TableResults: make(map[string]int64),
	}

	log.Println("Cleaning all data from test tables...")

	// Clean Vault data FIRST (before MySQL) because cleanVaultData() needs to read
	// from MySQL kvs/released_kvs tables to get the paths for deletion
	if g.vaultClient != nil {
		log.Println("Cleaning Vault data (must be done before MySQL cleanup)...")
		kvCount, rkvCount, err := g.cleanVaultData()
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("vault: %v", err))
			report.Success = false
		} else {
			report.VaultKvs = kvCount
			report.VaultRKvs = rkvCount
			log.Printf("  Cleaned %d KVs, %d released KVs from Vault", kvCount, rkvCount)
		}
	}

	// Disable foreign key checks
	if err := g.db.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
		return nil, fmt.Errorf("failed to disable foreign key checks: %w", err)
	}
	defer func() {
		if err := g.db.Exec("SET FOREIGN_KEY_CHECKS = 1").Error; err != nil {
			log.Printf("Warning: failed to re-enable foreign key checks: %v", err)
		}
	}()

	// Tables to clean (order doesn't matter with TRUNCATE and FK disabled)
	tables := []string{
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
		"group_app_binds",
		"credential_scopes",
		"hook_revisions",
		"template_variables",
		"templates",
		"template_sets",
		"strategy_sets",
		"releases",
		"config_items",
		"credentials",
		"hooks",
		"groups",
		"template_spaces",
		"applications",
		"sharding_bizs",
	}

	log.Println("Cleaning MySQL tables...")
	for _, table := range tables {
		// Count before truncate
		var count int64
		if err := g.db.Table(table).Count(&count).Error; err != nil {
			log.Printf("Warning: failed to count %s: %v", table, err)
			continue
		}

		// TRUNCATE table
		if err := g.db.Exec(fmt.Sprintf("TRUNCATE TABLE `%s`", table)).Error; err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", table, err))
			report.Success = false
			log.Printf("  Error truncating %s: %v", table, err)
		} else {
			report.TableResults[table] = count
			log.Printf("  Truncated %s (%d records)", table, count)
		}
	}

	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(report.StartTime)

	return report, nil
}

// cleanVaultData removes all Vault KV data
func (g *TestDataGenerator) cleanVaultData() (int64, int64, error) {
	if g.vaultClient == nil {
		return 0, 0, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	var kvCount, rkvCount int64

	// Get all KV records from database
	var kvRecords []struct {
		BizID uint32 `gorm:"column:biz_id"`
		AppID uint32 `gorm:"column:app_id"`
		Key   string `gorm:"column:key"`
	}
	if err := g.db.Table("kvs").Find(&kvRecords).Error; err != nil {
		return 0, 0, err
	}

	for _, kv := range kvRecords {
		path := fmt.Sprintf("biz/%d/apps/%d/kvs/%s", kv.BizID, kv.AppID, kv.Key)
		if err := g.vaultClient.KVv2(MountPath).DeleteMetadata(ctx, path); err != nil {
			log.Printf("Warning: failed to delete KV %s: %v", path, err)
		} else {
			kvCount++
		}
	}

	// Get all released KV records from database
	var rkvRecords []struct {
		BizID     uint32 `gorm:"column:biz_id"`
		AppID     uint32 `gorm:"column:app_id"`
		ReleaseID uint32 `gorm:"column:release_id"`
		Key       string `gorm:"column:key"`
	}
	if err := g.db.Table("released_kvs").Find(&rkvRecords).Error; err != nil {
		return kvCount, 0, err
	}

	for _, rkv := range rkvRecords {
		path := fmt.Sprintf("biz/%d/apps/%d/releases/%d/kvs/%s", rkv.BizID, rkv.AppID, rkv.ReleaseID, rkv.Key)
		if err := g.vaultClient.KVv2(MountPath).DeleteMetadata(ctx, path); err != nil {
			log.Printf("Warning: failed to delete released KV %s: %v", path, err)
		} else {
			rkvCount++
		}
	}

	return kvCount, rkvCount, nil
}

// PrintReport prints the test data generation report
func (g *TestDataGenerator) PrintReport(report *TestDataReport) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("TEST DATA GENERATION REPORT")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Start Time:  %s\n", report.StartTime.Format(time.RFC3339))
	fmt.Printf("End Time:    %s\n", report.EndTime.Format(time.RFC3339))
	fmt.Printf("Duration:    %v\n", report.Duration)
	fmt.Printf("Status:      %s\n", boolToStatus(report.Success))
	fmt.Println()

	if len(report.TableResults) > 0 {
		fmt.Println("MySQL Records Generated:")
		fmt.Println(strings.Repeat("-", 60))
		fmt.Printf("%-40s %12s\n", "Table", "Count")
		fmt.Println(strings.Repeat("-", 60))

		var total int64
		for table, count := range report.TableResults {
			fmt.Printf("%-40s %12d\n", table, count)
			total += count
		}
		fmt.Println(strings.Repeat("-", 60))
		fmt.Printf("%-40s %12d\n", "Total", total)
		fmt.Println()
	}

	if report.VaultKvs > 0 || report.VaultRKvs > 0 {
		fmt.Println("Vault Records Generated:")
		fmt.Println(strings.Repeat("-", 60))
		fmt.Printf("KV Records:          %d\n", report.VaultKvs)
		fmt.Printf("Released KV Records: %d\n", report.VaultRKvs)
		fmt.Println()
	}

	if len(report.Errors) > 0 {
		fmt.Println("Errors:")
		fmt.Println(strings.Repeat("-", 60))
		for _, err := range report.Errors {
			fmt.Printf("  - %s\n", err)
		}
		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 60))
}
