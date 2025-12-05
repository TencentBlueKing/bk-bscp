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

// Package pbtb provides task batch core protocol struct and convert functions.
package pbtb

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// PbTaskBatch convert table TaskBatch to pb TaskBatch
func PbTaskBatch(tb *table.TaskBatch) *TaskBatch {
	if tb == nil {
		return nil
	}

	result := &TaskBatch{
		Id: tb.ID,
	}

	// 处理 Spec 相关字段
	if tb.Spec != nil {
		result.TaskObject = string(tb.Spec.TaskObject)
		result.TaskAction = string(tb.Spec.TaskAction)
		result.Status = string(tb.Spec.Status)

		// 解析 TaskData 从 JSON 字符串到 TaskExecutionData 对象
		if taskData, err := tb.Spec.GetTaskExecutionData(); err != nil {
			logs.Errorf("get task execution data failed, err: %v, task_data: %s", err, tb.Spec.TaskData)
		} else if taskData != nil {
			result.TaskData = &ProcessTaskData{
				Environment: taskData.Environment,
				OperateRange: &OperateRange{
					SetNames:       taskData.OperateRange.SetNames,
					ModuleNames:    taskData.OperateRange.ModuleNames,
					ServiceNames:   taskData.OperateRange.ServiceNames,
					CcProcessNames: taskData.OperateRange.ProcessAlias,
					CcProcessIds:   taskData.OperateRange.CCProcessID,
				},
			}
		}

		// 转换时间字段
		if tb.Spec.StartAt != nil {
			result.StartAt = timestamppb.New(*tb.Spec.StartAt)
		}
		if tb.Spec.EndAt != nil {
			result.EndAt = timestamppb.New(*tb.Spec.EndAt)
		}

		// 计算执行耗时
		if tb.Spec.StartAt != nil && tb.Spec.EndAt != nil {
			result.ExecutionTime = float32(tb.Spec.EndAt.Sub(*tb.Spec.StartAt).Seconds())
		}
	}

	// 处理 Revision 相关字段
	if tb.Revision != nil {
		result.Creator = tb.Revision.Creator
	}

	return result
}

// PbTaskBatches convert table TaskBatch list to pb TaskBatch list
func PbTaskBatches(tbs []*table.TaskBatch) []*TaskBatch {
	if tbs == nil {
		return make([]*TaskBatch, 0)
	}

	result := make([]*TaskBatch, 0, len(tbs))
	for _, tb := range tbs {
		result = append(result, PbTaskBatch(tb))
	}
	return result
}
