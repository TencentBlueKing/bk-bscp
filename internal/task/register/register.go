// * Tencent is pleased to support the open source community by making Blueking Container Service available.
//  * Copyright (C) 20\d\d THL A29 Limited, a Tencent company. All rights reserved.
//  * Licensed under the MIT License (the "License"); you may not use this file except
//  * in compliance with the License. You may obtain a copy of the License at
//  * http://opensource.org/licenses/MIT
//  * Unless required by applicable law or agreed to in writing, software distributed under
//  * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  * either express or implied. See the License for the specific language governing permissions and
//  * limitations under the License.

package register

import "github.com/TencentBlueKing/bk-bscp/internal/task/executor/hello"

// RegisterExecutor register all executor.
// RegisterExecutor 中可以补充参数，比如执行器依赖的配置，执行器依赖的第三方服务等
// nolint: revive
func RegisterExecutor() {
	// 注册 hello 执行器，
	e := &hello.HelloExecutor{}
	hello.Register(e)

	// 注册回调 等待补充
}
