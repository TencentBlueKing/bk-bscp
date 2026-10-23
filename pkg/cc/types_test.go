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
	"math"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestCMDBRenderCacheDefaultDurations(t *testing.T) {
	var conf CMDBRenderCacheConfig
	conf.trySetDefault()

	if got, err := conf.BuildWaitTTLDuration(); err != nil || got != 30*time.Second {
		t.Fatalf("build wait ttl = %v, err: %v, want %v", got, err, 30*time.Second)
	}
	if got, err := conf.BuildTimeoutDuration(); err != nil || got != 5*time.Minute {
		t.Fatalf("build timeout = %v, err: %v, want %v", got, err, 5*time.Minute)
	}
	if got, err := conf.BuildLockTTLDuration(); err != nil || got != 30*time.Second {
		t.Fatalf("build lock ttl = %v, err: %v, want %v", got, err, 30*time.Second)
	}
}

func TestCMDBRenderCacheDefaultWaitTTLKeepsConfiguredLockCompatibility(t *testing.T) {
	var conf CMDBRenderCacheConfig
	if err := yaml.Unmarshal([]byte("buildLockTTL: 45s\n"), &conf); err != nil {
		t.Fatalf("unmarshal render cache config failed: %v", err)
	}
	conf.trySetDefault()

	if got, err := conf.BuildWaitTTLDuration(); err != nil || got != 45*time.Second {
		t.Fatalf("build wait ttl = %v, err: %v, want %v", got, err, 45*time.Second)
	}
}

func TestCMDBRenderCacheBuildTimeoutCanBeConfiguredSeparately(t *testing.T) {
	var conf CMDBRenderCacheConfig
	if err := yaml.Unmarshal([]byte("buildWaitTTL: 10s\nbuildTimeout: 2m\nbuildLockTTL: 2m\n"), &conf); err != nil {
		t.Fatalf("unmarshal render cache config failed: %v", err)
	}
	conf.trySetDefault()

	if got, err := conf.BuildWaitTTLDuration(); err != nil || got != 10*time.Second {
		t.Fatalf("build wait ttl = %v, err: %v, want %v", got, err, 10*time.Second)
	}
	if got, err := conf.BuildTimeoutDuration(); err != nil || got != 2*time.Minute {
		t.Fatalf("build timeout = %v, err: %v, want %v", got, err, 2*time.Minute)
	}
}

func TestWatchCmdbResourceDefaultDisabled(t *testing.T) {
	var conf CrontabConfig
	conf.trySetDefault()

	if conf.WatchCmdbResource.Enabled {
		t.Fatalf("expected watchCmdbResource to be disabled by default")
	}
}

func TestSyncCmdbGseConfigDefaults(t *testing.T) {
	var conf SyncCmdbGseConfig
	conf.trySetDefault()

	if conf.Interval != "30m" {
		t.Fatalf("interval = %q, want 30m", conf.Interval)
	}
	if conf.BizBatchSize != 10 {
		t.Fatalf("bizBatchSize = %d, want 10", conf.BizBatchSize)
	}
}

