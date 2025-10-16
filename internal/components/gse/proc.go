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

var (
	operateProcMulti           = "%s/api/v2/proc/operate_proc_multi"
	updateProcInfo             = "%s/api/v2/proc/update_proc_info"
	asyncTransferFile          = "%s/api/v2/task/async_transfer_file"
	asyncTerminateTransferFile = "%s/api/v2/task/async_terminate_transfer_file"
	getTaskState               = "%s/api/v2/task/get_task_state"
)

// OperateProcMulti 批量进程操作
func (gse *Service) OperateProcMulti(ctx context.Context, req *MultiProcOperateReq) (*MultiProcOperateResp, error) {
	url := fmt.Sprintf(operateProcMulti, gse.host)

	resp := new(GESResponse)
	if err := gse.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	var multiProcOperateResp MultiProcOperateResp
	if err := resp.Decode(&multiProcOperateResp); err != nil {
		return nil, err
	}

	return &multiProcOperateResp, nil
}

// UpdateProcInfo 更新进程信息
func (gse *Service) UpdateProcInfo(ctx context.Context, req *UpdateProcInfoReq) (*GESResponse, error) {
	url := fmt.Sprintf(updateProcInfo, gse.host)

	resp := new(GESResponse)
	if err := gse.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	UpdateProcInfoResp := make(map[string]ProcTestItem, 0)
	if err := resp.Decode(&UpdateProcInfoResp); err != nil {
		return nil, err
	}

	return resp, nil
}

// AsyncTransferFile 文件传输
func (gse *Service) AsyncTransferFile(ctx context.Context, req *FileTaskReq) (*GESResponse, error) {
	url := fmt.Sprintf(asyncTransferFile, gse.host)

	resp := new(GESResponse)
	if err := gse.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	var taskResp TaskResp
	if err := resp.Decode(&taskResp); err != nil {
		return nil, err
	}

	return resp, nil
}

// AsyncTerminateTransferFile 终止文件任务执行
func (gse *Service) AsyncTerminateTransferFile(ctx context.Context, req *TaskReq) (*GESResponse, error) {
	url := fmt.Sprintf(asyncTerminateTransferFile, gse.host)

	resp := new(GESResponse)
	if err := gse.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	var taskResp TaskResp
	if err := resp.Decode(&taskResp); err != nil {
		return nil, err
	}

	return resp, nil
}

// GetTaskState 查询任务状态
func (gse *Service) GetTaskState(ctx context.Context, req *TaskReq) (*TaskOperateResult, error) {
	url := fmt.Sprintf(getTaskState, gse.host)

	resp := new(GESResponse)
	if err := gse.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	var taskResp TaskOperateResult
	if err := resp.Decode(&taskResp); err != nil {
		return nil, err
	}

	return &taskResp, nil
}
