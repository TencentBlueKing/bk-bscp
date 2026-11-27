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
	"errors"
	"fmt"
	"testing"
	"time"

	itypes "github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"github.com/stretchr/testify/assert"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbtb "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/task_batch"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/taskerr"
)

func TestErrorCodeOf(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    int32
	}{
		{
			name:    "structured bscp error",
			message: taskerr.New(taskerr.CodeProcessMismatch, taskerr.WithMsg("script exit failed")).Error(),
			want:    taskerr.CodeProcessMismatch,
		},
		{
			name:    "structured bscp error with original code",
			message: taskerr.New(1015000, taskerr.WithMsg("gse operate failed")).Error(),
			want:    1015000,
		},
		{
			name:    "structured gse error",
			message: taskerr.New(791001, taskerr.WithMsg("request failed")).Error(),
			want:    791001,
		},
		{
			name:    "historical message fallback",
			message: "[Operate STEP]: process operate failed, task_id=t, errorCode=828, errorMsg=dup",
			want:    828,
		},
		{
			name:    "no error code",
			message: "task exec timeout",
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, errorCodeOf(tt.message))
		})
	}
}

func TestErrorCodeKeyOf(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "structured bscp error",
			message: taskerr.New(taskerr.CodeProcessMismatch, taskerr.WithMsg("script exit failed")).Error(),
			want:    "4100500",
		},
		{
			name:    "structured gse error",
			message: taskerr.New(791001, taskerr.WithMsg("request failed")).Error(),
			want:    "791001",
		},
		{
			name:    "structured error without code is excluded",
			message: taskerr.New(0).Error(),
			want:    "",
		},
		{
			name:    "historical message with gse code",
			message: "[Operate STEP]: errorCode=828, errorMsg=dup",
			want:    "828",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, errorCodeKeyOf(tt.message))
		})
	}
}

func TestFailedReasonOf(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name: "structured error without cause falls back to msg",
			message: "step ValidateOperateProcess running failed, err=" +
				taskerr.New(taskerr.CodeProcessMismatch, taskerr.WithMsg("the register command does not exist")).Error(),
			want: "the register command does not exist",
		},
		{
			name: "structured error with cause keeps cause text",
			message: "step Operate running failed, err=" +
				taskerr.New(1015000, taskerr.WithMsg("wait gse operate task finish failed")).
					WithCause(errors.New("context deadline exceeded")).Error(),
			want: "context deadline exceeded",
		},
		{
			name:    "historical message strips framework prefix",
			message: "step Operate running failed, err=get gse task result failed, timeout",
			want:    "get gse task result failed, timeout",
		},
		{
			name: "retry suffix is converted to readable text",
			message: "step Operate running failed, err=" +
				taskerr.New(taskerr.CodeProcessMismatch, taskerr.WithMsg("script execution failed")).Error() + ", retried=1, maxRetries=3",
			want: "script execution failed（已重试 1/3 次）",
		},
		{
			name:    "non step message is kept",
			message: "task exec timeout",
			want:    "task exec timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, failedReasonOf(tt.message))
		})
	}
}

// TestBuildStatisticsOverviewExcludesPending 运行中与未开始不算失败，但仍计入总数，
// 保证 total = success + ignored + failed + running + not_started。
func TestBuildStatisticsOverviewExcludesPending(t *testing.T) {
	counts := map[string]int64{
		itypes.TaskStatusSuccess:    60,
		itypes.TaskStatusIgnored:    5,
		itypes.TaskStatusFailure:    20,
		itypes.TaskStatusTimeout:    5,
		itypes.TaskStatusRevoked:    2,
		itypes.TaskStatusRunning:    6,
		itypes.TaskStatusInit:       1,
		itypes.TaskStatusNotStarted: 1,
	}

	got := buildStatisticsOverview(counts)
	// 失败只含 FAILURE + TIMEOUT + REVOKED
	assert.Equal(t, uint32(27), got.GetFailed())
	assert.Equal(t, uint32(100), got.GetTotal())
	assert.Equal(t, uint32(60), got.GetSuccess())
	assert.Equal(t, uint32(5), got.GetIgnored())
	assert.Equal(t, uint32(7), got.GetRunning())
	assert.Equal(t, uint32(1), got.GetNotStarted())
	assert.InDelta(t, 0.27, got.GetFailureRate(), 1e-6)
}

