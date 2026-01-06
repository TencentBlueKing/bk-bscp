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

// Package asyncdownload NOTES
package asyncdownload

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	prm "github.com/prometheus/client_golang/prometheus"

	clientset "github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/client-set"
	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/uuid"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/pkg/runtime/jsoni"
)

const (
	// CollectWindowSeconds 收集窗口时间（秒）
	// 同一时间窗口内的请求会被合并到同一个 Job 中
	// 窗口结束后 Job 开始处理，新请求会创建新的 Job
	CollectWindowSeconds int64 = 10

	// RetryMaxAttempts SetNX 失败后的最大重试次数
	RetryMaxAttempts = 5
	// RetryInitialDelay 初始重试延迟（毫秒）
	RetryInitialDelay = 5 * time.Millisecond
	// RetryMaxDelay 最大重试延迟（毫秒）
	RetryMaxDelay = 50 * time.Millisecond
	// RetryTotalTimeout 总重试超时时间（毫秒）
	RetryTotalTimeout = 200 * time.Millisecond

	// LPushRetryMaxAttempts LPUSH 失败后的最大重试次数
	LPushRetryMaxAttempts = 3
	// LPushRetryInitialDelay LPUSH 初始重试延迟（毫秒）
	LPushRetryInitialDelay = 10 * time.Millisecond
	// LPushRetryMaxDelay LPUSH 最大重试延迟（毫秒）
	LPushRetryMaxDelay = 100 * time.Millisecond
	// LPushRetryTotalTimeout LPUSH 总重试超时时间（毫秒）
	LPushRetryTotalTimeout = 300 * time.Millisecond
)

// getTimeBucket 获取当前时间窗口
func getTimeBucket() int64 {
	return time.Now().Unix() / CollectWindowSeconds
}

// GetJobKey 获取带时间窗口的 Job Key
// 格式: AsyncDownloadJob:{bizID}:{appID}:{fullPath}:{timeBucket}
func GetJobKey(bizID, appID uint32, fullPath string, timeBucket int64) string {
	return fmt.Sprintf("AsyncDownloadJob:%d:%d:%s:%d", bizID, appID, fullPath, timeBucket)
}

// GetTargetsKey 获取带时间窗口的 Targets Key
// 格式: AsyncDownloadJob:Targets:{bizID}:{appID}:{fullPath}:{timeBucket}
func GetTargetsKey(bizID, appID uint32, fullPath string, timeBucket int64) string {
	return fmt.Sprintf("AsyncDownloadJob:Targets:%d:%d:%s:%d", bizID, appID, fullPath, timeBucket)
}

// GetTargetsKeyFromJobKey 从 JobKey 获取对应的 TargetsKey
// 通过替换前缀实现
func GetTargetsKeyFromJobKey(jobKey string) string {
	return strings.Replace(jobKey, "AsyncDownloadJob:", "AsyncDownloadJob:Targets:", 1)
}

// ParseTimeBucketFromJobKey 从 JobKey 中解析时间窗口
// JobKey 格式: AsyncDownloadJob:{bizID}:{appID}:{fullPath}:{timeBucket}
// 返回 timeBucket，如果解析失败返回 0
func ParseTimeBucketFromJobKey(jobKey string) int64 {
	parts := strings.Split(jobKey, ":")
	if len(parts) < 2 {
		return 0
	}
	// timeBucket 是最后一个部分
	lastPart := parts[len(parts)-1]
	timeBucket, err := strconv.ParseInt(lastPart, 10, 64)
	if err != nil {
		return 0
	}
	return timeBucket
}

// IsTimeWindowExpired 判断时间窗口是否已结束
// 根据 jobKey 中的 timeBucket 计算窗口结束时间，判断是否已过期
func IsTimeWindowExpired(jobKey string) bool {
	timeBucket := ParseTimeBucketFromJobKey(jobKey)
	if timeBucket == 0 {
		// 解析失败（可能是旧格式），使用保守策略，认为已过期
		return true
	}
	// 窗口结束时间 = (timeBucket + 1) * CollectWindowSeconds
	windowEndTime := time.Unix((timeBucket+1)*CollectWindowSeconds, 0)
	return time.Now().After(windowEndTime)
}

