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
	"strings"
	"testing"

	"gorm.io/gen/field"
)

// renderScopeCond 离线渲染进程范围条件为 SQL 片段，用于断言条件的列与括号语义（不依赖数据库）。
func renderScopeCond(t *testing.T, processTemplateIDs, ccProcessIDs []uint32) string {
	t.Helper()

	cond := processScopeCond(field.NewUint32("processes", "process_template_id"),
		field.NewUint32("processes", "cc_process_id"), processTemplateIDs, ccProcessIDs)
	if cond == nil {
		return ""
	}

	var sb strings.Builder
	cond.Build(&sqlFragmentBuilder{sb: &sb})
	return sb.String()
}

// TestProcessScopeCondEmpty 两个参数都不传时应不追加任何过滤条件，保持进程管理页面等场景的原有行为。
func TestProcessScopeCondEmpty(t *testing.T) {
	if got := renderScopeCond(t, nil, nil); got != "" {
		t.Fatalf("两个参数均为空时不应生成条件，实际片段: %s", got)
	}
}

// TestProcessScopeCondUnion 两个参数同时传入时应取并集：OR 组整体带括号，
// 避免 biz_id / 环境等前置条件被并入 OR 右侧放大范围。
func TestProcessScopeCondUnion(t *testing.T) {
	got := renderScopeCond(t, []uint32{1, 2}, []uint32{3, 4})

	if !strings.HasPrefix(got, "(") || !strings.HasSuffix(got, ")") {
		t.Fatalf("模板进程与 CC 进程的 OR 组必须整体带括号，实际片段: %s", got)
	}
	if !strings.Contains(got, "process_template_id") || !strings.Contains(got, "cc_process_id") {
		t.Fatalf("并集条件应同时包含 process_template_id 与 cc_process_id，实际片段: %s", got)
	}
	if !strings.Contains(got, "OR") {
		t.Fatalf("两个参数同时传入时应为 OR 并集，实际片段: %s", got)
	}
}

// TestProcessScopeCondSingle 只传一个参数时按该列单独过滤，不引入另一列的恒假条件。
func TestProcessScopeCondSingle(t *testing.T) {
	tplOnly := renderScopeCond(t, []uint32{1}, nil)
	if !strings.Contains(tplOnly, "process_template_id") || strings.Contains(tplOnly, "cc_process_id") {
		t.Fatalf("仅传进程模板ID时应只按 process_template_id 过滤，实际片段: %s", tplOnly)
	}

	ccOnly := renderScopeCond(t, nil, []uint32{3})
	if !strings.Contains(ccOnly, "cc_process_id") || strings.Contains(ccOnly, "process_template_id") {
		t.Fatalf("仅传进程ID时应只按 cc_process_id 过滤，实际片段: %s", ccOnly)
	}
}
