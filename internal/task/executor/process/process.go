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

package process

import (
	"encoding/json"
	"fmt"
	"time"

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"

	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	gesprocessor "github.com/TencentBlueKing/bk-bscp/internal/processor/ges"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	// CompareWithCMDBProcessInfoStepName compare with cmdb process info step name
	CompareWithCMDBProcessInfoStepName istep.StepName = "CompareWithCMDBProcessInfo"
	// CompareWithGSEProcessStatusStepName compare with gse process status step name
	CompareWithGSEProcessStatusStepName istep.StepName = "CompareWithGSEProcessStatus"
	// CompareWithGSEProcessConfigStepName compare with gse process config step name
	CompareWithGSEProcessConfigStepName istep.StepName = "CompareWithGSEProcessConfig"
	// OperateProcessStepName operate process step name
	OperateProcessStepName istep.StepName = "OperateProcess"
	// FinalizeOperateProcessStepName finalize operate process step name
	FinalizeOperateProcessStepName istep.StepName = "FinalizeOperateProcess"
)

// ProcessExecutor process step executor
// nolint: revive
type ProcessExecutor struct {
	*common.Executor
}

// NewProcessExecutor new process executor
func NewProcessExecutor(gseService *gse.Service, dao dao.Set) *ProcessExecutor {
	return &ProcessExecutor{
		Executor: &common.Executor{
			GseService: gseService,
			Dao:        dao,
		},
	}
}

// OperatePayload 进程操作负载
type OperatePayload struct {
	BizID             uint32
	OperateType       table.ProcessOperateType
	ProcessID         uint32
	ProcessInstanceID uint32
}

// CompareWithCMDBProcessInfo 对比CMDB进程信息（TODO: 待实现）
func (e *ProcessExecutor) CompareWithCMDBProcessInfo(c *istep.Context) error {
	// TODO: 实现与CMDB进程信息的对比逻辑
	logs.Infof("CompareWithCMDBProcessInfo: skip for now (TODO)")
	return nil
}

// CompareWithGSEProcessStatus 对比GSE进程状态（TODO: 待实现）
func (e *ProcessExecutor) CompareWithGSEProcessStatus(c *istep.Context) error {
	// TODO: 实现与GSE进程状态的对比逻辑
	logs.Infof("CompareWithGSEProcessStatus: skip for now (TODO)")
	return nil
}

// CompareWithGSEProcessConfig 对比GSE进程配置（TODO: 待实现）
func (e *ProcessExecutor) CompareWithGSEProcessConfig(c *istep.Context) error {
	// TODO: 实现与GSE进程配置的对比逻辑
	logs.Infof("CompareWithGSEProcessConfig: skip for now (TODO)")
	return nil
}

// TODO：保留只需要的content
// GSEProcessStatusContent GSE 返回的进程状态 content 内容结构
type GSEProcessStatusContent struct {
	IP        string             `json:"ip"`
	BkAgentID string             `json:"bk_agent_id"`
	UTCTime   string             `json:"utctime"`
	UTCTime2  string             `json:"utctime2"`
	Timezone  int                `json:"timezone"`
	Process   []GSEProcessDetail `json:"process"`
}

// GSEProcessDetail GSE 进程详情
type GSEProcessDetail struct {
	ProcName string               `json:"procname"`
	Instance []GSEProcessInstance `json:"instance"`
}

// GSEProcessInstance GSE 进程实例
type GSEProcessInstance struct {
	Cmdline       string  `json:"cmdline"`
	ProcessName   string  `json:"processName"`
	Version       string  `json:"version"`
	Health        string  `json:"health"`
	IsAuto        bool    `json:"isAuto"`          // 是否托管
	CPUUsage      float64 `json:"cpuUsage"`        // CPU 使用率
	CPUUsageAve   float64 `json:"cpuUsageAve"`     // CPU 平均使用率
	PhyMemUsage   float64 `json:"phyMemUsage"`     // 物理内存使用率
	UsePhyMem     int64   `json:"usePhyMem"`       // 使用的物理内存
	DiskSize      int64   `json:"diskSize"`        // 磁盘大小
	PID           int     `json:"pid"`             // 进程ID，小于0表示进程未运行
	StartTime     string  `json:"startTime"`       // 启动时间
	Stat          string  `json:"stat"`            // 状态
	UTime         string  `json:"utime"`           // 用户态时间
	STime         string  `json:"stime"`           // 内核态时间
	ThreadCount   int     `json:"threadCount"`     // 线程数
	ElapsedTime   int64   `json:"elapsedTime"`     // 运行时长
	RegisterTime  int64   `json:"register_time"`   // 注册时间
	LastStartTime int64   `json:"last_start_time"` // 最后启动时间
	ReportTime    int64   `json:"report_time"`     // 上报时间
}

// Operate 进程操作
func (e *ProcessExecutor) Operate(c *istep.Context) error {
	payload := &OperatePayload{}
	if err := c.GetPayload(payload); err != nil {
		return err
	}

	commonPayload := &common.ProcessPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return err
	}

	// 解析进程配置信息
	var processInfo table.ProcessInfo
	err := json.Unmarshal([]byte(commonPayload.ConfigData), &processInfo)
	if err != nil {
		return err
	}

	// 转换操作类型
	gseOpType, err := payload.OperateType.ToGSEOpType()
	if err != nil {
		return fmt.Errorf("failed to convert operate type: %w", err)
	}

	// 构建进程操作接口请求参数
	processOperate := gesprocessor.BuildProcessOperate(gesprocessor.BuildProcessOperateParams{
		BizID:             payload.BizID,
		Alias:             commonPayload.Alias,
		ProcessInstanceID: payload.ProcessInstanceID,
		AgentID:           []string{commonPayload.AgentID},
		GseOpType:         gseOpType,
		ProcessInfo:       processInfo,
	})

	items := []gse.ProcessOperate{processOperate}

	req := &gse.MultiProcOperateReq{
		ProcOperateReq: items,
	}

	resp, err := e.GseService.OperateProcMulti(c.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to operate process via gseService.OperateProcMulti: %w", err)
	}

	_, err = e.WaitTaskFinish(c.Context(), resp.TaskID,
		payload.BizID, payload.ProcessInstanceID, commonPayload.Alias, commonPayload.AgentID)
	if err != nil {
		return fmt.Errorf("failed to wait for task finish: %w", err)
	}

	return nil
}

