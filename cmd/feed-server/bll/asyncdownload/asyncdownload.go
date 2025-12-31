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

// nolint
func (ad *Service) upsertAsyncDownloadJob(kt *kit.Kit, bizID, appID uint32, filePath, fileName,
	targetAgentID, targetContainerID, targetUser, targetDir, signature string) (string, error) {
	rid := kt.Rid
	fullPath := path.Join(filePath, fileName)

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
		// Job 已存在，直接添加 target（使用 LPUSH，原子操作，不需要锁）
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

		// 使用 LPUSH 原子性地添加 target，不需要锁
		if e := ad.cs.Redis().LPush(kt.Ctx, targetsKey, string(targetData)); e != nil {
			logs.Errorf("[upsertAsyncDownloadJob] redis lpush failed, rid: %s, biz_id: %d, err: %v",
				rid, bizID, e)
			return "", e
		}

		// 确保 targetsKey 的 TTL
		_ = ad.cs.Redis().Expire(kt.Ctx, targetsKey, 30*60, bedis.NX)

		logs.Infof("[upsertAsyncDownloadJob] add target to existing job, rid: %s, biz_id: %d, job_id: %s",
			rid, bizID, jobKey)
		return jobKey, nil
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
	ok, err := ad.cs.Redis().SetNX(kt.Ctx, jobKey, string(js), 30*60)
	if err != nil {
		logs.Errorf("[upsertAsyncDownloadJob] redis setnx failed, rid: %s, biz_id: %d, job_id: %s, err: %v",
			rid, bizID, jobKey, err)
		return "", err
	}

	if ok {
		// 创建成功，说明是第一个请求
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

		if e := ad.cs.Redis().LPush(kt.Ctx, targetsKey, string(targetData)); e != nil {
			logs.Errorf("[upsertAsyncDownloadJob] redis lpush failed, rid: %s, biz_id: %d, err: %v",
				rid, bizID, e)
			return "", e
		}

		// 设置 targetsKey 的 TTL
		_ = ad.cs.Redis().Expire(kt.Ctx, targetsKey, 30*60, bedis.NX)

		logs.Infof("[upsertAsyncDownloadJob] create new job, rid: %s, biz_id: %d, job_id: %s",
			rid, bizID, jobKey)
		return jobKey, nil
	}

	// 创建失败，说明其他请求已经创建了，需要添加 target
	// 等待一小段时间，让其他请求完成创建
	time.Sleep(10 * time.Millisecond)

	// 重新检查 job 是否存在
	jobData, err = ad.cs.Redis().Get(kt.Ctx, jobKey)
	if err != nil {
		logs.Errorf("[upsertAsyncDownloadJob] redis get retry failed, rid: %s, biz_id: %d, file: %s, err: %v",
			rid, bizID, fullPath, err)
		return "", err
	}

	if jobData == "" {
		return "", fmt.Errorf("failed to find or create job, rid: %s", rid)
	}

	// 添加 target 到 List
	target := &types.AsyncDownloadTarget{
		AgentID:     targetAgentID,
		ContainerID: targetContainerID,
	}
	targetData, e := jsoni.Marshal(target)
	if e != nil {
		logs.Errorf("[upsertAsyncDownloadJob] marshal target retry failed, rid: %s, biz_id: %d, err: %v",
			rid, bizID, e)
		return "", e
	}

	if e := ad.cs.Redis().LPush(kt.Ctx, targetsKey, string(targetData)); e != nil {
		logs.Errorf("[upsertAsyncDownloadJob] redis lpush retry failed, rid: %s, biz_id: %d, err: %v",
			rid, bizID, e)
		return "", e
	}

	// 确保 targetsKey 的 TTL
	_ = ad.cs.Redis().Expire(kt.Ctx, targetsKey, 30*60, bedis.NX)

	logs.Infof("[upsertAsyncDownloadJob] add target to existing job after retry, rid: %s, biz_id: %d, job_id: %s",
		rid, bizID, jobKey)
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
