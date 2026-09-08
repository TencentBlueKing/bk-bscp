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
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

func TestGetExecutionUser(t *testing.T) {
	// 断言字面量而不是常量本身：Windows 填 Administrator 时 GSE agent 无法完成 logon，
	// 脚本在启动阶段就以 scriptExitCode=126 失败，screen 为空、没有任何可定位的线索。
	tests := []struct {
		name     string
		fileMode table.FileMode
		want     string
	}{
		{"linux uses root", table.Unix, "root"},
		{"windows uses system", table.Windows, "system"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GetExecutionUser(tt.fileMode))
		})
	}
}

func TestBuildLinuxPushScriptRejectsInvalidInput(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Unix}

	_, err := builder.BuildConfigPushScript("Y29udGVudA==", "relative/path.conf", "644", "root", "root")
	assert.ErrorContains(t, err, "absPath must be absolute")

	_, err = builder.BuildConfigPushScript("Y29udGVudA==", "/etc/app.conf", "888", "root", "root")
	assert.ErrorContains(t, err, "invalid fileMode")
}

// TestBuildWindowsPushScriptRejectsInvalidInput 与 Linux 侧对称：相对路径不能进入
// Windows 脚本，否则 %~dpi 会解析出相对于脚本工作目录的路径，目录收集、备份、move
// 全部落在不可控的位置。
func TestBuildWindowsPushScriptRejectsInvalidInput(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Windows}

	_, err := builder.BuildConfigPushScript("Y29udGVudA==", `relative\path.conf`, "", "appuser", "appgroup")
	assert.ErrorContains(t, err, "absPath must be absolute")

	// 驱动器相对路径（盘符后无分隔符）同样不是绝对路径
	_, err = builder.BuildConfigPushScript("Y29udGVudA==", `C:app.conf`, "", "appuser", "appgroup")
	assert.ErrorContains(t, err, "absPath must be absolute")
}

// TestBuildLinuxPushScriptAtomicOrder 校验脚本按「解析软链接 -> 备份 -> 写临时文件 -> 设权限属主 -> 原子替换」编排，
// 权限与属主必须设置在临时文件上，否则失败时会留下属主不正确的目标文件。
func TestBuildLinuxPushScriptAtomicOrder(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Unix}

	script, err := builder.BuildConfigPushScript("Y29udGVudA==", "/etc/app/app.conf", "444", "www-data", "www-data")
	require.NoError(t, err)

	// 属主只在开头引用一次，其余位置统一走变量：这些值曾经作为多个位置参数
	// 反复出现在同一个格式串里，顺序错一对就会把属主和属组写反
	assert.Contains(t, script, `OWNER='www-data'`)
	// 属组不再下发：由脚本从属主解析出主组 GID
	assert.Contains(t, script, `GROUP_GID="$(id -g -- "$OWNER")"`)

	assert.Contains(t, script, `readlink -f -- "$TARGET_PATH"`)
	assert.Contains(t, script, `chmod 444 -- "$TMP_PATH"`)
	assert.Contains(t, script, `chown "$OWNER_UID:$GROUP_GID" -- "$TMP_PATH"`)
	assert.Contains(t, script, `chown "$OWNER_UID:$GROUP_GID" -- "$BACKUP_PATH"`)
	assert.Contains(t, script, `mv -f -- "$TMP_PATH" "$TARGET_PATH"`)
	assert.Contains(t, script, `trap 'rm -f -- "$TMP_PATH"' EXIT`)

	// 临时文件必须与目标同目录，否则 mv 跨文件系统退化为「拷贝 + 删除」而不再原子
	assert.Contains(t, script, `TMP_PATH="${TARGET_PATH}.tmp.$$.${RANDOM}"`)

	orderOf := func(needle string) int {
		idx := strings.Index(script, needle)
		require.NotEqual(t, -1, idx, "script should contain %q", needle)
		return idx
	}
	assert.Less(t, orderOf("readlink -f"), orderOf("BACKUP_PATH="))
	assert.Less(t, orderOf(`trap 'rm -f`), orderOf("BACKUP_PATH="))
	assert.Less(t, orderOf("base64 -d"), orderOf("chmod 444"))
	assert.Less(t, orderOf("chmod 444"), orderOf(`chown "$OWNER_UID:$GROUP_GID" -- "$TMP_PATH"`))
	assert.Less(t, orderOf(`chown "$OWNER_UID:$GROUP_GID" -- "$TMP_PATH"`), orderOf("mv -f --"))
}

