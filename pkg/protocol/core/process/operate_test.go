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

// Package pbproc provides process core protocol struct and convert functions.
package pbproc

import (
	"testing"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

func TestCanProcessOperate(t *testing.T) {
	tests := []struct {
		name          string
		op            table.ProcessOperateType
		status        string
		managedStatus string
		syncStatus    string
		want          bool
	}{
		// 进程状态
		{
			name:          "已停止允许启动",
			op:            table.StartProcessOperate,
			status:        table.ProcessStatusStopped.String(),
			managedStatus: table.ProcessManagedStatusUnmanaged.String(),
			syncStatus:    "",
			want:          true,
		},
		{
			name:          "已停止不允许停止",
			op:            table.StopProcessOperate,
			status:        table.ProcessStatusStopped.String(),
			managedStatus: table.ProcessManagedStatusUnmanaged.String(),
			syncStatus:    "",
			want:          false,
		},
		{
			name:          "运行中不允许启动",
			op:            table.StartProcessOperate,
			status:        table.ProcessStatusRunning.String(),
			managedStatus: table.ProcessManagedStatusUnmanaged.String(),
			syncStatus:    "",
			want:          false,
		},
		{
			name:          "运行中允许停止",
			op:            table.StopProcessOperate,
			status:        table.ProcessStatusRunning.String(),
			managedStatus: table.ProcessManagedStatusManaged.String(),
			syncStatus:    "",
			want:          true,
		},

		// 托管状态
		{
			name:          "未托管允许托管",
			op:            table.RegisterProcessOperate,
			status:        table.ProcessStatusStopped.String(),
			managedStatus: table.ProcessManagedStatusUnmanaged.String(),
			syncStatus:    "",
			want:          true,
		},
		{
			name:          "未托管不允许取消托管",
			op:            table.UnregisterProcessOperate,
			status:        table.ProcessStatusRunning.String(),
			managedStatus: table.ProcessManagedStatusUnmanaged.String(),
			syncStatus:    "",
			want:          false,
		},
		{
			name:          "托管中允许取消托管",
			op:            table.UnregisterProcessOperate,
			status:        table.ProcessStatusRunning.String(),
			managedStatus: table.ProcessManagedStatusManaged.String(),
			syncStatus:    "",
			want:          true,
		},
		{
			name:          "托管中不允许托管",
			op:            table.RegisterProcessOperate,
			status:        table.ProcessStatusStopped.String(),
			managedStatus: table.ProcessManagedStatusManaged.String(),
			syncStatus:    "",
			want:          false,
		},

		// running 操作
		{
			name:          "运行中允许重启",
			op:            table.RestartProcessOperate,
			status:        table.ProcessStatusRunning.String(),
			managedStatus: table.ProcessManagedStatusManaged.String(),
			syncStatus:    "",
			want:          true,
		},
		{
			name:          "运行中允许重载",
			op:            table.ReloadProcessOperate,
			status:        table.ProcessStatusRunning.String(),
			managedStatus: table.ProcessManagedStatusManaged.String(),
			syncStatus:    "",
			want:          true,
		},

		// ing 状态禁止所有
		{
			name:          "启动中所有操作禁止",
			op:            table.StopProcessOperate,
			status:        table.ProcessStatusStarting.String(),
			managedStatus: table.ProcessManagedStatusManaged.String(),
			syncStatus:    "",
			want:          false,
		},
		{
			name:          "取消托管中所有操作禁止",
			op:            table.RegisterProcessOperate,
			status:        table.ProcessStatusStopped.String(),
			managedStatus: table.ProcessManagedStatusStopping.String(),
			syncStatus:    "",
			want:          false,
		},

		// deleted 状态
		{
			name:          "运行中且已删除允许停止",
			op:            table.StopProcessOperate,
			status:        table.ProcessStatusRunning.String(),
			managedStatus: table.ProcessManagedStatusManaged.String(),
			syncStatus:    table.Deleted.String(),
			want:          true,
		},
		{
			name:          "已停止且已删除不允许停止",
			op:            table.StopProcessOperate,
			status:        table.ProcessStatusStopped.String(),
			managedStatus: table.ProcessManagedStatusManaged.String(),
			syncStatus:    table.Deleted.String(),
			want:          false,
		},
		{
			name:          "已删除且托管中允许取消托管",
			op:            table.UnregisterProcessOperate,
			status:        table.ProcessStatusRunning.String(),
			managedStatus: table.ProcessManagedStatusManaged.String(),
			syncStatus:    table.Deleted.String(),
			want:          true,
		},
		{
			name:          "已删除且未托管不允许托管",
			op:            table.RegisterProcessOperate,
			status:        table.ProcessStatusRunning.String(),
			managedStatus: table.ProcessManagedStatusUnmanaged.String(),
			syncStatus:    table.Deleted.String(),
			want:          false,
		},
		{
			name:          "已删除不允许启动",
			op:            table.StartProcessOperate,
			status:        table.ProcessStatusStopped.String(),
			managedStatus: table.ProcessManagedStatusUnmanaged.String(),
			syncStatus:    table.Deleted.String(),
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, _ := CanProcessOperate(tt.op, table.ProcessInfo{
				BkStartParamRegex: "",
				WorkPath:          "/data/work",
				PidFile:           "test",
				User:              "test",
				ReloadCmd:         "reload test",
				RestartCmd:        "restart test",
				StartCmd:          "start test",
				StopCmd:           "stop test",
				FaceStopCmd:       "kill -9",
				Timeout:           30,
				StartCheckSecs:    60,
			}, tt.status, tt.managedStatus, tt.syncStatus)
			if got != tt.want {
				t.Errorf("canProcessOperate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// fullAttrsInfo 命令与通用 3 项齐全的进程配置（属性矩阵用例基线）
func fullAttrsInfo() table.ProcessInfo {
	return table.ProcessInfo{
		WorkPath:          "/data/work",
		PidFile:           "test.pid",
		User:              "test",
		ReloadCmd:         "reload test",
		RestartCmd:        "restart test",
		StartCmd:          "start test",
		StopCmd:           "stop test",
		FaceStopCmd:       "kill -9",
		Timeout:           30,
		StartCheckSecs:    60,
		BkStartParamRegex: "",
	}
}

// mutateAttrs 基于完整配置构造字段变更的变体
func mutateAttrs(info table.ProcessInfo, mutate func(*table.ProcessInfo)) table.ProcessInfo {
	mutate(&info)
	return info
}

// TestCanProcessOperateByAttrs 属性矩阵全组合 + 状态类校验：
// 属性矩阵对齐 gsekit（必备命令 + 通用 3 项），叠加状态未知 / ing 中间态 / Abnormal / Updated 状态类校验
func TestCanProcessOperateByAttrs(t *testing.T) {
	// 状态基线：属性矩阵用例不受状态影响，统一使用正常状态
	running := table.ProcessStatusRunning.String()
	stopped := table.ProcessStatusStopped.String()
	unmanaged := table.ProcessManagedStatusUnmanaged.String()
	managed := table.ProcessManagedStatusManaged.String()
	synced := table.Synced.String()

	tests := []struct {
		name         string
		op           table.ProcessOperateType
		info         table.ProcessInfo
		processState string
		managedState string
		syncStatus   string
		want         bool
		wantReason   string
	}{
		// 属性齐全：所有操作允许（状态基线，不受进程状态 / 托管状态 / 同步状态变化影响）
		{"start 属性齐全允许", table.StartProcessOperate, fullAttrsInfo(), running, unmanaged, synced, true, DisableReasonNone},
		{"stop 属性齐全允许", table.StopProcessOperate, fullAttrsInfo(), running, unmanaged, synced, true, DisableReasonNone},
		{"kill 属性齐全允许", table.KillProcessOperate, fullAttrsInfo(), running, unmanaged, synced, true, DisableReasonNone},
		{"restart 属性齐全允许", table.RestartProcessOperate, fullAttrsInfo(), running, unmanaged, synced, true, DisableReasonNone},
		{"reload 属性齐全允许", table.ReloadProcessOperate, fullAttrsInfo(), running, unmanaged, synced, true, DisableReasonNone},
		{"register 属性齐全允许", table.RegisterProcessOperate, fullAttrsInfo(), stopped, unmanaged, synced, true, DisableReasonNone},
		{"unregister 属性齐全允许", table.UnregisterProcessOperate, fullAttrsInfo(), running, managed, synced, true, DisableReasonNone},
		{"update_register 属性齐全允许", table.UpdateRegisterProcessOperate, fullAttrsInfo(), stopped, unmanaged, synced, true, DisableReasonNone},
		{"pull 属性齐全允许", table.PullProcessOperate, fullAttrsInfo(), stopped, unmanaged, synced, true, DisableReasonNone},

		// 必备命令缺失
		{"start 缺 start_cmd", table.StartProcessOperate,
			mutateAttrs(fullAttrsInfo(), func(i *table.ProcessInfo) { i.StartCmd = "" }),
			running, unmanaged, synced, false, DisableReasonCmdNotConfigured},
		{"stop 缺 stop_cmd", table.StopProcessOperate,
			mutateAttrs(fullAttrsInfo(), func(i *table.ProcessInfo) { i.StopCmd = "" }),
			running, unmanaged, synced, false, DisableReasonCmdNotConfigured},
		{"kill 缺 face_stop_cmd", table.KillProcessOperate,
			mutateAttrs(fullAttrsInfo(), func(i *table.ProcessInfo) { i.FaceStopCmd = "" }),
			running, unmanaged, synced, false, DisableReasonCmdNotConfigured},
		{"restart 缺 restart_cmd", table.RestartProcessOperate,
			mutateAttrs(fullAttrsInfo(), func(i *table.ProcessInfo) { i.RestartCmd = "" }),
			running, unmanaged, synced, false, DisableReasonCmdNotConfigured},
		{"reload 缺 reload_cmd", table.ReloadProcessOperate,
			mutateAttrs(fullAttrsInfo(), func(i *table.ProcessInfo) { i.ReloadCmd = "" }),
			running, unmanaged, synced, false, DisableReasonCmdNotConfigured},

		// register 依赖 start_cmd（对齐 gsekit SET_AUTO，行为变化点）
		{"register 缺 start_cmd", table.RegisterProcessOperate,
			mutateAttrs(fullAttrsInfo(), func(i *table.ProcessInfo) { i.StartCmd = "" }),
			stopped, unmanaged, synced, false, DisableReasonCmdNotConfigured},

		// unregister 零属性要求（对齐 gsekit UNSET_AUTO）
		{"unregister 全空配置允许", table.UnregisterProcessOperate,
			table.ProcessInfo{}, running, managed, synced, true, DisableReasonNone},

		// pull 零属性要求
		{"pull 全空配置允许", table.PullProcessOperate,
			table.ProcessInfo{}, stopped, unmanaged, synced, true, DisableReasonNone},

		// update_register 仅依赖通用 3 项，不依赖命令
		{"update_register 无命令但通用 3 项齐全允许", table.UpdateRegisterProcessOperate,
			mutateAttrs(fullAttrsInfo(), func(i *table.ProcessInfo) {
				i.StartCmd = ""
				i.StopCmd = ""
				i.RestartCmd = ""
				i.ReloadCmd = ""
				i.FaceStopCmd = ""
			}),
			stopped, unmanaged, synced, true, DisableReasonNone},

		// 通用 3 项缺失（work_path / pid_file / user）
		{"start 缺 work_path", table.StartProcessOperate,
			mutateAttrs(fullAttrsInfo(), func(i *table.ProcessInfo) { i.WorkPath = "" }),
			running, unmanaged, synced, false, DisableReasonCmdNotConfigured},
		{"start 缺 pid_file", table.StartProcessOperate,
			mutateAttrs(fullAttrsInfo(), func(i *table.ProcessInfo) { i.PidFile = "" }),
			running, unmanaged, synced, false, DisableReasonCmdNotConfigured},
		{"start 缺 user", table.StartProcessOperate,
			mutateAttrs(fullAttrsInfo(), func(i *table.ProcessInfo) { i.User = "" }),
			running, unmanaged, synced, false, DisableReasonCmdNotConfigured},
		{"stop 缺 work_path", table.StopProcessOperate,
			mutateAttrs(fullAttrsInfo(), func(i *table.ProcessInfo) { i.WorkPath = "" }),
			running, unmanaged, synced, false, DisableReasonCmdNotConfigured},
		{"register 缺 work_path", table.RegisterProcessOperate,
			mutateAttrs(fullAttrsInfo(), func(i *table.ProcessInfo) { i.WorkPath = "" }),
			stopped, unmanaged, synced, false, DisableReasonCmdNotConfigured},
		{"update_register 缺 pid_file", table.UpdateRegisterProcessOperate,
			mutateAttrs(fullAttrsInfo(), func(i *table.ProcessInfo) { i.PidFile = "" }),
			stopped, unmanaged, synced, false, DisableReasonCmdNotConfigured},

		// 全空配置：除零属性要求操作外全部禁止
		{"start 全空配置禁止", table.StartProcessOperate,
			table.ProcessInfo{}, running, unmanaged, synced, false, DisableReasonCmdNotConfigured},
		{"stop 全空配置禁止", table.StopProcessOperate,
			table.ProcessInfo{}, running, unmanaged, synced, false, DisableReasonCmdNotConfigured},
		{"kill 全空配置禁止", table.KillProcessOperate,
			table.ProcessInfo{}, running, unmanaged, synced, false, DisableReasonCmdNotConfigured},
		{"restart 全空配置禁止", table.RestartProcessOperate,
			table.ProcessInfo{}, running, unmanaged, synced, false, DisableReasonCmdNotConfigured},
		{"reload 全空配置禁止", table.ReloadProcessOperate,
			table.ProcessInfo{}, running, unmanaged, synced, false, DisableReasonCmdNotConfigured},
		{"register 全空配置禁止", table.RegisterProcessOperate,
			table.ProcessInfo{}, stopped, unmanaged, synced, false, DisableReasonCmdNotConfigured},
		{"update_register 全空配置禁止", table.UpdateRegisterProcessOperate,
			table.ProcessInfo{}, stopped, unmanaged, synced, false, DisableReasonCmdNotConfigured},

		// 状态未知：进程状态或托管状态为空
		{"进程状态未知禁止", table.StartProcessOperate,
			fullAttrsInfo(), "", unmanaged, synced, false, DisableReasonUnknownProcessState},
		{"托管状态未知禁止", table.StartProcessOperate,
			fullAttrsInfo(), running, "", synced, false, DisableReasonUnknownProcessState},

		// ing（中间态）：进程状态或托管状态处于中间态时禁止所有操作
		{"启动中禁止启动", table.StartProcessOperate,
			fullAttrsInfo(), table.ProcessStatusStarting.String(), unmanaged, synced, false, DisableReasonTaskRunning},
		{"停止中禁止停止", table.StopProcessOperate,
			fullAttrsInfo(), table.ProcessStatusStopping.String(), unmanaged, synced, false, DisableReasonTaskRunning},
		{"重启中禁止重启", table.RestartProcessOperate,
			fullAttrsInfo(), table.ProcessStatusRestarting.String(), unmanaged, synced, false, DisableReasonTaskRunning},
		{"托管中禁止重启", table.RestartProcessOperate,
			fullAttrsInfo(), running, table.ProcessManagedStatusStarting.String(), synced, false, DisableReasonTaskRunning},
		{"取消托管中禁止取消托管", table.UnregisterProcessOperate,
			fullAttrsInfo(), running, table.ProcessManagedStatusStopping.String(), synced, false, DisableReasonTaskRunning},

		// 进程异常（syncStatus = Abnormal）：仅运行中可停、已托管可取消托管，其余一律禁止
		{"异常进程运行中允许停止", table.StopProcessOperate,
			fullAttrsInfo(), running, managed, table.Abnormal.String(), true, DisableReasonNone},
		{"异常进程部分运行允许强停", table.KillProcessOperate,
			fullAttrsInfo(), table.ProcessStatusPartlyRunning.String(), unmanaged, table.Abnormal.String(), true, DisableReasonNone},
		{"异常进程已停止禁止停止", table.StopProcessOperate,
			fullAttrsInfo(), stopped, unmanaged, table.Abnormal.String(), false, DisableReasonNoNeedOperate},
		{"异常进程已托管允许取消托管", table.UnregisterProcessOperate,
			fullAttrsInfo(), stopped, managed, table.Abnormal.String(), true, DisableReasonNone},
		{"异常进程未托管禁止取消托管", table.UnregisterProcessOperate,
			fullAttrsInfo(), stopped, unmanaged, table.Abnormal.String(), false, DisableReasonNoNeedOperate},
		{"异常进程禁止启动", table.StartProcessOperate,
			fullAttrsInfo(), stopped, unmanaged, table.Abnormal.String(), false, DisableReasonProcessAbnormal},
		{"异常进程禁止更新托管", table.UpdateRegisterProcessOperate,
			fullAttrsInfo(), stopped, unmanaged, table.Abnormal.String(), false, DisableReasonProcessAbnormal},

		// 更新托管信息特殊规则：syncStatus = Updated 仅允许 update_register / pull
		{"已更新允许更新托管", table.UpdateRegisterProcessOperate,
			fullAttrsInfo(), stopped, unmanaged, table.Updated.String(), true, DisableReasonNone},
		{"已更新允许下发", table.PullProcessOperate,
			fullAttrsInfo(), stopped, unmanaged, table.Updated.String(), true, DisableReasonNone},
		{"已更新禁止启动", table.StartProcessOperate,
			fullAttrsInfo(), stopped, unmanaged, table.Updated.String(), false, DisableReasonNoRegisterUpdate},
		{"已更新禁止停止", table.StopProcessOperate,
			fullAttrsInfo(), running, unmanaged, table.Updated.String(), false, DisableReasonNoRegisterUpdate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, reason := CanProcessOperateByAttrs(
				tt.op, tt.info, tt.processState, tt.managedState, tt.syncStatus)
			if got != tt.want {
				t.Errorf("CanProcessOperateByAttrs(%s) = %v, want %v, reason: %s", tt.op, got, tt.want, reason)
			}
			if reason != tt.wantReason {
				t.Errorf("CanProcessOperateByAttrs(%s) reason = %s, want %s", tt.op, reason, tt.wantReason)
			}
		})
	}
}

// TestCanProcessOperateByAttrsReasonUnique 属性类失败原因唯一：正常状态基线下，属性缺失只产生
// NONE / CMD_NOT_CONFIGURED（状态类失败原因由 TestCanProcessOperateByAttrs 覆盖）
func TestCanProcessOperateByAttrsReasonUnique(t *testing.T) {
	ops := []table.ProcessOperateType{
		table.StartProcessOperate,
		table.StopProcessOperate,
		table.KillProcessOperate,
		table.RestartProcessOperate,
		table.ReloadProcessOperate,
		table.RegisterProcessOperate,
		table.UnregisterProcessOperate,
		table.UpdateRegisterProcessOperate,
		table.PullProcessOperate,
	}

	running := table.ProcessStatusRunning.String()
	unmanaged := table.ProcessManagedStatusUnmanaged.String()
	synced := table.Synced.String()

	for _, op := range ops {
		_, message, reason := CanProcessOperateByAttrs(
			op, table.ProcessInfo{}, running, unmanaged, synced)
		if reason != DisableReasonNone && reason != DisableReasonCmdNotConfigured {
			t.Errorf("CanProcessOperateByAttrs(%s) reason = %s, only NONE / CMD_NOT_CONFIGURED expected", op, reason)
		}
		if reason == DisableReasonCmdNotConfigured && message == "" {
			t.Errorf("CanProcessOperateByAttrs(%s) reason is CMD_NOT_CONFIGURED but message is empty", op)
		}
	}
}
