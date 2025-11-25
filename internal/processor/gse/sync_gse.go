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

// Package gse provides gse service.
package gse

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	// DefaultCPULimit 默认 CPU 使用率上限百分比
	DefaultCPULimit = 30.0

	// DefaultMemLimit 默认内存使用率上限百分比
	DefaultMemLimit = 10.0

	// DefaultStartCheckSecs 默认启动后检查存活的时间（秒）
	DefaultStartCheckSecs = 5

	defaultMaxWait  = 30 * time.Second
	defaultInterval = 2 * time.Second
)

// NewSyncGESService 初始化同步gse
func NewSyncGESService(bizID int, svc *gse.Service, dao dao.Set) *syncGSEService {
	return &syncGSEService{
		bizID: bizID,
		svc:   svc,
		dao:   dao,
	}
}

// syncGSEService 同步gse
type syncGSEService struct {
	bizID int
	svc   *gse.Service
	dao   dao.Set
}

// SyncSingleBiz 同步gse状态
// 1. 按业务获取进程数据
// 2. 调用gse接口
func (s *syncGSEService) SyncSingleBiz(ctx context.Context) error {
	kit := kit.FromGrpcContext(ctx)
	processes, err := s.dao.Process().ListActiveProcesses(kit, uint32(s.bizID))
	if err != nil {
		logs.Errorf("list active processes failed: %v", err)
		return err
	}
	if len(processes) == 0 {
		logs.Infof("no active processes found, skip sync")
		return nil
	}

	for _, process := range processes {
		// 查询实例表
		insts, err := s.dao.ProcessInstance().GetByProcessIDs(kit, uint32(s.bizID), []uint32{process.ID})
		if err != nil {
			logs.Errorf("biz %d: get process instances failed, processID=%d, err=%v", s.bizID, process.ID, err)
			continue
		}

		req, instMap, err := buildGSEOperateReq(process, insts, uint32(s.bizID))
		if err != nil {
			logs.Errorf("biz %d: build GSE operate request failed, processID=%d, err=%v", s.bizID, process.ID, err)
			continue
		}

		proc, err := s.svc.OperateProcMulti(kit.Ctx, &gse.MultiProcOperateReq{
			ProcOperateReq: req,
		})
		if err != nil {
			logs.Errorf("biz %d: operate process failed, processID=%d, err=%v", s.bizID, process.ID, err)
			continue
		}

		result, err := waitForProcResult(kit.Ctx, s.svc, proc.TaskID, defaultMaxWait, defaultInterval)
		if err != nil {
			logs.Errorf("biz %d: wait for process result failed, taskID=%s, err=%v", s.bizID, proc.TaskID, err)
			continue
		}

		for key, val := range result {
			inst := instMap[key]
			if inst == nil {
				logs.Warnf("biz %d: unmatched instance key: %s", s.bizID, key)
				continue
			}

			status, managed := parseGSEProcResult(key, val)
			inst.Spec.Status = status
			inst.Spec.ManagedStatus = managed

			if err := s.dao.ProcessInstance().Update(kit, inst); err != nil {
				logs.Errorf("biz %d: update instance failed for key=%s, err=%v", s.bizID, key, err)
			}
		}

	}

	return nil
}

