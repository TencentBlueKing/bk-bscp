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
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	prm "github.com/prometheus/client_golang/prometheus"

	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/internal/components/bcs"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/repository"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/pkg/runtime/jsoni"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
)

var (
	// JobTimeoutSeconds is the timeout seconds for async download job
	JobTimeoutSeconds = 10 * 60
)

// Scheduler scheduled task to process download jobs
// GSECreateTaskFunc 定义 GSE 创建任务的函数类型，用于依赖注入和测试
type GSECreateTaskFunc func(ctx context.Context, sourceAgentID, sourceContainerID, sourceFileDir, sourceUser,
	filename string, targetFileDir string, targetsAgents []gse.TransferFileAgent) (string, error)

type Scheduler struct {
	ctx               context.Context
	cancel            context.CancelFunc
	bds               bedis.Client
	redLock           *lock.RedisLock
	fileLock          *lock.FileLock
	provider          repository.Provider
	serverAgentID     string
	serverContainerID string
	metric            *metric
	// 以下字段用于测试和依赖注入，如果为空则使用默认值（从 cc.FeedServer() 读取）
	cacheDir          string            // 缓存目录，如果为空则使用 cc.FeedServer().GSE.CacheDir
	agentUser         string            // Agent 用户，如果为空则使用 cc.FeedServer().GSE.AgentUser
	gseCreateTaskFunc GSECreateTaskFunc // GSE 创建任务函数，如果为空则使用 gse.CreateTransferFileTask
}

// NewScheduler create a async download scheduler
func NewScheduler(mc *metric, redLock *lock.RedisLock) (*Scheduler, error) {
	ctx, cancel := context.WithCancel(context.Background())
	bds, err := bedis.NewRedisCache(cc.FeedServer().RedisCluster)
	if err != nil {
		cancel()
		return nil, err
	}
	// set ttl to 60 seconds, cause the job include downloading which may cost a lot of time
	fileLock := lock.NewFileLock()
	provider, err := repository.NewProvider(cc.FeedServer().Repository)
	if err != nil {
		cancel()
		return nil, err
	}

	// bcs-watch report pod/container data may delay, so retry to get server agent id and container id
	retry := tools.NewRetryPolicy(5, [2]uint{3000, 5000})

	var serverAgentID, serverContainerID string
	var lastErr error
	for {
		select {
		case <-ctx.Done():
			cancel()
			return nil, fmt.Errorf("get server agent id and container id failed, err, %s", ctx.Err().Error())
		default:
		}

		if retry.RetryCount() == 5 {
			cancel()
			return nil, lastErr
		}

		serverAgentID, serverContainerID, lastErr = getAsyncDownloadServerInfo(ctx, cc.FeedServer().GSE)
		if lastErr != nil {
			retry.Sleep()
			continue
		}
		break
	}

	logs.Infof("server agent id: %s, server container id: %s", serverAgentID, serverContainerID)
	return &Scheduler{
		ctx:               ctx,
		cancel:            cancel,
		bds:               bds,
		redLock:           redLock,
		fileLock:          fileLock,
		provider:          provider,
		serverAgentID:     serverAgentID,
		serverContainerID: serverContainerID,
		metric:            mc,
		// cacheDir, agentUser, gseCreateTaskFunc 保持为空，使用默认值
	}, nil
}

// Run run a scheduled task
func (a *Scheduler) Run() {
	// 注册shutdown notifier
	notifier := shutdown.AddNotifier()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				a.do()
			case <-a.ctx.Done():
				logs.Infof("async downloader stopped")
				return
			case <-notifier.Signal:
				// 收到shutdown信号，显式停止ticker避免资源泄漏，然后等待旧数据格式的job处理完成
				ticker.Stop()
				logs.Infof("received shutdown signal, waiting for old format jobs to complete...")
				a.waitForOldFormatJobsComplete()
				notifier.Done()
				return
			}
		}
	}()
}

