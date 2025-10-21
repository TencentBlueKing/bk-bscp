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

package service

import (
	"testing"
)

func TestIsValidGrayPercentValue(t *testing.T) {
	s := &Service{}

	// 测试用例
	testCases := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		// 整数类型
		{"int64_valid_20", int64(20), true},
		{"int64_valid_1", int64(1), true},
		{"int64_valid_99", int64(99), true},
		{"int64_invalid_0", int64(0), false},
		{"int64_invalid_100", int64(100), false},
		{"int32_valid_30", int32(30), true},
		{"int_valid_40", int(40), true},

		// 浮点类型
		{"float64_valid_25.0", float64(25.0), true},
		{"float64_valid_50.5", float64(50.5), true},
		{"float32_valid_35", float32(35), true},

		// 无效类型
		{"bool_invalid", true, false},
		{"nil_invalid", nil, false},
		{"slice_invalid", []int{20}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := s.isValidGrayPercentValue(tc.value)
			if result != tc.expected {
				t.Errorf("isValidGrayPercentValue(%v) = %v, expected %v",
					tc.value, result, tc.expected)
			}
		})
	}
}
