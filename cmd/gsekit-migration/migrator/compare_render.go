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
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	processorCmdb "github.com/TencentBlueKing/bk-bscp/internal/processor/cmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/render"
)

// CompareRenderReport contains the overall comparison report
type CompareRenderReport struct {
	Success    bool               `json:"success"`
	BizReports []BizCompareReport `json:"biz_reports"`
}

// BizCompareReport contains comparison results for a single biz
type BizCompareReport struct {
	BizID        uint32              `json:"biz_id"`
	Total        int                 `json:"total"`
	Matched      int                 `json:"matched"`
	Mismatched   int                 `json:"mismatched"`
	RenderFailed int                 `json:"render_failed"`
	Skipped      int                 `json:"skipped"`
	Diffs        []CompareRenderDiff `json:"diffs,omitempty"`
}

// CompareRenderDiff contains details of a single mismatched comparison
type CompareRenderDiff struct {
	ConfigTemplateID int64  `json:"config_template_id"`
	ConfigVersionID  int64  `json:"config_version_id"`
	BkProcessID      int64  `json:"bk_process_id"`
	TemplateName     string `json:"template_name"`
	Reason           string `json:"reason"` // "content_mismatch" / "render_error" / "gsekit_render_error" / "ginclude_expand_error"
	ExpectedPreview  string `json:"expected_preview,omitempty"`
	ActualPreview    string `json:"actual_preview,omitempty"`
	ErrorMsg         string `json:"error_msg,omitempty"`
}

// CompareRenderOptions holds options for compare-render command
type CompareRenderOptions struct {
	ShowDiff         bool
	DiffContextLines int
	OutputFile       string
	RenderTimeout    time.Duration
	RequestInterval  time.Duration
}

// templateWithVersion holds a config template joined with its latest version
type templateWithVersion struct {
	Template GSEKitConfigTemplate
	Version  GSEKitConfigTemplateVersion
}

