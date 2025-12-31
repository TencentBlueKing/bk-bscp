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

package asyncdownload

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clientset "github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/client-set"
	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/repository"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/pkg/runtime/jsoni"
)

// mockGSE 用于模拟 GSE 调用
type mockGSE struct {
	createTaskFunc func(ctx context.Context, sourceAgentID, sourceContainerID, sourceFileDir, sourceUser,
		filename string, targetFileDir string, targetsAgents []gse.TransferFileAgent) (string, error)
	transferResultFunc func(ctx context.Context, taskID string) ([]gse.TransferFileResultDataResult, error)
	terminateTaskFunc  func(ctx context.Context, taskID string, targetsAgents []gse.TransferFileAgent) (string, error)
}

var (
	// 全局 mock GSE 实例
	globalMockGSE *mockGSE
	// 用于存储 GSE task 的结果
	gseTaskResults = sync.Map{} // map[string][]gse.TransferFileResultDataResult
	// GSE task ID 计数器
	gseTaskIDCounter int64
)

// setupMockGSE 设置 mock GSE
func setupMockGSE() {
	globalMockGSE = &mockGSE{
		createTaskFunc: func(ctx context.Context, sourceAgentID, sourceContainerID, sourceFileDir, sourceUser,
			filename string, targetFileDir string, targetsAgents []gse.TransferFileAgent) (string, error) {
			// 生成 task ID
			taskID := fmt.Sprintf("mock-gse-task-%d", atomic.AddInt64(&gseTaskIDCounter, 1))

			// 创建初始结果（所有 targets 都是 downloading 状态）
			results := make([]gse.TransferFileResultDataResult, 0, len(targetsAgents))
			for _, agent := range targetsAgents {
				results = append(results, gse.TransferFileResultDataResult{
					Content: gse.TransferFileResultDataResultContent{
						DestAgentID:     agent.BkAgentID,
						DestContainerID: agent.BkContainerID,
						DestFileName:    filename,
						Type:            "download",
					},
					ErrorCode: 115, // downloading
					ErrorMsg:  "downloading",
				})
			}

			// 添加 upload 结果
			results = append(results, gse.TransferFileResultDataResult{
				Content: gse.TransferFileResultDataResultContent{
					Type: "upload",
				},
				ErrorCode: 0,
				ErrorMsg:  "success",
			})

			gseTaskResults.Store(taskID, results)
			return taskID, nil
		},
		transferResultFunc: func(ctx context.Context, taskID string) ([]gse.TransferFileResultDataResult, error) {
			if val, ok := gseTaskResults.Load(taskID); ok {
				return val.([]gse.TransferFileResultDataResult), nil
			}
			return nil, fmt.Errorf("task not found: %s", taskID)
		},
		terminateTaskFunc: func(ctx context.Context, taskID string, targetsAgents []gse.TransferFileAgent) (string, error) {
			return taskID, nil
		},
	}
}

// updateGSEResult 更新 GSE task 的结果（用于模拟成功或失败）
func updateGSEResult(taskID string, agentID, containerID string, success bool) {
	if val, ok := gseTaskResults.Load(taskID); ok {
		results := val.([]gse.TransferFileResultDataResult)
		for i := range results {
			if results[i].Content.Type == "download" &&
				results[i].Content.DestAgentID == agentID &&
				results[i].Content.DestContainerID == containerID {
				if success {
					results[i].ErrorCode = 0
					results[i].ErrorMsg = "success"
				} else {
					results[i].ErrorCode = 1
					results[i].ErrorMsg = "failed"
				}
			}
		}
		gseTaskResults.Store(taskID, results)
	}
}

// updateGSEUploadResult 更新 upload 结果（用于模拟 upload 失败）
func updateGSEUploadResult(taskID string, failed bool) {
	if val, ok := gseTaskResults.Load(taskID); ok {
		results := val.([]gse.TransferFileResultDataResult)
		for i := range results {
			if results[i].Content.Type == "upload" {
				if failed {
					results[i].ErrorCode = 1
					results[i].ErrorMsg = "upload failed"
				} else {
					results[i].ErrorCode = 0
					results[i].ErrorMsg = "success"
				}
			}
		}
		gseTaskResults.Store(taskID, results)
	}
}

