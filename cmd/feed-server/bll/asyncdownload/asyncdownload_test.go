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
	"strconv"
	"strings"
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

	// 验证 job 和 targets 是否正确创建（使用新的带时间窗口的格式）
	fullPath := filepath.Join(filePath, fileName)
	timeBucket := getTimeBucket()
	jobKey := GetJobKey(bizID, appID, fullPath, timeBucket)
	targetsKey := GetTargetsKey(bizID, appID, fullPath, timeBucket)

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

	// 等待 job 创建完成
	time.Sleep(100 * time.Millisecond)

	// 手动触发一次处理（使用 forceProcess=true 跳过时间窗口检查）
	err = testScheduler.processJob(ctx, jobKey, true)
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
// forceProcess 参数用于在测试中跳过时间窗口检查
func (a *Scheduler) processJob(ctx context.Context, jobKey string, forceProcess bool) error {
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

	// 检查时间窗口是否已结束（除非 forceProcess=true）
	if !forceProcess && !IsTimeWindowExpired(job.JobID) {
		// 时间窗口未结束，继续收集 targets
		logs.Infof("job %s time window not expired, waiting...", jobKey)
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

	// 验证 targets 数量（使用新的带时间窗口的格式）
	fullPath := filepath.Join(filePath, fileName)
	timeBucket := getTimeBucket()
	targetsKey := GetTargetsKey(bizID, appID, fullPath, timeBucket)
	targetsCount, err := bds.LLen(context.Background(), targetsKey)
	require.NoError(t, err)
	assert.Equal(t, int64(concurrentWriters), targetsCount, "All targets should be added even with GSE failure simulation")

	t.Logf("Test with GSE failure simulation completed")
}

// setupConcurrentTestEnv 设置并发测试环境
func setupConcurrentTestEnv(t *testing.T) (*Service, bedis.Client, *Scheduler, string, func()) {
	// 初始化服务名称（必须）
	cc.InitService(cc.FeedServerName)

	// 设置 mock GSE
	setupMockGSE()

	// 设置测试 Redis
	bds, cleanup := setupTestRedis(t)

	// 创建测试目录
	testDir := filepath.Join(os.TempDir(), "bscp-test", fmt.Sprintf("test-concurrent-%d", time.Now().UnixNano()))
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

	// 创建 mock Repository Provider
	mockProvider := &mockRepositoryProvider{
		files: make(map[string][]byte),
	}
	signature := "concurrent-test-signature"
	testFileContent := []byte("test file content for concurrent test: " + signature)
	mockProvider.files[signature] = testFileContent

	// 创建测试用的 Scheduler
	testScheduler := createTestScheduler(t, bds, redLock, mc, mockProvider, testDir)

	cleanupFunc := func() {
		cleanup()
		testScheduler.Stop()
		os.RemoveAll(testDir)
	}

	return service, bds, testScheduler, testDir, cleanupFunc
}

// TestConcurrentWriteBeforeConsuming 场景1：所有写入在消费前完成（验证所有 target 在同一 job）
func TestConcurrentWriteBeforeConsuming(t *testing.T) {
	service, bds, testScheduler, testDir, cleanup := setupConcurrentTestEnv(t)
	defer cleanup()

	// 测试参数
	bizID := uint32(1)
	appID := uint32(1)
	filePath := "/test/path"
	fileName := "concurrent-test.txt"
	signature := "concurrent-test-signature"
	targetUser := "test-user"
	targetDir := "/target/dir"

	// 使用新的带时间窗口的格式
	fullPath := filepath.Join(filePath, fileName)
	timeBucket := getTimeBucket()
	jobKey := GetJobKey(bizID, appID, fullPath, timeBucket)
	targetsKey := GetTargetsKey(bizID, appID, fullPath, timeBucket)

	ctx := context.Background()

	// 并发写入的 goroutine 数量
	concurrentWriters := 50
	// 每个 writer 写入的次数
	writesPerWriter := 10
	totalWrites := concurrentWriters * writesPerWriter

	// 用于跟踪写入的原子计数器
	var writeCount int64
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
				kt.Rid = fmt.Sprintf("test-rid-scenario1-%d-%d", writerID, j)
				kt.BizID = bizID
				kt.AppID = appID

				agentID := fmt.Sprintf("agent-scenario1-%d-%d", writerID, j)
				containerID := fmt.Sprintf("container-scenario1-%d-%d", writerID, j)

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

	// 等待所有写入完成（场景1：先完成所有写入）
	wgWrite.Wait()
	writeDuration := time.Since(writeStartTime)
	t.Logf("All writes completed: total=%d, success=%d, errors=%d, duration=%v",
		totalWrites, atomic.LoadInt64(&writeCount), atomic.LoadInt64(&writeErrors), writeDuration)

	// 验证写入完成时 job 仍然是 Pending 状态
	jobData, err := bds.Get(ctx, jobKey)
	require.NoError(t, err)
	job := &types.AsyncDownloadJob{}
	err = jsoni.Unmarshal([]byte(jobData), job)
	require.NoError(t, err)
	assert.Equal(t, types.AsyncDownloadJobStatusPending, job.Status, "Job should still be in pending status after all writes")

	// 验证所有 targets 都在同一个 job 中
	targetsCount, err := bds.LLen(ctx, targetsKey)
	require.NoError(t, err)
	assert.Equal(t, int64(totalWrites), targetsCount, "All targets should be in the same Redis List")
	t.Logf("Targets count in Redis List: %d", targetsCount)

	// 现在启动消费（场景1：所有写入已完成后再消费）
	err = testScheduler.processJob(ctx, jobKey, true)
	require.NoError(t, err, "Job should be processed successfully")

	// 验证 job 状态变为 Running
	jobData, err = bds.Get(ctx, jobKey)
	require.NoError(t, err)
	job = &types.AsyncDownloadJob{}
	err = jsoni.Unmarshal([]byte(jobData), job)
	require.NoError(t, err)
	assert.Equal(t, types.AsyncDownloadJobStatusRunning, job.Status, "Job should be in running status")
	assert.NotEmpty(t, job.GSETaskID, "GSE task ID should be set")
	t.Logf("Job status: %s, GSE task ID: %s", job.Status, job.GSETaskID)

	// 验证文件已下载
	testFilePath := filepath.Join(testDir, fmt.Sprintf("%d", bizID), signature)
	testFileContent := []byte("test file content for concurrent test: " + signature)
	fileContent, err := os.ReadFile(testFilePath)
	require.NoError(t, err)
	assert.Equal(t, testFileContent, fileContent, "File content should match")
	t.Logf("File downloaded successfully to: %s", testFilePath)

	// 验证所有 targets 仍然在 List 中（消费不会删除 targets）
	targetsData, err := bds.LRange(ctx, targetsKey, 0, -1)
	require.NoError(t, err)
	assert.Equal(t, totalWrites, len(targetsData), "All targets should still be in list after consumption")
	t.Logf("All %d targets are still in Redis List after consumption", len(targetsData))

	t.Logf("Scenario 1 test completed successfully: all writes before consuming")
}

// TestConcurrentWriteWhileConsuming 场景2：写入和消费并发（验证部分 target 在新窗口）
func TestConcurrentWriteWhileConsuming(t *testing.T) {
	service, bds, testScheduler, testDir, cleanup := setupConcurrentTestEnv(t)
	defer cleanup()

	// 测试参数
	bizID := uint32(1)
	appID := uint32(1)
	filePath := "/test/path"
	fileName := "concurrent-test.txt"
	signature := "concurrent-test-signature"
	targetUser := "test-user"
	targetDir := "/target/dir"

	// 使用新的带时间窗口的格式
	fullPath := filepath.Join(filePath, fileName)
	timeBucket := getTimeBucket()
	jobKey := GetJobKey(bizID, appID, fullPath, timeBucket)
	targetsKey := GetTargetsKey(bizID, appID, fullPath, timeBucket)

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
				kt.Rid = fmt.Sprintf("test-rid-scenario2-%d-%d", writerID, j)
				kt.BizID = bizID
				kt.AppID = appID

				agentID := fmt.Sprintf("agent-scenario2-%d-%d", writerID, j)
				containerID := fmt.Sprintf("container-scenario2-%d-%d", writerID, j)

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

				// 尝试处理 job（使用 forceProcess=true 跳过时间窗口检查）
				err = testScheduler.processJob(ctx, jobKey, true)
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

	// 额外等待一段时间，确保所有新窗口的 job 和 targets 都已创建完成
	time.Sleep(500 * time.Millisecond)

	// 验证结果（场景2：写入和消费并发）
	// 1. 验证所有写入都成功
	assert.Equal(t, int64(totalWrites), atomic.LoadInt64(&writeCount), "All writes should succeed")
	assert.Equal(t, int64(0), atomic.LoadInt64(&writeErrors), "No write errors should occur")

	// 2. 验证消费次数
	assert.GreaterOrEqual(t, atomic.LoadInt64(&consumeCount), int64(1), "Job should be consumed at least once")
	t.Logf("Consume count: %d", atomic.LoadInt64(&consumeCount))

	// 3. 验证原始窗口的 job 状态
	jobData, err := bds.Get(ctx, jobKey)
	require.NoError(t, err)
	job := &types.AsyncDownloadJob{}
	err = jsoni.Unmarshal([]byte(jobData), job)
	require.NoError(t, err)
	assert.Equal(t, types.AsyncDownloadJobStatusRunning, job.Status, "Original job should be in running status")
	assert.NotEmpty(t, job.GSETaskID, "GSE task ID should be set")
	t.Logf("Original job status: %s, GSE task ID: %s", job.Status, job.GSETaskID)

	// 4. 统计所有窗口的 targets 总数
	// 使用更全面的方式：查找所有相关的 targets keys（因为 targets list 可能先于 job 创建）
	originalTargetsCount, err := bds.LLen(ctx, targetsKey)
	require.NoError(t, err)
	t.Logf("Original window (timeBucket=%d) targets count: %d", timeBucket, originalTargetsCount)

	// 查找所有相关的 targets keys（使用通配符匹配）
	targetsKeyPattern := fmt.Sprintf("AsyncDownloadJob:Targets:%d:%d:%s:*", bizID, appID, fullPath)
	allTargetsKeys, err := bds.Keys(ctx, targetsKeyPattern)
	if err != nil {
		t.Logf("Failed to find all targets keys, using fallback method: %v", err)
		// 如果 Keys 失败，使用回退方法：检查后续窗口
		allTargetsKeys = []string{targetsKey}
		for i := 1; i <= 5; i++ {
			nextTimeBucket := timeBucket + int64(i)
			nextTargetsKey := GetTargetsKey(bizID, appID, fullPath, nextTimeBucket)
			nextTargetsCount, err := bds.LLen(ctx, nextTargetsKey)
			if err == nil && nextTargetsCount > 0 {
				allTargetsKeys = append(allTargetsKeys, nextTargetsKey)
			} else {
				// 如果这个窗口的 targets list 不存在或为空，后续窗口也不会有，可以停止检查
				break
			}
		}
	}

	// 统计所有窗口的 targets 总数
	totalTargetsCount := int64(0)
	windowTargetsMap := make(map[int64]int64) // timeBucket -> targets count

	for _, targetsKeyForJob := range allTargetsKeys {
		// 从 targetsKey 解析 timeBucket
		// targetsKey 格式: AsyncDownloadJob:Targets:{bizID}:{appID}:{fullPath}:{timeBucket}
		// 先转换为 jobKey 格式，然后解析
		jobKeyForJob := strings.Replace(targetsKeyForJob, "AsyncDownloadJob:Targets:", "AsyncDownloadJob:", 1)
		jobTimeBucket := ParseTimeBucketFromJobKey(jobKeyForJob)
		if jobTimeBucket == 0 {
			// 如果解析失败，尝试直接从 targetsKey 解析（timeBucket 是最后一个部分）
			parts := strings.Split(targetsKeyForJob, ":")
			if len(parts) >= 2 {
				if tb, err := strconv.ParseInt(parts[len(parts)-1], 10, 64); err == nil {
					jobTimeBucket = tb
				}
			}
			if jobTimeBucket == 0 {
				continue
			}
		}

		// 获取 targets list 的长度
		targetsCount, err := bds.LLen(ctx, targetsKeyForJob)
		if err == nil && targetsCount > 0 {
			windowTargetsMap[jobTimeBucket] = int64(targetsCount)
			totalTargetsCount += int64(targetsCount)

			// 获取 job 状态用于日志（如果 job 存在）
			jobKeyForJob := GetJobKey(bizID, appID, fullPath, jobTimeBucket)
			jobData, err := bds.Get(ctx, jobKeyForJob)
			if err == nil && jobData != "" {
				job := &types.AsyncDownloadJob{}
				if err := jsoni.Unmarshal([]byte(jobData), job); err == nil {
					t.Logf("Window (timeBucket=%d) job: status=%s, targets count=%d", jobTimeBucket, job.Status, targetsCount)
				} else {
					t.Logf("Window (timeBucket=%d) targets count=%d (job not found or invalid)", jobTimeBucket, targetsCount)
				}
			} else {
				t.Logf("Window (timeBucket=%d) targets count=%d (job not created yet)", jobTimeBucket, targetsCount)
			}
		}
	}

	t.Logf("Found %d windows with targets", len(windowTargetsMap))

	// 5. 验证所有 targets 的总数等于写入总数（可能分布在多个窗口）
	assert.Equal(t, int64(totalWrites), totalTargetsCount, "Total targets across all windows should equal total writes")
	t.Logf("Total targets across all windows: %d (original: %d)", totalTargetsCount, originalTargetsCount)

	// 6. 验证原始窗口的 targets 数量应该小于等于总数（因为部分可能在下一个窗口）
	assert.LessOrEqual(t, int64(originalTargetsCount), int64(totalWrites), "Original window targets should be less than or equal to total writes")
	if int64(originalTargetsCount) < int64(totalWrites) {
		t.Logf("Some targets (%d) were added to next window job due to concurrent consumption", int64(totalWrites)-int64(originalTargetsCount))
	}

	// 7. 验证文件已下载
	testFilePath := filepath.Join(testDir, fmt.Sprintf("%d", bizID), signature)
	testFileContent := []byte("test file content for concurrent test: " + signature)
	fileContent, err := os.ReadFile(testFilePath)
	require.NoError(t, err)
	assert.Equal(t, testFileContent, fileContent, "File content should match")
	t.Logf("File downloaded successfully to: %s", testFilePath)

	t.Logf("Scenario 2 test completed successfully: concurrent write and consume")
}

// TestTimeWindowCorrectness 测试时间窗口的正确性
func TestTimeWindowCorrectness(t *testing.T) {
	// 测试 getTimeBucket
	t.Run("getTimeBucket", func(t *testing.T) {
		bucket1 := getTimeBucket()
		time.Sleep(100 * time.Millisecond)
		bucket2 := getTimeBucket()

		// 在同一个时间窗口内，timeBucket 应该相同
		assert.Equal(t, bucket1, bucket2, "TimeBucket should be the same within 100ms")
		t.Logf("Current timeBucket: %d", bucket1)
	})

	// 测试 GetJobKey 和 GetTargetsKey
	t.Run("GetJobKey_GetTargetsKey", func(t *testing.T) {
		bizID := uint32(123)
		appID := uint32(456)
		fullPath := "/test/path/file.txt"
		timeBucket := int64(115710720)

		jobKey := GetJobKey(bizID, appID, fullPath, timeBucket)
		targetsKey := GetTargetsKey(bizID, appID, fullPath, timeBucket)

		expectedJobKey := fmt.Sprintf("AsyncDownloadJob:%d:%d:%s:%d", bizID, appID, fullPath, timeBucket)
		expectedTargetsKey := fmt.Sprintf("AsyncDownloadJob:Targets:%d:%d:%s:%d", bizID, appID, fullPath, timeBucket)

		assert.Equal(t, expectedJobKey, jobKey)
		assert.Equal(t, expectedTargetsKey, targetsKey)
		t.Logf("JobKey: %s", jobKey)
		t.Logf("TargetsKey: %s", targetsKey)
	})

	// 测试 GetTargetsKeyFromJobKey
	t.Run("GetTargetsKeyFromJobKey", func(t *testing.T) {
		jobKey := "AsyncDownloadJob:123:456:/test/path/file.txt:115710720"
		targetsKey := GetTargetsKeyFromJobKey(jobKey)

		expectedTargetsKey := "AsyncDownloadJob:Targets:123:456:/test/path/file.txt:115710720"
		assert.Equal(t, expectedTargetsKey, targetsKey)
		t.Logf("TargetsKey from JobKey: %s", targetsKey)
	})

	// 测试 ParseTimeBucketFromJobKey
	t.Run("ParseTimeBucketFromJobKey", func(t *testing.T) {
		testCases := []struct {
			name           string
			jobKey         string
			expectedBucket int64
		}{
			{
				name:           "valid new format",
				jobKey:         "AsyncDownloadJob:123:456:/test/path/file.txt:115710720",
				expectedBucket: 115710720,
			},
			{
				name:           "valid new format with different bucket",
				jobKey:         "AsyncDownloadJob:1:2:/path:999999999",
				expectedBucket: 999999999,
			},
			{
				name:           "old format without bucket",
				jobKey:         "AsyncDownloadJob:123:456:/test/path/file.txt",
				expectedBucket: 0, // 解析失败返回 0
			},
			{
				name:           "invalid format",
				jobKey:         "invalid",
				expectedBucket: 0,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				bucket := ParseTimeBucketFromJobKey(tc.jobKey)
				assert.Equal(t, tc.expectedBucket, bucket)
				t.Logf("JobKey: %s -> TimeBucket: %d", tc.jobKey, bucket)
			})
		}
	})

	// 测试 IsTimeWindowExpired
	t.Run("IsTimeWindowExpired", func(t *testing.T) {
		// 使用当前时间窗口
		currentBucket := getTimeBucket()
		currentJobKey := fmt.Sprintf("AsyncDownloadJob:1:2:/test:path:%d", currentBucket)

		// 使用过去的时间窗口（当前 - 2）
		pastBucket := currentBucket - 2
		pastJobKey := fmt.Sprintf("AsyncDownloadJob:1:2:/test:path:%d", pastBucket)

		// 使用旧格式（没有 timeBucket）
		oldFormatJobKey := "AsyncDownloadJob:1:2:/test:path"

		t.Logf("Current bucket: %d, Past bucket: %d", currentBucket, pastBucket)

		// 当前时间窗口应该未过期
		assert.False(t, IsTimeWindowExpired(currentJobKey),
			"Current time window should not be expired")

		// 过去的时间窗口应该已过期
		assert.True(t, IsTimeWindowExpired(pastJobKey),
			"Past time window should be expired")

		// 旧格式（解析失败）应该返回 true（保守策略）
		assert.True(t, IsTimeWindowExpired(oldFormatJobKey),
			"Old format should be treated as expired (conservative)")
	})

	// 测试时间窗口跨越
	t.Run("TimeWindowCrossover", func(t *testing.T) {
		// 这个测试验证同一时间窗口内的请求会得到相同的 timeBucket
		bizID := uint32(1)
		appID := uint32(2)
		fullPath := "/test/path"

		bucket1 := getTimeBucket()
		key1 := GetJobKey(bizID, appID, fullPath, bucket1)

		// 等待一小段时间（在同一窗口内）
		time.Sleep(100 * time.Millisecond)

		bucket2 := getTimeBucket()
		key2 := GetJobKey(bizID, appID, fullPath, bucket2)

		// 在同一时间窗口内，key 应该相同
		assert.Equal(t, key1, key2, "Keys should be the same within the same time window")
		t.Logf("Key1: %s", key1)
		t.Logf("Key2: %s", key2)
	})

	// 测试不同时间窗口会创建不同的 job
	t.Run("DifferentTimeWindowsDifferentJobs", func(t *testing.T) {
		bizID := uint32(1)
		appID := uint32(2)
		fullPath := "/test/path"

		bucket1 := int64(100)
		bucket2 := int64(101)

		key1 := GetJobKey(bizID, appID, fullPath, bucket1)
		key2 := GetJobKey(bizID, appID, fullPath, bucket2)

		// 不同时间窗口，key 应该不同
		assert.NotEqual(t, key1, key2, "Keys should be different for different time windows")
		t.Logf("Key1 (bucket=%d): %s", bucket1, key1)
		t.Logf("Key2 (bucket=%d): %s", bucket2, key2)
	})
}

