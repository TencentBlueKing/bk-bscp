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

package components

import (
	"context"
	"sync"
)

// Limiter throttles outbound requests by component.
type Limiter interface {
	Acquire(ctx context.Context, component string) error
}

var (
	limiterMu        sync.RWMutex
	componentLimiter Limiter
)

// SetComponentLimiter replaces the global component limiter.
func SetComponentLimiter(limiter Limiter) {
	limiterMu.Lock()
	defer limiterMu.Unlock()

	componentLimiter = limiter
}

// GetComponentLimiter returns the global component limiter.
func GetComponentLimiter() Limiter {
	limiterMu.RLock()
	defer limiterMu.RUnlock()

	return componentLimiter
}
