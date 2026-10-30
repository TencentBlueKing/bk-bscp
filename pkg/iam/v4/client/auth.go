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

package client

import (
	"fmt"
	"net/http"

	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// IAMPathAttrKey 资源属性中承载拓扑路径的键。
const IAMPathAttrKey = "_bk_iam_path_"

// BuildIAMPath 构造拓扑路径，格式为 /Type,ID/Type,ID/。
// 参数按"从根到叶"的顺序成对给出，例如 BuildIAMPath("biz", "1", "project", "2")。
// 任一层级缺失（空类型或空 ID）时返回空串：调用方据此不携带 attributes，
// 权限中心按无拓扑处理，鉴权结果为无权限，避免把不完整的路径发给权限中心。
func BuildIAMPath(typesAndIDs ...string) string {
	if len(typesAndIDs) < 2 || len(typesAndIDs)%2 != 0 {
		return ""
	}

	path := ""
	for i := 0; i < len(typesAndIDs); i += 2 {
		typ, id := typesAndIDs[i], typesAndIDs[i+1]
		if typ == "" || id == "" {
			return ""
		}

		path += fmt.Sprintf("/%s,%s", typ, id)
	}

	// 每级形如 /Type,ID，整体以 "/" 收尾：/biz,1/project,2/
	return path + "/"
}

// AuthResource 鉴权请求中的资源。资源类型由 action_id 隐含决定，无需在此声明。
type AuthResource struct {
	// ID 资源实例 ID
	ID string `json:"id"`
	// Attributes 资源属性，用于承载 _bk_iam_path_ 声明资源的归属拓扑
	Attributes map[string]string `json:"attributes,omitempty"`
}

// NewAuthResource 构造一个带拓扑路径的鉴权资源，iamPath 为空时不携带 attributes。
func NewAuthResource(id, iamPath string) AuthResource {
	res := AuthResource{ID: id}
	if iamPath != "" {
		res.Attributes = map[string]string{IAMPathAttrKey: iamPath}
	}

	return res
}

type authReq struct {
	Subject  Subject       `json:"subject"`
	ActionID string        `json:"action_id"`
	Resource *AuthResource `json:"resource,omitempty"`
}

type authByActionsReq struct {
	Subject   Subject       `json:"subject"`
	ActionIDs []string      `json:"action_ids"`
	Resource  *AuthResource `json:"resource,omitempty"`
}

type authByResourcesReq struct {
	Subject   Subject        `json:"subject"`
	ActionID  string         `json:"action_id"`
	Resources []AuthResource `json:"resources"`
}

// Auth 判断主体对单个资源是否有指定操作的权限。无关资源类型的操作传 res 为 nil。
func (c *Client) Auth(kt *kit.Kit, subject Subject, actionID string, res *AuthResource) (
	bool, error) {

	resp := new(struct {
		Allowed bool `json:"allowed"`
	})

	err := c.do(kt, &request{
		method: http.MethodPost,
		path:   fmt.Sprintf("/api/v1/open/rbac/authorization/systems/%s/auth/", c.cfg.SystemID),
		body:   &authReq{Subject: subject, ActionID: actionID, Resource: res},
		result: resp,
	})
	if err != nil {
		return false, err
	}

	return resp.Allowed, nil
}

// AuthByActions 判断主体对单个资源的多个操作是否有权限，返回以操作 ID 为键的结果。
// 同批操作必须关联相同的资源类型，或均为无关资源类型的操作。
func (c *Client) AuthByActions(kt *kit.Kit, subject Subject, actionIDs []string,
	res *AuthResource) (map[string]bool, error) {

	if len(actionIDs) > MaxAuthBatchSize {
		return nil, fmt.Errorf("action count %d exceeds the limit %d",
			len(actionIDs), MaxAuthBatchSize)
	}

	var results []struct {
		ActionID string `json:"action_id"`
		Allowed  bool   `json:"allowed"`
	}

	err := c.do(kt, &request{
		method: http.MethodPost,
		path: fmt.Sprintf("/api/v1/open/rbac/authorization/systems/%s/auth-by-actions/",
			c.cfg.SystemID),
		body:   &authByActionsReq{Subject: subject, ActionIDs: actionIDs, Resource: res},
		result: &results,
	})
	if err != nil {
		return nil, err
	}

	decisions := make(map[string]bool, len(results))
	for _, r := range results {
		decisions[r.ActionID] = r.Allowed
	}

	return decisions, nil
}

// AuthByResources 判断主体对多个资源的同一操作是否有权限，返回以资源 ID 为键的结果。
func (c *Client) AuthByResources(kt *kit.Kit, subject Subject, actionID string,
	resources []AuthResource) (map[string]bool, error) {

	if len(resources) > MaxAuthBatchSize {
		return nil, fmt.Errorf("resource count %d exceeds the limit %d",
			len(resources), MaxAuthBatchSize)
	}

	var results []struct {
		ResourceID string `json:"resource_id"`
		Allowed    bool   `json:"allowed"`
	}

	err := c.do(kt, &request{
		method: http.MethodPost,
		path: fmt.Sprintf("/api/v1/open/rbac/authorization/systems/%s/auth-by-resources/",
			c.cfg.SystemID),
		body:   &authByResourcesReq{Subject: subject, ActionID: actionID, Resources: resources},
		result: &results,
	})
	if err != nil {
		return nil, err
	}

	decisions := make(map[string]bool, len(results))
	for _, r := range results {
		decisions[r.ResourceID] = r.Allowed
	}

	return decisions, nil
}
