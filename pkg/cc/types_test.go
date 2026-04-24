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

package cc

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWatchCmdbResourceDefaultDisabled(t *testing.T) {
	var conf CrontabConfig
	conf.trySetDefault()

	if conf.WatchCmdbResource.Enabled {
		t.Fatalf("expected watchCmdbResource to be disabled by default")
	}
}

func TestWatchCmdbResourceCanBeEnabled(t *testing.T) {
	var conf CrontabConfig
	if err := yaml.Unmarshal([]byte("watchCmdbResource:\n  enabled: true\n"), &conf); err != nil {
		t.Fatalf("unmarshal crontab config failed: %v", err)
	}
	conf.trySetDefault()

	if !conf.WatchCmdbResource.Enabled {
		t.Fatalf("expected watchCmdbResource to be enabled when configured true")
	}
}
