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
	"reflect"
	"testing"
)

const testMigrationCreator = "xiaolnwang"
const testNativeBizID uint32 = 100148

func TestClassifyTemplate(t *testing.T) {
	cases := []struct {
		name        string
		row         BSCPConfigTemplateRow
		nameID      uint32
		isNativeBiz bool
		wantClass   AlignClassification
		wantID      uint32
	}{
		{
			name:      "name matched wins regardless of creator",
			row:       BSCPConfigTemplateRow{ID: 55, BizID: 5000079, Creator: "regyhuang"},
			nameID:    11120,
			wantClass: ClassMatchedName,
			wantID:    11120,
		},
		{
			name:      "name matched for a migration artifact",
			row:       BSCPConfigTemplateRow{ID: 60, BizID: 100605, Creator: testMigrationCreator},
			nameID:    11012,
			wantClass: ClassMatchedName,
			wantID:    11012,
		},
		{
			name:      "unmatched with migration creator is unknown",
			row:       BSCPConfigTemplateRow{ID: 70, BizID: 2, Creator: testMigrationCreator},
			nameID:    0,
			wantClass: ClassUnmatchedUnknown,
			wantID:    0,
		},
		{
			name:      "unmatched with another creator is native",
			row:       BSCPConfigTemplateRow{ID: 71, BizID: 2, Creator: "someone"},
			nameID:    0,
			wantClass: ClassUnmatchedNative,
			wantID:    0,
		},
		{
			name:        "native biz ignores a GSEKit name hit",
			row:         BSCPConfigTemplateRow{ID: 11196, BizID: testNativeBizID, Creator: testMigrationCreator},
			nameID:      11196,
			isNativeBiz: true,
			wantClass:   ClassForcedNative,
			wantID:      0,
		},
		{
			name:        "native biz without a GSEKit hit is never blocked",
			row:         BSCPConfigTemplateRow{ID: 12, BizID: testNativeBizID, Creator: testMigrationCreator},
			nameID:      0,
			isNativeBiz: true,
			wantClass:   ClassForcedNative,
			wantID:      0,
		},
		{
			name:      "same biz is not forced when not listed as native",
			row:       BSCPConfigTemplateRow{ID: 11196, BizID: testNativeBizID, Creator: testMigrationCreator},
			nameID:    11196,
			wantClass: ClassMatchedName,
			wantID:    11196,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotClass, gotID := classifyTemplate(c.row, c.nameID, testMigrationCreator, c.isNativeBiz)
			if gotClass != c.wantClass {
				t.Errorf("classification = %v, want %v", gotClass, c.wantClass)
			}
			if gotID != c.wantID {
				t.Errorf("gsekit id = %d, want %d", gotID, c.wantID)
			}
		})
	}
}

func TestClassifyTemplateEmptyMigrationCreatorNeverMatches(t *testing.T) {
	row := BSCPConfigTemplateRow{ID: 80, BizID: 2, Creator: ""}

	gotClass, _ := classifyTemplate(row, 0, "", false)
	if gotClass != ClassUnmatchedNative {
		t.Errorf("classification = %v, want %v", gotClass, ClassUnmatchedNative)
	}
}

func TestDecideForcedNative(t *testing.T) {
	aligner := &TemplateIDAligner{reserveBase: 20000}

	cases := []struct {
		name       string
		row        BSCPConfigTemplateRow
		wantSource string
		wantFinal  uint32
	}{
		{
			name:       "inside the GSEKit range is evacuated",
			row:        BSCPConfigTemplateRow{ID: 11196, BizID: testNativeBizID, Name: "test-biz"},
			wantSource: decisionEvacuate,
			wantFinal:  0,
		},
		{
			name:       "already in the reserved range stays put",
			row:        BSCPConfigTemplateRow{ID: 20005, BizID: testNativeBizID, Name: "test-biz"},
			wantSource: decisionNoop,
			wantFinal:  20005,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := aligner.decide(c.row, ClassForcedNative, 0)
			if rec.DecisionSource != c.wantSource {
				t.Errorf("decision source = %q, want %q", rec.DecisionSource, c.wantSource)
			}
			if rec.FinalNewID != c.wantFinal {
				t.Errorf("final id = %d, want %d", rec.FinalNewID, c.wantFinal)
			}
		})
	}
}

func TestNeedsManualDecision(t *testing.T) {
	cases := map[AlignClassification]bool{
		ClassMatchedName:      false,
		ClassUnmatchedNative:  false,
		ClassForcedNative:     false,
		ClassUnmatchedUnknown: true,
	}

	for class, want := range cases {
		if got := needsManualDecision(class); got != want {
			t.Errorf("needsManualDecision(%v) = %v, want %v", class, got, want)
		}
	}
}

