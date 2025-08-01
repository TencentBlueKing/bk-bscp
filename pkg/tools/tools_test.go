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

package tools

import (
	"reflect"
	"strings"
	"testing"
)

func TestGetIntList(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
		err      bool
	}{
		{"1,2,3,4,5", []int{1, 2, 3, 4, 5}, false},
		{"", []int{}, false},
		{"1,invalid,3", []int{}, true},
	}

	for _, test := range tests {
		result, err := GetIntList(test.input)

		if test.err {
			if err == nil {
				t.Errorf("Expected error for input: %s", test.input)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for input: %s, err: %s", test.input, err)
			}

			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("For input %s, expected %v, but got %v", test.input, test.expected, result)
			}
		}
	}
}

func TestGetUint32List(t *testing.T) {
	tests := []struct {
		input    string
		expected []uint32
		err      bool
	}{
		{"1,2,3,4,5", []uint32{1, 2, 3, 4, 5}, false},
		{"", []uint32{}, false},
		{"1,invalid,3", []uint32{}, true},
	}

	for _, test := range tests {
		result, err := GetUint32List(test.input)

		if test.err {
			if err == nil {
				t.Errorf("Expected error for input: %s", test.input)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for input: %s, err: %s", test.input, err)
			}

			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("For input %s, expected %v, but got %v", test.input, test.expected, result)
			}
		}
	}
}

func TestSliceDiff(t *testing.T) {
	slice1 := []uint32{1, 2, 3, 4, 5}
	slice2 := []uint32{3, 4, 5, 6, 7}
	expected := []uint32{1, 2}

	result := SliceDiff(slice1, slice2)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected difference: %v, but got %v", expected, result)
	}
}

func TestSliceRepeatedElements(t *testing.T) {
	slice := []uint32{1, 2, 2, 3, 4, 4, 5, 5, 5}
	expected := []uint32{2, 4, 5}

	result := SliceRepeatedElements(slice)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected repeated elements: %v, but got %v", expected, result)
	}
}

func TestRemoveDuplicates(t *testing.T) {
	tests := []struct {
		input    []uint32
		expected []uint32
	}{
		{[]uint32{1, 2, 3, 2, 4, 1, 5, 6, 3}, []uint32{1, 2, 3, 4, 5, 6}},
		{[]uint32{2, 2, 2, 2, 2}, []uint32{2}},
		{[]uint32{}, []uint32{}},
		{[]uint32{1, 2, 3, 4}, []uint32{1, 2, 3, 4}},
	}

	for _, test := range tests {
		result := RemoveDuplicates(test.input)
		if !reflect.DeepEqual(result, test.expected) {
			t.Errorf("For input %v, expected %v, but got %v", test.input, test.expected, result)
		}
	}
}

func TestSplitPathAndName(t *testing.T) {
	tests := []struct {
		input        string
		expectedPath string
		expectedName string
	}{
		{"/folder/file.txt", "/folder/", "file.txt"},
		{"folder/file.txt", "/folder/", "file.txt"},
		{"/file.txt", "/", "file.txt"},
		{"/folder/", "/folder/", ""},
		{"folder/", "/folder/", ""},
		{"/", "/", ""},
	}

	for _, test := range tests {
		path, name := SplitPathAndName(test.input)
		if path != test.expectedPath || name != test.expectedName {
			t.Errorf("splitPathAndName(%s) = %s, %s; want %s, %s",
				test.input, path, name, test.expectedPath, test.expectedName)
		}
	}
}

func TestMatch(t *testing.T) {
	tests := []struct {
		input    string
		patterns []string
		expected bool
	}{
		{"file.txt", []string{"*.txt"}, true},
		{"file.txt", []string{"file.txt"}, true},
		{"file.txt", []string{"file.*"}, true},
		{"file.txt", []string{"file"}, false},
		{"file.txt", []string{"file.txt*"}, true},
		{"key1", []string{"{key1,key2}"}, true},
		{"key2", []string{"{key1,key2}"}, true},
		{"key3", []string{"key1", "key2", "key3"}, true},
	}
	for _, test := range tests {
		result := MatchPattern(test.input, test.patterns)
		if result != test.expected {
			t.Errorf("MatchPattern(%s, %v) = %t, want %t", test.input, test.patterns, result, test.expected)
		}
	}
}

func TestTruncateWithHumanize(t *testing.T) {
	// 模拟一个大字符串（70KB）
	s := strings.Repeat("这是一段测试内容。This is test content. ", 2000) // 大概 > 60KB

	result := TruncateWithHumanize(s)

	// 判断是否被截断（应包含 "...(Truncated, Total "）
	if !strings.Contains(result, "...(Truncated, Total ") {
		t.Errorf("expected truncated output, got:\n%s", result)
	}

	// 检查结果是否不超过 65535 字节（TEXT 最大限制）
	if len(result) > 65535 {
		t.Errorf("result exceeds TEXT field size: %d bytes", len(result))
	}
}
