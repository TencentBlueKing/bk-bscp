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

package dao

import (
	"strconv"

	rawgen "gorm.io/gen"

	"github.com/TencentBlueKing/bk-bscp/internal/expression"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbproc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
)

// loadExpressionCandidates 按业务 + 环境（排除已删除）加载表达式匹配的候选进程集合。
// 环境为空时不限定环境；候选集是内存表达式匹配的输入。
func (dao *processDao) loadExpressionCandidates(kit *kit.Kit, bizID uint32,
	environment string) ([]*table.Process, error) {
	m := dao.genQ.Process
	q := dao.genQ.Process.WithContext(kit.Ctx)

	conds := []rawgen.Condition{m.BizID.Eq(bizID), m.CcSyncStatus.Neq(table.Deleted.String())}
	if environment != "" {
		conds = append(conds, m.Environment.Eq(environment))
	}
	return q.Where(conds...).Find()
}

// matchedCcIDsByExpressionScope 加载候选进程并按表达式范围过滤，返回命中的 CC 进程 ID 列表。
// 命中为空时返回空切片（调用方据此得到空结果集，不降级为全选）。
func (dao *processDao) matchedCcIDsByExpressionScope(kit *kit.Kit, bizID uint32, environment string,
	es *pbproc.ExpressionScope) ([]uint32, error) {
	candidates, err := dao.loadExpressionCandidates(kit, bizID, environment)
	if err != nil {
		return nil, err
	}
	matched, err := filterProcessesByExpressionScope(candidates, es)
	if err != nil {
		return nil, err
	}
	ids := make([]uint32, 0, len(matched))
	for _, p := range matched {
		ids = append(ids, p.Attachment.CcProcessID)
	}
	return ids, nil
}

// filterProcessesByExpressionScope 在内存中按表达式范围过滤候选进程，语义对齐 gsekit
// expression_scope_to_scope：将进程五段字段拼成 expression 后做表达式匹配，返回命中的进程。
// 结果顺序与切片语义保持匹配后的顺序（切片 `[a:b]` 取的是匹配列表的子序列）。
func filterProcessesByExpressionScope(processes []*table.Process,
	es *pbproc.ExpressionScope) ([]*table.Process, error) {

	candidates := make([]expression.Candidate, 0, len(processes))
	idToProc := make(map[uint32]*table.Process, len(processes))
	for _, p := range processes {
		ccID := p.Attachment.CcProcessID
		candidates = append(candidates, expression.Candidate{
			Expression: expression.JoinProcessExpression(
				p.Spec.SetName, p.Spec.ModuleName, p.Spec.ServiceName, p.Spec.Alias,
				strconv.FormatUint(uint64(ccID), 10),
			),
			CcProcessID: ccID,
		})
		if _, ok := idToProc[ccID]; !ok {
			idToProc[ccID] = p
		}
	}

	scope := expression.ExpressionScope{
		SetName:      es.GetSetName(),
		ModuleName:   es.GetModuleName(),
		ServiceName:  es.GetServiceName(),
		ProcessAlias: es.GetProcessAlias(),
		ProcessID:    es.GetProcessId(),
	}

	matchedIDs, err := expression.ExpressionScopeToCcIDs(scope, candidates)
	if err != nil {
		// 非法表达式属于用户入参错误，归类为 InvalidParameter，避免被上层误报为 DB 操作失败。
		return nil, errf.Errorf(errf.InvalidParameter, "invalid expression scope: %v", err)
	}

	result := make([]*table.Process, 0, len(matchedIDs))
	for _, id := range matchedIDs {
		if p, ok := idToProc[id]; ok {
			result = append(result, p)
		}
	}
	return result, nil
}