func buildGSEOperateReq(process *table.Process, insts []*table.ProcessInstance, bizID uint32) (
	[]gse.ProcessOperate, map[string]*table.ProcessInstance, error) {
	req := make([]gse.ProcessOperate, 0, len(insts))
	instMap := make(map[string]*table.ProcessInstance, len(insts))
	var processInfo table.ProcessInfo
	if err := json.Unmarshal([]byte(process.Spec.SourceData), &processInfo); err != nil {
		return nil, nil, fmt.Errorf("unmarshal process source data failed: %w", err)
	}
	for _, inst := range insts {
		key := gse.BuildResultKey(process.Attachment.AgentID, bizID, process.Spec.Alias, inst.Spec.HostInstSeq)
		instMap[key] = inst
		processOperate, err := BuildProcessOperate(BuildProcessOperateParams{
			BizID:         bizID,
			Alias:         process.Spec.Alias,
			FuncName:      process.Spec.FuncName,
			AgentID:       []string{process.Attachment.AgentID},
			HostInstSeq:   inst.Spec.HostInstSeq,
			ModuleInstSeq: inst.Spec.ModuleInstSeq,
			SetName:       process.Spec.SetName,
			ModuleName:    process.Spec.ModuleName,
			GseOpType:     gse.OpTypeQuery,
			ProcessInfo:   processInfo,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("build process operate failed: %w", err)
		}
		req = append(req, *processOperate)
	}

	return req, instMap, nil
}

// parseGSEProcResult 解析 GSE 返回结果
func parseGSEProcResult(key string, v gse.ProcResult) (status table.ProcessStatus, managed table.ProcessManagedStatus) {
	status = table.ProcessStatusStopped
	managed = table.ProcessManagedStatusUnmanaged

	switch v.ErrorCode {
	case gse.ErrCodeSuccess:
		var contents gse.ProcessStatusContent
		if err := json.Unmarshal([]byte(v.Content), &contents); err != nil {
			logs.Warnf("unmarshal success content failed for %s: %v", key, err)
			return status, managed
		}
		status = table.ProcessStatusStarting
		for _, p := range contents.Process {
			for _, i := range p.Instance {
				if i.IsAuto {
					managed = table.ProcessManagedStatusManaged
				}
				if i.PID > 0 {
					status = table.ProcessStatusRunning
				}
				// 启动失败了
				if i.PID < 0 {
					status = table.ProcessStatusStopped
				}
			}
		}

	case gse.ErrCodeStopping:
		var contents gse.StoppingContent
		if err := json.Unmarshal([]byte(v.Content), &contents); err != nil {
			logs.Warnf("unmarshal stopping content failed for %s: %v", key, err)
			return status, managed
		}
		status = table.ProcessStatusStopping
		for _, c := range contents.Value {
			if c.IsAuto {
				managed = table.ProcessManagedStatusManaged
			}
		}

	case gse.ErrCodeInProgress:
		status = table.ProcessStatusStarting

	default:
		logs.Warnf("key=%s, unknown gse code: %d", key, v.ErrorCode)
	}

	return status, managed
}

func waitForProcResult(ctx context.Context, svc *gse.Service, taskID string, maxWait, interval time.Duration) (map[string]gse.ProcResult, error) {
	var result map[string]gse.ProcResult

	err := wait.PollUntilContextTimeout(
		ctx,
		interval,
		maxWait,
		true,
		func(ctx context.Context) (bool, error) {
			resp, err := svc.GetProcOperateResultV2(ctx, &gse.QueryProcResultReq{TaskID: taskID})
			if err != nil {
				return false, fmt.Errorf("get process result failed, taskID=%s, err=%v", taskID, err)
			}

			if resp.Code != 0 {
				return false, fmt.Errorf("gse API error, code=%d, msg=%s", resp.Code, resp.Message)
			}

			// 解码结果
			if err := resp.Decode(&result); err != nil {
				return false, fmt.Errorf("decode gse result failed, taskID=%s, err=%v", taskID, err)
			}

			// 检查是否还有实例在进行中
			for _, proc := range result {
				if gse.IsInProgress(proc.ErrorCode) { // 进程仍在执行中
					return false, nil // 继续轮询
				}
			}

			return true, nil // 全部完成
		},
	)

	if err != nil {
		if wait.Interrupted(err) || err == context.DeadlineExceeded || err == context.Canceled {
			return nil, fmt.Errorf("timeout or canceled waiting for GSE result, taskID=%s", taskID)
		}
		return nil, err
	}

	return result, nil
}
