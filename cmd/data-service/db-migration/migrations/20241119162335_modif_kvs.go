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
	"time"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/cmd/data-service/db-migration/migrator"
)

func init() {
	// add current migration to migrator
	migrator.GetMigrator().AddMigration(&migrator.Migration{
		Version: "20241119162335",
		Name:    "20241119162335_modif_kvs",
		Mode:    migrator.GormMode,
		Up:      mig20241119162335Up,
		Down:    mig20241119162335Down,
	})
}

// mig20241119162335Up for up migration
func mig20241119162335Up(tx *gorm.DB) error {
	// Kvs  : KV
	type Kvs struct {
		CertificateExpirationDate time.Time `gorm:"column:certificate_expiration_date;type:datetime(6);default:NULL"`
	}

	// ReleasedKvs  : 发布的KV
	type ReleasedKvs struct {
		CertificateExpirationDate time.Time `gorm:"column:certificate_expiration_date;type:datetime(6);default:NULL"`
	}

	// Kvs add new column
	if !tx.Migrator().HasColumn(&Kvs{}, "certificate_expiration_date") {
		if err := tx.Migrator().AddColumn(&Kvs{}, "certificate_expiration_date"); err != nil {
			return err
		}
	}

	// Released Kvs add new column
	if !tx.Migrator().HasColumn(&ReleasedKvs{}, "certificate_expiration_date") {
		if err := tx.Migrator().AddColumn(&ReleasedKvs{}, "certificate_expiration_date"); err != nil {
			return err
		}
	}

	return nil
}

// mig20241119162335Down for down migration
func mig20241119162335Down(tx *gorm.DB) error {
	// Kvs  : KV
	type Kvs struct {
		CertificateExpirationDate time.Time `gorm:"column:certificate_expiration_date;type:datetime(6);default:NULL"`
	}

	// ReleasedKvs  : 发布的KV
	type ReleasedKvs struct {
		CertificateExpirationDate time.Time `gorm:"column:certificate_expiration_date;type:datetime(6);default:NULL"`
	}

	// Kvs drop column
	if tx.Migrator().HasColumn(&Kvs{}, "certificate_expiration_date") {
		if err := tx.Migrator().DropColumn(&Kvs{}, "certificate_expiration_date"); err != nil {
			return err
		}
	}

	// Released Kvs drop column
	if tx.Migrator().HasColumn(&ReleasedKvs{}, "certificate_expiration_date") {
		if err := tx.Migrator().DropColumn(&ReleasedKvs{}, "certificate_expiration_date"); err != nil {
			return err
		}
	}

	return nil
}
