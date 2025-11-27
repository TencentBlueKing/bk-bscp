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

import "github.com/TencentBlueKing/bk-bscp/pkg/criteria/enumor"

// ConfigInstance defines a config instance's detail information
type ConfigInstance struct {
	ID         uint32                    `json:"id" gorm:"primaryKey"`
	Attachment *ConfigInstanceAttachment `json:"attachment" gorm:"embedded"`
	Revision   *Revision                 `json:"revision" gorm:"embedded"`
}

// ConfigInstanceAttachment defines the config instance attachments.
type ConfigInstanceAttachment struct {
	// BizID is the business ID.
	BizID uint32 `json:"biz_id" gorm:"column:biz_id"`
	// ConfigTemplateID is the config template ID.
	ConfigTemplateID uint32 `json:"config_template_id" gorm:"column:config_template_id"`
	// ConfigVersionID is the config template version ID.
	ConfigVersionID uint32 `json:"config_version_id" gorm:"column:config_version_id"`
	// CcProcessID cc进程id
	CcProcessID uint32 `json:"cc_process_id" gorm:"column:cc_process_id"`
	// InstID 模块下的进程实例序列号
	ModuleInstSeq uint32 `json:"module_inst_seq" gorm:"column:module_inst_seq"`
	// AppID 每个配置实例关联一个应用
	AppID uint32 `json:"app_id" gorm:"column:app_id"`
	// ReleaseID 每个配置实例关联一个发布版本
	ReleaseID uint32 `json:"release_id" gorm:"column:release_id"`
	// ReleaseConfigItem 每个配置实例关联的配置项
	ReleaseConfigItemID uint32 `json:"release_config_item_id" gorm:"column:release_config_item_id"`
	// BatchID is the batch ID.
	BatchID uint32 `json:"batch_id" gorm:"column:batch_id"`
	// TaskID is the task ID.
	TaskID string `json:"task_id" gorm:"column:task_id"`
	// TenantID is the tenant ID.
	TenantID string `json:"tenant_id" gorm:"column:tenant_id"`
}

// TableName is the config instance's database table name.
func (c *ConfigInstance) TableName() Name {
	return ConfigInstancesTable
}

// ResType AuditRes interface
func (c *ConfigInstance) ResType() string {
	return string(enumor.ConfigInstance)
}
