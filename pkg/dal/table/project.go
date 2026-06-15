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

	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

const (
	DefaultProjectName = "默认项目"
	DefaultCreator     = "system"
	projectKeyPrefix   = "BK-BSCP-"
)

// Project defines a project's detail information
type Project struct {
	// ID is an auto-increased value, which is a unique identity of a project.
	ID         uint32             `json:"id" gorm:"primaryKey"`
	Spec       *ProjectSpec       `json:"spec" gorm:"embedded"`
	Attachment *ProjectAttachment `json:"attachment" gorm:"embedded"`
	Revision   *Revision          `json:"revision" gorm:"embedded"`
}

// ProjectSpec defines the project spec information
type ProjectSpec struct {
	Name      string `json:"name" gorm:"column:name;"`
	Key       string `json:"key" gorm:"column:key;"`
	Memo      string `json:"memo" gorm:"column:memo;"`
	Protected bool   `json:"protected" gorm:"column:protected;"`
}

// ProjectAttachment defines the project attachment information
type ProjectAttachment struct {
	TenantID string `json:"tenant_id" gorm:"column:tenant_id"`
	BizID    uint32 `json:"biz_id" gorm:"column:biz_id"`
}

// TableName  is the Project's database table name.
func (p *Project) TableName() string {
	return "projects"
}

// AppID AuditRes interface
func (p *Project) AppID() uint32 {
	return 0
}

// ResID AuditRes interface
func (p *Project) ResID() uint32 {
	return p.ID
}

// ResType AuditRes interface
func (p *Project) ResType() string {
	return "project"
}

// ProjectID AuditRes interface, 项目自身即维度，返回自身 ID。
func (p *Project) ProjectID() uint32 {
	return p.ID
}

// ValidateCreate validate project spec when it is created.
func (p *ProjectSpec) Validate(kit *kit.Kit) error {
	if p.Name == "" {
		return errors.New(i18n.T(kit, "name should be set"))
	}

	if p.Key == "" {
		return errors.New(i18n.T(kit, "key should be set"))
	}

	return nil
}

func (p *ProjectAttachment) Validate(kit *kit.Kit) error {
	if p.BizID <= 0 {
		return errors.New(i18n.T(kit, "invalid attachment biz id"))
	}

	return nil
}

// ValidateCreate validate Project is valid or not when create it.
func (p *Project) ValidateCreate(kit *kit.Kit) error {

	if p.Spec == nil {
		return errors.New(i18n.T(kit, "spec not set"))
	}

	if err := p.Spec.Validate(kit); err != nil {
		return err
	}

	if p.Attachment == nil {
		return errors.New(i18n.T(kit, "attachment not set"))
	}

	if err := p.Attachment.Validate(kit); err != nil {
		return err
	}

	if p.Revision == nil {
		return errors.New(i18n.T(kit, "revision not set"))
	}

	if err := p.Revision.ValidateCreate(); err != nil {
		return err
	}

	return nil
}

func (p *Project) ValidateDelete(kit *kit.Kit) error {

	if p.ID <= 0 {
		return errors.New(i18n.T(kit, "id should be set"))
	}

	if p.Attachment == nil {
		return errors.New(i18n.T(kit, "attachment not set"))
	}

	if err := p.Attachment.Validate(kit); err != nil {
		return err
	}

	return nil
}

// GenerateProjectKey 生成项目 Key，格式为 BK-BSCP-XXXXX，主键 ID 左侧补零到 5 位。
func GenerateProjectKey(id uint32) string {
	return projectKeyPrefix + fmt.Sprintf("%05d", id)
}
