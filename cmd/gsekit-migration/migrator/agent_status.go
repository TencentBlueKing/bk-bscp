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

package migrator

import (
	"context"

	"github.com/TencentBlueKing/bk-bscp/cmd/gsekit-migration/config"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// gseAgentRunningCode 是 GSE list_agent_state 返回的“运行中”状态码。
// 仅该状态映射为 agent_status=normal，与运行时 buildProcessEntities 保持一致。
const gseAgentRunningCode = 2

// AgentStateClient queries GSE agent states for a set of agent IDs.
// It is abstracted as an interface so the aggregation/mapping logic can be
// tested without issuing real HTTP requests.
type AgentStateClient interface {
	// ListAgentStatus returns a map of agent_id → status_code for the given agent IDs.
	ListAgentStatus(ctx context.Context, agentIDs []string) (map[string]int, error)
}

// resolveAgentStatus maps a single agent's GSE status to the BSCP agent_status value.
//
// 查询成功时：规则与运行时 buildProcessEntities 一致——仅 status_code==2（运行中）
// 为 normal，其余状态码 / 无 agent_id / 未命中查询结果 均为 abnormal。
//
// queryFailed 为 true（GSE 查询整体失败）时：兜底为 normal
func resolveAgentStatus(agentID string, statusMap map[string]int, queryFailed bool) string {
	if queryFailed {
		return string(table.AgentStatusNormal)
	}
	if agentID == "" {
		return string(table.AgentStatusAbnormal)
	}
	if code, ok := statusMap[agentID]; ok && code == gseAgentRunningCode {
		return string(table.AgentStatusNormal)
	}
	return string(table.AgentStatusAbnormal)
}

// collectAgentIDs returns the unique, non-empty bk_agent_id values of the given processes.
func collectAgentIDs(processes []GSEKitProcess) []string {
	seen := make(map[string]bool, len(processes))
	result := make([]string, 0, len(processes))
	for _, p := range processes {
		if p.BkAgentID == "" || seen[p.BkAgentID] {
			continue
		}
		seen[p.BkAgentID] = true
		result = append(result, p.BkAgentID)
	}
	return result
}

// gseAgentStateClient adapts internal/components/gse.Service to AgentStateClient.
type gseAgentStateClient struct {
	svc      *gse.Service
	tenantID string
}

// NewAgentStateClient creates an AgentStateClient backed by the GSE API gateway.
func NewAgentStateClient(cfg *config.GSEConfig, tenantID string) AgentStateClient {
	return &gseAgentStateClient{
		svc:      gse.NewService(cfg.AppCode, cfg.AppSecret, cfg.Endpoint),
		tenantID: tenantID,
	}
}

// ListAgentStatus queries GSE and returns an agent_id → status_code map.
func (c *gseAgentStateClient) ListAgentStatus(_ context.Context, agentIDs []string) (map[string]int, error) {
	if len(agentIDs) == 0 {
		return map[string]int{}, nil
	}

	// GSE 客户端从 ctx 携带的 kit 中读取租户信息，这里按迁移目标租户构造上下文。
	kt := kit.NewWithTenant(c.tenantID)
	data, err := c.svc.ListAgentState(kt.Ctx, &gse.ListAgentStateReq{AgentIDList: agentIDs})
	if err != nil {
		return nil, err
	}

	statusMap := make(map[string]int, len(data))
	for _, d := range data {
		if d.BkAgentID == "" {
			continue
		}
		statusMap[d.BkAgentID] = d.StatusCode
	}
	return statusMap, nil
}
