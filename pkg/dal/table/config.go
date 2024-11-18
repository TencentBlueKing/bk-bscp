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

// Config defines a config model
type Config struct {
	// ID is an auto-increased value, which is a unique identity
	// of a itsm_config.
	ID    uint32 `db:"id" json:"id" gorm:"primaryKey"`
	Key   string `db:"key" json:"key" gorm:"column:key"`
	Value string `db:"value" json:"value" gorm:"column:value"`
}

// TableName is the strategy's database table name.
func (i *Config) TableName() string {
	return "configs"
}
