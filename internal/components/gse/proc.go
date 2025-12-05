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
	operateProcMulti       = "%s/api/bk-gse/prod/api/v2/proc/operate_proc_multi"
	updateProcInfo         = "%s/api/v2/proc/update_proc_info"
	getTaskState           = "%s/api/v2/task/get_task_state"
	getProcOperateResultV2 = "%s/api/bk-gse/prod/api/v2/proc/get_proc_operate_result_v2"
	getProcStatusV2        = "%s/api/bk-gse/prod/api/v2/proc/get_proc_status_v2"
	syncProcStatus         = "%s/api/bk-gse/prod/api/v2/proc/sync_proc_status"
	operateProcV2          = "%s/api/bk-gse/prod/api/v2/proc/operate_proc_v2"
)

// OperateProcMulti 批量进程操作
func (gse *Service) OperateProcMulti(ctx context.Context, req *MultiProcOperateReq) (*ProcOperationData, error) {
	url := fmt.Sprintf(operateProcMulti, gse.host)

	resp := new(GESResponse)
	if err := gse.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	var multiProcOperateResp ProcOperationData
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

	UpdateProcInfoResp := make(map[string]ProcResult, 0)
	if err := resp.Decode(&UpdateProcInfoResp); err != nil {
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

// GetProcOperateResultV2 进程操作
// 查询进程操作结果
func (gse *Service) GetProcOperateResultV2(ctx context.Context, req *QueryProcResultReq) (*GESResponse, error) {
	url := fmt.Sprintf(getProcOperateResultV2, gse.host)

	resp := new(GESResponse)
	if err := gse.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	result := make(map[string]ProcResult, 0)
	if err := resp.Decode(&result); err != nil {
		return nil, err
	}

	return resp, nil
}

// GetProcStatusV2 查询进程状态信息
func (gse *Service) GetProcStatusV2(ctx context.Context, req *QueryProcStatusReq) (*GESResponse, error) {
	url := fmt.Sprintf(getProcStatusV2, gse.host)

	resp := new(GESResponse)
	if err := gse.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	var result ProcStatusData
	if err := resp.Decode(&result); err != nil {
		return nil, err
	}

	return resp, nil
}

// SyncProcStatus  同步查询进程状态信息
func (gse *Service) SyncProcStatus(ctx context.Context, req *SyncQueryProcStatusReq) (*GESResponse, error) {
	url := fmt.Sprintf(syncProcStatus, gse.host)

	resp := new(GESResponse)
	if err := gse.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	var result SyncProcStatusData
	if err := resp.Decode(&result); err != nil {
		return nil, err
	}

	return resp, nil
}

// OperateProcV2  进程操作
func (gse *Service) OperateProcV2(ctx context.Context, req *ProcOperationReq) (*GESResponse, error) {
	url := fmt.Sprintf(operateProcV2, gse.host)

	resp := new(GESResponse)
	if err := gse.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	var result ProcOperationData
	if err := resp.Decode(&result); err != nil {
		return nil, err
	}

	return resp, nil
}
