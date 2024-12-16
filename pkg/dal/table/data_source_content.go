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

const TableNameDataSourceContent = "data_source_contents"

// DataSourceContent mapped from table <data_source_contents>
type DataSourceContent struct {
	ID         uint32                       `gorm:"column:id" json:"id"`
	Attachment *DataSourceContentAttachment `json:"attachment" gorm:"embedded"`
	Spec       *DataSourceContentSpec       `json:"spec" gorm:"embedded"`
	Revision   *Revision                    `json:"revision" gorm:"embedded"`
}

// DataSourceContentAttachment xxx
type DataSourceContentAttachment struct {
	ID                  uint32 `gorm:"column:id;primaryKey" json:"id"`
	DataSourceMappingID uint32 `gorm:"column:data_source_mapping_id;not null" json:"data_source_mapping_id"`
}

// DataSourceContentSpec xxx
type DataSourceContentSpec struct {
	Content string `gorm:"column:content" json:"content"`
	Status  string `gorm:"column:status;not null" json:"status"`
}

// TableName DataSourceContent's table name
func (*DataSourceContent) TableName() string {
	return TableNameDataSourceContent
}

// ResID AuditRes interface
func (d *DataSourceContent) ResID() uint32 {
	return d.ID
}

// ResType AuditRes interface
func (d *DataSourceContent) ResType() string {
	return "data_source_content"
}
