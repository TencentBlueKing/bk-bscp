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

// Package gse provides gse api client.
package gse

import (
	"context"
	"fmt"
)

const (
	asyncExtensionsTransferFile     = "%s/api/v2/task/extensions/async_transfer_file"
	asyncTerminateTransferFile      = "%s/api/v2/task/extensions/async_terminate_transfer_file"
	getExtensionsTransferFileResult = "%s/api/v2/task/extensions/get_transfer_file_result"
)

// TransferFileReq defines transfer file task request
type TransferFileReq struct {
	TimeOutSeconds int                `json:"timeout_seconds"` // 任务超时秒数，超时后任务会被强制终止，必须大于0
	AutoMkdir      bool               `json:"auto_mkdir"`      // 目录创建策略，true: 自动创建，false: 即使目录不存在也不自动创建
	UploadSpeed    int                `json:"upload_speed"`    // 文件上传速度限制(MB)
	DownloadSpeed  int                `json:"download_speed"`  // 文件下载速度限制(MB)，0:无限制
	Tasks          []TransferFileTask `json:"tasks"`           // 文件任务配置信息
}

// TransferFileTask defines transfer file task
type TransferFileTask struct {
	Source TransferFileSource `json:"source"`
	Target TransferFileTarget `json:"target"`
}

// TransferFileSource defines transfer file task source
type TransferFileSource struct {
	FileName string            `json:"file_name"`
	StoreDir string            `json:"store_dir"`
	Agent    TransferFileAgent `json:"agent"`
}

// TransferFileTarget defines transfer file task target
type TransferFileTarget struct {
	FileName string              `json:"file_name"`
	StoreDir string              `json:"store_dir"`
	Agents   []TransferFileAgent `json:"agents"`
}

// TransferFileAgent defines transfer file task agent
type TransferFileAgent struct {
	User          string `json:"user"`
	BkAgentID     string `json:"bk_agent_id"`
	BkContainerID string `json:"bk_container_id"`
}

// CommonTaskRespData defines gse common task response data
type CommonTaskResp struct {
	Code    int                `json:"code"`
	Message string             `json:"message"`
	Data    CommonTaskRespData `json:"data"`
}
type CommonTaskRespData struct {
	Result CommonTaskRespResult `json:"result"`
}

// CommonTaskRespResult defines gse common task response result
type CommonTaskRespResult struct {
	TaskID string `json:"task_id"`
}

// TerminateTransferFileTaskReq defines terminate transfer file task request
type TerminateTransferFileTaskReq struct {
	Agents []TransferFileAgent `json:"agents"`
	TaskID string              `json:"task_id"`
}

// TransferFileResultData defines transfer file task result data
type TransferFileResultData struct {
	Version string                         `json:"version"`
	Result  []TransferFileResultDataResult `json:"result"`
}

// TransferFileResultDataResult defines transfer file task result data result
type TransferFileResultDataResult struct {
	Content   TransferFileResultDataResultContent `json:"content"`
	ErrorCode int                                 `json:"error_code"`
	ErrorMsg  string                              `json:"error_msg"`
}

// TransferFileResultDataResultContent defines transfer file task result data result content
type TransferFileResultDataResultContent struct {
	DestAgentID       string `json:"dest_agent_id"`
	DestContainerID   string `json:"dest_container_id"`
	DestFileDir       string `json:"dest_file_dir"`
	DestFileName      string `json:"dest_file_name"`
	Mode              int    `json:"mode"`
	Progress          int    `json:"progress"`
	SourceAgentID     string `json:"source_agent_id"`
	SourceContainerID string `json:"source_container_id"`
	SourceFileDir     string `json:"source_file_dir"`
	SourceFileName    string `json:"source_file_name"`
	Speed             int    `json:"speed"`
	Status            int    `json:"status"`
	StatusInfo        string `json:"status_info"`
	Type              string `json:"type"`
	StartTime         int64  `json:"start_time"`
	EndTime           int64  `json:"end_time"`
	Size              int64  `json:"size"`
}

// AsyncExtensionsTransferFile 启动文件分发任务, 可支持扩展型目标, 包括容器和主机
func (gse *Service) AsyncExtensionsTransferFile(ctx context.Context, req *TransferFileReq) (*CommonTaskRespData, error) {
	// 1. if sourceContainerID is set, means source is container, else is node
	// 2. if targetContainerID is set, means target is container, else is node
	url := fmt.Sprintf(asyncExtensionsTransferFile, gse.host)

	resp := new(CommonTaskResp)
	if err := gse.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return &resp.Data, nil
}

// AsyncTerminateTransferFile 终止文件分发任务, 可支持扩展型目标, 包括容器和主机
func (gse *Service) AsyncTerminateTransferFile(ctx context.Context, req *TerminateTransferFileTaskReq) (*CommonTaskRespData, error) {
	url := fmt.Sprintf(asyncTerminateTransferFile, gse.host)

	resp := new(CommonTaskRespData)
	if err := gse.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// GetExtensionsTransferFileResult 查询文件传输结果, 可支持扩展型目标, 包括容器和主机
func (gse *Service) GetExtensionsTransferFileResult(ctx context.Context, req *GetTransferFileResultReq) (*TransferFileResultData, error) {
	url := fmt.Sprintf(getExtensionsTransferFileResult, gse.host)

	resp := new(TransferFileResultData)
	if err := gse.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// GetTransferFileResultReq defines get transfer file result request
type GetTransferFileResultReq struct {
	TaskID string      `json:"task_id"` // 启动任务时返回的任务 ID
	Agents []AgentList `json:"agents"`  // 目标节点 Agent ID 列表, 单 ID 最大长度不超过64个字符
}

type AgentList struct {
	BkAgentID     string `json:"bk_agent_id"`     // 目标 Agent ID，最大长度不超过64个字符
	BkContainerID string `json:"bk_container_id"` // 目标容器 ID, 空则为主机
}
