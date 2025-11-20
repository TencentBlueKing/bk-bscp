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
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/types"
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
	// 待补充
}

// ConfigTemplateAttachment defines the config template attachments.
type ConfigTemplateAttachment struct {
	BizID    uint32 `json:"biz_id" gorm:"column:biz_id"`
	TenantID string `json:"tenant_id" gorm:"column:tenant_id"`
	// TemplateID 关联 BSCP templates 表
	TemplateID uint32 `json:"template_id" gorm:"column:template_id"`
	// ProcessIDs 关联cc服务模版下的模板进程，模版进程的实例数量等于所有使用了服务模版的模块的主机数量
	TemplateProcessIDs types.Uint32Slice `json:"template_process_ids" gorm:"column:template_process_ids;type:json;default:'[]'"`
	// ProcessInstanceIDs 关联cc中未通过服务模板创建的进程实例
	ProcessInstanceIDs types.Uint32Slice `json:"process_instance_ids" gorm:"column:process_instance_ids;type:json;default:'[]'"`
}

// TableName is the config template's database table name.
func (c *ConfigTemplate) TableName() Name {
	return ConfigTemplatesTable
}

// ResType AuditRes interface
func (c *ConfigTemplate) ResType() string {
	return string(enumor.ConfigTemplate)
}
