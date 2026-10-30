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
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	clientv4 "github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/model"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/modelsync"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// iamV4TenantID 调用权限中心时携带的租户 ID，由 --tenant-id 指定，为空时按租户模式选择。
//
// 多租户环境的网关要求 X-Bk-Tenant-Id 非空，缺失时直接返回跨租户禁止而非继续调用；
// 多租户的 IAM 不认 default 租户，因此多租户环境必须用 system。
var iamV4TenantID string

var migrateInitIAMV4Cmd = &cobra.Command{
	Use:   "init-iam-v4",
	Short: "Sync the IAM V4 permission model",
	Long: "把本地定义的 V4 权限模型（资源类型、操作、角色、角色展示层级）同步到权限中心。\n" +
		"先与线上状态求差异再施加变更，可重复执行：模型未变时不会产生任何写请求。\n" +
		"只做新增与更新；线上多余的元素仅报告，删除请用 prune-iam-v4。",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runIAMV4Sync(false); err != nil {
			fmt.Println("同步 IAM V4 模型失败:", err)
			os.Exit(1)
		}
	},
}

var migrateDiffIAMV4Cmd = &cobra.Command{
	Use:   "diff-iam-v4",
	Short: "Show the diff between local and online IAM V4 model",
	Long:  "只读地对比本地模型定义与权限中心线上状态，输出将要发生的变更，不做任何修改。",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runIAMV4Diff(); err != nil {
			fmt.Println("对比 IAM V4 模型失败:", err)
			os.Exit(1)
		}
	},
}

var migratePruneIAMV4Cmd = &cobra.Command{
	Use:   "prune-iam-v4",
	Short: "Delete online IAM V4 model elements absent from local definition",
	Long: "删除权限中心里存在、但本地定义中已移除的资源类型、操作与角色。\n" +
		"删除角色会失效该角色下的全部授权，删除操作会让引用它的角色失去该权限，\n" +
		"因此执行前需要交互确认。",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runIAMV4Sync(true); err != nil {
			fmt.Println("清理 IAM V4 模型失败:", err)
			os.Exit(1)
		}
	},
}

// deleteIAMV4Yes 跳过 delete-iam-v4 的交互确认，供流水线与无人值守场景使用。
var deleteIAMV4Yes bool

var migrateDeleteIAMV4Cmd = &cobra.Command{
	Use:   "delete-iam-v4",
	Short: "Delete all IAM V4 model elements of this system",
	Long: "清空权限中心 V4 中当前系统的权限模型，不做任何差异比较：\n" +
		"1. 删除全部角色；\n" +
		"2. 删除全部操作；\n" +
		"3. 删除全部资源类型。\n" +
		"删除顺序遵循依赖关系，角色下的全部授权会一并失效且无法恢复。\n" +
		"系统本身保留，回调地址与 clients 白名单不受影响。\n" +
		"资源类型的祖先链、操作关联的资源类型不支持更新，需要变更时用该命令清空，" +
		"随后必须执行 migrate init-iam-v4 重建模型。",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runIAMV4DeleteAll(); err != nil {
			fmt.Println("清空 IAM V4 模型失败:", err)
			os.Exit(1)
		}
	},
}

// runIAMV4DeleteAll 清空线上 V4 模型，供 init-iam-v4 全量重建。
func runIAMV4DeleteAll() error {
	syncer, err := prepareIAMV4()
	if err != nil {
		return err
	}

	kt := newIAMV4Kit()

	plan, err := syncer.PlanDeleteAll(kt)
	if err != nil {
		return err
	}

	if !plan.HasDeletions() {
		fmt.Println("线上没有可删除的元素")

		return nil
	}

	fmt.Println("待删除的元素：")
	fmt.Print(plan)

	if !deleteIAMV4Yes {
		if confirmErr := confirmPrompt("\n该操作不可恢复，清空后需执行 migrate init-iam-v4 重建。输入 yes 继续："); confirmErr != nil {
			return confirmErr
		}
	}

	if err := syncer.Apply(kt, plan, modelsync.ApplyOption{
		AllowDelete: true,
		Logf: func(format string, args ...interface{}) {
			fmt.Printf("  "+format+"\n", args...)
		},
	}); err != nil {
		return err
	}

	fmt.Printf("删除完成：角色 %d 个、操作 %d 个、资源类型 %d 个\n",
		len(plan.DeleteRoles), len(plan.DeleteActions), len(plan.DeleteResourceTypes))
	fmt.Println("请执行 migrate init-iam-v4 按本地定义重建模型")

	return nil
}

// runIAMV4Diff 对比本地模型与 IAM V4 线上状态
func runIAMV4Diff() error {
	syncer, err := prepareIAMV4()
	if err != nil {
		return err
	}

	plan, err := syncer.Plan(newIAMV4Kit())
	if err != nil {
		return err
	}

	fmt.Print(plan)

	if len(plan.Conflicts) > 0 {
		return errors.Errorf("存在 %d 处需人工处理的差异", len(plan.Conflicts))
	}

	return nil
}

