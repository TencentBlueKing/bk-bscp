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

package common

import (
	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
)

// Builder common builder
type Builder struct {
	dao dao.Set
}

// NewBuilder new
func NewBuilder(dao dao.Set) *Builder {
	return &Builder{
		dao: dao,
	}
}

// SetCommonProcessParam 设置
func (builder *Builder) CommonProcessFinalize(task *types.Task, processInstanceID uint32) {
	// TODO: 获取根据process_instance_id获取进程相关配置存储在commonParma中，方便查询task可以直接查询任务发起时的配置快照（也方便进行对比）
	// 从db主动获取进行信息组装payload
	// task.SetCommonPayload(&commonExecutor.ProcessPayload{})
}
