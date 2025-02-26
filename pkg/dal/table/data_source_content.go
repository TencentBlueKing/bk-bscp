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
	"gorm.io/datatypes"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

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
	DataSourceMappingID uint32 `gorm:"column:data_source_mapping_id;not null" json:"data_source_mapping_id"`
}

// DataSourceContentSpec xxx
type DataSourceContentSpec struct {
	Content datatypes.JSONMap `gorm:"column:content" json:"content"`
	Status  string            `gorm:"column:status;not null" json:"status"`
}

// TableName DataSourceContent's table name
func (*DataSourceContent) TableName() string {
	return TableNameDataSourceContent
}

// AppID AuditRes interface
func (d *DataSourceContent) AppID() uint32 {
	return 0
}

// ResID AuditRes interface
func (d *DataSourceContent) ResID() uint32 {
	return d.ID
}

// ResType AuditRes interface
func (d *DataSourceContent) ResType() string {
	return "data_source_content"
}

// ValidateCreate 验证创建数据
func (d DataSourceContent) ValidateCreate(kit *kit.Kit) error {
	if d.Attachment == nil {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "attachment should be set"))
	}

	if d.Attachment.DataSourceMappingID <= 0 {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "invalid data source mapping id"))
	}

	if d.Spec == nil {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "spec should be set"))
	}

	if d.Spec.Content == nil {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "content cannot be empty"))
	}

	if d.Revision == nil {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "revision should be set"))
	}

	return nil
}
