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
	"testing"
	"time"
)

func TestGlobalObjectAttrCache_GetSet(t *testing.T) {
	cache := GetGlobalObjectAttrCache()
	cache.Clear() // Clear before test

	// Test 1: Set and Get
	testFields := []string{"field1", "field2", "field3"}
	cache.Set(123, BK_SET_OBJ_ID, testFields)

	value, exists := cache.Get(123, BK_SET_OBJ_ID)
	if !exists {
		t.Fatal("Cache should contain the value")
	}

	fields, ok := value.([]string)
	if !ok {
		t.Fatal("Cache value should be []string")
	}

	if len(fields) != len(testFields) {
		t.Fatalf("Expected %d fields, got %d", len(testFields), len(fields))
	}

	// Test 2: Non-existent key
	_, exists = cache.Get(999, BK_MODULE_OBJ_ID)
	if exists {
		t.Fatal("Cache should not contain non-existent key")
	}

	// Test 3: Different bizID should be independent
	fields2 := []string{"field4", "field5"}
	cache.Set(456, BK_SET_OBJ_ID, fields2)

	value1, _ := cache.Get(123, BK_SET_OBJ_ID)
	fields1 := value1.([]string)
	if len(fields1) != len(testFields) {
		t.Fatal("BizID 123 cache should not be affected by BizID 456")
	}

	// Test 4: Different objID should be independent
	hostFields := []string{"host1", "host2"}
	cache.Set(123, BK_HOST_OBJ_ID, hostFields)

	setVal, _ := cache.Get(123, BK_SET_OBJ_ID)
	setF := setVal.([]string)
	hostVal, _ := cache.Get(123, BK_HOST_OBJ_ID)
	hostF := hostVal.([]string)

	if len(setF) != len(testFields) || len(hostF) != len(hostFields) {
		t.Fatal("Different object types should have independent caches")
	}
}

func TestGlobalObjectAttrCache_Delete(t *testing.T) {
	cache := GetGlobalObjectAttrCache()
	cache.Clear()

	testFields := []string{"field1", "field2"}
	cache.Set(123, BK_SET_OBJ_ID, testFields)

	_, exists := cache.Get(123, BK_SET_OBJ_ID)
	if !exists {
		t.Fatal("Cache should exist before delete")
	}

	cache.Delete(123, BK_SET_OBJ_ID)

	_, exists = cache.Get(123, BK_SET_OBJ_ID)
	if exists {
		t.Fatal("Cache should be deleted")
	}
}

func TestGlobalObjectAttrCache_TTL(t *testing.T) {
	cache := GetGlobalObjectAttrCache()
	cache.Clear()
	cache.SetTTL(100 * time.Millisecond) // 100ms for testing

	testFields := []string{"field1"}
	cache.Set(123, BK_SET_OBJ_ID, testFields)

	// Should be available immediately
	_, exists := cache.Get(123, BK_SET_OBJ_ID)
	if !exists {
		t.Fatal("Cache should exist immediately after Set")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	_, exists = cache.Get(123, BK_SET_OBJ_ID)
	if exists {
		t.Fatal("Cache should have expired after TTL")
	}

	// Reset TTL back to 1 hour for other tests
	cache.SetTTL(time.Hour)
}

func TestGlobalObjectAttrCache_SetWithTTL(t *testing.T) {
	cache := GetGlobalObjectAttrCache()
	cache.Clear()

	testFields := []string{"field1"}

	// Set with custom TTL (100ms)
	cache.SetWithTTL(123, BK_SET_OBJ_ID, testFields, 100*time.Millisecond)

	// Should be available immediately
	_, exists := cache.Get(123, BK_SET_OBJ_ID)
	if !exists {
		t.Fatal("Cache should exist immediately after SetWithTTL")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	_, exists = cache.Get(123, BK_SET_OBJ_ID)
	if exists {
		t.Fatal("Cache should have expired after custom TTL")
	}
}

func TestGlobalObjectAttrCache_Clear(t *testing.T) {
	cache := GetGlobalObjectAttrCache()
	cache.SetTTL(time.Hour) // Reset TTL

	testFields := []string{"field1", "field2"}
	cache.Set(123, BK_SET_OBJ_ID, testFields)
	cache.Set(456, BK_MODULE_OBJ_ID, testFields)
	cache.Set(789, BK_HOST_OBJ_ID, testFields)

	// All should exist
	_, e1 := cache.Get(123, BK_SET_OBJ_ID)
	_, e2 := cache.Get(456, BK_MODULE_OBJ_ID)
	_, e3 := cache.Get(789, BK_HOST_OBJ_ID)

	if !e1 || !e2 || !e3 {
		t.Fatal("All cache entries should exist before Clear")
	}

	cache.Clear()

	// All should be gone
	_, e1 = cache.Get(123, BK_SET_OBJ_ID)
	_, e2 = cache.Get(456, BK_MODULE_OBJ_ID)
	_, e3 = cache.Get(789, BK_HOST_OBJ_ID)

	if e1 || e2 || e3 {
		t.Fatal("All cache entries should be cleared")
	}
}

func TestGlobalObjectAttrCache_Singleton(t *testing.T) {
	cache1 := GetGlobalObjectAttrCache()
	cache2 := GetGlobalObjectAttrCache()

	// Should be the same instance
	if cache1 != cache2 {
		t.Fatal("GetGlobalObjectAttrCache should return the same singleton instance")
	}

	// Verify they share the same data
	testFields := []string{"field1"}
	cache1.Set(999, BK_SET_OBJ_ID, testFields)

	_, exists := cache2.Get(999, BK_SET_OBJ_ID)
	if !exists {
		t.Fatal("Cache instances should share the same data")
	}
}

func TestGlobalObjectAttrCache_Stats(t *testing.T) {
	cache := GetGlobalObjectAttrCache()
	cache.Clear()
	cache.SetTTL(time.Hour)

	testFields := []string{"field1"}
	cache.Set(123, BK_SET_OBJ_ID, testFields)
	cache.Set(456, BK_MODULE_OBJ_ID, testFields)

	stats := cache.GetStats()
	if stats.TotalEntries != 2 {
		t.Fatalf("Expected 2 total entries, got %d", stats.TotalEntries)
	}

	if stats.ValidEntries != 2 {
		t.Fatalf("Expected 2 valid entries, got %d", stats.ValidEntries)
	}

	if stats.ExpiredEntries != 0 {
		t.Fatalf("Expected 0 expired entries, got %d", stats.ExpiredEntries)
	}
}
