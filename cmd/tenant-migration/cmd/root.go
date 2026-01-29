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

// Package cmd provides CLI commands for the tenant migration tool
package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TencentBlueKing/bk-bscp/cmd/tenant-migration/config"
	"github.com/TencentBlueKing/bk-bscp/cmd/tenant-migration/migrator"
)

var (
	cfgFile string
	cfg     *config.Config
	bizIDs  string // Command line flag for biz IDs, comma-separated
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "bk-bscp-tenant-migration",
	Short: "BSCP Tenant Migration Tool",
	Long: `A tool for migrating BSCP data from single-tenant environment 
to multi-tenant environment.

This tool handles:
- MySQL data migration with tenant_id population
- Vault KV data migration via API
- Data validation after migration`,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path (required)")
	if err := rootCmd.MarkPersistentFlagRequired("config"); err != nil {
		panic(err)
	}

	// Add subcommands
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(versionCmd)
}

func initConfig() {
	if cfgFile == "" {
		return
	}

	var err error
	cfg, err = config.LoadConfig(cfgFile)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}
}

// parseBizIDs parses comma-separated biz IDs string to []uint32
func parseBizIDs(bizIDsStr string) ([]uint32, error) {
	if bizIDsStr == "" {
		return nil, nil
	}

	parts := strings.Split(bizIDsStr, ",")
	result := make([]uint32, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid biz ID '%s': %w", part, err)
		}
		result = append(result, uint32(id))
	}

	return result, nil
}

// applyBizIDsFlag applies command line biz-ids flag to config
func applyBizIDsFlag() error {
	if bizIDs == "" {
		return nil
	}

	ids, err := parseBizIDs(bizIDs)
	if err != nil {
		return err
	}

	if len(ids) > 0 {
		cfg.Migration.BizIDs = ids
		fmt.Printf("Using biz_ids from command line: %v\n", ids)
	}

	return nil
}

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run data migration",
	Long:  `Migrate data from source environment to target environment.`,
}

// migrateMySQLCmd migrates only MySQL data
var migrateMySQLCmd = &cobra.Command{
	Use:   "mysql",
	Short: "Migrate MySQL data only",
	Long:  `Migrate MySQL data from source to target database with tenant_id population.`,
	Run: func(cmd *cobra.Command, args []string) {
		if cfg == nil {
			fmt.Println("Error: configuration not loaded")
			os.Exit(1)
		}

		if err := applyBizIDsFlag(); err != nil {
			fmt.Printf("Error parsing biz-ids: %v\n", err)
			os.Exit(1)
		}

		m, err := migrator.NewMigrator(cfg)
		if err != nil {
			fmt.Printf("Error creating migrator: %v\n", err)
			os.Exit(1)
		}
		defer m.Close()

		report, err := m.RunMySQL()
		if err != nil {
			fmt.Printf("Error during migration: %v\n", err)
		}

		m.PrintReport(report)

		if !report.Success {
			os.Exit(1)
		}
	},
}

// migrateVaultCmd migrates only Vault data
var migrateVaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Migrate Vault KV data only",
	Long:  `Migrate Vault KV data from source to target Vault via API.`,
	Run: func(cmd *cobra.Command, args []string) {
		if cfg == nil {
			fmt.Println("Error: configuration not loaded")
			os.Exit(1)
		}

		if err := applyBizIDsFlag(); err != nil {
			fmt.Printf("Error parsing biz-ids: %v\n", err)
			os.Exit(1)
		}

		if cfg.Source.Vault.Address == "" || cfg.Target.Vault.Address == "" {
			fmt.Println("Error: Vault configuration is required for vault migration")
			os.Exit(1)
		}

		m, err := migrator.NewMigrator(cfg)
		if err != nil {
			fmt.Printf("Error creating migrator: %v\n", err)
			os.Exit(1)
		}
		defer m.Close()

		report, err := m.RunVault()
		if err != nil {
			fmt.Printf("Error during migration: %v\n", err)
		}

		m.PrintReport(report)

		if !report.Success {
			os.Exit(1)
		}
	},
}

