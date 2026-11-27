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

package taskerr

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	info := Info{Code: 1015000, Msg: `bind() to 0.0.0.0:8080 failed`}

	encoded := Encode(info)
	if !strings.HasPrefix(encoded, SentinelBegin) || !strings.HasSuffix(encoded, SentinelEnd) {
		t.Fatalf("encoded message missing sentinel: %s", encoded)
	}

	got, ok := Decode("prefix " + encoded + ", retried=1")
	if !ok {
		t.Fatalf("decode failed: %s", encoded)
	}
	if got != info {
		t.Fatalf("decode mismatch, want %+v, got %+v", info, got)
	}
}

func TestErrorKeepsCauseAndInfo(t *testing.T) {
	cause := errors.New("[Operate STEP]: process operate failed, task_id=task-1")
	err := New(1015000, WithMsg("bind() failed")).WithCause(cause)

	msg := err.Error()
	if !strings.HasPrefix(msg, SentinelBegin) {
		t.Fatalf("structured segment should be at the front: %s", msg)
	}
	if !strings.Contains(msg, cause.Error()) {
		t.Fatalf("cause lost in error text: %s", msg)
	}

	info, ok := Decode(msg)
	if !ok {
		t.Fatalf("decode failed: %s", msg)
	}
	if info.Code != 1015000 || info.Msg != "bind() failed" {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestProcessMismatchCode(t *testing.T) {
	info := New(CodeProcessMismatch, WithMsg("compare process config failed")).Info()
	if info.Code != CodeProcessMismatch {
		t.Fatalf("unexpected code: %d", info.Code)
	}
}

func TestMsgTruncatedAtLimit(t *testing.T) {
	info := New(CodeProcessMismatch, WithMsg(strings.Repeat("a", maxMsgLen+10))).Info()
	if len(info.Msg) != maxMsgLen {
		t.Fatalf("expect msg truncated to %d bytes, got %d", maxMsgLen, len(info.Msg))
	}

	msg := strings.Repeat("配", 200)
	err := New(1, WithMsg(msg))

	info, ok := Decode(err.Error())
	if !ok {
		t.Fatalf("decode failed: %s", err.Error())
	}
	if info.Msg != msg {
		t.Fatalf("msg should not be truncated, got len=%d, want len=%d", len(info.Msg), len(msg))
	}
}

func TestTruncateUTF8Boundary(t *testing.T) {
	// 每个汉字 3 字节，7 不是 3 的整数倍，截断必须回退到合法边界。
	got := truncate(strings.Repeat("配", 10), 7)
	if len(got) != 6 {
		t.Fatalf("unexpected length: %d", len(got))
	}
	if !utf8.ValidString(got) {
		t.Fatalf("truncated string is not valid utf8: %q", got)
	}
	if truncate("abc", 0) != "abc" {
		t.Fatalf("max<=0 should not truncate")
	}
}

func TestMsgSentinelStripped(t *testing.T) {
	err := New(1, WithMsg("bad "+SentinelEnd+" value "+SentinelBegin+" here"))

	msg := err.Error()
	if strings.Count(msg, SentinelBegin) != 1 || strings.Count(msg, SentinelEnd) != 1 {
		t.Fatalf("sentinel not stripped from msg: %s", msg)
	}
	if _, ok := Decode(msg); !ok {
		t.Fatalf("decode failed after sanitize: %s", msg)
	}
}

func TestDecodeHistoricalSrcIgnored(t *testing.T) {
	// 改造前的结构段带 src 字段，读侧只取错误码，未知字段直接忽略。
	msg := SentinelBegin + `{"src":"gse","err_code":0}` + SentinelEnd
	info, ok := Decode(msg)
	if !ok {
		t.Fatalf("decode failed: %s", msg)
	}
	if info.Code != 0 {
		t.Fatalf("unexpected code: %d", info.Code)
	}
}

func TestDecodeFailure(t *testing.T) {
	cases := []string{
		"",
		"no sentinel here",
		SentinelBegin + `{"code":"bad"}` + SentinelEnd,
		SentinelBegin + "not-json" + SentinelEnd,
	}
	for _, c := range cases {
		if _, ok := Decode(c); ok {
			t.Fatalf("expect decode failure: %s", c)
		}
	}
}

func TestReadable(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    string
	}{
		{
			name: "framework prefix and structured section stripped, retry info kept",
			message: "step OperateProcess running failed, err=" +
				New(CodeProcessMismatch, WithMsg("compare process config failed")).Error() +
				" process config differs between db and cmdb, retried=1, maxRetries=3",
			want: "process config differs between db and cmdb（已重试 1/3 次）",
		},
		{
			name:    "retry info kept when only framework text exists",
			message: "step OperateProcess running failed, err=, retried=2, maxRetries=5",
			want:    "（已重试 2/5 次）",
		},
		{
			name: "retry info after structured section without step prefix",
			message: "err=" + New(1015000, WithMsg("invalid user(admin) with logon error")).Error() +
				" retried=1, maxRetries=3",
			want: "invalid user(admin) with logon error（已重试 1/3 次）",
		},
		{
			name:    "retry info without leading space",
			message: "err=" + New(1015000, WithMsg("timeout")).Error() + ", retried=2, maxRetries=5",
			want:    "timeout（已重试 2/5 次）",
		},
		{
			name:    "structured section only falls back to msg",
			message: New(CodeProcessCmdNotConfigured, WithMsg("the start command does not exist")).Error(),
			want:    "the start command does not exist",
		},
		{
			name:    "plain message kept",
			message: "task exec timeout",
			want:    "task exec timeout",
		},
	}
	for _, c := range cases {
		if got := Readable(c.message); got != c.want {
			t.Fatalf("%s: Readable() = %q, want %q", c.name, got, c.want)
		}
	}
}