func TestNeedsTempID(t *testing.T) {
	cases := []struct {
		name string
		rec  AlignRecord
		want bool
	}{
		{
			name: "already aligned needs no move",
			rec:  AlignRecord{BSCPID: 11120, FinalNewID: 11120, DecisionSource: decisionNoop},
			want: false,
		},
		{
			name: "moving to another id needs evacuation",
			rec:  AlignRecord{BSCPID: 55, FinalNewID: 11120, DecisionSource: decisionName},
			want: true,
		},
		{
			name: "native template awaiting a reserved id needs evacuation",
			rec:  AlignRecord{BSCPID: 7, FinalNewID: 0, DecisionSource: decisionEvacuate},
			want: true,
		},
		{
			name: "blocked record is not moved",
			rec:  AlignRecord{BSCPID: 9, FinalNewID: 0, DecisionSource: decisionBlocked},
			want: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := needsTempID(c.rec); got != c.want {
				t.Errorf("needsTempID = %v, want %v", got, c.want)
			}
		})
	}
}

func TestPlanAlignment(t *testing.T) {
	records := []AlignRecord{
		{BSCPID: 55, FinalNewID: 11120, DecisionSource: decisionName},
		{BSCPID: 7, FinalNewID: 0, DecisionSource: decisionEvacuate},
		{BSCPID: 11122, FinalNewID: 11122, DecisionSource: decisionNoop},
	}
	tempIDs := []uint32{20001, 20002}

	steps, err := planAlignment(records, tempIDs)
	if err != nil {
		t.Fatalf("planAlignment failed: %v", err)
	}

	// 步骤按旧 ID 升序，且已对齐的记录不产生步骤
	want := []MoveStep{
		{OldID: 7, TempID: 20001, FinalID: 20001},
		{OldID: 55, TempID: 20002, FinalID: 11120},
	}
	if !reflect.DeepEqual(steps, want) {
		t.Errorf("steps = %+v, want %+v", steps, want)
	}

	// 搬往预留区的记录在这里才拿到终值
	if records[1].FinalNewID != 20001 {
		t.Errorf("evacuated record final id = %d, want 20001", records[1].FinalNewID)
	}
}

func TestPlanAlignmentNotEnoughTempIDs(t *testing.T) {
	records := []AlignRecord{
		{BSCPID: 1, FinalNewID: 100, DecisionSource: decisionName},
		{BSCPID: 2, FinalNewID: 200, DecisionSource: decisionName},
	}

	if _, err := planAlignment(records, []uint32{20001}); err == nil {
		t.Fatal("expected an error when temporary ids run out")
	}
}

func TestPlanAlignmentRejectsDuplicateFinalID(t *testing.T) {
	records := []AlignRecord{
		{BSCPID: 1, FinalNewID: 500, DecisionSource: decisionName},
		{BSCPID: 2, FinalNewID: 500, DecisionSource: decisionName},
	}

	if _, err := planAlignment(records, []uint32{20001, 20002}); err == nil {
		t.Fatal("expected an error when two records target the same id")
	}
}

// TestMoveStepsSimulateExecution 用链式 ID 关系模拟两阶段搬迁，
// 确认执行过程中任何一步都不会撞上已被占用的主键。
func TestMoveStepsSimulateExecution(t *testing.T) {
	records := []AlignRecord{
		{BSCPID: 1, FinalNewID: 2, DecisionSource: decisionName},
		{BSCPID: 2, FinalNewID: 3, DecisionSource: decisionName},
		{BSCPID: 3, FinalNewID: 1, DecisionSource: decisionName},
		{BSCPID: 4, FinalNewID: 0, DecisionSource: decisionEvacuate},
	}
	tempIDs := []uint32{20001, 20002, 20003, 20004}

	steps, err := planAlignment(records, tempIDs)
	if err != nil {
		t.Fatalf("planAlignment failed: %v", err)
	}

	occupied := map[uint32]bool{1: true, 2: true, 3: true, 4: true}

	for _, s := range steps {
		if s.TempID == 0 {
			continue
		}
		if occupied[s.TempID] {
			t.Fatalf("evacuation collision: temp id %d already occupied", s.TempID)
		}
		delete(occupied, s.OldID)
		occupied[s.TempID] = true
	}

	for _, s := range steps {
		from := s.OldID
		if s.TempID != 0 {
			from = s.TempID
		}
		if from == s.FinalID {
			continue
		}
		if occupied[s.FinalID] {
			t.Fatalf("placement collision: final id %d already occupied", s.FinalID)
		}
		delete(occupied, from)
		occupied[s.FinalID] = true
	}

	want := map[uint32]bool{1: true, 2: true, 3: true, 20004: true}
	if !reflect.DeepEqual(occupied, want) {
		t.Errorf("final occupancy = %v, want %v", occupied, want)
	}
}