// CompareRender performs a per-template render comparison.
// For each config template, it finds a bound process via the binding table,
// selects the first process instance (by primary key ASC, matching GSEKit's
// ProcessInst.get_single_inst()), then renders via both GSEKit preview API
// and BSCP Mako renderer, and compares the two outputs.
//
// nolint:funlen
func (m *Migrator) CompareRender(opts CompareRenderOptions) (*CompareRenderReport, error) {
	ctx := context.Background()
	report := &CompareRenderReport{Success: true}

	if m.gsekitClient == nil {
		return nil, fmt.Errorf("gsekit config is required for compare-render (set gsekit.endpoint in config)")
	}

	cc.SetG(cc.GlobalSettings{
		FeatureFlags: cc.FeatureFlags{EnableMultiTenantMode: false},
	})

	cmdbSvc, err := bkcmdb.New(&cc.CMDBConfig{
		Host:       m.cfg.CMDB.Endpoint,
		AppCode:    m.cfg.CMDB.AppCode,
		AppSecret:  m.cfg.CMDB.AppSecret,
		BkUserName: m.cfg.CMDB.Username,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("create bkcmdb service failed: %w", err)
	}

	bscpRenderer, err := render.NewRenderer(render.WithTimeout(opts.RenderTimeout))
	if err != nil {
		return nil, fmt.Errorf("create renderer failed: %w", err)
	}

	for _, bizID := range m.cfg.Migration.BizIDs {
		log.Printf("=== Comparing render results for biz %d ===", bizID)

		bizReport, err := m.compareRenderForBiz(ctx, bizID, bscpRenderer, cmdbSvc, opts)
		if err != nil {
			return nil, fmt.Errorf("compare render for biz %d failed: %w", bizID, err)
		}

		if bizReport.Mismatched > 0 || bizReport.RenderFailed > 0 {
			report.Success = false
		}

		report.BizReports = append(report.BizReports, *bizReport)
	}

	return report, nil
}

// compareRenderForBiz performs render comparison for a single biz.
// nolint:funlen,gocyclo
func (m *Migrator) compareRenderForBiz(
	ctx context.Context,
	bizID uint32,
	bscpRenderer *render.Renderer,
	cmdbSvc bkcmdb.Service,
	opts CompareRenderOptions,
) (*BizCompareReport, error) {
	bizReport := &BizCompareReport{BizID: bizID}

	// Step 1: 查询该业务下的所有配置模板
	var templates []GSEKitConfigTemplate
	if err := m.sourceDB.Where("bk_biz_id = ?", bizID).Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("fetch config templates for biz %d failed: %w", bizID, err)
	}
	log.Printf("  Found %d config templates for biz %d", len(templates), bizID)

	if len(templates) == 0 {
		return bizReport, nil
	}

	// Step 2: 为每个模板查找其最新的已发布版本（排除草稿），
	// 没有已发布版本的模板将被跳过
	templateVersionMap := make(map[int64]*templateWithVersion)
	for _, tmpl := range templates {
		var version GSEKitConfigTemplateVersion
		err := m.sourceDB.Where("config_template_id = ? AND is_draft = ?", tmpl.ConfigTemplateID, false).
			Order("config_version_id DESC").
			First(&version).Error
		if err != nil {
			log.Printf("  Skip template %d/%s: no published version", tmpl.ConfigTemplateID, tmpl.TemplateName)
			continue
		}
		templateVersionMap[tmpl.ConfigTemplateID] = &templateWithVersion{
			Template: tmpl,
			Version:  version,
		}
	}
	log.Printf("  %d templates have published versions", len(templateVersionMap))

	if len(templateVersionMap) == 0 {
		return bizReport, nil
	}

	// Step 3: 通过绑定关系表为每个模板找到一个关联的进程，
	// 优先使用 INSTANCE 绑定（直接进程ID），其次使用 TEMPLATE 绑定（按进程模板ID匹配）
	templateProcessMap := make(map[int64]int64)
	for configTemplateID := range templateVersionMap {
		processID, err := m.resolveTemplateProcess(bizID, configTemplateID)
		if err != nil {
			return nil, fmt.Errorf("resolve process for template %d failed: %w", configTemplateID, err)
		}
		if processID > 0 {
			templateProcessMap[configTemplateID] = processID
		}
	}

	// Step 4: 收集所有关联的去重进程ID，批量查询进程记录和进程实例
	// processMap: bk_process_id → 进程记录
	// firstInstMap: bk_process_id → 该进程的第一个实例（按主键 ASC，与 GSEKit 的 get_single_inst 一致）
	uniqueProcessIDs := collectUniqueValues(templateProcessMap)
	if len(uniqueProcessIDs) == 0 {
		for _, tv := range templateVersionMap {
			bizReport.Total++
			bizReport.Skipped++
			log.Printf("  Skip template %d/%s: no bound process",
				tv.Template.ConfigTemplateID, tv.Template.TemplateName)
		}
		return bizReport, nil
	}

	processMap, err := m.batchFetchProcesses(uniqueProcessIDs)
	if err != nil {
		return nil, fmt.Errorf("batch fetch processes failed: %w", err)
	}

	firstInstMap, err := m.batchFetchFirstInstances(uniqueProcessIDs)
	if err != nil {
		return nil, fmt.Errorf("batch fetch process instances failed: %w", err)
	}

	// Step 5: 从进程记录中提取 CMDB 关联 ID，批量查询补全数据（集群名、模块名、服务实例、进程详情），
	// 这些数据用于构建渲染上下文
	setIDs, moduleIDs, svcInstIDs, processDetailIDs := collectCMDBQueryIDs(processMap)

	setNames, err := m.cmdbClient.FindSetBatch(ctx, bizID, setIDs)
	if err != nil {
		return nil, fmt.Errorf("FindSetBatch for biz %d failed: %w", bizID, err)
	}

	moduleNames, err := m.cmdbClient.FindModuleBatch(ctx, bizID, moduleIDs)
	if err != nil {
		return nil, fmt.Errorf("FindModuleBatch for biz %d failed: %w", bizID, err)
	}

	svcInstDetails, err := m.cmdbClient.ListServiceInstanceDetail(ctx, bizID, svcInstIDs)
	if err != nil {
		return nil, fmt.Errorf("ListServiceInstanceDetail for biz %d failed: %w", bizID, err)
	}

	processDetails, err := m.cmdbClient.ListProcessDetailByIds(ctx, bizID, processDetailIDs)
	if err != nil {
		return nil, fmt.Errorf("ListProcessDetailByIds for biz %d failed: %w", bizID, err)
	}

	// Step 6: 创建 CC 拓扑 XML 服务，获取业务全局变量（set/module/host 的属性列表），
	// 并初始化 CC XML 缓存（按 BkSetEnv 缓存，避免相同环境类型重复构建拓扑 XML）
	topoSvc := processorCmdb.NewCCTopoXMLService(int(bizID), cmdbSvc)

	globalVars, err := topoSvc.GetBizGlobalVariablesMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetBizGlobalVariablesMap for biz %d failed: %w", bizID, err)
	}

	ccXMLCache := make(map[string]string)

	// Step 7: 构建模板名称 → 模板内容的查找表，用于展开 Ginclude 指令（模板间引用）
	templateContentByName := make(map[string]string)
	for _, tv := range templateVersionMap {
		templateContentByName[tv.Template.TemplateName] = string(tv.Version.Content)
	}

	// Step 8: 按模板ID升序遍历，保证每次运行的处理顺序和报告输出一致
	sortedTemplateIDs := sortedKeys(templateVersionMap)

	for _, configTemplateID := range sortedTemplateIDs {
		tv := templateVersionMap[configTemplateID]
		bizReport.Total++

		// 跳过没有绑定进程的模板
		processID, hasBound := templateProcessMap[configTemplateID]
		if !hasBound {
			bizReport.Skipped++
			log.Printf("  Skip template %d/%s: no bound process",
				configTemplateID, tv.Template.TemplateName)
			continue
		}

		// 跳过进程记录或进程实例缺失的模板
		proc := processMap[processID]
		inst := firstInstMap[processID]
		if proc == nil || inst == nil {
			bizReport.Skipped++
			log.Printf("  Skip template %d/%s: process %d has no data or instances",
				configTemplateID, tv.Template.TemplateName, processID)
			continue
		}

		// 获取该进程所属环境类型对应的 CC 拓扑 XML，命中缓存则直接复用
		ccXML, ok := ccXMLCache[proc.BkSetEnv]
		if !ok {
			ccXML, err = topoSvc.GetTopoTreeXML(ctx, proc.BkSetEnv)
			if err != nil {
				return nil, fmt.Errorf("GetTopoTreeXML for biz %d (setEnv=%s) failed: %w",
					bizID, proc.BkSetEnv, err)
			}
			ccXMLCache[proc.BkSetEnv] = ccXML
		}

		// 从 CMDB 批量查询结果中提取当前进程的补全信息（服务实例名、功能名、工作路径、PID 文件路径）
		svcInstName := ""
		if detail, found := svcInstDetails[proc.ServiceInstanceID]; found {
			svcInstName = detail.Name
		}

		detail := processDetails[proc.BkProcessID]
		funcName := ""
		workPath := ""
		pidFile := ""
		if detail != nil {
			funcName = detail.BkFuncName
			workPath = detail.WorkPath
			pidFile = detail.PidFile
		}

		// 组装渲染上下文，包含进程实例序号、拓扑信息（集群/模块/主机）、CC XML、全局变量等
		processCtx := render.BuildProcessContext(render.ProcessContextParams{
			ModuleInstSeq: inst.InstID,
			HostInstSeq:   inst.LocalInstID,
			SetName:       setNames[proc.BkSetID],
			ModuleName:    moduleNames[proc.BkModuleID],
			ServiceName:   svcInstName,
			ProcessName:   proc.BkProcessName,
			ProcessID:     int(proc.BkProcessID),
			FuncName:      funcName,
			WorkPath:      workPath,
			PidFile:       pidFile,
			HostInnerIP:   proc.BkHostInnerip,
			CloudID:       int(proc.BkCloudID),
			CcXML:         ccXML,
			GlobalVariables: map[string]interface{}{
				"biz_global_variables": globalVars,
			},
		})

		// 请求间隔限流，避免对 GSEKit 预览接口产生过高频率的请求
		if opts.RequestInterval > 0 {
			time.Sleep(opts.RequestInterval)
		}

		// --- GSEKit 侧：调用预览 API 渲染模板 ---
		gsekitRendered, gsekitRenderErr := m.gsekitClient.PreviewConfigTemplate(
			ctx, bizID, string(tv.Version.Content), processID)

		if gsekitRenderErr != nil {
			bizReport.RenderFailed++
			bizReport.Diffs = append(bizReport.Diffs, CompareRenderDiff{
				ConfigTemplateID: configTemplateID,
				ConfigVersionID:  tv.Version.ConfigVersionID,
				BkProcessID:      processID,
				TemplateName:     tv.Template.TemplateName,
				Reason:           "gsekit_render_error",
				ErrorMsg:         gsekitRenderErr.Error(),
			})
			continue
		}

		// --- BSCP 侧：用 Mako 渲染器渲染模板 ---
		templateContent := string(tv.Version.Content)

		// 展开模板中的 Ginclude 指令（模板间引用），最大递归深度 10 层
		templateContent, expandErr := render.ExpandGinclude(templateContent, func(name string) (string, error) {
			if c, found := templateContentByName[name]; found {
				return c, nil
			}
			return "", fmt.Errorf("referenced template %q not found in biz %d", name, bizID)
		}, 10)
		if expandErr != nil {
			bizReport.RenderFailed++
			bizReport.Diffs = append(bizReport.Diffs, CompareRenderDiff{
				ConfigTemplateID: configTemplateID,
				ConfigVersionID:  tv.Version.ConfigVersionID,
				BkProcessID:      processID,
				TemplateName:     tv.Template.TemplateName,
				Reason:           "ginclude_expand_error",
				ErrorMsg:         expandErr.Error(),
			})
			continue
		}

		bscpRendered, err := bscpRenderer.RenderWithContext(ctx, templateContent, processCtx)
		if err != nil {
			bizReport.RenderFailed++
			bizReport.Diffs = append(bizReport.Diffs, CompareRenderDiff{
				ConfigTemplateID: configTemplateID,
				ConfigVersionID:  tv.Version.ConfigVersionID,
				BkProcessID:      processID,
				TemplateName:     tv.Template.TemplateName,
				Reason:           "render_error",
				ErrorMsg:         err.Error(),
			})
			continue
		}

		// 对比两端渲染结果（去除尾部空白后比较）
		expectedStr := strings.TrimRight(gsekitRendered, "\n\r \t")
		actualStr := strings.TrimRight(bscpRendered, "\n\r \t")

		if expectedStr == actualStr {
			bizReport.Matched++
		} else {
			bizReport.Mismatched++
			bizReport.Diffs = append(bizReport.Diffs, CompareRenderDiff{
				ConfigTemplateID: configTemplateID,
				ConfigVersionID:  tv.Version.ConfigVersionID,
				BkProcessID:      processID,
				TemplateName:     tv.Template.TemplateName,
				Reason:           "content_mismatch",
				ExpectedPreview:  truncateStr(expectedStr, 200),
				ActualPreview:    truncateStr(actualStr, 200),
			})

			if opts.ShowDiff {
				printUnifiedDiff(configTemplateID, expectedStr, actualStr, opts.DiffContextLines)
			}
		}
	}

	log.Printf("  Biz %d: total=%d matched=%d mismatched=%d render_failed=%d skipped=%d",
		bizID, bizReport.Total, bizReport.Matched, bizReport.Mismatched,
		bizReport.RenderFailed, bizReport.Skipped)

	return bizReport, nil
}