// TestBuildLinuxPushScriptChownsOnlyNewDirs 目录归属只能作用于本次 mkdir 新建出来的层级。
// 路径上已存在的目录往往是 /data、/etc 这类被多方共用的目录，一旦被递归 chown，
// 与本次下发无关的文件会跟着换属主，是比配置下发失败严重得多的事故。
func TestBuildLinuxPushScriptChownsOnlyNewDirs(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Unix}

	script, err := builder.BuildConfigPushScript("Y29udGVudA==", "/data/app/conf/app.conf", "444",
		"www-data", "www-data")
	require.NoError(t, err)

	assert.NotContains(t, script, "chown -R", "递归 chown 会改掉已存在的共享目录属主")
	// 自底向上 chown，遇到最深的已存在祖先即停止
	assert.Contains(t, script, `while [ "$CREATED_DIR" != "$DIR_ANCESTOR" ] && [ "$CREATED_DIR" != "/" ]; do`)
	assert.Contains(t, script, `chown "$OWNER_UID:$GROUP_GID" -- "$CREATED_DIR"`)

	// chown 失败时只拆本次新建的空目录，好让重试能重新收集。不得 rm -rf，也不得越过 DIR_ANCESTOR
	assert.NotContains(t, script, "rm -rf", "不得递归删除目录")
	assert.Contains(t, script, `rmdir -- "$CLEAN_DIR"`)
	assert.Contains(t, script, `while [ "$CLEAN_DIR" != "$DIR_ANCESTOR" ] && [ "$CLEAN_DIR" != "/" ]; do`)
	assert.Contains(t, script, "CHOWN_DIR_FAILED")

	// 目录权限必须沿用 mkdir 的默认结果：用例特意取了不含 x 位的权限值，
	// 一旦被套到目录上，目录就无法进入
	assert.NotContains(t, script, `chmod 444 -- "$CREATED_DIR"`)

	orderOf := func(needle string) int {
		idx := strings.Index(script, needle)
		require.NotEqual(t, -1, idx, "script should contain %q", needle)
		return idx
	}
	// 祖先必须在 mkdir 之前记下来，否则建完目录就分不清哪几层是新建的
	assert.Less(t, orderOf(`while [ ! -d "$DIR_ANCESTOR" ]`), orderOf(`mkdir -p -- "$TARGET_DIR"`))
	assert.Less(t, orderOf(`mkdir -p -- "$TARGET_DIR"`), orderOf(`chown "$OWNER_UID:$GROUP_GID" -- "$CREATED_DIR"`))
	// 已存在的配置目录不能每次都被 chown，否则会改掉业务原来的目录属主
	assert.NotContains(t, script, `chown "$OWNER_UID:$GROUP_GID" -- "$TARGET_DIR"`)
}

// TestLinuxPushScriptCreatesMissingDirs 目标路径上的目录不存在时应逐层建出来并完成下发
func TestLinuxPushScriptCreatesMissingDirs(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a", "b", "c", "app.conf")

	out, err := runPushScript(t, target, "new content", "644", 5)
	require.NoError(t, err, "script output: %s", out)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(got))

	// 新建的目录必须可进入，否则业务进程读不到配置
	for _, sub := range []string{"a", filepath.Join("a", "b"), filepath.Join("a", "b", "c")} {
		info, statErr := os.Stat(filepath.Join(dir, sub))
		require.NoError(t, statErr)
		require.True(t, info.IsDir())
		assert.NotZero(t, info.Mode().Perm()&0o100, "目录缺少 x 位会导致无法进入: %s", sub)
	}
}

// TestLinuxPushScriptFailsBeforeTouchingDiskWhenOwnerUnresolvable 属主解析不出来时，
// 必须在 mkdir 之前退出。同目录并发下发时所有任务都会在预检失败，不会建目录、不会写文件。
func TestLinuxPushScriptFailsBeforeTouchingDiskWhenOwnerUnresolvable(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	tests := []struct {
		name       string
		owner      string
		wantOutput string
	}{
		{"missing user", "bscp-user-does-not-exist", "OWNER_NOT_FOUND"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "a", "b", "app.conf")

			builder := &ScriptBuilder{FileMode: table.Unix, MaxBackups: 5}
			script, err := builder.BuildConfigPushScript(
				base64.StdEncoding.EncodeToString([]byte("new content")), target, "644", tt.owner, "")
			require.NoError(t, err)

			scriptPath := filepath.Join(t.TempDir(), "push.sh")
			require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o700))

			out, err := exec.Command("bash", scriptPath).CombinedOutput()
			require.Error(t, err, "script should fail when the account cannot be resolved, output: %s", out)
			assert.Contains(t, string(out), tt.wantOutput)

			_, statErr := os.Stat(filepath.Join(dir, "a"))
			assert.True(t, os.IsNotExist(statErr), "预检失败时一个目录都不该建出来, output: %s", out)

			assertNoTempLeftover(t, dir)
		})
	}
}