// Stop stop scheduled task
func (a *Scheduler) Stop() {
	a.cancel()
	// 等待旧数据格式的job处理完成
	a.waitForOldFormatJobsComplete()
}

// waitForOldFormatJobsComplete 等待旧数据格式的job处理完成
func (a *Scheduler) waitForOldFormatJobsComplete() {
	logs.Infof("checking for old format jobs (with Targets in job struct)...")

	maxWaitTime := 5 * time.Minute // 最大等待时间
	checkInterval := 5 * time.Second
	startTime := time.Now()

	for {
		// 检查是否超时
		if time.Since(startTime) > maxWaitTime {
			logs.Warnf("wait for old format jobs timeout after %v", maxWaitTime)
			return
		}

		// 查找所有job
		keys, err := a.bds.Keys(a.ctx, "AsyncDownloadJob:*")
		if err != nil {
			logs.Errorf("list async download job keys failed, err: %s", err.Error())
			return
		}

		oldFormatJobs := make([]string, 0)
		for _, key := range keys {
			// 过滤掉 targets keys，只处理 job keys
			if strings.Contains(key, "AsyncDownloadJob:Targets:") {
				continue
			}

			data, err := a.bds.Get(a.ctx, key)
			if err != nil {
				continue
			}
			if data == "" {
				continue
			}

			job := &types.AsyncDownloadJob{}
			if err = jsoni.Unmarshal([]byte(data), job); err != nil {
				continue
			}

			// 检查是否是旧格式：job结构体中有Targets且Redis List为空
			// 使用从 JobID 解析的 targetsKey（支持新旧格式）
			targetsKey := GetTargetsKeyFromJobKey(job.JobID)
			count, err := a.bds.LLen(a.ctx, targetsKey)

			// 旧格式判断：job.Targets不为空 且 Redis List为空或不存在
			if len(job.Targets) > 0 && (err != nil || count == 0) {
				// 检查job状态，只等待pending或running状态的job
				if job.Status == types.AsyncDownloadJobStatusPending ||
					job.Status == types.AsyncDownloadJobStatusRunning {
					oldFormatJobs = append(oldFormatJobs, job.JobID)
				}
			}
		}

		// 如果没有旧格式的job，退出
		if len(oldFormatJobs) == 0 {
			logs.Infof("all old format jobs have been processed")
			return
		}

		logs.Infof("waiting for %d old format jobs to complete: %v", len(oldFormatJobs), oldFormatJobs)
		time.Sleep(checkInterval)
	}
}

func (a *Scheduler) do() {

	keys, err := a.bds.Keys(a.ctx, "AsyncDownloadJob:*")
	if err != nil {
		logs.Errorf("list async download job keys from redis failed, err: %s", err.Error())
		return
	}

	for _, key := range keys {
		// 过滤掉 targets keys，只处理 job keys
		if strings.Contains(key, "AsyncDownloadJob:Targets:") {
			continue
		}
		if err := func() error {
			// lock by job to prevent from
			// 1. concurrency writing in api AsyncDownload
			// 2. concurrency writing in other feedserver instance cronjob
			if a.redLock.TryAcquire(key) {
				defer a.redLock.Release(key)
				data, err := a.bds.Get(a.ctx, key)
				if err != nil {
					return err
				}
				if data == "" {
					return nil
				}
				job := &types.AsyncDownloadJob{}
				if err := jsoni.Unmarshal([]byte(data), job); err != nil {
					logs.Errorf("unmarshal async download job failed, job_id: %s, err: %v", key, err)
					return err
				}
				switch job.Status {
				case types.AsyncDownloadJobStatusPending:
					// 根据时间窗口判断是否可以开始处理
					// 只有当时间窗口结束后才开始处理，这样可以收集更多的 targets
					if !IsTimeWindowExpired(job.JobID) {
						// 时间窗口未结束，继续收集 targets
						return nil
					}
					return a.handleDownload(job)
				case types.AsyncDownloadJobStatusRunning:
					return a.checkJobStatus(job)
				case types.AsyncDownloadJobStatusSuccess,
					types.AsyncDownloadJobStatusFailed,
					types.AsyncDownloadJobStatusTimeout:
					return nil
				default:
					logs.Errorf("invalid async download job status: %s", job.Status)
				}
			}
			return nil
		}(); err != nil {
			logs.Errorf("handle async download job %s failed, err: %s", key, err.Error())
		}
	}
}