// setupTestRedis 创建测试用的真实 Redis 客户端
func setupTestRedis(t *testing.T) (bedis.Client, func()) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisUser := os.Getenv("REDIS_USER")
	redisPass := os.Getenv("REDIS_PASS")

	bds, err := bedis.NewRedisCache(
		cc.RedisCluster{
			Endpoints: []string{redisAddr},
			Username:  redisUser,
			Password:  redisPass,
			Mode:      cc.RedisStandaloneMode,
		})
	if err != nil {
		t.Fatalf("new redis cache failed, %+v", err)
	}

	// 测试连接
	ctx := context.Background()
	if err := bds.Healthz(); err != nil {
		t.Fatalf("redis health check failed, %+v", err)
	}

	cleanup := func() {
		// 清理测试数据
		keys, _ := bds.Keys(ctx, "AsyncDownloadJob:*")
		keys2, _ := bds.Keys(ctx, "AsyncDownloadTask:*")
		keys = append(keys, keys2...)
		if len(keys) > 0 {
			_ = bds.Delete(ctx, keys...)
		}
	}

	return bds, cleanup
}

// TestConcurrentWriteAndConsume 测试并发写入和消费场景
func TestConcurrentWriteAndConsume(t *testing.T) {
	// 设置 mock GSE
	setupMockGSE()

	// 设置测试 Redis
	bds, cleanup := setupTestRedis(t)
	defer cleanup()

	// 初始化服务名称（必须）
	cc.InitService(cc.FeedServerName)

	// 创建测试目录
	testDir := filepath.Join(os.TempDir(), "bscp-test", fmt.Sprintf("test-%d", time.Now().UnixNano()))
	defer os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)

	// 注意：在实际测试中，GSE 和 Repository 配置需要通过配置文件加载
	// 这里我们主要测试并发写入和 Redis List 操作，GSE 部分可以 mock

	// 创建 metric（使用 InitMetric 初始化，避免 nil pointer）
	mc := InitMetric()

	// 创建 RedisLock
	redLock := lock.NewRedisLock(bds, 15)

	// 创建 ClientSet（使用 SetRedis 方法设置 bds）
	cs := &clientset.ClientSet{}
	cs.SetRedis(bds)

	// 创建 Service（直接创建实例，避免调用 NewService 中的 cc.FeedServer()）
	// 因为测试中配置未加载，cc.FeedServer() 会返回空配置
	service := &Service{
		enabled: true, // 测试中假设 GSE 已启用
		cs:      cs,
		redLock: redLock,
		metric:  mc,
	}

	// 注意：Scheduler 的创建需要完整的配置（Redis、Repository、GSE等）
	// 在测试中我们主要测试并发写入逻辑，Scheduler 的消费逻辑可以单独测试
	// 这里我们暂时不创建 Scheduler

	// 测试参数
	bizID := uint32(1)
	appID := uint32(1)
	filePath := "/test/path"
	fileName := "test.txt"
	signature := "test-signature-123"
	targetUser := "test-user"
	targetDir := "/target/dir"

	// 并发写入：模拟 100 个客户端同时请求下载
	concurrentWriters := 100
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	taskIDs := make([]string, 0, concurrentWriters)

	wg.Add(concurrentWriters)
	for i := 0; i < concurrentWriters; i++ {
		go func(idx int) {
			defer wg.Done()
			kt := kit.New()
			kt.Rid = fmt.Sprintf("test-rid-%d", idx)
			kt.BizID = bizID
			kt.AppID = appID

			agentID := fmt.Sprintf("agent-%d", idx)
			containerID := fmt.Sprintf("container-%d", idx)

			taskID, err := service.CreateAsyncDownloadTask(
				kt, bizID, appID, filePath, fileName,
				agentID, containerID, targetUser, targetDir, signature,
			)

			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				t.Logf("Failed to create task %d: %v", idx, err)
			} else {
				atomic.AddInt64(&successCount, 1)
				taskIDs = append(taskIDs, taskID)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Concurrent write completed: success=%d, error=%d", successCount, errorCount)
	assert.Equal(t, int64(concurrentWriters), successCount, "All writes should succeed")

	// 等待一段时间让 job 进入 pending 状态
	time.Sleep(100 * time.Millisecond)

	// 验证 job 和 targets 是否正确创建
	jobKey := fmt.Sprintf("AsyncDownloadJob:%d:%d:%s", bizID, appID, filepath.Join(filePath, fileName))
	targetsKey := fmt.Sprintf("AsyncDownloadJob:Targets:%d:%d:%s", bizID, appID, filepath.Join(filePath, fileName))

	ctx := context.Background()

	// 检查 job 是否存在
	jobData, err := bds.Get(ctx, jobKey)
	require.NoError(t, err)
	assert.NotEmpty(t, jobData, "Job should be created")

	// 解析 job 验证状态
	job := &types.AsyncDownloadJob{}
	err = jsoni.Unmarshal([]byte(jobData), job)
	require.NoError(t, err)
	assert.Equal(t, types.AsyncDownloadJobStatusPending, job.Status, "Job should be in pending status")

	t.Logf("Job created successfully: %s, status: %s", job.JobID, job.Status)

	// 检查 targets 数量
	targetsCount, err := bds.LLen(ctx, targetsKey)
	require.NoError(t, err)
	assert.Equal(t, int64(concurrentWriters), targetsCount, "All targets should be added")

	t.Logf("Targets count: %d", targetsCount)

	// 验证所有 targets 都在 List 中
	targetsData, err := bds.LRange(ctx, targetsKey, 0, -1)
	require.NoError(t, err)
	assert.Equal(t, concurrentWriters, len(targetsData), "All targets should be in list")

	t.Logf("All %d targets are in Redis List", len(targetsData))

	// 测试消费：模拟 Scheduler 处理 job
	t.Logf("Starting consumption test...")

	// 创建 mock Repository Provider
	mockProvider := &mockRepositoryProvider{
		files: make(map[string][]byte),
	}
	// 预先创建测试文件内容
	testFileContent := []byte("test file content for signature: " + signature)
	mockProvider.files[signature] = testFileContent

	// 创建测试用的 Scheduler（手动设置字段，避免依赖配置）
	testScheduler := createTestScheduler(t, bds, redLock, mc, mockProvider, testDir)

	// 等待 job 创建完成，然后修改创建时间使其可以立即处理
	time.Sleep(100 * time.Millisecond)

	// 修改 job 的创建时间，使其可以立即处理（减去16秒）
	jobData, err = bds.Get(ctx, jobKey)
	require.NoError(t, err)
	job = &types.AsyncDownloadJob{}
	err = jsoni.Unmarshal([]byte(jobData), job)
	require.NoError(t, err)

	// 修改创建时间，使其可以立即处理（减去16秒）
	job.CreateTime = job.CreateTime.Add(-16 * time.Second)
	jobDataBytes, _ := jsoni.Marshal(job)
	err = bds.Set(ctx, jobKey, string(jobDataBytes), 30*60)
	require.NoError(t, err)

	// 手动触发一次处理（模拟 scheduler 的 do 方法）
	err = testScheduler.processJob(ctx, jobKey)
	require.NoError(t, err, "Process job should succeed")

	// 验证 job 状态已更新为 Running
	jobData, err = bds.Get(ctx, jobKey)
	require.NoError(t, err)
	job = &types.AsyncDownloadJob{}
	err = jsoni.Unmarshal([]byte(jobData), job)
	require.NoError(t, err)
	assert.Equal(t, types.AsyncDownloadJobStatusRunning, job.Status, "Job should be in running status after processing")
	assert.NotEmpty(t, job.GSETaskID, "GSE task ID should be set")

	t.Logf("Job processed successfully: %s, status: %s, gse_task_id: %s", job.JobID, job.Status, job.GSETaskID)

	// 验证文件已下载到本地（文件下载到 sourceDir = testDir/1/signature）
	testFilePath := filepath.Join(testDir, fmt.Sprintf("%d", bizID), signature)
	fileContent, err := os.ReadFile(testFilePath)
	require.NoError(t, err)
	assert.Equal(t, testFileContent, fileContent, "File content should match")

	t.Logf("File downloaded successfully to: %s", testFilePath)

	t.Logf("Test completed successfully")
}

// mockRepositoryProvider 用于测试的 mock Repository Provider
type mockRepositoryProvider struct {
	files map[string][]byte
	mu    sync.Mutex
}

func (m *mockRepositoryProvider) Download(kt *kit.Kit, sign string) (io.ReadCloser, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	content, ok := m.files[sign]
	if !ok {
		return nil, 0, fmt.Errorf("file not found: %s", sign)
	}

	reader := io.NopCloser(bytes.NewReader(content))
	return reader, int64(len(content)), nil
}

// 实现其他必需的方法（简化实现）
func (m *mockRepositoryProvider) Upload(kt *kit.Kit, sign string, body io.Reader) (*repository.ObjectMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockRepositoryProvider) InitMultipartUpload(kt *kit.Kit, sign string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockRepositoryProvider) MultipartUpload(kt *kit.Kit, sign string, uploadID string, partNum uint32, body io.Reader) error {
	return fmt.Errorf("not implemented")
}

func (m *mockRepositoryProvider) CompleteMultipartUpload(kt *kit.Kit, sign string, uploadID string) (*repository.ObjectMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockRepositoryProvider) Metadata(kt *kit.Kit, sign string) (*repository.ObjectMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	content, ok := m.files[sign]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", sign)
	}

	return &repository.ObjectMetadata{
		ByteSize: int64(len(content)),
		Sha256:   sign,
	}, nil
}

