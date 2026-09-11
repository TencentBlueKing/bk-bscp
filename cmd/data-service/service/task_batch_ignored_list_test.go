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
	"context"
	"fmt"
	"sort"
	"testing"

	istore "github.com/Tencent/bk-bcs/bcs-common/common/task/stores/iface"
	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// fakeTaskStore 仅实现 listIgnoredTaskDetails 依赖的 ListTask，其余方法兜底报错。
// perQueryCap 模拟存储实现单页返回上限小于请求 limit 的行为（0 表示不限制）。
type fakeTaskStore struct {
	tasks       []*types.Task
	perQueryCap int64
	queries     [][2]int64
}

func (f *fakeTaskStore) ListTask(_ context.Context, opt *istore.ListOption) (
	*istore.Pagination[types.Task], error) {

	f.queries = append(f.queries, [2]int64{opt.Offset, opt.Limit})

	items := make([]*types.Task, 0, len(f.tasks))
	for _, t := range f.tasks {
		if len(opt.StatusList) == 0 {
			items = append(items, t)
			continue
		}
		for _, st := range opt.StatusList {
			if t.Status == st {
				items = append(items, t)
				break
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].TaskID < items[j].TaskID })

	if opt.Offset >= int64(len(items)) {
		return &istore.Pagination[types.Task]{Count: int64(len(items))}, nil
	}
	end := opt.Offset + opt.Limit
	if end > int64(len(items)) {
		end = int64(len(items))
	}
	if f.perQueryCap > 0 && end-opt.Offset > f.perQueryCap {
		end = opt.Offset + f.perQueryCap
	}
	return &istore.Pagination[types.Task]{
		Count: int64(len(items)),
		Items: items[opt.Offset:end],
	}, nil
}

func (f *fakeTaskStore) EnsureTable(context.Context, ...any) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeTaskStore) CreateTask(context.Context, *types.Task) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeTaskStore) GetTask(context.Context, string) (*types.Task, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeTaskStore) DeleteTask(context.Context, string) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeTaskStore) UpdateTask(context.Context, *types.Task) error {
	return fmt.Errorf("not implemented")
}

func ignoredTestTask(id, status string, gseErrorCode int32) *types.Task {
	commonPayload := `{}`
	if gseErrorCode != 0 {
		commonPayload = fmt.Sprintf(`{"gsePayload":{"errorCode":%d,"errorMsg":"ignored"}}`, gseErrorCode)
	}
	return &types.Task{
		TaskID:        id,
		Status:        status,
		Creator:       "tester",
		CommonPayload: commonPayload,
	}
}

// TestListIgnoredTaskDetails IGNORED 查询须独立计算过滤后总数并按过滤结果分页：
// 页内命中数（旧行为）不能充当总数，页窗口基于过滤后序号而非存储层 Offset。
func TestListIgnoredTaskDetails(t *testing.T) {
	// 构造 10 个候选任务：偶数号命中 IGNORED（828），奇数号为普通失败
	store := &fakeTaskStore{}
	for i := 0; i < 10; i++ {
		status := types.TaskStatusSuccess
		code := int32(0)
		if i%2 == 0 {
			code = 828
		} else {
			status = types.TaskStatusFailure
		}
		store.tasks = append(store.tasks, ignoredTestTask(fmt.Sprintf("task-%02d", i), status, code))
	}

	kt := &kit.Kit{Ctx: context.Background()}

	// 一页装得下：Count = 过滤后总数 5，明细为全部命中任务
	details, total, err := listIgnoredTaskDetails(kt, store, "1", "ProcessOperate", 0, 50)
	if err != nil {
		t.Fatalf("listIgnoredTaskDetails page0 err: %v", err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5（旧行为会返回页内长度）", total)
	}
	if len(details) != 5 {
		t.Fatalf("len(details) = %d, want 5", len(details))
	}
	for i, d := range details {
		wantID := fmt.Sprintf("task-%02d", i*2)
		if d.TaskId != wantID {
			t.Fatalf("details[%d].TaskId = %s, want %s", i, d.TaskId, wantID)
		}
		if d.Status != TaskStatusIgnored {
			t.Fatalf("details[%d].Status = %s, want IGNORED", i, d.Status)
		}
	}

	// 第二页（start=3, limit=2）：Count 仍为总数 5，明细为第 3、4 个命中任务
	details, total, err = listIgnoredTaskDetails(kt, store, "1", "ProcessOperate", 3, 2)
	if err != nil {
		t.Fatalf("listIgnoredTaskDetails page1 err: %v", err)
	}
	if total != 5 {
		t.Fatalf("page1 total = %d, want 5", total)
	}
	if len(details) != 2 || details[0].TaskId != "task-06" || details[1].TaskId != "task-08" {
		t.Fatalf("page1 details = %+v, want [task-06 task-08]", details)
	}

	// start 超出总数：空明细但 Count 仍为总数，客户端可据此终止翻页
	details, total, err = listIgnoredTaskDetails(kt, store, "1", "ProcessOperate", 5, 50)
	if err != nil {
		t.Fatalf("listIgnoredTaskDetails overflow err: %v", err)
	}
	if total != 5 || len(details) != 0 {
		t.Fatalf("overflow: total = %d, len(details) = %d, want 5 / 0", total, len(details))
	}
}

// TestListIgnoredTaskDetailsMultiPage 存储层多页枚举：存储单页返回上限小于请求 limit 时
// 须逐页翻全，过滤序号跨页连续，页窗口与总数不受影响。
func TestListIgnoredTaskDetailsMultiPage(t *testing.T) {
	// 20 个候选全部命中（829），存储单页仅返回 8 条
	store := &fakeTaskStore{perQueryCap: 8}
	for i := 0; i < 20; i++ {
		store.tasks = append(store.tasks,
			ignoredTestTask(fmt.Sprintf("task-%02d", i), types.TaskStatusFailure, 829))
	}

	kt := &kit.Kit{Ctx: context.Background()}

	details, total, err := listIgnoredTaskDetails(kt, store, "1", "ProcessOperate", 10, 5)
	if err != nil {
		t.Fatalf("listIgnoredTaskDetails err: %v", err)
	}
	if total != 20 {
		t.Fatalf("total = %d, want 20", total)
	}
	if len(details) != 5 {
		t.Fatalf("len(details) = %d, want 5", len(details))
	}
	for i, d := range details {
		wantID := fmt.Sprintf("task-%02d", i+10)
		if d.TaskId != wantID {
			t.Fatalf("details[%d].TaskId = %s, want %s", i, d.TaskId, wantID)
		}
	}

	// 单页上限 8，20 条候选须翻 3 页（offset 0/8/16）
	if len(store.queries) != 3 {
		t.Fatalf("storage queries = %v, want 3", store.queries)
	}
}
