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

// Package priority 把进程操作的启动优先级映射成任务框架的阶段编排。
//
// 阶段的串行推进、计数、级联阻断与重启恢复都由任务框架负责，这里只决定
// 「哪些任务归入哪个阶段、阶段之间谁先谁后、阻断时给用户看什么文案」。
package priority

import (
	"fmt"
	"sort"

	taskTypes "github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

// immediateStageName 不参与优先级编排的阶段名
const immediateStageName = "immediate"

// TaskItem 参与阶段编排的单个实例任务
type TaskItem struct {
	TaskID   string
	Priority int
	OpType   table.ProcessOperateType
	// HostKey 主机标识（bk_host_id），同阶段内同主机任务据此错峰延迟；
	// 不同主机的任务序号各自独立计数，互不影响
	HostKey string
}

// StagePlan 一次进程操作请求的阶段编排结果
type StagePlan struct {
	// Stages 传给任务框架的阶段定义，Seq 从 0 连续递增
	Stages []*taskTypes.Stage
	// StageSeqOf 任务与阶段的归属关系
	StageSeqOf map[string]int
	// StaggerIndexOf 任务在所属阶段内、同主机分组中的序号（0 起），用于同主机任务错峰下发
	StaggerIndexOf map[string]int
}

// StaggerIndex 返回任务在所属阶段内同主机分组中的序号（0 起），未纳入编排时返回 0
func (p *StagePlan) StaggerIndex(taskID string) int {
	if p == nil {
		return 0
	}
	return p.StaggerIndexOf[taskID]
}

// StageSeq 返回任务所属的阶段序号，未纳入编排时返回 0
func (p *StagePlan) StageSeq(taskID string) int {
	if p == nil {
		return 0
	}
	return p.StageSeqOf[taskID]
}

// BuildStages 按操作类型与启动优先级把实例任务排成阶段序列。
//
//  1. 托管类操作（register / unregister / update_register）不参与优先级编排，
//     归入首个阶段并且失败不阻断后续，与第一个优先级阶段同时下发；
//
//  2. 参与编排的操作按 priority 分组，start / reload / kill 升序，stop / restart 降序；
//
//  3. 每个 priority 一个阶段，阶段内并行、阶段之间串行，任一阶段有失败即阻断后续全部阶段；
//
//  4. 阶段内同主机（HostKey 相同）的任务按序号错峰延迟，不同主机互不影响。
//     同主机同优先级下不同进程的实例合并计数，例如同一台机器上 4 个实例（interval=3s）：
//
//     任务（同主机同优先级）  主机内序号	 延迟
//     nginx-1 实例 A			0		   0s
//     mysql-1 实例 B			1		   3s
//     nginx-1 实例 C			2		   6s
//     mysql-1 实例 D			3		   9s
func BuildStages(items []TaskItem) *StagePlan {
	immediate := make([]TaskItem, 0)
	groups := make(map[int][]TaskItem)
	var order table.PriorityOrder
	hasOrdered := false

	for _, item := range items {
		// 获取操作类型对应的优先级排序方向
		o, ok := table.ProcessOperatePriorityOrder(item.OpType)
		if !ok {
			immediate = append(immediate, item)
			continue
		}
		// 同一次请求内的排序方向由首个参与编排的任务决定
		if !hasOrdered {
			order = o
			hasOrdered = true
		}
		// 将任务按优先级分组
		groups[item.Priority] = append(groups[item.Priority], item)
	}

	// 创建阶段编排结果
	plan := &StagePlan{
		Stages:         make([]*taskTypes.Stage, 0, len(groups)+1),
		StageSeqOf:     make(map[string]int, len(items)),
		StaggerIndexOf: make(map[string]int, len(items)),
	}

	if len(immediate) > 0 {
		plan.appendStage(&taskTypes.Stage{
			Name:      immediateStageName,
			OnFailure: taskTypes.StageFailureContinue,
		}, immediate)
	}

	for i, prio := range sortedPriorities(groups, order) {
		plan.appendStage(&taskTypes.Stage{
			Name:         fmt.Sprintf("priority-%d", prio),
			OnFailure:    taskTypes.StageFailureBlock,
			BlockMessage: table.CascadeBlockMessage(prio, order),
			// 首个优先级阶段与托管类阶段同时起跑，保持托管操作不被优先级串行拖慢
			StartWithPrevious: i == 0 && len(immediate) > 0,
		}, groups[prio])
	}

	return plan
}

func (p *StagePlan) appendStage(stage *taskTypes.Stage, items []TaskItem) {
	stage.Seq = len(p.Stages)
	stage.Status = taskTypes.StageStatusNotStarted
	stage.Total = len(items)
	p.Stages = append(p.Stages, stage)

	// 阶段内按主机分组独立计数：同主机第 N 个任务延迟 N*interval，不同主机互不影响
	counters := make(map[string]int, len(items))
	for _, item := range items {
		p.StageSeqOf[item.TaskID] = stage.Seq
		p.StaggerIndexOf[item.TaskID] = counters[item.HostKey]
		counters[item.HostKey]++
	}
}

// sortedPriorities 按操作方向排序优先级取值
func sortedPriorities(groups map[int][]TaskItem, order table.PriorityOrder) []int {
	prios := make([]int, 0, len(groups))
	for prio := range groups {
		prios = append(prios, prio)
	}
	sort.Ints(prios)

	if order == table.PriorityOrderDesc {
		for i, j := 0, len(prios)-1; i < j; i, j = i+1, j-1 {
			prios[i], prios[j] = prios[j], prios[i]
		}
	}
	return prios
}
