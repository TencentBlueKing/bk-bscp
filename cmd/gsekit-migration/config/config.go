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

// Package config provides configuration management for the GSEKit migration tool
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the main configuration for GSEKit to BSCP migration
type Config struct {
	Migration  MigrationConfig  `yaml:"migration"`
	Source     SourceConfig     `yaml:"source"`
	Target     TargetConfig     `yaml:"target"`
	Repository RepositoryConfig `yaml:"repository"`
	CMDB       CMDBConfig       `yaml:"cmdb"`
	Log        LogConfig        `yaml:"log"`
}

// MigrationConfig contains migration-specific settings
type MigrationConfig struct {
	// MultiTenant indicates whether to run in multi-tenant mode.
	// When false (single-tenant), TenantID is forced to "default".
	MultiTenant bool `yaml:"multi_tenant"`
	// TenantID is the target tenant ID for all migrated records
	TenantID string `yaml:"tenant_id"`
	// BizIDs is the list of business IDs to migrate
	BizIDs []uint32 `yaml:"biz_ids"`
	// BatchSize is the number of records to process in each batch
	BatchSize int `yaml:"batch_size"`
	// ContinueOnError if true, will continue migration even if some records fail
	ContinueOnError bool `yaml:"continue_on_error"`
	// SkipCMDB if true, skip CMDB API calls and use default values
	SkipCMDB bool `yaml:"skip_cmdb"`
}

// HasBizFilter returns true if business ID filter is configured
func (m *MigrationConfig) HasBizFilter() bool {
	return len(m.BizIDs) > 0
}

// SourceConfig contains source (GSEKit) environment configuration
type SourceConfig struct {
	MySQL MySQLConfig `yaml:"mysql"`
}

// TargetConfig contains target (BSCP) environment configuration
type TargetConfig struct {
	MySQL MySQLConfig `yaml:"mysql"`
}

// MySQLConfig contains MySQL connection configuration
type MySQLConfig struct {
	Endpoints       []string `yaml:"endpoints"`
	Database        string   `yaml:"database"`
	User            string   `yaml:"user"`
	Password        string   `yaml:"password"`
	DialTimeoutSec  uint     `yaml:"dialTimeoutSec"`
	ReadTimeoutSec  uint     `yaml:"readTimeoutSec"`
	WriteTimeoutSec uint     `yaml:"writeTimeoutSec"`
	MaxOpenConn     uint     `yaml:"maxOpenConn"`
	MaxIdleConn     uint     `yaml:"maxIdleConn"`
}

// RepositoryConfig contains content repository configuration
type RepositoryConfig struct {
	// StorageType is the storage backend type: "BKREPO" or "S3"
	StorageType string       `yaml:"storage_type"`
	BkRepo      BkRepoConfig `yaml:"bk_repo"`
	S3          S3Config     `yaml:"s3"`
}

// BkRepoConfig contains BK-Repo (BlueKing artifact repository) configuration
type BkRepoConfig struct {
	Endpoint string `yaml:"endpoint"`
	Project  string `yaml:"project"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// S3Config contains S3/COS configuration
type S3Config struct {
	Endpoint        string `yaml:"endpoint"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	BucketName      string `yaml:"bucket_name"`
	UseSSL          bool   `yaml:"use_ssl"`
}

// CMDBConfig contains CMDB API configuration
type CMDBConfig struct {
	Endpoint  string `yaml:"endpoint"`
	AppCode   string `yaml:"app_code"`
	AppSecret string `yaml:"app_secret"`
	Username  string `yaml:"username"`
}

// LogConfig contains logging configuration
type LogConfig struct {
	Level string `yaml:"level"`
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
	// In single-tenant mode, force TenantID to "default"
	if !c.Migration.MultiTenant {
		c.Migration.TenantID = "default"
	}

	if c.Migration.BatchSize <= 0 {
		c.Migration.BatchSize = 500
	}

	setMySQLDefaults(&c.Source.MySQL)
	setMySQLDefaults(&c.Target.MySQL)

	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
}

func setMySQLDefaults(m *MySQLConfig) {
	if m.DialTimeoutSec == 0 {
		m.DialTimeoutSec = 15
	}
	if m.ReadTimeoutSec == 0 {
		m.ReadTimeoutSec = 60
	}
	if m.WriteTimeoutSec == 0 {
		m.WriteTimeoutSec = 60
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
	if c.Migration.MultiTenant && c.Migration.TenantID == "" {
		return errors.New("migration.tenant_id is required when multi_tenant is true")
	}

	if !c.Migration.HasBizFilter() {
		return errors.New("migration.biz_ids is required (at least one business ID)")
	}

	if err := c.Source.MySQL.Validate("source.mysql"); err != nil {
		return err
	}

	if err := c.Target.MySQL.Validate("target.mysql"); err != nil {
		return err
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
