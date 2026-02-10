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

// Package config provides configuration management for the tenant migration tool
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the main configuration for tenant migration
type Config struct {
	Migration  MigrationConfig   `yaml:"migration"`
	Source     EnvironmentConfig `yaml:"source"`
	Target     EnvironmentConfig `yaml:"target"`
	Tables     []string          `yaml:"tables"`      // Tables to migrate (if empty, use default CoreTables)
	SkipTables []string          `yaml:"skip_tables"` // Tables to skip during migration
	Log        LogConfig         `yaml:"log"`
}

// MigrationConfig contains migration-specific settings
type MigrationConfig struct {
	// TargetTenantID is the tenant ID to set for all migrated records
	TargetTenantID string `yaml:"target_tenant_id"`
	// BizIDs is the list of business IDs to migrate
	// If empty, migrate all businesses
	BizIDs []uint32 `yaml:"biz_ids"`
	// BatchSize is the number of records to process in each batch
	BatchSize int `yaml:"batch_size"`
	// ContinueOnError if true, will continue migration even if some records fail
	ContinueOnError bool `yaml:"continue_on_error"`
}

// HasBizFilter returns true if business ID filter is configured
func (m *MigrationConfig) HasBizFilter() bool {
	return len(m.BizIDs) > 0
}

// ContainsBizID checks if a business ID is in the filter list
func (m *MigrationConfig) ContainsBizID(bizID uint32) bool {
	if !m.HasBizFilter() {
		return true // No filter, include all
	}
	for _, id := range m.BizIDs {
		if id == bizID {
			return true
		}
	}
	return false
}

// EnvironmentConfig contains source or target environment configuration
type EnvironmentConfig struct {
	MySQL MySQLConfig `yaml:"mysql"`
	Vault VaultConfig `yaml:"vault"`
}

// MySQLConfig contains MySQL connection configuration
type MySQLConfig struct {
	// Endpoints is a list of MySQL endpoints in host:port format
	Endpoints []string `yaml:"endpoints"`
	// Database is the database name
	Database string `yaml:"database"`
	// User is the database user
	User string `yaml:"user"`
	// Password is the database password
	Password string `yaml:"password"`
	// DialTimeoutSec is the connection timeout in seconds
	DialTimeoutSec uint `yaml:"dialTimeoutSec"`
	// ReadTimeoutSec is the read timeout in seconds
	ReadTimeoutSec uint `yaml:"readTimeoutSec"`
	// WriteTimeoutSec is the write timeout in seconds
	WriteTimeoutSec uint `yaml:"writeTimeoutSec"`
	// MaxOpenConn is the maximum number of open connections
	MaxOpenConn uint `yaml:"maxOpenConn"`
	// MaxIdleConn is the maximum number of idle connections
	MaxIdleConn uint `yaml:"maxIdleConn"`
}

// VaultConfig contains Vault connection configuration
type VaultConfig struct {
	// Address is the Vault server address
	Address string `yaml:"address"`
	// Token is the Vault access token
	Token string `yaml:"token"`
}

// LogConfig contains logging configuration
type LogConfig struct {
	// Level is the log level (debug, info, warn, error)
	Level string `yaml:"level"`
	// ToStdErr if true, logs to stderr
	ToStdErr bool `yaml:"toStdErr"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", filePath, err)
	}

	cfg.setDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// setDefaults sets default values for configuration
func (c *Config) setDefaults() {
	// Migration defaults
	if c.Migration.BatchSize <= 0 {
		c.Migration.BatchSize = 1000
	}

	// MySQL defaults
	setMySQLDefaults(&c.Source.MySQL)
	setMySQLDefaults(&c.Target.MySQL)

	// Log defaults
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}

	// Default skip tables
	if len(c.SkipTables) == 0 {
		c.SkipTables = DefaultSkipTables()
	}
}

func setMySQLDefaults(m *MySQLConfig) {
	if m.DialTimeoutSec == 0 {
		m.DialTimeoutSec = 15
	}
	if m.ReadTimeoutSec == 0 {
		m.ReadTimeoutSec = 30
	}
	if m.WriteTimeoutSec == 0 {
		m.WriteTimeoutSec = 30
	}
	if m.MaxOpenConn == 0 {
		m.MaxOpenConn = 50
	}
	if m.MaxIdleConn == 0 {
		m.MaxIdleConn = 10
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Migration.TargetTenantID == "" {
		return errors.New("migration.target_tenant_id is required")
	}

	if err := c.Source.MySQL.Validate("source.mysql"); err != nil {
		return err
	}

	if err := c.Target.MySQL.Validate("target.mysql"); err != nil {
		return err
	}

	// Vault is optional, only validate if address is provided
	if c.Source.Vault.Address != "" {
		if err := c.Source.Vault.Validate("source.vault"); err != nil {
			return err
		}
	}

	if c.Target.Vault.Address != "" {
		if err := c.Target.Vault.Validate("target.vault"); err != nil {
			return err
		}
	}

	return nil
}

// Validate validates MySQL configuration
func (m *MySQLConfig) Validate(prefix string) error {
	if len(m.Endpoints) == 0 {
		return fmt.Errorf("%s.endpoints is required", prefix)
	}
	if m.Database == "" {
		return fmt.Errorf("%s.database is required", prefix)
	}
	if m.User == "" {
		return fmt.Errorf("%s.user is required", prefix)
	}
	if m.Password == "" {
		return fmt.Errorf("%s.password is required", prefix)
	}
	return nil
}

// Validate validates Vault configuration
func (v *VaultConfig) Validate(prefix string) error {
	if v.Address == "" {
		return fmt.Errorf("%s.address is required", prefix)
	}
	if v.Token == "" {
		return fmt.Errorf("%s.token is required", prefix)
	}
	return nil
}

// DSN returns the MySQL DSN connection string
func (m *MySQLConfig) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?parseTime=true&loc=UTC&timeout=%ds&readTimeout=%ds&writeTimeout=%ds&charset=%s",
		m.User,
		m.Password,
		strings.Join(m.Endpoints, ","),
		m.Database,
		m.DialTimeoutSec,
		m.ReadTimeoutSec,
		m.WriteTimeoutSec,
		"utf8mb4",
	)
}

// DefaultSkipTables returns the default list of tables to skip during migration
func DefaultSkipTables() []string {
	return []string{
		"events",
		"current_released_instances",
		"resource_locks",
		"clients",
		"client_events",
		"client_querys",
		"audits",
		"published_strategy_histories",
		"archived_apps",
		"configs",
		"schema_migrations",
		"biz_hosts",
		"sharding_dbs",
	}
}

// CoreTables returns the list of core tables to migrate in dependency order
func CoreTables() []string {
	return []string{
		// First level - no foreign key dependencies
		"sharding_bizs",
		"applications",
		"template_spaces",
		"groups",
		"hooks",
		"credentials",

		// Second level - depends on first level
		"config_items",
		"releases",
		"strategy_sets",
		"template_sets",
		"templates",
		"template_variables",
		"hook_revisions",
		"credential_scopes",
		"group_app_binds",

		// Third level - depends on second level
		"commits",
		"contents",
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

// ShouldSkipTable checks if a table should be skipped during migration
func (c *Config) ShouldSkipTable(tableName string) bool {
	for _, skip := range c.SkipTables {
		if skip == tableName {
			return true
		}
	}
	return false
}

// GetTablesToMigrate returns the list of tables to migrate
// If Tables is configured, use it; otherwise use default CoreTables
func (c *Config) GetTablesToMigrate() []string {
	if len(c.Tables) > 0 {
		return c.Tables
	}
	return CoreTables()
}