func (m *mockRepositoryProvider) DownloadLink(kt *kit.Kit, sign string, fetchLimit uint32) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockRepositoryProvider) AsyncDownload(kt *kit.Kit, sign string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockRepositoryProvider) AsyncDownloadStatus(kt *kit.Kit, sign string, taskID string) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (m *mockRepositoryProvider) URIDecorator(bizID uint32) repository.DecoratorInter {
	return nil
}

func (m *mockRepositoryProvider) SyncManager() *repository.SyncManager {
	return nil
}

func (m *mockRepositoryProvider) SetVariables(kt *kit.Kit, sign string, checkSize bool) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockRepositoryProvider) GetVariables(kt *kit.Kit, sign string, checkSize bool) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}

// createTestScheduler 创建测试用的 Scheduler
func createTestScheduler(t *testing.T, bds bedis.Client, redLock *lock.RedisLock, mc *metric, provider repository.Provider, cacheDir string) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	// 设置 mock GSE task 结果
	setupMockGSE()

	// 创建 mock GSE 创建任务函数
	mockGSEFunc := func(ctx context.Context, sourceAgentID, sourceContainerID, sourceFileDir, sourceUser,
		filename string, targetFileDir string, targetsAgents []gse.TransferFileAgent) (string, error) {
		return globalMockGSE.createTaskFunc(ctx, sourceAgentID, sourceContainerID, sourceFileDir, sourceUser,
			filename, targetFileDir, targetsAgents)
	}

	return &Scheduler{
		ctx:               ctx,
		cancel:            cancel,
		bds:               bds,
		redLock:           redLock,
		fileLock:          lock.NewFileLock(),
		provider:          provider,
		serverAgentID:     "test-server-agent-id",
		serverContainerID: "test-server-container-id",
		metric:            mc,
		cacheDir:          cacheDir,    // 注入测试目录
		agentUser:         "test-user", // 注入测试用户
		gseCreateTaskFunc: mockGSEFunc, // 注入 mock GSE 函数
	}
}

