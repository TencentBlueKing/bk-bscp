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

// Package xxx
package pbdsm

import (
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	pbbase "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
	"gorm.io/datatypes"
)

// DataSourceMappingSpec convert pb DataSourceMappingSpec to table DataSourceMappingSpec
func (m *DataSourceMappingSpec) DataSourceMappingSpec() *table.DataSourceMappingSpec {
	if m == nil {
		return nil
	}

	return &table.DataSourceMappingSpec{
		DatabasesName: m.DatabasesName,
		TableName_:    m.TableName,
		TableMemo:     m.TableMemo,
		VisibleRange:  datatypes.NewJSONSlice(m.VisibleRange),
		Columns_:      datatypes.NewJSONSlice(DataSourceMappingColumns(m.Columns)),
	}
}

// DataSourceMappingAttachment convert pb DataSourceMappingAttachment to table DataSourceMappingAttachment
func (m *DataSourceMappingAttachment) DataSourceMappingAttachment() *table.DataSourceMappingAttachment {
	if m == nil {
		return nil
	}

	return &table.DataSourceMappingAttachment{
		BizID:            m.BizId,
		DataSourceInfoID: m.DataSourceInfoId,
	}
}

// PbDataSourceMappingSpec convert table DataSourceMappingSpec to pb DataSourceMappingSpec
func PbDataSourceMappingSpec(spec *table.DataSourceMappingSpec) *DataSourceMappingSpec {
	if spec == nil {
		return nil
	}

	// spec.Columns_
	return &DataSourceMappingSpec{
		DatabasesName: spec.DatabasesName,
		TableName:     spec.TableName_,
		TableMemo:     spec.TableMemo,
		VisibleRange:  spec.VisibleRange,
		Columns:       PbDataSourceMappingColumns(spec.Columns_),
	}
}

// PbDataSourceMappingAttachment convert table DataSourceMappingAttachment to pb DataSourceMappingAttachment
func PbDataSourceMappingAttachment(attachment *table.DataSourceMappingAttachment) *DataSourceMappingAttachment {
	if attachment == nil {
		return nil
	}

	return &DataSourceMappingAttachment{
		BizId:            attachment.BizID,
		DataSourceInfoId: attachment.DataSourceInfoID,
	}
}

// PbDataSourceMapping convert table DataSourceMapping to pb DataSourceMapping
func PbDataSourceMapping(c *table.DataSourceMapping) *DataSourceMapping {
	if c == nil {
		return nil
	}

	return &DataSourceMapping{
		Id:         c.ID,
		Spec:       PbDataSourceMappingSpec(c.Spec),
		Attachment: PbDataSourceMappingAttachment(c.Attachment),
		Revision:   pbbase.PbRevision(c.Revision),
	}
}

// PbDataSourceMappings convert table DataSourceMapping to pb DataSourceMapping
func PbDataSourceMappings(c []*table.DataSourceMapping) []*DataSourceMapping {
	if c == nil {
		return make([]*DataSourceMapping, 0)
	}
	result := make([]*DataSourceMapping, 0)
	for _, v := range c {
		result = append(result, PbDataSourceMapping(v))
	}
	return result
}

func (m *Columns) DataSourceMappingColumns() *table.Columns_ {
	if m == nil {
		return nil
	}

	return &table.Columns_{
		Name:          m.Name,
		Alias:         m.Alias,
		Length:        uint(m.Length),
		Primary:       m.Primary,
		ColumnType:    table.ColumnType(m.ColumnType),
		NotNull:       m.NotNull,
		DefaultValue:  m.DefaultValue,
		Unique:        m.Unique,
		ReadOnly:      m.ReadOnly,
		AutoIncrement: m.AutoIncrement,
		EnumValue:     m.EnumValue,
		Selected:      m.Selected,
	}
}

func PbDataSourceMappingColumn(c *table.Columns_) *Columns {
	if c == nil {
		return nil
	}

	return &Columns{
		Name:          c.Name,
		Alias:         c.Alias,
		Length:        int32(c.Length),
		Primary:       c.Primary,
		ColumnType:    string(c.ColumnType),
		NotNull:       c.NotNull,
		DefaultValue:  c.DefaultValue,
		Unique:        c.Unique,
		ReadOnly:      c.ReadOnly,
		AutoIncrement: c.AutoIncrement,
		EnumValue:     c.EnumValue,
		Selected:      c.Selected,
	}
}

func PbDataSourceMappingColumns(c []*table.Columns_) []*Columns {
	if c == nil {
		return make([]*Columns, 0)
	}
	result := make([]*Columns, 0)
	for _, v := range c {
		result = append(result, PbDataSourceMappingColumn(v))
	}
	return result
}

func (c *Columns) DataSourceMappingColumn() *table.Columns_ {
	if c == nil {
		return nil
	}

	return &table.Columns_{
		Name:          c.Name,
		Alias:         c.Alias,
		Length:        uint(c.Length),
		Primary:       c.Primary,
		ColumnType:    table.ColumnType(c.ColumnType),
		NotNull:       c.NotNull,
		DefaultValue:  c.DefaultValue,
		Unique:        c.Unique,
		ReadOnly:      c.ReadOnly,
		AutoIncrement: c.AutoIncrement,
		EnumValue:     c.EnumValue,
		Selected:      c.Selected,
	}
}

func DataSourceMappingColumns(c []*Columns) []*table.Columns_ {
	if c == nil {
		return make([]*table.Columns_, 0)
	}
	result := make([]*table.Columns_, 0)
	for _, v := range c {
		result = append(result, v.DataSourceMappingColumn())
	}

	return result
}