// TestLinuxPushScriptAcceptsNumericOwner 属主字段是自由文本，填数字 uid/gid 是合法用法：
// chown 直接把它当 id 用，不要求 passwd/group 里有同名条目。
func TestLinuxPushScriptAcceptsNumericOwner(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	// 纯数字属主必须跳过 id -u 预检：没有 passwd 条目的 UID（容器/服务账号的随机 UID）
	// 用 id 解析必然失败，报 OWNER_NOT_FOUND 会让这类配置在任何文件操作前就整单失败。
	// 注意不能用当前进程的 uid 验证这一点：它通常已映射到 passwd，id 总能解析成功。
	builder := &ScriptBuilder{FileMode: table.Unix, MaxBackups: 5}
	script, err := builder.BuildConfigPushScript(
		base64.StdEncoding.EncodeToString([]byte("new content")), "/data/app/conf/app.conf", "644",
		"12345", "")
	require.NoError(t, err)

	// 数字分支不做 id 预检：UID 与 GID 直接落定为该数字，不依赖 passwd/group 条目；
	// 赋值必须排在 else 分支的 id 预检之前
	assert.Contains(t, script, `if [[ "$OWNER" =~ ^[0-9]+$ ]]; then`)
	assert.Contains(t, script, `OWNER_UID="$OWNER"`)
	assert.Contains(t, script, `GROUP_GID="$OWNER"`)
	assert.Less(t, strings.Index(script, `OWNER_UID="$OWNER"`), strings.Index(script, `id -u -- "$OWNER"`))

	dir := t.TempDir()
	target := filepath.Join(dir, "app.conf")

	// 数字属主会同时被用作 UID 与 GID：非 root 且 uid != gid 时 chown 会 EPERM，跳过执行验证
	if os.Geteuid() != 0 && os.Getuid() != os.Getgid() {
		t.Skip("非 root 且 uid != gid，无法执行数字属主下发验证")
	}

	script, err = builder.BuildConfigPushScript(
		base64.StdEncoding.EncodeToString([]byte("new content")), target, "644",
		strconv.Itoa(os.Getuid()), strconv.Itoa(os.Getgid()))
	require.NoError(t, err)

	scriptPath := filepath.Join(t.TempDir(), "push.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o700))

	out, err := exec.Command("bash", scriptPath).CombinedOutput()
	require.NoError(t, err, "纯数字 uid/gid 应能下发, output: %s", out)

	got, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, "new content", string(got))

	// 没有 passwd 条目的数字 UID 才是真正的目标场景；只有 root 能 chown 到任意 UID
	if os.Geteuid() != 0 {
		t.Skip("需要 root 才能验证无 passwd 条目的数字 UID，跳过")
	}

	uid := "3999999"
	if err := exec.Command("id", "-u", "--", uid).Run(); err == nil {
		t.Skipf("uid %s 在该主机有 passwd 条目，无法覆盖无条目场景", uid)
	}

	target = filepath.Join(t.TempDir(), "app.conf")
	script, err = builder.BuildConfigPushScript(
		base64.StdEncoding.EncodeToString([]byte("new content")), target, "644", uid, "")
	require.NoError(t, err)

	scriptPath = filepath.Join(t.TempDir(), "push_no_passwd.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o700))

	out, err = exec.Command("bash", scriptPath).CombinedOutput()
	require.NoError(t, err, "无 passwd 条目的纯数字 UID 应能下发, output: %s", out)

	got, readErr = os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, "new content", string(got))

	statOut, statErr := exec.Command("stat", "-c", "%u:%g", target).Output()
	require.NoError(t, statErr)
	assert.Equal(t, uid+":"+uid, strings.TrimSpace(string(statOut)),
		"数字属主应直接落定为 UID 与 GID，不依赖 passwd/group 条目")
}

// TestBuildLinuxPushScriptValidatesAccountsFirst 校验必须排在所有写操作之前。
func TestBuildLinuxPushScriptValidatesAccountsFirst(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Unix}

	script, err := builder.BuildConfigPushScript("Y29udGVudA==", "/data/app/conf/app.conf", "644",
		"www-data", "www-data")
	require.NoError(t, err)

	assert.Contains(t, script, `id -u -- "$OWNER"`)
	assert.Contains(t, script, `id -g -- "$OWNER"`)
	assert.Contains(t, script, "OWNER_NOT_FOUND")
	assert.Contains(t, script, "GROUP_NOT_FOUND")

	orderOf := func(needle string) int {
		idx := strings.Index(script, needle)
		require.NotEqual(t, -1, idx, "script should contain %q", needle)
		return idx
	}
	assert.Less(t, orderOf(`id -u -- "$OWNER"`), orderOf(`mkdir -p -- "$TARGET_DIR"`))
	assert.Less(t, orderOf(`id -g -- "$OWNER"`), orderOf(`mkdir -p -- "$TARGET_DIR"`))
}

