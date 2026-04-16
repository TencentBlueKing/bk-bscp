package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

func TestLogUnaryServerInterceptorPropagatesGeneratedRID(t *testing.T) {
	interceptor := LogUnaryServerInterceptor()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})

	var ridInHandler string
	_, err := interceptor(ctx, struct{}{}, &grpc.UnaryServerInfo{
		FullMethod: "/pbfs.Upstream/AsyncDownloadStatus",
	}, func(ctx context.Context, req interface{}) (interface{}, error) {
		kt := kit.FromGrpcContext(ctx)
		ridInHandler = kt.Rid
		require.Equal(t, ridInHandler, ctx.Value(constant.RidKey))
		return struct{}{}, nil
	})

	require.NoError(t, err)
	require.NotEmpty(t, ridInHandler)
}