// resolveTemplateProcess finds one bound process for a template via the binding table.
// Priority: INSTANCE binding (direct bk_process_id) > TEMPLATE binding (find first
// process with matching process_template_id).
// Returns 0 if no bound process is found.
func (m *Migrator) resolveTemplateProcess(bizID uint32, configTemplateID int64) (int64, error) {
	var bindings []GSEKitConfigTemplateBindingRelationship
	if err := m.sourceDB.Where("config_template_id = ? AND bk_biz_id = ?", configTemplateID, bizID).
		Find(&bindings).Error; err != nil {
		return 0, fmt.Errorf("fetch bindings for template %d failed: %w", configTemplateID, err)
	}

	// Prefer INSTANCE binding (direct process ID)
	for _, b := range bindings {
		if b.ProcessObjectType == "INSTANCE" {
			return b.ProcessObjectID, nil
		}
	}

	// Fallback to TEMPLATE binding: find first process with matching process_template_id
	for _, b := range bindings {
		if b.ProcessObjectType == "TEMPLATE" {
			var proc GSEKitProcess
			err := m.sourceDB.Where("process_template_id = ? AND bk_biz_id = ?", b.ProcessObjectID, bizID).
				Order("bk_process_id ASC").First(&proc).Error
			if err == nil {
				return proc.BkProcessID, nil
			}
		}
	}

	return 0, nil
}

