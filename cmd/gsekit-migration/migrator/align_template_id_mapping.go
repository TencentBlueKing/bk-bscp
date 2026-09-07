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
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// AlignClassification 描述一条 BSCP config_templates 记录的对齐分类。
type AlignClassification string

const (
	// ClassMatchedName 名字在 GSEKit 同业务下命中，可自动对齐。
	// 存量数据实测无一条改过名，名字匹配是唯一的锚定手段。
	ClassMatchedName AlignClassification = "MATCHED_NAME"
	// ClassUnmatchedNative 名字未命中，特征像 BSCP 自建
	ClassUnmatchedNative AlignClassification = "UNMATCHED_NATIVE"
	// ClassUnmatchedUnknown 名字未命中，特征却像迁移产物，需人工确认
	ClassUnmatchedUnknown AlignClassification = "UNMATCHED_UNKNOWN"
	// ClassForcedNative 属于自建业务，不参与名字匹配，直接归入自建区
	ClassForcedNative AlignClassification = "FORCED_NATIVE"
)

// 决策来源，写进报告的 decision_source 字段。
const (
	// decisionName 名字在 GSEKit 命中，终值对齐到对应 GSEKit ID。
	decisionName = "name"
	// decisionEvacuate BSCP 自建模版占用了 GSEKit 区间，终值待分配高位 ID 后搬出。
	decisionEvacuate = "evacuate"
	// decisionNoop 已在终值上（已对齐，或自建已在预留区），无需搬迁。
	decisionNoop = "noop"
	// decisionBlocked 像迁移产物却对不上 GSEKit，阻塞执行，交人工处理。
	decisionBlocked = "blocked"
)

// BSCPConfigTemplateRow 是目标库 config_templates 的一行，只取对齐需要的列。
type BSCPConfigTemplateRow struct {
	ID      uint32 `gorm:"column:id"`
	BizID   uint32 `gorm:"column:biz_id"`
	Name    string `gorm:"column:name"`
	Creator string `gorm:"column:creator"`
}

// AlignRecord 是报告中的一条记录，同时承载映射决策的中间结论。
type AlignRecord struct {
	BSCPID         uint32              `json:"bscp_id"`
	BizID          uint32              `json:"biz_id"`
	Name           string              `json:"name"`
	Classification AlignClassification `json:"classification"`
	GSEKitIDByName uint32              `json:"gsekit_id_by_name"`
	FinalNewID     uint32              `json:"final_new_id"`
	DecisionSource string              `json:"decision_source"`
	Note           string              `json:"note,omitempty"`
}

// MoveStep 描述一条记录的搬迁路径。TempID 为 0 表示不需要腾空，直接从 OldID 改到 FinalID。
type MoveStep struct {
	OldID   uint32 `json:"old_id"`
	TempID  uint32 `json:"temp_id"`
	FinalID uint32 `json:"final_id"`
}

// IDPair 是一次 ID 改写的前后值。
type IDPair struct {
	From uint32 `json:"from"`
	To   uint32 `json:"to"`
}

// looksLikeMigrated 判断一条未匹配的记录是否更像迁移产物。
// 仅看 creator 是否等于配置里的 migration.creator
func looksLikeMigrated(bscpTemplate BSCPConfigTemplateRow, migrationCreator string) bool {
	return migrationCreator != "" && bscpTemplate.Creator == migrationCreator
}

// classifyTemplate 按名字匹配结果给记录分类，并返回自动决策出的 GSEKit ID。
// nameID 是 (业务, 模版名) 在 GSEKit 命中的 config_template_id，为 0 表示未命中。
// isNativeBiz 为 true 时整条记录不参与名字匹配，直接归入自建区。
// 返回的 ID 为 0 表示无自动结论。
func classifyTemplate(bscpTemplate BSCPConfigTemplateRow, gsekitConfigTemplateID uint32,
	migrationCreator string, isNativeBiz bool) (AlignClassification, uint32) {

	switch {
	// 自建业务的判定优先于名字匹配：即便名字恰好与 GSEKit 撞上，那也是巧合而非同一份模版
	case isNativeBiz:
		return ClassForcedNative, 0
	case gsekitConfigTemplateID != 0:
		return ClassMatchedName, gsekitConfigTemplateID
	case looksLikeMigrated(bscpTemplate, migrationCreator):
		return ClassUnmatchedUnknown, 0
	default:
		return ClassUnmatchedNative, 0
	}
}

