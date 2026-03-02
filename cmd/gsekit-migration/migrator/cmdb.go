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

package migrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/TencentBlueKing/bk-bscp/cmd/gsekit-migration/config"
)

// CMDBClient defines the interface for CMDB API operations needed by migration.
type CMDBClient interface {
	// ListServiceInstanceDetail gets service instance details including embedded process instances.
	// Returns a map keyed by service_instance_id.
	ListServiceInstanceDetail(ctx context.Context, bizID uint32, svcInstIDs []int64) (
		map[int64]*CMDBServiceInstance, error)
	// FindSetBatch gets set names by set IDs. Returns a map of set_id → set_name.
	FindSetBatch(ctx context.Context, bizID uint32, setIDs []int64) (map[int64]string, error)
	// FindModuleBatch gets module names by module IDs. Returns a map of module_id → module_name.
	FindModuleBatch(ctx context.Context, bizID uint32, moduleIDs []int64) (map[int64]string, error)
	// ListProcessDetailByIds gets process details by process IDs.
	// Returns a map of bk_process_id → CMDBProcessDetail.
	ListProcessDetailByIds(ctx context.Context, bizID uint32, processIDs []int64) (
		map[int64]*CMDBProcessDetail, error)
	// ListBizHosts gets host info for a biz, optionally filtered by module IDs.
	// When moduleIDs is non-empty, only hosts under those modules are returned.
	// Returns a map of bk_host_id → CMDBHostInfo.
	ListBizHosts(ctx context.Context, bizID uint32, moduleIDs []int64) (map[int64]*CMDBHostInfo, error)
}

// ----- CMDB response types (self-contained, independent of internal/components/bkcmdb) -----

// CMDBServiceInstance holds service instance detail returned by list_service_instance_detail API.
type CMDBServiceInstance struct {
	ID                int                   `json:"id"`
	Name              string                `json:"name"`
	BkBizID           int                   `json:"bk_biz_id"`
	BkModuleID        int                   `json:"bk_module_id"`
	ServiceTemplateID int                   `json:"service_template_id"`
	BkHostID          int                   `json:"bk_host_id"`
	ProcessInstances  []CMDBProcessInstance `json:"process_instances"`
}

// CMDBProcessInstance holds a single process instance within a service instance.
type CMDBProcessInstance struct {
	Property *CMDBProcessProperty `json:"process"`
	Relation *CMDBProcessRelation `json:"relation"`
}

// CMDBProcessProperty holds process property fields.
type CMDBProcessProperty struct {
	BkProcessID       int    `json:"bk_process_id"`
	BkProcessName     string `json:"bk_process_name"`
	BkFuncName        string `json:"bk_func_name"`
	BkStartParamRegex string `json:"bk_start_param_regex"`
	WorkPath          string `json:"work_path"`
	PidFile           string `json:"pid_file"`
	User              string `json:"user"`
	ReloadCmd         string `json:"reload_cmd"`
	RestartCmd        string `json:"restart_cmd"`
	StartCmd          string `json:"start_cmd"`
	StopCmd           string `json:"stop_cmd"`
	FaceStopCmd       string `json:"face_stop_cmd"`
	Timeout           int    `json:"timeout"`
	BkStartCheckSecs  int    `json:"bk_start_check_secs"`
	ProcNum           int    `json:"proc_num"`
}

// CMDBProcessRelation holds process relation fields.
type CMDBProcessRelation struct {
	BkBizID           int `json:"bk_biz_id"`
	BkProcessID       int `json:"bk_process_id"`
	ServiceInstanceID int `json:"service_instance_id"`
	ProcessTemplateID int `json:"process_template_id"`
	BkHostID          int `json:"bk_host_id"`
}

// CMDBSetInfo holds set info returned by find_set_batch API.
type CMDBSetInfo struct {
	BkSetID   int    `json:"bk_set_id"`
	BkSetName string `json:"bk_set_name"`
}

// CMDBModuleInfo holds module info returned by find_module_batch API.
type CMDBModuleInfo struct {
	BkModuleID   int    `json:"bk_module_id"`
	BkModuleName string `json:"bk_module_name"`
	BkSetID      int    `json:"bk_set_id"`
}

