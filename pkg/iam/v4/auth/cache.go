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

package auth

import (
	"fmt"
	"time"

	"github.com/bluele/gcache"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/adaptor"
)

// noPermissionTTL 是无权限结果的缓存时长。
// 取值明显短于有权限结果：用户拿到权限后应尽快生效，而已有权限被回收的紧迫性较低。
// 与 feed-server 的本地鉴权缓存采用同样的策略。
const noPermissionTTL = 30 * time.Second

// decisionCache 缓存鉴权结果。
//
// IAM V3 拉回的是判定规则，一次请求可对任意多个资源反复本地求值；IAM V4 拿到的是绑定到
// (action, resource) 的判定结果，无法复用，请求数随资源条数线性增长。
// 加上批量接口单次上限 20 条，服务列表页的请求数比 V3 明显上升，缓存因此是必需的。
type decisionCache struct {
	client gcache.Cache
	ttl    time.Duration
}

// newDecisionCache 构造鉴权结果缓存。size 或 ttl 为非正数时返回 nil，表示禁用缓存。
func newDecisionCache(size int, ttl time.Duration) *decisionCache {
	if size <= 0 || ttl <= 0 {
		return nil
	}

	return &decisionCache{
		client: gcache.New(size).LRU().Expiration(ttl).Build(),
		ttl:    ttl,
	}
}

// get 读缓存。第二个返回值为 false 表示未命中。
func (c *decisionCache) get(key string) (bool, bool) {
	if c == nil {
		return false, false
	}

	val, err := c.client.GetIFPresent(key)
	if err != nil {
		return false, false
	}

	allowed, ok := val.(bool)
	if !ok {
		return false, false
	}

	return allowed, true
}

// set 写缓存。无权限的结果用更短的 TTL，让授权后能较快生效。
func (c *decisionCache) set(key string, allowed bool) {
	if c == nil {
		return
	}

	if allowed {
		_ = c.client.Set(key, allowed)

		return
	}

	ttl := noPermissionTTL
	if c.ttl < ttl {
		ttl = c.ttl
	}

	_ = c.client.SetWithExpire(key, allowed, ttl)
}

// cacheKey 构造缓存键。
// 必须包含租户：多租户下同名用户属于不同租户，混用会导致跨租户的权限泄漏。
// 资源类型与实例 ID 都要带上，因为 IAM V4 的判定结果是绑定到具体资源实例的；
// 归属（项目/环境）也要带上，资源被移动到其他环境后旧路径上的判定不能继续命中。
func cacheKey(tenantID, user string, m *adaptor.Mapped) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s", tenantID, user,
		m.ActionID, m.ResourceType, m.ResourceID, m.ProjectID, m.EnvironmentID)
}
