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
	listAgentState = "%s/api/v2/cluster/list_agent_state"
)

// ListAgentState 查询Agent状态列表信息
func (gse *Service) ListAgentState(ctx context.Context, req *ListAgentStateReq) ([]*ListAgentStateData, error) {
	url := fmt.Sprintf(listAgentState, gse.host)

	resp := new(GESResponse)
	if err := gse.doRequest(ctx, POST, url, req, resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("gse error, code=%d, msg=%s", resp.Code, resp.Message)
	}

	var data []*ListAgentStateData
	if err := resp.Decode(&data); err != nil {
		return nil, err
	}

	return data, nil
}
