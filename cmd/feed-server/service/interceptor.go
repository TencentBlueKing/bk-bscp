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
	"path"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	"github.com/TencentBlueKing/bk-bscp/internal/runtime/brpc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbfs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/feed-server"
	"github.com/TencentBlueKing/bk-bscp/pkg/runtime/jsoni"
	sfs "github.com/TencentBlueKing/bk-bscp/pkg/sf-share"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

var (
	// 老的请求,不使用中间件
	disabledMethod = map[string]struct{}{
		"/pbfs.Upstream/Handshake":            {},
		"/pbfs.Upstream/Messaging":            {},
		"/pbfs.Upstream/Watch":                {},
		"/pbfs.Upstream/PullAppFileMeta":      {},
		"/pbfs.Upstream/GetDownloadURL":       {},
		"/pbfs.Upstream/GetSingleFileContent": {},
	}
)

// ctxKey context key
type ctxKey int

const (
	credentialKey ctxKey = iota
)

func withCredential(ctx context.Context, value *types.CredentialCache) context.Context {
	return context.WithValue(ctx, credentialKey, value)
}

// getCredential 包内私有方法断言, 认为一直可用
func getCredential(ctx context.Context) *types.CredentialCache {
	return ctx.Value(credentialKey).(*types.CredentialCache)
}

func getBearerToken(md metadata.MD) (string, error) {
	values := md.Get("authorization")
	if len(values) < 1 {
		return "", fmt.Errorf("missing authorization header")
	}

	authorizationHeader := values[0]
	authHeaderParts := strings.Split(authorizationHeader, " ")
	if len(authHeaderParts) != 2 || strings.ToLower(authHeaderParts[0]) != "bearer" {
		return "", fmt.Errorf("invalid authorization header format")
	}

	return authHeaderParts[1], nil
}

func (s *Service) authorize(ctx context.Context, bizID uint32) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Aborted, "missing grpc metadata")
	}

	token, err := getBearerToken(md)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	cred, err := s.bll.Auth().GetCred(kit.FromGrpcContext(ctx), bizID, token)
	if err != nil {
		if isNotFoundErr(err) {
			return nil, err
		}
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	if !cred.Enabled {
		return nil, status.Errorf(codes.PermissionDenied, "credential is disabled")
	}

	// 获取scope，到下一步处理
	ctx = withCredential(ctx, cred)
	return ctx, nil
}

// FeedUnaryAuthInterceptor feed 鉴权中间件
func FeedUnaryAuthInterceptor(
	ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// 兼容老的请求
	if _, ok := disabledMethod[info.FullMethod]; ok {
		return handler(ctx, req)
	}

	var bizID uint32
	switch r := req.(type) {
	case interface{ GetBizId() uint32 }: // 请求都必须有 uint32 biz_id 参数
		bizID = r.GetBizId()
	default:
		return nil, status.Error(codes.Aborted, "missing bizId in request")
	}

	ctx = context.WithValue(ctx, constant.BizIDKey, bizID) //nolint:staticcheck

	svc, ok := info.Server.(*Service)
	// 处理非业务 Service 时不鉴权，如 GRPC Reflection
	if !ok {
		return handler(ctx, req)
	}

	ctx, err := svc.authorize(ctx, bizID)
	if err != nil {
		return nil, err
	}

	return handler(ctx, req)
}

// LogUnaryServerInterceptor 添加请求日志
func LogUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (
		resp any, err error) {
		st := time.Now()
		kt := kit.FromGrpcContext(ctx)
		service := path.Dir(info.FullMethod)[1:]
		method := path.Base(info.FullMethod)
		realIP := brpc.MustGetRealIP(ctx)

		biz, app := extractBizIDAndApp(req, info.FullMethod)

		defer func() {
			if err != nil {
				klog.InfoS("grpc", "rid", kt.Rid, "ip", realIP, "biz", biz, "app", app,
					"service", service, "method", method, "grpc.duration", time.Since(st), "err", err)
				return
			}

			klog.InfoS("grpc", "rid", kt.Rid, "ip", realIP, "biz", biz, "app", app,
				"service", service, "method", method, "grpc.duration", time.Since(st))
		}()

		resp, err = handler(ctx, req)
		return resp, err
	}
}

