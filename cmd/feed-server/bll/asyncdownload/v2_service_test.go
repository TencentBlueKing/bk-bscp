package asyncdownload

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

func TestAsyncDownloadV2ConfigDefaults(t *testing.T) {
	g := cc.GSE{}
	g.TrySetDefaultForTest()

	require.False(t, g.AsyncDownloadV2.Enabled)
	require.Equal(t, 10, g.AsyncDownloadV2.CollectWindowSeconds)
	require.Equal(t, 5000, g.AsyncDownloadV2.MaxTargetsPerBatch)
	require.Equal(t, 500, g.AsyncDownloadV2.ShardSize)
	require.Equal(t, 15, g.AsyncDownloadV2.DispatchHeartbeatSeconds)
	require.Equal(t, 60, g.AsyncDownloadV2.DispatchLeaseSeconds)
	require.Equal(t, 3, g.AsyncDownloadV2.MaxDispatchAttempts)
	require.Equal(t, 100, g.AsyncDownloadV2.MaxDueBatchesPerTick)
	require.Equal(t, 86400, g.AsyncDownloadV2.TaskTTLSeconds)
	require.Equal(t, 86400, g.AsyncDownloadV2.BatchTTLSeconds)
}
