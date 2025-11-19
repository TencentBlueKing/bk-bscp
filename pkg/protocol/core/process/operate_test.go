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
			got := CanProcessOperate(tt.op, tt.status, tt.managedStatus, tt.syncStatus)
			if got != tt.want {
				t.Errorf("canProcessOperate() = %v, want %v", got, tt.want)
			}
		})
	}
}
