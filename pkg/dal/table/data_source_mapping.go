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

package table

import (
	"errors"
	"fmt"
	"regexp"

	"gorm.io/datatypes"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/enumor"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

const TableNameDataSourceMapping = "data_source_mappings"

// DataSourceMapping mapped from table <data_source_mappings>
type DataSourceMapping struct {
	ID         uint32                       `gorm:"column:id" json:"id"`
	Attachment *DataSourceMappingAttachment `json:"attachment" gorm:"embedded"`
	Spec       *DataSourceMappingSpec       `json:"spec" gorm:"embedded"`
	Revision   *Revision                    `json:"revision" gorm:"embedded"`
}

// DataSourceMappingAttachment xxx
type DataSourceMappingAttachment struct {
	BizID            uint32 `gorm:"column:biz_id;not null" json:"biz_id"`
	DataSourceInfoID uint32 `gorm:"column:data_source_info_id;not null" json:"data_source_info_id"`
}

// DataSourceMappingSpec xxx
type DataSourceMappingSpec struct {
	DatabasesName string                         `gorm:"column:databases_name;not null" json:"databases_name"`
	TableName_    string                         `gorm:"column:table_name;not null" json:"table_name"`
	TableMemo     string                         `gorm:"column:table_memo" json:"table_memo"`
	VisibleRange  datatypes.JSONSlice[uint32]    `gorm:"column:visible_range" json:"visible_range"`
	Columns_      datatypes.JSONSlice[*Columns_] `gorm:"column:columns" json:"columns"`
}

type Columns_ struct {
	// Name 字段名称
	Name string `json:"name"`
	// Alias 字段别名
	Alias string `json:"alias"`
	// Length 字段长度
	Length int `json:"length"`
	// Primary 是否为主键
	Primary bool `json:"primary"`
	// ColumnType 字段类型
	ColumnType ColumnType `json:"column_type"`
	// NotNull 非空
	NotNull bool `json:"not_null"`
	// DefaultValue 默认值
	DefaultValue string `json:"default_value"`
	// Unique 唯一
	Unique bool `json:"unique"`
	// ReadOnly 只读
	ReadOnly bool `json:"read_only"`
	// AutoIncrement 自增
	AutoIncrement bool `json:"auto_increment"`
	// EnumValue 枚举值
	EnumValue string `json:"enum_value"`
	// Selected 枚举值多选
	Selected bool `json:"selected"`
}

// ColumnType column type (number、string、enum、json).
type ColumnType string

const (
	// Number xxx
	NumberColumn ColumnType = "number"
	// String xxx
	StringColumn ColumnType = "string"
	// Enum xxx
	EnumColumn ColumnType = "enum"
	// Command xxx
	JsonColumn ColumnType = "json"
	// Unknown xxx
	UnknownColumn ColumnType = "unknown"
)

// Validate the column type is valid or not.
func (ct ColumnType) Validate() error {
	switch ct {
	case NumberColumn:
	case StringColumn:
	case EnumColumn:
	case JsonColumn:
	case UnknownColumn:
	default:
		return fmt.Errorf("unknown %s client type", ct)
	}

	return nil
}

// TableName DataSourceMapping's table name
func (*DataSourceMapping) TableName() string {
	return TableNameDataSourceMapping
}

// AppID AuditRes interface
func (d *DataSourceMapping) AppID() uint32 {
	return 0
}

// ResID AuditRes interface
func (d *DataSourceMapping) ResID() uint32 {
	return d.ID
}

// ResType AuditRes interface
func (d *DataSourceMapping) ResType() string {
	return string(enumor.Table)
}

// ValidateCreate 验证创建数据
func (d DataSourceMapping) ValidateCreate(kit *kit.Kit) error {
	if d.Attachment.BizID == 0 {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "invalid biz id"))
	}

	if d.Spec == nil {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "spec should be set"))
	}

	if err := d.Spec.ValidateCreate(kit); err != nil {
		return err
	}

	if d.Revision == nil {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "revision should be set"))
	}

	return nil
}

// ValidateUpdate 验证编辑数据
func (d DataSourceMapping) ValidateUpdate(kit *kit.Kit) error {
	if d.ID <= 0 {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "id should be set"))
	}
	if d.Attachment.BizID == 0 {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "invalid biz id"))
	}

	if d.Spec == nil {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "spec should be set"))
	}

	if err := d.Spec.ValidateCreate(kit); err != nil {
		return err
	}

	if d.Revision == nil {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "revision should be set"))
	}

	return nil
}

// ValidateDelete 验证删除数据
func (d DataSourceMapping) ValidateDelete(kit *kit.Kit) error {
	if d.ID <= 0 {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "id should be set"))
	}
	if d.Attachment.BizID == 0 {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "invalid biz id"))
	}

	return nil
}

// ValidateCreate xxx
func (d DataSourceMappingSpec) ValidateCreate(kit *kit.Kit) error {
	if len(d.TableName_) == 0 {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "table name cannot be empty"))
	}

	if len(d.Columns_) == 0 {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "please set the field"))
	}
	// 编译正则表达式
	re := regexp.MustCompile(pattern)
	nameSet := make(map[string]bool)
	for _, v := range d.Columns_ {
		if _, exists := nameSet[v.Name]; exists {
			return errors.New(i18n.T(kit, "duplicate name found: %s", v.Name))
		}
		nameSet[v.Name] = true
		if err := v.ValidateColumn(kit, re); err != nil {
			return err
		}
	}

	return nil
}

// ValidateColumn 验证字段
func (v Columns_) ValidateColumn(kit *kit.Kit, re *regexp.Regexp) error {
	// 验证类型
	if err := v.ColumnType.Validate(); err != nil {
		return err
	}
	// 别名不能超过20个字符
	if len(v.Alias) > 20 {
		return errf.Errorf(errf.InvalidArgument,
			i18n.T(kit, "the display name cannot exceed 20 characters"))
	}
	if v.Primary {
		if v.ColumnType == EnumColumn {
			return errf.Errorf(errf.InvalidArgument,
				i18n.T(kit, "the primary key must be a number or string type"))
		}
		if !v.NotNull {
			return errf.Errorf(errf.InvalidArgument,
				i18n.T(kit, "the primary key cannot be empty"))
		}
		if !v.Unique {
			return errf.Errorf(errf.InvalidArgument,
				i18n.T(kit, "the primary key must be unique"))
		}
	}
	// 自增的字段不能为空
	if v.AutoIncrement && !v.NotNull {
		return errf.Errorf(errf.InvalidArgument,
			i18n.T(kit, "the auto-increment field cannot be empty"))
	}
	// 自增的字段类型必须是number
	if v.AutoIncrement && v.ColumnType != NumberColumn {
		return errf.Errorf(errf.InvalidArgument,
			i18n.T(kit, "the auto-increment field type must be number"))
	}
	// 判断字段名是否符合
	if !re.MatchString(v.Name) {
		return errf.Errorf(errf.InvalidArgument,
			i18n.T(kit, "the name %s must be letters, numbers, underscores (_), "+
				", and dollar signs ($), and must be a maximum of 64 characters", v.Name))
	}
	// 枚举类型需要设置枚举值
	if v.ColumnType == EnumColumn && len(v.EnumValue) == 0 {
		return errf.Errorf(errf.InvalidArgument,
			i18n.T(kit, "please set the enumeration value, %s", v.Name))
	}

	return nil
}

const pattern = `^[a-zA-Z0-9_$]{1,64}$`
