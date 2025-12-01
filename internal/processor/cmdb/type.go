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

import "github.com/TencentBlueKing/bk-bscp/pkg/dal/table"

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
	Name    string
	IP      string
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
	CloudId int
	AgentID string
}

// Bizs 业务
type Bizs map[int][]Set
