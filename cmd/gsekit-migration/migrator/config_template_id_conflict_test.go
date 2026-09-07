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
	"strings"
	"testing"
)

func TestConfigTemplateIDConflictError_NoConflict(t *testing.T) {
	if err := configTemplateIDConflictError(nil); err != nil {
		t.Fatalf("nil conflicts: got %v, want nil", err)
	}
	if err := configTemplateIDConflictError([]existingConfigTemplate{}); err != nil {
		t.Fatalf("empty conflicts: got %v, want nil", err)
	}
}

func TestConfigTemplateIDConflictError_ListsRowsAndHint(t *testing.T) {
	err := configTemplateIDConflictError([]existingConfigTemplate{
		{ID: 100, BizID: 2, Name: "nginx.conf", Creator: "alice", TenantID: "default"},
		{ID: 200, BizID: 3, Name: "app.yaml", Creator: "bob", TenantID: "default"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	msg := err.Error()
	for _, want := range []string{
		"2 GSEKit id(s) already exist",
		`id=100 biz_id=2 name="nginx.conf" creator="alice" tenant_id="default"`,
		`id=200 biz_id=3 name="app.yaml" creator="bob" tenant_id="default"`,
		"run align-template-id",
		"cleanup leftover data",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error missing %q\nfull message:\n%s", want, msg)
		}
	}
}

func TestConfigTemplateIDConflictError_TruncatesDetails(t *testing.T) {
	conflicts := make([]existingConfigTemplate, maxConfigTemplateIDConflictDetails+5)
	for i := range conflicts {
		conflicts[i] = existingConfigTemplate{ID: uint32(i + 1), BizID: 1, Name: "t"}
	}

	err := configTemplateIDConflictError(conflicts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "25 GSEKit id(s) already exist") {
		t.Errorf("missing total count in:\n%s", msg)
	}
	if !strings.Contains(msg, "... and 5 more") {
		t.Errorf("missing truncation marker in:\n%s", msg)
	}
	if got := strings.Count(msg, "\n  id="); got != maxConfigTemplateIDConflictDetails {
		t.Errorf("listed %d ids, want %d", got, maxConfigTemplateIDConflictDetails)
	}
}
