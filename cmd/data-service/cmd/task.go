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
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/pkg/task"
	"github.com/TencentBlueKing/bk-bscp/pkg/task/builder/hello"
	"github.com/TencentBlueKing/bk-bscp/pkg/task/register"
)

// cmd for migration
var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "task command",
	Run: func(cmd *cobra.Command, args []string) {

	},
}

var taskRunCmd = &cobra.Command{
	Use:   "run",
	Short: "run task",
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		if err = cc.LoadSettings(SysOpt); err != nil {
			fmt.Println("load settings from config files failed, err:", err)
			return
		}

		logs.InitLogger(cc.DataService().Log.Logs())
		// 注意 register 要在 taskMgr 初始化之前
		register.RegisterExecutor()
		taskMgr, err := task.NewTaskMgr(
			context.Background(),
			cc.DataService().Service.Etcd,
			cc.DataService().Sharding.AdminDatabase,
		)
		if err != nil {
			fmt.Println("new task manager failed, err:", err)
			return
		}
		if err = taskMgr.Run(); err != nil {
			taskMgr.Stop()
			fmt.Println("run task manager failed, err:", err)
			return
		}

	},
}

var taskSendCmd = &cobra.Command{
	Use:   "send",
	Short: "send task",
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		if err = cc.LoadSettings(SysOpt); err != nil {
			fmt.Println("load settings from config files failed, err:", err)
			return
		}

		logs.InitLogger(cc.DataService().Log.Logs())
		taskMgr, err := task.NewTaskMgr(
			context.Background(),
			cc.DataService().Service.Etcd,
			cc.DataService().Sharding.AdminDatabase,
		)
		if err != nil {
			fmt.Println("new task manager failed, err:", err)
			return
		}

		// args
		a, err := cmd.Flags().GetInt("a")
		if err != nil {
			fmt.Println("get a failed, err:", err)
			return
		}
		b, err := cmd.Flags().GetInt("b")
		if err != nil {
			fmt.Println("get b failed, err:", err)
			return
		}
		helloTask, err := task.NewByTaskBuilder(
			hello.NewHelloTask(a, b),
		)
		if err != nil {
			fmt.Println("new task failed, err:", err)
			return
		}
		taskMgr.Dispatch(helloTask)
	},
}

func init() {
	taskSendCmd.Flags().Int("a", 1, "a")
	taskSendCmd.Flags().Int("b", 2, "b")

	taskCmd.AddCommand(taskSendCmd)
	taskCmd.AddCommand(taskRunCmd)

	rootCmd.AddCommand(taskCmd)
}
