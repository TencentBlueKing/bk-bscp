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

package service

import (
	"testing"
)

// TestRetryLatestConfigData 校验重试刷新 LatestConfigData 的合并语义：
// 刷新成功但未命中（进程已在 CMDB 删除）必须清空旧快照，仅刷新降级（nil）时保留原值。
func TestRetryLatestConfigData(t *testing.T) {
	cases := map[string]struct {
		snapshot    map[uint32]string
		ccProcessID uint32
		original    string
		want        string
	}{
		"命中时采用 CMDB 最新配置": {
			snapshot:    map[uint32]string{7: `{"work_path":"/x"}`},
			ccProcessID: 7,
			original:    "stale",
			want:        `{"work_path":"/x"}`,
		},
		"刷新成功但未命中时清空旧快照": {
			snapshot:    map[uint32]string{8: `{"work_path":"/y"}`},
			ccProcessID: 7,
			original:    "stale",
			want:        "",
		},
		"全部进程已删除（空 map 且刷新成功）时清空旧快照": {
			snapshot:    map[uint32]string{},
			ccProcessID: 7,
			original:    "stale",
			want:        "",
		},
		"刷新降级（nil）时保留原值": {
			snapshot:    nil,
			ccProcessID: 7,
			original:    "stale",
			want:        "stale",
		},
		"原本无快照且未命中时保持为空": {
			snapshot:    map[uint32]string{8: "data"},
			ccProcessID: 7,
			original:    "",
			want:        "",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := retryLatestConfigData(c.snapshot, c.ccProcessID, c.original)
			if got != c.want {
				t.Fatalf("retryLatestConfigData(%v, %d, %q) = %q, want %q",
					c.snapshot, c.ccProcessID, c.original, got, c.want)
			}
		})
	}
}
