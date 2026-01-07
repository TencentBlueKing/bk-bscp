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
	"encoding/xml"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// ApplicationXML 应用根节点（对应 Python 中的 Application）
// 参考 Python 代码中的 XML 结构：doc.createElement("Application")
type ApplicationXML struct {
	XMLName xml.Name `xml:"Application"`
	Sets    []SetXML `xml:"Set"`
}

// SetXML 集群节点（包含所有属性）
// 参考 Python 代码，确保属性名与 Python 输出一致
// 注意：所有属性都通过 Attrs 动态添加，与 Python 代码统一处理方式一致
type SetXML struct {
	XMLName xml.Name `xml:"Set"`
	// 所有属性通过 Attrs 动态添加（包括 SetName, SetID 等，通过 buildAttrsFromStruct 统一处理）
	Attrs   []xml.Attr  `xml:",any,attr"` // 动态属性
	Modules []ModuleXML `xml:"Module"`
}

// ModuleXML 模块节点（包含所有属性）
// 注意：所有属性都通过 Attrs 动态添加，与 Python 代码统一处理方式一致
type ModuleXML struct {
	XMLName xml.Name `xml:"Module"`
	// 所有属性通过 Attrs 动态添加（包括 ModuleName, ModuleID 等，通过 buildAttrsFromStruct 统一处理）
	Attrs []xml.Attr `xml:",any,attr"` // 动态属性
	Hosts []HostXML  `xml:"Host"`
}

// HostXML 主机节点（包含所有属性）
// 注意：所有属性都通过 Attrs 动态添加，与 Python 代码统一处理方式一致
type HostXML struct {
	XMLName xml.Name `xml:"Host"`
	// 所有属性通过 Attrs 动态添加（包括 InnerIP, CloudID, HostID, HostName 等，通过 buildAttrsFromStruct 统一处理）
	Attrs []xml.Attr `xml:",any,attr"` // 动态属性
}

// convertSetInfoToXML 将 SetInfo 转换为 SetXML
// topoFields: 从 biz_global_variables 获取的字段列表，用于补充缺失字段
// 与 Python 代码统一处理方式一致：所有属性都通过 buildAttrsFromStruct 统一处理
func convertSetInfoToXML(setInfo interface{}, topoFields []string) SetXML {
	setXML := SetXML{
		Modules: []ModuleXML{},
	}

	// 统一通过 buildAttrsFromStruct 处理所有属性（包括 SetName, SetID 等）
	// 与 Python 代码的 set_attr_to_xml_element 逻辑一致
	attrs := buildAttrsFromStruct(setInfo, map[string]bool{
		// 不需要排除任何字段，所有字段都通过 buildAttrsFromStruct 统一处理
	})

	// Python 代码逻辑：为 topo_variables 中但 CMDB 数据中没有的字段设置空字符串
	// 参考：set_attr_to_xml_element 中的逻辑
	attrs = fillMissingFields(attrs, topoFields)

	setXML.Attrs = attrs

	return setXML
}

// convertModuleInfoToXML 将 ModuleInfo 转换为 ModuleXML
// topoFields: 从 biz_global_variables 获取的字段列表，用于补充缺失字段
// 与 Python 代码统一处理方式一致：所有属性都通过 buildAttrsFromStruct 统一处理
func convertModuleInfoToXML(moduleInfo interface{}, topoFields []string) ModuleXML {
	moduleXML := ModuleXML{
		Hosts: []HostXML{},
	}

	// 统一通过 buildAttrsFromStruct 处理所有属性（包括 ModuleName, ModuleID 等）
	// 与 Python 代码的 set_attr_to_xml_element 逻辑一致
	attrs := buildAttrsFromStruct(moduleInfo, map[string]bool{
		// 不需要排除任何字段，所有字段都通过 buildAttrsFromStruct 统一处理
	})

	// Python 代码逻辑：为 topo_variables 中但 CMDB 数据中没有的字段设置空字符串
	attrs = fillMissingFields(attrs, topoFields)

	moduleXML.Attrs = attrs

	return moduleXML
}

