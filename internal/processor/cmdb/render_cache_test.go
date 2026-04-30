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

package cmdb

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRenderCacheStore struct {
	values  map[string]string
	hashes  map[string]map[string]string
	ttl     map[string]time.Duration
	renewed []string
	deleted []string
	err     error
}

func newFakeRenderCacheStore() *fakeRenderCacheStore {
	return &fakeRenderCacheStore{
		values: make(map[string]string),
		hashes: make(map[string]map[string]string),
		ttl:    make(map[string]time.Duration),
	}
}

func (s *fakeRenderCacheStore) Get(_ context.Context, key string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.values[key], nil
}

func (s *fakeRenderCacheStore) Set(_ context.Context, key string, value interface{}, ttlSeconds int) error {
	if s.err != nil {
		return s.err
	}
	s.values[key] = value.(string)
	s.ttl[key] = time.Duration(ttlSeconds) * time.Second
	return nil
}

func (s *fakeRenderCacheStore) SetWithDuration(_ context.Context, key string, value interface{}, ttl time.Duration) error {
	if s.err != nil {
		return s.err
	}
	s.values[key] = value.(string)
	s.ttl[key] = ttl
	return nil
}

func (s *fakeRenderCacheStore) HGet(_ context.Context, hashKey string, field string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	fields := s.hashes[hashKey]
	if fields == nil {
		return "", nil
	}
	return fields[field], nil
}

func (s *fakeRenderCacheStore) HSets(_ context.Context, hashKey string, kv map[string]string, ttlSeconds int) error {
	if s.err != nil {
		return s.err
	}
	if s.hashes[hashKey] == nil {
		s.hashes[hashKey] = make(map[string]string)
	}
	for k, v := range kv {
		s.hashes[hashKey][k] = v
	}
	s.ttl[hashKey] = time.Duration(ttlSeconds) * time.Second
	return nil
}

func (s *fakeRenderCacheStore) HSetsWithDuration(_ context.Context,
	hashKey string, kv map[string]string, ttl time.Duration) error {
	if s.err != nil {
		return s.err
	}
	if s.hashes[hashKey] == nil {
		s.hashes[hashKey] = make(map[string]string)
	}
	for k, v := range kv {
		s.hashes[hashKey][k] = v
	}
	s.ttl[hashKey] = ttl
	return nil
}

func (s *fakeRenderCacheStore) HGetAll(_ context.Context, hashKey string) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	fields := s.hashes[hashKey]
	if fields == nil {
		return map[string]string{}, nil
	}
	result := make(map[string]string, len(fields))
	for field, value := range fields {
		result[field] = value
	}
	return result, nil
}

func (s *fakeRenderCacheStore) SetNX(_ context.Context, key string, value interface{}, ttlSeconds int) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if _, exists := s.values[key]; exists {
		return false, nil
	}
	s.values[key] = value.(string)
	s.ttl[key] = time.Duration(ttlSeconds) * time.Second
	return true, nil
}

func (s *fakeRenderCacheStore) SetNXWithDuration(
	_ context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if _, exists := s.values[key]; exists {
		return false, nil
	}
	s.values[key] = value.(string)
	s.ttl[key] = ttl
	return true, nil
}

func (s *fakeRenderCacheStore) RenewLockWithDuration(
	_ context.Context, key string, value string, ttl time.Duration) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if s.values[key] != value {
		return false, nil
	}
	s.ttl[key] = ttl
	s.renewed = append(s.renewed, key)
	return true, nil
}

func (s *fakeRenderCacheStore) ReleaseLock(_ context.Context, key string, value string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if s.values[key] != value {
		return false, nil
	}
	delete(s.values, key)
	delete(s.ttl, key)
	s.deleted = append(s.deleted, key)
	return true, nil
}

func (s *fakeRenderCacheStore) Delete(_ context.Context, keys ...string) error {
	if s.err != nil {
		return s.err
	}
	for _, key := range keys {
		delete(s.values, key)
		delete(s.hashes, key)
		delete(s.ttl, key)
		s.deleted = append(s.deleted, key)
	}
	return nil
}

