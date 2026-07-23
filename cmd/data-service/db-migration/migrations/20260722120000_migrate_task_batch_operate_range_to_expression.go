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

package migrations

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/cmd/data-service/db-migration/migrator"
	"github.com/TencentBlueKing/bk-bscp/internal/expression"
)

func init() {
	migrator.GetMigrator().AddMigration(&migrator.Migration{
		Version: "20260722120000",
		Name:    "20260722120000_migrate_task_batch_operate_range_to_expression",
		Mode:    migrator.GormMode,
		Up:      mig20260722120000Up,
		Down:    mig20260722120000Down,
	})
}

// oldOperateRange 迁移前的旧数组结构（本地定义，不依赖 table.OperateRange 当前形态）。
type oldOperateRange struct {
	SetNames     []string `json:"set_names"`
	ModuleNames  []string `json:"module_names"`
	ServiceNames []string `json:"service_names"`
	ProcessAlias []string `json:"process_alias"`
	CCProcessIDs []uint32 `json:"cc_process_ids"`
}

// newOperateRange 迁移后的五段表达式字符串结构。
type newOperateRange struct {
	SetName      string `json:"set_name"`
	ModuleName   string `json:"module_name"`
	ServiceName  string `json:"service_name"`
	ProcessAlias string `json:"process_alias"`
	ProcessID    string `json:"process_id"`
}

// convertOperateRangeJSON 把一条 task_data JSON 中旧数组格式的 operate_range 转为五段表达式字符串。
// 返回 (新 task_data, 是否发生变更, error)。已是新格式或无旧字段则原样返回、changed=false（幂等）。
func convertOperateRangeJSON(taskData string) (string, bool, error) {
	if taskData == "" {
		return taskData, false, nil
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(taskData), &top); err != nil {
		return taskData, false, err
	}

	raw, ok := top["operate_range"]
	if !ok {
		return taskData, false, nil
	}

	// 幂等判定：旧格式含复数键 set_names；不含则视为已迁移/无需处理。
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return taskData, false, err
	}
	if _, isOld := probe["set_names"]; !isOld {
		return taskData, false, nil
	}

	var old oldOperateRange
	if err := json.Unmarshal(raw, &old); err != nil {
		return taskData, false, err
	}

	newRange := newOperateRange{
		SetName:      expression.List2Expr(old.SetNames),
		ModuleName:   expression.List2Expr(old.ModuleNames),
		ServiceName:  expression.List2Expr(old.ServiceNames),
		ProcessAlias: expression.List2Expr(old.ProcessAlias),
		ProcessID:    expression.IDsToExpr(old.CCProcessIDs),
	}
	newRaw, err := json.Marshal(newRange)
	if err != nil {
		return taskData, false, err
	}

	// 仅替换 operate_range，保留 environment / config_template_ids 等其余字段。
	top["operate_range"] = newRaw
	out, err := json.Marshal(top)
	if err != nil {
		return taskData, false, err
	}
	return string(out), true, nil
}

// mig20260722120000Up 把存量 task_batches.task_data 中旧数组格式的 operate_range 无损刷成表达式字符串。
// 幂等：已迁移记录跳过；解析失败的记录跳过（不阻断迁移），保持原值。
func mig20260722120000Up(tx *gorm.DB) error {
	type taskBatchRow struct {
		ID       uint32 `gorm:"column:id"`
		TaskData string `gorm:"column:task_data"`
	}

	var rows []taskBatchRow
	if err := tx.Table("task_batches").Select("id, task_data").Find(&rows).Error; err != nil {
		return err
	}

	for _, r := range rows {
		newTaskData, changed, err := convertOperateRangeJSON(r.TaskData)
		if err != nil || !changed {
			// 解析失败或无需变更：跳过，不阻断整体迁移
			continue
		}
		if err := tx.Table("task_batches").Where("id = ?", r.ID).
			Update("task_data", newTaskData).Error; err != nil {
			return err
		}
	}
	return nil
}

// mig20260722120000Down 空操作：表达式 -> 数组不可逆（范围/切片无法还原），故不做反向迁移。
func mig20260722120000Down(_ *gorm.DB) error {
	return nil
}
