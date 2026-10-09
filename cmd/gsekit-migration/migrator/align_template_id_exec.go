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

package migrator

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// execute 在单个事务里完成腾空、入位、引用改写与水位收尾。
// 高位 ID 段已在事务外分配（水位单调，先提交是安全的），事务内只做 UPDATE。
func (a *TemplateIDAligner) execute(steps []MoveStep) error {
	if len(steps) == 0 {
		log.Println("  Nothing to move, every config_template is already aligned")
		return nil
	}

	pairs := finalMapping(steps)
	log.Printf("  Executing: moving %d config_templates", len(steps))

	return a.targetDB.Transaction(func(tx *gorm.DB) error {
		// 腾空：先把所有要动的行搬到高位，GSEKit ID 区间随之彻底空出
		for _, s := range steps {
			if s.TempID == 0 {
				continue
			}
			if err := moveTemplateID(tx, s.OldID, s.TempID); err != nil {
				return err
			}
		}

		// 入位：从临时高位落到终值。自建模版的终值就是临时 ID，这里无事可做
		for _, s := range steps {
			from := s.OldID
			if s.TempID != 0 {
				from = s.TempID
			}
			if from == s.FinalID {
				continue
			}
			if err := moveTemplateID(tx, from, s.FinalID); err != nil {
				return err
			}
		}

		if err := a.updateReferences(tx, pairs); err != nil {
			return err
		}

		return a.finalizeWatermark(tx)
	})
}

