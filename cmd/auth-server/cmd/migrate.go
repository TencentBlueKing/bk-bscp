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

// Package cmd provides operations for upgrading the permission model.
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TencentBlueKing/bk-bscp/internal/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/pkg/metrics"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
)

// iamTenantID init-iam 注册模型时携带的租户 ID，由 --tenant-id 指定，为空时按租户模式选择。
var iamTenantID string

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "auth-server migrations tool",
	Run: func(cmd *cobra.Command, args []string) {

	},
}

var migrateInitCmd = &cobra.Command{
	Use:   "init-iam",
	Short: "Initializes the authority center model",
	Run: func(cmd *cobra.Command, args []string) {

		if err := cc.LoadSettings(SysOpt.Sys); err != nil {
			fmt.Println("load settings from config files failed, err:", err)
			os.Exit(1)
		}

		logs.InitLogger(cc.AuthServer().Log.Logs())

		iamSys, err := NewIamSys()
		if err != nil {
			fmt.Printf("new iam sys failed, err: %v\n", err)
			os.Exit(1)
		}

		tenantID := iamTenantID
		if tenantID == "" {
			tenantID = modelTenantID()
		}

		kt := &kit.Kit{
			Ctx:      context.Background(),
			TenantID: tenantID,
		}

		if err := iamSys.Register(kt.InternalRpcCtx(), cc.AuthServer().Esb.BscpHost); err != nil {
			// 注册失败必须让命令以非零码退出：调用方（流水线/运维）依赖退出码判断模型是否真的生效。
			fmt.Printf("initialize service failed, err: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("initialize iam model succeed")
	},
}

// deleteIAMYes 跳过 delete-iam 的交互确认，供流水线与无人值守场景使用。
var deleteIAMYes bool

var migrateDeleteIAMCmd = &cobra.Command{
	Use:   "delete-iam",
	Short: "Delete all IAM V3 model elements of this system",
	Long: "清空权限中心 V3 中当前系统的权限模型，不做任何差异比较：\n" +
		"1. 删除所有操作的策略，再删除所有操作；\n" +
		"2. 删除所有实例视图；\n" +
		"3. 删除所有资源类型。\n" +
		"操作组、创建者动作与通用操作只有整组替换接口，删完操作后它们的引用会悬空，" +
		"由随后的 migrate init-iam 用本地定义整组覆盖。\n" +
		"该命令会清掉全部授权且不可恢复，清空后必须执行 migrate init-iam 重建模型。",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDeleteIAM(); err != nil {
			fmt.Println("清空 IAM 模型失败:", err)
			os.Exit(1)
		}
	},
}

// runDeleteIAM 清空线上模型，供 init-iam 全量重建。
func runDeleteIAM() error {
	if err := cc.LoadSettings(SysOpt.Sys); err != nil {
		return fmt.Errorf("load settings from config files failed: %w", err)
	}

	logs.InitLogger(cc.AuthServer().Log.Logs())

	iamSys, err := NewIamSys()
	if err != nil {
		return fmt.Errorf("new iam sys failed: %w", err)
	}

	tenantID := iamTenantID
	if tenantID == "" {
		tenantID = modelTenantID()
	}

	kt := &kit.Kit{
		Ctx:      context.Background(),
		TenantID: tenantID,
	}

	fmt.Println("即将删除权限中心 V3 模型的全部元素：所有操作（含策略）、实例视图、资源类型。")

	if !deleteIAMYes {
		if confirmErr := confirmPrompt("\n该操作不可恢复，清空后需执行 migrate init-iam 重建。输入 yes 继续："); confirmErr != nil {
			return confirmErr
		}
	}

	summary, err := iamSys.DeleteAll(kt.InternalRpcCtx())
	if err != nil {
		return err
	}

	fmt.Printf("删除完成：%s\n", summary)
	fmt.Println("请执行 migrate init-iam 按本地定义重建模型")

	return nil
}

// confirmPrompt 要求人工输入 yes 确认，非交互环境读不到输入时视为拒绝。
func confirmPrompt(prompt string) error {
	fmt.Print(prompt)

	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("读取确认输入失败，非交互环境请使用 --yes: %w", err)
	}

	if strings.TrimSpace(answer) != "yes" {
		return fmt.Errorf("已取消")
	}

	return nil
}

// modelTenantID 注册权限模型时携带的租户，调用前需已加载配置。
// 模型属于平台而非业务租户，多租户环境用运营租户 system，单租户环境用 default，与 IAM 版本无关。
func modelTenantID() string {
	return resolveModelTenantID(cc.AuthServer().FeatureFlags.EnableMultiTenantMode)
}

func resolveModelTenantID(multiTenant bool) string {
	if multiTenant {
		return constant.SystemTenantID
	}

	return constant.DefaultTenantID
}

// NewIamSys new a iamSystem
func NewIamSys() (*sys.Sys, error) {

	iamSettings := cc.AuthServer().IAM
	tlsConfig := new(tools.TLSConfig)
	if iamSettings.TLS.Enable() {
		tlsConfig = &tools.TLSConfig{
			InsecureSkipVerify: iamSettings.TLS.InsecureSkipVerify,
			CertFile:           iamSettings.TLS.CertFile,
			KeyFile:            iamSettings.TLS.KeyFile,
			CAFile:             iamSettings.TLS.CAFile,
			Password:           iamSettings.TLS.Password,
		}
	}
	cfg := &client.Config{
		Address:   []string{iamSettings.APIURL},
		AppCode:   iamSettings.AppCode,
		AppSecret: iamSettings.AppSecret,
		SystemID:  sys.SystemIDBSCP,
		TLS:       tlsConfig,
	}
	iamCli, err := client.NewClient(cfg, metrics.Register())
	if err != nil {
		return nil, err
	}

	iamSys, err := sys.NewSys(iamCli)
	if err != nil {
		return nil, fmt.Errorf("new iam sys failed, err: %v", err)
	}
	logs.Infof("initialize iam sys success.")

	return iamSys, nil

}

func init() {

	// Add "--debug" flag to all migrate sub commands
	migrateCmd.PersistentFlags().BoolP("debug", "d", false,
		"whether to debug output the execution process,, default is false")

	migrateInitCmd.Flags().StringVar(&iamTenantID, "tenant-id", "",
		"the tenant id used to register the model, defaults to system in multi-tenant mode, otherwise default")
	migrateCmd.AddCommand(migrateInitCmd)

	migrateDeleteIAMCmd.Flags().StringVar(&iamTenantID, "tenant-id", "",
		"the tenant id used to delete the model, defaults to system in multi-tenant mode, otherwise default")
	migrateDeleteIAMCmd.Flags().BoolVar(&deleteIAMYes, "yes", false,
		"skip the interactive confirmation, for non-interactive environments")
	migrateCmd.AddCommand(migrateDeleteIAMCmd)

	// Add "migrate" command to the root command
	rootCmd.AddCommand(migrateCmd)
}