// needsManualDecision 判断一个分类是否必须人工裁决。
// 看着像迁移产物却匹配不到 GSEKit，不能靠猜：它既不该留在原 ID 上，
// 也不该被当成自建搬进预留区，只能阻塞执行由报告列出交人工处理。
func needsManualDecision(class AlignClassification) bool {
	return class == ClassUnmatchedUnknown
}

// tempIDsNeeded 统计需要腾空的记录数，即需要预分配多少个临时高位 ID。
func tempIDsNeeded(records []AlignRecord) int {
	count := 0
	for _, r := range records {
		if needsTempID(r) {
			count++
		}
	}
	return count
}

// needsTempID 判断一条记录是否需要先搬到高位腾空。
// 每一条要移动的记录都走中转，不做"目标必然空闲所以可以一步到位"的优化：
// 那类优化依赖对当前占用情况的推断，而搬迁会让一行的新 ID 恰好是另一行的旧 ID，
// 少走一步中转就可能撞主键。多一次 UPDATE 换来的是任意 ID 链式关系下都成立的正确性。
// 已经落在终值上的记录完全不动，这让重复执行天然成为空操作。
func needsTempID(r AlignRecord) bool {
	if r.FinalNewID == 0 {
		// 终值待定的只有搬往预留区的自建模版，它的终值就是这里分配的临时 ID
		return r.DecisionSource == decisionEvacuate
	}
	return r.FinalNewID != r.BSCPID
}

// simulateHighIDs 在 dry-run 下伪造腾空用的高位 ID，只为让报告算出终值与引用影响，
// 不触碰 id_generators。起点取当前水位与预留基线的较大者，保证不与任何在用 ID 相撞。
func simulateHighIDs(currentMaxID, reserveBase uint32, count int) []uint32 {
	if count <= 0 {
		return nil
	}

	start := currentMaxID
	if reserveBase > start {
		start = reserveBase
	}

	ids := make([]uint32, count)
	for i := 0; i < count; i++ {
		ids[i] = start + uint32(i) + 1
	}
	return ids
}

// planAlignment 给搬往预留区的记录定终值，并规划搬迁步骤。
// tempIDs 必须是 id_generators 分配的连续高位 ID，数量与 tempIDsNeeded 一致。
// records 会被就地修改：自建模版的 FinalNewID 在这里才落定。
// 返回的步骤按 OldID 升序，保证结果可复现。
func planAlignment(records []AlignRecord, tempIDs []uint32) ([]MoveStep, error) {
	order := make([]int, len(records))
	for i := range records {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool { return records[order[i]].BSCPID < records[order[j]].BSCPID })

	tempOf := make(map[uint32]uint32, len(tempIDs))
	used := 0
	for _, idx := range order {
		r := &records[idx]
		if !needsTempID(*r) {
			continue
		}
		if used >= len(tempIDs) {
			return nil, fmt.Errorf("not enough temporary IDs: need more than %d", len(tempIDs))
		}
		tempOf[r.BSCPID] = tempIDs[used]
		if r.FinalNewID == 0 {
			r.FinalNewID = tempIDs[used]
		}
		used++
	}

	if err := validateFinalIDs(records); err != nil {
		return nil, err
	}

	steps := make([]MoveStep, 0, len(order))
	for _, idx := range order {
		r := records[idx]
		if r.FinalNewID == 0 || r.FinalNewID == r.BSCPID {
			continue
		}
		steps = append(steps, MoveStep{OldID: r.BSCPID, TempID: tempOf[r.BSCPID], FinalID: r.FinalNewID})
	}

	return steps, nil
}

