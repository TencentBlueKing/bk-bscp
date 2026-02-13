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

// Package cmd provides CLI commands for the GSEKit migration tool
package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TencentBlueKing/bk-bscp/cmd/gsekit-migration/config"
	"github.com/TencentBlueKing/bk-bscp/cmd/gsekit-migration/migrator"
)

var (
	cfgFile      string
	cfg          *config.Config
	bizIDs       string
	forceCleanup bool
	mockOutput   string
	maxProcesses int
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "gsekit-migration",
	Short: "GSEKit to BSCP Migration Tool",
	Long: `A tool for migrating data from GSEKit (process config management)
to BSCP (BlueKing Service Config Platform).

This tool handles:
- Process data migration (Process, ProcessInst)
- Config template migration with COS upload
- Config instance migration
- Data validation after migration
- Cleanup of migrated data`,
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

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path (required for migrate/validate/cleanup)")

	// Add subcommands
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(generateMockCmd)
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

// requireConfig is a PreRunE hook that ensures config is loaded
func requireConfig(cmd *cobra.Command, args []string) error {
	if cfgFile == "" {
		return fmt.Errorf("config file is required, use --config / -c flag")
	}
	if cfg == nil {
		return fmt.Errorf("configuration not loaded")
	}
	return nil
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
	Short: "Run GSEKit to BSCP data migration",
	Long: `Migrate data from GSEKit MySQL to BSCP MySQL.

This command migrates:
- Process and ProcessInstance data
- Config templates with COS upload
- Config instances`,
	PreRunE: requireConfig,
	Run: func(cmd *cobra.Command, args []string) {
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

		report, err := m.Run()
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
	Use:     "validate",
	Short:   "Validate migrated data",
	Long:    `Validate that data was migrated correctly by comparing source and target databases.`,
	PreRunE: requireConfig,
	Run: func(cmd *cobra.Command, args []string) {
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

		report, err := m.Validate()
		if err != nil {
			fmt.Printf("Error during validation: %v\n", err)
			os.Exit(1)
		}

		m.PrintValidationReport(report)

		if !report.Success {
			os.Exit(1)
		}
	},
}

// cleanupCmd represents the cleanup command
var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up migrated data from BSCP",
	Long: `Delete all migrated data from BSCP target database for specified biz_ids.
WARNING: This will delete data from the target database!`,
	PreRunE: requireConfig,
	Run: func(cmd *cobra.Command, args []string) {
		if err := applyBizIDsFlag(); err != nil {
			fmt.Printf("Error parsing biz-ids: %v\n", err)
			os.Exit(1)
		}

		if !forceCleanup {
			fmt.Printf("WARNING: This will delete migrated data for biz_ids %v from the BSCP target database!\n",
				cfg.Migration.BizIDs)
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

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("gsekit-migration v1.0.0")
		fmt.Println("GSEKit to BSCP Data Migration Tool")
	},
}

// generateMockCmd represents the generate-mock command
var generateMockCmd = &cobra.Command{
	Use:   "generate-mock",
	Short: "Generate mock-data.sql from real CMDB data",
	Long: `Query CMDB APIs for a given business (default biz=2) and generate a realistic
mock-data.sql file with real IPs, set/module IDs, and process details.`,
	PreRunE: requireConfig,
	Run: func(cmd *cobra.Command, args []string) {
		bizID := uint32(2)
		if len(cfg.Migration.BizIDs) > 0 {
			bizID = cfg.Migration.BizIDs[0]
		}

		gen := migrator.NewMockGenerator(&cfg.CMDB, bizID, maxProcesses)
		if err := gen.Generate(mockOutput); err != nil {
			fmt.Printf("Error generating mock data: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Mock data written to %s\n", mockOutput)
	},
}

func init() {
	migrateCmd.Flags().StringVar(&bizIDs, "biz-ids", "",
		"Comma-separated list of business IDs to migrate (overrides config)")

	validateCmd.Flags().StringVar(&bizIDs, "biz-ids", "",
		"Comma-separated list of business IDs to validate (overrides config)")

	cleanupCmd.Flags().BoolVarP(&forceCleanup, "force", "f", false, "Skip confirmation prompt")
	cleanupCmd.Flags().StringVar(&bizIDs, "biz-ids", "",
		"Comma-separated list of business IDs to cleanup (overrides config)")

	generateMockCmd.Flags().StringVarP(&mockOutput, "output", "o", "mock-data.sql",
		"Output path for generated SQL file")
	generateMockCmd.Flags().IntVar(&maxProcesses, "max-processes", 20,
		"Maximum number of processes to include in mock data")
}