// NewService initialize the async download service instance.
func NewService(cs *clientset.ClientSet, mc *metric, redLock *lock.RedisLock) (*Service, error) {

	return &Service{
		enabled: cc.FeedServer().GSE.Enabled,
		cs:      cs,
		redLock: redLock,
		metric:  mc,
	}, nil
}

// Service defines async download related operations.
type Service struct {
	enabled bool
	cs      *clientset.ClientSet
	redLock *lock.RedisLock
	metric  *metric
}

// CreateAsyncDownloadTask creates a new async download task.
func (ad *Service) CreateAsyncDownloadTask(kt *kit.Kit, bizID, appID uint32, filePath, fileName,
	targetAgentID, targetContainerID, targetUser, targetDir, signature string) (string, error) {
	taskID := fmt.Sprintf("AsyncDownloadTask:%d:%d:%s:%s",
		bizID, appID, path.Join(filePath, fileName), uuid.UUID())

	jobID, err := ad.upsertAsyncDownloadJob(kt, bizID, appID, filePath, fileName, targetAgentID,
		targetContainerID, targetUser, targetDir, signature)
	if err != nil {
		return "", err
	}
	task := &types.AsyncDownloadTask{
		BizID:             bizID,
		AppID:             appID,
		JobID:             jobID,
		TargetAgentID:     targetAgentID,
		TargetContainerID: targetContainerID,
		FilePath:          filePath,
		FileName:          fileName,
		FileSignature:     signature,
		Status:            types.AsyncDownloadJobStatusPending,
		CreateTime:        time.Now(),
	}

	if err = ad.upsertAsyncDownloadTask(kt.Ctx, taskID, task); err != nil {
		logs.Errorf("upsert async download task %s failed, err %s", taskID, err.Error())
		return "", err
	}
	logs.Infof("upsert async download task %s success, biz:%d, app:%d, file:%s, status:%s",
		taskID, task.BizID, task.AppID, path.Join(task.FilePath, task.FileName), task.Status)
	ad.metric.taskCounter.With(prm.Labels{"biz": strconv.Itoa(int(task.BizID)),
		"app": strconv.Itoa(int(task.AppID)), "file": path.Join(task.FilePath, task.FileName), "status": task.Status}).
		Inc()

	return taskID, nil
}

// GetAsyncDownloadTask get async download task record.
func (ad *Service) GetAsyncDownloadTask(kt *kit.Kit, bizID uint32, taskID string) (
	*types.AsyncDownloadTask, error) {

	taskData, err := ad.cs.Redis().Get(kt.Ctx, taskID)
	if err != nil {
		return nil, err
	}
	if taskData == "" {
		// task not exists
		logs.Errorf("async download task %s not exists in redis", taskID)
		return nil, fmt.Errorf("async download task %s not exists in redis", taskID)
	}

	task := new(types.AsyncDownloadTask)
	if err := jsoni.UnmarshalFromString(taskData, &task); err != nil {
		logs.Errorf("unmarshal task %s failed, err %s", taskID, err.Error())
		return nil, err
	}

	return task, nil
}