func TestChunkStrings(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		size   int
		want   int
	}{
		{name: "empty", values: nil, size: 2, want: 0},
		{name: "smaller than chunk", values: []string{"1", "2"}, size: 5, want: 1},
		{name: "exact multiple", values: []string{"1", "2", "3", "4"}, size: 2, want: 2},
		{name: "remainder", values: []string{"1", "2", "3"}, size: 2, want: 2},
		{name: "invalid size falls back to one chunk", values: []string{"1", "2"}, size: 0, want: 1},
	}
	for _, c := range cases {
		chunks := chunkStrings(c.values, c.size)
		if len(chunks) != c.want {
			t.Fatalf("%s: got %d chunks, want %d", c.name, len(chunks), c.want)
		}
		total := 0
		for _, chunk := range chunks {
			total += len(chunk)
		}
		if total != len(c.values) {
			t.Fatalf("%s: lost elements, got %d want %d", c.name, total, len(c.values))
		}
	}
}

// failureRow 构造一条失败明细基础行，便于筛选与选项用例复用。
func failureRow(taskID, creator, status string, bizID uint32, message string,
	start, end time.Time) *dao.TaskFailureRow {
	return &dao.TaskFailureRow{
		TaskID:  taskID,
		Creator: creator,
		Status:  status,
		BizID:   bizID,
		Message: message,
		Start:   start,
		End:     end,
	}
}

// TestBuildFailureSearchParsesOptions 覆盖扩展筛选项的解析：空值忽略、-- 表示无错误码、
// 业务 ID 必须为数字、时间格式非法时报错。
func TestBuildFailureSearchParsesOptions(t *testing.T) {
	kt := kit.New()
	baseStart := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	baseEnd := time.Date(2026, 11, 30, 23, 59, 59, 0, time.UTC)

	search, err := buildFailureSearch(kt, &pbds.TaskFailureSearch{
		TaskIds:        []string{" t1 ", ""},
		Creators:       []string{"admin"},
		Statuses:       []string{"FAILURE"},
		BizNames:       []string{"bscp"},
		BizIds:         []string{"2"},
		ErrorCodes:     []string{"--", "4100500", " "},
		FailureMessage: "  Timeout  ",
		StartTime:      "2026-11-01 10:00:00",
		EndTime:        "2026-11-02",
	}, baseStart, baseEnd, []string{"admin"})
	assert.NoError(t, err)
	assert.Contains(t, search.taskIDs, "t1")
	assert.NotContains(t, search.taskIDs, "")
	assert.True(t, search.noErrorCode)
	assert.Contains(t, search.errorCodes, "4100500")
	assert.Contains(t, search.bizIDs, uint32(2))
	assert.Equal(t, "timeout", search.failureMessage)
	assert.False(t, search.startTime.dateOnly)
	assert.True(t, time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC).Equal(search.startTime.value))
	// 只给日期时上界取当天 23:59:59，保证当天数据不被排除
	assert.True(t, search.endTime.dateOnly)
	assert.True(t, time.Date(2026, 11, 2, 23, 59, 59, 0, time.UTC).Equal(search.endTime.value))

	_, err = buildFailureSearch(kt, &pbds.TaskFailureSearch{BizIds: []string{"not-a-number"}},
		baseStart, baseEnd, nil)
	assert.Error(t, err)

	_, err = buildFailureSearch(kt, &pbds.TaskFailureSearch{StartTime: "2026/11/01"},
		baseStart, baseEnd, nil)
	assert.Error(t, err)

	empty, err := buildFailureSearch(kt, nil, baseStart, baseEnd, nil)
	assert.NoError(t, err)
	assert.Empty(t, empty.taskIDs)
}