// CMDBProcessDetail holds detailed process fields from ListProcessDetailByIds API.
type CMDBProcessDetail struct {
	BkProcessID       int    `json:"bk_process_id"`
	BkProcessName     string `json:"bk_process_name"`
	BkFuncName        string `json:"bk_func_name"`
	BkStartParamRegex string `json:"bk_start_param_regex"`
	WorkPath          string `json:"work_path"`
	PidFile           string `json:"pid_file"`
	User              string `json:"user"`
	ReloadCmd         string `json:"reload_cmd"`
	RestartCmd        string `json:"restart_cmd"`
	StartCmd          string `json:"start_cmd"`
	StopCmd           string `json:"stop_cmd"`
	FaceStopCmd       string `json:"face_stop_cmd"`
	Timeout           int    `json:"timeout"`
	BkStartCheckSecs  int    `json:"bk_start_check_secs"`
	ProcNum           int    `json:"proc_num"`
}

// CMDBHostInfo holds host info returned by ListBizHosts API.
type CMDBHostInfo struct {
	BkHostID      int    `json:"bk_host_id"`
	BkHostInnerIP string `json:"bk_host_innerip"`
	BkCloudID     int    `json:"bk_cloud_id"`
	BkAgentID     string `json:"bk_agent_id"`
}

// ----- CMDB base response -----

type cmdbBaseResp struct {
	Result  bool            `json:"result"`
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// ----- Real CMDB client implementation -----

type realCMDBClient struct {
	cfg    *config.CMDBConfig
	client *http.Client
}

// NewRealCMDBClient creates a real CMDB client.
func NewRealCMDBClient(cfg *config.CMDBConfig) CMDBClient {
	return &realCMDBClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *realCMDBClient) doRequest(ctx context.Context, url string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body failed: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bkapi-Authorization", fmt.Sprintf(
		`{"bk_app_code":"%s","bk_app_secret":"%s","bk_username":"%s"}`,
		c.cfg.AppCode, c.cfg.AppSecret, c.cfg.Username))

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response failed: %w", err)
	}

	var baseResp cmdbBaseResp
	if err := json.Unmarshal(respBody, &baseResp); err != nil {
		return fmt.Errorf("unmarshal response failed: %w, body: %s", err, string(respBody))
	}

	if !baseResp.Result {
		return fmt.Errorf("CMDB API error: code=%d, message=%s", baseResp.Code, baseResp.Message)
	}

	if result != nil && baseResp.Data != nil {
		if err := json.Unmarshal(baseResp.Data, result); err != nil {
			return fmt.Errorf("unmarshal data failed: %w", err)
		}
	}

	return nil
}

// cmdbPagedResp is a generic paged response wrapper.
type cmdbPagedResp[T any] struct {
	Count int `json:"count"`
	Info  []T `json:"info"`
}

// maxBatchSize is the max number of IDs per CMDB API call.
const maxBatchSize = 500

// ListServiceInstanceDetail calls list_service_instance_detail API.
// API: POST /api/v3/findmany/proc/service_instance/details
// Request: { bk_biz_id, service_instance_ids, page }
// Response data: { count, info: [ServiceInstanceInfo] } where each item includes process_instances.
func (c *realCMDBClient) ListServiceInstanceDetail(ctx context.Context, bizID uint32, svcInstIDs []int64) (
	map[int64]*CMDBServiceInstance, error) {

	result := make(map[int64]*CMDBServiceInstance, len(svcInstIDs))
	if len(svcInstIDs) == 0 {
		return result, nil
	}

	url := fmt.Sprintf("%s/api/v3/findmany/proc/service_instance/details", c.cfg.Endpoint)

	for start := 0; start < len(svcInstIDs); start += maxBatchSize {
		end := start + maxBatchSize
		if end > len(svcInstIDs) {
			end = len(svcInstIDs)
		}
		batch := svcInstIDs[start:end]

		reqBody := map[string]interface{}{
			"bk_biz_id":            bizID,
			"service_instance_ids": batch,
			"page": map[string]interface{}{
				"start": 0,
				"limit": len(batch),
			},
		}

		var paged cmdbPagedResp[CMDBServiceInstance]
		if err := c.doRequest(ctx, url, reqBody, &paged); err != nil {
			return nil, fmt.Errorf("ListServiceInstanceDetail for biz %d failed: %w", bizID, err)
		}

		for i := range paged.Info {
			svcInst := &paged.Info[i]
			result[int64(svcInst.ID)] = svcInst
		}
	}

	log.Printf("  [CMDB] ListServiceInstanceDetail: biz=%d, requested=%d, got=%d",
		bizID, len(svcInstIDs), len(result))
	return result, nil
}