// GetAsyncDownloadTaskStatus get async download task and update it's status.
// task is in instance level, so do not need to lock it.
func (ad *Service) GetAsyncDownloadTaskStatus(kt *kit.Kit, bizID uint32, taskID string) (
	string, error) {

	taskData, err := ad.cs.Redis().Get(kt.Ctx, taskID)
	if err != nil {
		return "", err
	}
	if taskData == "" {
		// task not exists
		logs.Errorf("async download task %s not exists in redis", taskID)
		return "", fmt.Errorf("async download task %s not exists in redis", taskID)
	}

	task := new(types.AsyncDownloadTask)
	if e := jsoni.UnmarshalFromString(taskData, &task); e != nil {
		logs.Errorf("unmarshal task %s failed, err %s", taskID, e.Error())
		return "", e
	}

	jobData, err := ad.cs.Redis().Get(kt.Ctx, task.JobID)
	if err != nil {
		return "", err
	}
	if jobData == "" {
		// job not exists
		logs.Errorf("async download job %s not exists in redis, it should not happen!", task.JobID)
		return "", fmt.Errorf("async download job %s not exists in redis", taskID)
	}

	job := &types.AsyncDownloadJob{}
	if err := jsoni.UnmarshalFromString(jobData, &job); err != nil {
		return "", err
	}

	oldTaskStatus := task.Status

	// ! ensure task can only exists in specific status
	if _, ok := job.SuccessTargets[fmt.Sprintf("%s:%s", task.TargetAgentID, task.TargetContainerID)]; ok {
		task.Status = types.AsyncDownloadJobStatusSuccess
		if err := ad.upsertAsyncDownloadTask(kt.Ctx, taskID, task); err != nil {
			logs.Errorf("update task %s status to success failed, err %s", taskID, err.Error())
		}
	}

	if _, ok := job.FailedTargets[fmt.Sprintf("%s:%s", task.TargetAgentID, task.TargetContainerID)]; ok {
		task.Status = types.AsyncDownloadJobStatusFailed
		if err := ad.upsertAsyncDownloadTask(kt.Ctx, taskID, task); err != nil {
			logs.Errorf("update task %s status to success failed, err %s", taskID, err.Error())
		}
	}

	if _, ok := job.TimeoutTargets[fmt.Sprintf("%s:%s", task.TargetAgentID, task.TargetContainerID)]; ok {
		task.Status = types.AsyncDownloadJobStatusTimeout
		if err := ad.upsertAsyncDownloadTask(kt.Ctx, taskID, task); err != nil {
			logs.Errorf("update task %s status to success failed, err %s", taskID, err.Error())
		}
	}

	if _, ok := job.DownloadingTargets[fmt.Sprintf("%s:%s", task.TargetAgentID, task.TargetContainerID)]; ok {
		task.Status = types.AsyncDownloadJobStatusRunning
		if err := ad.upsertAsyncDownloadTask(kt.Ctx, taskID, task); err != nil {
			logs.Errorf("update task %s status to success failed, err %s", taskID, err.Error())
		}
	}

	if task.Status != oldTaskStatus {
		ad.metric.taskCounter.With(prm.Labels{"biz": strconv.Itoa(int(task.BizID)),
			"app": strconv.Itoa(int(task.AppID)), "file": path.Join(task.FilePath, task.FileName),
			"status": task.Status}).Inc()

		ad.metric.taskDurationSeconds.With(prm.Labels{"biz": strconv.Itoa(int(task.BizID)),
			"app": strconv.Itoa(int(task.AppID)), "file": path.Join(task.FilePath, task.FileName),
			"status": oldTaskStatus}).Observe(time.Since(task.CreateTime).Seconds())
	}

	return task.Status, nil
}