// moveTemplateID 把一行的主键从 from 改到 to。
func moveTemplateID(tx *gorm.DB, from, to uint32) error {
	res := tx.Exec("UPDATE config_templates SET id = ? WHERE id = ?", to, from)
	if res.Error != nil {
		return fmt.Errorf("move config_template %d to %d failed: %w", from, to, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("move config_template %d to %d affected no rows", from, to)
	}
	return nil
}

// updateReferences 改写三处引用，用的是 old_id → final_id 的最终映射，
// 中间的临时 ID 对引用表不可见。
//
// 这三处是逐列核对过的全部引用点：config_instances.config_template_id、
// task_batches.task_data 里的 config_template_ids 数组、audits 中
// res_type = 'config_template' 的 res_id。template_sets.template_ids 引用的是
// templates.id，config_templates.cc_process_ids / cc_template_process_ids 存的是
// CC 进程 ID，都不在此列。BSCP 不使用外键约束，改主键既不级联也不报错，
// 漏改一处只会留下静默的脏数据。
func (a *TemplateIDAligner) updateReferences(tx *gorm.DB, pairs []IDPair) error {
	if len(pairs) == 0 {
		return nil
	}

	for _, chunk := range chunkPairs(pairs, alignReferenceChunkSize) {
		if err := updateConfigInstanceRefs(tx, chunk); err != nil {
			return err
		}
		if err := updateAuditRefs(tx, chunk); err != nil {
			return err
		}
	}

	mapping := make(map[uint32]uint32, len(pairs))
	for _, p := range pairs {
		mapping[p.From] = p.To
	}
	return a.updateTaskBatchRefs(tx, mapping)
}

// updateConfigInstanceRefs 改写历史配置实例指向的模版 ID。
func updateConfigInstanceRefs(tx *gorm.DB, chunk []IDPair) error {
	caseExpr, caseArgs := buildIDCaseWhen("config_template_id", chunk)
	args := make([]interface{}, 0, len(caseArgs)+1)
	args = append(args, caseArgs...)
	args = append(args, pairSourceIDs(chunk))

	if err := tx.Exec(
		"UPDATE config_instances SET config_template_id = "+caseExpr+
			" WHERE config_template_id IN ?", args...).Error; err != nil {
		return fmt.Errorf("update config_instances references failed: %w", err)
	}
	return nil
}

// updateAuditRefs 改写审计记录指向的模版 ID。
func updateAuditRefs(tx *gorm.DB, chunk []IDPair) error {
	caseExpr, caseArgs := buildIDCaseWhen("res_id", chunk)
	args := make([]interface{}, 0, len(caseArgs)+2)
	args = append(args, caseArgs...)
	args = append(args, auditResTypeConfigTemplate, pairSourceIDs(chunk))

	if err := tx.Exec(
		"UPDATE audits SET res_id = "+caseExpr+
			" WHERE res_type = ? AND res_id IN ?", args...).Error; err != nil {
		return fmt.Errorf("update audits references failed: %w", err)
	}
	return nil
}

// updateTaskBatchRefs 改写任务批次里的模版 ID 数组。
// 按主键逐行写回，不存在 CASE 表达式要解决的链式覆盖问题。
func (a *TemplateIDAligner) updateTaskBatchRefs(tx *gorm.DB, mapping map[uint32]uint32) error {
	batches, err := a.loadTaskBatchesToRewrite(mapping)
	if err != nil {
		return err
	}

	for _, b := range batches {
		res := tx.Exec("UPDATE task_batches SET task_data = ? WHERE id = ?", b.TaskData, b.ID)
		if res.Error != nil {
			return fmt.Errorf("update task_batches %d task_data failed: %w", b.ID, res.Error)
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("update task_batches %d task_data affected no rows", b.ID)
		}
	}

	if len(batches) > 0 {
		log.Printf("  Rewrote config_template_ids in %d task_batches", len(batches))
	}
	return nil
}

// finalizeWatermark 收尾抬高水位。
// 只增不减：腾空时水位已被推到高位，若在此把它降回预留基线，
// 并发新建就可能重新分配到刚刚用掉的临时 ID。代价是白白消耗掉一段 ID 空间，可忽略。
func (a *TemplateIDAligner) finalizeWatermark(tx *gorm.DB) error {
	var maxID uint32
	if err := tx.Raw("SELECT COALESCE(MAX(id), 0) FROM config_templates").Scan(&maxID).Error; err != nil {
		return fmt.Errorf("read max config_templates id failed: %w", err)
	}

	floor := a.reserveBase
	if maxID > floor {
		floor = maxID
	}
	return ensureIDWatermark(tx, idGeneratorConfigTemplates, floor)
}

// verify 是阶段 4 的只读校验。任一项失败都说明搬迁没有按计划落地，
// 此时应按报告 moves 段反向执行做人工回滚。
func (a *TemplateIDAligner) verify(pre *alignPreflight, steps []MoveStep) []AlignVerifyResult {
	return []AlignVerifyResult{
		a.verifyNoDuplicateID(),
		a.verifyReservedGapEmpty(pre.gsekitAutoIncrement),
		a.verifyMovedRows(steps),
	}
}

// verifyNoDuplicateID 兜底确认主键唯一性没有被破坏。
func (a *TemplateIDAligner) verifyNoDuplicateID() AlignVerifyResult {
	res := AlignVerifyResult{Name: "config_templates has no duplicate id"}

	var dup int64
	if err := a.targetDB.Raw(
		"SELECT COUNT(*) FROM (SELECT id FROM config_templates GROUP BY id HAVING COUNT(*) > 1) dups").
		Scan(&dup).Error; err != nil {
		res.Details = err.Error()
		return res
	}
	if dup > 0 {
		res.Details = fmt.Sprintf("found %d duplicated ids", dup)
		return res
	}

	res.Pass = true
	return res
}

// verifyReservedGapEmpty 确认 [gsekit 水位, 预留基线) 这段无人占用。
// 对齐后的模版 ID 全部低于 GSEKit 水位，自建模版全部不低于预留基线，中间这段必须为空。
// 出现占用说明有记录既没对齐也没搬进预留区。
func (a *TemplateIDAligner) verifyReservedGapEmpty(gsekitAutoIncrement uint32) AlignVerifyResult {
	res := AlignVerifyResult{
		Name: fmt.Sprintf("no config_template id lands in [%d, %d)", gsekitAutoIncrement, a.reserveBase),
	}

	var stranded int64
	if err := a.targetDB.Raw(
		"SELECT COUNT(*) FROM config_templates WHERE id >= ? AND id < ?",
		gsekitAutoIncrement, a.reserveBase).Scan(&stranded).Error; err != nil {
		res.Details = err.Error()
		return res
	}
	if stranded > 0 {
		res.Details = fmt.Sprintf("%d template(s) stranded in the gap", stranded)
		return res
	}

	res.Pass = true
	return res
}

// verifyMovedRows 确认每条搬迁都落到了计划的终值上。
func (a *TemplateIDAligner) verifyMovedRows(steps []MoveStep) AlignVerifyResult {
	res := AlignVerifyResult{Name: "every moved template landed on its final id"}
	if len(steps) == 0 {
		res.Pass = true
		return res
	}

	finalIDs := make([]uint32, 0, len(steps))
	for _, s := range steps {
		finalIDs = append(finalIDs, s.FinalID)
	}

	var found int64
	if err := a.targetDB.Raw(
		"SELECT COUNT(*) FROM config_templates WHERE id IN ?", finalIDs).Scan(&found).Error; err != nil {
		res.Details = err.Error()
		return res
	}
	if found != int64(len(steps)) {
		res.Details = fmt.Sprintf("expected %d rows on final ids, found %d", len(steps), found)
		return res
	}

	res.Pass = true
	return res
}
