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

package expression

import "strings"

// ExpressionScope 对齐 gsekit ExpresssionScopeSerializer 的五段表达式 + 环境类型。
// 五段与进程 expression 一一对应：集群名 / 模块名 / 服务实例名 / 进程别名 / CC 进程 ID。
type ExpressionScope struct {
	Environment  string // bk_set_env，必填
	SetName      string // 集群名称表达式，缺省 "*"
	ModuleName   string // 模块名称表达式，缺省 "*"
	ServiceName  string // 服务实例名称表达式，缺省 "*"
	ProcessAlias string // 进程别名表达式，缺省 "*"
	ProcessID    string // CC 进程 ID 表达式，支持切片，缺省 "*"
}

// Candidate 是候选进程在内存中的表达式载体（表达式串 + 其 CC 进程 ID）。
type Candidate struct {
	Expression  string
	CcProcessID uint32
}

const defaultSegment = "*"

// orDefault 空段回退为 "*"（对齐 gsekit ExpresssionScopeSerializer.validate）。
func orDefault(seg string) string {
	if seg == "" {
		return defaultSegment
	}
	return seg
}

// GenExpression 将五段拼接为一条完整表达式（对齐 gsekit serializers.gen_expression）。
func GenExpression(s ExpressionScope) string {
	return strings.Join([]string{
		orDefault(s.SetName),
		orDefault(s.ModuleName),
		orDefault(s.ServiceName),
		orDefault(s.ProcessAlias),
		orDefault(s.ProcessID),
	}, ExpressionSplitter)
}

// JoinProcessExpression 将进程的五个字段拼接为 expression 串（字段顺序与 GenExpression 一致）。
func JoinProcessExpression(setName, moduleName, serviceName, alias, ccProcessID string) string {
	return strings.Join([]string{setName, moduleName, serviceName, alias, ccProcessID}, ExpressionSplitter)
}

// ExpressionScopeToCcIDs 将表达式范围解析为命中的 CC 进程 ID 列表
// （对齐 gsekit ProcessHandler.expression_scope_to_scope 的 6 步流程）。
func ExpressionScopeToCcIDs(s ExpressionScope, candidates []Candidate) ([]uint32, error) {
	// 1. 建立 expression -> cc_process_id 映射，并保持候选顺序
	exprToID := make(map[string]uint32, len(candidates))
	orderedExprs := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if _, ok := exprToID[c.Expression]; !ok {
			orderedExprs = append(orderedExprs, c.Expression)
		}
		exprToID[c.Expression] = c.CcProcessID
	}

	// 2. 切片语法单独处理：命中切片则提取，并将进程 ID 段临时置为 "*"
	sliceExpression := defaultSegment
	if IsSliceExpression(orDefault(s.ProcessID)) {
		sliceExpression = s.ProcessID
		s.ProcessID = defaultSegment
	}

	// 3~5. 五段拼接 -> list_match -> 换回 CC 进程 ID
	matchedExprs, err := ListMatch(orderedExprs, GenExpression(s))
	if err != nil {
		return nil, err
	}
	ids := make([]uint32, 0, len(matchedExprs))
	for _, expr := range matchedExprs {
		ids = append(ids, exprToID[expr])
	}

	// 6. 对结果列表切片
	return ExecuteSlice(ids, sliceExpression)
}
