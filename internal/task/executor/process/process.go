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
	// TODO 构造请求参数，这里是批量进程接口，转换一下，
	req := &gse.MultiProcOperateReq{}
	req.ProcOperateReq = []gse.ProcessOperate{}
	resp, err := e.gseService.OperateProcMulti(c.Context(), req)
	if err != nil {
		return err
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
