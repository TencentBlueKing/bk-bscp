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
	CollectWindowSeconds int64 = 15

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

		// LPUSH 失败，需要重试
		attempt++
		delay := LPushRetryInitialDelay * time.Duration(1<<uint(attempt-1))
		if delay > LPushRetryMaxDelay {
			delay = LPushRetryMaxDelay
		}

		// nolint
		logs.Warnf("[lpushWithRetry] redis lpush failed, will retry, rid: %s, biz_id: %d, targets_key: %s, attempt: %d, err: %v",
			rid, bizID, targetsKey, attempt, err)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// 所有重试都失败了
	// nolint
	logs.Errorf("[lpushWithRetry] redis lpush failed after retries, rid: %s, biz_id: %d, targets_key: %s, attempts: %d, duration_ms: %d",
		rid, bizID, targetsKey, attempt, time.Since(startTime).Milliseconds())
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

	// nolint
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
		// nolint
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

// tryAddTargetToNewWindowJob 尝试在新窗口的 job 中添加 target
// 如果新窗口的 job 存在且是 Pending 状态，则添加 target 并返回 true
// 如果新窗口的 job 不存在或不是 Pending，返回 false 和新的 jobKey
func (ad *Service) tryAddTargetToNewWindowJob(ctx context.Context, bizID, appID uint32, fullPath string,
	nextTimeBucket int64, targetAgentID, targetContainerID string, rid string, startTime time.Time) (
	success bool, jobKey string, err error) {

	jobKey = GetJobKey(bizID, appID, fullPath, nextTimeBucket)
	targetsKey := GetTargetsKey(bizID, appID, fullPath, nextTimeBucket)

	jobData, err := ad.cs.Redis().Get(ctx, jobKey)
	if err != nil {
		logs.Errorf("[upsertAsyncDownloadJob] redis get new window job failed, rid: %s, biz_id: %d, file: %s, "+
			"job_id: %s, err: %v",
			rid, bizID, fullPath, jobKey, err)
		return false, jobKey, err
	}

	if jobData == "" {
		// 新窗口的 job 不存在，需要创建
		return false, jobKey, nil
	}

	// 新窗口的 job 已存在，检查状态
	job, err := parseAndCheckJobStatus(jobData, jobKey, rid, bizID, appID, fullPath)
	if err != nil {
		return false, jobKey, err
	}

	if job.Status == types.AsyncDownloadJobStatusPending {
		// 新窗口的 job 是 Pending，可以添加 target
		if err := ad.addTargetToJob(ctx, targetsKey, targetAgentID, targetContainerID, rid, bizID, appID,
			fullPath, jobKey, job.Status, startTime); err != nil {
			return false, jobKey, err
		}
		return true, jobKey, nil
	}

	// 新窗口的 job 也不是 Pending，需要继续创建下一个窗口
	return false, jobKey, nil
}