// FindSetBatch calls find_set_batch API.
// API: POST /api/v3/findmany/set/bk_biz_id/{bk_biz_id}
// Request: { bk_ids, fields }
// Response data: [SetInfo]
func (c *realCMDBClient) FindSetBatch(ctx context.Context, bizID uint32, setIDs []int64) (
	map[int64]string, error) {

	result := make(map[int64]string, len(setIDs))
	if len(setIDs) == 0 {
		return result, nil
	}

	url := fmt.Sprintf("%s/api/v3/findmany/set/bk_biz_id/%d", c.cfg.Endpoint, bizID)

	for start := 0; start < len(setIDs); start += maxBatchSize {
		end := start + maxBatchSize
		if end > len(setIDs) {
			end = len(setIDs)
		}
		batch := setIDs[start:end]

		// Convert []int64 to []int for CMDB API
		bkIDs := make([]int, len(batch))
		for i, id := range batch {
			bkIDs[i] = int(id)
		}

		reqBody := map[string]interface{}{
			"bk_ids": bkIDs,
			"fields": []string{"bk_set_id", "bk_set_name"},
		}

		var sets []CMDBSetInfo
		if err := c.doRequest(ctx, url, reqBody, &sets); err != nil {
			return nil, fmt.Errorf("FindSetBatch for biz %d failed: %w", bizID, err)
		}

		for _, s := range sets {
			result[int64(s.BkSetID)] = s.BkSetName
		}
	}

	log.Printf("  [CMDB] FindSetBatch: biz=%d, requested=%d, got=%d",
		bizID, len(setIDs), len(result))
	return result, nil
}

// findModuleBatchRaw calls find_module_batch API and returns full module info.
func (c *realCMDBClient) findModuleBatchRaw(ctx context.Context, bizID uint32, moduleIDs []int64) (
	map[int64]*CMDBModuleInfo, error) {

	result := make(map[int64]*CMDBModuleInfo, len(moduleIDs))
	if len(moduleIDs) == 0 {
		return result, nil
	}

	url := fmt.Sprintf("%s/api/v3/findmany/module/bk_biz_id/%d", c.cfg.Endpoint, bizID)

	for start := 0; start < len(moduleIDs); start += maxBatchSize {
		end := start + maxBatchSize
		if end > len(moduleIDs) {
			end = len(moduleIDs)
		}
		batch := moduleIDs[start:end]

		bkIDs := make([]int, len(batch))
		for i, id := range batch {
			bkIDs[i] = int(id)
		}

		reqBody := map[string]interface{}{
			"bk_ids": bkIDs,
			"fields": []string{"bk_module_id", "bk_module_name", "bk_set_id"},
		}

		var modules []CMDBModuleInfo
		if err := c.doRequest(ctx, url, reqBody, &modules); err != nil {
			return nil, fmt.Errorf("FindModuleBatch for biz %d failed: %w", bizID, err)
		}

		for i := range modules {
			m := &modules[i]
			result[int64(m.BkModuleID)] = m
		}
	}

	log.Printf("  [CMDB] FindModuleBatch: biz=%d, requested=%d, got=%d",
		bizID, len(moduleIDs), len(result))
	return result, nil
}

// FindModuleBatch calls find_module_batch API and returns module_id → module_name.
func (c *realCMDBClient) FindModuleBatch(ctx context.Context, bizID uint32, moduleIDs []int64) (
	map[int64]string, error) {

	raw, err := c.findModuleBatchRaw(ctx, bizID, moduleIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[int64]string, len(raw))
	for id, mi := range raw {
		result[id] = mi.BkModuleName
	}
	return result, nil
}