// TestBuildFailureSearchValidatesBaseRange 扩展的时间与人员必须落在基础范围内（允许相等）。
func TestBuildFailureSearchValidatesBaseRange(t *testing.T) {
	kt := kit.New()
	baseStart := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	baseEnd := time.Date(2026, 11, 30, 23, 0, 0, 0, time.UTC)

	// 与基础范围边界相等允许
	_, err := buildFailureSearch(kt, &pbds.TaskFailureSearch{
		StartTime: "2026-11-01 00:00:00",
		EndTime:   "2026-11-30 23:00:00",
	}, baseStart, baseEnd, nil)
	assert.NoError(t, err)

	// 开始时间早于基础范围、结束时间晚于基础范围都拒绝
	_, err = buildFailureSearch(kt, &pbds.TaskFailureSearch{StartTime: "2026-10-31"},
		baseStart, baseEnd, nil)
	assert.Error(t, err)
	_, err = buildFailureSearch(kt, &pbds.TaskFailureSearch{EndTime: "2026-12-01"},
		baseStart, baseEnd, nil)
	assert.Error(t, err)

	// 按天条件只比较日期：范围内任意一天都允许
	_, err = buildFailureSearch(kt, &pbds.TaskFailureSearch{StartTime: "2026-11-30"},
		baseStart, baseEnd, nil)
	assert.NoError(t, err)

	// 人员必须在基础人员范围内
	_, err = buildFailureSearch(kt, &pbds.TaskFailureSearch{Creators: []string{"admin"}},
		baseStart, baseEnd, []string{"admin", "ops"})
	assert.NoError(t, err)
	_, err = buildFailureSearch(kt, &pbds.TaskFailureSearch{Creators: []string{"unknown"}},
		baseStart, baseEnd, []string{"admin"})
	assert.Error(t, err)

	// 基础人员为空表示不限，不做校验
	_, err = buildFailureSearch(kt, &pbds.TaskFailureSearch{Creators: []string{"anyone"}},
		baseStart, baseEnd, nil)
	assert.NoError(t, err)

	// 结束时间早于开始时间
	_, err = buildFailureSearch(kt, &pbds.TaskFailureSearch{
		StartTime: "2026-11-20 10:00:00",
		EndTime:   "2026-11-20 09:00:00",
	}, baseStart, baseEnd, nil)
	assert.Error(t, err)
}