// FeedUnaryUpdateLastConsumedTimeInterceptor feed 更新拉取时间中间件
func FeedUnaryUpdateLastConsumedTimeInterceptor(ctx context.Context, req interface{},
	info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

	svc, ok := info.Server.(*Service)
	// 跳过非业务 Service，如 GRPC Reflection
	if !ok {
		return handler(ctx, req)
	}

	type lastConsumedTime struct {
		BizID    uint32
		AppNames []string
		AppIDs   []uint32
	}

	param := lastConsumedTime{}

	switch info.FullMethod {
	case pbfs.Upstream_GetKvValue_FullMethodName:
		request := req.(*pbfs.GetKvValueReq)
		param.BizID = request.BizId
		param.AppNames = append(param.AppNames, request.GetAppMeta().GetApp())
	case pbfs.Upstream_PullKvMeta_FullMethodName:
		request := req.(*pbfs.PullKvMetaReq)
		param.BizID = request.BizId
		param.AppNames = append(param.AppNames, request.GetAppMeta().GetApp())
	case pbfs.Upstream_Messaging_FullMethodName:
		request := req.(*pbfs.MessagingMeta)
		if sfs.MessagingType(request.Type) == sfs.VersionChangeMessage {
			vc := new(sfs.VersionChangePayload)
			if err := vc.Decode(request.Payload); err != nil {
				logs.Errorf("version change message decoding failed, %s", err.Error())
				return handler(ctx, req)
			}
			param.BizID = vc.BasicData.BizID
			param.AppNames = append(param.AppNames, vc.Application.App)
		}
	case pbfs.Upstream_Watch_FullMethodName:
		request := req.(*pbfs.SideWatchMeta)
		payload := new(sfs.SideWatchPayload)
		if err := jsoni.Unmarshal(request.Payload, payload); err != nil {
			logs.Errorf("parse request payload failed, %s", err.Error())
			return handler(ctx, req)
		}
		param.BizID = payload.BizID
		for _, v := range payload.Applications {
			param.AppNames = append(param.AppNames, v.App)
		}
	case pbfs.Upstream_PullAppFileMeta_FullMethodName:
		request := req.(*pbfs.PullAppFileMetaReq)
		param.BizID = request.BizId
		param.AppNames = append(param.AppNames, request.GetAppMeta().GetApp())
	case pbfs.Upstream_GetDownloadURL_FullMethodName:
		request := req.(*pbfs.GetDownloadURLReq)
		param.BizID = request.BizId
		param.AppIDs = append(param.AppIDs, request.GetFileMeta().GetConfigItemAttachment().GetAppId())
	case pbfs.Upstream_GetSingleKvValue_FullMethodName, pbfs.Upstream_GetSingleKvMeta_FullMethodName:
		request := req.(*pbfs.GetSingleKvValueReq)
		param.BizID = request.BizId
		param.AppNames = append(param.AppNames, request.GetAppMeta().GetApp())
	default:
		return handler(ctx, req)
	}

	if param.BizID != 0 {
		ctx = context.WithValue(ctx, constant.BizIDKey, param.BizID) //nolint:staticcheck

		if len(param.AppIDs) == 0 {
			for _, appName := range param.AppNames {
				appID, err := svc.bll.AppCache().GetAppID(kit.FromGrpcContext(ctx), param.BizID, appName)
				if err != nil {
					logs.Errorf("get app id failed, err: %v", err)
					return handler(ctx, req)
				}
				param.AppIDs = append(param.AppIDs, appID)
			}
		}

		if err := svc.bll.AppCache().SetAppLastConsumedTime(kit.FromGrpcContext(ctx),
			param.BizID, param.AppIDs); err != nil {
			logs.Errorf("set app last consumed time failed, err: %v", err)
			return handler(ctx, req)
		}
		logs.Infof("set app last consumed time success")
	}

	return handler(ctx, req)
}

