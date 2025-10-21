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
package hello

import (
	"strconv"
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/hello"
)

// Add 一个简单的加法计算任务
func Add(a, b int) *types.Step {
	// 可能存在多个step，使用同一个stepName（定位执行器），说明最终执行器是一样的，可能只是参数不一样
	add := types.NewStep("add-task", hello.AddStepName.String()).
		SetAlias("add").
		AddParam("a", strconv.Itoa(a)).
		AddParam("b", strconv.Itoa(b)).
		SetMaxExecution(10 * time.Second).
		SetMaxTries(3)
	// add.SetPayload(obj) 负载类型参数通过 SetPayload 设置

	return add
}
