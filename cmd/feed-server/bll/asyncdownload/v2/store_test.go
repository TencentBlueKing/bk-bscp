package v2

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

func TestStoreCreateBatchAndInflight(t *testing.T) {
	mr := miniredis.RunT(t)
	opt := cc.RedisCluster{Mode: cc.RedisStandaloneMode, Endpoints: []string{mr.Addr()}}
	bds, err := bedis.NewRedisCache(opt)
	require.NoError(t, err)

	store := NewStore(bds, cc.AsyncDownloadV2{
		TaskTTLSeconds:  86400,
		BatchTTLSeconds: 86400,
	})

	ctx := context.Background()
	key := BuildFileVersionKey(706, 192, "/path", "protocol.tar.gz", "sha256")
	targetID := "agent:container"
	taskID := "task-1"
	batchID := "batch-1"

	err = store.CreateBatchAndTask(ctx, key, batchID, targetID, taskID, &types.AsyncDownloadV2Batch{
		BatchID: batchID,
		State:   types.AsyncDownloadBatchStateCollecting,
	}, &types.AsyncDownloadV2Task{TaskID: taskID, BatchID: batchID, TargetID: targetID})
	require.NoError(t, err)

	gotTaskID, err := store.GetInflightTaskID(ctx, key, BuildInflightTargetKey(targetID, "", ""))
	require.NoError(t, err)
	require.Equal(t, taskID, gotTaskID)

	targets, err := store.ListBatchTargets(ctx, batchID)
	require.NoError(t, err)
	require.Equal(t, []string{targetID}, targets)
}
