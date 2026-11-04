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

package iam

import (
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server/types"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// FetchInstanceInfo obtain resource instance details in batch.
func (i *IAM) FetchInstanceInfo(kt *kit.Kit, resType client.TypeID, ft *types.FetchInstanceInfoFilter) (
	[]*types.InstanceInfo, error) {

	// 权限中心生成申请链接时会不带 ids 调用，此时没有需要返回的实例。
	// 空 ids 返回空列表而不是报错，否则整个申请链接生成会失败，与 V4 的实现保持一致。
	if ft == nil || len(ft.IDs) == 0 {
		return make([]*types.InstanceInfo, 0), nil
	}

	// Note: f.Attrs need to deal with, if add attribute authentication.

	req := &pbds.FetchInstanceInfoReq{
		ResourceType: string(resType),
		Ids:          ft.IDs,
	}
	resp, err := i.ds.FetchInstanceInfo(kt.RpcCtx(), req)
	if err != nil {
		return nil, err
	}
	results := make([]*types.InstanceInfo, 0)
	for _, detail := range resp.Details {
		results = append(results, &types.InstanceInfo{
			ID:            detail.Id,
			DisplayName:   detail.DisplayName,
			BKIAMApprover: detail.Approver,
			BKIAMPath:     detail.Path,
		})
	}
	return results, nil
}