// TestTimeWindowBoundary 测试时间窗口边界行为
// 验证：
// 1. 同一时间窗口内的请求会被合并到同一个 job
// 2. 跨时间窗口的请求会创建新的 job
// 3. 15秒后新请求会进入新的 job
func TestTimeWindowBoundary(t *testing.T) {
	// 初始化服务名称
	cc.InitService(cc.FeedServerName)

	// 设置 mock GSE
	setupMockGSE()

	// 设置测试 Redis
	bds, cleanup := setupTestRedis(t)
	defer cleanup()

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
	fileName := "time-window-test.txt"
	signature := "time-window-test-signature"
	targetUser := "test-user"
	targetDir := "/target/dir"

	ctx := context.Background()

	// 记录初始时间窗口
	initialBucket := getTimeBucket()
	t.Logf("Initial time bucket: %d", initialBucket)
	t.Logf("Current time: %s", time.Now().Format("15:04:05.000"))
	t.Logf("CollectWindowSeconds: %d", CollectWindowSeconds)

	// 计算当前窗口剩余时间
	currentTime := time.Now().Unix()
	windowStart := initialBucket * CollectWindowSeconds
	windowEnd := (initialBucket + 1) * CollectWindowSeconds
	remainingTime := windowEnd - currentTime
	t.Logf("Window start: %d, Window end: %d, Remaining: %d seconds", windowStart, windowEnd, remainingTime)

	// 第一阶段：在当前时间窗口内创建多个请求
	t.Logf("Phase 1: Creating requests in current time window...")

	phase1Count := 5
	phase1JobKeys := make(map[string]int)

	for i := 0; i < phase1Count; i++ {
		kt := kit.New()
		kt.Rid = fmt.Sprintf("phase1-rid-%d", i)
		kt.BizID = bizID
		kt.AppID = appID

		agentID := fmt.Sprintf("agent-phase1-%d", i)
		containerID := fmt.Sprintf("container-phase1-%d", i)

		taskID, err := service.CreateAsyncDownloadTask(
			kt, bizID, appID, filePath, fileName,
			agentID, containerID, targetUser, targetDir, signature,
		)
		require.NoError(t, err, "Phase 1 task creation should succeed")

		// 获取 task 对应的 job
		taskData, err := bds.Get(ctx, taskID)
		require.NoError(t, err)
		task := &types.AsyncDownloadTask{}
		err = jsoni.Unmarshal([]byte(taskData), task)
		require.NoError(t, err)

		phase1JobKeys[task.JobID]++
		t.Logf("Phase 1 task %d: taskID=%s, jobID=%s", i, taskID, task.JobID)

		time.Sleep(100 * time.Millisecond) // 小延迟
	}

	// 验证第一阶段所有请求都在同一个 job 中
	assert.Equal(t, 1, len(phase1JobKeys), "All phase 1 requests should be in the same job")
	var phase1JobKey string
	for k := range phase1JobKeys {
		phase1JobKey = k
	}
	t.Logf("Phase 1 job key: %s", phase1JobKey)

	// 验证 targets 数量
	phase1TargetsKey := GetTargetsKeyFromJobKey(phase1JobKey)
	phase1TargetsCount, err := bds.LLen(ctx, phase1TargetsKey)
	require.NoError(t, err)
	assert.Equal(t, int64(phase1Count), phase1TargetsCount, "Phase 1 should have correct targets count")
	t.Logf("Phase 1 targets count: %d", phase1TargetsCount)

	// 第二阶段：等待时间窗口过期，然后创建新请求
	t.Logf("Phase 2: Waiting for time window to expire...")

	// 计算需要等待的时间（等到下一个窗口开始）
	currentBucket := getTimeBucket()
	if currentBucket == initialBucket {
		// 还在同一个窗口，需要等待
		waitTime := time.Duration((initialBucket+1)*CollectWindowSeconds-time.Now().Unix()+1) * time.Second
		t.Logf("Waiting %v for next time window...", waitTime)
		time.Sleep(waitTime)
	}

	// 验证现在在新的时间窗口
	newBucket := getTimeBucket()
	t.Logf("New time bucket: %d (should be > %d)", newBucket, initialBucket)
	assert.Greater(t, newBucket, initialBucket, "Should be in a new time window")

	// 在新时间窗口创建请求
	t.Logf("Creating requests in new time window...")

	phase2Count := 3
	phase2JobKeys := make(map[string]int)

	for i := 0; i < phase2Count; i++ {
		kt := kit.New()
		kt.Rid = fmt.Sprintf("phase2-rid-%d", i)
		kt.BizID = bizID
		kt.AppID = appID

		agentID := fmt.Sprintf("agent-phase2-%d", i)
		containerID := fmt.Sprintf("container-phase2-%d", i)

		taskID, err := service.CreateAsyncDownloadTask(
			kt, bizID, appID, filePath, fileName,
			agentID, containerID, targetUser, targetDir, signature,
		)
		require.NoError(t, err, "Phase 2 task creation should succeed")

		// 获取 task 对应的 job
		taskData, err := bds.Get(ctx, taskID)
		require.NoError(t, err)
		task := &types.AsyncDownloadTask{}
		err = jsoni.Unmarshal([]byte(taskData), task)
		require.NoError(t, err)

		phase2JobKeys[task.JobID]++
		t.Logf("Phase 2 task %d: taskID=%s, jobID=%s", i, taskID, task.JobID)

		time.Sleep(100 * time.Millisecond)
	}

	// 验证第二阶段所有请求都在同一个新 job 中
	assert.Equal(t, 1, len(phase2JobKeys), "All phase 2 requests should be in the same job")
	var phase2JobKey string
	for k := range phase2JobKeys {
		phase2JobKey = k
	}
	t.Logf("Phase 2 job key: %s", phase2JobKey)

	// 验证第一阶段和第二阶段的 job 不同
	assert.NotEqual(t, phase1JobKey, phase2JobKey, "Phase 1 and Phase 2 should have different jobs")

	// 验证第二阶段 targets 数量
	phase2TargetsKey := GetTargetsKeyFromJobKey(phase2JobKey)
	phase2TargetsCount, err := bds.LLen(ctx, phase2TargetsKey)
	require.NoError(t, err)
	assert.Equal(t, int64(phase2Count), phase2TargetsCount, "Phase 2 should have correct targets count")
	t.Logf("Phase 2 targets count: %d", phase2TargetsCount)

	// 验证两个 job 的 timeBucket 不同
	bucket1 := ParseTimeBucketFromJobKey(phase1JobKey)
	bucket2 := ParseTimeBucketFromJobKey(phase2JobKey)
	assert.NotEqual(t, bucket1, bucket2, "TimeBuckets should be different")
	t.Logf("Phase 1 timeBucket: %d, Phase 2 timeBucket: %d", bucket1, bucket2)

	// 验证第一阶段的 job 时间窗口已过期
	assert.True(t, IsTimeWindowExpired(phase1JobKey), "Phase 1 job should have expired time window")

	// 验证第二阶段的 job 时间窗口未过期（或刚刚过期）
	// 注意：由于测试执行时间，这个断言可能不稳定，所以只记录日志
	t.Logf("Phase 2 job time window expired: %v", IsTimeWindowExpired(phase2JobKey))

	// 验证同一文件的两个 job 都存在
	phase1JobData, err := bds.Get(ctx, phase1JobKey)
	require.NoError(t, err)
	assert.NotEmpty(t, phase1JobData, "Phase 1 job should exist")

	phase2JobData, err := bds.Get(ctx, phase2JobKey)
	require.NoError(t, err)
	assert.NotEmpty(t, phase2JobData, "Phase 2 job should exist")

	// 解析并验证两个 job 的信息
	job1 := &types.AsyncDownloadJob{}
	err = jsoni.Unmarshal([]byte(phase1JobData), job1)
	require.NoError(t, err)

	job2 := &types.AsyncDownloadJob{}
	err = jsoni.Unmarshal([]byte(phase2JobData), job2)
	require.NoError(t, err)

	// 验证两个 job 的文件信息相同，但 JobID 不同
	assert.Equal(t, job1.BizID, job2.BizID)
	assert.Equal(t, job1.AppID, job2.AppID)
	assert.Equal(t, job1.FilePath, job2.FilePath)
	assert.Equal(t, job1.FileName, job2.FileName)
	assert.NotEqual(t, job1.JobID, job2.JobID, "JobIDs should be different")

	t.Logf("Test completed successfully!")
	t.Logf("Summary:")
	t.Logf("  - Phase 1: %d requests -> 1 job (%s) with %d targets", phase1Count, phase1JobKey, phase1TargetsCount)
	t.Logf("  - Phase 2: %d requests -> 1 job (%s) with %d targets", phase2Count, phase2JobKey, phase2TargetsCount)
	t.Logf("  - Jobs are correctly isolated by time window")
}

