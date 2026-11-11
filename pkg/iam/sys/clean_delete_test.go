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

package sys

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
)

var cleanOKBody = []byte(`{"code":0,"message":""}`)

// cleanFakeIAM 模拟权限中心的模型存储：query 返回当前模型，删除接口同期更新，
// 从而可以验证"删除后再回读"的校验行为。
type cleanFakeIAM struct {
	mu sync.Mutex

	model client.RegisteredSystemInfo

	// deletedTypes 按调用顺序记录资源类型的删除顺序
	deletedTypes []string
	// failActionID 对该操作的删除请求返回失败，模拟仍被引用/清理不掉的元素
	failActionID string
	// dropDeletions 对删除请求返回成功但不修改模型，模拟权限中心静默保留元素
	dropDeletions bool
}

func (f *cleanFakeIAM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/query") {
		f.mu.Lock()
		payload, err := json.Marshal(client.SystemResp{Data: f.model})
		f.mu.Unlock()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}
		_, _ = w.Write(payload)

		return
	}

	// 操作策略删除不影响模型本身，直接返回成功。
	if r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/policies") {
		_, _ = w.Write(cleanOKBody)

		return
	}

	var ids []struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &ids)

	switch {
	case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/actions"):
		f.deleteActions(w, ids)
	case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/instance-selections"):
		f.deleteSelections(w, ids)
	case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/resource-types"):
		f.deleteTypes(w, ids)
	default:
		_, _ = w.Write(cleanOKBody)
	}
}

func (f *cleanFakeIAM) deleteActions(w http.ResponseWriter, ids []struct {
	ID string `json:"id"`
}) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, item := range ids {
		if item.ID == f.failActionID {
			_, _ = w.Write([]byte(`{"code":1900001,"message":"action still referenced"}`))

			return
		}
	}

	if f.dropDeletions {
		_, _ = w.Write(cleanOKBody)

		return
	}

	kept := make([]client.ResourceAction, 0, len(f.model.Actions))
	for _, action := range f.model.Actions {
		drop := false
		for _, item := range ids {
			if string(action.ID) == item.ID {
				drop = true

				break
			}
		}
		if !drop {
			kept = append(kept, action)
		}
	}
	f.model.Actions = kept

	_, _ = w.Write(cleanOKBody)
}

func (f *cleanFakeIAM) deleteSelections(w http.ResponseWriter, ids []struct {
	ID string `json:"id"`
}) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.dropDeletions {
		_, _ = w.Write(cleanOKBody)

		return
	}

	kept := make([]client.InstanceSelection, 0, len(f.model.InstanceSelections))
	for _, selection := range f.model.InstanceSelections {
		drop := false
		for _, item := range ids {
			if string(selection.ID) == item.ID {
				drop = true

				break
			}
		}
		if !drop {
			kept = append(kept, selection)
		}
	}
	f.model.InstanceSelections = kept

	_, _ = w.Write(cleanOKBody)
}

func (f *cleanFakeIAM) deleteTypes(w http.ResponseWriter, ids []struct {
	ID string `json:"id"`
}) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, item := range ids {
		f.deletedTypes = append(f.deletedTypes, item.ID)
	}

	if f.dropDeletions {
		_, _ = w.Write(cleanOKBody)

		return
	}

	kept := make([]client.ResourceType, 0, len(f.model.ResourceTypes))
	for _, resourceType := range f.model.ResourceTypes {
		drop := false
		for _, item := range ids {
			if string(resourceType.ID) == item.ID {
				drop = true

				break
			}
		}
		if !drop {
			kept = append(kept, resourceType)
		}
	}
	f.model.ResourceTypes = kept

	_, _ = w.Write(cleanOKBody)
}

// newCleanFakeSys 用给定模型构造 Sys 与对应的假权限中心。
func newCleanFakeSys(t *testing.T, model client.RegisteredSystemInfo) (*Sys, *cleanFakeIAM) {
	t.Helper()

	fake := &cleanFakeIAM{model: model}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	cli, err := client.NewClient(&client.Config{
		Address:   []string{server.URL},
		AppCode:   "bk-bscp",
		AppSecret: "secret",
		SystemID:  SystemIDBSCP,
	}, nil)
	require.NoError(t, err)

	s, err := NewSys(cli)
	require.NoError(t, err)

	return s, fake
}

// projectEnvironmentModel 返回带祖先链的模型：biz ← project ← environment。
func projectEnvironmentModel() client.RegisteredSystemInfo {
	return client.RegisteredSystemInfo{
		BaseInfo: client.System{ID: SystemIDBSCP},
		ResourceTypes: []client.ResourceType{
			{ID: "biz"},
			{ID: "project", Parents: []client.Parent{
				{SystemID: SystemIDBSCP, ResourceID: "biz"},
			}},
			{ID: "environment", Parents: []client.Parent{
				{SystemID: SystemIDBSCP, ResourceID: "project"},
			}},
		},
		InstanceSelections: []client.InstanceSelection{{ID: "project_selection"}},
		Actions: []client.ResourceAction{
			{ID: "project_view"}, {ID: "environment_view"},
		},
	}
}

// 资源类型必须从叶子往根删：先删根类型会因仍被子类型引用而失败，导致整批残留。
func TestDeleteAllDeletesLeafResourceTypesFirst(t *testing.T) {
	s, fake := newCleanFakeSys(t, projectEnvironmentModel())

	summary, err := s.DeleteAll(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, summary.Actions)
	require.Equal(t, 1, summary.InstanceSelections)
	require.Equal(t, 3, summary.ResourceTypes)

	require.Equal(t, []string{"environment", "project", "biz"}, fake.deletedTypes,
		"应先删叶子类型，最后删根类型")
}

// 单个操作删不掉时，其余操作、实例视图与资源类型仍要继续清理，不能整批放弃。
func TestDeleteAllContinuesAfterActionFailure(t *testing.T) {
	s, fake := newCleanFakeSys(t, projectEnvironmentModel())
	fake.failActionID = "project_view"

	_, err := s.DeleteAll(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "project_view", "错误里要指出没删掉的元素")

	require.Equal(t, []client.ResourceAction{{ID: "project_view"}}, fake.model.Actions,
		"失败之外的操作都应被删除")
	require.Empty(t, fake.model.InstanceSelections, "实例视图不应受操作删除失败影响")
	require.Empty(t, fake.model.ResourceTypes, "资源类型不应受操作删除失败影响")
}

// 权限中心返回成功但没有真正删除时，必须回读发现残留并以非零结果报错。
func TestDeleteAllReportsSilentlyKeptElements(t *testing.T) {
	s, fake := newCleanFakeSys(t, projectEnvironmentModel())
	fake.dropDeletions = true

	summary, err := s.DeleteAll(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "残留")
	require.Empty(t, summary.Failures, "删除请求本身没有报错，残留由回读校验发现")
}
