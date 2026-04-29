package cmdbGse

import (
	"context"
	"errors"
	"testing"

	processorcmdb "github.com/TencentBlueKing/bk-bscp/internal/processor/cmdb"
)

type recordingRenderCache struct {
	processorcmdb.CMDBRenderCache
	tenantID string
	bizID    int
	err      error
	calls    int
}

func (c *recordingRenderCache) InvalidateBiz(_ context.Context, tenantID string, bizID int) error {
	c.calls++
	c.tenantID = tenantID
	c.bizID = bizID
	return c.err
}

func TestSyncCmdbGseExecutorInvalidateRenderCache(t *testing.T) {
	cache := &recordingRenderCache{}
	executor := &syncCmdbGseExecutor{renderCache: cache}

	executor.invalidateRenderCache(context.Background(), "tenant-a", 42)

	if cache.calls != 1 {
		t.Fatalf("InvalidateBiz calls = %d, want 1", cache.calls)
	}
	if cache.tenantID != "tenant-a" || cache.bizID != 42 {
		t.Fatalf("InvalidateBiz args = (%q, %d), want (tenant-a, 42)", cache.tenantID, cache.bizID)
	}
}

func TestSyncCmdbGseExecutorInvalidateRenderCacheIgnoresError(t *testing.T) {
	cache := &recordingRenderCache{err: errors.New("redis unavailable")}
	executor := &syncCmdbGseExecutor{renderCache: cache}

	executor.invalidateRenderCache(context.Background(), "tenant-a", 42)

	if cache.calls != 1 {
		t.Fatalf("InvalidateBiz calls = %d, want 1", cache.calls)
	}
}