// Finalize 进程操作完成，查询进程状态并更新进程实例状态
func (e *ProcessExecutor) Finalize(c *istep.Context) error {
	payload := &OperatePayload{}
	if err := c.GetPayload(payload); err != nil {
		return err
	}

	commonPayload := &common.ProcessPayload{}
	if err := c.GetCommonPayload(commonPayload); err != nil {
		return err
	}

	// 解析进程配置信息
	var processInfo table.ProcessInfo
	err := json.Unmarshal([]byte(commonPayload.ConfigData), &processInfo)
	if err != nil {
		return err
	}

	// 使用 OperateProcMulti 接口查询进程状态，操作码为 2（OpTypeQuery）
	processOperate := gesprocessor.BuildProcessOperate(gesprocessor.BuildProcessOperateParams{
		BizID:             payload.BizID,
		Alias:             commonPayload.Alias,
		ProcessInstanceID: payload.ProcessInstanceID,
		AgentID:           []string{commonPayload.AgentID},
		GseOpType:         int(gse.OpTypeQuery),
		ProcessInfo:       processInfo,
	})

	req := &gse.MultiProcOperateReq{
		ProcOperateReq: []gse.ProcessOperate{processOperate},
	}

	resp, err := e.GseService.OperateProcMulti(c.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to query process status via gseService.OperateProcMulti: %w", err)
	}
	// 等待查询任务完成
	result, err := e.WaitTaskFinish(c.Context(),
		resp.TaskID, payload.BizID, payload.ProcessInstanceID, commonPayload.Alias, commonPayload.AgentID)
	if err != nil {
		return fmt.Errorf("failed to wait for query task finish: %w", err)
	}

	// 构建 GSE 返回结果的 key
	key := gse.BuildResultKey(commonPayload.AgentID, payload.BizID, commonPayload.Alias, payload.ProcessInstanceID)
	logs.Infof("Finalize key: %s", key)
	procResult, ok := result[key]
	if !ok {
		return fmt.Errorf("process result not found for key: %s", key)
	}

	// 检查查询操作是否成功
	if !gse.IsSuccess(procResult.ErrorCode) {
		// TODO: 后续需要处理查询进程状态失败情况，比如查询不到进程状态，或者查询进程状态失败，需要回滚状态
		return fmt.Errorf("failed to query process status, errorCode=%d, errorMsg=%s",
			procResult.ErrorCode, procResult.ErrorMsg)
	}

	// 解析 content 获取进程状态
	var statusContent GSEProcessStatusContent
	if err = json.Unmarshal([]byte(procResult.Content), &statusContent); err != nil {
		return fmt.Errorf("failed to unmarshal process status content: %w", err)
	}

	// 默认状态为停止和未托管
	processStatus := table.ProcessStatusStopped
	managedStatus := table.ProcessManagedStatusUnmanaged

	// 从 process 数组中提取进程实例信息
	if len(statusContent.Process) > 0 && len(statusContent.Process[0].Instance) > 0 {
		instance := statusContent.Process[0].Instance[0]

		// 根据 PID 判断进程运行状态：PID < 0 表示进程未运行
		if instance.PID > 0 {
			processStatus = table.ProcessStatusRunning
		} else {
			processStatus = table.ProcessStatusStopped
		}

		// 根据 isAuto 判断托管状态
		if instance.IsAuto {
			managedStatus = table.ProcessManagedStatusManaged
		} else {
			managedStatus = table.ProcessManagedStatusUnmanaged
		}
	}

	// 获取并更新进程实例
	processInstance, err := e.Dao.ProcessInstance().GetByID(kit.New(), payload.BizID, payload.ProcessInstanceID)
	if err != nil {
		return fmt.Errorf("failed to get process instance: %w", err)
	}

	// 更新状态字段
	processInstance.Spec.Status = processStatus
	processInstance.Spec.ManagedStatus = managedStatus
	processInstance.Spec.StatusUpdatedAt = time.Now()

	if err = e.Dao.ProcessInstance().Update(kit.New(), processInstance); err != nil {
		return fmt.Errorf("failed to update process instance: %w", err)
	}

	return nil
}

// RegisterExecutor register executor
func RegisterExecutor(e *ProcessExecutor) {
	// 注册前置检查步骤
	istep.Register(CompareWithCMDBProcessInfoStepName, istep.StepExecutorFunc(e.CompareWithCMDBProcessInfo))
	istep.Register(CompareWithGSEProcessStatusStepName, istep.StepExecutorFunc(e.CompareWithGSEProcessStatus))
	istep.Register(CompareWithGSEProcessConfigStepName, istep.StepExecutorFunc(e.CompareWithGSEProcessConfig))

	// 注册主要执行步骤
	istep.Register(OperateProcessStepName, istep.StepExecutorFunc(e.Operate))
	istep.Register(FinalizeOperateProcessStepName, istep.StepExecutorFunc(e.Finalize))
}
