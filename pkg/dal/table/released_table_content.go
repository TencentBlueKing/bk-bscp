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
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"gorm.io/datatypes"
)

const TableNameReleasedTableContent = "released_table_contents"

// ReleasedTableContent mapped from table <released_table_contents>
type ReleasedTableContent struct {
	ID         uint32                          `gorm:"column:id" json:"id"`
	Attachment *ReleasedTableContentAttachment `json:"attachment" gorm:"embedded"`
	Spec       *ReleasedTableContentSpec       `json:"spec" gorm:"embedded"`
	Revision   *Revision                       `json:"revision" gorm:"embedded"`
}

// ReleasedTableContentAttachment xxx
type ReleasedTableContentAttachment struct {
	BizID       uint32 `gorm:"column:biz_id;not null" json:"biz_id"`
	AppID       uint32 `gorm:"column:app_id;not null" json:"app_id"`
	ReleaseKvID uint32 `gorm:"column:release_kv_id;not null" json:"release_kv_id"`
}

// DataSourceMappingSpec xxx
type ReleasedTableContentSpec struct {
	Content datatypes.JSONMap `gorm:"column:content;not null" json:"content"`
}

// TableName ReleasedTableContent's table name
func (*ReleasedTableContent) TableName() string {
	return TableNameReleasedTableContent
}

// AppID AuditRes interface
func (d *ReleasedTableContent) AppID() uint32 {
	return d.Attachment.AppID
}

// ResID AuditRes interface
func (d *ReleasedTableContent) ResID() uint32 {
	return d.ID
}

// ResType AuditRes interface
func (d *ReleasedTableContent) ResType() string {
	return "released_table_content"
}

// ValidateCreate 验证创建数据
func (d ReleasedTableContent) ValidateCreate(kit *kit.Kit) error {
	if d.Attachment.BizID == 0 {
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "invalid biz id"))
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
