package logs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
)

func TestInjectRIDFormat(t *testing.T) {
	require.Equal(t, "hello, rid: %s", injectRIDFormat("hello"))
	require.Equal(t, "hello., rid: %s", injectRIDFormat("hello."))
}

func TestRidFromContext(t *testing.T) {
	require.Equal(t, "", ridFromContext(context.Background()))

	ctxWithString := context.WithValue(context.Background(), constant.RidKey, "rid-1") //nolint:staticcheck
	require.Equal(t, "rid-1", ridFromContext(ctxWithString))

	ctxWithSlice := context.WithValue(context.Background(), constant.RidKey, []string{"rid-2"}) //nolint:staticcheck
	require.Equal(t, "rid-2", ridFromContext(ctxWithSlice))

	mdCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(constant.RidKey, "rid-3"))
	require.Equal(t, "rid-3", ridFromContext(mdCtx))
}
