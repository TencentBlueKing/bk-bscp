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
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// Rule interface for validating a single field
type Rule interface {
	Validate(kit *kit.Kit, value interface{}, col *table.Columns_,
		row map[string]interface{}) (interface{}, error)
}

// ColumnTypeRule validates the type of a field
type ColumnTypeRule struct{}

func (r ColumnTypeRule) Validate(kit *kit.Kit, value interface{}, col *table.Columns_,
	row map[string]interface{}) (interface{}, error) {
	if value == nil {
		return value, nil
	}

	switch col.ColumnType {
	case table.NumberColumn:
		if _, err := strconv.Atoi(fmt.Sprintf("%v", value)); err != nil {
			return nil, errors.New(i18n.T(kit, "field %s must be a number", col.Name))
		}
	case table.StringColumn:
		if _, ok := value.(string); !ok {
			return nil, errors.New(i18n.T(kit, "field %s must be a string", col.Name))
		}
	case table.EnumColumn:
		switch {
		// 尝试解析为字符串数组
		case isStringArray(col.EnumValue):
			var stringArray []string
			_ = json.Unmarshal([]byte(col.EnumValue), &stringArray)
			if err := handleEnumValue(kit, value, col.Name, stringArray, col.Selected); err != nil {
				return nil, err
			}
		// 尝试解析为对象数组
		case isObjectArray(col.EnumValue):
			var objectArray []map[string]string
			_ = json.Unmarshal([]byte(col.EnumValue), &objectArray)
			enumValues := []string{}
			for _, arr := range objectArray {
				for _, v := range arr {
					enumValues = append(enumValues, v)
				}
			}
			if err := handleEnumValue(kit, value, col.Name, enumValues, col.Selected); err != nil {
				return nil, err
			}
		default:
			return nil, errors.New(i18n.T(kit, "please check the enumeration field %s", col.Name))
		}
	}

	return value, nil
}

// NotNullRule validates that a field is not null
type NotNullRule struct{}

func (r NotNullRule) Validate(kit *kit.Kit, value interface{}, col *table.Columns_,
	row map[string]interface{}) (interface{}, error) {
	if col.NotNull && (value == nil || value == "") {
		return nil, errors.New(i18n.T(kit, "field %s cannot be null", col.Name))
	}

	return value, nil
}

// UniqueRule validates that a field is unique
type UniqueRule struct {
	seenValues map[string]map[interface{}]bool
}

func NewUniqueRule() *UniqueRule {
	return &UniqueRule{
		seenValues: make(map[string]map[interface{}]bool),
	}
}

// Validate checks if the value is unique for a specific column
func (r *UniqueRule) Validate(kit *kit.Kit, value interface{}, col *table.Columns_,
	row map[string]interface{}) (interface{}, error) {
	if value == nil {
		return value, nil
	}

	if _, exists := r.seenValues[col.Name]; !exists {
		r.seenValues[col.Name] = make(map[interface{}]bool)
	}

	// 将值统一为字符串，确保不同类型的值可以正确比较
	valueStr := fmt.Sprintf("%v", value)

	// 检查是否已经遇到该值
	if r.seenValues[col.Name][valueStr] {
		return nil, errors.New(i18n.T(kit, "field %s must be unique, but value %v is duplicated", col.Name, value))
	}

	// 将此值标记为已看到
	r.seenValues[col.Name][valueStr] = true

	return value, nil
}

// LengthRule validates the length of a string field
type LengthRule struct{}

func (r LengthRule) Validate(kit *kit.Kit, value interface{}, col *table.Columns_,
	row map[string]interface{}) (interface{}, error) {
	if col.Length > 0 {
		if strVal, ok := value.(string); ok {
			if len(strVal) > int(col.Length) {
				return nil, errors.New(i18n.T(kit, "field %s must not exceed %d characters", col.Name, col.Length))
			}
		}
	}
	return value, nil
}

// DefaultValueRule handles the default_value logic
type DefaultValueRule struct{}

// Validate handles the default_value logic for a specific field
func (r DefaultValueRule) Validate(kit *kit.Kit, value interface{}, col *table.Columns_,
	row map[string]interface{}) (interface{}, error) {
	// 设置默认值
	if value == nil || value == "" {
		if col.DefaultValue != "" {
			if isStringArray(col.DefaultValue) {
				var stringArray []string
				_ = json.Unmarshal([]byte(col.EnumValue), &stringArray)
				return stringArray, nil
			}

			return col.DefaultValue, nil
		}
	}
	return value, nil
}

type RuleManager struct {
	rules []Rule
}

// AddRule Add Validation Rules
func (rm *RuleManager) AddRule(rule Rule) {
	rm.rules = append(rm.rules, rule)
}

// Validate validate rules
func (rm *RuleManager) Validate(kit *kit.Kit, value interface{}, col *table.Columns_,
	row map[string]interface{}) (interface{}, error) {
	for _, rule := range rm.rules {
		updatedValue, err := rule.Validate(kit, value, col, row)
		if err != nil {
			return nil, err
		}
		value = updatedValue
	}

	return value, nil
}

// BuildRuleManager Building a rules manager
func BuildRuleManager(col *table.Columns_) *RuleManager {
	rm := &RuleManager{}

	rm.AddRule(ColumnTypeRule{})

	// 自增和有默认值的都不需要校验是否为空
	if !col.AutoIncrement && col.DefaultValue == "" && col.NotNull {
		rm.AddRule(NotNullRule{})
	}

	if col.Unique || col.Primary {
		rm.AddRule(NewUniqueRule())
	}

	if col.Length > 0 {
		rm.AddRule(LengthRule{})
	}

	if col.DefaultValue != "" {
		rm.AddRule(DefaultValueRule{})
	}

	return rm
}

// 处理枚举值
func handleEnumValue(kit *kit.Kit, value interface{}, name string, enumValues []string, selected bool) error {
	if selected {
		vals, ok := value.([]interface{})
		if !ok {
			return errors.New(i18n.T(kit, "field %s must be a list of valid values", name))
		}

		for _, val := range vals {
			valStr := fmt.Sprintf("%v", val)
			valid := false
			for _, enum := range enumValues {
				if valStr == enum {
					valid = true
					break
				}
			}
			if !valid {
				return errors.New(i18n.T(kit, "field %s must be one of %v, but value %v is invalid",
					name, enumValues, valStr))
			}
		}
	} else {
		// 处理只允许单个值的情况
		valStr := fmt.Sprintf("%v", value)
		valid := false
		for _, enum := range enumValues {
			if valStr == enum {
				valid = true
				break
			}
		}
		if !valid {
			return errors.New(i18n.T(kit, "field %s must be one of %v, but value %v is invalid",
				name, enumValues, valStr))
		}
	}

	return nil
}

// 判断是否为字符串数组
func isStringArray(input string) bool {
	var temp []string
	return json.Unmarshal([]byte(input), &temp) == nil
}

// 判断是否为对象数组
func isObjectArray(input string) bool {
	var temp []map[string]string
	return json.Unmarshal([]byte(input), &temp) == nil
}

func isNumber(value interface{}) bool {
	// 获取值的类型
	valType := reflect.TypeOf(value)

	// 检查类型是否为数字
	switch valType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	case reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}