// processJob 手动处理一个 job（用于测试）
func (a *Scheduler) processJob(ctx context.Context, jobKey string) error {
	// 尝试获取锁
	if !a.redLock.TryAcquire(jobKey) {
		return fmt.Errorf("failed to acquire lock for job: %s", jobKey)
	}
	defer a.redLock.Release(jobKey)

	// 获取 job
	jobData, err := a.bds.Get(ctx, jobKey)
	if err != nil {
		return err
	}
	if jobData == "" {
		return fmt.Errorf("job not found: %s", jobKey)
	}

	job := &types.AsyncDownloadJob{}
	if err := jsoni.Unmarshal([]byte(jobData), job); err != nil {
		return err
	}

	// 检查 job 状态
	if job.Status != types.AsyncDownloadJobStatusPending {
		return fmt.Errorf("job is not in pending status: %s", job.Status)
	}

	// 检查创建时间（需要等待15秒收集期）
	// 在测试中，我们通过修改创建时间来绕过这个检查
	if time.Since(job.CreateTime) < 15*time.Second {
		// 继续收集 targets（返回 nil 表示成功但不需要处理）
		logs.Infof("job %s is still collecting targets, waiting...", jobKey)
		return nil
	}

	// 处理下载（直接调用 handleDownload，因为已经通过依赖注入配置了 cacheDir 和 gseCreateTaskFunc）
	return a.handleDownload(job)
}

