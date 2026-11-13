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
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

// catScreen 按 Windows 读取脚本的输出协议拼装 screen：Base64 + 换行 + 原始字节数。
func catScreen(raw string) string {
	return base64.StdEncoding.EncodeToString([]byte(raw)) + "\n" + strconv.Itoa(len(raw))
}

// 现网配置对比乱码的回归用例：
// 中文环境 Windows 的 GSE Agent 按 GBK 解释脚本输出再转 UTF-8 上报，
// 读取脚本若直接 type 文件，UTF-8 的韩文/日文/俄文会被二次解码成乱码。
func TestCatOutputToConfigContent(t *testing.T) {
	const multilingual = "中文 한국어 日本語 русский العربية\r\n"
	multilingualB64 := base64.StdEncoding.EncodeToString([]byte(multilingual))
	multilingualLen := strconv.Itoa(len(multilingual))

	tests := []struct {
		name     string
		screen   string
		fileMode table.FileMode
		charset  table.FileCharset
		want     string
		wantErr  bool
	}{
		{
			name:     "windows utf8 multilingual keeps bytes",
			screen:   catScreen(multilingual),
			fileMode: table.Windows,
			charset:  table.UTF8,
			want:     multilingual,
		},
		{
			name:     "windows utf8 with delivered bom strips bom",
			screen:   catScreen(utf8BOM + multilingual),
			fileMode: table.Windows,
			charset:  table.UTF8,
			want:     multilingual,
		},
		{
			name:     "windows empty charset treats as utf8",
			screen:   catScreen("한국어"),
			fileMode: table.Windows,
			charset:  "",
			want:     "한국어",
		},
		{
			name: "windows screen with line breaks and padding spaces",
			screen: "  " + multilingualB64[:8] + "\r\n" +
				multilingualB64[8:] + "\n" + multilingualLen + "\n",
			fileMode: table.Windows,
			charset:  table.UTF8,
			want:     multilingual,
		},
		{
			name:     "windows gbk file decoded to utf8",
			screen:   catScreen("\xd6\xd0\xce\xc4"),
			fileMode: table.Windows,
			charset:  table.GBK,
			want:     "中文",
		},
		{
			name:     "windows empty file",
			screen:   catScreen(""),
			fileMode: table.Windows,
			charset:  table.UTF8,
			want:     "",
		},
		{
			// 解码失败意味着采集异常，不能把 screen 原样当成现网配置
			name:     "windows non base64 screen returns error",
			screen:   "FILE_NOT_FOUND",
			fileMode: table.Windows,
			charset:  table.UTF8,
			wantErr:  true,
		},
		{
			name:     "windows powershell failure message returns error",
			screen:   "BSCP_READ_FAILED: Access to the path is denied.",
			fileMode: table.Windows,
			charset:  table.UTF8,
			wantErr:  true,
		},
		{
			name:     "windows missing byte length trailer returns error",
			screen:   multilingualB64,
			fileMode: table.Windows,
			charset:  table.UTF8,
			wantErr:  true,
		},
		{
			name:     "windows truncated base64 returns error",
			screen:   multilingualB64[:10] + "\n" + multilingualLen,
			fileMode: table.Windows,
			charset:  table.UTF8,
			wantErr:  true,
		},
		{
			// 原文长度是 3 的倍数时 Base64 不带 padding，截断到 4 字符组边界仍能解码，
			// 只有字节数对不上才能发现内容不完整
			name:     "windows truncated base64 on group boundary returns error",
			screen:   multilingualB64[:8] + "\n" + multilingualLen,
			fileMode: table.Windows,
			charset:  table.UTF8,
			wantErr:  true,
		},
		{
			name:     "windows removed type style raw bytes returns error",
			screen:   "\ufffd\ufffd\ufffd",
			fileMode: table.Windows,
			charset:  table.UTF8,
			wantErr:  true,
		},
		{
			name:     "linux keeps agent output as is",
			screen:   multilingual,
			fileMode: table.Unix,
			charset:  table.UTF8,
			want:     multilingual,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := catOutputToConfigContent(tt.screen, tt.fileMode, tt.charset)
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, got, "解码失败不能把 screen 当成配置内容返回")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.True(t, utf8.ValidString(got), "展示内容必须是合法 UTF-8，否则 proto/json 会拒绝序列化")
		})
	}
}

