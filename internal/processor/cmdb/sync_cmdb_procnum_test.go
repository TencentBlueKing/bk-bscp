/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmdb

import (
	"testing"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// TestNormalizeProcNum 校验 CMDB proc_num 为 0 时归一化为 1
func TestNormalizeProcNum(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want uint
	}{
		{"zero to one", 0, 1},
		{"one stays one", 1, 1},
		{"two stays two", 2, 2},
		{"large value", 100, 100},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := normalizeProcNum(c.in); got != c.want {
				t.Fatalf("normalizeProcNum(%d) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

// TestBuildProcessEntitiesNormalizesProcNum 校验 buildProcessEntities（新同步链路）
// 在 CMDB proc_num 为 0 时归一化为 1
func TestBuildProcessEntitiesNormalizesProcNum(t *testing.T) {
	svc := &osTypeStubCMDB{}
	s := &syncCMDBService{bizID: 3, svc: svc}
	kt := kit.New()

	item := newOsTypeProcessItem(0, "127.0.0.1")
	item.Process.ProcNum = 0
	data := []*bkcmdb.ProcessRelatedInfoItem{item}

	procs := s.buildProcessEntities(kt, data, "default")

	if len(procs) != 1 {
		t.Fatalf("processes count = %d, want 1", len(procs))
	}
	if got := procs[0].Spec.ProcNum; got != 1 {
		t.Fatalf("ProcNum = %d, want 1 (normalized from 0)", got)
	}
}
