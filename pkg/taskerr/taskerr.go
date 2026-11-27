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

// Package taskerr 提供任务步骤层的结构化错误记录。
//
// 约定（详见 docs/reqs/进程操作任务统计.md 3.4）：
//   - 结构化段只记录错误码，不记录来源；归属由码表决定，
//     BSCP 自有码（见下方码表）归 self，其余非 0 码视为外部平台（gse）原始码；
//   - 拿不到错误码时不写结构化段，读侧直接按 BSCP 自身失败处理；
//   - msg 为错误摘要，写入前截断，仅用于排障展示。
//
// 结构化段以 [BSCP-ERR]...[/BSCP-ERR] 双哨兵包裹，**置于错误文本最前面**，
// 由任务框架原样写入 task_records.message，读侧通过 Decode 提取。
// 前置的原因：message 为 text 列，超长时尾部会被静默截断，段在前可保证归因不丢。
package taskerr

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// SentinelBegin 结构化段起始哨兵。
	SentinelBegin = "[BSCP-ERR]"
	// SentinelEnd 结构化段结束哨兵。
	SentinelEnd = "[/BSCP-ERR]"
)

// BSCP 自有错误码。
// 这里登记的都是 BSCP 自身产生的码；读侧按本表判定归属，
// 未登记的非 0 码一律视为外部平台（GSE）原始码。
const (
	// CodeProcessMismatch 执行的进程与实际绑定的业务信息不匹配，
	// 建议执行前先核对进程配置信息与 CMDB 是否一致。
	CodeProcessMismatch int32 = 4100500
	// CodeProcessCmdNotConfigured 操作所需的命令未配置（start_cmd / stop_cmd 等为空），
	// 进程配置不完整，无法执行该操作。
	CodeProcessCmdNotConfigured int32 = 4100501
	// CodeProcessStageBlocked 优先级级联阻断：该优先级阶段已有进程操作失败，
	// 优先级低于（或高于，取决于操作排序方向）此的进程操作不会被继续执行。
	CodeProcessStageBlocked int32 = 4103003
)

// maxMsgLen 为 msg 字段入库前的最大字节数。
// task_records.message 为 text（65535 字节），结构段之外还有原始错误文本，
// 这里留足余量取 8KB；truncate 会保证 UTF-8 边界安全。
const maxMsgLen = 8192

// Info 是写入 task_records.message 的结构化错误信息。
type Info struct {
	// Code 错误码：BSCP 自有码见上方码表，其余为外部平台原始码。
	Code int32 `json:"code"`
	// Msg 错误摘要，可选。
	Msg string `json:"msg,omitempty"`
}

// Option 用于自定义 Info。
type Option func(*Info)

// WithMsg 设置错误摘要，超长时按字节截断并保证不破坏 UTF-8。
func WithMsg(msg string) Option {
	return func(info *Info) {
		info.Msg = truncate(msg, maxMsgLen)
	}
}

// Error 携带结构化信息的任务步骤错误。
type Error struct {
	info  Info
	cause error
}

// New 构造带错误码的结构化错误。
func New(code int32, opts ...Option) *Error {
	info := Info{Code: code}
	for _, opt := range opts {
		opt(&info)
	}
	return &Error{info: info}
}

// WithCause 附加底层原因，输出时保留在结构化段之前。
func (e *Error) WithCause(cause error) *Error {
	if e == nil {
		return nil
	}
	e.cause = cause
	return e
}

// Error 实现 error 接口，格式为「结构化段 原始错误」。
// 结构化段前置：task_records.message 为 text，超长时 MySQL 会静默截断尾部，
// 段放在最前面可保证归因信息不随原文截断而丢失。
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	body := Encode(e.info)
	if e.cause == nil {
		return body
	}
	return body + " " + e.cause.Error()
}

// Unwrap 返回底层原因。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Info 返回结构化信息副本。
func (e *Error) Info() Info {
	if e == nil {
		return Info{}
	}
	return e.info
}

// Encode 将 Info 编码为带双哨兵的字符串。
func Encode(info Info) string {
	info.Msg = sanitize(info.Msg)

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(info); err != nil {
		return SentinelBegin + SentinelEnd
	}
	return SentinelBegin + strings.TrimRight(buf.String(), "\n") + SentinelEnd
}

// Decode 从 message 中提取结构化信息；不存在或解析失败时返回 false。
// 兼容改造前带 src 字段的历史数据：未知字段被忽略，只取错误码。
func Decode(message string) (Info, bool) {
	info := Info{}

	begin := strings.Index(message, SentinelBegin)
	if begin < 0 {
		return info, false
	}
	begin += len(SentinelBegin)

	offset := strings.Index(message[begin:], SentinelEnd)
	if offset < 0 {
		return info, false
	}

	if err := json.Unmarshal([]byte(message[begin:begin+offset]), &info); err != nil {
		return Info{}, false
	}
	return info, true
}

var (
	// errSectionRegexp 匹配结构化错误段，用于从展示文本中剥离。
	errSectionRegexp = regexp.MustCompile(regexp.QuoteMeta(SentinelBegin) + `.*?` + regexp.QuoteMeta(SentinelEnd))
	// frameworkPrefixRegexp 匹配框架失败模板的前缀：step <name> running failed, err=
	// 也兼容只带 err= 的写法：结构段被剥离后，正文可能只剩 err= 开头。
	frameworkPrefixRegexp = regexp.MustCompile(`^(task \S+ )?(step .*? running failed, )?err=`)
	// frameworkRetryRegexp 匹配框架追加的重试信息，捕获已重试次数与最大重试次数。
	// 前导分隔符可有可无：历史数据里存在 "[BSCP-ERR]xxx[/BSCP-ERR] retried=1, maxRetries=3"。
	frameworkRetryRegexp = regexp.MustCompile(`[\s,;]*retried=(\d+),\s*maxRetries=(\d+)$`)
)

// Readable 返回适合直接展示给用户的错误文本：
// 去掉 [BSCP-ERR] 结构段与框架的 "step xxx running failed, err=" 前缀；
// 框架追加的 ", retried=N, maxRetries=M" 不丢弃，转成末尾的「（已重试 N/M 次）」保留排障信息；
// 剥离后为空时回退到结构段的 msg。
func Readable(message string) string {
	display := strings.TrimSpace(errSectionRegexp.ReplaceAllString(message, ""))
	if loc := frameworkPrefixRegexp.FindStringIndex(display); loc != nil {
		display = strings.TrimSpace(display[loc[1]:])
	}

	retrySuffix := ""
	if matches := frameworkRetryRegexp.FindStringSubmatch(display); len(matches) == 3 {
		display = strings.TrimSpace(frameworkRetryRegexp.ReplaceAllString(display, ""))
		retrySuffix = "（已重试 " + matches[1] + "/" + matches[2] + " 次）"
	}

	if display == "" {
		if info, ok := Decode(message); ok && info.Msg != "" {
			display = info.Msg
		}
	}
	return display + retrySuffix
}

// sanitize 移除 msg 中的哨兵字符，避免提前闭合结构段。
func sanitize(msg string) string {
	if msg == "" {
		return ""
	}
	msg = strings.ReplaceAll(msg, SentinelBegin, "")
	return strings.ReplaceAll(msg, SentinelEnd, "")
}

// truncate 按字节截断，并回退到合法的 UTF-8 边界；max <= 0 表示不限制。
func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	s = s[:max]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
