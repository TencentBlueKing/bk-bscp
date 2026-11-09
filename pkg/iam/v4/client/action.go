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

// Action 操作。一个操作最多关联一个资源类型，且该资源类型须在本系统内已注册。
// ResourceTypeID 为空表示无关资源类型的操作，查询时该字段返回空字符串而非 null。
type Action struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ResourceTypeID string `json:"resource_type_id,omitempty"`
}

// UpdateActionReq 更新操作的请求体。
// 字段为空表示不修改，PUT 时该字段不会发送。
// 不含 resource_type_id：权限中心会忽略该字段（返回成功但不生效），关联资源类型只能删除重建。
type UpdateActionReq struct {
	Name string `json:"name,omitempty"`
}

// BatchCreateActions 批量创建操作，返回创建成功的 ID 列表。请求体顶层为数组。
func (c *Client) BatchCreateActions(kt *kit.Kit, actions []Action) ([]string, error) {
	var ids []string

	err := c.do(kt, &request{
		method: http.MethodPost,
		path:   fmt.Sprintf("/api/v1/open/rbac/model/systems/%s/actions/", c.cfg.SystemID),
		body:   actions,
		result: &ids,
	})
	if err != nil {
		return nil, err
	}

	return ids, nil
}

// ListActions 分页查询操作，返回当前页数据与总数。
func (c *Client) ListActions(kt *kit.Kit, page, pageSize int) ([]Action, int, error) {
	resp := new(struct {
		Count   int      `json:"count"`
		Results []Action `json:"results"`
	})

	err := c.do(kt, &request{
		method: http.MethodGet,
		path:   fmt.Sprintf("/api/v1/open/rbac/model/systems/%s/actions/", c.cfg.SystemID),
		query:  pageQuery(page, pageSize),
		result: resp,
	})
	if err != nil {
		return nil, 0, err
	}

	return resp.Results, resp.Count, nil
}

// UpdateAction 更新指定操作。
func (c *Client) UpdateAction(kt *kit.Kit, id string, req *UpdateActionReq) error {
	return c.do(kt, &request{
		method: http.MethodPut,
		path:   fmt.Sprintf("/api/v1/open/rbac/model/systems/%s/actions/%s/", c.cfg.SystemID, id),
		body:   req,
	})
}

// DeleteAction 删除指定操作。
func (c *Client) DeleteAction(kt *kit.Kit, id string) error {
	return c.do(kt, &request{
		method: http.MethodDelete,
		path:   fmt.Sprintf("/api/v1/open/rbac/model/systems/%s/actions/%s/", c.cfg.SystemID, id),
	})
}