func TestSyncCmdbGseConfigRejectsNegativeBizBatchSize(t *testing.T) {
	conf := SyncCmdbGseConfig{BizBatchSize: -1}
	conf.trySetDefault()
	if conf.BizBatchSize != -1 {
		t.Fatalf("trySetDefault should not rewrite negative bizBatchSize, got %d", conf.BizBatchSize)
	}
	if err := conf.validate(); err == nil {
		t.Fatal("expected validate to reject negative bizBatchSize")
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

func TestWatchCmdbResourceDefaults(t *testing.T) {
	var conf WatchCmdbResourceConfig
	conf.trySetDefault()

	if conf.Interval != "1m" {
		t.Fatalf("interval = %q, want 1m", conf.Interval)
	}
	if conf.IdleInterval != "1s" {
		t.Fatalf("idleInterval = %q, want 1s", conf.IdleInterval)
	}
	if conf.MaxConcurrency != 8 {
		t.Fatalf("maxConcurrency = %d, want 8", conf.MaxConcurrency)
	}
	if conf.QpsLimit != 20.0 {
		t.Fatalf("qpsLimit = %f, want 20", conf.QpsLimit)
	}
	if conf.HandlerConcurrency != 4 {
		t.Fatalf("handlerConcurrency = %d, want 4", conf.HandlerConcurrency)
	}
	if conf.StartFromOffset != "5m" {
		t.Fatalf("startFromOffset = %q, want 5m", conf.StartFromOffset)
	}
	if conf.RequestTimeout != "35s" {
		t.Fatalf("requestTimeout = %q, want 35s", conf.RequestTimeout)
	}
	if conf.MaxWatchChains != 100 {
		t.Fatalf("maxWatchChains = %d, want 100", conf.MaxWatchChains)
	}
	if err := conf.validate(); err != nil {
		t.Fatalf("validate default config failed: %v", err)
	}
}

func TestWatchCmdbResourceKeepsConfiguredValues(t *testing.T) {
	raw := `
watchCmdbResource:
  enabled: true
  interval: 2m
  idleInterval: 3s
  maxConcurrency: 16
  qpsLimit: 40
  handlerConcurrency: 8
  startFromOffset: 10m
  requestTimeout: 40s
`
	var conf CrontabConfig
	if err := yaml.Unmarshal([]byte(raw), &conf); err != nil {
		t.Fatalf("unmarshal crontab config failed: %v", err)
	}
	conf.trySetDefault()

	watch := conf.WatchCmdbResource
	if watch.MaxConcurrency != 16 {
		t.Fatalf("maxConcurrency = %d, want 16", watch.MaxConcurrency)
	}
	if watch.RequestTimeout != "40s" {
		t.Fatalf("requestTimeout = %q, want 40s", watch.RequestTimeout)
	}
	if err := watch.validate(); err != nil {
		t.Fatalf("validate configured config failed: %v", err)
	}
}

// TestWatchCmdbResourceDisabledSkipsLongPollingTimeout 未启用 watch 时，
// 历史短超时配置不应阻塞服务启动（运行期不会使用该值）
func TestWatchCmdbResourceDisabledSkipsLongPollingTimeout(t *testing.T) {
	conf := WatchCmdbResourceConfig{Enabled: false, RequestTimeout: "10s"}
	if err := conf.validate(); err != nil {
		t.Fatalf("disabled watch should not enforce requestTimeout, got: %v", err)
	}
}

// TestWatchCmdbResourceIgnoresRemovedResourceConfig watch 资源类型与 bk_fields 已写死在代码内，
// 存量部署 yaml 中残留的 resources/fields 应被忽略，不影响启动
func TestWatchCmdbResourceIgnoresRemovedResourceConfig(t *testing.T) {
	raw := `
watchCmdbResource:
  enabled: true
  resources:
    - set
    - module
  fields:
    module: ["bk_module_id"]
`
	var conf CrontabConfig
	if err := yaml.Unmarshal([]byte(raw), &conf); err != nil {
		t.Fatalf("legacy watchCmdbResource keys should be ignored, got: %v", err)
	}
	conf.trySetDefault()
	if err := conf.validate(); err != nil {
		t.Fatalf("validate legacy config failed: %v", err)
	}
}

func TestWatchCmdbResourceValidateRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name string
		conf WatchCmdbResourceConfig
	}{
		{"requestTimeout too short", WatchCmdbResourceConfig{Enabled: true, RequestTimeout: "10s"}},
		{"requestTimeout at boundary", WatchCmdbResourceConfig{Enabled: true, RequestTimeout: "20s"}},
		{"invalid interval", WatchCmdbResourceConfig{Interval: "abc"}},
		{"invalid idleInterval", WatchCmdbResourceConfig{IdleInterval: "xyz"}},
		{"negative maxConcurrency", WatchCmdbResourceConfig{MaxConcurrency: -1}},
		{"negative handlerConcurrency", WatchCmdbResourceConfig{HandlerConcurrency: -1}},
		{"negative qpsLimit", WatchCmdbResourceConfig{QpsLimit: -1}},
		{"NaN qpsLimit", WatchCmdbResourceConfig{QpsLimit: math.NaN()}},
		{"infinite qpsLimit", WatchCmdbResourceConfig{QpsLimit: math.Inf(1)}},
		{"zero interval", WatchCmdbResourceConfig{Interval: "0s"}},
		{"negative offset", WatchCmdbResourceConfig{StartFromOffset: "-1m"}},
		{"startFromOffset above cap", WatchCmdbResourceConfig{Enabled: true, StartFromOffset: "2h"}},
		{"negative maxWatchChains", WatchCmdbResourceConfig{MaxWatchChains: -1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.conf.validate(); err == nil {
				t.Fatalf("expected validate to reject %s", c.name)
			}
		})
	}
}

