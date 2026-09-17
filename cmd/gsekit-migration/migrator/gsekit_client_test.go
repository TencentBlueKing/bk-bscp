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
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/TencentBlueKing/bk-bscp/cmd/gsekit-migration/config"
)

// testEnv holds environment-based test configuration.
type testEnv struct {
	Endpoint  string
	AppCode   string
	AppSecret string
	BkTicket  string
	BizID     uint32
	ProcessID int64
}

// loadTestEnv reads test configuration from environment variables.
// Skips the test if required vars are not set.
//
// Required env vars:
//
//	GSEKIT_ENDPOINT   - e.g. https://bk-gsekit.apigw.o.woa.com/prod
//	GSEKIT_APP_CODE   - e.g. bk-bscp
//	GSEKIT_APP_SECRET
//	GSEKIT_BK_TICKET
//	GSEKIT_BIZ_ID     - e.g. 100148
//	GSEKIT_PROCESS_ID - e.g. 22445554
func loadTestEnv(t *testing.T) *testEnv {
	t.Helper()

	endpoint := os.Getenv("GSEKIT_ENDPOINT")
	appCode := os.Getenv("GSEKIT_APP_CODE")
	appSecret := os.Getenv("GSEKIT_APP_SECRET")
	bkTicket := os.Getenv("GSEKIT_BK_TICKET")

	if endpoint == "" || appCode == "" || appSecret == "" || bkTicket == "" {
		t.Skip("skipping: set GSEKIT_ENDPOINT, GSEKIT_APP_CODE, GSEKIT_APP_SECRET, GSEKIT_BK_TICKET to run")
	}

	env := &testEnv{
		Endpoint:  endpoint,
		AppCode:   appCode,
		AppSecret: appSecret,
		BkTicket:  bkTicket,
		BizID:     100148,
		ProcessID: 22445554,
	}

	if s := os.Getenv("GSEKIT_BIZ_ID"); s != "" {
		n, err := json.Number(s).Int64()
		if err != nil {
			t.Fatalf("invalid GSEKIT_BIZ_ID: %s", s)
		}
		env.BizID = uint32(n)
	}

	if s := os.Getenv("GSEKIT_PROCESS_ID"); s != "" {
		n, err := json.Number(s).Int64()
		if err != nil {
			t.Fatalf("invalid GSEKIT_PROCESS_ID: %s", s)
		}
		env.ProcessID = n
	}

	return env
}

func (e *testEnv) newClient() GSEKitClient {
	return NewGSEKitClient(&config.GSEKitConfig{
		Endpoint:  e.Endpoint,
		AppCode:   e.AppCode,
		AppSecret: e.AppSecret,
		BkTicket:  e.BkTicket,
	})
}

// TestPreviewConfigTemplate_Success verifies a basic preview request returns non-empty rendered content.
func TestPreviewConfigTemplate_Success(t *testing.T) {
	env := loadTestEnv(t)
	client := env.newClient()

	content := "${InstID}\n${LocalInstID}\n${FuncID}\n${bk_module_name}\n${bk_set_name}"
	result, err := client.PreviewConfigTemplate(context.Background(), env.BizID, content, env.ProcessID)
	if err != nil {
		t.Fatalf("PreviewConfigTemplate failed: %v", err)
	}

	t.Logf("rendered result:\n%s", result)

	if result == "" {
		t.Error("expected non-empty rendered result")
	}

	lines := strings.Split(result, "\n")
	if len(lines) < 5 {
		t.Errorf("expected at least 5 lines, got %d", len(lines))
	}
}

// TestPreviewConfigTemplate_PlainText verifies that plain text without variables is returned as-is.
func TestPreviewConfigTemplate_PlainText(t *testing.T) {
	env := loadTestEnv(t)
	client := env.newClient()

	content := "hello world, no variables here"
	result, err := client.PreviewConfigTemplate(context.Background(), env.BizID, content, env.ProcessID)
	if err != nil {
		t.Fatalf("PreviewConfigTemplate failed: %v", err)
	}

	t.Logf("rendered result: %q", result)

	if !strings.Contains(result, "hello world, no variables here") {
		t.Errorf("plain text should be returned as-is, got %q", result)
	}
}

// TestPreviewConfigTemplate_EmptyContent verifies that empty template content is handled.
func TestPreviewConfigTemplate_EmptyContent(t *testing.T) {
	env := loadTestEnv(t)
	client := env.newClient()

	result, err := client.PreviewConfigTemplate(context.Background(), env.BizID, "", env.ProcessID)
	if err != nil {
		t.Fatalf("PreviewConfigTemplate failed: %v", err)
	}

	t.Logf("rendered result for empty content: %q", result)
}

// TestPreviewConfigTemplate_InvalidProcessID verifies that an invalid process ID returns an error.
func TestPreviewConfigTemplate_InvalidProcessID(t *testing.T) {
	env := loadTestEnv(t)
	client := env.newClient()

	_, err := client.PreviewConfigTemplate(context.Background(), env.BizID, "${InstID}", 999999999)
	if err == nil {
		t.Error("expected error for invalid process ID, got nil")
	} else {
		t.Logf("expected error: %v", err)
	}
}

// TestPreviewConfigTemplate_InvalidBizID verifies that an invalid biz ID returns an error.
func TestPreviewConfigTemplate_InvalidBizID(t *testing.T) {
	env := loadTestEnv(t)
	client := env.newClient()

	_, err := client.PreviewConfigTemplate(context.Background(), 0, "${InstID}", env.ProcessID)
	if err == nil {
		t.Error("expected error for invalid biz ID, got nil")
	} else {
		t.Logf("expected error: %v", err)
	}
}

// TestPreviewConfigTemplate_MultipleVariables verifies that all common template variables are rendered.
func TestPreviewConfigTemplate_MultipleVariables(t *testing.T) {
	env := loadTestEnv(t)
	client := env.newClient()

	content := strings.Join([]string{
		"InstID=${InstID}",
		"LocalInstID=${LocalInstID}",
		"FuncID=${FuncID}",
		"bk_module_name=${bk_module_name}",
		"bk_set_env=${bk_set_env}",
		"bk_set_name=${bk_set_name}",
		"bk_host_innerip=${bk_host_innerip}",
		"bk_process_name=${bk_process_name}",
	}, "\n")

	result, err := client.PreviewConfigTemplate(context.Background(), env.BizID, content, env.ProcessID)
	if err != nil {
		t.Fatalf("PreviewConfigTemplate failed: %v", err)
	}

	t.Logf("rendered result:\n%s", result)

	for _, varName := range []string{"InstID=", "LocalInstID=", "FuncID=", "bk_module_name=", "bk_set_name="} {
		if !strings.Contains(result, varName) {
			t.Errorf("expected result to contain %q", varName)
		}
	}

	if strings.Contains(result, "${") {
		t.Error("rendered result still contains unresolved ${...} variables")
	}
}
