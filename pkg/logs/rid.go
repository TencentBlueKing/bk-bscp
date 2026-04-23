package logs

import (
	"context"
	"strings"

	"google.golang.org/grpc/metadata"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
)

func injectRIDFormat(format string) string {
	return strings.TrimRight(format, " ") + ", rid: %s"
}

func ridInfoDepthf(depth int, rid, format string, args ...interface{}) {
	if rid == "" {
		InfoDepthf(depth, format, args...)
		return
	}
	args = append(args, rid)
	InfoDepthf(depth, injectRIDFormat(format), args...)
}

func ridErrorDepthf(depth int, rid, format string, args ...interface{}) {
	if rid == "" {
		ErrorDepthf(depth, format, args...)
		return
	}
	args = append(args, rid)
	ErrorDepthf(depth, injectRIDFormat(format), args...)
}

func ridFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	switch v := ctx.Value(constant.RidKey).(type) { //nolint:staticcheck
	case string:
		if v != "" {
			return v
		}
	case []string:
		if len(v) > 0 && v[0] != "" {
			return v[0]
		}
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	rids := md.Get(strings.ToLower(constant.RidKey))
	if len(rids) > 0 {
		return rids[0]
	}
	return ""
}

func RidInfof(rid, format string, args ...interface{}) {
	ridInfoDepthf(1, rid, format, args...)
}

func RidWarnf(rid, format string, args ...interface{}) {
	if rid == "" {
		Warnf(format, args...)
		return
	}
	args = append(args, rid)
	Warnf(injectRIDFormat(format), args...)
}

func RidErrorf(rid, format string, args ...interface{}) {
	ridErrorDepthf(1, rid, format, args...)
}

func CtxInfof(ctx context.Context, format string, args ...interface{}) {
	ridInfoDepthf(2, ridFromContext(ctx), format, args...)
}

func CtxWarnf(ctx context.Context, format string, args ...interface{}) {
	RidWarnf(ridFromContext(ctx), format, args...)
}

func CtxErrorf(ctx context.Context, format string, args ...interface{}) {
	ridErrorDepthf(2, ridFromContext(ctx), format, args...)
}
