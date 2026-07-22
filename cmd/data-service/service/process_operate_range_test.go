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

package service

import (
	"testing"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	pbproc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// TestBuildOperateRangePluginRaw 插件路径：原样记录请求 expression_scope 五段，缺省段补 "*"（AC-001/AC-T01）。
func TestBuildOperateRangePluginRaw(t *testing.T) {
	req := &pbds.OperateProcessReq{
		OperateRange: &pbproc.OperateRange{
			Environment: "1",
			ExpressionScope: &pbproc.ExpressionScope{
				SetName:   "[管控平台,PaaS平台]",
				ProcessId: "4[6,8,9]",
				// module/service/alias 留空，期望回退为 "*"
			},
		},
	}

	got := buildOperateRange(nil, req)
	want := table.OperateRange{
		SetName:      "[管控平台,PaaS平台]",
		ModuleName:   "*",
		ServiceName:  "*",
		ProcessAlias: "*",
		ProcessID:    "4[6,8,9]",
	}
	if got != want {
		t.Fatalf("plugin buildOperateRange = %+v, want %+v", got, want)
	}
}

// TestBuildOperateRangeNonPlugin 非插件路径：命中进程 CC 进程 ID 拼压缩表达式记入 process_id，
// 其余段 "*"（AC-T03）。
func TestBuildOperateRangeNonPlugin(t *testing.T) {
	procs := []*table.Process{
		{Attachment: &table.ProcessAttachment{CcProcessID: 6}},
		{Attachment: &table.ProcessAttachment{CcProcessID: 7}},
		{Attachment: &table.ProcessAttachment{CcProcessID: 8}},
	}
	got := buildOperateRange(procs, &pbds.OperateProcessReq{})
	want := table.OperateRange{
		SetName:      "*",
		ModuleName:   "*",
		ServiceName:  "*",
		ProcessAlias: "*",
		ProcessID:    "[6-8]",
	}
	if got != want {
		t.Fatalf("non-plugin buildOperateRange = %+v, want %+v", got, want)
	}
}

// TestBuildConfigOperateRange 配置链路 buildOperateRange：插件模式原样存请求表达式；
// 非插件模式拼命中进程压缩表达式（AC-002/AC-T03）。
func TestBuildConfigOperateRange(t *testing.T) {
	s := &Service{}

	plugin := s.buildOperateRange(nil, true, &pbproc.OperateRange{
		ExpressionScope: &pbproc.ExpressionScope{
			ModuleName: "gse",
			ProcessId:  "[1-100]",
		},
	})
	wantPlugin := table.OperateRange{
		SetName:      "*",
		ModuleName:   "gse",
		ServiceName:  "*",
		ProcessAlias: "*",
		ProcessID:    "[1-100]",
	}
	if plugin != wantPlugin {
		t.Fatalf("config plugin buildOperateRange = %+v, want %+v", plugin, wantPlugin)
	}

	procs := []*table.Process{
		{Attachment: &table.ProcessAttachment{CcProcessID: 9}},
		{Attachment: &table.ProcessAttachment{CcProcessID: 8}},
		{Attachment: &table.ProcessAttachment{CcProcessID: 6}},
		{Attachment: &table.ProcessAttachment{CcProcessID: 6}},
	}
	nonPlugin := s.buildOperateRange(procs, false, nil)
	wantNonPlugin := table.OperateRange{
		SetName:      "*",
		ModuleName:   "*",
		ServiceName:  "*",
		ProcessAlias: "*",
		ProcessID:    "[6,8-9]",
	}
	if nonPlugin != wantNonPlugin {
		t.Fatalf("config non-plugin buildOperateRange = %+v, want %+v", nonPlugin, wantNonPlugin)
	}
}
