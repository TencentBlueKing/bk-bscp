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

package validator

import (
	"strings"
	"testing"

	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

func TestValidateFileName(t *testing.T) {
	k := kit.New()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// valid cases
		{"normal ascii name", "config.yaml", false},
		{"chinese name", "配置文件.yaml", false},
		{"name with hyphen", "my-config", false},
		{"name with underscore", "my_config", false},
		{"name with dot", "app.config.json", false},
		{"name with space", "my config", false},
		{"single char", "a", false},
		{"mixed chinese and english", "配置config-01.yaml", false},
		{"numeric name", "12345", false},

		// empty and length
		{"empty name", "", true},
		{"name exceeds 64 chars", strings.Repeat("a", 65), true},
		{"name exactly 64 chars", strings.Repeat("a", 64), false},

		// reserved prefix
		{"reserved prefix _bk", "_bk_test", true},
		{"reserved prefix _BK uppercase", "_BK_test", true},

		// dots only
		{"single dot", ".", true},
		{"double dots", "..", true},
		{"triple dots", "...", true},

		// XSS payloads - must be rejected by whitelist
		{"script tag", "<script>alert('xss')</script>", true},
		{"img onerror", `<img src=x onerror="alert(1)">`, true},
		{"svg onload", `<svg onload="alert(1)">`, true},
		{"iframe tag", `<iframe src="evil.com">`, true},
		{"javascript protocol", `javascript:alert(1)`, true},
		{"html entity attempt", `&lt;script&gt;`, true},
		{"angle brackets only", "<>", true},
		{"single less than", "config<name", true},
		{"single greater than", "config>name", true},
		{"double quotes", `config"name`, true},
		{"single quotes", "config'name", true},
		{"ampersand", "config&name", true},
		{"slash in name", "path/name", true},
		{"backslash in name", `path\name`, true},
		{"event handler attempt", `name onerror=alert(1)`, true}, // contains = and (), rejected by whitelist
		{"encoded xss attempt", `%3Cscript%3E`, true},            // contains % which is not in whitelist
		{"null byte", "config\x00name", true},
		{"tab char", "config\tname", true},
		{"newline char", "config\nname", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFileName(k, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFileName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
