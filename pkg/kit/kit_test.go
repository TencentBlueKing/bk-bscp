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

package kit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
)

func TestGRPCTenantID(t *testing.T) {
	kt := &Kit{
		Ctx:      context.Background(),
		TenantID: "default",
	}

	ctx := kt.InternalRpcCtx()
	newKit := FromGrpcContext(ctx)
	assert.Equal(t, kt.TenantID, newKit.TenantID)
}

func TestFromGrpcContextWritesRidIntoContext(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(constant.RidKey, "rid-1"))

	kt := FromGrpcContext(ctx)

	assert.Equal(t, "rid-1", kt.Rid)
	assert.Equal(t, "rid-1", kt.Ctx.Value(constant.RidKey))
}

func TestFromGrpcContextGeneratesRidIntoContext(t *testing.T) {
	kt := FromGrpcContext(metadata.NewIncomingContext(context.Background(), metadata.MD{}))

	assert.NotEmpty(t, kt.Rid)
	assert.Equal(t, kt.Rid, kt.Ctx.Value(constant.RidKey))
}

func TestFromGrpcContextPrefersRidFromContextValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), constant.RidKey, "rid-from-ctx") //nolint:staticcheck
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(constant.RidKey, "rid-from-md"))

	kt := FromGrpcContext(ctx)

	assert.Equal(t, "rid-from-ctx", kt.Rid)
	assert.Equal(t, "rid-from-ctx", kt.Ctx.Value(constant.RidKey))
}