// nolint:funlen
func (a *Scheduler) handleDownload(job *types.AsyncDownloadJob) error {
	logs.Infof("handle async download job %s, biz_id: %d, app_id: %d", job.JobID, job.BizID, job.AppID)
	kt := kit.New()
	kt.BizID = job.BizID
	kt.AppID = job.AppID

	// 1. 更新任务状态
	job.Status = types.AsyncDownloadJobStatusRunning
	job.ExecuteTime = time.Now()
	if err := a.updateAsyncDownloadJobStatus(a.ctx, job); err != nil {
		return err
	}

	// 2. 只从Redis List读取Targets（使用从 JobID 解析的 targetsKey）
	targetsKey := GetTargetsKeyFromJobKey(job.JobID)
	targetsData, err := a.bds.LRange(a.ctx, targetsKey, 0, -1)
	if err != nil {
		return fmt.Errorf("read targets from redis list failed, job_id: %s, err: %v", job.JobID, err)
	}

	// 如果Redis List为空，记录警告。可能原因：
	// 1）该任务为旧格式数据（targets 未拆分存储到 Redis List）；
	// 2）新格式任务中 SetNX 成功但后续 LPush 失败，导致任务处于不一致状态。
	if len(targetsData) == 0 {
		logs.Warnf("targets list is empty for job %s, this may be legacy (old-format) job data "+
			"or a partially created new-format job where LPush to Redis failed after SetNX; "+
			"please verify Redis keys for this job and consider retrying or recreating the job", job.JobID)
		return fmt.Errorf("targets list is empty for job %s: job data may be legacy or inconsistent; "+
			"check Redis for this job and retry or recreate the job", job.JobID)
	}

	// 解析targets
	targets := make([]*types.AsyncDownloadTarget, 0, len(targetsData))
	for _, targetData := range targetsData {
		target := &types.AsyncDownloadTarget{}
		if err = jsoni.Unmarshal([]byte(targetData), target); err != nil {
			logs.Errorf("unmarshal target failed, job_id: %s, err: %v", job.JobID, err)
			continue
		}
		targets = append(targets, target)
	}

	if len(targets) == 0 {
		return fmt.Errorf("no valid targets found for job %s", job.JobID)
	}

	// 3. 下载文件到本地
	// 使用注入的 cacheDir，如果为空则使用默认值
	cacheDir := a.cacheDir
	if cacheDir == "" {
		cacheDir = cc.FeedServer().GSE.CacheDir
	}
	sourceDir := path.Join(cacheDir, strconv.Itoa(int(job.BizID)))
	if err = os.MkdirAll(sourceDir, os.ModePerm); err != nil {
		return err
	}
	// filepath = source/{biz_id}/{sha256}
	signature := job.FileSignature
	serverFilePath := path.Join(sourceDir, signature)
	if err = a.checkAndDownloadFile(kt, serverFilePath, signature); err != nil {
		return err
	}

	// 4. 创建GSE文件传输任务
	targetAgents := make([]gse.TransferFileAgent, 0, len(targets))
	for _, target := range targets {
		targetAgents = append(targetAgents, gse.TransferFileAgent{
			BkAgentID:     target.AgentID,
			BkContainerID: target.ContainerID,
			User:          job.TargetUser,
		})
	}

	// 使用注入的 agentUser，如果为空则使用默认值
	agentUser := a.agentUser
	if agentUser == "" {
		agentUser = cc.FeedServer().GSE.AgentUser
	}

	// 使用注入的 gseCreateTaskFunc，如果为空则使用默认的 gse.CreateTransferFileTask
	createTaskFunc := a.gseCreateTaskFunc
	if createTaskFunc == nil {
		createTaskFunc = gse.CreateTransferFileTask
	}

	taskID, err := createTaskFunc(a.ctx, a.serverAgentID, a.serverContainerID, sourceDir,
		agentUser, signature, job.TargetFileDir, targetAgents)
	if err != nil {
		return fmt.Errorf("create gse transfer file task failed, %s", err.Error())
	}

	// 5. 更新任务状态
	job.GSETaskID = taskID

	if err := a.updateAsyncDownloadJobStatus(a.ctx, job); err != nil {
		return err
	}

	return nil
}

