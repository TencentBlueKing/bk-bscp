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

package migrations

import (
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/cmd/data-service/db-migration/migrator"
)

func init() {
	// add current migration to migrator
	migrator.GetMigrator().AddMigration(&migrator.Migration{
		Version: "20250311162946",
		Name:    "20250311162946_complete_unique_index",
		Mode:    migrator.GormMode,
		Up:      mig20250311162946Up,
		Down:    mig20250311162946Down,
	})
}

// mig20250311162946Up for up migration
// nolint:funlen
func mig20250311162946Up(tx *gorm.DB) error {
	type Release struct {
		ID         uint32    `db:"id" json:"id" gorm:"column:id;primaryKey"`
		Name       string    `db:"name" json:"name" gorm:"column:name;uniqueIndex:idx_bizID_appID_name"`
		BizID      uint32    `db:"biz_id" json:"biz_id" gorm:"column:biz_id;uniqueIndex:idx_bizID_appID_name"`
		AppID      uint32    `db:"app_id" json:"app_id" gorm:"column:app_id;uniqueIndex:idx_bizID_appID_name"`
		Deprecated bool      `db:"deprecated" json:"deprecated" gorm:"column:deprecated"`
		PublishNum uint32    `db:"publish_num" json:"publish_num" gorm:"column:publish_num"`
		CreatedAt  time.Time `db:"created_at" json:"created_at" gorm:"column:created_at"`
	}
	type Application struct {
		// ID is an auto-increased value, which is an application's unique identity.
		ID uint32 `json:"id" gorm:"primaryKey"`
		// BizID is the business is which this app belongs to
		BizID uint32 `json:"biz_id" gorm:"column:biz_id;uniqueIndex:idx_bizID_name"`
		// Name is application's name
		Name string `json:"name" gorm:"column:name;uniqueIndex:idx_bizID_name"`
	}
	type Group struct {
		ID    uint32 `json:"id" gorm:"primaryKey"`
		BizID uint32 `json:"biz_id" gorm:"column:biz_id;uniqueIndex:idx_bizID_name"`
		Name  string `json:"name" gorm:"column:name;uniqueIndex:idx_bizID_name"`
	}

	type RepeatedRelease struct {
		Name  string `db:"name" json:"name" gorm:"column:name;uniqueIndex:idx_bizID_appID_name"`
		BizID uint32 `db:"biz_id" json:"biz_id" gorm:"column:biz_id;uniqueIndex:idx_bizID_appID_name"`
		AppID uint32 `db:"app_id" json:"app_id" gorm:"column:app_id;uniqueIndex:idx_bizID_appID_name"`
	}

	// 处理历史 release 脏数据，删除唯一键重复的数据
	// 因为创建 release 流程较长，容易出现重复创建的情况
	repeated := make([]RepeatedRelease, 0)
	if err := tx.Model(&Release{}).
		Select("name, biz_id, app_id").
		Group("name, biz_id, app_id").
		Having("count(*) > 1").
		Scan(&repeated).Error; err != nil {
		return err
	}

	for _, r := range repeated {
		releases := make([]Release, 0)
		if err := tx.Where("name = ? AND biz_id = ? AND app_id = ?", r.Name, r.BizID, r.AppID).
			Find(&releases).Error; err != nil {
			return err
		}

		// 检查是否存在多个有效记录（非废弃且已发布）
		validCount := 0
		for _, rel := range releases {
			if !rel.Deprecated && rel.PublishNum > 0 {
				validCount++
			}
		}
		if validCount > 1 {
			return fmt.Errorf("multiple active releases [%d]-[%d]-[%s] found, need manual handling",
				r.BizID, r.AppID, r.Name)
		}

		// compare and delete
		sort.Slice(releases, func(i, j int) bool {
			// 优先删除已废弃的版本
			if releases[i].Deprecated != releases[j].Deprecated {
				return releases[i].Deprecated
			}
			// 优先删除未发布的版本
			if releases[i].PublishNum == 0 && releases[j].PublishNum != 0 {
				return true
			} else if releases[j].PublishNum == 0 && releases[i].PublishNum != 0 {
				return false
			}
			// 优先删除创建时间较早的版本
			return releases[i].ID < releases[j].ID
		})
		// 保留最后一个，删除其他
		toDelete := make([]uint32, 0, len(releases)-1)
		for i := range len(releases) - 1 {
			toDelete = append(toDelete, releases[i].ID)
		}
		if len(toDelete) > 0 {
			if err := tx.Where("id IN (?)", toDelete).Delete(&Release{}).Error; err != nil {
				return err
			}
		}
	}

	// 创建唯一索引
	if !tx.Migrator().HasIndex(&Release{}, "idx_bizID_appID_name") {
		if err := tx.Migrator().CreateIndex(&Release{}, "idx_bizID_appID_name"); err != nil {
			return err
		}
	}
	if !tx.Migrator().HasIndex(&Application{}, "idx_bizID_name") {
		if err := tx.Migrator().CreateIndex(&Application{}, "idx_bizID_name"); err != nil {
			return err
		}
	}
	if !tx.Migrator().HasIndex(&Group{}, "idx_bizID_name") {
		if err := tx.Migrator().CreateIndex(&Group{}, "idx_bizID_name"); err != nil {
			return err
		}
	}

	return nil
}

// mig20250311162946Down for down migration
func mig20250311162946Down(tx *gorm.DB) error {
	type Release struct {
		ID         uint32    `db:"id" json:"id" gorm:"column:id;primaryKey"`
		Name       string    `db:"name" json:"name" gorm:"column:name;uniqueIndex:idx_bizID_appID_name"`
		BizID      uint32    `db:"biz_id" json:"biz_id" gorm:"column:biz_id;uniqueIndex:idx_bizID_appID_name"`
		AppID      uint32    `db:"app_id" json:"app_id" gorm:"column:app_id;uniqueIndex:idx_bizID_appID_name"`
		Deprecated bool      `db:"deprecated" json:"deprecated" gorm:"column:deprecated"`
		PublishNum uint32    `db:"publish_num" json:"publish_num" gorm:"column:publish_num"`
		CreatedAt  time.Time `db:"created_at" json:"created_at" gorm:"column:created_at"`
	}
	type Application struct {
		// ID is an auto-increased value, which is an application's unique identity.
		ID uint32 `json:"id" gorm:"primaryKey"`
		// BizID is the business is which this app belongs to
		BizID uint32 `json:"biz_id" gorm:"column:biz_id;uniqueIndex:idx_bizID_name"`
		// Name is application's name
		Name string `json:"name" gorm:"column:name;uniqueIndex:idx_bizID_name"`
	}
	type Group struct {
		ID    uint32 `json:"id" gorm:"primaryKey"`
		BizID uint32 `json:"biz_id" gorm:"column:biz_id;uniqueIndex:idx_bizID_name"`
		Name  string `json:"name" gorm:"column:name;uniqueIndex:idx_bizID_name"`
	}
	if tx.Migrator().HasIndex(&Release{}, "idx_bizID_appID_name") {
		if err := tx.Migrator().DropIndex(&Release{}, "idx_bizID_appID_name"); err != nil {
			return err
		}
	}
	if tx.Migrator().HasIndex(&Application{}, "idx_bizID_name") {
		if err := tx.Migrator().DropIndex(&Application{}, "idx_bizID_name"); err != nil {
			return err
		}
	}
	if tx.Migrator().HasIndex(&Group{}, "idx_bizID_name") {
		if err := tx.Migrator().DropIndex(&Group{}, "idx_bizID_name"); err != nil {
			return err
		}
	}

	return nil
}
