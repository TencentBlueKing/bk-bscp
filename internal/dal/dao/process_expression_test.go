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
	"fmt"
	"strings"
	"testing"

	"gorm.io/gen/field"
	"gorm.io/gorm/clause"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	pbproc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
)

// sqlFragmentBuilder 是最小化的 clause.Builder，仅用于离线渲染单个条件表达式为 SQL 片段，
// 不依赖数据库连接。
type sqlFragmentBuilder struct {
	sb *strings.Builder
}

func (b *sqlFragmentBuilder) WriteByte(c byte) error               { return b.sb.WriteByte(c) }
func (b *sqlFragmentBuilder) WriteString(s string) (int, error)    { return b.sb.WriteString(s) }
func (b *sqlFragmentBuilder) AddVar(clause.Writer, ...interface{}) {}
func (b *sqlFragmentBuilder) AddError(error) error                 { return nil }
func (b *sqlFragmentBuilder) WriteQuoted(field interface{}) {
	if col, ok := field.(clause.Column); ok {
		b.sb.WriteString(col.Name)
		return
	}
	fmt.Fprintf(b.sb, "%v", field)
}

func newProc(ccID uint32, set, module, service, alias string) *table.Process {
	return &table.Process{
		Attachment: &table.ProcessAttachment{CcProcessID: ccID},
		Spec:       &table.ProcessSpec{SetName: set, ModuleName: module, ServiceName: service, Alias: alias},
	}
}

func matchedIDs(t *testing.T, procs []*table.Process, es *pbproc.ExpressionScope) []uint32 {
	t.Helper()
	got, err := filterProcessesByExpressionScope(procs, es)
	if err != nil {
		t.Fatalf("filterProcessesByExpressionScope error: %v", err)
	}
	ids := make([]uint32, 0, len(got))
	for _, p := range got {
		ids = append(ids, p.Attachment.CcProcessID)
	}
	return ids
}

func equalIDs(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFilterProcessesByExpressionScope(t *testing.T) {
	procs := []*table.Process{
		newProc(1, "set", "module", "svc", "p1"),
		newProc(2, "set", "module", "svc", "p2"),
		newProc(3, "set", "module", "svc", "p10"),
		newProc(4, "other", "module", "svc", "p1"),
	}

	cases := []struct {
		name string
		es   *pbproc.ExpressionScope
		want []uint32
	}{
		{
			name: "空表达式段回退为通配，命中全部",
			es:   &pbproc.ExpressionScope{},
			want: []uint32{1, 2, 3, 4},
		},
		{
			name: "别名通配 p*",
			es:   &pbproc.ExpressionScope{ProcessAlias: "p*"},
			want: []uint32{1, 2, 3, 4},
		},
		{
			name: "集群等值 set 过滤 other",
			es:   &pbproc.ExpressionScope{SetName: "set"},
			want: []uint32{1, 2, 3},
		},
		{
			name: "别名枚举 [p1,p2]",
			es:   &pbproc.ExpressionScope{SetName: "set", ProcessAlias: "[p1,p2]"},
			want: []uint32{1, 2},
		},
		{
			name: "进程ID数字范围 [1-2]",
			es:   &pbproc.ExpressionScope{ProcessId: "[1-2]"},
			want: []uint32{1, 2},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := matchedIDs(t, procs, tc.es)
			if !equalIDs(got, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, got)
			}
		})
	}
}

// TestExpressionScopeEmptyMatchYieldsEmptySet 锁定页面路径空命中语义：
// 表达式解析后命中为空时，handleSearch 用 m.CcProcessID.In(空) 追加条件，
// 必须渲染为 `cc_process_id IN (NULL)`（匹配不到任何进程 = 空结果集），
// 不能被降级为丢弃条件从而变成全选（对齐需求 R-003 / AC-003）。
// 用 gorm DryRun 生成 SQL 断言，不连接真实数据库。
func TestExpressionScopeEmptyMatchYieldsEmptySet(t *testing.T) {
	// 与 gen 生成的 processes.gen.go 中 CcProcessID 字段定义一致。
	ccProcessID := field.NewUint32("processes", "cc_process_id")
	var emptyIDs []uint32

	var sb strings.Builder
	ccProcessID.In(emptyIDs...).Build(&sqlFragmentBuilder{sb: &sb})
	got := sb.String()

	if !strings.Contains(got, "IN (NULL)") || !strings.Contains(got, "cc_process_id") {
		t.Fatalf("空命中应渲染为 cc_process_id IN (NULL)（匹配空集），实际片段: %s", got)
	}
}

// TestFilterProcessesByExpressionScopeSlice 验证切片语义作用于匹配结果列表，保持匹配顺序的子序列。
func TestFilterProcessesByExpressionScopeSlice(t *testing.T) {
	procs := []*table.Process{
		newProc(11, "set", "module", "svc", "a"),
		newProc(12, "set", "module", "svc", "b"),
		newProc(13, "set", "module", "svc", "c"),
	}
	got := matchedIDs(t, procs, &pbproc.ExpressionScope{ProcessId: "[0:2]"})
	if !equalIDs(got, []uint32{11, 12}) {
		t.Fatalf("slice [0:2] want [11 12], got %v", got)
	}
}

// TestFilterProcessesByExpressionScopeSliceOrdering 锁定切片顺序对齐 gsekit：
// 候选进程无论传入顺序如何，都按 CC 进程 ID 升序参与匹配，切片取升序列表的子序列。
// 这保证从 gsekit 迁移的数据（bscp 自增 ID 序与 CC 进程 ID 序不一致）下，
// 同一切片表达式在 bscp 与 gsekit 命中相同进程。
func TestFilterProcessesByExpressionScopeSliceOrdering(t *testing.T) {
	// 故意打乱传入顺序（CC 进程 ID 非升序），模拟 bscp 自增 ID 序与 CC 进程 ID 序不一致。
	procs := []*table.Process{
		newProc(13, "set", "module", "svc", "c"),
		newProc(11, "set", "module", "svc", "a"),
		newProc(12, "set", "module", "svc", "b"),
	}

	cases := []struct {
		name string
		expr string
		want []uint32
	}{
		{name: "切片 [0:2] 取升序前两个", expr: "[0:2]", want: []uint32{11, 12}},
		{name: "切片 [-1:] 取升序最后一个", expr: "[-1:]", want: []uint32{13}},
		{name: "切片 [1:] 从升序第二个到末尾", expr: "[1:]", want: []uint32{12, 13}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := matchedIDs(t, procs, &pbproc.ExpressionScope{ProcessId: tc.expr})
			if !equalIDs(got, tc.want) {
				t.Fatalf("slice %s want %v, got %v", tc.expr, tc.want, got)
			}
		})
	}
}
