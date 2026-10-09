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
	"os"
	"path/filepath"
	"testing"
)

func TestCompareRenderedContentDetectsJSONSemanticMatch(t *testing.T) {
	result := compareRenderedContent(
		`{"ServerList":[{"zoneid":"1","name":"一区"}],"GroupList":[{"groupid":1}]}`,
		`{
			"GroupList": [{"groupid": 1}],
			"ServerList": [{"name": "一区", "zoneid": "1"}]
		}`,
		false,
	)

	if result.Matched {
		t.Fatal("expected text mismatch")
	}
	if !result.JSONSemanticMatched {
		t.Fatal("expected JSON semantic match")
	}
	if result.Reason != "content_mismatch_json_equivalent" {
		t.Fatalf("expected json equivalent reason, got %q", result.Reason)
	}
}

func TestCompareRenderedContentKeepsJSONArrayOrderSignificant(t *testing.T) {
	result := compareRenderedContent(
		`{"ServerList":[{"zoneid":"1"},{"zoneid":"2"}]}`,
		`{"ServerList":[{"zoneid":"2"},{"zoneid":"1"}]}`,
		false,
	)

	if result.JSONSemanticMatched {
		t.Fatal("expected different array order to remain a real mismatch")
	}
	if result.Reason != "content_mismatch" {
		t.Fatalf("expected content mismatch reason, got %q", result.Reason)
	}
}

func TestCompareRenderedContentIgnoresJSONArrayOrderWhenEnabled(t *testing.T) {
	result := compareRenderedContent(
		`{"ServerList":[{"zoneid":"1"},{"zoneid":"2"}]}`,
		`{"ServerList":[{"zoneid":"2"},{"zoneid":"1"}]}`,
		true,
	)

	if !result.OrderInsensitiveMatched {
		t.Fatalf("expected JSON order-insensitive match, got %+v", result)
	}
	if result.Reason != "content_mismatch_order_insensitive_equivalent" {
		t.Fatalf("expected order-insensitive reason, got %q", result.Reason)
	}
}

func TestCompareRenderedContentIgnoresXMLChildAndAttributeListOrderWhenEnabled(t *testing.T) {
	result := compareRenderedContent(
		`<daemonlist>
  <daemon type="gamed" ip="30.49.244.150" id="9702"/>
  <daemon type="gamed" ip="30.49.244.164" id="9701"/>
  <daemon type="arenad" ip="30.49.244.237" id="2" public_raid_3_dedicated_world_id="9702;9701"/>
</daemonlist>`,
		`<daemonlist>
  <daemon type="gamed" ip="30.49.244.164" id="9701"/>
  <daemon type="gamed" ip="30.49.244.150" id="9702"/>
  <daemon type="arenad" ip="30.49.244.237" id="2" public_raid_3_dedicated_world_id="9701;9702"/>
</daemonlist>`,
		true,
	)

	if !result.OrderInsensitiveMatched {
		t.Fatalf("expected XML order-insensitive match, got %+v", result)
	}
	if result.Reason != "content_mismatch_order_insensitive_equivalent" {
		t.Fatalf("expected order-insensitive reason, got %q", result.Reason)
	}
}

func TestCompareRenderedContentIgnoresLineOrderWhenEnabled(t *testing.T) {
	result := compareRenderedContent("a=1\nb=2\nc=3", "c=3\na=1\nb=2", true)

	if !result.OrderInsensitiveMatched {
		t.Fatalf("expected line order-insensitive match, got %+v", result)
	}
	if result.Reason != "content_mismatch_order_insensitive_equivalent" {
		t.Fatalf("expected order-insensitive reason, got %q", result.Reason)
	}
}

func TestCompareRenderedContentKeepsNonJSONMismatch(t *testing.T) {
	result := compareRenderedContent("port=1001", "port=1002", false)

	if result.Matched || result.JSONSemanticMatched || result.OrderInsensitiveMatched {
		t.Fatalf("expected plain text mismatch, got %+v", result)
	}
	if result.Reason != "content_mismatch" {
		t.Fatalf("expected content mismatch reason, got %q", result.Reason)
	}
}

func TestCompareRenderedContentForTemplateIgnoresHelpOnlyDifference(t *testing.T) {
	expected := `
***********************************
* NOW: 2026-06-04 17:49:51.815021 *
***********************************

old help
************
* end help *
************

111
222`
	actual := `
***********************************
* NOW: 2026-06-04 17:49:52.311083 *
***********************************

new help
************
* end help *
************

111
222`

	result := compareRenderedContentForTemplate("${HELP}\n111\n222", expected, actual, false)

	if !result.Ignored {
		t.Fatalf("expected HELP-only mismatch to be ignored, got %+v", result)
	}
	if result.Reason != "content_mismatch_help_only" {
		t.Fatalf("expected help-only reason, got %q", result.Reason)
	}
}