// TestWatchCmdbResourceStartFromOffsetCap 游标不落库，startFromOffset 是重启后唯一的
// 回溯手段：窗口越大重放越多，因此限制上限；上限值本身允许，未启用时不校验。
func TestWatchCmdbResourceStartFromOffsetCap(t *testing.T) {
	if err := (WatchCmdbResourceConfig{Enabled: true, StartFromOffset: "1h"}).validate(); err != nil {
		t.Fatalf("startFromOffset at cap should be accepted, got: %v", err)
	}
	// 未启用时不应因运行期不生效的字段阻塞启动
	if err := (WatchCmdbResourceConfig{Enabled: false, StartFromOffset: "2h"}).validate(); err != nil {
		t.Fatalf("disabled watch should not enforce startFromOffset cap, got: %v", err)
	}
}

func TestFeatureFlagsTrySetDefault_TenantModeDefaults(t *testing.T) {
	var ff FeatureFlags
	ff.trySetDefault()

	if ff.TenantMode != TenantModeSingle {
		t.Fatalf("TenantMode = %q, want %q", ff.TenantMode, TenantModeSingle)
	}
	if ff.EnableMultiTenantMode {
		t.Fatalf("EnableMultiTenantMode should remain false when EnableTenantMode is false")
	}
}

func TestFeatureFlagsTrySetDefault_EnableTenantModeDerives(t *testing.T) {
	ff := FeatureFlags{EnableTenantMode: true}
	ff.trySetDefault()

	if !ff.EnableMultiTenantMode {
		t.Fatalf("EnableMultiTenantMode should be true when EnableTenantMode is true")
	}
	if ff.TenantMode != TenantModeSingle {
		t.Fatalf("TenantMode = %q, want %q", ff.TenantMode, TenantModeSingle)
	}
}

func TestFeatureFlagsTrySetDefault_ExplicitMultiMode(t *testing.T) {
	ff := FeatureFlags{EnableTenantMode: true, TenantMode: TenantModeMulti}
	ff.trySetDefault()

	if !ff.EnableMultiTenantMode {
		t.Fatalf("EnableMultiTenantMode should be true when EnableTenantMode is true")
	}
	if ff.TenantMode != TenantModeMulti {
		t.Fatalf("TenantMode = %q, want %q", ff.TenantMode, TenantModeMulti)
	}
}

func TestFeatureFlagsValidate_InvalidTenantMode(t *testing.T) {
	ff := FeatureFlags{EnableTenantMode: true, TenantMode: "invalid"}
	if err := ff.validate(); err == nil {
		t.Fatalf("expected error for invalid TenantMode, got nil")
	}
}

func TestFeatureFlagsValidate_ValidTenantModes(t *testing.T) {
	for _, mode := range []TenantMode{TenantModeSingle, TenantModeMulti} {
		ff := FeatureFlags{EnableTenantMode: true, TenantMode: mode}
		if err := ff.validate(); err != nil {
			t.Fatalf("unexpected error for TenantMode %q: %v", mode, err)
		}
	}
}

func TestFeatureFlagsValidate_DisabledTenantModeSkipsCheck(t *testing.T) {
	ff := FeatureFlags{EnableTenantMode: false, TenantMode: "whatever"}
	if err := ff.validate(); err != nil {
		t.Fatalf("expected no error when EnableTenantMode is false, got: %v", err)
	}
}