// nolint
func (ad *Service) upsertAsyncDownloadJob(kt *kit.Kit, bizID, appID uint32, filePath, fileName,
	targetAgentID, targetContainerID, targetUser, targetDir, signature string) (string, error) {
	rid := kt.Rid
	fullPath := path.Join(filePath, fileName)
	upsertStartTime := time.Now()

	// 使用时间窗口的 job key
	// 同一时间窗口内的请求会复用同一个 Job，窗口结束后新请求会创建新的 Job
	timeBucket := getTimeBucket()
	jobKey := GetJobKey(bizID, appID, fullPath, timeBucket)
	targetsKey := GetTargetsKey(bizID, appID, fullPath, timeBucket)

	// 1. 检查 job 是否存在
	jobData, err := ad.cs.Redis().Get(kt.Ctx, jobKey)
	if err != nil {
		logs.Errorf("[upsertAsyncDownloadJob] redis get failed, rid: %s, biz_id: %d, file: %s, err: %v",
			rid, bizID, fullPath, err)
		return "", err
	}

	if jobData != "" {
		// Job 已存在，解析并检查状态
		existingJob, err := parseAndCheckJobStatus(jobData, jobKey, rid, bizID, appID, fullPath)
		if err != nil {
			return "", err
		}

		// 如果 job 是 Pending 状态，直接添加 target
		if existingJob.Status == types.AsyncDownloadJobStatusPending {
			if err := ad.addTargetToJob(kt.Ctx, targetsKey, targetAgentID, targetContainerID, rid, bizID, appID,
				fullPath, jobKey, existingJob.Status, upsertStartTime); err != nil {
				return "", err
			}
			return jobKey, nil
		}

		// Job 不是 Pending 状态，需要创建新窗口的 job
		// 不能向 Running 状态的 job 添加 target，因为 scheduler 已经开始处理，会导致数据不一致
		nextTimeBucket, reason := calculateNextTimeBucket(jobKey, timeBucket, existingJob.Status)
		logs.Infof("[upsertAsyncDownloadJob] job not in pending, create new window, rid: %s, biz_id: %d, app_id: %d, "+
			"file: %s, old_job_id: %s, old_status: %s, old_time_bucket: %d, new_time_bucket: %d, reason: %s",
			rid, bizID, appID, fullPath, existingJob.JobID, existingJob.Status, timeBucket, nextTimeBucket, reason)

		// 尝试在新窗口的 job 中添加 target
		success, newJobKey, err := ad.tryAddTargetToNewWindowJob(kt.Ctx, bizID, appID, fullPath,
			nextTimeBucket, targetAgentID, targetContainerID, rid, upsertStartTime)
		if err != nil {
			return "", err
		}
		if success {
			return newJobKey, nil
		}

		// 新窗口的 job 不存在或不是 Pending，继续创建下一个窗口（最多再尝试一次）
		nextTimeBucket = nextTimeBucket + 1
		jobKey = GetJobKey(bizID, appID, fullPath, nextTimeBucket)
		targetsKey = GetTargetsKey(bizID, appID, fullPath, nextTimeBucket)

		logs.Warnf("[upsertAsyncDownloadJob] new window job also not in pending, create next window, rid: %s, biz_id: %d, app_id: %d, file: %s, new_time_bucket: %d",
			rid, bizID, appID, fullPath, nextTimeBucket)

		// 检查第二个新窗口的 job 是否存在
		jobData, err = ad.cs.Redis().Get(kt.Ctx, jobKey)
		if err != nil {
			logs.Errorf("[upsertAsyncDownloadJob] redis get second new window job failed, rid: %s, biz_id: %d, file: %s, job_id: %s, err: %v",
				rid, bizID, fullPath, jobKey, err)
			return "", err
		}
		// 如果 jobData 为空，继续到 SetNX 流程创建新 job
	}

	// 2. Job 不存在，尝试创建新 job（使用 SetNX，原子操作，不需要外层锁）
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
		logs.Errorf("[upsertAsyncDownloadJob] marshal new job failed, rid: %s, biz_id: %d, job_id: %s, err: %v",
			rid, bizID, jobKey, err)
		return "", err
	}

	// 使用 SetNX 原子性地创建 job，不需要外层锁
	// 如果 SetNX 失败，使用指数退避重试机制
	startTime := time.Now()
	attempt := 0
	var ok bool

	for attempt < RetryMaxAttempts {
		// 检查总超时时间
		if time.Since(startTime) > RetryTotalTimeout {
			logs.Errorf("[upsertAsyncDownloadJob] retry timeout, rid: %s, biz_id: %d, job_id: %s, attempts: %d",
				rid, bizID, jobKey, attempt)
			break
		}

		setnxStartTime := time.Now()
		ok, err = ad.cs.Redis().SetNX(kt.Ctx, jobKey, string(js), 30*60)
		setnxLatency := time.Since(setnxStartTime)
		if err != nil {
			// SetNX 失败（网络错误等），需要重试
			attempt++
			delay := RetryInitialDelay * time.Duration(1<<uint(attempt-1))
			if delay > RetryMaxDelay {
				delay = RetryMaxDelay
			}
			logs.Warnf("[upsertAsyncDownloadJob] redis setnx failed, will retry, rid: %s, biz_id: %d, app_id: %d, "+
				"file: %s, job_id: %s, attempt: %d, latency_ms: %d, err: %v",
				rid, bizID, appID, fullPath, jobKey, attempt, setnxLatency.Milliseconds(), err)
			select {
			case <-time.After(delay):
			case <-kt.Ctx.Done():
				return "", kt.Ctx.Err()
			}
			continue
		}

		if ok {
			// SetNX 成功，跳出循环
			logs.Infof("[upsertAsyncDownloadJob] setnx success, rid: %s, biz_id: %d, app_id: %d, file: %s, "+
				"job_id: %s, attempt: %d, latency_ms: %d",
				rid, bizID, appID, fullPath, jobKey, attempt+1, setnxLatency.Milliseconds())
			break
		}

		// SetNX 返回 false，说明其他请求已经创建了 job
		// 等待一小段时间后检查 job 是否存在
		attempt++
		delay := RetryInitialDelay * time.Duration(1<<uint(attempt-1))
		if delay > RetryMaxDelay {
			delay = RetryMaxDelay
		}
		select {
		case <-time.After(delay):
		case <-kt.Ctx.Done():
			return "", kt.Ctx.Err()
		}

		// 检查 job 是否已经存在
		jobData, err = ad.cs.Redis().Get(kt.Ctx, jobKey)
		if err != nil {
			logs.Warnf("[upsertAsyncDownloadJob] redis get failed during retry, will retry, rid: %s, biz_id: %d, job_id: %s, attempt: %d, err: %v",
				rid, bizID, jobKey, attempt, err)
			continue
		}

		if jobData != "" {
			// Job 已存在，跳出循环，进入添加 target 流程
			break
		}

		// Job 仍然不存在，继续重试 SetNX
		// 使用 Infof 记录调试信息（在高并发场景下可能产生较多日志，但有助于排查问题）
		logs.Infof("[upsertAsyncDownloadJob] job still not exists, will retry SetNX, rid: %s, biz_id: %d, job_id: %s, attempt: %d",
			rid, bizID, jobKey, attempt)
	}

	// 检查最终结果
	if err != nil {
		logs.Errorf("[upsertAsyncDownloadJob] redis setnx failed after retries, rid: %s, biz_id: %d, app_id: %d, "+
			"file: %s, job_id: %s, attempts: %d, duration_ms: %d, err: %v",
			rid, bizID, appID, fullPath, jobKey, attempt, time.Since(upsertStartTime).Milliseconds(), err)
		return "", fmt.Errorf("failed to create job after %d attempts, err: %v", attempt, err)
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
			logs.Errorf("[upsertAsyncDownloadJob] marshal target failed, rid: %s, biz_id: %d, err: %v",
				rid, bizID, e)
			return "", e
		}

		// 使用 LPUSH 添加第一个 target，带重试机制
		if e := ad.lpushWithRetry(kt.Ctx, targetsKey, string(targetData), rid, bizID); e != nil {
			logs.Errorf("[upsertAsyncDownloadJob] redis lpush failed after retries, rid: %s, biz_id: %d, app_id: %d, "+
				"file: %s, job_id: %s, duration_ms: %d, err: %v",
				rid, bizID, appID, fullPath, jobKey, time.Since(upsertStartTime).Milliseconds(), e)
			return "", e
		}

		logs.Infof("[upsertAsyncDownloadJob] create new job, rid: %s, biz_id: %d, app_id: %d, file: %s, "+
			"job_id: %s, setnx_attempts: %d, duration_ms: %d",
			rid, bizID, appID, fullPath, jobKey, attempt+1, time.Since(upsertStartTime).Milliseconds())
		return jobKey, nil
	}

	// SetNX 失败但 job 已存在（其他请求创建了），需要检查状态并添加 target
	// 再次确认 job 存在（可能在重试循环中已经检查过了）
	if jobData == "" {
		jobData, err = ad.cs.Redis().Get(kt.Ctx, jobKey)
		if err != nil {
			logs.Errorf("[upsertAsyncDownloadJob] redis get failed after retries, rid: %s, biz_id: %d, file: %s, err: %v",
				rid, bizID, fullPath, err)
			return "", fmt.Errorf("failed to get job after retries, err: %v", err)
		}
		if jobData == "" {
			return "", fmt.Errorf("failed to find or create job after %d attempts, rid: %s", attempt, rid)
		}
	}

	// 解析 job 数据并检查状态
	existingJob, err := parseAndCheckJobStatus(jobData, jobKey, rid, bizID, appID, fullPath)
	if err != nil {
		return "", err
	}

	// 如果 job 是 Pending 状态，直接添加 target
	if existingJob.Status == types.AsyncDownloadJobStatusPending {
		if err := ad.addTargetToJob(kt.Ctx, targetsKey, targetAgentID, targetContainerID, rid, bizID, appID,
			fullPath, jobKey, existingJob.Status, upsertStartTime); err != nil {
			return "", err
		}
		logs.Infof("[upsertAsyncDownloadJob] add target to existing job after retry, rid: %s, biz_id: %d, app_id: %d, "+
			"file: %s, job_id: %s, status: %s, setnx_attempts: %d, duration_ms: %d",
			rid, bizID, appID, fullPath, jobKey, existingJob.Status, attempt,
			time.Since(upsertStartTime).Milliseconds())
		return jobKey, nil
	}

	// Job 不是 Pending 状态，需要创建新窗口的 job
	currentTimeBucket := ParseTimeBucketFromJobKey(jobKey)
	if currentTimeBucket == 0 {
		logs.Errorf("[upsertAsyncDownloadJob] failed to parse time bucket from job key, rid: %s, biz_id: %d, "+
			"app_id: %d, file: %s, job_id: %s",
			rid, bizID, appID, fullPath, jobKey)
		return "", fmt.Errorf("failed to parse time bucket from job key")
	}

	nextTimeBucket, reason := calculateNextTimeBucket(jobKey, currentTimeBucket, existingJob.Status)
	logs.Infof("[upsertAsyncDownloadJob] job not in pending after retry, create new window, rid: %s, biz_id: %d, "+
		"app_id: %d, file: %s, old_job_id: %s, old_status: %s, old_time_bucket: %d, new_time_bucket: %d, reason: %s",
		rid, bizID, appID, fullPath, existingJob.JobID, existingJob.Status, currentTimeBucket, nextTimeBucket, reason)

	// 尝试在新窗口的 job 中添加 target
	success, newJobKey, err := ad.tryAddTargetToNewWindowJob(kt.Ctx, bizID, appID, fullPath,
		nextTimeBucket, targetAgentID, targetContainerID, rid, upsertStartTime)
	if err != nil {
		return "", err
	}
	if success {
		return newJobKey, nil
	}

	// 新窗口的 job 不存在或不是 Pending，继续创建下一个窗口
	nextTimeBucket = nextTimeBucket + 1
	jobKey = GetJobKey(bizID, appID, fullPath, nextTimeBucket)
	targetsKey = GetTargetsKey(bizID, appID, fullPath, nextTimeBucket)

	logs.Warnf("[upsertAsyncDownloadJob] new window job also not in pending after retry, create next window, rid: %s, biz_id: %d, app_id: %d, file: %s, new_time_bucket: %d",
		rid, bizID, appID, fullPath, nextTimeBucket)

	// 检查第二个新窗口的 job 是否存在
	jobData, err = ad.cs.Redis().Get(kt.Ctx, jobKey)
	if err != nil {
		logs.Errorf("[upsertAsyncDownloadJob] redis get second new window job failed after retry, rid: %s, biz_id: %d, file: %s, job_id: %s, err: %v",
			rid, bizID, fullPath, jobKey, err)
		return "", err
	}

	// 如果 jobData 为空，需要创建新 job，但已经超出 SetNX 循环
	// 返回错误，让调用方重试
	if jobData == "" {
		return "", fmt.Errorf("job not in pending status and new window job not found, need retry, rid: %s", rid)
	}

	// 如果 jobData 存在，再次检查状态
	existingJob, err = parseAndCheckJobStatus(jobData, jobKey, rid, bizID, appID, fullPath)
	if err != nil {
		return "", err
	}

	if existingJob.Status == types.AsyncDownloadJobStatusPending {
		if err := ad.addTargetToJob(kt.Ctx, targetsKey, targetAgentID, targetContainerID, rid, bizID, appID,
			fullPath, jobKey, existingJob.Status, upsertStartTime); err != nil {
			return "", err
		}
		return jobKey, nil
	}

	// 第二个新窗口的 job 也不是 Pending，返回错误让调用方重试
	return "", fmt.Errorf("job not in pending status after multiple window attempts, need retry, rid: %s", rid)
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
