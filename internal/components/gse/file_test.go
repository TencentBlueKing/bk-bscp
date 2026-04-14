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

package gse

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/internal/components"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestFileService(t *testing.T, handler roundTripFunc) (*Service, func()) {
	t.Helper()

	client := components.GetClient()
	oldTransport := client.GetClient().Transport
	client.SetTransport(handler)

	service := NewService("test-app-code", "test-app-secret", "http://gse.test")
	return service, func() {
		client.SetTransport(oldTransport)
	}
}

func testFileContext() *kit.Kit {
	return kit.NewWithTenant("test-tenant")
}

func TestAsyncExtensionsTransferFile(t *testing.T) {
	service, cleanup := newTestFileService(t, func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v2/task/extensions/async_transfer_file", r.URL.Path)

		var req TransferFileReq
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Len(t, req.Tasks, 1)
		require.Equal(t, "source-file", req.Tasks[0].Source.FileName)
		require.Equal(t, "target-file", req.Tasks[0].Target.FileName)

		return newTestResponse(http.StatusOK, `{"code":0,"message":"ok","data":{"result":{"task_id":"task-123"}}}`), nil
	})
	defer cleanup()

	resp, err := service.AsyncExtensionsTransferFile(testFileContext().Ctx, &TransferFileReq{
		TimeOutSeconds: 600,
		AutoMkdir:      true,
		Tasks: []TransferFileTask{
			{
				Source: TransferFileSource{
					FileName: "source-file",
					StoreDir: "/tmp/source",
					Agent: TransferFileAgent{
						BkAgentID: "src-agent",
					},
				},
				Target: TransferFileTarget{
					FileName: "target-file",
					StoreDir: "/tmp/target",
					Agents: []TransferFileAgent{
						{BkAgentID: "target-agent"},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "task-123", resp.Result.TaskID)
}

func TestAsyncTerminateTransferFile(t *testing.T) {
	service, cleanup := newTestFileService(t, func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v2/task/extensions/async_terminate_transfer_file", r.URL.Path)

		var req TerminateTransferFileTaskReq
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "task-terminate", req.TaskID)
		require.Len(t, req.Agents, 1)
		require.Equal(t, "target-agent", req.Agents[0].BkAgentID)

		return newTestResponse(http.StatusOK, `{"code":0,"message":"ok","data":{"result":{"task_id":"task-terminate"}}}`), nil
	})
	defer cleanup()

	resp, err := service.AsyncTerminateTransferFile(testFileContext().Ctx, &TerminateTransferFileTaskReq{
		TaskID: "task-terminate",
		Agents: []TransferFileAgent{
			{BkAgentID: "target-agent"},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "task-terminate", resp.Result.TaskID)
}

func TestGetExtensionsTransferFileResult(t *testing.T) {
	service, cleanup := newTestFileService(t, func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v2/task/extensions/get_transfer_file_result", r.URL.Path)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NotContains(t, string(body), `"agents":`)

		var req GetTransferFileResultReq
		require.NoError(t, json.Unmarshal(body, &req))
		require.Equal(t, "task-result", req.TaskID)
		require.Nil(t, req.Agents)

		return newTestResponse(http.StatusOK, `{
			"code": 0,
			"message": "ok",
			"data": {
				"version": "v2",
				"result": [{
					"error_code": 0,
					"error_msg": "",
					"content": {
						"dest_agent_id": "target-agent",
						"dest_container_id": "container-1",
						"dest_file_dir": "/tmp",
						"dest_file_name": "config.yaml"
					}
				}]
			}
		}`), nil
	})
	defer cleanup()

	resp, err := service.GetExtensionsTransferFileResult(testFileContext().Ctx, &GetTransferFileResultReq{
		TaskID: "task-result",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "v2", resp.Version)
	require.Len(t, resp.Result, 1)
	require.Equal(t, "target-agent", resp.Result[0].Content.DestAgentID)
}

func TestGetExtensionsTransferFileResultErrorCode(t *testing.T) {
	service, cleanup := newTestFileService(t, func(r *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, `{"code":123,"message":"query failed","data":{}}`), nil
	})
	defer cleanup()

	resp, err := service.GetExtensionsTransferFileResult(testFileContext().Ctx, &GetTransferFileResultReq{
		TaskID: "task-result",
	})
	require.Nil(t, resp)
	require.Error(t, err)
	require.ErrorContains(t, err, "code=123")
	require.ErrorContains(t, err, "query failed")
}

func newTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}
