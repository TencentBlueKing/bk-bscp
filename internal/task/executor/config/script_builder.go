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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

const (
	// defaultMaxBackups 是默认的脚本备份文件最大数量，超过后会删除最旧的备份文件
	defaultMaxBackups = 5
	// GSE 脚本执行账号
	linuxExecutionUser = "root"
	// windowsExecutionUser 只能是 system。bkop 环境 window 类型机器需要传入密码。
	// gse 建议使用 system 账号执行
	windowsExecutionUser = "system"
	// windowsAdminGroup 是 Windows 侧配置目录与文件授权使用的属组，固定为内置管理员组：
	// 部分账号（域账号、服务账号等）通过 WMI 查不到关联组，动态查询还容易受 WMI 服务
	// 状态影响。用 SID 而非组名授权（icacls 的 * 前缀表示按 SID 解析）：内置管理员组的
	// 名字随区域设置本地化（如德语 Administratoren）且可被重命名，按名字解析不到时
	// icacls 会失败并导致下发任务报错；S-1-5-32-544 在任何机器上都稳定存在。
	windowsAdminGroup = "*S-1-5-32-544"
)

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
		return b.buildWindowsPushScript(base64Content, absPath, owner, b.MaxBackups)
	}
	return buildLinuxPushScript(base64Content, absPath, fileMode, owner, b.MaxBackups)
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

// windowsAbsPathRe 匹配 Windows 绝对路径：盘符路径（D:\conf、D:/conf）或 UNC 路径（\\host\share）。
// 相对路径与驱动器相对路径（C:app.conf，盘符后没有分隔符）都不算绝对路径。
var windowsAbsPathRe = regexp.MustCompile(`^[A-Za-z]:[\\/]|^\\\\`)

