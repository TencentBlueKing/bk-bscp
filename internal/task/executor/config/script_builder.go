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
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

// defaultMaxBackups 是默认的脚本备份文件最大数量，超过后会删除最旧的备份文件
const defaultMaxBackups = 5

// ScriptBuilder 根据 FileMode (OS 类型) 构建不同平台的脚本
type ScriptBuilder struct {
	FileMode   table.FileMode
	MaxBackups int
}

// IsWindows 判断目标平台是否为 Windows
func (b *ScriptBuilder) IsWindows() bool {
	return b.FileMode == table.Windows
}

// BuildConfigPushScript 构建配置下发脚本
func (b *ScriptBuilder) BuildConfigPushScript(base64Content, absPath, fileMode, owner, group string) (string, error) {
	if b.MaxBackups <= 0 {
		b.MaxBackups = defaultMaxBackups
	}
	if b.IsWindows() {
		return b.buildWindowsPushScript(base64Content, absPath, owner, group, b.MaxBackups)
	}
	return buildLinuxPushScript(base64Content, absPath, fileMode, owner, group, b.MaxBackups)
}

// BuildFileMD5Script 构建计算文件 MD5 的脚本
func (b *ScriptBuilder) BuildFileMD5Script(absPath string) (string, error) {
	if b.IsWindows() {
		return b.buildWindowsMD5Script(absPath)
	}
	return buildLinuxMD5Script(absPath)
}

// BuildFileCatScript 构建读取文件内容的脚本
func (b *ScriptBuilder) BuildFileCatScript(absPath string) (string, error) {
	if b.IsWindows() {
		return b.buildWindowsCatScript(absPath)
	}
	return buildLinuxCatScript(absPath)
}

// ---- Linux 脚本 ----

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\'"\'"'`) + "'"
}

var fileModeRe = regexp.MustCompile(`^[0-7]{3,4}$`)

// buildLinuxPushScript 构建 Linux 配置下发脚本
func buildLinuxPushScript(base64Content, absPath, fileMode, owner, group string, maxBackups int) (string, error) {
	if !strings.HasPrefix(absPath, "/") {
		return "", fmt.Errorf("absPath must be absolute")
	}

	if !fileModeRe.MatchString(fileMode) {
		return "", fmt.Errorf("invalid fileMode: %s", fileMode)
	}

	return fmt.Sprintf(`#!/bin/bash
set -euo pipefail

TARGET_PATH=%s
TARGET_DIR="$(dirname "$TARGET_PATH")"
TARGET_NAME="$(basename "$TARGET_PATH")"
MAX_BACKUPS=%d

# 1. 创建目标目录
mkdir -p -- "$TARGET_DIR"

# 2. 备份原文件（如果存在）
if [ -f "$TARGET_PATH" ]; then
    TIMESTAMP="$(date +%%s)"
    BACKUP_PATH="${TARGET_DIR}/${TARGET_NAME}.${TIMESTAMP}.bak"
    cp -- "$TARGET_PATH" "$BACKUP_PATH"
    echo "Backup created: $BACKUP_PATH"

    # 3. 清理旧备份：超过 MAX_BACKUPS 份则删除最旧的
    # 按修改时间从旧到新排列，找出需要删除的文件
    BACKUP_COUNT="$(ls -1 "${TARGET_DIR}/${TARGET_NAME}".*.bak 2>/dev/null | wc -l)"
    if [ "$BACKUP_COUNT" -gt "$MAX_BACKUPS" ]; then
        DELETE_COUNT=$(( BACKUP_COUNT - MAX_BACKUPS ))
        # ls -1t 按时间降序（最新在前），tail 取最旧的
        ls -1t "${TARGET_DIR}/${TARGET_NAME}".*.bak 2>/dev/null \
            | tail -n "$DELETE_COUNT" \
            | xargs -r rm -f --
        echo "Cleaned $DELETE_COUNT old backup(s), kept latest $MAX_BACKUPS"
    fi
fi

# 3. 写入配置文件（base64 解码）
echo %s | base64 -d > "$TARGET_PATH"

