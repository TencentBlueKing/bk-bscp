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
	"encoding/json"
	"fmt"

	"gorm.io/datatypes"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbbase "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
	pbcontent "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/content"
	pbkv "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/kv"
	pbrkv "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/released-kv"
	released_kv "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/released-kv"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// GetReleasedKv get released kv
func (s *Service) GetReleasedKv(ctx context.Context, req *pbds.GetReleasedKvReq) (*released_kv.ReleasedKv, error) {

	kt := kit.FromGrpcContext(ctx)

	rkv, err := s.dao.ReleasedKv().Get(kt, req.BizId, req.AppId, req.ReleaseId, req.Key)
	if err != nil {
		logs.Errorf("get released kv failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	kvType, value, err := s.getReleasedKv(kt, req.BizId, req.AppId, rkv.Spec.Version, req.ReleaseId, req.Key)
	if err != nil {
		logs.Errorf("get vault released kv failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return &pbrkv.ReleasedKv{
		Id:        rkv.ID,
		ReleaseId: rkv.ReleaseID,
		Spec: &pbkv.KvSpec{
			Key:    rkv.Spec.Key,
			KvType: string(kvType),
			Value:  value,
		},
		Attachment: &pbkv.KvAttachment{
			BizId: req.BizId,
			AppId: req.AppId,
		},
		Revision:    pbbase.PbRevision(rkv.Revision),
		ContentSpec: pbcontent.PbContentSpec(rkv.ContentSpec),
	}, nil

}

// ListReleasedKvs list app bound kv revisions.
func (s *Service) ListReleasedKvs(ctx context.Context, req *pbds.ListReleasedKvReq) (*pbds.ListReleasedKvResp, error) {

	kt := kit.FromGrpcContext(ctx)

	if len(req.Sort) == 0 {
		req.Sort = "key"
	}
	page := &types.BasePage{
		Start: req.Start,
		Limit: uint(req.Limit),
		Sort:  req.Sort,
		Order: types.Order(req.Order),
	}
	opt := &types.ListRKvOption{
		ReleaseID: req.ReleaseId,
		BizID:     req.BizId,
		AppID:     req.AppId,
		Key:       req.Key,
		SearchKey: req.SearchKey,
		All:       req.All,
		Page:      page,
		KvType:    req.KvType,
	}
	po := &types.PageOption{
		EnableUnlimitedLimit: true,
	}
	if err := opt.Validate(po); err != nil {
		return nil, err
	}
	details, count, err := s.dao.ReleasedKv().List(kt, opt)
	if err != nil {
		logs.Errorf("list released kv failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	var rkvs []*pbrkv.ReleasedKv
	for _, detail := range details {
		var val, name string
		if detail.Spec.KvType == table.KvTab {
			name, err = s.getKvTableConfigPreviewName(kt, req.BizId, detail.Spec.ManagedTableID, detail.Spec.ExternalSourceID)
			if err != nil {
				return nil, err
			}
			val, err = s.getReleasedKvTableConfigValue(kt, detail)
			if err != nil {
				return nil, err
			}
		}
		if detail.Spec.KvType != table.KvTab {
			_, val, err = s.getReleasedKv(kt, req.BizId, req.AppId, detail.Spec.Version, detail.ReleaseID, detail.Spec.Key)
			if err != nil {
				logs.Errorf("get vault released kv failed, err: %v, rid: %s", err, kt.Rid)
				return nil, err
			}
		}

		rkv, err := pbrkv.PbRKv(detail, val, name)
		if err != nil {
			return nil, err
		}
		rkvs = append(rkvs, rkv)
	}

	resp := &pbds.ListReleasedKvResp{
		Count:   uint32(count),
		Details: rkvs,
	}
	return resp, nil

}

func (s *Service) getReleasedKv(kt *kit.Kit, bizID, appID, version, releasedID uint32,
	key string) (table.DataType, string, error) {

	opt := &types.GetRKvOption{
		BizID:      bizID,
		AppID:      appID,
		Key:        key,
		Version:    int(version),
		ReleasedID: releasedID,
	}
	return s.vault.GetRKv(kt, opt)
}

func (s *Service) getReleasedKvTableConfigValue(kit *kit.Kit, rkv *table.ReleasedKv) (string, error) {
	if rkv == nil {
		return "", nil
	}
	contents, _, err := s.dao.ReleasedTableContent().List(kit, rkv.ID, &types.BasePage{All: true})
	if err != nil {
		return "", err
	}

	if len(contents) != 0 {
		result := make([]datatypes.JSONMap, 0)
		for _, v := range contents {
			result = append(result, v.Spec.Content)
		}

		// 将 result 切片转换为 JSON 格式的字符串
		contentBytes, err := json.Marshal(result)
		if err != nil {
			return "", fmt.Errorf("failed to marshal JSONMap slice: %w", err)
		}
		// 返回转换后的字符串
		return string(contentBytes), nil
	}

	return "", nil
}