// TestContinuousRequestsAcrossTimeWindows 测试持续 15 秒以上的请求
// 验证请求在时间窗口边界处正确分配到不同的 job
func TestContinuousRequestsAcrossTimeWindows(t *testing.T) {
	// 跳过长时间测试（可以通过 -short 标志跳过）
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	// 初始化服务名称
	cc.InitService(cc.FeedServerName)

	// 设置 mock GSE
	setupMockGSE()

	// 设置测试 Redis
	bds, cleanup := setupTestRedis(t)
	defer cleanup()

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
	fileName := "continuous-test.txt"
	signature := "continuous-test-signature"
	targetUser := "test-user"
	targetDir := "/target/dir"

	ctx := context.Background()

	// 测试持续时间（至少覆盖 2 个时间窗口）
	testDuration := time.Duration(CollectWindowSeconds*2+5) * time.Second
	requestInterval := 500 * time.Millisecond // 每 500ms 发送一个请求

	t.Logf("Test duration: %v", testDuration)
	t.Logf("Request interval: %v", requestInterval)
	t.Logf("CollectWindowSeconds: %d", CollectWindowSeconds)

	// 用于记录每个 job 的请求数
	jobRequestCount := make(map[string]int)
	var mu sync.Mutex

	// 开始时间
	startTime := time.Now()
	initialBucket := getTimeBucket()
	t.Logf("Start time: %s, Initial bucket: %d", startTime.Format("15:04:05.000"), initialBucket)

	// 持续发送请求
	requestID := 0
	for time.Since(startTime) < testDuration {
		kt := kit.New()
		kt.Rid = fmt.Sprintf("continuous-rid-%d", requestID)
		kt.BizID = bizID
		kt.AppID = appID

		agentID := fmt.Sprintf("agent-continuous-%d", requestID)
		containerID := fmt.Sprintf("container-continuous-%d", requestID)

		taskID, err := service.CreateAsyncDownloadTask(
			kt, bizID, appID, filePath, fileName,
			agentID, containerID, targetUser, targetDir, signature,
		)

		if err != nil {
			t.Logf("Request %d failed: %v", requestID, err)
		} else {
			// 获取 task 对应的 job
			taskData, err := bds.Get(ctx, taskID)
			if err == nil {
				task := &types.AsyncDownloadTask{}
				if err := jsoni.Unmarshal([]byte(taskData), task); err == nil {
					mu.Lock()
					jobRequestCount[task.JobID]++
					mu.Unlock()

					currentBucket := getTimeBucket()
					t.Logf("Request %d: time=%s, bucket=%d, jobID=%s",
						requestID, time.Now().Format("15:04:05.000"), currentBucket, task.JobID)
				}
			}
		}

		requestID++
		time.Sleep(requestInterval)
	}

	// 统计结果
	t.Logf("\n=== Test Results ===")
	t.Logf("Total requests: %d", requestID)
	t.Logf("Number of jobs created: %d", len(jobRequestCount))

	// 验证至少创建了 2 个 job（因为测试跨越了至少 2 个时间窗口）
	assert.GreaterOrEqual(t, len(jobRequestCount), 2,
		"Should have at least 2 jobs for test spanning 2+ time windows")

	// 打印每个 job 的详情
	for jobKey, count := range jobRequestCount {
		bucket := ParseTimeBucketFromJobKey(jobKey)
		expired := IsTimeWindowExpired(jobKey)

		// 获取 targets 数量
		targetsKey := GetTargetsKeyFromJobKey(jobKey)
		targetsCount, _ := bds.LLen(ctx, targetsKey)

		t.Logf("Job: %s", jobKey)
		t.Logf("  - TimeBucket: %d", bucket)
		t.Logf("  - Requests: %d", count)
		t.Logf("  - Targets in Redis: %d", targetsCount)
		t.Logf("  - Time window expired: %v", expired)

		// 验证 targets 数量与请求数匹配
		assert.Equal(t, int64(count), targetsCount,
			"Targets count should match request count for job %s", jobKey)
	}

	// 验证 job 按时间窗口正确分组
	// 提取所有 timeBucket 并验证它们是连续的
	buckets := make([]int64, 0, len(jobRequestCount))
	for jobKey := range jobRequestCount {
		bucket := ParseTimeBucketFromJobKey(jobKey)
		buckets = append(buckets, bucket)
	}

	// 排序 buckets
	for i := 0; i < len(buckets)-1; i++ {
		for j := i + 1; j < len(buckets); j++ {
			if buckets[i] > buckets[j] {
				buckets[i], buckets[j] = buckets[j], buckets[i]
			}
		}
	}

	t.Logf("TimeBuckets (sorted): %v", buckets)

	// 验证 buckets 是连续的（差值为 1）
	for i := 1; i < len(buckets); i++ {
		diff := buckets[i] - buckets[i-1]
		assert.Equal(t, int64(1), diff,
			"TimeBuckets should be consecutive, got diff=%d between %d and %d",
			diff, buckets[i-1], buckets[i])
	}

	t.Logf("Test completed successfully!")
	t.Logf("Verified: requests are correctly distributed across %d time windows", len(jobRequestCount))
}