func TestDefaultRenderCacheOptionsMatchLegacyProject(t *testing.T) {
	options := DefaultRenderCacheOptions()

	if options.TopoXMLTTL != time.Hour {
		t.Fatalf("topo xml ttl = %v, want %v", options.TopoXMLTTL, time.Hour)
	}
	if options.BizGlobalVariablesTTL != 5*time.Minute {
		t.Fatalf("biz global variables ttl = %v, want %v", options.BizGlobalVariablesTTL, 5*time.Minute)
	}
	if options.BuildWaitTTL != 30*time.Second {
		t.Fatalf("build wait ttl = %v, want %v", options.BuildWaitTTL, 30*time.Second)
	}
	if options.BuildTimeout != 5*time.Minute {
		t.Fatalf("build timeout = %v, want %v", options.BuildTimeout, 5*time.Minute)
	}
	if options.BuildLockTTL != 30*time.Second {
		t.Fatalf("build lock ttl = %v, want %v", options.BuildLockTTL, 30*time.Second)
	}
}

func TestRenderCacheOptionsKeepsBuildTimeoutSeparateFromWaitTTL(t *testing.T) {
	options := RenderCacheOptions{
		BuildWaitTTL: 30 * time.Second,
		BuildTimeout: 2 * time.Minute,
		BuildLockTTL: 30 * time.Second,
	}.withDefaults()

	if options.BuildWaitTTL != 30*time.Second {
		t.Fatalf("build wait ttl = %v, want %v", options.BuildWaitTTL, 30*time.Second)
	}
	if options.BuildTimeout != 2*time.Minute {
		t.Fatalf("build timeout = %v, want %v", options.BuildTimeout, 2*time.Minute)
	}
	if options.BuildLockTTL != 30*time.Second {
		t.Fatalf("build lock ttl = %v, want %v", options.BuildLockTTL, 30*time.Second)
	}
}

func TestRedisCMDBRenderCacheKeysIncludeTenantAndSetEnv(t *testing.T) {
	store := newFakeRenderCacheStore()
	cache := newRedisCMDBRenderCacheWithStore(store, RenderCacheOptions{
		TopoXMLTTL:            time.Minute,
		BizGlobalVariablesTTL: 2 * time.Minute,
		BuildLockTTL:          30 * time.Second,
	})

	cache.SetTopoXML(context.Background(), "tenant-a", 42, "3", "formal")
	cache.SetTopoXML(context.Background(), "tenant-a", 42, "1", "test")
	cache.SetTopoXML(context.Background(), "tenant-b", 42, "3", "tenant-b-formal")

	formal, ok := cache.GetTopoXML(context.Background(), "tenant-a", 42, "3")
	if !ok || formal != "formal" {
		t.Fatalf("formal topo cache = %q, %v, want formal true", formal, ok)
	}
	test, ok := cache.GetTopoXML(context.Background(), "tenant-a", 42, "1")
	if !ok || test != "test" {
		t.Fatalf("test topo cache = %q, %v, want test true", test, ok)
	}
	tenantBFormal, ok := cache.GetTopoXML(context.Background(), "tenant-b", 42, "3")
	if !ok || tenantBFormal != "tenant-b-formal" {
		t.Fatalf("tenant-b topo cache = %q, %v, want tenant-b-formal true", tenantBFormal, ok)
	}
	if got := store.ttl[cache.topoXMLKey("tenant-a", 42)]; got != time.Minute {
		t.Fatalf("topo xml ttl = %v, want %v", got, time.Minute)
	}
}

func TestRedisCMDBRenderCacheIgnoresInvalidJSON(t *testing.T) {
	store := newFakeRenderCacheStore()
	cache := newRedisCMDBRenderCacheWithStore(store, DefaultRenderCacheOptions())
	store.values[cache.bizObjectAttributesKey("tenant-a", 42)] = "{invalid-json"

	if _, ok := cache.GetBizObjectAttributes(context.Background(), "tenant-a", 42); ok {
		t.Fatal("invalid json should be treated as cache miss")
	}
}

func TestRedisCMDBRenderCacheFailsOpenOnStoreError(t *testing.T) {
	store := newFakeRenderCacheStore()
	store.err = errors.New("redis unavailable")
	cache := newRedisCMDBRenderCacheWithStore(store, DefaultRenderCacheOptions())

	if _, ok := cache.GetTopoXML(context.Background(), "tenant-a", 42, "3"); ok {
		t.Fatal("redis get error should be treated as topo cache miss")
	}
	cache.SetTopoXML(context.Background(), "tenant-a", 42, "3", "formal")

	attrs := map[string][]ObjectAttribute{
		BK_SET_OBJ_ID: {{BkPropertyID: "set_cached"}},
	}
	cache.SetBizObjectAttributes(context.Background(), "tenant-a", 42, attrs)
	if _, ok := cache.GetBizObjectAttributes(context.Background(), "tenant-a", 42); ok {
		t.Fatal("redis get error should be treated as attributes cache miss")
	}
}

