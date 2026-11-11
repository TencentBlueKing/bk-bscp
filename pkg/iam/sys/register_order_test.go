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

const (
	resourceTypesPath  = "/api/v1/model/systems/" + SystemIDBSCP + "/resource-types"
	actionsPath        = "/api/v1/model/systems/" + SystemIDBSCP + "/actions"
	selectionsPath     = "/api/v1/model/systems/" + SystemIDBSCP + "/instance-selections"
	creatorActionsPath = "/api/v1/model/systems/" + SystemIDBSCP + "/configs/resource_creator_actions"
	querySystemPathTpl = "/api/v1/model/systems/" + SystemIDBSCP + "/query"
)

// fakeIAM 只实现注册流程用到的接口，并按顺序记录调用与请求体。
type fakeIAM struct {
	mu             sync.Mutex
	calls          []string
	createdTypes   []string
	createdActions []string
	createdSelec   []string
	creatorActions []string
	existing       client.SystemResp
}

func (f *fakeIAM) record(r *http.Request, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, r.Method+" "+r.URL.Path)

	switch {
	case r.Method == http.MethodPost && r.URL.Path == resourceTypesPath:
		f.createdTypes = append(f.createdTypes, string(body))
	case r.Method == http.MethodPost && r.URL.Path == actionsPath:
		f.createdActions = append(f.createdActions, string(body))
	case r.Method == http.MethodPost && r.URL.Path == selectionsPath:
		f.createdSelec = append(f.createdSelec, string(body))
	case r.URL.Path == creatorActionsPath:
		f.creatorActions = append(f.creatorActions, string(body))
	}
}

// indexOf 返回某次调用的序号，未出现时返回 -1。
func (f *fakeIAM) indexOf(method, path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	for idx, call := range f.calls {
		if call == method+" "+path {
			return idx
		}
	}

	return -1
}

func (f *fakeIAM) joined(list []string) string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return strings.Join(list, "\n")
}

func (f *fakeIAM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	f.record(r, body)

	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet && r.URL.Path == querySystemPathTpl {
		payload, err := json.Marshal(f.existing)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}
		_, _ = w.Write(payload)

		return
	}

	_, _ = w.Write([]byte(`{"code":0,"message":"","data":{}}`))
}

