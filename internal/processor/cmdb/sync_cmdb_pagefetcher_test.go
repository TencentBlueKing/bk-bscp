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

package cmdb

import (
	"reflect"
	"testing"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
)

// TestPageFetcherAdvancesByReturnedCount CC 把页大小截断到 limit 以下时，
// 必须按实际返回条数推进 start，否则会跳页漏段（漏段会让全量 diff 误删进程）
func TestPageFetcherAdvancesByReturnedCount(t *testing.T) {
	data := []int{1, 2, 3, 4, 5}
	const ccPageCap = 2

	calls := 0
	got, err := PageFetcher(func(page *bkcmdb.PageParam) ([]int, int, error) {
		calls++
		if page.Start >= len(data) {
			return nil, len(data), nil
		}
		end := page.Start + ccPageCap
		if end > len(data) {
			end = len(data)
		}
		return data[page.Start:end], len(data), nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, data) {
		t.Fatalf("got %v, want %v", got, data)
	}
	if calls != 3 {
		t.Fatalf("fetch calls = %d, want 3", calls)
	}
}

// TestPageFetcherHandlesUnreliableCount CC 返回 count=0 时不能提前结束，
// 否则会漏掉后续页
func TestPageFetcherHandlesUnreliableCount(t *testing.T) {
	data := []int{1, 2, 3, 4, 5}
	const ccPageCap = 2

	got, err := PageFetcher(func(page *bkcmdb.PageParam) ([]int, int, error) {
		if page.Start >= len(data) {
			return nil, 0, nil
		}
		end := page.Start + ccPageCap
		if end > len(data) {
			end = len(data)
		}
		return data[page.Start:end], 0, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, data) {
		t.Fatalf("got %v, want %v", got, data)
	}
}

// TestPageFetcherStopsAfterSinglePage 总数可信且首页返回完整数据时只请求一次
func TestPageFetcherStopsAfterSinglePage(t *testing.T) {
	data := []int{1, 2, 3}
	calls := 0

	got, err := PageFetcher(func(page *bkcmdb.PageParam) ([]int, int, error) {
		calls++
		if page.Start != 0 {
			t.Fatalf("unexpected page start %d", page.Start)
		}
		return data, len(data), nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, data) {
		t.Fatalf("got %v, want %v", got, data)
	}
	if calls != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls)
	}
}

// TestPageFetcherFailsAtMaxPages CC 分页异常（总数永远大于已拉取数）时必须有翻页上限，
// 且不能把部分数据当全量返回：全量 diff 会把漏掉的进程误判为已删除
func TestPageFetcherFailsAtMaxPages(t *testing.T) {
	calls := 0
	got, err := PageFetcher(func(page *bkcmdb.PageParam) ([]int, int, error) {
		calls++
		return []int{calls}, 1 << 20, nil // 始终返回 1 条，总数远大于实际数据
	})
	if err == nil {
		t.Fatalf("expected error when max pages reached, got %d items", len(got))
	}
	if got != nil {
		t.Fatalf("partial data must not be returned together with error, got %d items", len(got))
	}
	if calls != maxPageFetcherPages {
		t.Fatalf("fetch calls = %d, want %d", calls, maxPageFetcherPages)
	}
}

// TestPageFetcherFailsOnEarlyEmptyPage 空页早于总数出现（CC 抖动/限流）时，
// 无法确认已取满全量，必须返回错误而不是部分数据
func TestPageFetcherFailsOnEarlyEmptyPage(t *testing.T) {
	calls := 0
	_, err := PageFetcher(func(page *bkcmdb.PageParam) ([]int, int, error) {
		calls++
		if calls == 1 {
			return []int{1, 2}, 10, nil
		}
		return nil, 10, nil
	})
	if err == nil {
		t.Fatal("expected error when empty page returned before total reached")
	}
}

// TestPageFetcherProbesWhenCountLooksLikePageSize count 恰好等于首页条数（可能是「本页条数」
// 语义）时，不能据此提前结束，否则只拉到第一页就当作全量
func TestPageFetcherProbesWhenCountLooksLikePageSize(t *testing.T) {
	data := make([]int, pageFetcherLimit+3)
	for i := range data {
		data[i] = i
	}

	got, err := PageFetcher(func(page *bkcmdb.PageParam) ([]int, int, error) {
		if page.Start >= len(data) {
			return nil, pageFetcherLimit, nil
		}
		end := min(page.Start+pageFetcherLimit, len(data))
		pageData := data[page.Start:end]
		return pageData, len(pageData), nil // count 语义为「本页条数」
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, data) {
		t.Fatalf("got %d items, want %d", len(got), len(data))
	}
}