// TestBuildLinuxPushScriptPreservesSELinuxContext 原子替换换了 inode，新文件只能拿到目录的默认
// SELinux 标签；原文件若被 chcon 定制过就会退化，业务进程会在下发「成功」后读不到配置。
// 因此必须在替换前把原标签复制到临时文件上，且该步骤在无 SELinux 的机器上要能整段跳过。
func TestBuildLinuxPushScriptPreservesSELinuxContext(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Unix}

	script, err := builder.BuildConfigPushScript("Y29udGVudA==", "/etc/app/app.conf", "644", "root", "root")
	require.NoError(t, err)

	assert.Contains(t, script, `chcon --reference="$TARGET_PATH" -- "$TMP_PATH"`)
	// 目标文件不存在（首次下发）或机器未装 chcon 时跳过，避免无谓报错
	assert.Contains(t, script, `if [ -e "$TARGET_PATH" ] && command -v chcon >/dev/null 2>&1; then`)
	// 失败不得中断下发：SELinux 关闭的机器上 chcon 会报错退出
	assert.Contains(t, script, `chcon --reference="$TARGET_PATH" -- "$TMP_PATH" 2>/dev/null || true`)

	chconAt := strings.Index(script, "chcon --reference")
	mvAt := strings.Index(script, "mv -f --")
	chownAt := strings.Index(script, `chown "$OWNER_UID:$GROUP_GID" -- "$TMP_PATH"`)
	require.NotEqual(t, -1, chconAt)
	require.NotEqual(t, -1, mvAt)
	require.NotEqual(t, -1, chownAt)

	// 必须作用在临时文件上并早于替换，否则标签设不到最终文件上
	assert.Less(t, chownAt, chconAt, "chcon 应在 chown 之后，避免属主变更影响标签")
	assert.Less(t, chconAt, mvAt, "chcon 必须早于 mv")
}

// TestLinuxPushScriptSucceedsWithoutSELinuxTooling 构建机通常没有 chcon，
// 该用例确认标签保留步骤不会让下发在这类机器上失败。
func TestLinuxPushScriptSucceedsWithoutSELinuxTooling(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "app.conf")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o644))

	out, err := runPushScript(t, target, "new content", "644", 5)
	require.NoError(t, err, "script output: %s", out)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(got))

	assertNoTempLeftover(t, dir)
}

// currentOwner 返回当前进程的用户名与组名。
// 非 root 下 chown 到自身 uid/gid 是允许的，因此脚本执行类用例无需 root 也能跑。
func currentOwner(t *testing.T) (string, string) {
	t.Helper()

	u, err := user.Current()
	require.NoError(t, err)

	g, err := user.LookupGroupId(u.Gid)
	require.NoError(t, err)

	return u.Username, g.Name
}

// runPushScript 生成并执行下发脚本，返回脚本输出
func runPushScript(t *testing.T, targetPath, content, fileMode string, maxBackups int) (string, error) {
	t.Helper()

	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	owner, group := currentOwner(t)
	builder := &ScriptBuilder{FileMode: table.Unix, MaxBackups: maxBackups}
	script, err := builder.BuildConfigPushScript(
		base64.StdEncoding.EncodeToString([]byte(content)), targetPath, fileMode, owner, group)
	require.NoError(t, err)

	scriptPath := filepath.Join(t.TempDir(), "push.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o700))

	out, err := exec.Command("bash", scriptPath).CombinedOutput()
	return string(out), err
}

// TestLinuxPushScriptReplacesReadOnlyFile 覆盖故障场景：目标文件为 444 只读。
// 旧实现用 `> "$TARGET_PATH"` 直写，会被文件的只读位挡住；原子替换只改目录项，因此能成功。
func TestLinuxPushScriptReplacesReadOnlyFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "app.conf")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o444))

	out, err := runPushScript(t, target, "new content", "444", 5)
	require.NoError(t, err, "script output: %s", out)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(got))

	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o444), info.Mode().Perm())

	assertNoTempLeftover(t, dir)
}

// TestLinuxPushScriptFollowsSymlink 目标为软链接时应更新其指向的真实文件，且软链接本身保留
func TestLinuxPushScriptFollowsSymlink(t *testing.T) {
	dir := t.TempDir()
	realPath := filepath.Join(dir, "app.conf.real")
	linkPath := filepath.Join(dir, "app.conf")
	require.NoError(t, os.WriteFile(realPath, []byte("old"), 0o644))
	require.NoError(t, os.Symlink(realPath, linkPath))

	out, err := runPushScript(t, linkPath, "new content", "644", 5)
	require.NoError(t, err, "script output: %s", out)

	info, err := os.Lstat(linkPath)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink, "symlink should be preserved, not replaced by a regular file")

	got, err := os.ReadFile(realPath)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(got))
}

