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

package pbas

import (
	"testing"

	pbstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server/types"
)

// 权限中心逐级展开资源选择器时可能不带分页（limit=0），此时必须放行，
// 否则回调返回 400，选择器会整页提示"接入系统资源接口请求失败"。
func TestPullResourceReqAllowsEmptyPageLimit(t *testing.T) {
	filter := new(pbstruct.Struct)
	require.NoError(t, filter.UnmarshalJSON([]byte(`{"parent":{"type":"biz","id":"2"}}`)))

	req := &PullResourceReq{
		Type:   string(client.TypeID("project")),
		Method: string(types.ListInstanceMethod),
		Filter: filter,
		Page:   &Page{Limit: 0, Offset: 0},
	}

	parsed, err := req.PullResourceReq()
	require.NoError(t, err)
	require.Equal(t, uint(0), parsed.Page.Limit)

	filterParsed, ok := parsed.Filter.(types.ListInstanceFilter)
	require.True(t, ok, "filter should be parsed as list instance filter")
	require.NotNil(t, filterParsed.Parent)
	require.Equal(t, "2", filterParsed.Parent.ID)
}

// 超过权限中心单页上限时仍然拒绝，避免一次拉取过多实例。
func TestPullResourceReqRejectsOversizedPageLimit(t *testing.T) {
	req := &PullResourceReq{
		Type:   string(client.TypeID("project")),
		Method: string(types.ListInstanceMethod),
		Page:   &Page{Limit: client.BkIAMMaxPageSize + 1},
	}

	_, err := req.PullResourceReq()
	require.ErrorContains(t, err, "limit should")
}