# 4. 设置权限和属主
chmod %s "$TARGET_PATH"
chown %s:%s "$TARGET_PATH"

# 5. 校验（不影响主流程）
set +e
ls -l "$TARGET_PATH" || true
md5sum "$TARGET_PATH" || true
`,
		shellQuote(absPath),
		maxBackups,
		shellQuote(base64Content),
		fileMode,
		shellQuote(owner),
		shellQuote(group),
	), nil
}

// buildLinuxMD5Script 构建 Linux MD5 校验脚本
func buildLinuxMD5Script(absPath string) (string, error) {
	if !strings.HasPrefix(absPath, "/") {
		return "", fmt.Errorf("absPath must be absolute")
	}

	return fmt.Sprintf(`#!/bin/bash
set -euo pipefail

TARGET_PATH=%s

md5sum "$TARGET_PATH" | awk '{print $1}'
`,
		shellQuote(absPath),
	), nil
}

// buildLinuxCatScript 构建 Linux 文件内容读取脚本
func buildLinuxCatScript(absPath string) (string, error) {
	if !strings.HasPrefix(absPath, "/") {
		return "", fmt.Errorf("absPath must be absolute")
	}

	return fmt.Sprintf(`#!/bin/bash
set -euo pipefail

TARGET_PATH=%s

cat "$TARGET_PATH"
`,
		shellQuote(absPath),
	), nil
}

// ---- Windows 脚本 ----

// buildWindowsPushScript 构建 Windows 配置下发脚本
func (b *ScriptBuilder) buildWindowsPushScript(base64Content, absPath, owner, group string, maxBackups int) (string, error) {
	winPath := ToWindowsPath(absPath)

	return fmt.Sprintf(`@echo off
setlocal enabledelayedexpansion

set "TARGET_PATH=%s"
set /a MAX_BACKUPS=%d

REM 1. 解析目录和文件名
for %%%%i in ("%%TARGET_PATH%%") do (
    set "TARGET_DIR=%%%%~dpi"
    set "TARGET_NAME=%%%%~nxi"
)

REM 2. 创建目标目录
if not exist "%%TARGET_DIR%%" mkdir "%%TARGET_DIR%%"

if exist "!TARGET_PATH!" (
    echo [INFO] 发现原文件，准备备份...

    REM 获取时间戳
    for /f "delims=" %%%%i in (
        'powershell -NoProfile -Command "Get-Date -Format yyyyMMddHHmmss"'
    ) do set "STAMP=%%%%i"

    echo [INFO] 时间戳: !STAMP!

    set "BACKUP_FILE=!TARGET_NAME!.!STAMP!.bak"
    set "BACKUP_FULL_PATH=!TARGET_DIR!!BACKUP_FILE!"

    copy /y "!TARGET_PATH!" "!BACKUP_FULL_PATH!" >nul || (
        echo [ERROR] 备份失败
        exit /b 1
    )
    echo [OK] 备份已生成: !BACKUP_FILE!

    REM 3. 统计备份数量
    set /a COUNT=0
    for /f "delims=" %%%%f in (
        'dir /b /o:d "!TARGET_DIR!!TARGET_NAME!.*.bak" 2^>nul'
    ) do set /a COUNT+=1

    echo [INFO] 当前备份数: !COUNT! / 最大保留: %%MAX_BACKUPS%%

    REM 4. 删除最旧备份（无临时文件版本）
    if !COUNT! gtr %%MAX_BACKUPS%% (
        set /a DEL_COUNT=!COUNT!-%%MAX_BACKUPS%%
        echo [INFO] 需删除最旧备份数: !DEL_COUNT!

        set /a IDX=0
        for /f "delims=" %%%%f in (
            'dir /b /o:d "!TARGET_DIR!!TARGET_NAME!.*.bak" 2^>nul'
        ) do (
            if !IDX! lss !DEL_COUNT! (
                echo [CLEAN] 删除旧备份: %%%%f
                del /f /q "!TARGET_DIR!%%%%f" >nul 2>&1
                set /a IDX+=1
            )
        )
    )
) else (
    echo [INFO] 目标文件不存在，跳过备份。
)

