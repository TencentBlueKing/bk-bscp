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

package migrations

import (
	"encoding/json"
	"testing"
)

func operateRangeOf(t *testing.T, taskData string) newOperateRange {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(taskData), &top); err != nil {
		t.Fatalf("unmarshal task_data: %v", err)
	}
	var nr newOperateRange
	if err := json.Unmarshal(top["operate_range"], &nr); err != nil {
		t.Fatalf("unmarshal operate_range: %v", err)
	}
	return nr
}

// TestConvertOperateRangeJSON 旧数组 -> 表达式字符串无损转换，且保留其余字段（AC-005/AC-T02）。
func TestConvertOperateRangeJSON(t *testing.T) {
	old := `{"environment":"1","operate_range":{"set_names":["管控平台","PaaS平台"],` +
		`"module_names":[],"service_names":[],"process_alias":[],"cc_process_ids":[6,7,8]},` +
		`"config_template_ids":[100]}`

	got, changed, err := convertOperateRangeJSON(old)
	if err != nil || !changed {
		t.Fatalf("convert failed: changed=%v err=%v", changed, err)
	}

	nr := operateRangeOf(t, got)
	if nr.SetName != "[PaaS平台,管控平台]" && nr.SetName != "[管控平台,PaaS平台]" {
		t.Fatalf("set_name = %q", nr.SetName)
	}
	if nr.ModuleName != "*" || nr.ServiceName != "*" || nr.ProcessAlias != "*" {
		t.Fatalf("empty segments not '*': %+v", nr)
	}
	if nr.ProcessID != "[6-8]" {
		t.Fatalf("process_id = %q, want [6-8]", nr.ProcessID)
	}

	// 保留其余字段
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(got), &top); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(top["environment"]) != `"1"` {
		t.Fatalf("environment lost: %s", top["environment"])
	}
	if string(top["config_template_ids"]) != `[100]` {
		t.Fatalf("config_template_ids lost: %s", top["config_template_ids"])
	}
}

// TestConvertOperateRangeJSONIdempotent 已迁移记录再次运行不变更（幂等，AC-T02）。
func TestConvertOperateRangeJSONIdempotent(t *testing.T) {
	old := `{"environment":"1","operate_range":{"set_names":[],"module_names":[],` +
		`"service_names":[],"process_alias":[],"cc_process_ids":[]}}`

	first, changed, err := convertOperateRangeJSON(old)
	if err != nil || !changed {
		t.Fatalf("first convert: changed=%v err=%v", changed, err)
	}
	// 全空数组 -> 五段均为 "*"
	nr := operateRangeOf(t, first)
	if nr.SetName != "*" || nr.ModuleName != "*" || nr.ServiceName != "*" ||
		nr.ProcessAlias != "*" || nr.ProcessID != "*" {
		t.Fatalf("all-empty should be all '*': %+v", nr)
	}

	second, changed2, err := convertOperateRangeJSON(first)
	if err != nil {
		t.Fatalf("second convert err: %v", err)
	}
	if changed2 {
		t.Fatalf("second convert should be no-op (idempotent), but changed")
	}
	if second != first {
		t.Fatalf("idempotent output differs:\n first=%s\n second=%s", first, second)
	}
}
