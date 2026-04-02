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

package brpc

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
)

// tenantExemptMethods lists gRPC methods that are exempt from tenant_id validation.
// These are system-level RPCs where the caller legitimately does not yet know
// or does not need the tenant_id:
//   - GetCurrentCursorReminder: global event cursor, not tenant-scoped
//   - ListEventsMeta: global event list, server-side uses WithSkipTenantFilter
//   - GetTenantIDByBiz: reverse-lookup tenant from biz_id (chicken-egg problem)
//   - BatchUpsertClientMetrics: clients/client_events tables are excluded from tenant filter
//   - BatchUpdateLastConsumedTime: system-level batch update, server-side uses WithSkipTenantFilter
//   - GetAuthConf: bootstrap config (login/ESB/CMDB), read from static config, not tenant-scoped
//   - GetUserInfo: cookie-based login exchanges token for user info (including tenant_id), caller does not yet know tenant_id
//   - GetAllBizsOfTmplSpaces: api-server bootstrap, enumerates all biz IDs with template spaces
var tenantExemptMethods = map[string]struct{}{
	"/pbcs.Cache/GetCurrentCursorReminder":   {},
	"/pbcs.Cache/ListEventsMeta":             {},
	"/pbcs.Cache/GetTenantIDByBiz":           {},
	"/pbds.Data/BatchUpsertClientMetrics":    {},
	"/pbds.Data/BatchUpdateLastConsumedTime": {},
	"/pbas.Auth/GetAuthConf":                 {},
	"/pbas.Auth/GetUserInfo":                 {},
	"/pbcs.Config/GetAllBizsOfTmplSpaces":    {},
	"/pbds.Data/GetAllBizsOfTmplSpaces":      {},
}

// isTenantExempt checks if the given method is exempt from tenant_id validation.
func isTenantExempt(fullMethod string) bool {
	_, ok := tenantExemptMethods[fullMethod]
	return ok
}

// TenantUnaryServerInterceptor returns a unary server interceptor that validates tenant_id
// is present in gRPC metadata when multi-tenant mode is enabled.
func TenantUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {

		if !cc.G().FeatureFlags.EnableMultiTenantMode {
			return handler(ctx, req)
		}

		if isTenantExempt(info.FullMethod) {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok || len(md.Get(strings.ToLower(constant.BkTenantID))) == 0 ||
			md.Get(strings.ToLower(constant.BkTenantID))[0] == "" {
			return nil, status.Errorf(codes.InvalidArgument,
				"tenant_id is required in multi-tenant mode, method: %s", info.FullMethod)
		}
		return handler(ctx, req)
	}
}

// TenantStreamServerInterceptor returns a stream server interceptor that validates tenant_id
// is present in gRPC metadata when multi-tenant mode is enabled.
func TenantStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo,
		handler grpc.StreamHandler) error {

		if !cc.G().FeatureFlags.EnableMultiTenantMode {
			return handler(srv, ss)
		}

		if isTenantExempt(info.FullMethod) {
			return handler(srv, ss)
		}

		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok || len(md.Get(strings.ToLower(constant.BkTenantID))) == 0 ||
			md.Get(strings.ToLower(constant.BkTenantID))[0] == "" {
			return status.Errorf(codes.InvalidArgument,
				"tenant_id is required in multi-tenant mode, method: %s", info.FullMethod)
		}
		return handler(srv, ss)
	}
}
