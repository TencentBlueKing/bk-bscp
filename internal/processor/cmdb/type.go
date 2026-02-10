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

// Package cmdb provides cmdb client.
package cmdb

import (
	"time"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// Set 集群
type Set struct {
	ID     int
	Name   string
	SetEnv string
	Module []Module
}

// Module 模块
type Module struct {
	ID                int
	ServiceTemplateID int
	Name              string
	Host              []Host
	SvcInst           []SvcInst
}

// Host 主机
type Host struct {
	ID      int
	IP      string
	IPV6    string // 内网 IPv6，同步自 CC bk_host_innerip_v6
	CloudId int
	AgentID string
}

// SvcInst 服务实例
type SvcInst struct {
	ID       int
	Name     string
	ProcInst []ProcInst
}

// ProcInst 进程实例
type ProcInst struct {
	ID                int
	HostID            int
	ProcessTemplateID int
	Name              string
	FuncName          string
	ProcNum           int
	table.ProcessInfo
}

// HostInfo 构建 HostID -> HostInfo 映射
type HostInfo struct {
	IP      string
	IPV6    string
	CloudId int
	AgentID string
}

// Bizs 业务
type Bizs map[int][]Set

// ProcessSyncItem 一次同步中单个进程的结果
type ProcessWithInstances struct {
	Process   *table.Process
	Instances []*table.ProcessInstance
}

// SyncProcessResult 表示一次进程同步任务的汇总结果
type SyncProcessResult struct {
	Items []*ProcessWithInstances
}

// ModuleAliasKey 用于 moduleCounter 的 key，表示 (moduleID, alias) 组合
type ModuleAliasKey struct {
	ModuleID int
	Alias    string
}

// HostProcessKey 用于 HostCounter 的 key，表示 (ccProcessID, hostID) 组合
type HostProcessKey struct {
	CcProcessID int
	HostID      int
}

// SyncContext 同步过程中的共享上下文
type SyncContext struct {
	Kit           *kit.Kit
	Dao           dao.Set
	Tx            *gen.QueryTx
	Now           time.Time
	HostCounter   map[HostProcessKey]int // key: HostProcessKey{ccProcessID, hostID}
	ModuleCounter map[ModuleAliasKey]int // key: ModuleAliasKey{moduleID, alias}
}

// BuildInstancesParams buildInstances 函数的参数
type BuildInstancesParams struct {
	BizID            uint32
	HostID           uint32
	ModuleID         uint32
	CcProcessID      uint32
	ProcNum          int
	ExistCount       int
	MaxModuleInstSeq int
	MaxHostInstSeq   int
	Alias            string
}

// ReconcileInstancesParams reconcileProcessInstances 函数的参数
type ReconcileInstancesParams struct {
	BizID       uint32
	ProcessID   uint32
	HostID      uint32
	ModuleID    uint32
	CcProcessID uint32
	Alias       string
	OldNum      int
	NewNum      int
}

// BuildProcessChangesParams BuildProcessChanges 函数的参数
type BuildProcessChangesParams struct {
	NewProcess *table.Process
	OldProcess *table.Process
}

// ReorderParams reorderModuleInstSeq 函数的参数
type ReorderParams struct {
	BizID      uint32
	ModuleID   uint32
	Alias      string
	ExcludeIDs []uint32
}