// updateAsyncDownloadJobStatus update async download job status to redis
// ! make sure job must be locked by upper caller to avoid concurrency update
func (a *Scheduler) updateAsyncDownloadJobStatus(ctx context.Context, job *types.AsyncDownloadJob) error {
	data, err := a.bds.Get(ctx, job.JobID)
	if err != nil {
		return err
	}
	if data == "" {
		logs.Errorf("update asyncdownload job %s status failed, not found in redis", job.JobID)
		return nil
	}

	old := new(types.AsyncDownloadJob)
	if err = jsoni.UnmarshalFromString(data, old); err != nil {
		return err
	}

	// 只从Redis List获取targets数量（使用从 JobID 解析的 targetsKey）
	targetsKey := GetTargetsKeyFromJobKey(job.JobID)
	targetsCount, err := a.bds.LLen(ctx, targetsKey)
	if err != nil {
		// 如果获取失败，记录错误日志并使用0（可能是旧数据），避免静默失败导致监控指标误导
		logs.Errorf("update asyncdownload job %s status: failed to get targets count from redis, key: %s, err: %v",
			job.JobID, targetsKey, err)
		targetsCount = 0
	}

	if old.Status != job.Status {
		var duration float64
		if old.Status == types.AsyncDownloadJobStatusPending {
			duration = time.Since(job.CreateTime).Seconds()
		} else {
			duration = time.Since(job.ExecuteTime).Seconds()
		}
		logs.Infof("update asyncdownload job %s status from %s to %s, duration: %f, app: %d, fileName: %s",
			job.JobID, old.Status, job.Status, duration, job.AppID, job.FileName)
		a.metric.jobDurationSeconds.With(prm.Labels{"biz": strconv.Itoa(int(job.BizID)),
			"app": strconv.Itoa(int(job.AppID)), "file": path.Join(job.FilePath, job.FileName),
			"targets": strconv.Itoa(int(targetsCount)), "status": old.Status}).
			Observe(duration)
		a.metric.jobCounter.With(prm.Labels{"biz": strconv.Itoa(int(job.BizID)),
			"app": strconv.Itoa(int(job.AppID)), "file": path.Join(job.FilePath, job.FileName),
			"targets": strconv.Itoa(int(targetsCount)), "status": job.Status}).Inc()
	}

	js, err := jsoni.Marshal(job)
	if err != nil {
		return err
	}
	return a.bds.Set(ctx, job.JobID, string(js), 30*60)
}

func (a *Scheduler) checkAndDownloadFile(kt *kit.Kit, filePath, signature string) error {
	// block until file download to avoid repeat download from another job
	a.fileLock.Acquire(filePath)
	defer a.fileLock.Release(filePath)
	if _, iErr := os.Stat(filePath); iErr != nil {
		if !os.IsNotExist(iErr) {
			return iErr
		}
		// not exists in feed server, download to local disk
		file, iErr := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
		if iErr != nil {
			return iErr
		}
		defer file.Close()

		reader, _, iErr := a.provider.Download(kt, signature)
		if iErr != nil {
			return iErr
		}
		defer reader.Close()
		if _, e := io.Copy(file, reader); e != nil {
			return e
		}
		if e := file.Sync(); e != nil {
			return e
		}
	}
	return nil
}