// calculateExponentialBackoffDelay 计算指数退避延迟时间，避免位移或乘法溢出
// attempt: 当前重试次数（从1开始）
// initialDelay: 初始延迟时间
// maxDelay: 最大延迟时间
func calculateExponentialBackoffDelay(attempt int, initialDelay, maxDelay time.Duration) time.Duration {
	// 基本健壮性检查
	if attempt <= 0 {
		return initialDelay
	}
	if initialDelay <= 0 || maxDelay <= 0 {
		return initialDelay
	}

	// 计算在不超过最大重试间隔的情况下，倍率的最大安全值
	maxMultiplier := maxDelay / initialDelay
	if maxMultiplier <= 1 {
		return maxDelay
	}

	// 使用循环方式计算倍率，避免位移溢出
	multiplier := time.Duration(1)
	for i := 1; i < attempt && multiplier < maxMultiplier; i++ {
		multiplier <<= 1
		if multiplier > maxMultiplier {
			multiplier = maxMultiplier
			break
		}
	}

	delay := initialDelay * multiplier
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

// lpushWithRetry 带重试机制的 LPUSH 操作
// 使用指数退避策略，在 LPUSH 失败时自动重试
func (ad *Service) lpushWithRetry(
	ctx context.Context,
	targetsKey string,
	targetData string,
	rid string,
	bizID uint32) error {
	startTime := time.Now()
	attempt := 0
	var lastErr error

	for attempt < LPushRetryMaxAttempts {
		// 检查总超时时间
		if time.Since(startTime) > LPushRetryTotalTimeout {
			logs.Errorf("[lpushWithRetry] retry timeout, rid: %s, biz_id: %d, targets_key: %s, attempts: %d, duration_ms: %d",
				rid, bizID, targetsKey, attempt, time.Since(startTime).Milliseconds())
			break
		}

		lpushStartTime := time.Now()
		err := ad.cs.Redis().LPush(ctx, targetsKey, targetData)
		if err == nil {
			// LPUSH 成功，设置 TTL
			lpushLatency := time.Since(lpushStartTime)
			setExpireErr := ad.cs.Redis().Expire(ctx, targetsKey, 30*60, bedis.ExpireMode(""))
			if setExpireErr != nil {
				logs.Warnf("[lpushWithRetry] set expire failed, rid: %s, biz_id: %d, targets_key: %s, err: %v",
					rid, bizID, targetsKey, setExpireErr)
			}
			logs.Infof("[lpushWithRetry] success, rid: %s, biz_id: %d, targets_key: %s, attempts: %d, "+
				"latency_ms: %d, total_duration_ms: %d",
				rid, bizID, targetsKey, attempt+1, lpushLatency.Milliseconds(),
				time.Since(startTime).Milliseconds())
			return nil
		}

		// LPUSH 失败，保存错误信息并重试
		lastErr = err
		attempt++
		delay := calculateExponentialBackoffDelay(attempt, LPushRetryInitialDelay, LPushRetryMaxDelay)

		// nolint:lll // 日志行较长但保持可读性
		logs.Warnf("[lpushWithRetry] redis lpush failed, will retry, rid: %s, biz_id: %d, targets_key: %s, attempt: %d, err: %v",
			rid, bizID, targetsKey, attempt, err)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// 所有重试都失败了，返回包含最后一次错误信息的错误
	// nolint:lll // 日志行较长但保持可读性
	logs.Errorf("[lpushWithRetry] redis lpush failed after retries, rid: %s, biz_id: %d, targets_key: %s, attempts: %d, duration_ms: %d, last_err: %v",
		rid, bizID, targetsKey, attempt, time.Since(startTime).Milliseconds(), lastErr)
	if lastErr != nil {
		return fmt.Errorf("failed to lpush after %d attempts, last error: %w", attempt, lastErr)
	}
	return fmt.Errorf("failed to lpush after %d attempts", attempt)
}

// addTargetToJob 添加 target 到指定的 job
func (ad *Service) addTargetToJob(ctx context.Context, targetsKey string, targetAgentID, targetContainerID string,
	rid string, bizID, appID uint32, fullPath, jobKey string, jobStatus string, startTime time.Time) error {
	target := &types.AsyncDownloadTarget{
		AgentID:     targetAgentID,
		ContainerID: targetContainerID,
	}
	targetData, err := jsoni.Marshal(target)
	if err != nil {
		logs.Errorf("[upsertAsyncDownloadJob] marshal target failed, rid: %s, biz_id: %d, err: %v", rid, bizID, err)
		return err
	}

	if err := ad.lpushWithRetry(ctx, targetsKey, string(targetData), rid, bizID); err != nil {
		logs.Errorf("[upsertAsyncDownloadJob] redis lpush failed after retries, rid: %s, biz_id: %d, app_id: %d, file: %s, "+
			"job_id: %s, duration_ms: %d, err: %v",
			rid, bizID, appID, fullPath, jobKey, time.Since(startTime).Milliseconds(), err)
		return err
	}

	// nolint:lll // 日志行较长但保持可读性
	logs.Infof("[upsertAsyncDownloadJob] add target to job, rid: %s, biz_id: %d, app_id: %d, file: %s, job_id: %s, status: %s, "+
		"duration_ms: %d",
		rid, bizID, appID, fullPath, jobKey, jobStatus, time.Since(startTime).Milliseconds())
	return nil
}

// parseAndCheckJobStatus 解析 job 数据并检查状态
func parseAndCheckJobStatus(jobData, jobKey, rid string, bizID, appID uint32, fullPath string) (
	*types.AsyncDownloadJob, error) {
	job := &types.AsyncDownloadJob{}
	if err := jsoni.UnmarshalFromString(jobData, job); err != nil {
		// nolint:lll // 日志行较长但保持可读性
		logs.Errorf("[upsertAsyncDownloadJob] unmarshal job failed, rid: %s, biz_id: %d, app_id: %d, file: %s, job_id: %s, err: %v",
			rid, bizID, appID, fullPath, jobKey, err)
		return nil, fmt.Errorf("unmarshal job failed, err: %v", err)
	}
	return job, nil
}

// calculateNextTimeBucket 计算下一个时间窗口的 timeBucket 和原因
func calculateNextTimeBucket(jobKey string, currentTimeBucket int64, jobStatus string) (
	nextTimeBucket int64, reason string) {
	windowExpired := IsTimeWindowExpired(jobKey)
	isFinalStatus := jobStatus == types.AsyncDownloadJobStatusSuccess ||
		jobStatus == types.AsyncDownloadJobStatusFailed ||
		jobStatus == types.AsyncDownloadJobStatusTimeout

	nextTimeBucket = currentTimeBucket + 1

	if windowExpired {
		reason = "window_expired"
	} else if isFinalStatus {
		reason = "job_final_status"
	} else {
		// job 是 Running 状态，创建新窗口（不能向 Running 状态的 job 添加 target）
		reason = "job_running"
	}

	return nextTimeBucket, reason
}

const (
	// MaxWindowLookahead 最大窗口前瞻数量，限制查找未来窗口的数量
	MaxWindowLookahead = 3
)

// findOrCreatePendingJob 在指定的时间窗口范围内查找或创建 Pending 状态的 job
// 从 startTimeBucket 开始，最多查找 maxWindows 个窗口
// 如果找到 Pending 状态的 job，添加 target 并返回 jobKey
// 如果找不到，尝试创建新的 job（使用 SetNX 重试循环）
// 返回: (jobKey, nil) 成功, ("", error) 失败
// nolint:funlen // 函数较长但逻辑清晰，拆分会影响可读性和控制流
func (ad *Service) findOrCreatePendingJob(
	ctx context.Context,
	bizID, appID uint32,
	filePath, fileName, targetAgentID, targetContainerID, targetUser, targetDir, signature string,
	startTimeBucket int64,
	maxWindows int,
	rid string,
	startTime time.Time) (string, error) {

	for windowOffset := int64(0); windowOffset < int64(maxWindows); windowOffset++ {
		currentTimeBucket := startTimeBucket + windowOffset
		jobKey := GetJobKey(bizID, appID, path.Join(filePath, fileName), currentTimeBucket)
		targetsKey := GetTargetsKey(bizID, appID, path.Join(filePath, fileName), currentTimeBucket)

		// 1. 检查 job 是否存在
		jobData, err := ad.cs.Redis().Get(ctx, jobKey)
		if err != nil {
			logs.Errorf("[findOrCreatePendingJob] redis get failed, rid: %s, biz_id: %d, file: %s, job_id: %s, err: %v",
				rid, bizID, path.Join(filePath, fileName), jobKey, err)
			continue // 继续尝试下一个窗口
		}

		if jobData != "" {
			// Job 已存在，检查状态
			job, parseErr := parseAndCheckJobStatus(jobData, jobKey, rid, bizID, appID, path.Join(filePath, fileName))
			if parseErr != nil {
				logs.Errorf("[findOrCreatePendingJob] parse and check job status failed, rid: %s, "+
					"biz_id: %d, file: %s, job_id: %s, err: %v",
					rid, bizID, path.Join(filePath, fileName), jobKey, parseErr)
				continue // 继续尝试下一个窗口
			}

			if job.Status == types.AsyncDownloadJobStatusPending {
				// 找到 Pending 状态的 job，添加 target
				if addErr := ad.addTargetToJob(ctx, targetsKey, targetAgentID, targetContainerID, rid, bizID, appID,
					path.Join(filePath, fileName), jobKey, job.Status, startTime); addErr != nil {
					logs.Errorf("[findOrCreatePendingJob] add target to job failed, rid: %s, "+
						"biz_id: %d, file: %s, job_id: %s, err: %v",
						rid, bizID, path.Join(filePath, fileName), jobKey, addErr)
					continue // 继续尝试下一个窗口
				}
				if windowOffset > 0 {
					logs.Infof("[findOrCreatePendingJob] found pending job in window offset %d, rid: %s, biz_id: %d, job_id: %s",
						windowOffset, rid, bizID, jobKey)
				}
				return jobKey, nil
			}

			// Job 存在但不是 Pending 状态，继续尝试下一个窗口
			if windowOffset == 0 {
				nextTimeBucket, reason := calculateNextTimeBucket(jobKey, currentTimeBucket, job.Status)
				logs.Infof("[findOrCreatePendingJob] job not in pending, try next window, rid: %s, biz_id: %d, "+
					"file: %s, old_job_id: %s, old_status: %s, old_time_bucket: %d, new_time_bucket: %d, reason: %s",
					rid, bizID, path.Join(filePath, fileName), job.JobID, job.Status, currentTimeBucket, nextTimeBucket, reason)
			}
			continue
		}

		// 2. Job 不存在，尝试创建新 job（使用 SetNX，原子操作）
		job := &types.AsyncDownloadJob{
			JobID:         jobKey,
			BizID:         bizID,
			AppID:         appID,
			FilePath:      filePath,
			FileName:      fileName,
			TargetFileDir: targetDir,
			TargetUser:    targetUser,
			FileSignature: signature,
			// 注意：不包含 Targets 数组（新数据只存储在 Redis List）
			Status:             types.AsyncDownloadJobStatusPending,
			CreateTime:         time.Now(),
			SuccessTargets:     make(map[string]gse.TransferFileResultDataResultContent),
			FailedTargets:      make(map[string]gse.TransferFileResultDataResultContent),
			DownloadingTargets: make(map[string]gse.TransferFileResultDataResultContent),
			TimeoutTargets:     make(map[string]gse.TransferFileResultDataResultContent),
		}

		js, err := jsoni.Marshal(job)
		if err != nil {
			logs.Errorf("[findOrCreatePendingJob] marshal new job failed, rid: %s, biz_id: %d, job_id: %s, err: %v",
				rid, bizID, jobKey, err)
			continue // 继续尝试下一个窗口
		}

		// 使用 SetNX 原子性地创建 job，使用指数退避重试机制
		setnxStartTime := time.Now()
		attempt := 0
		var ok bool

		for attempt < RetryMaxAttempts {
			// 检查总超时时间
			if time.Since(setnxStartTime) > RetryTotalTimeout {
				logs.Errorf("[findOrCreatePendingJob] retry timeout, rid: %s, biz_id: %d, job_id: %s, attempts: %d",
					rid, bizID, jobKey, attempt)
				break
			}

			setnxAttemptStartTime := time.Now()
			ok, err = ad.cs.Redis().SetNX(ctx, jobKey, string(js), 30*60)
			setnxLatency := time.Since(setnxAttemptStartTime)
			if err != nil {
				// SetNX 失败（网络错误等），需要重试
				attempt++
				delay := calculateExponentialBackoffDelay(attempt, RetryInitialDelay, RetryMaxDelay)
				logs.Warnf("[findOrCreatePendingJob] redis setnx failed, will retry, rid: %s, biz_id: %d, "+
					"file: %s, job_id: %s, attempt: %d, latency_ms: %d, err: %v",
					rid, bizID, path.Join(filePath, fileName), jobKey, attempt, setnxLatency.Milliseconds(), err)
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return "", ctx.Err()
				}
				continue
			}

			if ok {
				// SetNX 成功，创建了 job
				logs.Infof("[findOrCreatePendingJob] setnx success, rid: %s, biz_id: %d, file: %s, "+
					"job_id: %s, attempt: %d, latency_ms: %d, window_offset: %d",
					rid, bizID, path.Join(filePath, fileName), jobKey, attempt+1, setnxLatency.Milliseconds(), windowOffset)
				break
			}

			// SetNX 返回 false，说明其他请求已经创建了 job
			// 等待一小段时间后检查 job 是否存在
			attempt++
			delay := calculateExponentialBackoffDelay(attempt, RetryInitialDelay, RetryMaxDelay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return "", ctx.Err()
			}

			// 检查 job 是否已经存在
			jobData, err = ad.cs.Redis().Get(ctx, jobKey)
			if err != nil {
				logs.Warnf("[findOrCreatePendingJob] redis get failed during retry, will retry, "+
					"rid: %s, biz_id: %d, job_id: %s, attempt: %d, err: %v",
					rid, bizID, jobKey, attempt, err)
				continue
			}

			if jobData != "" {
				// Job 已存在，跳出循环，检查状态
				break
			}

			// Job 仍然不存在，继续重试 SetNX
			logs.Infof("[findOrCreatePendingJob] job still not exists, will retry SetNX, "+
				"rid: %s, biz_id: %d, job_id: %s, attempt: %d",
				rid, bizID, jobKey, attempt)
		}

		// 检查 SetNX 结果
		if err != nil {
			logs.Errorf("[findOrCreatePendingJob] redis setnx failed after retries, rid: %s, biz_id: %d, "+
				"file: %s, job_id: %s, attempts: %d, err: %v",
				rid, bizID, path.Join(filePath, fileName), jobKey, attempt, err)
			continue // 继续尝试下一个窗口
		}

		if ok {
			// SetNX 成功，说明是第一个请求，创建了 job
			ad.metric.jobCounter.With(prm.Labels{"biz": strconv.Itoa(int(job.BizID)),
				"app": strconv.Itoa(int(job.AppID)), "file": path.Join(job.FilePath, job.FileName),
				"targets": "1", "status": job.Status}).Inc()

			// 添加第一个 target 到 List
			target := &types.AsyncDownloadTarget{
				AgentID:     targetAgentID,
				ContainerID: targetContainerID,
			}
			targetData, e := jsoni.Marshal(target)
			if e != nil {
				logs.Errorf("[findOrCreatePendingJob] marshal target failed, rid: %s, biz_id: %d, err: %v",
					rid, bizID, e)
				continue // 继续尝试下一个窗口
			}

			// 使用 LPUSH 添加第一个 target，带重试机制
			if e := ad.lpushWithRetry(ctx, targetsKey, string(targetData), rid, bizID); e != nil {
				logs.Errorf("[findOrCreatePendingJob] redis lpush failed after retries, rid: %s, biz_id: %d, "+
					"file: %s, job_id: %s, err: %v",
					rid, bizID, path.Join(filePath, fileName), jobKey, e)
				continue // 继续尝试下一个窗口
			}

			logs.Infof("[findOrCreatePendingJob] create new job, rid: %s, biz_id: %d, file: %s, "+
				"job_id: %s, setnx_attempts: %d, window_offset: %d",
				rid, bizID, path.Join(filePath, fileName), jobKey, attempt+1, windowOffset)
			return jobKey, nil
		}

		// SetNX 失败但 job 已存在（其他请求创建了），检查状态
		if jobData == "" {
			jobData, err = ad.cs.Redis().Get(ctx, jobKey)
			if err != nil {
				logs.Errorf("[findOrCreatePendingJob] redis get failed after retries, rid: %s, biz_id: %d, file: %s, err: %v",
					rid, bizID, path.Join(filePath, fileName), err)
				continue // 继续尝试下一个窗口
			}
		}

		if jobData != "" {
			// 解析 job 数据并检查状态
			existingJob, err := parseAndCheckJobStatus(jobData, jobKey, rid, bizID, appID, path.Join(filePath, fileName))
			if err != nil {
				continue // 继续尝试下一个窗口
			}

			if existingJob.Status == types.AsyncDownloadJobStatusPending {
				// Job 是 Pending 状态，添加 target
				if err := ad.addTargetToJob(ctx, targetsKey, targetAgentID, targetContainerID, rid, bizID, appID,
					path.Join(filePath, fileName), jobKey, existingJob.Status, startTime); err != nil {
					return "", err
				}
				logs.Infof("[findOrCreatePendingJob] add target to existing job after retry, rid: %s, biz_id: %d, "+
					"file: %s, job_id: %s, status: %s, setnx_attempts: %d, window_offset: %d",
					rid, bizID, path.Join(filePath, fileName), jobKey, existingJob.Status, attempt, windowOffset)
				return jobKey, nil
			}

			// Job 存在但不是 Pending 状态，继续尝试下一个窗口
			continue
		}
	}

	// 所有窗口都尝试失败
	return "", fmt.Errorf("failed to find or create pending job after checking %d windows, rid: %s", maxWindows, rid)
}

// upsertAsyncDownloadJob 创建或更新异步下载任务
// 使用统一的时间窗口查找逻辑，避免不对称行为
func (ad *Service) upsertAsyncDownloadJob(kt *kit.Kit, bizID, appID uint32, filePath, fileName,
	targetAgentID, targetContainerID, targetUser, targetDir, signature string) (string, error) {
	rid := kt.Rid
	upsertStartTime := time.Now()

	// 使用时间窗口的 job key
	// 同一时间窗口内的请求会复用同一个 Job，窗口结束后新请求会创建新的 Job
	timeBucket := getTimeBucket()

	// 使用统一的窗口查找逻辑，从当前时间窗口开始，最多查找 MaxWindowLookahead 个窗口
	// 这样可以统一处理所有情况，避免不对称行为
	jobKey, err := ad.findOrCreatePendingJob(
		kt.Ctx,
		bizID, appID,
		filePath, fileName, targetAgentID, targetContainerID, targetUser, targetDir, signature,
		timeBucket,
		MaxWindowLookahead,
		rid,
		upsertStartTime,
	)

	if err != nil {
		logs.Errorf("[upsertAsyncDownloadJob] failed to find or create pending job, rid: %s, biz_id: %d, app_id: %d, "+
			"file: %s, duration_ms: %d, err: %v",
			rid, bizID, appID, path.Join(filePath, fileName), time.Since(upsertStartTime).Milliseconds(), err)
		return "", err
	}

	logs.Infof("[upsertAsyncDownloadJob] success, rid: %s, biz_id: %d, app_id: %d, file: %s, job_id: %s, duration_ms: %d",
		rid, bizID, appID, path.Join(filePath, fileName), jobKey, time.Since(upsertStartTime).Milliseconds())
	return jobKey, nil
}

func (ad *Service) upsertAsyncDownloadTask(ctx context.Context, taskID string,
	task *types.AsyncDownloadTask) error {
	js, err := jsoni.Marshal(task)
	if err != nil {
		return err
	}
	logs.Infof("upsert async download task %s, biz:%d, app:%d, file:%s, status:%s",
		taskID, task.BizID, task.AppID, path.Join(task.FilePath, task.FileName), task.Status)
	return ad.cs.Redis().Set(ctx, taskID, string(js), 30*60)
}
