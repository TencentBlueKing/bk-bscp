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
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// mysqlMaxPlaceholders MySQL 单条 prepared statement 允许的最大占位符数量，超过会报 Error 1390
const mysqlMaxPlaceholders = 65535

// newDryRunQueryTx 构造只生成 SQL、不连接数据库的事务对象，并记录每条 INSERT 语句的占位符数量
func newDryRunQueryTx(t *testing.T) (*gen.QueryTx, *[]int) {
	t.Helper()

	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "user:pass@tcp(127.0.0.1:1)/bscp?parseTime=true",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open dry run db failed: %v", err)
	}

	varsPerStmt := make([]int, 0)
	if err := db.Callback().Create().After("gorm:create").Register("test:count_vars", func(d *gorm.DB) {
		varsPerStmt = append(varsPerStmt, len(d.Statement.Vars))
	}); err != nil {
		t.Fatalf("register callback failed: %v", err)
	}

	return &gen.QueryTx{Query: gen.Use(db)}, &varsPerStmt
}

func newTestProcesses(n int) []*table.Process {
	procs := make([]*table.Process, 0, n)
	for i := 1; i <= n; i++ {
		procs = append(procs, &table.Process{
			ID:         uint32(i),
			Attachment: &table.ProcessAttachment{TenantID: "default", BizID: 1, CcProcessID: uint32(i)},
			Spec:       &table.ProcessSpec{Alias: "proc", SourceData: "{}", PrevData: "{}", ProcNum: 1},
			Revision:   &table.Revision{},
		})
	}
	return procs
}

// TestProcessBatchUpdateWithTxPlaceholderLimit 业务下需更新的进程数量较多时（约 2000 个以上），
// 单条 INSERT ... ON DUPLICATE KEY UPDATE 的占位符会超过 MySQL 上限，必须分批执行
func TestProcessBatchUpdateWithTxPlaceholderLimit(t *testing.T) {
	tx, varsPerStmt := newDryRunQueryTx(t)
	d := &processDao{genQ: tx.Query}

	if err := d.BatchUpdateWithTx(kit.New(), tx, newTestProcesses(2000)); err != nil {
		t.Fatalf("batch update failed: %v", err)
	}

	if len(*varsPerStmt) != 4 {
		t.Fatalf("expected 4 statements for 2000 processes in batches of 500, got %d", len(*varsPerStmt))
	}
	for i, n := range *varsPerStmt {
		if n > mysqlMaxPlaceholders {
			t.Fatalf("statement %d has %d placeholders, exceeds mysql limit %d", i, n, mysqlMaxPlaceholders)
		}
	}
}
