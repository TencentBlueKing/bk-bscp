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
	"fmt"
	"strconv"

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"
)

const (
	// AddStepName add step name
	AddStepName istep.StepName = "Add"
)

// HelloExecutor hello step executor
// nolint: revive
type HelloExecutor struct {
}

// Add implements istep.Step.
func (e *HelloExecutor) Add(c *istep.Context) error {
	a, exists := c.GetParam("a")
	if !exists {
		return fmt.Errorf("a not exists")
	}
	b, exists := c.GetParam("b")
	if !exists {
		return fmt.Errorf("b not exists")
	}

	aInt, err := strconv.Atoi(a)
	if err != nil {
		return err
	}
	bInt, err := strconv.Atoi(b)
	if err != nil {
		return err
	}

	fmt.Printf("a(%d) + b(%d) = %d\n", aInt, bInt, aInt+bInt)
	return nil
}

// Register register step
func Register(e *HelloExecutor) {
	istep.Register(AddStepName, istep.StepExecutorFunc(e.Add))
}