// batchFetchProcesses fetches process records by bk_process_id list.
func (m *Migrator) batchFetchProcesses(processIDs []int64) (map[int64]*GSEKitProcess, error) {
	result := make(map[int64]*GSEKitProcess, len(processIDs))
	var procs []GSEKitProcess
	if err := m.sourceDB.Where("bk_process_id IN ?", processIDs).Find(&procs).Error; err != nil {
		return nil, fmt.Errorf("fetch processes failed: %w", err)
	}
	for i := range procs {
		result[procs[i].BkProcessID] = &procs[i]
	}
	return result, nil
}

// batchFetchFirstInstances fetches the first process instance (by primary key
// id ASC) for each process, matching GSEKit's ProcessInst.get_single_inst()
// which uses Django's .first() (default ordering by pk).
func (m *Migrator) batchFetchFirstInstances(processIDs []int64) (map[int64]*GSEKitProcessInst, error) {
	result := make(map[int64]*GSEKitProcessInst, len(processIDs))
	var insts []GSEKitProcessInst
	if err := m.sourceDB.Where("bk_process_id IN ?", processIDs).
		Order("id ASC").Find(&insts).Error; err != nil {
		return nil, fmt.Errorf("fetch process instances failed: %w", err)
	}
	for i := range insts {
		if _, exists := result[insts[i].BkProcessID]; !exists {
			result[insts[i].BkProcessID] = &insts[i]
		}
	}
	return result, nil
}

