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
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/enumor"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/types"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// ConfigTemplate defines a config template's detail information
type ConfigTemplate struct {
	ID         uint32                    `json:"id" gorm:"primaryKey"`
	Spec       *ConfigTemplateSpec       `json:"spec" gorm:"embedded"`
	Attachment *ConfigTemplateAttachment `json:"attachment" gorm:"embedded"`
	Revision   *Revision                 `json:"revision" gorm:"embedded"`
}

// ConfigTemplateSpec defines all the specifics for config template set by user.
type ConfigTemplateSpec struct {
	// 配置模版名称, 需要区别于templates表的name字段。templates表的name用于和path共同组成文件的路径
	Name           string         `json:"name" gorm:"column:name"`
	HighlightStyle HighlightStyle `json:"highlight_style" gorm:"column:highlight_style"`
}

// ConfigTemplateAttachment defines the config template attachments.
type ConfigTemplateAttachment struct {
	BizID      uint32 `json:"biz_id" gorm:"column:biz_id"` // TemplateID 关联 BSCP templates 表
	TemplateID uint32 `json:"template_id" gorm:"column:template_id"`
	// CcTemplateProcessIDs 关联cc服务模版下的模板进程
	CcTemplateProcessIDs types.Uint32Slice `json:"cc_template_process_ids" gorm:"column:cc_template_process_ids;type:json;default:'[]'"`
	// CcProcessIDs 关联cc中未通过服务模板创建的进程
	CcProcessIDs types.Uint32Slice `json:"cc_process_ids" gorm:"column:cc_process_ids;type:json;default:'[]'"`
	TenantID     string            `json:"tenant_id" gorm:"column:tenant_id"`
}

// TableName is the config template's database table name.
func (c *ConfigTemplate) TableName() Name {
	return ConfigTemplatesTable
}

// AppID AuditRes interface
func (c *ConfigTemplate) AppID() uint32 {
	return 0
}

// ResType AuditRes interface
func (c *ConfigTemplate) ResType() string {
	return string(enumor.ConfigTemplate)
}

// ResID AuditRes interface
func (c *ConfigTemplate) ResID() uint32 {
	return c.ID
}

const (
	// HighlightStylePython Python
	HighlightStylePython HighlightStyle = "python"
	// HighlightStyleShell Shell
	HighlightStyleShell HighlightStyle = "shell"
	// HighlightStylePowershell PowerShell
	HighlightStylePowershell HighlightStyle = "powershell"
	// HighlightStyleJSON JSON
	HighlightStyleJSON HighlightStyle = "json"
	// HighlightStyleYAML YAML
	HighlightStyleYAML HighlightStyle = "yaml"
)

// HighlightStyle style type
type HighlightStyle string

// Validate the highlight style is supported or not.
func (h HighlightStyle) Validate(kit *kit.Kit) error {
	switch h {
	case HighlightStylePython,
		HighlightStyleShell,
		HighlightStylePowershell,
		HighlightStyleJSON,
		HighlightStyleYAML:
		return nil

	default:
		return errf.Errorf(errf.InvalidArgument,
			i18n.T(kit, "unsupported highlight style: %s", h))
	}
}