// TestLinuxPushScriptKeepsMaxBackups 备份份数超过上限后应删除最旧的
func TestLinuxPushScriptKeepsMaxBackups(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "app.conf")
	require.NoError(t, os.WriteFile(target, []byte("v0"), 0o644))

	const maxBackups = 2
	for i := range maxBackups + 2 {
		if i > 0 {
			// 备份文件名按秒取时间戳，同一秒内的多次下发会复用同一个文件名
			time.Sleep(1100 * time.Millisecond)
		}
		out, err := runPushScript(t, target, "v"+strconv.Itoa(i+1), "644", maxBackups)
		require.NoError(t, err, "script output: %s", out)
	}

	backups, err := filepath.Glob(filepath.Join(dir, "app.conf.*.bak"))
	require.NoError(t, err)
	assert.Len(t, backups, maxBackups)
}

// TestLinuxPushScriptKeepsTargetIntactOnChownFailure 属主无法解析时，已存在的目标文件必须
// 保持原状：既不能被覆盖，也不能留下临时文件或多出一份备份。前置校验在备份之前就退出，
// 即便校验被绕过（比如账号解析在校验之后才坏掉），属主也只设置在临时文件上，目标同样不受影响。
func TestLinuxPushScriptKeepsTargetIntactOnChownFailure(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "app.conf")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o644))

	builder := &ScriptBuilder{FileMode: table.Unix, MaxBackups: 5}
	script, err := builder.BuildConfigPushScript(
		base64.StdEncoding.EncodeToString([]byte("new content")), target, "644",
		"bscp-user-does-not-exist", "bscp-group-does-not-exist")
	require.NoError(t, err)

	scriptPath := filepath.Join(t.TempDir(), "push.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o700))

	out, err := exec.Command("bash", scriptPath).CombinedOutput()
	require.Error(t, err, "script should fail when the owner does not exist, output: %s", out)

	got, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, "old", string(got), "target file must keep its original content")

	assertNoTempLeftover(t, dir)
}

// TestBuildWindowsPushScriptUsesUniqueTempPath 同一台主机上多个任务下发同名配置很常见
// （若干个 .NET 服务各有一份 Web.config，只是目录不同）。这些任务曾经共用
// %TEMP%\bscp_<文件名>_%RANDOM% 这一个命名空间，而 cmd 的 %RANDOM% 在进程启动时以系统时钟
// 播种，被 GSE 同时拉起的脚本会取到相同的值：它们互相覆盖 base64 中转文件、互相删除解码
// 产物，表现为 certutil 报 ERROR_FILE_EXISTS 或 move 找不到文件，甚至静默写入他人的内容。
func TestBuildWindowsPushScriptUsesUniqueTempPath(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Windows}
	build := func(absPath string) string {
		script, err := builder.BuildConfigPushScript(
			"Y29udGVudA==", absPath, "", "Administrator", "Administrators")
		require.NoError(t, err)
		return script
	}

	first := build(`D:\bdo\Tmp\GB.BlackDesert.Trade.Web\Web.config`)
	second := build(`D:\bdo\Tmp\GB.BlackDesert.Trade.Web.Game\Web.config`)
	// 同一目标路径重复构建（重试、或同一配置并发下发）同样不能复用临时文件名
	third := build(`D:\bdo\Tmp\GB.BlackDesert.Trade.Web\Web.config`)

	assert.NotContains(t, first, "%RANDOM%", "唯一性不能依赖 cmd 的 %RANDOM%")
	assert.NotContains(t, first, "%TEMP%",
		"临时文件必须与目标同目录：%TEMP% 全机共享，且跨卷 move 不是原子操作")

	firstTmp, firstOut := windowsTempPaths(t, first)
	secondTmp, secondOut := windowsTempPaths(t, second)
	thirdTmp, thirdOut := windowsTempPaths(t, third)

	assert.Contains(t, firstTmp, `!TARGET_DIR!!TARGET_NAME!.`, "临时文件应落在目标同目录")
	assert.NotEqual(t, firstTmp, firstOut, "中转文件与解码产物不能同名")
	assert.NotEqual(t, firstTmp, secondTmp)
	assert.NotEqual(t, firstOut, secondOut)
	assert.NotEqual(t, firstTmp, thirdTmp)
	assert.NotEqual(t, firstOut, thirdOut)
}