// runIAMV4Sync 同步 IAM V4 模型
func runIAMV4Sync(allowDelete bool) error {
	syncer, err := prepareIAMV4()
	if err != nil {
		return errors.Wrap(err, "prepare iam v4 syncer")
	}

	kt := newIAMV4Kit()

	plan, err := syncer.Plan(kt)
	if err != nil {
		return err
	}

	if !plan.HasChanges() {
		fmt.Println("线上模型与本地定义一致，无需变更")

		return nil
	}

	fmt.Println("待施加的变更：")
	fmt.Print(plan)

	if allowDelete {
		if !plan.HasDeletions() {
			fmt.Println("线上没有需要清理的元素")

			return nil
		}

		if err := confirmDeletion(plan); err != nil {
			return err
		}
	}

	if err := syncer.Apply(kt, plan, modelsync.ApplyOption{
		AllowDelete: allowDelete,
		Logf: func(format string, args ...interface{}) {
			fmt.Printf("  "+format+"\n", args...)
		},
	}); err != nil {
		return err
	}

	fmt.Println("同步完成")

	return nil
}

// confirmDeletion 要求人工确认删除。非交互环境（读不到输入）视为拒绝，
// 避免在流水线里被无人值守地执行掉。
func confirmDeletion(plan *modelsync.Plan) error {
	fmt.Printf("\n即将删除 %d 个角色、%d 个操作、%d 个资源类型，"+
		"相关授权会一并失效且无法恢复。\n输入 yes 继续：",
		len(plan.DeleteRoles), len(plan.DeleteActions), len(plan.DeleteResourceTypes))

	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return errors.Wrap(err, "读取确认输入失败，非交互环境请勿使用该命令")
	}

	if strings.TrimSpace(answer) != "yes" {
		return errors.New("已取消")
	}

	return nil
}

// newIAMV4Kit 构造调用权限中心的 kit，租户 ID 必须落在 kit 上，
// 客户端据此填充 X-Bk-Tenant-Id 请求头。
func newIAMV4Kit() *kit.Kit {
	return &kit.Kit{Ctx: context.Background(), TenantID: iamV4TenantID}
}

// prepareIAMV4 构建 IAM V4 同步器
func prepareIAMV4() (*modelsync.Syncer, error) {
	if err := cc.LoadSettings(SysOpt.Sys); err != nil {
		return nil, errors.Wrap(err, "load settings from config files")
	}

	logs.InitLogger(cc.AuthServer().Log.Logs())

	if iamV4TenantID == "" {
		iamV4TenantID = modelTenantID()
	}

	settings := cc.AuthServer().IAM.V4
	if err := checkIAMV4Settings(settings); err != nil {
		return nil, err
	}

	cli, err := clientv4.NewClient(&clientv4.Config{
		GatewayURL: settings.GatewayURL,
		SystemID:   settings.SystemID,
		AppCode:    settings.AppCode,
		AppSecret:  settings.AppSecret,
	})
	if err != nil {
		return nil, errors.Wrap(err, "new iam v4 gateway client")
	}

	spec := buildSystemSpec(settings)

	syncer, err := modelsync.NewSyncer(cli, spec)
	if err != nil {
		return nil, err
	}

	fmt.Printf("网关 %s，系统 %s，回调地址 %s，租户 %s\n",
		settings.GatewayURL, settings.SystemID, spec.CallbackURL, iamV4TenantID)

	return syncer, nil
}

// buildSystemSpec 组装系统的期望状态
func buildSystemSpec(settings cc.IAMV4) modelsync.SystemSpec {
	return modelsync.SystemSpec{
		ID:          settings.SystemID,
		Name:        model.SystemName,
		Clients:     []string{settings.AppCode},
		CallbackURL: strings.TrimSuffix(settings.CallbackHost, "/") + model.CallbackPath,
		// 管理员留空，不覆盖权限中心侧已配置的人员
	}
}

// checkIAMV4Settings 校验 iam.v4 配置。
func checkIAMV4Settings(settings cc.IAMV4) error {
	missing := make([]string, 0, 5)
	if settings.GatewayURL == "" {
		missing = append(missing, "gateway_url")
	}
	if settings.SystemID == "" {
		missing = append(missing, "system_id")
	}
	if settings.AppCode == "" {
		missing = append(missing, "app_code")
	}
	if settings.AppSecret == "" {
		missing = append(missing, "app_secret")
	}
	if settings.CallbackHost == "" {
		missing = append(missing, "callback_host")
	}

	if len(missing) > 0 {
		return errors.Errorf("iam.v4 配置缺少 %s", strings.Join(missing, ", "))
	}

	return nil
}

func init() {
	for _, sub := range []*cobra.Command{
		migrateInitIAMV4Cmd, migrateDiffIAMV4Cmd, migratePruneIAMV4Cmd, migrateDeleteIAMV4Cmd,
	} {
		sub.Flags().StringVar(&iamV4TenantID, "tenant-id", "",
			"the tenant id carried in the X-Bk-Tenant-Id header when requesting bkiam, "+
				"defaults to system in multi-tenant mode, otherwise default")
		migrateCmd.AddCommand(sub)
	}

	migrateDeleteIAMV4Cmd.Flags().BoolVar(&deleteIAMV4Yes, "yes", false,
		"skip the interactive confirmation, for non-interactive environments")
}
