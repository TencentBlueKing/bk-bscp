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

package ratelimit

import (
	"fmt"

	"github.com/TencentBlueKing/bk-bscp/internal/components"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

// Setup wires the component limiter for a service after settings are loaded.
func Setup(serviceName cc.Name) error {
	cfg, err := resolveServiceConfig(serviceName)
	if err != nil {
		return err
	}

	if !cfg.Enabled {
		components.SetComponentLimiter(nil)
		return nil
	}

	bds, redisErr := bedis.NewRedisCache(cfg.RedisCluster)
	if redisErr != nil {
		return fmt.Errorf("init component rate limit redis failed: %w", redisErr)
	}
	limiter, err := NewRedisLimiter(cfg, bds)
	if err != nil {
		return err
	}

	components.SetComponentLimiter(limiter)
	return nil
}

func resolveServiceConfig(serviceName cc.Name) (cc.ComponentRateLimit, error) {
	switch serviceName {
	case cc.DataServiceName:
		return cc.DataService().ComponentRateLimit, nil
	case cc.FeedServerName:
		return cc.FeedServer().ComponentRateLimit, nil
	case cc.AuthServerName:
		return cc.AuthServer().ComponentRateLimit, nil
	case cc.APIServerName:
		return cc.ApiServer().ComponentRateLimit, nil
	default:
		return cc.ComponentRateLimit{}, fmt.Errorf("component rate limit is unsupported for service %s", serviceName)
	}
}