// collectCMDBQueryIDs collects unique CMDB IDs from a process map for batch queries.
func collectCMDBQueryIDs(processMap map[int64]*GSEKitProcess) (
	setIDs, moduleIDs, svcInstIDs, processIDs []int64,
) {
	setIDSet := make(map[int64]bool)
	moduleIDSet := make(map[int64]bool)
	svcInstIDSet := make(map[int64]bool)
	processIDSet := make(map[int64]bool)

	for _, proc := range processMap {
		setIDSet[proc.BkSetID] = true
		moduleIDSet[proc.BkModuleID] = true
		svcInstIDSet[proc.ServiceInstanceID] = true
		processIDSet[proc.BkProcessID] = true
	}

	return mapKeysInt64(setIDSet), mapKeysInt64(moduleIDSet),
		mapKeysInt64(svcInstIDSet), mapKeysInt64(processIDSet)
}

// collectUniqueValues returns unique values from a map[int64]int64.
func collectUniqueValues(m map[int64]int64) []int64 {
	seen := make(map[int64]bool, len(m))
	result := make([]int64, 0, len(m))
	for _, v := range m {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// mapKeysInt64 returns the keys of a map[int64]bool as a sorted slice.
func mapKeysInt64(m map[int64]bool) []int64 {
	result := make([]int64, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

// sortedKeys returns the keys of templateVersionMap sorted in ascending order.
func sortedKeys(m map[int64]*templateWithVersion) []int64 {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// truncateStr truncates a string to maxLen runes, appending "..." if truncated.
// Uses rune-based slicing to avoid splitting multi-byte UTF-8 characters.
func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// printUnifiedDiff prints a unified diff between expected and actual content
func printUnifiedDiff(templateID int64, expected, actual string, contextLines int) {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(expected),
		B:        difflib.SplitLines(actual),
		FromFile: fmt.Sprintf("GSEKit (template %d)", templateID),
		ToFile:   fmt.Sprintf("BSCP rendered (template %d)", templateID),
		Context:  contextLines,
	}
	text, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		log.Printf("  Error generating diff for template %d: %v", templateID, err)
		return
	}
	if text != "" {
		fmt.Printf("\n--- Diff for template %d ---\n%s\n", templateID, text)
	}
}

// PrintCompareRenderReport prints the comparison report to stdout
func (m *Migrator) PrintCompareRenderReport(report *CompareRenderReport) {
	fmt.Println("\n========== Compare Render Report ==========")
	fmt.Printf("Status: %s\n", boolToStatus(report.Success))

	for _, biz := range report.BizReports {
		fmt.Printf("\nBiz %d:\n", biz.BizID)
		fmt.Printf("  Total:         %d\n", biz.Total)
		fmt.Printf("  Matched:       %d\n", biz.Matched)
		fmt.Printf("  Mismatched:    %d\n", biz.Mismatched)
		fmt.Printf("  Render Failed: %d\n", biz.RenderFailed)
		fmt.Printf("  Skipped:       %d\n", biz.Skipped)

		if len(biz.Diffs) > 0 {
			fmt.Printf("\n  Differences (%d):\n", len(biz.Diffs))
			for _, d := range biz.Diffs {
				fmt.Printf("    - Template %d/%s (version=%d, process=%d): %s",
					d.ConfigTemplateID, d.TemplateName, d.ConfigVersionID, d.BkProcessID, d.Reason)
				if d.ErrorMsg != "" {
					fmt.Printf(" [%s]", d.ErrorMsg)
				}
				fmt.Println()
			}
		}
	}
	fmt.Println("=============================================")
}

// WriteCompareRenderReportJSON writes the report as JSON to the specified file
func (m *Migrator) WriteCompareRenderReportJSON(report *CompareRenderReport, outputFile string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report to JSON failed: %w", err)
	}
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return fmt.Errorf("write report to %s failed: %w", outputFile, err)
	}
	log.Printf("Report written to %s", outputFile)
	return nil
}