// TestMatchFailureSearch 覆盖维度内 OR、维度间 AND、错误码特殊选项与失败信息模糊匹配。
func TestMatchFailureSearch(t *testing.T) {
	start := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 11, 20, 10, 0, 3, 0, time.UTC)
	rows := []*dao.TaskFailureRow{
		failureRow("t1", "admin", itypes.TaskStatusFailure, 2,
			taskerr.New(taskerr.CodeProcessMismatch, taskerr.WithMsg("script exit failed")).Error(), start, end),
		failureRow("t2", "", itypes.TaskStatusTimeout, 3, "task exec timeout", start, time.Time{}),
		failureRow("t3", "admin", itypes.TaskStatusFailure, 2, "task exec timeout",
			start.Add(30*time.Second), end),
	}
	bizNames := map[uint32]string{2: "bscp"}
	baseStart := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	baseEnd := time.Date(2026, 11, 30, 23, 59, 59, 0, time.UTC)

	newSearch := func(t *testing.T, req *pbds.TaskFailureSearch) *failureSearch {
		t.Helper()
		search, err := buildFailureSearch(kit.New(), req, baseStart, baseEnd, nil)
		assert.NoError(t, err)
		return search
	}

	// 任务 ID 多选内部为 OR
	search := newSearch(t, &pbds.TaskFailureSearch{TaskIds: []string{"t1", "t2"}})
	assert.True(t, matchFailureSearch(rows[0], search, bizNames))
	assert.True(t, matchFailureSearch(rows[1], search, bizNames))

	search = newSearch(t, &pbds.TaskFailureSearch{TaskIds: []string{"t2"}})
	assert.False(t, matchFailureSearch(rows[0], search, bizNames))

	// 不同维度之间为 AND：任务 ID 命中但状态不命中时不保留
	search = newSearch(t, &pbds.TaskFailureSearch{TaskIds: []string{"t1", "t2"}, Statuses: []string{"TIMEOUT"}})
	assert.False(t, matchFailureSearch(rows[0], search, bizNames))
	assert.True(t, matchFailureSearch(rows[1], search, bizNames))

	// 创建人为空时归 "-"
	search = newSearch(t, &pbds.TaskFailureSearch{Creators: []string{"-"}})
	assert.False(t, matchFailureSearch(rows[0], search, bizNames))
	assert.True(t, matchFailureSearch(rows[1], search, bizNames))

	// 业务名称与业务 ID 都能命中同一行
	search = newSearch(t, &pbds.TaskFailureSearch{BizNames: []string{"bscp"}})
	assert.True(t, matchFailureSearch(rows[0], search, bizNames))
	search = newSearch(t, &pbds.TaskFailureSearch{BizNames: []string{"2"}})
	assert.True(t, matchFailureSearch(rows[0], search, bizNames))
	search = newSearch(t, &pbds.TaskFailureSearch{BizIds: []string{"3"}})
	assert.True(t, matchFailureSearch(rows[1], search, bizNames))

	// 错误码多选：-- 表示无错误码
	search = newSearch(t, &pbds.TaskFailureSearch{ErrorCodes: []string{"4100500"}})
	assert.True(t, matchFailureSearch(rows[0], search, bizNames))
	assert.False(t, matchFailureSearch(rows[1], search, bizNames))
	search = newSearch(t, &pbds.TaskFailureSearch{ErrorCodes: []string{"--"}})
	assert.False(t, matchFailureSearch(rows[0], search, bizNames))
	assert.True(t, matchFailureSearch(rows[1], search, bizNames))

	// 失败信息模糊匹配，大小写不敏感
	search = newSearch(t, &pbds.TaskFailureSearch{FailureMessage: "EXIT"})
	assert.True(t, matchFailureSearch(rows[0], search, bizNames))
	assert.False(t, matchFailureSearch(rows[1], search, bizNames))

	// 开始时间是下界（>=）：等于边界也命中，晚于边界的同样命中
	search = newSearch(t, &pbds.TaskFailureSearch{StartTime: "2026-11-20 10:00:00"})
	assert.True(t, matchFailureSearch(rows[0], search, bizNames))
	assert.True(t, matchFailureSearch(rows[2], search, bizNames))
	search = newSearch(t, &pbds.TaskFailureSearch{StartTime: "2026-11-20 10:00:01"})
	assert.False(t, matchFailureSearch(rows[0], search, bizNames))
	assert.True(t, matchFailureSearch(rows[2], search, bizNames))
	search = newSearch(t, &pbds.TaskFailureSearch{StartTime: "2026-11-20 10:01"})
	assert.False(t, matchFailureSearch(rows[0], search, bizNames))
	search = newSearch(t, &pbds.TaskFailureSearch{StartTime: "2026-11-20"})
	assert.True(t, matchFailureSearch(rows[0], search, bizNames))

	// 结束时间是上界（<=）：等于边界也命中，超出边界的不命中；结束时间为空的行不命中
	search = newSearch(t, &pbds.TaskFailureSearch{EndTime: "2026-11-20 10:00:03"})
	assert.True(t, matchFailureSearch(rows[0], search, bizNames))
	assert.False(t, matchFailureSearch(rows[1], search, bizNames))
	search = newSearch(t, &pbds.TaskFailureSearch{EndTime: "2026-11-20 10:00:02"})
	assert.False(t, matchFailureSearch(rows[0], search, bizNames))
	// 只给日期时上界取当天 23:59:59
	search = newSearch(t, &pbds.TaskFailureSearch{EndTime: "2026-11-20"})
	assert.True(t, matchFailureSearch(rows[0], search, bizNames))
	assert.False(t, matchFailureSearch(rows[1], search, bizNames))
}

