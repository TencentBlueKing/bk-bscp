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

package dao

import (
	"strings"
	"testing"
	"time"

	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// TestFailureDetailQueryJoinByBatch 锁定失败明细的查询形态：
// 批次表做驱动表、join 任务记录表，不能退化成「先取批次 ID 再 IN 回查」，
// 因为批次与任务是一对多，分片 IN 后无法得到跨批次的全局有序分页。
func TestFailureDetailQueryJoinByBatch(t *testing.T) {
	tx, _ := newDryRunQueryTx(t)
	d := &taskRecordDao{genQ: tx.Query}

	start := time.Now().Add(-24 * time.Hour)
	query := d.failureDetailQuery(kit.New(), &TaskFailureFilter{
		TenantID:        "default",
		TaskIndexType:   "task_batch",
		TaskObjects:     []string{"process"},
		BatchStatuses:   []string{"failed"},
		BatchStartTime:  &start,
		Creators:        []string{"admin"},
		ExcludeStatuses: []string{"SUCCESS", "IGNORED"},
	})

	var details []*TaskFailureRow
	db := query.Select(failureRowColumns).Order("r.created_at DESC, r.id DESC").Limit(10).
		Find(&details)
	if db.Error != nil {
		t.Fatalf("build failure detail sql failed: %v", db.Error)
	}

	got := db.Statement.SQL.String()
	for _, want := range []string{
		"JOIN task_records AS r ON r.task_index = CAST(b.id AS CHAR) COLLATE utf8mb4_unicode_ci" +
			" AND r.task_index_type = ?",
		"r.deleted_at IS NULL",
		"b.tenant_id IN",
		"b.task_object IN",
		"b.status IN",
		"b.created_at >= ?",
		"b.creator IN",
		"r.status NOT IN",
		"b.biz_id",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("sql should contain %q, got: %s", want, got)
		}
	}

	if strings.Contains(got, "task_index IN") {
		t.Fatalf("failure detail must not use batch id IN list, got: %s", got)
	}
}