REM 5. 写入配置文件（base64 解码），用目标文件名隔离临时文件避免并发碰撞
set "BSCP_TMP=%%TEMP%%\bscp_!TARGET_NAME!_%%RANDOM%%.b64"
set "BSCP_OUT=%%TEMP%%\bscp_!TARGET_NAME!_%%RANDOM%%.out"
echo %s > "!BSCP_TMP!"
if not exist "!BSCP_TMP!" (
    echo WRITE_TMP_FAILED
    exit /b 1
)
certutil -decode "!BSCP_TMP!" "!BSCP_OUT!"
if !ERRORLEVEL! neq 0 (
    echo DECODE_FAILED
    del "!BSCP_TMP!" 2>nul
    del "!BSCP_OUT!" 2>nul
    exit /b 1
)
del "!BSCP_TMP!" 2>nul
move /y "!BSCP_OUT!" "%%TARGET_PATH%%" >nul || (
    echo MOVE_FAILED
    del "!BSCP_OUT!" 2>nul
    exit /b 1
)

REM 6. 设置权限
icacls "%%TARGET_PATH%%" /setowner "%s" >nul 2>&1
icacls "%%TARGET_PATH%%" /grant:r "%s:(F)" >nul 2>&1
icacls "%%TARGET_PATH%%" /grant:r "%s:(R)" >nul 2>&1

REM 7. 校验
dir "%%TARGET_PATH%%"
certutil -hashfile "%%TARGET_PATH%%" MD5

endlocal
`, winPath, maxBackups, base64Content, owner, owner, group), nil
}

// buildWindowsMD5Script 构建 Windows MD5 校验脚本
func (b *ScriptBuilder) buildWindowsMD5Script(absPath string) (string, error) {
	winPath := ToWindowsPath(absPath)

	return fmt.Sprintf(`@echo off
for /f "skip=1 tokens=*" %%%%a in ('certutil -hashfile "%s" MD5 ^| findstr /v "CertUtil"') do (
    echo %%%%a
    goto :eof
)
`, winPath), nil
}

// buildWindowsCatScript 构建 Windows 文件内容读取脚本
func (b *ScriptBuilder) buildWindowsCatScript(absPath string) (string, error) {
	winPath := ToWindowsPath(absPath)

	return fmt.Sprintf(`@echo off
if exist "%s" (
    type "%s"
) else (
    echo FILE_NOT_FOUND
    exit /b 1
)
`, winPath, winPath), nil
}

// ---- 公共辅助函数 ----

// ScriptStoreDirByFileMode 根据平台返回脚本存放目录
func ScriptStoreDirByFileMode(linuxBaseDir, windowsScriptDir string,
	fileMode table.FileMode) string {
	if fileMode == table.Windows {
		return windowsScriptDir
	}
	return linuxBaseDir
}

// BuildScriptNameByFileMode 生成脚本文件名（区分平台后缀）
func BuildScriptNameByFileMode(action string, p *common.TaskPayload, fileMode table.FileMode) string {
	ext := ".sh"
	if fileMode == table.Windows {
		ext = ".bat"
	}
	return fmt.Sprintf("bk_gse_script_%s_%d_%d_%d_%d%s",
		action,
		time.Now().Unix(),
		p.ConfigPayload.ConfigTemplateID,
		p.ProcessPayload.CcProcessID,
		p.ProcessPayload.ModuleInstSeq,
		ext,
	)
}

// BuildScriptCommand 根据平台构建脚本执行命令
func BuildScriptCommand(storeDir, scriptName string, fileMode table.FileMode) string {
	if fileMode == table.Windows {
		return ToWindowsPath(storeDir) + `\` + scriptName
	}
	return path.Join(storeDir, scriptName)
}

// GetExecutionUser 根据平台返回执行账号
func GetExecutionUser(fileMode table.FileMode, configUser string) string {
	if configUser != "" {
		return configUser
	}
	if fileMode == table.Windows {
		return "Administrator"
	}
	return "root"
}

// ToWindowsPath 将 POSIX 路径转换为 Windows 路径
func ToWindowsPath(posixPath string) string {
	return strings.ReplaceAll(posixPath, "/", `\`)
}