// wrappedStream stream 封装, 可自定义 context 传值
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context 覆盖 context
func (s *wrappedStream) Context() context.Context {
	return s.ctx
}

// FeedStreamAuthInterceptor feed 鉴权中间件
func FeedStreamAuthInterceptor(
	srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	// 兼容老的请求
	if _, ok := disabledMethod[info.FullMethod]; ok {
		return handler(srv, ss)
	}

	var bizID uint32
	svc, ok := srv.(*Service)
	// 处理非业务 Service 时不鉴权，如 GRPC Reflection
	if !ok {
		return handler(srv, ss)
	}
	ctx, err := svc.authorize(ss.Context(), bizID)
	if err != nil {
		return err
	}

	w := &wrappedStream{ServerStream: ss, ctx: ctx}
	return handler(srv, w)
}

type bizAppParam struct {
	BizID uint32
	App   string
}

func extractBizAndApp(req any, fullMethod string) *bizAppParam {
	switch fullMethod {
	case pbfs.Upstream_Messaging_FullMethodName:
		if m, ok := req.(*pbfs.MessagingMeta); ok {
			switch sfs.MessagingType(m.Type) {
			case sfs.VersionChangeMessage:
				vc := new(sfs.VersionChangePayload)
				if err := vc.Decode(m.Payload); err != nil {
					logs.Errorf("version change message decoding failed: %v", err)
					break
				}
				return &bizAppParam{
					BizID: vc.BasicData.BizID,
					App:   vc.Application.App,
				}

			case sfs.Heartbeat:
				hb := new(sfs.HeartbeatPayload)
				if err := hb.Decode(m.Payload); err != nil {
					logs.Errorf("heartbeat payload decoding failed: %v", err)
					break
				}
				if len(hb.Applications) > 0 {
					var apps []string
					for _, v := range hb.Applications {
						apps = append(apps, v.App)
					}
					return &bizAppParam{
						BizID: hb.BasicData.BizID,
						App:   strings.Join(apps, ","),
					}
				}
				return &bizAppParam{
					BizID: hb.BasicData.BizID,
				}
			}

		}
	case pbfs.Upstream_Watch_FullMethodName:
		if m, ok := req.(*pbfs.SideWatchMeta); ok {
			payload := new(sfs.SideWatchPayload)
			if err := jsoni.Unmarshal(m.Payload, payload); err != nil {
				logs.Errorf("watch payload unmarshal failed: %v", err)
				return nil
			}
			var apps []string
			for _, app := range payload.Applications {
				apps = append(apps, app.App)
			}
			return &bizAppParam{
				BizID: payload.BizID,
				App:   strings.Join(apps, ","),
			}
		}
	case pbfs.Upstream_Handshake_FullMethodName:
		if m, ok := req.(*pbfs.HandshakeMessage); ok && m.Spec != nil {
			return &bizAppParam{
				BizID: m.Spec.BizId,
			}
		}
	}
	return nil
}

func extractBizIDAndApp(req any, fullMethod string) (uint32, string) {
	// 尝试接口断言提取
	if r, ok := req.(interface{ GetBizId() uint32 }); ok {
		biz := r.GetBizId()

		if am, ok := req.(interface {
			GetAppMeta() *pbfs.AppMeta
		}); ok && am.GetAppMeta() != nil {
			return biz, am.GetAppMeta().GetApp()
		}

		if fm, ok := req.(interface {
			GetFileMeta() *pbfs.FileMeta
		}); ok && fm.GetFileMeta() != nil && fm.GetFileMeta().GetConfigItemAttachment() != nil {
			app := fmt.Sprintf("%d", fm.GetFileMeta().GetConfigItemAttachment().GetAppId())
			return biz, app
		}

		return biz, ""
	}

	// payload 特殊提取逻辑
	if param := extractBizAndApp(req, fullMethod); param != nil {
		return param.BizID, param.App
	}

	return 0, ""
}