// buildLinuxPushScript 构建 Linux 配置下发脚本
// nolint:funlen
func buildLinuxPushScript(base64Content, absPath, fileMode, owner string, maxBackups int) (string, error) {
	if !strings.HasPrefix(absPath, "/") {
		return "", fmt.Errorf("absPath must be absolute")
	}

	if !fileModeRe.MatchString(fileMode) {
		return "", fmt.Errorf("invalid fileMode: %s", fileMode)
	}

	return fmt.Sprintf(`#!/bin/bash
set -euo pipefail

TARGET_PATH=%s
MAX_BACKUPS=%d
OWNER=%s

# 1. 确认用户存在，并获取该用户的 UID 和主组 GID。
#    后续所有目录、备份文件、临时文件以及目标文件，
#    均使用该用户及其主组 GID。
if [[ "$OWNER" =~ ^[0-9]+$ ]]; then
    OWNER_UID="$OWNER"
    GROUP_GID="$OWNER"
else
    if ! OWNER_UID="$(id -u -- "$OWNER")"; then
        echo "OWNER_NOT_FOUND: cannot resolve user $OWNER on this host"
        exit 1
    fi

    if ! GROUP_GID="$(id -g -- "$OWNER")"; then
        echo "GROUP_NOT_FOUND: cannot resolve primary group for user $OWNER on this host"
        exit 1
    fi
fi

echo "Resolved owner: UID=$OWNER_UID GID=$GROUP_GID"

# 2. 目标为软链接时先解析真实路径，保持跟随语义：备份、临时文件与替换都作用于真实路径。
if [ -L "$TARGET_PATH" ]; then
    REAL_PATH="$(readlink -f -- "$TARGET_PATH")"
    echo "Resolved symlink: $TARGET_PATH -> $REAL_PATH"
    TARGET_PATH="$REAL_PATH"
fi

TARGET_DIR="$(dirname "$TARGET_PATH")"
TARGET_NAME="$(basename "$TARGET_PATH")"

# 3. 创建目标目录，并把本次新建的层级归属到目标用户及其主组 GID。
#
#    先记下最深的已存在祖先：只有它以下的层级是本次建出来的。
#    路径上已存在的目录可能是 /data、/etc 等与本次下发无关的共享目录，
#    不能递归 chown。
#
#    目录不做 chmod：文件权限模式不应直接套用到目录。
#    目录权限沿用 mkdir 的默认结果。
#
#    chown 失败仍拆掉本次新建的空目录，作为预检之后的兜底。
DIR_ANCESTOR="$TARGET_DIR"
while [ ! -d "$DIR_ANCESTOR" ]; do
    DIR_ANCESTOR="$(dirname "$DIR_ANCESTOR")"
done

mkdir -p -- "$TARGET_DIR"

CREATED_DIR="$TARGET_DIR"
while [ "$CREATED_DIR" != "$DIR_ANCESTOR" ] && [ "$CREATED_DIR" != "/" ]; do
    if ! chown "$OWNER_UID:$GROUP_GID" -- "$CREATED_DIR"; then
        echo "CHOWN_DIR_FAILED: cannot chown $CREATED_DIR to $OWNER_UID:$GROUP_GID"

        CLEAN_DIR="$TARGET_DIR"
        while [ "$CLEAN_DIR" != "$DIR_ANCESTOR" ] && [ "$CLEAN_DIR" != "/" ]; do
            rmdir -- "$CLEAN_DIR" 2>/dev/null || true
            CLEAN_DIR="$(dirname "$CLEAN_DIR")"
        done

        exit 1
    fi

    CREATED_DIR="$(dirname "$CREATED_DIR")"
done

# 4. 临时文件与目标同目录，保证后续 mv 在同一文件系统内落到 rename(2) 从而原子；
#    任一步骤失败都清理临时文件，不留残留、不触碰目标文件
TMP_PATH="${TARGET_PATH}.tmp.$$.${RANDOM}"
trap 'rm -f -- "$TMP_PATH"' EXIT

# 5. 备份原文件（如果存在），并让备份文件归属与目标文件保持一致。
if [ -f "$TARGET_PATH" ]; then
    TIMESTAMP="$(date +%%s)"
    BACKUP_PATH="${TARGET_DIR}/${TARGET_NAME}.${TIMESTAMP}.bak"

    cp -- "$TARGET_PATH" "$BACKUP_PATH"
    chown "$OWNER_UID:$GROUP_GID" -- "$BACKUP_PATH"

    echo "Backup created: $BACKUP_PATH"

    # 6. 清理旧备份：超过 MAX_BACKUPS 份则删除最旧的。
    #    按修改时间从旧到新排列，找出需要删除的文件。
    BACKUP_COUNT="$(ls -1 "${TARGET_DIR}/${TARGET_NAME}".*.bak 2>/dev/null | wc -l)"

    if [ "$BACKUP_COUNT" -gt "$MAX_BACKUPS" ]; then
        DELETE_COUNT=$(( BACKUP_COUNT - MAX_BACKUPS ))

        # ls -1t 按时间降序（最新在前），tail 取最旧的。
        ls -1t "${TARGET_DIR}/${TARGET_NAME}".*.bak 2>/dev/null \
            | tail -n "$DELETE_COUNT" \
            | xargs -r rm -f --

        echo "Cleaned $DELETE_COUNT old backup(s), kept latest $MAX_BACKUPS"
    fi
fi

# 7. 写入临时文件（base64 解码）
echo %s | base64 -d > "$TMP_PATH"

# 8. 在临时文件上设置权限和属主：失败即终止，目标文件保持原状
chmod %s -- "$TMP_PATH"
chown "$OWNER_UID:$GROUP_GID" -- "$TMP_PATH"

# 9. 继承目标文件原有的 SELinux 标签。
#
#    替换会产生新的 inode，新文件默认只能拿到目录的默认标签。
#    原文件若被 chcon 定制过就会退化，导致业务进程被拒绝读取
#    （下发成功但配置加载失败）。
#
#    SELinux 关闭或未安装 chcon 的机器上整段跳过，不影响下发。
if [ -e "$TARGET_PATH" ] && command -v chcon >/dev/null 2>&1; then
    chcon --reference="$TARGET_PATH" -- "$TMP_PATH" 2>/dev/null || true
fi

# 10. 原子替换目标文件
mv -f -- "$TMP_PATH" "$TARGET_PATH"

# 11. 校验（不影响主流程）
set +e
ls -l "$TARGET_PATH" || true
md5sum "$TARGET_PATH" || true
`,
		shellQuote(absPath),
		maxBackups,
		shellQuote(owner),
		shellQuote(base64Content),
		fileMode,
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

// newTempToken 生成临时文件名后缀。
// 唯一性必须由服务端保证，不能交给 cmd 的 %RANDOM%：它在 cmd 进程启动时以系统时钟播种，
// 同一时刻被 GSE 拉起的多个脚本会取到完全相同的序列，临时文件因而互相覆盖、互相删除。
func newTempToken() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}

// windowsTranslateOwnerCmd 用 NTAccount 把属主解析成 SID，账号取自脚本里的 BSCP_OWNER 变量。
// 与 icacls 走同一套 LookupAccountName；解析失败则脚本立刻退出，不会 mkdir。
func windowsTranslateOwnerCmd() string {
	return `powershell -NoProfile -Command "try { [void]([System.Security.Principal.NTAccount]$env:BSCP_OWNER)` +
		`.Translate([System.Security.Principal.SecurityIdentifier]) } catch { exit 1 }"`
}

// buildWindowsPushScript 构建 Windows 配置下发脚本
//
// base64 中转文件与解码产物都放在目标同目录，而不是曾经的 %TEMP%：
// 一是 %TEMP% 全机共享，同一台主机上多个任务下发同名配置（若干个 .NET 服务各有一份
// Web.config，只是目录不同）会撞进同一个命名空间；
// 二是 %TEMP% 与目标常常不在同一个卷上，move 会退化成「拷贝 + 删除」，
// 业务进程有机会读到只写了一半的配置，放同目录后才能走同卷内的原子 rename。
// nolint:funlen
func (b *ScriptBuilder) buildWindowsPushScript(base64Content, absPath, owner string, maxBackups int) (string, error) {
	if !windowsAbsPathRe.MatchString(absPath) {
		return "", fmt.Errorf("absPath must be absolute")
	}

	winPath := ToWindowsPath(absPath)
	token := newTempToken()

	return fmt.Sprintf(`@echo off
setlocal enabledelayedexpansion

set "TARGET_PATH=%s"
set /a MAX_BACKUPS=%d
set "BSCP_OWNER=%s"

REM 0. 写盘之前先确认属主能解析成 SID（与 icacls 同一套 LookupAccountName）。
REM    同目录并发时多个任务都会在这里失败，不会 mkdir，已存在的 conf 也不会被改属主。
%s
if !ERRORLEVEL! neq 0 (
    echo OWNER_NOT_FOUND
    exit /b 1
)

REM   属组固定用内置管理员组，不再动态查询用户所属组：
REM    部分账号（域账号、服务账号等）通过 WMI 查不到关联组，查询本身还容易受
REM    WMI 服务状态影响。按 SID 而非组名授权：组名随区域设置本地化且可被重命名，
REM    SID（icacls 的 * 前缀）在任何机器上都稳定存在。
set "TARGET_GROUP=%s"

REM 1. 解析目录和文件名
for %%%%i in ("%%TARGET_PATH%%") do (
    set "TARGET_DIR=%%%%~dpi"
    set "TARGET_NAME=%%%%~nxi"
)

REM 2. 创建目标目录。临时文件也落在这里，建不出来就必须立刻失败，
REM    否则错误会一路滑到后面的 move，报成含义完全不同的「找不到文件」。
REM
REM    建之前先逐级上溯，记下哪些层级是本次才会被创建的：mkdir 是递归的，
REM    建完就再也分不清哪几层是新建的、哪几层本来就有。已存在的目录一律不碰，
REM    它们可能是与本次下发无关的共享目录，改属主会波及其它业务。
set "DIR_CUR=!TARGET_DIR!"
if "!DIR_CUR:~-1!"=="\" set "DIR_CUR=!DIR_CUR:~0,-1!"
set /a NEW_DIR_COUNT=0
:collect_new_dirs
if exist "!DIR_CUR!\" goto collect_new_dirs_done
set /a NEW_DIR_COUNT+=1
set "NEW_DIR_!NEW_DIR_COUNT!=!DIR_CUR!"
for %%%%i in ("!DIR_CUR!") do set "DIR_PARENT=%%%%~dpi"
if "!DIR_PARENT:~-1!"=="\" set "DIR_PARENT=!DIR_PARENT:~0,-1!"
REM 上溯到驱动器根后父目录不再变化，据此收尾，避免路径异常时死循环
if "!DIR_PARENT!"=="!DIR_CUR!" goto collect_new_dirs_done
set "DIR_CUR=!DIR_PARENT!"
goto collect_new_dirs
:collect_new_dirs_done

if not exist "!TARGET_DIR!" mkdir "!TARGET_DIR!"
if not exist "!TARGET_DIR!" (
    echo MKDIR_FAILED
    exit /b 1
)

REM    把本次新建的层级归属到配置属主。Windows 下属主本身不带访问权（只隐含改权限的能力），
REM    所以还要显式授权，否则该账号进不了自己的配置目录。
REM    一律不带 (OI)(CI) 继承标记：权限只作用于目录本身，不影响目录内后续创建的文件。
REM    任一 icacls 失败都必须终止并拆掉本次新建的空目录：目录已经在了，下次
REM    NEW_DIR_COUNT=0，不会再修 ACL，任务却会一直报成功。
for /l %%%%n in (1,1,!NEW_DIR_COUNT!) do (
    set "NEW_DIR=!NEW_DIR_%%%%n!"
    icacls "!NEW_DIR!" /setowner "!BSCP_OWNER!" >nul
    if !ERRORLEVEL! neq 0 (
        echo ACL_FAILED
        echo [ERROR] icacls /setowner failed on !NEW_DIR!, errorlevel=!ERRORLEVEL!
        goto cleanup_new_dirs
    )
    icacls "!NEW_DIR!" /grant:r "!BSCP_OWNER!:(F)" >nul
    if !ERRORLEVEL! neq 0 (
        echo ACL_FAILED
        echo [ERROR] icacls grant owner failed on !NEW_DIR!, errorlevel=!ERRORLEVEL!
        goto cleanup_new_dirs
    )
    icacls "!NEW_DIR!" /grant:r "!TARGET_GROUP!:(RX)" >nul
    if !ERRORLEVEL! neq 0 (
        echo ACL_FAILED
        echo [ERROR] icacls grant group failed on !NEW_DIR!, errorlevel=!ERRORLEVEL!
        goto cleanup_new_dirs
    )
)

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

REM 5. 写入配置文件（base64 解码）。临时文件与目标同目录，保证后续 move 在同卷内原子完成；
REM    文件名后缀由服务端生成，全局唯一，避免同主机并发任务互相覆盖。
REM    base64 由服务端切成多行追加写入，单行绝不能超过 cmd 的行长上限，详见 windowsB64ChunkSize。
set "BSCP_TMP=!TARGET_DIR!!TARGET_NAME!.bscp.%s.b64"
set "BSCP_OUT=!TARGET_DIR!!TARGET_NAME!.bscp.%s.out"
del /f /q "!BSCP_TMP!" >nul 2>&1
del /f /q "!BSCP_OUT!" >nul 2>&1
%s
if not exist "!BSCP_TMP!" (
    echo WRITE_TMP_FAILED
    exit /b 1
)
REM certutil 默认拒绝覆盖已存在的输出文件，会报 0x80070050 ERROR_FILE_EXISTS，必须带 -f
certutil -f -decode "!BSCP_TMP!" "!BSCP_OUT!"
if !ERRORLEVEL! neq 0 (
    echo DECODE_FAILED
    del /f /q "!BSCP_TMP!" >nul 2>&1
    del /f /q "!BSCP_OUT!" >nul 2>&1
    exit /b 1
)
del /f /q "!BSCP_TMP!" >nul 2>&1
move /y "!BSCP_OUT!" "%%TARGET_PATH%%" >nul || (
    echo MOVE_FAILED
    del /f /q "!BSCP_OUT!" >nul 2>&1
    exit /b 1
)

REM 6. 设置文件属主与权限。失败必须终止：目录已在时写文件已经发生，
REM    若只打 WARN，坏属主仍会让任务报成功。
icacls "%%TARGET_PATH%%" /setowner "!BSCP_OWNER!" >nul
if !ERRORLEVEL! neq 0 (
    echo ACL_FAILED
    echo [ERROR] icacls /setowner failed, errorlevel=!ERRORLEVEL!
    exit /b 1
)
icacls "%%TARGET_PATH%%" /grant:r "!BSCP_OWNER!:(F)" >nul
if !ERRORLEVEL! neq 0 (
    echo ACL_FAILED
    echo [ERROR] icacls grant owner full control failed, errorlevel=!ERRORLEVEL!
    exit /b 1
)
icacls "%%TARGET_PATH%%" /grant:r "!TARGET_GROUP!:(R)" >nul
if !ERRORLEVEL! neq 0 (
    echo ACL_FAILED
    echo [ERROR] icacls grant group read failed, errorlevel=!ERRORLEVEL!
    exit /b 1
)

REM 7. 校验
dir "%%TARGET_PATH%%"
certutil -hashfile "%%TARGET_PATH%%" MD5

endlocal
goto :eof

REM 从最深到最浅拆掉本次新建的空目录（NEW_DIR_1 是最深的那层），下次下发才能重新收集并授权。
REM rmdir 不带 /s：目录里若已有无关内容就留着，避免误删。
:cleanup_new_dirs
for /l %%%%n in (1,1,!NEW_DIR_COUNT!) do (
    set "NEW_DIR=!NEW_DIR_%%%%n!"
    rmdir "!NEW_DIR!" >nul 2>&1
)
endlocal
exit /b 1
`,
		winPath,
		maxBackups,
		owner,
		windowsTranslateOwnerCmd(),
		windowsAdminGroup,
		token, token,
		buildWindowsB64WriteLines(base64Content),
	), nil
}

// windowsB64ChunkSize 是 base64 中转文件每行写入的字符数。
// cmd.exe 解析批处理时单行有约 8191 字符的上限，超出部分被静默丢弃。整份 base64 写在一行里，
// 配置超过 6 KB 左右就会被截断，而 certutil 仍会把残缺的 base64 解码成一个语法上"看起来正常"
// 的文件，move 和退出码全部正常，下发任务照样报成功——故障只能靠人工比对配置才能发现。
// 必须是 4 的倍数，否则行尾会切在 base64 四字符组中间。
const windowsB64ChunkSize = 1024

// buildWindowsB64WriteLines 把 base64 内容切成多行 echo，逐行追加进中转文件。
//
// 重定向写在 echo 之前而不是行尾，有两个原因：
// 一是 `echo x > f` 会把 `>` 前的那个空格一起写进文件；
// 二是 base64 字母表含数字，`echo ...1>f` 里的末位数字会被 cmd 当作重定向句柄吃掉，
// 导致内容静默少一个字符。base64 字母表（A-Za-z0-9+/=）不含 cmd 元字符，无需转义。
func buildWindowsB64WriteLines(base64Content string) string {
	// 空内容也要把中转文件建出来，否则后面的存在性检查会报成含义不同的 WRITE_TMP_FAILED
	if base64Content == "" {
		return `>>"!BSCP_TMP!" echo.`
	}

	var b strings.Builder
	for i := 0; i < len(base64Content); i += windowsB64ChunkSize {
		end := i + windowsB64ChunkSize
		if end > len(base64Content) {
			end = len(base64Content)
		}
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(`>>"!BSCP_TMP!" echo `)
		b.WriteString(base64Content[i:end])
	}
	return b.String()
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

// GetExecutionUser 返回 GSE 脚本的执行账号
// Linux 固定为 root，Windows 固定为 system
func GetExecutionUser(fileMode table.FileMode) string {
	if fileMode == table.Windows {
		return windowsExecutionUser
	}
	return linuxExecutionUser
}

// ToWindowsPath 将 POSIX 路径转换为 Windows 路径
func ToWindowsPath(posixPath string) string {
	return strings.ReplaceAll(posixPath, "/", `\`)
}