// TestBuildWindowsPushScriptGuardsDecodeAndMkdir 覆盖两处会把错误藏起来的地方：
// certutil 不带 -f 时拒绝覆盖已存在的输出文件；mkdir 失败若不拦截，错误会一路滑到 move。
func TestBuildWindowsPushScriptGuardsDecodeAndMkdir(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Windows}

	script, err := builder.BuildConfigPushScript(
		"Y29udGVudA==", `D:\bdo\Tmp\Web_IDIP_API\Web.config`, "", "Administrator", "Administrators")
	require.NoError(t, err)

	assert.Contains(t, script, `certutil -f -decode "!BSCP_TMP!" "!BSCP_OUT!"`)
	assert.NotContains(t, script, `certutil -decode`, "缺少 -f 会报 0x80070050 ERROR_FILE_EXISTS")

	mkdirAt := strings.Index(script, "MKDIR_FAILED")
	decodeAt := strings.Index(script, "certutil -f -decode")
	require.NotEqual(t, -1, mkdirAt, "建目录失败必须当场终止")
	require.NotEqual(t, -1, decodeAt)
	assert.Less(t, mkdirAt, decodeAt)
}

// TestBuildWindowsPushScriptChownsOnlyNewDirs 与 Linux 侧同一个约束：目录归属只能作用于
// 本次 mkdir 新建出来的层级。cmd 的 mkdir 同样是递归的，建完就分不清哪几层是新建的，
// 因此必须在 mkdir 之前把待创建的层级记下来。
func TestBuildWindowsPushScriptChownsOnlyNewDirs(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Windows, MaxBackups: 5}

	script, err := builder.BuildConfigPushScript(
		"Y29udGVudA==", `D:\app\conf\sub\app.conf`, "", "appuser", "appgroup")
	require.NoError(t, err)

	// 逐级上溯收集待创建层级，遇到已存在的目录即停止
	assert.Contains(t, script, `if exist "!DIR_CUR!\" goto collect_new_dirs_done`)
	assert.Contains(t, script, `set "NEW_DIR_!NEW_DIR_COUNT!=!DIR_CUR!"`)
	// 上溯到驱动器根后父目录不再变化，靠这个条件收尾；缺了它路径异常时会死循环
	assert.Contains(t, script, `if "!DIR_PARENT!"=="!DIR_CUR!" goto collect_new_dirs_done`)

	// Windows 下属主不带访问权，必须显式授权，否则该账号进不了自己的配置目录
	assert.Contains(t, script, `icacls "!NEW_DIR!" /setowner "!BSCP_OWNER!"`)
	assert.Contains(t, script, `icacls "!NEW_DIR!" /grant:r "!BSCP_OWNER!:(F)"`)
	// 目录的遍历需要 execute 权限，属组必须是 RX 而不是文件那边的 R；
	// 属组不再下发，固定用服务端注入的内置管理员组 TARGET_GROUP
	assert.Contains(t, script, `icacls "!NEW_DIR!" /grant:r "!TARGET_GROUP!:(RX)"`)
	assert.NotContains(t, script, "appgroup", "属组不应再取自下发参数")

	// 不得带继承标记：否则目录内后续创建的文件都会被套上这套权限。
	// 逐条命令核对而不是全文搜索，避免被注释里的字面量骗过
	for _, line := range strings.Split(script, "\n") {
		cmd := strings.TrimSpace(line)
		if !strings.HasPrefix(cmd, "icacls ") {
			continue
		}
		assert.NotContains(t, cmd, "(OI)", "icacls 不应带对象继承标记: %s", cmd)
		assert.NotContains(t, cmd, "(CI)", "icacls 不应带容器继承标记: %s", cmd)
	}

	orderOf := func(needle string) int {
		idx := strings.Index(script, needle)
		require.NotEqual(t, -1, idx, "script should contain %q", needle)
		return idx
	}
	// 层级必须在 mkdir 之前收集，否则收集到的永远是空集
	assert.Less(t, orderOf(":collect_new_dirs"), orderOf(`mkdir "!TARGET_DIR!"`))
	assert.Less(t, orderOf(`mkdir "!TARGET_DIR!"`), orderOf(`icacls "!NEW_DIR!" /setowner`))
	// 目录授权要早于备份：备份文件就落在这个目录里
	assert.Less(t, orderOf(`icacls "!NEW_DIR!" /setowner`), orderOf("BACKUP_FULL_PATH="))
}

