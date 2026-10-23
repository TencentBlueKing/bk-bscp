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

// ProcessSyncItem 一次同步中单个进程的结果
type ProcessWithInstances struct {
	Process   *table.Process
	Instances []*table.ProcessInstance
}

// SyncProcessResult 表示一次进程同步任务的汇总结果
type SyncProcessResult struct {
	Items []*ProcessWithInstances
	// Changed 表示本次 diff 是否产生了实际变更（增删改进程或实例、重排序列）。
	// 为 false 时调用方应跳过后续的冲突检查等全表级写操作，避免事件驱动的全量同步产生无意义写。
	Changed bool
}

// ModuleAliasKey 用于 moduleCounter 的 key，表示 (moduleID, alias) 组合
type ModuleAliasKey struct {
	ModuleID int
	Alias    string
}

// SyncContext 同步过程中的共享上下文
type SyncContext struct {
	Kit           *kit.Kit
	Dao           dao.Set
	Tx            *gen.QueryTx
	Now           time.Time
	ModuleCounter map[ModuleAliasKey]int // key: ModuleAliasKey{moduleID, alias}
	// InstancesByProcessID 事务开始时的进程实例快照（processID -> instances），
	// 由 SyncProcessData 预取，供 diff 阶段内存判定，避免逐进程查库（N+1）。
	// 为 nil 表示未预取，调用方回退到数据库查询。
	InstancesByProcessID map[uint32][]*table.ProcessInstance
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
	// 当前主机上该进程已占用的 HostInstSeq 列表（用于填洞）
	UsedHostInstSeqs []int
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
	BizID    uint32
	ModuleID uint32
	Alias    string
}
