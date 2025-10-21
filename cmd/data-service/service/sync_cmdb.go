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

package service

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/processor/cmdb"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// SyncCMDB implements pbds.DataServer.
func (s *Service) SyncCMDB(ctx context.Context, req *pbds.SyncCMDBReq) (*pbds.SyncCMDBResp, error) {
	// grpcKit := kit.FromGrpcContext(ctx)

	// s.SynchronizeCmdbData(ctx, []int{3})

	return &pbds.SyncCMDBResp{
		TaskId: 0,
	}, nil

}

// SynchronizeCmdbData 同步cmdb数据
func (s *Service) SynchronizeCmdbData(ctx context.Context, bizIDs []int) error {
	// 不指定业务同步，表示同步所有业务
	if len(bizIDs) == 0 {
		bizList, err := s.cmdb.SearchBusinessByAccount(ctx, bkcmdb.SearchSetReq{
			BkSupplierAccount: "0",
			Fields:            []string{"bk_biz_id", "bk_biz_name"},
		})
		if err != nil {
			return fmt.Errorf("get business data failed: %v", err)
		}

		var business bkcmdb.Business
		if err := bizList.Decode(&business); err != nil {
			return fmt.Errorf("parse business data: %v", err)
		}

		for _, item := range business.Info {
			bizIDs = append(bizIDs, item.BkBizID)
		}
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(5)

	for _, id := range bizIDs {
		bizID := id
		g.Go(func() error {
			svc := cmdb.NewSyncCMDBService(bizID, s.cmdb, s.dao)
			return svc.SyncSingleBiz(gctx)
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}
