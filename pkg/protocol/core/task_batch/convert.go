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
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

// PbTaskBatch convert table TaskBatch to pb TaskBatch
func PbTaskBatch(tb *table.TaskBatch) *TaskBatch {
	if tb == nil {
		return nil
	}

	result := &TaskBatch{
		Id:         tb.ID,
		TaskObject: string(tb.Spec.TaskObject),
		TaskAction: string(tb.Spec.TaskAction),
		TaskData:   tb.Spec.TaskData,
		Status:     string(tb.Spec.Status),
	}

	if tb.Spec.StartAt != nil {
		result.StartAt = tb.Spec.StartAt.Format("2006-01-02 15:04:05")
	}
	if tb.Spec.EndAt != nil {
		result.EndAt = tb.Spec.EndAt.Format("2006-01-02 15:04:05")
	}
	if tb.Revision != nil {
		result.CreatedAt = tb.Revision.CreatedAt.Format("2006-01-02 15:04:05")
		result.UpdatedAt = tb.Revision.UpdatedAt.Format("2006-01-02 15:04:05")
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