// TestBuildFailureFilterOptions 覆盖选项来源：基础行集去重、排序稳定、
// 错误码固定包含无码选项、业务名缺失时回落业务 ID。
func TestBuildFailureFilterOptions(t *testing.T) {
	start := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	rows := []*dao.TaskFailureRow{
		failureRow("t2", "", itypes.TaskStatusTimeout, 0, "task exec timeout", start, start),
		failureRow("t1", "admin", itypes.TaskStatusFailure, 2,
			taskerr.New(828, taskerr.WithMsg("dup")).Error(), start, start),
		failureRow("t1", "admin", itypes.TaskStatusFailure, 2,
			taskerr.New(taskerr.CodeProcessMismatch, taskerr.WithMsg("mismatch")).Error(), start, start),
	}

	options := buildFailureFilterOptions(rows, map[uint32]string{2: "bscp"})

	assert.Equal(t, []string{"t1", "t2"}, choiceIDs(options.GetTaskIds()))
	assert.Equal(t, []string{"-", "admin"}, choiceIDs(options.GetCreators()))
	assert.Equal(t, []string{itypes.TaskStatusFailure, itypes.TaskStatusTimeout}, choiceIDs(options.GetStatuses()))
	assert.Equal(t, []string{"bscp"}, choiceIDs(options.GetBizNames()))
	assert.Equal(t, []string{"2"}, choiceIDs(options.GetBizIds()))
	// 开始/结束时间选项按天返回，31 天范围最多 31 个选项
	assert.Equal(t, []string{"2026-11-20"}, choiceIDs(options.GetStartTimes()))
	assert.Equal(t, []string{"2026-11-20"}, choiceIDs(options.GetEndTimes()))
	// 无错误码固定第一个，其余按错误码数值升序
	assert.Equal(t, []string{"--", "828", "4100500"}, choiceIDs(options.GetErrorCodes()))
	assert.Equal(t, "bscp", options.GetBizIds()[0].GetName())
}

// TestBuildFailureFilterOptionsCapsTaskIDs 任务 ID 选项只保留最新的 maxFailureTaskIDOptions 个，
// 避免大范围查询把响应撑到 gRPC 接收上限以上。
func TestBuildFailureFilterOptionsCapsTaskIDs(t *testing.T) {
	rows := make([]*dao.TaskFailureRow, 0, maxFailureTaskIDOptions+100)
	start := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	for i := 0; i < maxFailureTaskIDOptions+100; i++ {
		rows = append(rows, failureRow(fmt.Sprintf("t%05d", i), "admin", itypes.TaskStatusFailure, 1,
			"failed", start, start))
	}

	options := buildFailureFilterOptions(rows, nil)
	assert.Len(t, options.GetTaskIds(), maxFailureTaskIDOptions)
	// 行按 created_at 倒序，取到的是最新的一批，排序后首项为 t00000
	assert.Equal(t, "t00000", options.GetTaskIds()[0].GetId())
	assert.Equal(t, fmt.Sprintf("t%05d", maxFailureTaskIDOptions-1), options.GetTaskIds()[maxFailureTaskIDOptions-1].GetId())
}

// TestBuildFailureFilterOptionsFallsBackToBizID 业务名映射失败时，业务名称选项回落为业务 ID。
func TestBuildFailureFilterOptionsFallsBackToBizID(t *testing.T) {
	row := failureRow("t1", "admin", itypes.TaskStatusFailure, 9, "failed", time.Time{}, time.Time{})
	options := buildFailureFilterOptions([]*dao.TaskFailureRow{row}, nil)

	assert.Equal(t, []string{"9"}, choiceIDs(options.GetBizNames()))
	assert.Equal(t, []string{"9"}, choiceIDs(options.GetBizIds()))
	assert.Empty(t, options.GetStartTimes())
	assert.Empty(t, options.GetEndTimes())
}

// TestPageFailureRows 覆盖内存分页边界。
func TestPageFailureRows(t *testing.T) {
	rows := []*dao.TaskFailureRow{{TaskID: "t1"}, {TaskID: "t2"}, {TaskID: "t3"}}

	assert.Len(t, pageFailureRows(rows, 0, 2), 2)
	assert.Equal(t, "t3", pageFailureRows(rows, 2, 2)[0].TaskID)
	assert.Empty(t, pageFailureRows(rows, 3, 2))
	assert.Len(t, pageFailureRows(rows, 1, 100), 2)
	assert.Empty(t, pageFailureRows(nil, 0, 10))
}

// choiceIDs 提取选项 ID 列表，用于断言选项内容与顺序。
func choiceIDs(choices []*pbtb.Choice) []string {
	ids := make([]string, 0, len(choices))
	for _, choice := range choices {
		ids = append(ids, choice.GetId())
	}
	return ids
}