func getAsyncDownloadServerInfo(ctx context.Context, gseConf cc.GSE) (
	agentID string, containerID string, err error) {
	if gseConf.NodeAgentID != "" {
		// if serverAgentID configured, it measn feed server was deployed in binary mode, source is node
		agentID = gseConf.NodeAgentID
		return agentID, "", nil
	}
	// if serverAgentID not configured, it means feed server was deployed in container mode, source is container
	if gseConf.ClusterID == "" || gseConf.PodID == "" {
		return "", "", fmt.Errorf("server agent_id or (cluster_id and pod_id is required")
	}
	pod, qErr := bcs.QueryPod(ctx, gseConf.ClusterID, gseConf.PodID)
	if qErr != nil {
		return "", "", qErr
	}
	for _, container := range pod.Status.ContainerStatuses {
		if container.Name == gseConf.ContainerName {
			containerID = tools.SplitContainerID(container.ContainerID)
		}
	}
	if containerID == "" {
		return "", "", fmt.Errorf("server container %s not found in pod %s/%s",
			gseConf.ContainerName, gseConf.ClusterID, gseConf.PodID)
	}
	node, qErr := bcs.QueryNode(ctx, gseConf.ClusterID, pod.Spec.NodeName)
	if qErr != nil {
		return "", "", qErr
	}
	agentID = node.Labels[constant.LabelKeyAgentID]
	if agentID == "" {
		return "", "", fmt.Errorf("bk-agent-id not found in server node %s/%s", gseConf.ClusterID, pod.Spec.NodeName)
	}
	return agentID, containerID, nil
}

func (a *Scheduler) checkJobStatus(job *types.AsyncDownloadJob) error {

	if err := a.updateJobTargetsStatus(job); err != nil {
		return err
	}

	// 只从Redis List获取targets数量（使用从 JobID 解析的 targetsKey）
	targetsKey := GetTargetsKeyFromJobKey(job.JobID)
	targetsCount, err := a.bds.LLen(a.ctx, targetsKey)
	if err != nil {
		return err
	}

	// if all targets are success, then the entire job is success
	if len(job.SuccessTargets) == int(targetsCount) {
		job.Status = types.AsyncDownloadJobStatusSuccess
		if err := a.updateAsyncDownloadJobStatus(a.ctx, job); err != nil {
			return err
		}
		return nil
	}
	// if all targets are in a final status and there is a failed target, then the entire job is failed
	if len(job.SuccessTargets)+len(job.FailedTargets) == int(targetsCount) {
		job.Status = types.AsyncDownloadJobStatusFailed
		if err := a.updateAsyncDownloadJobStatus(a.ctx, job); err != nil {
			return err
		}
		return nil
	}
	// if the entire job is not finished, check if timeout
	// if time out, set all the downloading status target to timeout
	if time.Since(job.ExecuteTime) > time.Duration(JobTimeoutSeconds)*time.Second {
		job.Status = types.AsyncDownloadJobStatusTimeout
		for k, v := range job.DownloadingTargets {
			job.TimeoutTargets[k] = v
		}
		for k := range job.DownloadingTargets {
			delete(job.DownloadingTargets, k)
		}
		if err := a.updateAsyncDownloadJobStatus(a.ctx, job); err != nil {
			return err
		}

		// TODO: need to check cancel gse task status ?
		timeoutTargets := make([]gse.TransferFileAgent, 0, len(job.TimeoutTargets))
		for _, content := range job.TimeoutTargets {
			timeoutTargets = append(timeoutTargets, gse.TransferFileAgent{
				BkAgentID:     content.DestAgentID,
				BkContainerID: content.DestContainerID,
			})
		}
		if len(timeoutTargets) > 0 && job.GSETaskID != "" {
			if _, err := gse.TerminateTransferFileTask(a.ctx, job.GSETaskID, timeoutTargets); err != nil {
				logs.Errorf("cancel timeout transfer file task %s failed, gse_task_id: %s, err: %s",
					job.JobID, job.GSETaskID, err.Error())
			}
		}
		return nil
	}

	// if the entire job is not finished and not timeout, continue to update job status
	// so that downloading status target can be updated
	return a.updateAsyncDownloadJobStatus(a.ctx, job)
}

