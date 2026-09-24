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

import "testing"

func TestIsPrecheckCandidate(t *testing.T) {
	cases := []struct {
		name    string
		row     BSCPConfigTemplateRow
		creator string
		want    bool
	}{
		{
			name:    "migration creator in normal biz",
			row:     BSCPConfigTemplateRow{ID: 55, BizID: 5000079, Creator: "xiaolnwang"},
			creator: "xiaolnwang",
			want:    true,
		},
		{
			name:    "native biz excluded",
			row:     BSCPConfigTemplateRow{ID: 1, BizID: testNativeBizID, Creator: "xiaolnwang"},
			creator: "xiaolnwang",
			want:    false,
		},
		{
			name:    "other creator skipped",
			row:     BSCPConfigTemplateRow{ID: 85, BizID: 5000079, Creator: "regyhuang"},
			creator: "xiaolnwang",
			want:    false,
		},
		{
			name:    "empty migration creator matches nothing",
			row:     BSCPConfigTemplateRow{ID: 55, BizID: 5000079, Creator: "xiaolnwang"},
			creator: "",
			want:    false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isPrecheckCandidate(c.row, c.creator, testNativeBizID); got != c.want {
				t.Errorf("isPrecheckCandidate = %v, want %v", got, c.want)
			}
		})
	}
}

func TestBuildPrecheckAlignReport(t *testing.T) {
	templates := []BSCPConfigTemplateRow{
		{ID: 55, BizID: 5000079, Name: "nginx-test", Creator: "xiaolnwang"},
		{ID: 70, BizID: 2, Name: "gone", Creator: "xiaolnwang"},
		{ID: 1, BizID: testNativeBizID, Name: "test-biz", Creator: "xiaolnwang"},
		{ID: 85, BizID: 5000079, Name: "native", Creator: "regyhuang"},
	}
	byBizName := map[bizNameKey]uint32{
		{bizID: 5000079, name: "nginx-test"}: 11120,
	}

	report := buildPrecheckAlignReport(templates, byBizName, "xiaolnwang", testNativeBizID)

	if report.Summary.Scanned != 2 {
		t.Fatalf("scanned = %d, want 2", report.Summary.Scanned)
	}
	if report.Summary.OK != 1 {
		t.Fatalf("ok = %d, want 1", report.Summary.OK)
	}
	if report.Summary.Alert != 1 {
		t.Fatalf("alert = %d, want 1", report.Summary.Alert)
	}
	if len(report.OKs) != 1 || report.OKs[0].BSCPID != 55 || report.OKs[0].GSEKitConfigTemplateID != 11120 {
		t.Fatalf("oks = %+v, want bscp 55 → gsekit 11120", report.OKs)
	}
	if len(report.Alerts) != 1 || report.Alerts[0].BSCPID != 70 {
		t.Fatalf("alerts = %+v, want bscp 70", report.Alerts)
	}
	if report.Alerts[0].Reason != precheckAlertReasonNoGSEKitMatch {
		t.Fatalf("alert reason = %q", report.Alerts[0].Reason)
	}
	if len(report.ExcludedBizIDs) != 1 || report.ExcludedBizIDs[0] != testNativeBizID {
		t.Fatalf("excluded_biz_ids = %v", report.ExcludedBizIDs)
	}
}