func TestCompareRenderedContentForTemplateKeepsNonHelpSuffixMismatch(t *testing.T) {
	expected := `
***********************************
* NOW: 2026-06-04 17:49:51.815021 *
***********************************

old help
************
* end help *
************

111`
	actual := `
***********************************
* NOW: 2026-06-04 17:49:52.311083 *
***********************************

new help
************
* end help *
************

222`

	result := compareRenderedContentForTemplate("${HELP}\n111", expected, actual, false)

	if result.Ignored {
		t.Fatalf("expected non-HELP suffix mismatch to remain actionable, got %+v", result)
	}
	if result.Reason != "content_mismatch" {
		t.Fatalf("expected content mismatch reason, got %q", result.Reason)
	}
}

func TestBizCompareReportJSONIncludesSemanticMatchCount(t *testing.T) {
	report := BizCompareReport{
		BizID:                   834,
		Total:                   2,
		Matched:                 1,
		JSONSemanticMatched:     1,
		OrderInsensitiveMatched: 1,
		Ignored:                 1,
		Diffs: []CompareRenderDiff{
			{Reason: "gsekit_render_error", ErrorMsg: "GSEKit preview failed"},
		},
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report failed: %v", err)
	}

	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("unmarshal report failed: %v", err)
	}
	if got := fields["json_semantic_matched"]; got != float64(1) {
		t.Fatalf("expected json_semantic_matched to be 1, got %v", got)
	}
	if got := fields["order_insensitive_matched"]; got != float64(1) {
		t.Fatalf("expected order_insensitive_matched to be 1, got %v", got)
	}
	if got := fields["ignored"]; got != float64(1) {
		t.Fatalf("expected ignored to be 1, got %v", got)
	}
	diffs, ok := fields["diffs"].([]any)
	if !ok || len(diffs) != 1 {
		t.Fatalf("expected one diff to be kept, got %v", fields["diffs"])
	}
	diff, ok := diffs[0].(map[string]any)
	if !ok || diff["reason"] != "gsekit_render_error" {
		t.Fatalf("expected gsekit_render_error diff to be kept, got %v", diffs[0])
	}
}

func TestAttachCompareRenderArtifactsWritesMismatchArtifactsByBizTemplateVersion(t *testing.T) {
	rootDir := t.TempDir()
	diff := CompareRenderDiff{
		ConfigTemplateID: 10676,
		ConfigVersionID:  107999,
		BkProcessID:      19768057,
		TemplateName:     "dir/managemenweb_web.config",
		Reason:           "content_mismatch",
	}

	err := attachCompareRenderArtifacts(rootDir, 5016710, &diff, compareRenderArtifactContents{
		Template: artifactContent("raw template"),
		Expected: artifactContent("gsekit rendered"),
		Actual:   artifactContent("bscp rendered"),
	})
	if err != nil {
		t.Fatalf("attach artifacts failed: %v", err)
	}
	if diff.Artifacts == nil {
		t.Fatal("expected artifacts to be attached")
	}

	expectedDir := filepath.Join(rootDir, "biz_5016710_template_10676_version_107999")
	expectedTemplatePath := filepath.Join(expectedDir, "template.mako")
	if got := diff.Artifacts.TemplatePath; got != expectedTemplatePath {
		t.Fatalf("expected template artifact path %q, got %q", expectedTemplatePath, got)
	}
	if diff.Artifacts.ErrorPath != "" {
		t.Fatalf("expected only three mismatch artifact files, got %+v", diff.Artifacts)
	}

	assertFileContent(t, diff.Artifacts.TemplatePath, "raw template")
	assertFileContent(t, diff.Artifacts.ExpectedPath, "gsekit rendered")
	assertFileContent(t, diff.Artifacts.ActualPath, "bscp rendered")
}

func TestAttachCompareRenderArtifactsWritesRenderErrorArtifacts(t *testing.T) {
	rootDir := t.TempDir()
	diff := CompareRenderDiff{
		ConfigTemplateID: 10676,
		ConfigVersionID:  107999,
		BkProcessID:      19768057,
		TemplateName:     "managemenweb_web.config",
		Reason:           "render_error",
	}

	err := attachCompareRenderArtifacts(rootDir, 5016710, &diff, compareRenderArtifactContents{
		Template: artifactContent("raw template"),
		Error:    artifactContent("render failed"),
	})
	if err != nil {
		t.Fatalf("attach artifacts failed: %v", err)
	}
	if diff.Artifacts == nil {
		t.Fatal("expected render error artifacts to be attached")
	}

	expectedDir := filepath.Join(rootDir, "biz_5016710_template_10676_version_107999")
	assertFileContent(t, filepath.Join(expectedDir, "template.mako"), "raw template")
	assertFileContent(t, filepath.Join(expectedDir, "error.txt"), "render failed")
}

func assertFileContent(t *testing.T, path string, expected string) {
	t.Helper()

	if path == "" {
		t.Fatal("expected artifact path to be set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact %s failed: %v", path, err)
	}
	if string(data) != expected {
		t.Fatalf("expected artifact %s content %q, got %q", path, expected, string(data))
	}
}
