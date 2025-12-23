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
	asyncExecuteScript     = "%s/api/v2/task/extensions/async_execute_script"
	getExecuteScriptResult = "%s/api/v2/task/extensions/get_execute_script_result"
)

// AsyncExtensionsExecuteScript 异步脚本执行, 可支持扩展型目标, 包括容器和主机
func (gse *Service) AsyncExtensionsExecuteScript(ctx context.Context, req *ExecuteScriptReq) (*CommonTaskRespData, error) {

	url := fmt.Sprintf(asyncExecuteScript, gse.host)

	resp := new(CommonTaskResp)
	if err := gse.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return &resp.Data, nil
}

// GetExecuteScriptResult 获取脚本执行结果
func (gse *Service) GetExecuteScriptResult(ctx context.Context, req *GetExecuteScriptResultReq) (*ExecuteScriptResult, error) {
	url := fmt.Sprintf(getExecuteScriptResult, gse.host)

	resp := new(GetExecuteScriptResultResp)
	if err := gse.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	return resp.Data, nil
}
