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

package process

import (
	"testing"

	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
)

// TestNeedUnregisterProcess 清除链路的取消托管判定：
// GSE 返回的实例中任一 isauto 为真即视为已托管，需要取消托管
func TestNeedUnregisterProcess(t *testing.T) {
	cases := []struct {
		name   string
		status *gse.ProcessStatusContent
		want   bool
	}{
		{"状态为空 -> 无需取消托管", nil, false},
		{"无进程信息 -> 无需取消托管", &gse.ProcessStatusContent{}, false},
		{"实例未托管 -> 无需取消托管", &gse.ProcessStatusContent{
			Process: []gse.ProcessDetail{
				{Instance: []gse.ProcessInstance{{IsAuto: false, PID: 100}}},
			},
		}, false},
		{"实例已托管 -> 需要取消托管", &gse.ProcessStatusContent{
			Process: []gse.ProcessDetail{
				{Instance: []gse.ProcessInstance{{IsAuto: true, PID: 100}}},
			},
		}, true},
		{"多实例任一托管 -> 需要取消托管", &gse.ProcessStatusContent{
			Process: []gse.ProcessDetail{
				{Instance: []gse.ProcessInstance{{IsAuto: false}, {IsAuto: true}}},
			},
		}, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := needUnregisterProcess(c.status); got != c.want {
				t.Fatalf("needUnregisterProcess() = %v, want %v", got, c.want)
			}
		})
	}
}