func TestRedisCMDBRenderCacheInvalidatesSingleBiz(t *testing.T) {
	store := newFakeRenderCacheStore()
	cache := newRedisCMDBRenderCacheWithStore(store, RenderCacheOptions{
		TopoXMLTTL:            time.Minute,
		BizGlobalVariablesTTL: 2 * time.Minute,
		BuildLockTTL:          30 * time.Second,
	})
	ctx := context.Background()

	cache.SetTopoXML(ctx, "tenant-a", 42, "3", "tenant-a-topo")
	cache.SetTopoXML(ctx, "tenant-b", 42, "3", "tenant-b-topo")
	cache.SetBizObjectAttributes(ctx, "tenant-a", 42, map[string][]ObjectAttribute{
		BK_SET_OBJ_ID: {{BkPropertyID: "set_cached"}},
	})

	if got := store.ttl[cache.bizObjectAttributesKey("tenant-a", 42)]; got != 2*time.Minute {
		t.Fatalf("biz global variables ttl = %v, want %v", got, 2*time.Minute)
	}

	if err := cache.InvalidateBiz(ctx, "tenant-a", 42); err != nil {
		t.Fatalf("InvalidateBiz failed: %v", err)
	}

	if _, ok := cache.GetTopoXML(ctx, "tenant-a", 42, "3"); ok {
		t.Fatal("tenant-a topo cache should be invalidated")
	}
	if _, ok := cache.GetBizObjectAttributes(ctx, "tenant-a", 42); ok {
		t.Fatal("tenant-a biz global variables cache should be invalidated")
	}
	if got, ok := cache.GetTopoXML(ctx, "tenant-b", 42, "3"); !ok || got != "tenant-b-topo" {
		t.Fatalf("tenant-b topo cache = %q, %v, want tenant-b-topo true", got, ok)
	}
}

func TestRedisCMDBRenderCacheUsesDurationForBuildLockTTL(t *testing.T) {
	store := newFakeRenderCacheStore()
	cache := newRedisCMDBRenderCacheWithStore(store, RenderCacheOptions{
		BuildLockTTL: 1500 * time.Millisecond,
		BuildTimeout: 1500 * time.Millisecond,
	})
	ctx := context.Background()

	locked, err := cache.AcquireBuildLock(ctx, "tenant-a", 42, renderCacheKindBizGlobalVariables, "")
	if err != nil {
		t.Fatalf("AcquireBuildLock failed: %v", err)
	}
	if !locked {
		t.Fatal("AcquireBuildLock should acquire lock")
	}

	lockKey := cache.buildLockKey("tenant-a", 42, renderCacheKindBizGlobalVariables, "")
	if got := store.ttl[lockKey]; got != 1500*time.Millisecond {
		t.Fatalf("build lock ttl = %v, want %v", got, 1500*time.Millisecond)
	}
	if got := store.ttl[cache.buildLockIndexKey("tenant-a", 42)]; got != 1500*time.Millisecond {
		t.Fatalf("build lock index ttl = %v, want %v", got, 1500*time.Millisecond)
	}
}

func TestRedisCMDBRenderCacheRenewsOwnedBuildLock(t *testing.T) {
	store := newFakeRenderCacheStore()
	cache := newRedisCMDBRenderCacheWithStore(store, RenderCacheOptions{
		BuildLockTTL: 1500 * time.Millisecond,
	})
	ctx := context.Background()

	locked, err := cache.AcquireBuildLock(ctx, "tenant-a", 42, renderCacheKindBizGlobalVariables, "")
	if err != nil {
		t.Fatalf("AcquireBuildLock failed: %v", err)
	}
	if !locked {
		t.Fatal("AcquireBuildLock should acquire lock")
	}
	renewed, err := cache.RenewBuildLock(ctx, "tenant-a", 42, renderCacheKindBizGlobalVariables, "")
	if err != nil {
		t.Fatalf("RenewBuildLock failed: %v", err)
	}
	if !renewed {
		t.Fatal("RenewBuildLock should renew owned lock")
	}
	if got := len(store.renewed); got != 1 {
		t.Fatalf("renew calls = %d, want 1", got)
	}
}

