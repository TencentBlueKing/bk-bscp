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
	"time"
)

// BizHost defines business host relationship
type BizHost struct {
	BizID         uint      `db:"bk_biz_id" json:"bk_biz_id" gorm:"column:bk_biz_id;primaryKey;autoIncrement:false"`
	HostID        uint      `db:"bk_host_id" json:"bk_host_id" gorm:"column:bk_host_id;primaryKey;autoIncrement:false"`
	AgentID       string    `db:"bk_agent_id" json:"bk_agent_id" gorm:"column:bk_agent_id"`
	BKHostInnerIP string    `db:"bk_host_innerip" json:"bk_host_innerip" gorm:"column:bk_host_innerip"`
	LastUpdated   time.Time `db:"last_updated" json:"last_updated" gorm:"column:last_updated;autoUpdateTime"`
}

// TableName is the biz_host table name.
func (b *BizHost) TableName() Name {
	return BizHostTable
}
