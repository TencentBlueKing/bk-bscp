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

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"

	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

const (
	// RegisterStepName register step name
	OperateStepName istep.StepName = "Operate"
)

// ProcessExecutor process step executor
// nolint: revive
type ProcessExecutor struct {
	*common.Executor
	gseService *gse.Service
	dao        dao.Set
}

// NewProcessExecutor new process executor
func NewProcessExecutor(gseService *gse.Service, dao dao.Set) *ProcessExecutor {
	return &ProcessExecutor{
		Executor:   common.NewExecutor(gseService),
		gseService: gseService,
		dao:        dao,
	}
}

// OperatePayload 进程操作负载
type OperatePayload struct {
	OperateType       table.ProcessOperateType
	ProcessID         uint32
	ProcessInstanceID uint32
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

	items := make([]gse.ProcessOperate, 0)

	hosts := make([]gse.HostInfo, 0)

	var processInfo table.ProcessInfo

	err := json.Unmarshal([]byte(commonPayload.ConfigData), &processInfo)
	if err != nil {
		return err
	}

	hosts = append(hosts, gse.HostInfo{
		IP:        commonPayload.InnerIP,
		BkCloudID: commonPayload.CloudID,
	})

	var autoType int

	items = append(items, gse.ProcessOperate{
		Meta: gse.ProcessMeta{
			Namespace: "bscp",
			Name:      commonPayload.Alias,
			Labels:    map[string]string{"env": commonPayload.Environment},
		},
		AgentIDList: []string{commonPayload.AgentID},
		Hosts:       hosts,
		OpType:      0,
		Spec: gse.ProcessSpec{
			Identity: gse.ProcessIdentity{
				ProcName:  commonPayload.Alias,
				SetupPath: processInfo.WorkPath,
				PidPath:   processInfo.PidFile,
				User:      processInfo.User,
			},
			Control: gse.ProcessControl{
				StartCmd:   processInfo.StartCmd,
				StopCmd:    processInfo.StopCmd,
				RestartCmd: processInfo.RestartCmd,
				ReloadCmd:  processInfo.ReloadCmd,
				KillCmd:    processInfo.FaceStopCmd,
			},
			Resource: gse.ProcessResource{
				CPU: 30.0,
				Mem: 10.0,
			},
			MonitorPolicy: gse.ProcessMonitorPolicy{
				AutoType:  autoType,
				OpTimeout: processInfo.Timeout,
			},
		},
	})

	req := &gse.MultiProcOperateReq{
		ProcOperateReq: items,
	}

	resp, err := e.gseService.OperateProcMulti(c.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to operate process via gseService.OperateProcMulti: %w", err)
	}

	taskResult, err := e.WaitTaskFinish(c.Context(), resp.TaskID, []string{commonPayload.AgentID})
	if err != nil {
		return err
	}

	// TODO:更新任务状态
	fmt.Println(taskResult)

	return nil
}

// RegisterExecutor register executor
func RegisterExecutor(e *ProcessExecutor) {
	istep.Register(OperateStepName, istep.StepExecutorFunc(e.Operate))
}
