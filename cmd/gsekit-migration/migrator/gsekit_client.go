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
	"net/http"
	"time"

	"github.com/TencentBlueKing/bk-bscp/cmd/gsekit-migration/config"
)

// GSEKitClient defines the interface for GSEKit API operations.
type GSEKitClient interface {
	// PreviewConfigTemplate calls GSEKit's config_version preview API.
	// It sends template content and bk_process_id, and returns the rendered result.
	PreviewConfigTemplate(ctx context.Context, bizID uint32, content string, bkProcessID int64) (string, error)
}

// realGSEKitClient is the real HTTP-based GSEKit API client.
type realGSEKitClient struct {
	cfg    *config.GSEKitConfig
	client *http.Client
}

// NewGSEKitClient creates a new GSEKit API client.
func NewGSEKitClient(cfg *config.GSEKitConfig) GSEKitClient {
	return &realGSEKitClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// gsekitBaseResp is the standard BK API Gateway response format.
// Code uses json.RawMessage because GSEKit returns it as either int or string.
type gsekitBaseResp struct {
	Result  bool            `json:"result"`
	Code    json.RawMessage `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// PreviewConfigTemplate calls:
//
//	POST {endpoint}/api/{biz_id}/config_version/preview/
//	Body: {"content": "<template>", "bk_process_id": <id>}
//	Header: X-Bkapi-Authorization: {"bk_app_code":"...", "bk_app_secret":"...", "bk_ticket":"..."}
//
// Returns the rendered content string from the response data.
func (c *realGSEKitClient) PreviewConfigTemplate(
	ctx context.Context, bizID uint32, content string, bkProcessID int64,
) (string, error) {
	url := fmt.Sprintf("%s/api/%d/config_version/preview/", c.cfg.Endpoint, bizID)

	reqBody := map[string]interface{}{
		"content":       content,
		"bk_process_id": bkProcessID,
	}
	bodyData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request body failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyData))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	authHeader := fmt.Sprintf(
		`{"bk_app_code":"%s","bk_app_secret":"%s","bk_ticket":"%s"}`,
		c.cfg.AppCode, c.cfg.AppSecret, c.cfg.BkTicket,
	)
	req.Header.Set("X-Bkapi-Authorization", authHeader)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var baseResp gsekitBaseResp
	if err := json.Unmarshal(respBody, &baseResp); err != nil {
		return "", fmt.Errorf("unmarshal response failed: %w, body: %s", err, truncateStr(string(respBody), 500))
	}

	if !baseResp.Result {
		return "", fmt.Errorf("GSEKit API error: code=%s, message=%s",
			string(baseResp.Code), baseResp.Message)
	}

	// data can be a plain string or an object like {"content": "..."}
	var rendered string
	if err := json.Unmarshal(baseResp.Data, &rendered); err == nil {
		return rendered, nil
	}

	var dataObj struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(baseResp.Data, &dataObj); err == nil && dataObj.Content != "" {
		return dataObj.Content, nil
	}

	return "", fmt.Errorf("unexpected response data format: %s", truncateStr(string(baseResp.Data), 500))
}
