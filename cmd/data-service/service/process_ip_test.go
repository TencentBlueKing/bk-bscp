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
	"reflect"
	"testing"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	pbproc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
)

func proc(ip string) *table.Process {
	return &table.Process{Spec: &table.ProcessSpec{InnerIP: ip}}
}

func TestDedupInnerIPs(t *testing.T) {
	cases := []struct {
		name  string
		procs []*table.Process
		want  []string
	}{
		{
			name:  "空进程集合返回空列表（AC-004）",
			procs: nil,
			want:  []string{},
		},
		{
			name:  "同主机多进程内网IP去重且保序（AC-002/AC-003）",
			procs: []*table.Process{proc("127.0.0.1"), proc("127.0.0.2"), proc("127.0.0.1"), proc("127.0.0.3")},
			want:  []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"},
		},
		{
			name:  "跳过空内网IP",
			procs: []*table.Process{proc(""), proc("10.0.0.1"), proc("")},
			want:  []string{"10.0.0.1"},
		},
		{
			name:  "多于单页默认条数一次性全量返回（AC-006）",
			procs: manyProcs(25),
			want:  manyIPs(25),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := dedupInnerIPs(c.procs)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("dedupInnerIPs() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestValidateExpressionEnv(t *testing.T) {
	scope := &pbproc.ExpressionScope{SetName: "*"}

	cases := []struct {
		name    string
		search  *pbproc.ProcessSearchCondition
		wantErr bool
	}{
		{
			name:    "表达式范围缺环境类型报参数错误（AC-005）",
			search:  &pbproc.ProcessSearchCondition{ExpressionScope: scope},
			wantErr: true,
		},
		{
			name:    "表达式范围带环境类型通过",
			search:  &pbproc.ProcessSearchCondition{Environment: "3", ExpressionScope: scope},
			wantErr: false,
		},
		{
			name:    "无表达式范围不校验环境",
			search:  &pbproc.ProcessSearchCondition{},
			wantErr: false,
		},
		{
			name:    "search 为空不报错",
			search:  nil,
			wantErr: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateExpressionEnv(c.search)
			if c.wantErr {
				if err == nil {
					t.Fatalf("validateExpressionEnv() expected error, got nil")
				}
				ef, ok := err.(*errf.ErrorF)
				if !ok || ef.Code != errf.InvalidParameter {
					t.Fatalf("validateExpressionEnv() expected InvalidParameter, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateExpressionEnv() unexpected error: %v", err)
			}
		})
	}
}

func manyProcs(n int) []*table.Process {
	procs := make([]*table.Process, 0, n)
	for _, ip := range manyIPs(n) {
		procs = append(procs, proc(ip))
	}
	return procs
}

func manyIPs(n int) []string {
	ips := make([]string, 0, n)
	for i := 0; i < n; i++ {
		ips = append(ips, "10.0.0."+itoa(i))
	}
	return ips
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
