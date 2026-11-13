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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	pbas "github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server"
)

// CheckPermission 的申请信息只应包含未通过的资源，已授权资源不能重复返回。
func TestUnauthorizedResources(t *testing.T) {
	resources := []*pbas.ResourceAttribute{
		{Basic: &pbas.Basic{Type: string(meta.Biz), Action: string(meta.FindBusinessResource)}, BizId: 1},
		{Basic: &pbas.Basic{Type: string(meta.Credential), Action: string(meta.View)}, BizId: 1},
	}

	tests := []struct {
		name      string
		decisions []*pbas.Decision
		want      int
	}{
		{
			name:      "all authorized",
			decisions: []*pbas.Decision{{Authorized: true}, {Authorized: true}},
			want:      0,
		},
		{
			name:      "only credential missing",
			decisions: []*pbas.Decision{{Authorized: true}, {Authorized: false}},
			want:      1,
		},
		{
			name:      "nil decision kept",
			decisions: []*pbas.Decision{nil, {Authorized: false}},
			want:      2,
		},
		{
			name:      "mismatched decisions fallback to all",
			decisions: []*pbas.Decision{{Authorized: true}},
			want:      2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Len(t, unauthorizedResources(tc.decisions, resources), tc.want)
		})
	}

	missing := unauthorizedResources([]*pbas.Decision{{Authorized: true}, {Authorized: false}}, resources)
	require.Len(t, missing, 1)
	require.Equal(t, string(meta.Credential), missing[0].Basic.Type)
}