// TestBuildWindowsPushScriptDirAclFailureAbortsAndCleansUp 目录 icacls 失败若只打 WARN
// 就继续下发：mkdir 已经把目录建出来，下次 NEW_DIR_COUNT=0，永远不会再修 ACL，
// 任务却一直报成功，模板账号进不了自己的配置目录。必须当场失败，并删掉本次新建的空目录。
func TestBuildWindowsPushScriptDirAclFailureAbortsAndCleansUp(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Windows, MaxBackups: 5}

	script, err := builder.BuildConfigPushScript(
		"Y29udGVudA==", `D:\app\conf\sub\app.conf`, "", "appuser", "appgroup")
	require.NoError(t, err)

	assert.NotContains(t, script, `echo [WARN] icacls /setowner failed on !NEW_DIR!`,
		"目录授权失败不能再只打 WARN")
	assert.NotContains(t, script, `echo [WARN] icacls grant owner failed on !NEW_DIR!`)
	assert.NotContains(t, script, `echo [WARN] icacls grant group failed on !NEW_DIR!`)

	assert.Contains(t, script, "ACL_FAILED")
	assert.Contains(t, script, "goto cleanup_new_dirs")
	assert.Contains(t, script, ":cleanup_new_dirs")
	assert.Contains(t, script, `rmdir "!NEW_DIR!"`)
	assert.Contains(t, script, `exit /b 1`)

	// 成功路径必须跳过清理标签，否则正常下发也会把刚建的目录删掉
	assert.Contains(t, script, "goto :eof")
	eofAt := strings.Index(script, "goto :eof")
	cleanupAt := strings.Index(script, ":cleanup_new_dirs")
	require.NotEqual(t, -1, eofAt)
	require.NotEqual(t, -1, cleanupAt)
	assert.Less(t, eofAt, cleanupAt)
}

// TestBuildWindowsPushScriptValidatesAccountsFirst 写盘前用 NTAccount 解析属主，
// 语义对齐 Linux 侧的 id -u。同目录并发时所有任务都会在预检失败，不会 mkdir，
// 已存在的 conf 也不会被改属主。属组固定为内置管理员组常量，不需要预检。
func TestBuildWindowsPushScriptValidatesAccountsFirst(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Windows, MaxBackups: 5}

	script, err := builder.BuildConfigPushScript(
		"Y29udGVudA==", `D:\app\conf\sub\app.conf`, "", "appuser", "appgroup")
	require.NoError(t, err)

	assert.Contains(t, script, "OWNER_NOT_FOUND")
	assert.Contains(t, script, `set "BSCP_OWNER=appuser"`)
	assert.Contains(t, script, `[System.Security.Principal.NTAccount]$env:BSCP_OWNER`)
	// 属组不再下发，由服务端常量注入脚本，也没有单独的组解析/预检；
	// 注入的是内置管理员组的稳定 SID，组名本地化/重命名的主机也能解析
	assert.Contains(t, script, `set "TARGET_GROUP=*S-1-5-32-544"`)
	assert.NotContains(t, script, `'appgroup'`, "属组不应再单独解析下发参数")
	assert.NotContains(t, script, `icacls "!TARGET_DIR!" /setowner`,
		"已存在的配置目录不能每次都被改属主")

	orderOf := func(needle string) int {
		idx := strings.Index(script, needle)
		require.NotEqual(t, -1, idx, "script should contain %q", needle)
		return idx
	}
	assert.Less(t, orderOf("OWNER_NOT_FOUND"), orderOf(`mkdir "!TARGET_DIR!"`))
}

// TestBuildWindowsPushScriptFileAclFailureAborts 文件级 icacls 若只 WARN，目录已在时
// 坏属主仍会写出配置并报成功。必须失败退出。
func TestBuildWindowsPushScriptFileAclFailureAborts(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Windows, MaxBackups: 5}

	script, err := builder.BuildConfigPushScript(
		"Y29udGVudA==", `D:\app\conf\sub\app.conf`, "", "appuser", "appgroup")
	require.NoError(t, err)

	assert.NotContains(t, script, `echo [WARN] icacls /setowner failed, errorlevel=`)
	assert.NotContains(t, script, `echo [WARN] icacls grant owner full control failed`)
	assert.NotContains(t, script, `echo [WARN] icacls grant group read failed`)
	assert.Contains(t, script, `icacls "%TARGET_PATH%" /setowner "!BSCP_OWNER!"`)
	assert.Contains(t, script, `icacls "%TARGET_PATH%" /grant:r "!TARGET_GROUP!:(R)"`)
	assert.Contains(t, script, "[ERROR] icacls /setowner failed, errorlevel=")
}

// TestBuildWindowsPushScriptEscapesBatchTokens cmd 的 %%i / %%n 与 %VAR% 都要经 fmt.Sprintf
// 转义，漏一层就会生成语法错误或把变量名当成格式动词，而这类问题在 Go 侧不会报错。
func TestBuildWindowsPushScriptEscapesBatchTokens(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Windows, MaxBackups: 5}

	script, err := builder.BuildConfigPushScript(
		"Y29udGVudA==", `D:\app\conf\app.conf`, "", "appuser", "appgroup")
	require.NoError(t, err)

	assert.Contains(t, script, `for %%i in ("!DIR_CUR!") do set "DIR_PARENT=%%~dpi"`)
	assert.Contains(t, script, `for /l %%n in (1,1,!NEW_DIR_COUNT!) do (`)
	assert.Contains(t, script, `set "NEW_DIR=!NEW_DIR_%%n!"`)
	// 转义漏了会退化成单个 %，cmd 会把 %i / %n 当成未定义变量而展开为空
	assert.NotContains(t, script, `for %i in`)
	assert.NotContains(t, script, `for /l %n in`)
}