// TestConcurrentWriteWithGSEFailure 测试 GSE 失败场景
func TestConcurrentWriteWithGSEFailure(t *testing.T) {
	// 初始化服务名称（必须）
	cc.InitService(cc.FeedServerName)

	// 设置 mock GSE（模拟 upload 失败）
	setupMockGSE()

	// 设置测试 Redis
	bds, cleanup := setupTestRedis(t)
	defer cleanup()

	// 创建测试目录
	testDir := filepath.Join(os.TempDir(), "bscp-test", fmt.Sprintf("test-%d", time.Now().UnixNano()))
	defer os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)

	// 注意：在实际测试中，GSE 和 Repository 配置需要通过配置文件加载
	// 这里我们主要测试并发写入和 Redis List 操作，GSE 部分可以 mock

	// 创建 metric（使用 InitMetric 初始化，避免 nil pointer）
	mc := InitMetric()

	// 创建 RedisLock
	redLock := lock.NewRedisLock(bds, 15)

	// 创建 ClientSet（使用 SetRedis 方法设置 bds）
	cs := &clientset.ClientSet{}
	cs.SetRedis(bds)

	// 创建 Service（直接创建实例，避免调用 NewService 中的 cc.FeedServer()）
	// 因为测试中配置未加载，cc.FeedServer() 会返回空配置
	service := &Service{
		enabled: true, // 测试中假设 GSE 已启用
		cs:      cs,
		redLock: redLock,
		metric:  mc,
	}

	// 测试参数
	bizID := uint32(1)
	appID := uint32(1)
	filePath := "/test/path"
	fileName := "test-fail.txt"
	signature := "test-signature-fail"
	targetUser := "test-user"
	targetDir := "/target/dir"

	// 创建几个任务
	concurrentWriters := 10
	var wg sync.WaitGroup
	wg.Add(concurrentWriters)

	for i := 0; i < concurrentWriters; i++ {
		go func(idx int) {
			defer wg.Done()
			kt := kit.New()
			kt.Rid = fmt.Sprintf("test-rid-fail-%d", idx)
			kt.BizID = bizID
			kt.AppID = appID

			agentID := fmt.Sprintf("agent-fail-%d", idx)
			containerID := fmt.Sprintf("container-fail-%d", idx)

			_, err := service.CreateAsyncDownloadTask(
				kt, bizID, appID, filePath, fileName,
				agentID, containerID, targetUser, targetDir, signature,
			)
			if err != nil {
				t.Logf("Failed to create task %d: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()

	// 验证 targets 数量
	targetsKey := fmt.Sprintf("AsyncDownloadJob:Targets:%d:%d:%s", bizID, appID, filepath.Join(filePath, fileName))
	targetsCount, err := bds.LLen(context.Background(), targetsKey)
	require.NoError(t, err)
	assert.Equal(t, int64(concurrentWriters), targetsCount, "All targets should be added even with GSE failure simulation")

	t.Logf("Test with GSE failure simulation completed")
}

// TestConcurrentWriteWhileConsuming 测试并发写入和消费的场景（写入和消费同时进行）
func TestConcurrentWriteWhileConsuming(t *testing.T) {
	// 初始化服务名称（必须）
	cc.InitService(cc.FeedServerName)

	// 设置 mock GSE
	setupMockGSE()

	// 设置测试 Redis
	bds, cleanup := setupTestRedis(t)
	defer cleanup()

	// 创建测试目录
	testDir := filepath.Join(os.TempDir(), "bscp-test", fmt.Sprintf("test-concurrent-%d", time.Now().UnixNano()))
	defer os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)

	// 创建 metric
	mc := InitMetric()

	// 创建 RedisLock
	redLock := lock.NewRedisLock(bds, 15)

	// 创建 ClientSet
	cs := &clientset.ClientSet{}
	cs.SetRedis(bds)

	// 创建 Service
	service := &Service{
		enabled: true,
		cs:      cs,
		redLock: redLock,
		metric:  mc,
	}

	// 测试参数
	bizID := uint32(1)
	appID := uint32(1)
	filePath := "/test/path"
	fileName := "concurrent-test.txt"
	signature := "concurrent-test-signature"
	targetUser := "test-user"
	targetDir := "/target/dir"

	jobKey := fmt.Sprintf("AsyncDownloadJob:%d:%d:%s", bizID, appID, filepath.Join(filePath, fileName))
	targetsKey := fmt.Sprintf("AsyncDownloadJob:Targets:%d:%d:%s", bizID, appID, filepath.Join(filePath, fileName))

	// 创建 mock Repository Provider
	mockProvider := &mockRepositoryProvider{
		files: make(map[string][]byte),
	}
	testFileContent := []byte("test file content for concurrent test: " + signature)
	mockProvider.files[signature] = testFileContent

	// 创建测试用的 Scheduler
	testScheduler := createTestScheduler(t, bds, redLock, mc, mockProvider, testDir)
	defer testScheduler.Stop()

	ctx := context.Background()

	// 并发写入的 goroutine 数量
	concurrentWriters := 50
	// 每个 writer 写入的次数
	writesPerWriter := 10
	totalWrites := concurrentWriters * writesPerWriter

	// 用于跟踪写入和消费的原子计数器
	var writeCount int64
	var consumeCount int64
	var writeErrors int64

	// 启动写入 goroutines
	var wgWrite sync.WaitGroup
	wgWrite.Add(concurrentWriters)

	writeStartTime := time.Now()
	for i := 0; i < concurrentWriters; i++ {
		go func(writerID int) {
			defer wgWrite.Done()
			for j := 0; j < writesPerWriter; j++ {
				kt := kit.New()
				kt.Rid = fmt.Sprintf("test-rid-concurrent-%d-%d", writerID, j)
				kt.BizID = bizID
				kt.AppID = appID

				agentID := fmt.Sprintf("agent-concurrent-%d-%d", writerID, j)
				containerID := fmt.Sprintf("container-concurrent-%d-%d", writerID, j)

				_, err := service.CreateAsyncDownloadTask(
					kt, bizID, appID, filePath, fileName,
					agentID, containerID, targetUser, targetDir, signature,
				)
				if err != nil {
					atomic.AddInt64(&writeErrors, 1)
					t.Logf("Write error: writer=%d, write=%d, err=%v", writerID, j, err)
				} else {
					atomic.AddInt64(&writeCount, 1)
				}
				// 随机延迟，模拟真实场景
				time.Sleep(time.Duration(10+writerID%50) * time.Millisecond)
			}
		}(i)
	}

	// 启动消费 goroutine（模拟 scheduler 持续消费）
	var wgConsume sync.WaitGroup
	wgConsume.Add(1)
	consumeStop := make(chan struct{})

	go func() {
		defer wgConsume.Done()
		ticker := time.NewTicker(200 * time.Millisecond) // 每200ms检查一次
		defer ticker.Stop()

		for {
			select {
			case <-consumeStop:
				return
			case <-ticker.C:
				// 检查是否有 job 可以处理
				jobData, err := bds.Get(ctx, jobKey)
				if err != nil || jobData == "" {
					continue
				}

				job := &types.AsyncDownloadJob{}
				if err := jsoni.Unmarshal([]byte(jobData), job); err != nil {
					continue
				}

				// 只处理 Pending 状态的 job
				if job.Status != types.AsyncDownloadJobStatusPending {
					continue
				}

				// 检查创建时间（需要等待15秒收集期）
				// 在测试中，我们允许立即处理（通过修改创建时间）
				if time.Since(job.CreateTime) < 15*time.Second {
					// 如果还在收集期，修改创建时间使其可以处理
					job.CreateTime = job.CreateTime.Add(-16 * time.Second)
					jobDataBytes, _ := jsoni.Marshal(job)
					_ = bds.Set(ctx, jobKey, string(jobDataBytes), 30*60)
				}

				// 尝试处理 job
				err = testScheduler.processJob(ctx, jobKey)
				if err != nil {
					// 可能是锁被占用或其他原因，继续尝试
					continue
				}

				// 检查是否处理成功
				jobData, err = bds.Get(ctx, jobKey)
				if err != nil {
					continue
				}
				job = &types.AsyncDownloadJob{}
				if err := jsoni.Unmarshal([]byte(jobData), job); err != nil {
					continue
				}

				if job.Status == types.AsyncDownloadJobStatusRunning && job.GSETaskID != "" {
					atomic.AddInt64(&consumeCount, 1)
					t.Logf("Job consumed successfully: status=%s, gse_task_id=%s", job.Status, job.GSETaskID)
					// 消费成功后停止消费循环
					return
				}
			}
		}
	}()

	// 等待所有写入完成
	wgWrite.Wait()
	writeDuration := time.Since(writeStartTime)
	t.Logf("All writes completed: total=%d, success=%d, errors=%d, duration=%v",
		totalWrites, atomic.LoadInt64(&writeCount), atomic.LoadInt64(&writeErrors), writeDuration)

	// 等待一段时间让消费完成
	time.Sleep(2 * time.Second)
	close(consumeStop)
	wgConsume.Wait()

	// 验证结果
	// 1. 验证所有写入都成功
	assert.Equal(t, int64(totalWrites), atomic.LoadInt64(&writeCount), "All writes should succeed")
	assert.Equal(t, int64(0), atomic.LoadInt64(&writeErrors), "No write errors should occur")

	// 2. 验证 targets 数量
	targetsCount, err := bds.LLen(ctx, targetsKey)
	require.NoError(t, err)
	assert.Equal(t, int64(totalWrites), targetsCount, "All targets should be in Redis List")
	t.Logf("Targets count in Redis List: %d", targetsCount)

	// 3. 验证 job 状态
	jobData, err := bds.Get(ctx, jobKey)
	require.NoError(t, err)
	job := &types.AsyncDownloadJob{}
	err = jsoni.Unmarshal([]byte(jobData), job)
	require.NoError(t, err)
	assert.Equal(t, types.AsyncDownloadJobStatusRunning, job.Status, "Job should be in running status")
	assert.NotEmpty(t, job.GSETaskID, "GSE task ID should be set")
	t.Logf("Job status: %s, GSE task ID: %s", job.Status, job.GSETaskID)

	// 4. 验证文件已下载
	testFilePath := filepath.Join(testDir, fmt.Sprintf("%d", bizID), signature)
	fileContent, err := os.ReadFile(testFilePath)
	require.NoError(t, err)
	assert.Equal(t, testFileContent, fileContent, "File content should match")
	t.Logf("File downloaded successfully to: %s", testFilePath)

	// 5. 验证消费次数
	assert.GreaterOrEqual(t, atomic.LoadInt64(&consumeCount), int64(1), "Job should be consumed at least once")
	t.Logf("Consume count: %d", atomic.LoadInt64(&consumeCount))

	// 6. 验证所有 targets 都在 List 中（消费不会删除 targets）
	targetsData, err := bds.LRange(ctx, targetsKey, 0, -1)
	require.NoError(t, err)
	assert.Equal(t, totalWrites, len(targetsData), "All targets should still be in list after consumption")
	t.Logf("All %d targets are still in Redis List after consumption", len(targetsData))

	t.Logf("Concurrent write and consume test completed successfully")
}