// ListProcessDetailByIds calls the process instance detail API.
// API: POST /api/v3/findmany/proc/process_instance/detail/biz/{bk_biz_id}
// Request: { bk_process_ids }
// Response data: [CMDBProcessDetail]  (direct array, not paged)
func (c *realCMDBClient) ListProcessDetailByIds(ctx context.Context, bizID uint32, processIDs []int64) (
	map[int64]*CMDBProcessDetail, error) {

	result := make(map[int64]*CMDBProcessDetail, len(processIDs))
	if len(processIDs) == 0 {
		return result, nil
	}

	url := fmt.Sprintf("%s/api/v3/findmany/proc/process_instance/detail/biz/%d", c.cfg.Endpoint, bizID)

	for start := 0; start < len(processIDs); start += maxBatchSize {
		end := start + maxBatchSize
		if end > len(processIDs) {
			end = len(processIDs)
		}
		batch := processIDs[start:end]

		bkIDs := make([]int, len(batch))
		for i, id := range batch {
			bkIDs[i] = int(id)
		}

		reqBody := map[string]interface{}{
			"bk_process_ids": bkIDs,
		}

		var details []CMDBProcessDetail
		if err := c.doRequest(ctx, url, reqBody, &details); err != nil {
			return nil, fmt.Errorf("ListProcessDetailByIds for biz %d failed: %w", bizID, err)
		}

		for i := range details {
			detail := &details[i]
			result[int64(detail.BkProcessID)] = detail
		}
	}

	log.Printf("  [CMDB] ListProcessDetailByIds: biz=%d, requested=%d, got=%d",
		bizID, len(processIDs), len(result))
	return result, nil
}

// ListBizHosts calls the list biz hosts API.
// API: POST /api/v3/hosts/app/{bk_biz_id}/list_hosts
// When moduleIDs is non-empty, adds bk_module_ids filter (max 500 per request).
func (c *realCMDBClient) ListBizHosts(ctx context.Context, bizID uint32, moduleIDs []int64) (
	map[int64]*CMDBHostInfo, error) {

	result := make(map[int64]*CMDBHostInfo)
	url := fmt.Sprintf("%s/api/v3/hosts/app/%d/list_hosts", c.cfg.Endpoint, bizID)
	fields := []string{"bk_host_id", "bk_host_innerip", "bk_cloud_id", "bk_agent_id"}

	var moduleChunks [][]int64
	if len(moduleIDs) > 0 {
		for i := 0; i < len(moduleIDs); i += maxBatchSize {
			end := i + maxBatchSize
			if end > len(moduleIDs) {
				end = len(moduleIDs)
			}
			moduleChunks = append(moduleChunks, moduleIDs[i:end])
		}
	} else {
		moduleChunks = [][]int64{nil}
	}

	for _, chunk := range moduleChunks {
		start := 0
		for {
			reqBody := map[string]interface{}{
				"page": map[string]interface{}{
					"start": start,
					"limit": maxBatchSize,
				},
				"fields": fields,
			}
			if chunk != nil {
				bkModuleIDs := make([]int, len(chunk))
				for i, id := range chunk {
					bkModuleIDs[i] = int(id)
				}
				reqBody["bk_module_ids"] = bkModuleIDs
			}

			var paged cmdbPagedResp[CMDBHostInfo]
			if err := c.doRequest(ctx, url, reqBody, &paged); err != nil {
				return nil, fmt.Errorf("ListBizHosts for biz %d failed: %w", bizID, err)
			}

			for i := range paged.Info {
				host := &paged.Info[i]
				result[int64(host.BkHostID)] = host
			}

			if len(paged.Info) < maxBatchSize {
				break
			}
			start += maxBatchSize
		}
	}

	log.Printf("  [CMDB] ListBizHosts: biz=%d, moduleFilter=%d, got=%d",
		bizID, len(moduleIDs), len(result))
	return result, nil
}

// ----- Mock CMDB client (skip_cmdb mode) -----

type mockCMDBClient struct{}

// NewMockCMDBClient creates a mock CMDB client that returns empty values.
func NewMockCMDBClient() CMDBClient {
	return &mockCMDBClient{}
}

func (c *mockCMDBClient) ListServiceInstanceDetail(_ context.Context, _ uint32, _ []int64) (
	map[int64]*CMDBServiceInstance, error) {
	return make(map[int64]*CMDBServiceInstance), nil
}

func (c *mockCMDBClient) FindSetBatch(_ context.Context, _ uint32, _ []int64) (map[int64]string, error) {
	return make(map[int64]string), nil
}

func (c *mockCMDBClient) FindModuleBatch(_ context.Context, _ uint32, _ []int64) (map[int64]string, error) {
	return make(map[int64]string), nil
}

func (c *mockCMDBClient) ListProcessDetailByIds(_ context.Context, _ uint32, _ []int64) (
	map[int64]*CMDBProcessDetail, error) {
	return make(map[int64]*CMDBProcessDetail), nil
}

func (c *mockCMDBClient) ListBizHosts(_ context.Context, _ uint32, _ []int64) (map[int64]*CMDBHostInfo, error) {
	return make(map[int64]*CMDBHostInfo), nil
}