func TestRedisCMDBRenderCacheReleaseDoesNotDeleteOtherOwnerLock(t *testing.T) {
	store := newFakeRenderCacheStore()
	cache := newRedisCMDBRenderCacheWithStore(store, DefaultRenderCacheOptions())
	ctx := context.Background()

	locked, err := cache.AcquireBuildLock(ctx, "tenant-a", 42, renderCacheKindBizGlobalVariables, "")
	if err != nil {
		t.Fatalf("AcquireBuildLock failed: %v", err)
	}
	if !locked {
		t.Fatal("AcquireBuildLock should acquire lock")
	}

	lockKey := cache.buildLockKey("tenant-a", 42, renderCacheKindBizGlobalVariables, "")
	store.values[lockKey] = "another-owner"
	if err = cache.ReleaseBuildLock(ctx, "tenant-a", 42, renderCacheKindBizGlobalVariables, ""); err != nil {
		t.Fatalf("ReleaseBuildLock failed: %v", err)
	}
	if got := store.values[lockKey]; got != "another-owner" {
		t.Fatalf("lock owner = %q, want another-owner", got)
	}
}

func TestRedisCMDBRenderCacheReleasesBuildLock(t *testing.T) {
	store := newFakeRenderCacheStore()
	cache := newRedisCMDBRenderCacheWithStore(store, DefaultRenderCacheOptions())
	ctx := context.Background()

	locked, err := cache.AcquireBuildLock(ctx, "tenant-a", 42, renderCacheKindBizGlobalVariables, "")
	if err != nil {
		t.Fatalf("AcquireBuildLock failed: %v", err)
	}
	if !locked {
		t.Fatal("first AcquireBuildLock should acquire lock")
	}
	if locked, err = cache.AcquireBuildLock(ctx, "tenant-a", 42, renderCacheKindBizGlobalVariables, ""); err != nil {
		t.Fatalf("second AcquireBuildLock failed: %v", err)
	} else if locked {
		t.Fatal("second AcquireBuildLock should be blocked by existing lock")
	}

	if err = cache.ReleaseBuildLock(ctx, "tenant-a", 42, renderCacheKindBizGlobalVariables, ""); err != nil {
		t.Fatalf("ReleaseBuildLock failed: %v", err)
	}
	if locked, err = cache.AcquireBuildLock(ctx, "tenant-a", 42, renderCacheKindBizGlobalVariables, ""); err != nil {
		t.Fatalf("third AcquireBuildLock failed: %v", err)
	} else if !locked {
		t.Fatal("AcquireBuildLock should acquire lock after release")
	}
}

func TestRedisCMDBRenderCacheInvalidatesBuildLocks(t *testing.T) {
	store := newFakeRenderCacheStore()
	cache := newRedisCMDBRenderCacheWithStore(store, DefaultRenderCacheOptions())
	ctx := context.Background()

	for _, tc := range []struct {
		kind     string
		identity string
	}{
		{kind: renderCacheKindTopoXML, identity: "3"},
		{kind: renderCacheKindBizGlobalVariables},
	} {
		locked, err := cache.AcquireBuildLock(ctx, "tenant-a", 42, tc.kind, tc.identity)
		if err != nil {
			t.Fatalf("AcquireBuildLock failed, kind: %s, identity: %s, err: %v", tc.kind, tc.identity, err)
		}
		if !locked {
			t.Fatalf("AcquireBuildLock should acquire lock, kind: %s, identity: %s", tc.kind, tc.identity)
		}
	}

	if err := cache.InvalidateBiz(ctx, "tenant-a", 42); err != nil {
		t.Fatalf("InvalidateBiz failed: %v", err)
	}

	for _, tc := range []struct {
		kind     string
		identity string
	}{
		{kind: renderCacheKindTopoXML, identity: "3"},
		{kind: renderCacheKindBizGlobalVariables},
	} {
		locked, err := cache.AcquireBuildLock(ctx, "tenant-a", 42, tc.kind, tc.identity)
		if err != nil {
			t.Fatalf("AcquireBuildLock after invalidation failed, kind: %s, identity: %s, err: %v",
				tc.kind, tc.identity, err)
		}
		if !locked {
			t.Fatalf("AcquireBuildLock should acquire after invalidation, kind: %s, identity: %s",
				tc.kind, tc.identity)
		}
	}
}