func TestSimulateHighIDs(t *testing.T) {
	// 起点取水位与基线的较大者，保证模拟 ID 不与在用 ID 相撞
	got := simulateHighIDs(203, 20000, 3)
	want := []uint32{20001, 20002, 20003}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("simulateHighIDs = %v, want %v", got, want)
	}

	got = simulateHighIDs(30000, 20000, 2)
	want = []uint32{30001, 30002}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("simulateHighIDs = %v, want %v", got, want)
	}

	if ids := simulateHighIDs(203, 20000, 0); ids != nil {
		t.Errorf("simulateHighIDs with zero count = %v, want nil", ids)
	}
}

func TestChunkPairs(t *testing.T) {
	pairs := []IDPair{{From: 1, To: 2}, {From: 3, To: 4}, {From: 5, To: 6}}

	chunks := chunkPairs(pairs, 2)
	if len(chunks) != 2 {
		t.Fatalf("chunk count = %d, want 2", len(chunks))
	}
	if len(chunks[0]) != 2 || len(chunks[1]) != 1 {
		t.Errorf("chunk sizes = %d/%d, want 2/1", len(chunks[0]), len(chunks[1]))
	}

	if chunkPairs(nil, 2) != nil {
		t.Error("chunkPairs(nil) should be nil")
	}
	if chunkPairs(pairs, 0) != nil {
		t.Error("chunkPairs with zero size should be nil")
	}
}

func TestFinalMapping(t *testing.T) {
	// 引用表看不到中间的临时 ID，映射必须直接从原值跳到终值
	steps := []MoveStep{
		{OldID: 55, TempID: 20001, FinalID: 11120},
		{OldID: 7, TempID: 20002, FinalID: 20002},
	}

	got := finalMapping(steps)
	want := []IDPair{{From: 55, To: 11120}, {From: 7, To: 20002}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("finalMapping = %+v, want %+v", got, want)
	}
}

func TestBuildIDCaseWhen(t *testing.T) {
	pairs := []IDPair{{From: 5, To: 11120}, {From: 11120, To: 20004}}

	expr, args := buildIDCaseWhen("config_template_id", pairs)
	wantExpr := "CASE config_template_id WHEN ? THEN ? WHEN ? THEN ? ELSE config_template_id END"
	if expr != wantExpr {
		t.Errorf("expr = %q, want %q", expr, wantExpr)
	}

	wantArgs := []interface{}{uint32(5), uint32(11120), uint32(11120), uint32(20004)}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Errorf("args = %v, want %v", args, wantArgs)
	}

	if expr, args := buildIDCaseWhen("res_id", nil); expr != "" || args != nil {
		t.Errorf("empty pairs should produce no expression, got %q / %v", expr, args)
	}
}

func TestPairSourceIDs(t *testing.T) {
	pairs := []IDPair{{From: 5, To: 11120}, {From: 7, To: 20002}}

	got := pairSourceIDs(pairs)
	want := []uint32{5, 7}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("pairSourceIDs = %v, want %v", got, want)
	}
}

func TestRewriteTaskData(t *testing.T) {
	mapping := map[uint32]uint32{5: 11120, 7: 20002}

	cases := []struct {
		name        string
		raw         string
		want        string
		wantChanged bool
	}{
		{
			name:        "empty stays empty",
			raw:         "",
			want:        "",
			wantChanged: false,
		},
		{
			name:        "no config_template_ids field is left alone",
			raw:         `{"process_ids":[1,2]}`,
			want:        `{"process_ids":[1,2]}`,
			wantChanged: false,
		},
		{
			name:        "ids outside the mapping are left alone",
			raw:         `{"config_template_ids":[99]}`,
			want:        `{"config_template_ids":[99]}`,
			wantChanged: false,
		},
		{
			name:        "mapped ids are rewritten and other fields survive",
			raw:         `{"config_template_ids":[5,99,7],"process_ids":[1]}`,
			want:        `{"config_template_ids":[11120,99,20002],"process_ids":[1]}`,
			wantChanged: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, changed, err := rewriteTaskData(c.raw, mapping)
			if err != nil {
				t.Fatalf("rewriteTaskData failed: %v", err)
			}
			if changed != c.wantChanged {
				t.Errorf("changed = %v, want %v", changed, c.wantChanged)
			}
			if got != c.want {
				t.Errorf("task_data = %s, want %s", got, c.want)
			}
		})
	}
}

func TestRewriteTaskDataInvalidJSON(t *testing.T) {
	if _, _, err := rewriteTaskData(`{not json`, map[uint32]uint32{1: 2}); err == nil {
		t.Fatal("expected an error for malformed task_data")
	}
}

func TestTempIDsNeeded(t *testing.T) {
	records := []AlignRecord{
		{BSCPID: 55, FinalNewID: 11120, DecisionSource: decisionName},
		{BSCPID: 7, FinalNewID: 0, DecisionSource: decisionEvacuate},
		{BSCPID: 11122, FinalNewID: 11122, DecisionSource: decisionNoop},
		{BSCPID: 9, FinalNewID: 0, DecisionSource: decisionBlocked},
	}

	if got := tempIDsNeeded(records); got != 2 {
		t.Errorf("tempIDsNeeded = %d, want 2", got)
	}
}
