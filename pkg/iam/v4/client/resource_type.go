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
	"strconv"

	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// ResourceType 资源类型。Ancestors 是从根到父级的资源类型 ID 列表，
// 如拓扑为 a→b→c 时 c 的 Ancestors 为 ["a","b"]。该字段没有 system 维度，
// 因此祖先链只能引用本系统内已注册的资源类型。
type ResourceType struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Ancestors []string `json:"ancestors,omitempty"`
}

// UpdateResourceTypeReq 更新资源类型的请求体。
// 字段为空表示不修改，PUT 时该字段不会发送；因此无法用空值清空祖先链（根资源类型不需要）。
type UpdateResourceTypeReq struct {
	Name string `json:"name,omitempty"`
	// Ancestors 祖先链（从根到父级）。服务资源类型需要从 [业务] 调整为
	// [业务, 项目, 环境]，由模型同步原地更新，避免删模型导致授权全部失效。
	Ancestors []string `json:"ancestors,omitempty"`
}

// BatchCreateResourceTypes 批量创建资源类型，返回创建成功的 ID 列表。请求体顶层为数组。
func (c *Client) BatchCreateResourceTypes(kt *kit.Kit, types []ResourceType) ([]string, error) {
	var ids []string

	err := c.do(kt, &request{
		method: http.MethodPost,
		path:   fmt.Sprintf("/api/v1/open/rbac/model/systems/%s/resource-types/", c.cfg.SystemID),
		body:   types,
		result: &ids,
	})
	if err != nil {
		return nil, err
	}

	return ids, nil
}

// ListResourceTypes 分页查询资源类型，返回当前页数据与总数。
func (c *Client) ListResourceTypes(kt *kit.Kit, page, pageSize int) ([]ResourceType, int, error) {
	resp := new(struct {
		Count   int            `json:"count"`
		Results []ResourceType `json:"results"`
	})

	err := c.do(kt, &request{
		method: http.MethodGet,
		path:   fmt.Sprintf("/api/v1/open/rbac/model/systems/%s/resource-types/", c.cfg.SystemID),
		query:  pageQuery(page, pageSize),
		result: resp,
	})
	if err != nil {
		return nil, 0, err
	}

	return resp.Results, resp.Count, nil
}

// UpdateResourceType 更新指定资源类型。
func (c *Client) UpdateResourceType(kt *kit.Kit, id string, req *UpdateResourceTypeReq) error {
	return c.do(kt, &request{
		method: http.MethodPut,
		path: fmt.Sprintf("/api/v1/open/rbac/model/systems/%s/resource-types/%s/",
			c.cfg.SystemID, id),
		body: req,
	})
}

// DeleteResourceType 删除指定资源类型。
func (c *Client) DeleteResourceType(kt *kit.Kit, id string) error {
	return c.do(kt, &request{
		method: http.MethodDelete,
		path: fmt.Sprintf("/api/v1/open/rbac/model/systems/%s/resource-types/%s/",
			c.cfg.SystemID, id),
	})
}

// pageQuery 构造分页查询参数，pageSize 超出上限时收敛到 MaxPageSize。
func pageQuery(page, pageSize int) map[string]string {
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 || pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	return map[string]string{
		"page":      strconv.Itoa(page),
		"page_size": strconv.Itoa(pageSize),
	}
}
