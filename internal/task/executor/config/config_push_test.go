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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

func TestDeliveredConfigContent(t *testing.T) {
	const multilingual = "中文 한국어 日本語 العربية हिन्दी русский 😀 e\u0301\r\n"
	tests := []struct {
		name     string
		content  string
		fileMode table.FileMode
		charset  table.FileCharset
		want     string
	}{
		{"multilingual", multilingual, table.Windows, table.UTF8, utf8BOM + multilingual},
		{"legacy empty charset", multilingual, table.Windows, "", utf8BOM + multilingual},
		{"ascii", "hello", table.Windows, table.UTF8, utf8BOM + "hello"},
		{"empty", "", table.Windows, table.UTF8, utf8BOM},
		{"existing bom", utf8BOM + multilingual, table.Windows, table.UTF8, utf8BOM + multilingual},
		{"repeated bom", utf8BOM + utf8BOM + multilingual, table.Windows, table.UTF8, utf8BOM + multilingual},
		{"only boms", utf8BOM + utf8BOM, table.Windows, table.UTF8, utf8BOM},
		{"interior feff", "a" + utf8BOM + "b", table.Windows, table.UTF8, utf8BOM + "a" + utf8BOM + "b"},
		{"gbk bytes mislabeled utf8", "\xd6\xd0\xce\xc4", table.Windows, table.UTF8, "\xd6\xd0\xce\xc4"},
		{"invalid legacy bytes", "\xff\x80", table.Windows, "", "\xff\x80"},
		{"utf16 bytes", "\xff\xfeh\x00i\x00", table.Windows, table.UTF8, "\xff\xfeh\x00i\x00"},
		{"truncated utf8", "\xe4\xb8", table.Windows, table.UTF8, "\xe4\xb8"},
		{"bom and invalid bytes", utf8BOM + "\xff", table.Windows, table.UTF8, utf8BOM + "\xff"},
		{"replacement character", "\ufffd", table.Windows, table.UTF8, utf8BOM + "\ufffd"},
		{"gbk", "\xd6\xd0", table.Windows, table.GBK, "\xd6\xd0"},
		{"gbk metadata with utf8", multilingual, table.Windows, table.GBK, multilingual},
		{"unknown charset", multilingual, table.Windows, "UTF-16", multilingual},
		{"unix", multilingual, table.Unix, table.UTF8, multilingual},
		{"unix existing boms", utf8BOM + utf8BOM + multilingual, table.Unix, table.UTF8, utf8BOM + utf8BOM + multilingual},
		{"unknown mode", multilingual, "", table.UTF8, multilingual},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deliveredConfigContent(tt.content, tt.fileMode, tt.charset)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, got, deliveredConfigContent(got, tt.fileMode, tt.charset), "重复处理应保持内容及 MD5 基线稳定")
		})
	}
}
