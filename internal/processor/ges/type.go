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

// ProcessReport xxx
type ProcessReport struct {
	IP        string    `json:"ip"`
	BkAgentID string    `json:"bk_agent_id"`
	UtcTime   string    `json:"utctime"`
	UtcTime2  string    `json:"utctime2"`
	TimeZone  int       `json:"timezone"`
	Process   []Process `json:"process"`
}

// Process 进程信息
type Process struct {
	ProcName string            `json:"procname"`
	Instance []ProcessInstance `json:"instance"`
}

// ProcessInstance 进程实例
type ProcessInstance struct {
	CmdLine       string  `json:"cmdline"`
	ProcessName   string  `json:"processName"`
	Version       string  `json:"version"`
	Health        string  `json:"health"`
	IsAuto        bool    `json:"isAuto"`
	CPUUsage      float64 `json:"cpuUsage"`
	CPUUsageAve   float64 `json:"cpuUsageAve"`
	PhyMemUsage   float64 `json:"phyMemUsage"`
	UsePhyMem     float64 `json:"usePhyMem"`
	DiskSize      float64 `json:"diskSize"`
	PID           int     `json:"pid"`
	StartTime     string  `json:"startTime"`
	Stat          string  `json:"stat"`
	UTime         string  `json:"utime"`
	STime         string  `json:"stime"`
	ThreadCount   int     `json:"threadCount"`
	ElapsedTime   int64   `json:"elapsedTime"`
	RegisterTime  int64   `json:"register_time"`
	LastStartTime int64   `json:"last_start_time"`
	ReportTime    int64   `json:"report_time"`
}

// ProcResult xxx
type ProcResult struct {
	Value []struct {
		ProcName   string `json:"procName"`
		SetupPath  string `json:"setupPath"`
		FuncID     string `json:"funcID"`
		InstanceID string `json:"instanceID"`
		Result     string `json:"result"`
		IsAuto     bool   `json:"isAuto"`
	} `json:"value"`
}