// validateFinalIDs 保证不会有两条记录搬到同一个 ID 上。
func validateFinalIDs(records []AlignRecord) error {
	owner := make(map[uint32]uint32, len(records))
	for _, r := range records {
		if r.FinalNewID == 0 {
			continue
		}
		if prev, dup := owner[r.FinalNewID]; dup {
			return fmt.Errorf("config_templates %d and %d both target id %d", prev, r.BSCPID, r.FinalNewID)
		}
		owner[r.FinalNewID] = r.BSCPID
	}
	return nil
}

// chunkPairs 把映射切成小块，避免单条 SQL 里的 CASE 分支和 IN 列表过长。
func chunkPairs(pairs []IDPair, size int) [][]IDPair {
	if size <= 0 || len(pairs) == 0 {
		return nil
	}

	chunks := make([][]IDPair, 0, (len(pairs)+size-1)/size)
	for start := 0; start < len(pairs); start += size {
		end := start + size
		if end > len(pairs) {
			end = len(pairs)
		}
		chunks = append(chunks, pairs[start:end])
	}
	return chunks
}

// finalMapping 把搬迁步骤压成 oldID → finalID 的映射，供引用表一次性改写使用。
// 引用表看不到中间的临时 ID，必须直接从原值跳到终值。
func finalMapping(steps []MoveStep) []IDPair {
	pairs := make([]IDPair, 0, len(steps))
	for _, s := range steps {
		pairs = append(pairs, IDPair{From: s.OldID, To: s.FinalID})
	}
	return pairs
}

// buildIDCaseWhen 生成 "CASE col WHEN ? THEN ? ... ELSE col END" 表达式。
// 引用表必须用单条语句改写：全区腾空会让一行的新 ID 恰好是另一行的旧 ID，
// 逐条 UPDATE 时先改出来的值会被后一条再次改掉，造成静默错乱。
// CASE 表达式在同一语句内基于原值求值，不存在这个问题。
func buildIDCaseWhen(column string, pairs []IDPair) (string, []interface{}) {
	if len(pairs) == 0 {
		return "", nil
	}

	var sb strings.Builder
	args := make([]interface{}, 0, len(pairs)*2)

	sb.WriteString("CASE ")
	sb.WriteString(column)
	for _, p := range pairs {
		sb.WriteString(" WHEN ? THEN ?")
		args = append(args, p.From, p.To)
	}
	sb.WriteString(" ELSE ")
	sb.WriteString(column)
	sb.WriteString(" END")

	return sb.String(), args
}

// pairSourceIDs 提取待改写的原值列表，用作 WHERE ... IN 的参数。
func pairSourceIDs(pairs []IDPair) []uint32 {
	ids := make([]uint32, 0, len(pairs))
	for _, p := range pairs {
		ids = append(ids, p.From)
	}
	return ids
}

// rewriteTaskData 改写 task_batches.task_data 里的 config_template_ids。
// 用 map[string]json.RawMessage 而非 table.TaskExecutionData 承接，其余字段保持原样透传，
// 避免结构体字段落后于线上数据时把未知字段丢掉。
// 返回改写后的 JSON 与是否发生变化。
func rewriteTaskData(raw string, mapping map[uint32]uint32) (string, bool, error) {
	if strings.TrimSpace(raw) == "" {
		return raw, false, nil
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return raw, false, fmt.Errorf("unmarshal task_data failed: %w", err)
	}

	rawIDs, ok := fields["config_template_ids"]
	if !ok {
		return raw, false, nil
	}

	var ids []uint32
	if err := json.Unmarshal(rawIDs, &ids); err != nil {
		return raw, false, fmt.Errorf("unmarshal config_template_ids failed: %w", err)
	}

	changed := false
	for i, id := range ids {
		if newID, hit := mapping[id]; hit && newID != id {
			ids[i] = newID
			changed = true
		}
	}

	if !changed {
		return raw, false, nil
	}

	encoded, err := json.Marshal(ids)
	if err != nil {
		return raw, false, fmt.Errorf("marshal config_template_ids failed: %w", err)
	}
	fields["config_template_ids"] = encoded

	out, err := json.Marshal(fields)
	if err != nil {
		return raw, false, fmt.Errorf("marshal task_data failed: %w", err)
	}

	return string(out), true, nil
}