// convertHostInfoToXML 将 HostInfo 转换为 HostXML
// topoFields: 从 biz_global_variables 获取的字段列表，用于补充缺失字段
// 与 Python 代码统一处理方式一致：所有属性都通过 buildAttrsFromStruct 统一处理
func convertHostInfoToXML(hostInfo interface{}, topoFields []string) HostXML {
	hostXML := HostXML{}

	// 统一通过 buildAttrsFromStruct 处理所有属性（包括 InnerIP, CloudID, HostID, HostName 等）
	// 与 Python 代码的 set_attr_to_xml_element 逻辑一致
	attrs := buildAttrsFromStruct(hostInfo, map[string]bool{
		// 不需要排除任何字段，所有字段都通过 buildAttrsFromStruct 统一处理
	})

	// Python 代码逻辑：为 topo_variables 中但 CMDB 数据中没有的字段设置空字符串
	attrs = fillMissingFields(attrs, topoFields)

	hostXML.Attrs = attrs

	return hostXML
}

// buildAttrsFromStruct 从结构体构建 XML 属性
// 使用反射获取所有字段，转换为 XML 属性
// exclude: 需要排除的字段名（已单独处理的字段）
// nolint
func buildAttrsFromStruct(v interface{}, exclude map[string]bool) []xml.Attr {
	if v == nil {
		return nil
	}

	var attrs []xml.Attr
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		// 跳过未导出的字段
		if !fieldVal.CanInterface() {
			continue
		}

		// 获取 JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// 提取字段名（去掉 omitempty 等选项）
		fieldName := strings.Split(jsonTag, ",")[0]
		if exclude[fieldName] {
			continue
		}

		// Python 代码逻辑：
		// 1. 跳过 list、tuple、dict 类型的值
		// 2. 对于其他类型的值（包括 None/空值），都转为字符串并设置
		// 注意：Python 不会跳过空值，而是会设置为空字符串或 "None"

		// 对于切片、数组、Map 等复杂类型，Python 会跳过
		if fieldVal.Kind() == reflect.Slice || fieldVal.Kind() == reflect.Array || fieldVal.Kind() == reflect.Map {
			// Python 代码中会跳过 list、tuple、dict
			if fieldVal.Kind() == reflect.Slice && fieldVal.Len() > 0 {
				// 对于非空切片，转换为逗号分隔的字符串
				var parts []string
				for j := 0; j < fieldVal.Len(); j++ {
					parts = append(parts, fmt.Sprintf("%v", fieldVal.Index(j).Interface()))
				}
				attrValue := strings.Join(parts, ",")
				// 设置新字段名（CC3.0）
				attrs = append(attrs, xml.Attr{
					Name:  xml.Name{Local: fieldName},
					Value: attrValue,
				})
				// 设置旧字段名（CC1.0），如果映射后的名称不同
				oldFieldName := mapCC3FieldToCC1(fieldName)
				if oldFieldName != fieldName {
					attrs = append(attrs, xml.Attr{
						Name:  xml.Name{Local: oldFieldName},
						Value: attrValue,
					})
				}
			}
			// 空切片、数组、Map 都跳过（与 Python 一致）
			continue
		}

		// Python 代码逻辑：
		// xml_element.setAttribute(attr_key, "%s" % attr_value)
		// - 如果 attr_value 是 None，会转为字符串 "None"
		// - 如果 attr_value 是空字符串 ""，会保持为空字符串 ""
		// - 如果 attr_value 是 0，会转为字符串 "0"
		// 所以 Python 不会跳过空值，而是会将所有值都转为字符串并设置

		// 转换为字符串（包括空值）
		var attrValue string
		switch fieldVal.Kind() {
		case reflect.String:
			attrValue = fieldVal.String() // 空字符串会保持为空字符串
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			attrValue = strconv.FormatInt(fieldVal.Int(), 10) // 0 会转为 "0"
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			attrValue = strconv.FormatUint(fieldVal.Uint(), 10) // 0 会转为 "0"
		case reflect.Bool:
			attrValue = strconv.FormatBool(fieldVal.Bool()) // false 会转为 "false"
		case reflect.Float32, reflect.Float64:
			attrValue = strconv.FormatFloat(fieldVal.Float(), 'f', -1, 64) // 0.0 会转为 "0"
		case reflect.Ptr, reflect.Interface:
			// 对于指针或接口类型，如果是 nil，Python 中 None 会转为 "None"
			if fieldVal.IsNil() {
				attrValue = "" // Go 中 nil 指针在 JSON 中通常为 null，这里转为空字符串以兼容
			} else {
				attrValue = fmt.Sprintf("%v", fieldVal.Elem().Interface())
			}
		default:
			// 其他类型直接转为字符串
			attrValue = fmt.Sprintf("%v", fieldVal.Interface())
		}

		// Python 代码会设置两套字段名：
		// 1. 新字段名（CC3.0）：直接使用原始字段名（如 bk_set_name）
		// 2. 旧字段名（CC1.0）：通过 map_cc3_field_to_cc1 映射（如 SetName）

		// 设置新字段名（CC3.0）
		// 注意：attrValue 可能包含 XML 特殊字符（<, >, &, ", '），但 xml.MarshalIndent 会自动转义这些字符
		// 因此即使 CMDB 数据包含这些字符，生成的 XML 也是安全且有效的
		attrs = append(attrs, xml.Attr{
			Name:  xml.Name{Local: fieldName},
			Value: attrValue,
		})

		// 设置旧字段名（CC1.0），如果映射后的名称不同
		oldFieldName := mapCC3FieldToCC1(fieldName)
		if oldFieldName != fieldName {
			attrs = append(attrs, xml.Attr{
				Name:  xml.Name{Local: oldFieldName},
				Value: attrValue,
			})
		}
	}

	return attrs
}