func TestCatOutputToConfigContentLinuxDoesNotDecode(t *testing.T) {
	raw := "한국어 config"
	b64 := base64.StdEncoding.EncodeToString([]byte(raw))

	// Linux 的 cat 输出原样透传，不能被误当成 Windows 的 Base64
	got, err := catOutputToConfigContent(b64, table.Unix, table.UTF8)
	require.NoError(t, err)
	assert.Equal(t, b64, got)
}

func TestToDisplayConfigContentInvalidBytes(t *testing.T) {
	// 元数据是 UTF-8 但文件里是非法字节：必须降级成合法 UTF-8，而不是让接口报错
	got := toDisplayConfigContent([]byte("\xff\x80abc"), table.UTF8)
	assert.True(t, utf8.ValidString(got))
	assert.Contains(t, got, "abc")
}

func TestToDisplayConfigContentKeepsInteriorBom(t *testing.T) {
	// 只去掉下发补的前导 BOM，正文里的 U+FEFF 属于用户内容，不能动
	got := toDisplayConfigContent([]byte(utf8BOM+"a"+utf8BOM+"b"), table.UTF8)
	assert.Equal(t, "a"+utf8BOM+"b", got)
}

func TestToDisplayConfigContentKeepsBomBeforeInvalidBytes(t *testing.T) {
	// 去掉 BOM 后不是合法 UTF-8，说明这 3 个字节是正文（例如 GBK 文件首行），不能剥
	got := toDisplayConfigContent([]byte(utf8BOM+"\xff"), table.UTF8)
	assert.True(t, strings.HasPrefix(got, "\ufeff"))
}

func TestBuildWindowsCatScriptOutputsBase64(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Windows}
	script, err := builder.BuildFileCatScript(`D:\bnsserver\web.config`)
	require.NoError(t, err)

	assert.Contains(t, script, `set "BSCP_TARGET=D:\bnsserver\web.config"`)
	assert.Contains(t, script, `$env:BSCP_TARGET`)
	// 用 Console.Out.Write 输出，避免 Write-Output 追加换行
	assert.Contains(t, script, `[Console]::Out.Write([Convert]::ToBase64String(`)
	// 末尾附带原始字节数，供服务端校验内容是否被截断
	assert.Contains(t, script, `+ [char]10 + $bytes.Length`)

	// type 会把文件字节直接喂给 Agent 的代码页，必须不再出现
	assert.NotContains(t, script, `type "`)
	assert.NotContains(t, script, `type %`)

	// 文件不存在时的失败信号保持不变
	assert.Contains(t, script, "FILE_NOT_FOUND")
	assert.Contains(t, script, "exit /b 1")
}

func TestBuildWindowsCatScriptHandlesReadFailure(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Windows}
	script, err := builder.BuildFileCatScript(`D:\bnsserver\web.config`)
	require.NoError(t, err)

	// if exist 只保证检查时刻存在，读取异常必须在 PowerShell 内被捕获并显式失败
	assert.Contains(t, script, `$ErrorActionPreference='Stop'`)
	assert.Contains(t, script, `catch`)
	assert.Contains(t, script, `exit 1`)
	// 退出码要传回批处理，否则 GSE 看到的 script_exit_code 仍为 0
	assert.Contains(t, script, "if errorlevel 1 exit /b 1")
}

func TestBuildWindowsCatScriptEscapesBatchSpecialChars(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Windows}
	script, err := builder.BuildFileCatScript(`C:\conf\100%done\app.conf`)
	require.NoError(t, err)

	// cmd 解析批处理时会展开 %NAME%，路径里的 % 必须成对转义成 %%
	assert.Contains(t, script, `set "BSCP_TARGET=C:\conf\100%%done\app.conf"`)

	// 继承的延迟展开会把路径里的 ! 吃掉，必须显式关闭
	assert.Contains(t, script, "setlocal DisableDelayedExpansion")
}
