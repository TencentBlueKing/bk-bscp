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

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/validator"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

const (
	DefaultEnvName = "Default"
)

// Environment defines an environment's detail information
type Environment struct {
	// ID is an auto-increased value, which is a unique identity of a Environment.
	ID         uint32                 `json:"id" gorm:"primaryKey"`
	Spec       *EnvironmentSpec       `json:"spec" gorm:"embedded"`
	Attachment *EnvironmentAttachment `json:"attachment" gorm:"embedded"`
	Revision   *Revision              `json:"revision" gorm:"embedded"`
}

// EnvironmentSpec defines the environment spec information
type EnvironmentSpec struct {
	Name         string          `json:"name" gorm:"column:name;"`
	Type         EnvironmentType `json:"type" gorm:"column:type;"`
	Memo         string          `json:"memo" gorm:"column:memo;"`
	DisplayOrder uint32          `json:"display_order" gorm:"column:display_order;"`
	Protected    bool            `json:"protected" gorm:"column:protected;"`
}

// EnvironmentAttachment defines the environment attachment information
type EnvironmentAttachment struct {
	TenantID  string `json:"tenant_id" gorm:"column:tenant_id"`
	BizID     uint32 `json:"biz_id" gorm:"column:biz_id"`
	ProjectID uint32 `json:"project_id" gorm:"column:project_id"`
}

// TableName  is the Environment's database table name.
func (e *Environment) TableName() string {
	return "environments"
}

// AppID AuditRes interface
func (e *Environment) AppID() uint32 {
	return 0
}

// ResID AuditRes interface
func (e *Environment) ResID() uint32 {
	return e.ID
}

// ResType AuditRes interface
func (e *Environment) ResType() string {
	return "environment"
}

// ProjectID AuditRes interface
func (e *Environment) ProjectID() uint32 {
	if e.Attachment == nil {
		return 0
	}
	return e.Attachment.ProjectID
}

// EnvironmentType defines the environment type
type EnvironmentType string

const (
	// EnvironmentTypeProd is the prod environment type.
	EnvironmentTypeProd EnvironmentType = "prod"
	// EnvironmentTypeStaging is the staging environment type.
	EnvironmentTypeStaging EnvironmentType = "staging"
	// EnvironmentTypeTest is the test environment type.
	EnvironmentTypeTest EnvironmentType = "test"
	// EnvironmentTypeDev is the dev environment type.
	EnvironmentTypeDev EnvironmentType = "dev"
)

// Validate validates the EnvironmentType is valid or not.
func (et EnvironmentType) Validate(kit *kit.Kit) error {
	switch et {
	case EnvironmentTypeProd, EnvironmentTypeStaging, EnvironmentTypeTest, EnvironmentTypeDev:
		return nil
	default:
		return errors.New(i18n.T(kit, "invalid environment type"))
	}
}

// String returns the string format of EnvironmentType.
func (e EnvironmentType) String() string {
	return string(e)
}

// ValidateCreate validate environment spec when it is created.
func (e *EnvironmentSpec) Validate(kit *kit.Kit) error {
	if e.Name == "" {
		return errors.New(i18n.T(kit, "name should be set"))
	}

	if err := validator.ValidateEnvName(kit, e.Name); err != nil {
		return err
	}

	if err := e.Type.Validate(kit); err != nil {
		return err
	}

	return nil
}

func (e *EnvironmentAttachment) Validate(kit *kit.Kit) error {
	if e.BizID <= 0 {
		return errors.New(i18n.T(kit, "invalid attachment biz id"))
	}

	if e.ProjectID <= 0 {
		return errors.New(i18n.T(kit, "invalid attachment project id"))
	}

	return nil
}

// ValidateCreate validate Environment is valid or not when create it.
func (e *Environment) ValidateCreate(kit *kit.Kit) error {

	if e.Spec == nil {
		return errors.New(i18n.T(kit, "spec not set"))
	}

	if err := e.Spec.Validate(kit); err != nil {
		return err
	}

	if e.Attachment == nil {
		return errors.New(i18n.T(kit, "attachment not set"))
	}

	if err := e.Attachment.Validate(kit); err != nil {
		return err
	}

	if e.Revision == nil {
		return errors.New(i18n.T(kit, "revision not set"))
	}

	if err := e.Revision.ValidateCreate(); err != nil {
		return err
	}

	return nil
}

func (e *Environment) ValidateDelete(kit *kit.Kit) error {

	if e.ID <= 0 {
		return errors.New(i18n.T(kit, "id should be set"))
	}

	if e.Attachment == nil {
		return errors.New(i18n.T(kit, "attachment not set"))
	}

	if err := e.Attachment.Validate(kit); err != nil {
		return err
	}

	return nil
}

func (e *Environment) ValidateUpdate(kit *kit.Kit) error {
	if e.ID <= 0 {
		return errors.New(i18n.T(kit, "id should be set"))
	}

	if e.Spec == nil {
		return errors.New(i18n.T(kit, "spec not set"))
	}

	if err := e.Spec.Validate(kit); err != nil {
		return err
	}

	if e.Attachment == nil {
		return errors.New(i18n.T(kit, "attachment not set"))
	}

	if err := e.Attachment.Validate(kit); err != nil {
		return err
	}

	if e.Revision == nil {
		return errors.New(i18n.T(kit, "revision not set"))
	}

	if err := e.Revision.ValidateUpdate(); err != nil {
		return err
	}

	return nil
}