// migrateAllCmd migrates both MySQL and Vault data
var migrateAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Migrate all data (MySQL + Vault)",
	Long:  `Migrate all data from source to target environment, including MySQL and Vault.`,
	Run: func(cmd *cobra.Command, args []string) {
		if cfg == nil {
			fmt.Println("Error: configuration not loaded")
			os.Exit(1)
		}

		if err := applyBizIDsFlag(); err != nil {
			fmt.Printf("Error parsing biz-ids: %v\n", err)
			os.Exit(1)
		}

		m, err := migrator.NewMigrator(cfg)
		if err != nil {
			fmt.Printf("Error creating migrator: %v\n", err)
			os.Exit(1)
		}
		defer m.Close()

		report, err := m.RunAll()
		if err != nil {
			fmt.Printf("Error during migration: %v\n", err)
		}

		m.PrintReport(report)

		if !report.Success {
			os.Exit(1)
		}
	},
}

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate migrated data",
	Long: `Validate that data was migrated correctly by comparing 
source and target databases.`,
	Run: func(cmd *cobra.Command, args []string) {
		if cfg == nil {
			fmt.Println("Error: configuration not loaded")
			os.Exit(1)
		}

		m, err := migrator.NewMigrator(cfg)
		if err != nil {
			fmt.Printf("Error creating migrator: %v\n", err)
			os.Exit(1)
		}
		defer m.Close()

		report, err := m.Validate()
		if err != nil {
			fmt.Printf("Error during validation: %v\n", err)
			os.Exit(1)
		}

		// Use validator's print report
		v := migrator.NewValidator(cfg, nil, nil)
		v.PrintReport(report)

		if !report.Success {
			os.Exit(1)
		}
	},
}

var scanAllTables bool

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan assets in source and target databases",
	Long: `Perform an asset scan to compare source and target databases.

This scans and reports:
- Number of tables in each database
- Record counts for each table
- Differences between source and target
- Vault KV data summary (if configured)

Use this to understand the current state before or after migration.

By default, only scans configured migration tables.
Use --all to scan all tables in the databases.`,
	Run: func(cmd *cobra.Command, args []string) {
		if cfg == nil {
			fmt.Println("Error: configuration not loaded")
			os.Exit(1)
		}

		m, err := migrator.NewMigrator(cfg)
		if err != nil {
			fmt.Printf("Error creating migrator: %v\n", err)
			os.Exit(1)
		}
		defer m.Close()

		var report *migrator.ScanReport
		if scanAllTables {
			report, err = m.ScanAll()
		} else {
			report, err = m.Scan()
		}
		if err != nil {
			fmt.Printf("Error during scan: %v\n", err)
			os.Exit(1)
		}

		m.PrintScanReport(report)
	},
}

// cleanupCmd represents the cleanup command
var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up target database",
	Long: `Delete all migrated data from target database.
This is useful when you need to re-run the migration after a failed attempt.

WARNING: This will delete all data from the core tables in the target database!`,
	Run: func(cmd *cobra.Command, args []string) {
		if cfg == nil {
			fmt.Println("Error: configuration not loaded")
			os.Exit(1)
		}

		// Confirm before cleanup
		if !forceCleanup {
			if len(cfg.Migration.BizIDs) > 0 {
				fmt.Printf("WARNING: This will delete data for biz_ids %v from the core tables in the target database!\n",
					cfg.Migration.BizIDs)
			} else {
				fmt.Println("WARNING: This will delete all data from the core tables in the target database!")
			}
			fmt.Print("Are you sure you want to continue? [y/N]: ")
			var confirm string
			if _, err := fmt.Scanln(&confirm); err != nil || (confirm != "y" && confirm != "Y") {
				fmt.Println("Cleanup canceled.")
				return
			}
		}

		m, err := migrator.NewMigrator(cfg)
		if err != nil {
			fmt.Printf("Error creating migrator: %v\n", err)
			os.Exit(1)
		}
		defer m.Close()

		result, err := m.Cleanup()
		if err != nil {
			fmt.Printf("Error during cleanup: %v\n", err)
			os.Exit(1)
		}

		m.PrintCleanupReport(result)

		if !result.Success {
			os.Exit(1)
		}
	},
}

var forceCleanup bool

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("bk-bscp-tenant-migration v1.0.0")
		fmt.Println("Single-tenant to Multi-tenant Data Migration Tool")
	},
}

func init() {
	// Add migrate subcommands
	migrateCmd.AddCommand(migrateMySQLCmd)
	migrateCmd.AddCommand(migrateVaultCmd)
	migrateCmd.AddCommand(migrateAllCmd)

	// Add migrate flags (persistent for all subcommands)
	migrateCmd.PersistentFlags().StringVar(&bizIDs, "biz-ids", "",
		"Comma-separated list of business IDs to migrate (e.g., '1001,1002,1003'). Overrides config file setting.")

	// Add cleanup flags
	cleanupCmd.Flags().BoolVarP(&forceCleanup, "force", "f", false, "Skip confirmation prompt")

	// Add scan flags
	scanCmd.Flags().BoolVarP(&scanAllTables, "all", "a", false, "Scan all tables in databases (not just configured tables)")
}
