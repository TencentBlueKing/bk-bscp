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
	"time"

	prm "github.com/prometheus/client_golang/prometheus"

	clientset "github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/client-set"
	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/uuid"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/pkg/runtime/jsoni"
)

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
	startTime := time.Now()
	rid := kt.Rid
	fullPath := path.Join(filePath, fileName)
	fileKey := fmt.Sprintf("AsyncDownloadJob:%d:%d:%s:*", bizID, appID, fullPath)

	// 获取外层锁（按文件路径）- 关键：记录等待锁的时间
	outerLockStart := time.Now()
	ad.redLock.Acquire(fileKey)
	outerLockWaitDuration := time.Since(outerLockStart)
	if outerLockWaitDuration > 100*time.Millisecond {
		logs.Warnf("[upsertAsyncDownloadJob] outer lock wait time long, rid: %s, biz_id: %d, file: %s, wait_duration: %v",
			rid, bizID, fullPath, outerLockWaitDuration)
	}
	defer ad.redLock.Release(fileKey)

	// 查找现有 job
	keysStart := time.Now()
	keys, err := ad.cs.Redis().Keys(kt.Ctx, fileKey)
	keysDuration := time.Since(keysStart)
	if err != nil {
		logs.Errorf("[upsertAsyncDownloadJob] redis keys failed, rid: %s, biz_id: %d, file: %s, duration: %v, err: %v",
			rid, bizID, fullPath, keysDuration, err)
		return "", err
	}
	if keysDuration > 100*time.Millisecond {
		logs.Warnf("[upsertAsyncDownloadJob] redis keys slow, rid: %s, biz_id: %d, file: %s, found_keys: %d, duration: %v",
			rid, bizID, fullPath, len(keys), keysDuration)
	}

	for _, key := range keys {
		if jobID, ok := func() (string, bool) {
			// 获取内层锁（按 job ID）- 关键：记录等待锁的时间
			innerLockStart := time.Now()
			ad.redLock.Acquire(key)
			innerLockWaitDuration := time.Since(innerLockStart)
			if innerLockWaitDuration > 100*time.Millisecond {
				logs.Warnf("[upsertAsyncDownloadJob] inner lock wait time long, rid: %s, biz_id: %d, job_key: %s, wait_duration: %v",
					rid, bizID, key, innerLockWaitDuration)
			}
			defer ad.redLock.Release(key)

			data, e := ad.cs.Redis().Get(kt.Ctx, key)
			if e != nil {
				logs.Errorf("[upsertAsyncDownloadJob] redis get failed, rid: %s, biz_id: %d, job_key: %s, err: %v",
					rid, bizID, key, e)
				return "", false
			}
			if data == "" {
				return "", false
			}
			job := &types.AsyncDownloadJob{}
			if e := jsoni.UnmarshalFromString(data, &job); e != nil {
				logs.Errorf("[upsertAsyncDownloadJob] unmarshal job failed, rid: %s, biz_id: %d, job_key: %s, err: %v",
					rid, bizID, key, e)
				return "", false
			}
			if job.Status == types.AsyncDownloadJobStatusPending {
				// pending job exists, update it
				updateStart := time.Now()
				job.Targets = append(job.Targets, &types.AsyncDownloadTarget{
					AgentID:     targetAgentID,
					ContainerID: targetContainerID,
				})
				js, e := jsoni.Marshal(job)
				if e != nil {
					logs.Errorf("[upsertAsyncDownloadJob] marshal job failed, rid: %s, biz_id: %d, job_key: %s, err: %v",
						rid, bizID, key, e)
					return "", false
				}
				if e := ad.cs.Redis().Set(kt.Ctx, key, string(js), 30*60); e != nil {
					logs.Errorf("[upsertAsyncDownloadJob] redis set failed, rid: %s, biz_id: %d, job_key: %s, err: %v",
						rid, bizID, key, e)
					return "", false
				}
				updateDuration := time.Since(updateStart)
				totalDuration := time.Since(startTime)
				logs.Infof("[upsertAsyncDownloadJob] update existing job, rid: %s, biz_id: %d, job_id: %s, targets_count: %d, update_duration: %v, total_duration: %v",
					rid, bizID, key, len(job.Targets), updateDuration, totalDuration)
				return key, true
			}
			return "", false

		}(); ok {
			logs.Infof("[upsertAsyncDownloadJob] update existing job, rid: %s, biz_id: %d, job_id: %s",
				rid, bizID, jobID)
			return jobID, nil
		}
	}

	// no pendeing job exists, create a new one
	// it's not possible to create two same job in same time, so use time stamp as unique id would be friendly.
	createStart := time.Now()
	jobID := fmt.Sprintf("AsyncDownloadJob:%d:%d:%s:%s", bizID, appID,
		fullPath, time.Now().Format("20060102150405"))
	job := &types.AsyncDownloadJob{
		JobID:         jobID,
		BizID:         bizID,
		AppID:         appID,
		FilePath:      filePath,
		FileName:      fileName,
		TargetFileDir: targetDir,
		TargetUser:    targetUser,
		FileSignature: signature,
		Targets: []*types.AsyncDownloadTarget{
			{
				AgentID:     targetAgentID,
				ContainerID: targetContainerID,
			},
		},
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
			rid, bizID, jobID, err)
		return "", err
	}

	ad.metric.jobCounter.With(prm.Labels{"biz": strconv.Itoa(int(job.BizID)),
		"app": strconv.Itoa(int(job.AppID)), "file": path.Join(job.FilePath, job.FileName),
		"targets": strconv.Itoa(len(job.Targets)), "status": job.Status}).Inc()

	err = ad.cs.Redis().Set(kt.Ctx, jobID, string(js), 30*60)
	if err != nil {
		logs.Errorf("[upsertAsyncDownloadJob] redis set new job failed, rid: %s, biz_id: %d, job_id: %s, err: %v",
			rid, bizID, jobID, err)
		return "", err
	}

	createDuration := time.Since(createStart)
	totalDuration := time.Since(startTime)
	logs.Infof("[upsertAsyncDownloadJob] create new job, rid: %s, biz_id: %d, job_id: %s, create_duration: %v, total_duration: %v (outer_lock_wait: %v, keys_duration: %v)",
		rid, bizID, jobID, createDuration, totalDuration, outerLockWaitDuration, keysDuration)

	return jobID, nil
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
