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

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// ManageConfigKV proxies the generic config KV management request to data-service.
func (s *Service) ManageConfigKV(ctx context.Context,
	req *pbcs.ManageConfigKVReq) (*pbcs.ManageConfigKVResp, error) {

	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}},
		{Basic: meta.Basic{Type: meta.GlobalConfigKV, Action: meta.ManageGlobalConfigKV}},
	}

	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	dsReq := &pbds.ManageConfigKVReq{
		Action:    req.Action,
		Key:       req.Key,
		KeyPrefix: req.KeyPrefix,
	}
	for _, kv := range req.Kvs {
		dsReq.Kvs = append(dsReq.Kvs, &pbds.ConfigKVItem{Key: kv.Key, Value: kv.Value})
	}

	resp, err := s.client.DS.ManageConfigKV(grpcKit.RpcCtx(), dsReq)
	if err != nil {
		logs.Errorf("manage config kv failed, action: %s, err: %v, rid: %s",
			req.Action, err, grpcKit.Rid)
		return nil, err
	}

	result := &pbcs.ManageConfigKVResp{}
	for _, item := range resp.Items {
		result.Items = append(result.Items, &pbcs.ConfigKVItem{Key: item.Key, Value: item.Value})
	}

	return result, nil
}
