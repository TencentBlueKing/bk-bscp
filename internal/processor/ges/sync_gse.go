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

// Package gse provides gse service.
package gse

import (
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

const (
	// DefaultCPULimit 默认 CPU 使用率上限百分比
	DefaultCPULimit = 30.0

	// DefaultMemLimit 默认内存使用率上限百分比
	DefaultMemLimit = 10.0

	// DefaultStartCheckSecs 默认启动后检查存活的时间（秒）
	DefaultStartCheckSecs = 5
)

// BuildProcessOperateParams 构建 ProcessOperate 的参数
type BuildProcessOperateParams struct {
	BizID             uint32            // 业务ID
	Alias             string            // 进程别名
	ProcessInstanceID uint32            // 进程实例ID
	AgentID           []string          // Agent ID列表
	GseOpType         int               // GSE操作类型
	ProcessInfo       table.ProcessInfo // 进程配置信息
}

// BuildProcessOperate 构建 GSE ProcessOperate 对象
// 查询操作（OpTypeQuery）只需要构建基本的 Meta 和 OpType 信息，不需要 Spec
// 其他操作需要完整的 Spec 信息（包括 Identity、Control、Resource、MonitorPolicy）
func BuildProcessOperate(params BuildProcessOperateParams) gse.ProcessOperate {
	// 构建基础的 ProcessOperate 对象
	processOperate := gse.ProcessOperate{
		Meta: gse.ProcessMeta{
			Namespace: gse.BuildNamespace(params.BizID),
			Name:      gse.BuildProcessName(params.Alias, params.ProcessInstanceID),
		},
		AgentIDList: params.AgentID,
		OpType:      gse.OpType(params.GseOpType),
	}

	// 查询操作不需要 Spec
	if params.GseOpType == int(gse.OpTypeQuery) {
		return processOperate
	}

	// 非查询操作需要添加完整的 Spec 信息
	processOperate.Spec = gse.ProcessSpec{
		Identity: gse.ProcessIdentity{
			ProcName:  params.Alias,
			SetupPath: params.ProcessInfo.WorkPath,
			PidPath:   params.ProcessInfo.PidFile,
			User:      params.ProcessInfo.User,
		},
		Control: gse.ProcessControl{
			StartCmd:   params.ProcessInfo.StartCmd,
			StopCmd:    params.ProcessInfo.StopCmd,
			RestartCmd: params.ProcessInfo.RestartCmd,
			ReloadCmd:  params.ProcessInfo.ReloadCmd,
			KillCmd:    params.ProcessInfo.FaceStopCmd,
		},
		Resource: gse.ProcessResource{
			CPU: DefaultCPULimit,
			Mem: DefaultMemLimit,
		},
		MonitorPolicy: gse.ProcessMonitorPolicy{
			AutoType:       gse.AutoTypePersistent,
			StartCheckSecs: DefaultStartCheckSecs,
			OpTimeout:      params.ProcessInfo.Timeout,
		},
	}

	return processOperate
}