// TestBuildWindowsPushScriptChunksBase64 base64 曾经整份写在一行 `echo <b64> > "!BSCP_TMP!"` 里。
// cmd.exe 解析批处理时单行有约 8191 字符的上限，超出部分被静默丢弃，配置超过 6 KB 左右就会被截断；
// 而 certutil 会把残缺的 base64 照样解码出一个语法上看起来正常的文件，move 与退出码全部正常，
// 下发任务报成功，只有人工比对配置才能发现机器上少了一大段。
func TestBuildWindowsPushScriptChunksBase64(t *testing.T) {
	// 取一份远超单行上限的配置：64 KB 原始内容对应约 87 KB base64
	raw := strings.Repeat("<add key=\"k\" value=\"v\" />\n", 2621)
	require.Greater(t, len(raw), 64*1024)
	b64 := base64.StdEncoding.EncodeToString([]byte(raw))

	builder := &ScriptBuilder{FileMode: table.Windows}
	script, err := builder.BuildConfigPushScript(
		b64, `D:\bdo\Tmp\GB.BlackDesert.Trade.Web\Web.config`, "", "Administrator", "Administrators")
	require.NoError(t, err)

	// cmd 会把超长行截断，脚本里任何一行都不允许触到上限
	for i, line := range strings.Split(script, "\n") {
		assert.LessOrEqual(t, len(line), 8191,
			"第 %d 行触到 cmd 的单行上限，内容会被静默截断", i+1)
	}

	writes := windowsB64WriteRe.FindAllStringSubmatch(script, -1)
	require.NotEmpty(t, writes, "base64 必须按行写入中转文件")

	var rebuilt strings.Builder
	for _, m := range writes {
		// 切分点必须落在 base64 四字符组的边界上，否则 certutil 解不出原字节
		assert.Zero(t, len(m[1])%4, "每行 base64 长度必须是 4 的倍数")
		rebuilt.WriteString(m[1])
	}
	assert.Equal(t, b64, rebuilt.String(), "逐行拼回来必须与原 base64 完全一致，不能有丢失")

	// 重定向必须在 echo 之前：`echo x > f` 会把 `>` 前的空格写进文件，而且 base64 含数字时
	// `echo ...1>f` 的末位数字会被 cmd 当成重定向句柄吃掉，两种情况都会静默改变内容
	assert.NotRegexp(t, `echo [A-Za-z0-9+/=]+ *>`, script,
		"base64 行不能把重定向写在行尾")
}

// TestBuildWindowsPushScriptHandlesEmptyContent 空内容也必须把中转文件建出来，
// 否则后面的存在性检查会把它报成含义完全不同的 WRITE_TMP_FAILED。
func TestBuildWindowsPushScriptHandlesEmptyContent(t *testing.T) {
	builder := &ScriptBuilder{FileMode: table.Windows}
	script, err := builder.BuildConfigPushScript(
		"", `D:\bdo\Tmp\Web_IDIP_API\Web.config`, "", "Administrator", "Administrators")
	require.NoError(t, err)

	assert.Contains(t, script, `>>"!BSCP_TMP!" echo.`)
}

var windowsB64WriteRe = regexp.MustCompile(`(?m)^>>"!BSCP_TMP!" echo ([A-Za-z0-9+/=]+)$`)

var windowsTempVarRe = regexp.MustCompile(`(?m)^set "(BSCP_TMP|BSCP_OUT)=(.+)"$`)

// windowsTempPaths 取出脚本里声明的 base64 中转文件与解码产物路径
func windowsTempPaths(t *testing.T, script string) (string, string) {
	t.Helper()

	paths := make(map[string]string, 2)
	for _, m := range windowsTempVarRe.FindAllStringSubmatch(script, -1) {
		paths[m[1]] = m[2]
	}
	require.Len(t, paths, 2, "script should declare both BSCP_TMP and BSCP_OUT")

	return paths["BSCP_TMP"], paths["BSCP_OUT"]
}

func assertNoTempLeftover(t *testing.T, dir string) {
	t.Helper()

	leftovers, err := filepath.Glob(filepath.Join(dir, "*.tmp.*"))
	require.NoError(t, err)
	assert.Empty(t, leftovers, "temporary files must be cleaned up")
}