// fillMissingFields 为 topo_variables 中但 CMDB 数据中没有的字段设置空字符串
// 参考 Python 代码中的 set_attr_to_xml_element 方法
// Python 逻辑：
//
//	for var in topo_variables:
//	    attr_key = self.map_cc3_field_to_cc1(var["bk_property_id"])
//	    if attr_key not in xml_element_keys:
//	        xml_element.setAttribute(attr_key, "")
func fillMissingFields(attrs []xml.Attr, topoFields []string) []xml.Attr {
	if len(topoFields) == 0 {
		return attrs
	}

	// 构建已设置属性的映射（包括 CC3.0 和 CC1.0 字段名）
	attrMap := make(map[string]bool)
	for _, attr := range attrs {
		attrMap[attr.Name.Local] = true
		// 如果字段有 CC1.0 映射，也记录
		cc1Name := mapCC3FieldToCC1(attr.Name.Local)
		if cc1Name != attr.Name.Local {
			attrMap[cc1Name] = true
		}
	}

	// 检查 topoFields 中的每个字段，如果不在已设置的属性中，设置为空字符串
	for _, fieldName := range topoFields {
		// 检查 CC1.0 字段名（Python 代码中使用的是 CC1.0 字段名）
		cc1Name := mapCC3FieldToCC1(fieldName)
		if !attrMap[cc1Name] {
			// 设置 CC1.0 字段名为空字符串
			attrs = append(attrs, xml.Attr{
				Name:  xml.Name{Local: cc1Name},
				Value: "",
			})
			attrMap[cc1Name] = true
		}
		// 如果 CC3.0 字段名与 CC1.0 不同，也检查 CC3.0 字段名
		if cc1Name != fieldName && !attrMap[fieldName] {
			attrs = append(attrs, xml.Attr{
				Name:  xml.Name{Local: fieldName},
				Value: "",
			})
			attrMap[fieldName] = true
		}
	}

	return attrs
}

// mapCC3FieldToCC1 映射 CC3.0 的字段到 CC1.0 的字段，兼容老的字段
// 参考 Python 代码中的 map_cc3_field_to_cc1 方法
func mapCC3FieldToCC1(newFieldName string) string {
	fieldNameMapping := map[string]string{
		// set
		"bk_set_name":       "SetName",
		"bk_set_env":        "SetEnviType", // 注意：Python 中是 SetEnviType，不是 SetEnv（维持了旧版本）
		"bk_world_id":       "SetWorldID",
		"bk_platform":       "Platform",
		"bk_system":         "System",
		"bk_chn_name":       "SetChnName",
		"bk_service_status": "SetServiceState",
		"bk_set_id":         "SetID",
		"bk_category":       "SetCategory",
		// module
		"bk_module_name": "ModuleName",
		"bk_module_id":   "ModuleID",
		// host
		"bk_host_innerip": "InnerIP",
		"bk_host_name":    "HostName",
		"bk_host_id":      "HostID",
		"bk_cloud_id":     "CloudID", // 注意：Python 代码中没有这个映射，但为了保持一致性，这里添加
	}
	oldFieldName := fieldNameMapping[newFieldName]
	if oldFieldName == "" {
		return newFieldName
	}
	return oldFieldName
}