func (a Scheduler) updateJobTargetsStatus(job *types.AsyncDownloadJob) error {
	gseTaskResults, err := gse.TransferFileResult(a.ctx, job.GSETaskID)
	if err != nil {
		return err
	}

	// 只从Redis List读取targets（使用从 JobID 解析的 targetsKey）
	targetsKey := GetTargetsKeyFromJobKey(job.JobID)
	targetsData, err := a.bds.LRange(a.ctx, targetsKey, 0, -1)
	if err != nil {
		return err
	}

	targets := make([]*types.AsyncDownloadTarget, 0, len(targetsData))
	for _, targetData := range targetsData {
		target := &types.AsyncDownloadTarget{}
		if err := jsoni.Unmarshal([]byte(targetData), target); err != nil {
			continue
		}
		targets = append(targets, target)
	}

	// ! make sure that success + failed + downloading + timeout = all targets
	// success/failed/timeout is the final status, downloading is the intermediate status
	// so when set a target as success/failed/timeout, need to delete if from downloading list
	for _, result := range gseTaskResults {
		// upload result would not append to the targets list
		// if upload task failed, set all the task to failed
		// case in gse, if upload failed, all the download tasks must be failed
		if result.Content.Type == "upload" {
			if result.ErrorCode != 0 && result.ErrorCode != 115 {
				for k := range job.SuccessTargets {
					delete(job.SuccessTargets, k)
				}
				for k := range job.FailedTargets {
					delete(job.FailedTargets, k)
				}
				for k := range job.DownloadingTargets {
					delete(job.DownloadingTargets, k)
				}
				for k := range job.TimeoutTargets {
					delete(job.TimeoutTargets, k)
				}
				for _, target := range targets {
					job.FailedTargets[fmt.Sprintf("%s:%s", target.AgentID, target.ContainerID)] = result.Content
				}
			}
		} else {
			// only download task would append to the targets list
			if result.ErrorCode == 0 {
				logs.Infof("download success, jobID: %s, file: %s, agentID: %s, containerID: %s",
					job.JobID, result.Content.DestFileName, result.Content.DestAgentID, result.Content.DestContainerID)
				job.SuccessTargets[fmt.Sprintf("%s:%s", result.Content.DestAgentID, result.Content.DestContainerID)] =
					result.Content
				delete(job.DownloadingTargets,
					fmt.Sprintf("%s:%s", result.Content.DestAgentID, result.Content.DestContainerID))
			} else if result.ErrorCode == 115 {
				logs.Infof("download in progress, jobID: %s, file: %s, agentID: %s, containerID: %s",
					job.JobID, result.Content.DestFileName, result.Content.DestAgentID, result.Content.DestContainerID)
				// If the result is 115 downloading state
				job.DownloadingTargets[fmt.Sprintf("%s:%s", result.Content.DestAgentID, result.Content.DestContainerID)] =
					result.Content
			} else {
				logs.Errorf("download failed, jobID: %s, file: %s, agentID: %s, containerID: %s, errorCode: %d",
					job.JobID, result.Content.DestFileName, result.Content.DestAgentID,
					result.Content.DestContainerID, result.ErrorCode)
				// other error code means failed
				job.FailedTargets[fmt.Sprintf("%s:%s", result.Content.DestAgentID, result.Content.DestContainerID)] =
					result.Content
				delete(job.DownloadingTargets,
					fmt.Sprintf("%s:%s", result.Content.DestAgentID, result.Content.DestContainerID))
			}
		}
	}
	return nil
}
