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

const TableNameDataSourceInfo = "data_source_infos"

// DataSourceInfo mapped from table <data_source_infos>
type DataSourceInfo struct {
	ID         uint32                    `gorm:"column:id" json:"id"`
	Attachment *DataSourceInfoAttachment `json:"attachment" gorm:"embedded"`
	Spec       *DataSourceInfoSpec       `json:"spec" gorm:"embedded"`
	Revision   *Revision                 `json:"revision" gorm:"embedded"`
}

// DataSourceInfoAttachment xxx
type DataSourceInfoAttachment struct {
	BizID uint32 `gorm:"column:biz_id;not null" json:"biz_id"`
}

// DataSourceInfoSpec xxx
type DataSourceInfoSpec struct {
	Name       string `gorm:"column:name;not null" json:"name"`
	Memo       string `gorm:"column:memo" json:"memo"`
	SourceType string `gorm:"column:source_type;not null" json:"source_type"`
	Dsn        string `gorm:"column:dsn;not null" json:"dsn"`
}

// TableName DataSourceInfo's table name
func (*DataSourceInfo) TableName() string {
	return TableNameDataSourceInfo
}

// AppID AuditRes interface
func (d *DataSourceInfo) AppID() uint32 {
	return 0
}

// ResID AuditRes interface
func (d *DataSourceInfo) ResID() uint32 {
	return d.ID
}

// ResType AuditRes interface
func (d *DataSourceInfo) ResType() string {
	return "data_source_info"
}
