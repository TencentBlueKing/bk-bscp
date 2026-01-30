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

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/TencentBlueKing/bk-bscp/cmd/tenant-migration/migrator"
)

var testdataForceClean bool

// testdataCmd represents the testdata command
var testdataCmd = &cobra.Command{
	Use:   "testdata",
	Short: "Manage test data",
	Long: `Generate or clean test data for migration testing.

This command helps you create test data in the source database
for testing the migration process.`,
}

// testdataGenerateCmd generates test data
var testdataGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate test data in source database",
	Long: `Generate test data in the source database for migration testing.

This command creates records in all core tables and optionally in Vault.
Data generation uses hardcoded defaults:
  - BizIDs: [1001, 1002, 1003]
  - AppsPerBiz: 10
  - ConfigsPerApp: 20
  - ReleasesPerApp: 5
  - KvsPerApp: 10
  - GroupsPerBiz: 5`,
	Run: func(cmd *cobra.Command, args []string) {
		if cfg == nil {
			fmt.Println("Error: configuration not loaded")
			os.Exit(1)
		}

		g, err := migrator.NewTestDataGenerator(cfg)
		if err != nil {
			fmt.Printf("Error creating test data generator: %v\n", err)
			os.Exit(1)
		}
		defer g.Close()

		report, err := g.Generate()
		if err != nil {
			fmt.Printf("Error generating test data: %v\n", err)
		}

		g.PrintReport(report)

		if !report.Success {
			os.Exit(1)
		}
	},
}

// testdataCleanCmd cleans test data
var testdataCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean all data from source database tables",
	Long: `Clean all data from the source database.

This command TRUNCATES all core tables and cleans Vault data.

WARNING: This will delete ALL data from the source database!`,
	Run: func(cmd *cobra.Command, args []string) {
		if cfg == nil {
			fmt.Println("Error: configuration not loaded")
			os.Exit(1)
		}

		// Confirm before cleanup
		if !testdataForceClean {
			fmt.Println("WARNING: This will TRUNCATE all core tables in the source database!")
			fmt.Print("Are you sure you want to continue? [y/N]: ")
			var confirm string
			if _, err := fmt.Scanln(&confirm); err != nil || (confirm != "y" && confirm != "Y") {
				fmt.Println("Cleanup canceled.")
				return
			}
		}

		g, err := migrator.NewTestDataGenerator(cfg)
		if err != nil {
			fmt.Printf("Error creating test data generator: %v\n", err)
			os.Exit(1)
		}
		defer g.Close()

		report, err := g.Clean()
		if err != nil {
			fmt.Printf("Error cleaning test data: %v\n", err)
		}

		g.PrintReport(report)

		if !report.Success {
			os.Exit(1)
		}
	},
}

func init() {
	// Register testdata command to root
	rootCmd.AddCommand(testdataCmd)

	// Add testdata subcommands
	testdataCmd.AddCommand(testdataGenerateCmd)
	testdataCmd.AddCommand(testdataCleanCmd)

	// Add testdata clean flags
	testdataCleanCmd.Flags().BoolVarP(&testdataForceClean, "force", "f", false,
		"Skip confirmation prompt")
}