// newFakeSysWith 用指定的线上模型构造 Sys。
func newFakeSysWith(t *testing.T, registered client.RegisteredSystemInfo) (*Sys, *fakeIAM) {
	t.Helper()

	resp := client.SystemResp{Data: registered}
	fake := &fakeIAM{existing: resp}
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

// newFakeSys 构造一个 Sys，其线上模型停留在「已注册项目、未注册环境」的状态。
func newFakeSys(t *testing.T) (*Sys, *fakeIAM) {
	t.Helper()

	isEnvironment := func(id string) bool { return strings.Contains(id, "environment") }

	resp := client.SystemResp{
		Data: client.RegisteredSystemInfo{
			BaseInfo: client.System{ID: SystemIDBSCP, Name: SystemNameBSCP},
		},
	}

	for _, rt := range GenerateStaticResourceTypes() {
		if isEnvironment(string(rt.ID)) {
			continue
		}
		resp.Data.ResourceTypes = append(resp.Data.ResourceTypes, rt)
	}

	for _, action := range GenerateStaticActions() {
		if isEnvironment(string(action.ID)) {
			continue
		}
		resp.Data.Actions = append(resp.Data.Actions, action)
	}

	for _, selection := range GenerateStaticInstanceSelections() {
		if isEnvironment(string(selection.ID)) {
			continue
		}
		resp.Data.InstanceSelections = append(resp.Data.InstanceSelections, selection)
	}

	return newFakeSysWith(t, resp.Data)
}

// 资源类型必须先于引用它的操作更新/创建，否则权限中心会报 related resource type not exists。
func TestRegisterCreatesResourceTypesBeforeTouchingActions(t *testing.T) {
	s, fake := newFakeSys(t)

	require.NoError(t, s.Register(context.Background(), "http://bscp.example.com"))

	createTypes := fake.indexOf(http.MethodPost, resourceTypesPath)
	createSelections := fake.indexOf(http.MethodPost, selectionsPath)
	updateType := fake.indexOf(http.MethodPut, resourceTypesPath+"/"+string(Project))
	updateAction := fake.indexOf(http.MethodPut, actionsPath+"/"+string(ProjectView))

	require.NotEqual(t, -1, createTypes, "environment resource type should be created")
	require.NotEqual(t, -1, createSelections, "environment instance selection should be created")
	require.NotEqual(t, -1, updateType, "existing resource types should still be updated")
	require.NotEqual(t, -1, updateAction, "existing project_view action should be updated")

	require.Less(t, createTypes, updateAction,
		"resource types must be created before an action referencing them is updated")
	require.Less(t, createSelections, updateAction,
		"instance selections must be created before an action referencing them is updated")
	require.Less(t, createTypes, updateType)
}

// 首次注册时，环境的资源类型、实例视图与操作都要被创建。
func TestRegisterCreatesEnvironmentModel(t *testing.T) {
	s, fake := newFakeSys(t)

	require.NoError(t, s.Register(context.Background(), "http://bscp.example.com"))

	createdTypes := fake.joined(fake.createdTypes)
	require.Contains(t, createdTypes, `"environment"`)

	createdSelections := fake.joined(fake.createdSelec)
	require.Contains(t, createdSelections, `"environment"`)

	createdActions := fake.joined(fake.createdActions)
	for _, id := range []string{
		string(EnvironmentCreate), string(EnvironmentView),
		string(EnvironmentEdit), string(EnvironmentDelete),
	} {
		require.Contains(t, createdActions, `"`+id+`"`)
	}

	// 环境操作声明在环境/项目维度，不能再落回 biz。
	require.Contains(t, createdActions, `"environment"`)
	require.Contains(t, createdActions, `"project"`)
}

// registeredWithoutProjectCreator 返回一个「线上模型完整、但创建者授权只配了服务和环境」的系统信息，
// 对应升级前已经在权限中心注册过模型的存量环境。
func registeredWithoutProjectCreator() client.RegisteredSystemInfo {
	return client.RegisteredSystemInfo{
		BaseInfo:           client.System{ID: SystemIDBSCP, Name: SystemNameBSCP},
		ResourceTypes:      GenerateStaticResourceTypes(),
		Actions:            GenerateStaticActions(),
		InstanceSelections: GenerateStaticInstanceSelections(),
		ResourceCreatorActions: client.ResourceCreatorActions{
			Config: []client.ResourceCreatorAction{
				{
					ResourceID: Application,
					Actions:    []client.CreatorRelatedAction{{ID: AppView}},
				},
				{
					ResourceID: Environment,
					Actions:    []client.CreatorRelatedAction{{ID: EnvironmentView}},
				},
			},
		},
	}
}

// 权限中心的 creator_authorization_instance 只对已注册 resource_creator_actions 的资源类型放行，
// 未注册时授权会被直接拒绝（1902403 creator_authorization_instance ... don't support the [project]）。
// 因此存量环境执行 migrate init-iam 时，必须用 PUT 把项目补进创建者授权配置，否则项目创建者永远拿不到权限。
func TestRegisterUpdatesCreatorActionsWithProject(t *testing.T) {
	s, fake := newFakeSysWith(t, registeredWithoutProjectCreator())

	require.NoError(t, s.Register(context.Background(), "http://bscp.example.com"))

	require.NotEqual(t, -1, fake.indexOf(http.MethodPut, creatorActionsPath),
		"线上已有创建者授权配置时必须走更新，而不是再次创建")

	body := fake.joined(fake.creatorActions)
	for _, want := range []string{
		`"id":"project"`,
		`"id":"project_view"`,
		`"id":"project_edit"`,
		`"id":"project_delete"`,
		`"id":"environment_create"`,
	} {
		require.Contains(t, body, want, "创建者授权配置缺少 %s", want)
	}

	// 存量配置里的服务与环境不能被丢掉，否则升级会让已有的创建者授权失效。
	require.Contains(t, body, `"id":"app"`)
	require.Contains(t, body, `"id":"environment"`)
}

// 首次注册（线上没有创建者授权配置）时用 POST 创建，同样要带上项目。
func TestRegisterCreatesCreatorActionsWithProject(t *testing.T) {
	registered := registeredWithoutProjectCreator()
	registered.ResourceCreatorActions = client.ResourceCreatorActions{}

	s, fake := newFakeSysWith(t, registered)

	require.NoError(t, s.Register(context.Background(), "http://bscp.example.com"))

	require.NotEqual(t, -1, fake.indexOf(http.MethodPost, creatorActionsPath))
	require.Contains(t, fake.joined(fake.creatorActions), `"id":"project"`)
}
