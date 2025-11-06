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
	"strconv"

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
)

// BuildProcessOperateParams 构建 ProcessOperate 的参数
type BuildProcessOperateParams struct {
	BizID             uint32            // 业务ID
	Alias             string            // 进程别名
	ProcessInstanceID uint32            // 进程实例ID
	AgentID           []string          // Agent ID列表
	GseOpType         int               // GSE操作类型
	ProcessInfo       table.ProcessInfo // 进程配置信息
}

// BuildProcessOperate 构建 GSE ProcessOperate 对象
// 查询操作（OpTypeQuery）只需要构建基本的 Meta 和 OpType 信息，不需要 Spec
// 其他操作需要完整的 Spec 信息（包括 Identity、Control、Resource、MonitorPolicy）
func BuildProcessOperate(params BuildProcessOperateParams) gse.ProcessOperate {
	// 构建基础的 ProcessOperate 对象
	processOperate := gse.ProcessOperate{
		Meta: gse.ProcessMeta{
			Namespace: gse.BuildNamespace(params.BizID),
			Name:      gse.BuildProcessName(params.Alias, params.ProcessInstanceID),
		},
		AgentIDList: params.AgentID,
		OpType:      gse.OpType(params.GseOpType),
	}

	// 查询操作不需要 Spec
	if params.GseOpType == int(gse.OpTypeQuery) {
		return processOperate
	}

	// 非查询操作需要添加完整的 Spec 信息
	processOperate.Spec = gse.ProcessSpec{
		Identity: gse.ProcessIdentity{
			ProcName:  params.Alias,
			SetupPath: params.ProcessInfo.WorkPath,
			PidPath:   params.ProcessInfo.PidFile,
			User:      params.ProcessInfo.User,
		},
		Control: gse.ProcessControl{
			StartCmd:   params.ProcessInfo.StartCmd,
			StopCmd:    params.ProcessInfo.StopCmd,
			RestartCmd: params.ProcessInfo.RestartCmd,
			ReloadCmd:  params.ProcessInfo.ReloadCmd,
			KillCmd:    params.ProcessInfo.FaceStopCmd,
		},
		Resource: gse.ProcessResource{
			CPU: DefaultCPULimit,
			Mem: DefaultMemLimit,
		},
		MonitorPolicy: gse.ProcessMonitorPolicy{
			AutoType:       gse.AutoTypePersistent,
			StartCheckSecs: DefaultStartCheckSecs,
			OpTimeout:      params.ProcessInfo.Timeout,
		},
	}

	return processOperate
}

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

		req, instMap := buildGSEOperateReq(process, insts, uint32(s.bizID))

		proc, err := s.svc.OperateProcMulti(kit.Ctx, &gse.MultiProcOperateReq{
			ProcOperateReq: req,
		})
		if err != nil {
			logs.Errorf("biz %d: operate process failed, processID=%d, err=%v", s.bizID, process.ID, err)
			continue
		}

		gseResp, err := s.svc.GetProcOperateResultV2(kit.Ctx, &gse.QueryProcResultReq{
			TaskID: proc.TaskID,
		})
		if err != nil {
			logs.Errorf("biz %d: get process result failed, taskID=%s, err=%v", s.bizID, proc.TaskID, err)
			continue
		}

		if gseResp.Code != 0 {
			logs.Errorf("biz %d: get process result failed, taskID=%s, msg=%v", s.bizID, proc.TaskID, gseResp.Message)
			continue
		}

		var result map[string]gse.ProcResult
		err = gseResp.Decode(&result)
		if err != nil {
			return err
		}

		for key, val := range result {
			inst := instMap[key]
			if inst == nil {
				logs.Warnf("biz %d: unmatched instance key: %s", s.bizID, key)
				continue
			}

			status, managed := ParseGSEProcResult(key, val)
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
	[]gse.ProcessOperate, map[string]*table.ProcessInstance) {
	req := make([]gse.ProcessOperate, 0, len(insts))
	instMap := make(map[string]*table.ProcessInstance, len(insts))

	for _, inst := range insts {
		instID, err := strconv.Atoi(inst.Spec.InstID)
		if err != nil {
			logs.Errorf("biz=%d, process_id=%d, invalid instance ID (%s): %v",
				bizID, process.ID, inst.Spec.InstID, err)
			continue
		}
		key := fmt.Sprintf("%s:%s:%s", process.Attachment.AgentID, gse.BuildNamespace(bizID),
			gse.BuildProcessName(process.Spec.Alias, uint32(instID)))
		instMap[key] = inst

		req = append(req, BuildProcessOperate(BuildProcessOperateParams{
			BizID:             bizID,
			Alias:             process.Spec.Alias,
			ProcessInstanceID: uint32(instID),
			AgentID:           []string{process.Attachment.AgentID},
			GseOpType:         gse.OpTypeQuery,
		}))
	}

	return req, instMap
}

// ParseGSEProcResult 解析 GSE 返回结果
func ParseGSEProcResult(key string, v gse.ProcResult) (status table.ProcessStatus, managed table.ProcessManagedStatus) {
	status = table.ProcessStatusStopped
	managed = table.ProcessManagedStatusUnmanaged

	switch v.ErrorCode {
	case gse.ErrCodeSuccess:
		var contents ProcessReport
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
				if i.PID < 0 {
					status = table.ProcessStatusStopping
				}
			}
		}

	case gse.ErrCodeStopping:
		var contents ProcResult
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
