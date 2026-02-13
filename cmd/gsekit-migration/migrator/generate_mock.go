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
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/TencentBlueKing/bk-bscp/cmd/gsekit-migration/config"
)

// MockGenerator generates mock-data.sql from real CMDB data.
type MockGenerator struct {
	cmdbClient   *realCMDBClient
	bizID        uint32
	maxProcesses int
}

// NewMockGenerator creates a new MockGenerator.
func NewMockGenerator(cmdbCfg *config.CMDBConfig, bizID uint32, maxProcesses int) *MockGenerator {
	return &MockGenerator{
		cmdbClient:   NewRealCMDBClient(cmdbCfg).(*realCMDBClient),
		bizID:        bizID,
		maxProcesses: maxProcesses,
	}
}

// processRow holds data for a gsekit_process row.
type processRow struct {
	BkProcessID       int
	BkBizID           uint32
	Expression        string
	BkHostInnerip     string
	BkCloudID         int
	BkSetEnv          string
	BkSetID           int
	BkModuleID        int
	ServiceTemplateID int
	ServiceInstanceID int
	BkProcessName     string
	ProcessTemplateID int
	ProcessStatus     int
	IsAuto            int
	BkAgentID         string
	BkHostID          int
	ProcNum           int
}

// processInstRow holds data for a gsekit_processinst row.
type processInstRow struct {
	ID                 int
	BkBizID            uint32
	BkHostNum          int
	BkHostInnerip      string
	BkCloudID          int
	BkProcessID        int
	BkModuleID         int
	BkProcessName      string
	InstID             int
	ProcessStatus      int
	IsAuto             int
	LocalInstID        int
	LocalInstIDUniqKey string
	ProcNum            int
	BkAgentID          string
	BkHostID           int
}

// configTplRow holds data for a gsekit_configtemplate row.
type configTplRow struct {
	ID           int
	BkBizID      uint32
	TemplateName string
	FileName     string
	AbsPath      string
	Owner        string
	Group        string
	Filemode     string
}

// configVerRow holds data for a gsekit_configtemplateversion row.
type configVerRow struct {
	ID               int
	ConfigTemplateID int
	Description      string
	Content          string
	IsDraft          int
	IsActive         int
	FileFormat       string
}

// configBindRow holds data for a gsekit_configtemplatebindingrelationship row.
type configBindRow struct {
	ID                int
	BkBizID           uint32
	ConfigTemplateID  int
	ProcessObjectType string
	ProcessObjectID   int
}

// configInstRow holds data for a gsekit_configinstance row.
type configInstRow struct {
	ID               int
	ConfigVersionID  int
	ConfigTemplateID int
	BkProcessID      int
	InstID           int
	Content          string
	Path             string
	IsLatest         int
	IsReleased       int
	SHA256           string
	Expression       string
	Name             string
}

// rawProcess holds extracted process data from CMDB service instances.
type rawProcess struct {
	BkProcessID       int
	BkProcessName     string
	BkFuncName        string
	BkBizID           int
	BkModuleID        int
	ServiceInstanceID int
	ServiceTemplateID int
	ProcessTemplateID int
	BkHostID          int
	ProcNum           int
}

// cmdbEnrichment holds enriched data fetched from CMDB for mock generation.
type cmdbEnrichment struct {
	moduleSetMap   map[int]int                  // module_id → set_id
	hostMap        map[int]*CMDBHostInfo        // host_id → host info
	processDetails map[int64]*CMDBProcessDetail // process_id → process detail
}

// Generate queries CMDB for the configured biz and writes mock-data.sql to outputPath.
func (g *MockGenerator) Generate(outputPath string) error {
	ctx := context.Background()
	bizID := g.bizID

	log.Printf("Generating mock data for biz_id=%d (max_processes=%d)", bizID, g.maxProcesses)

	// 1. List all service instances
	svcInstances, err := g.listAllServiceInstances(ctx, bizID)
	if err != nil {
		return fmt.Errorf("list service instances failed: %w", err)
	}
	log.Printf("  Fetched %d service instances from CMDB", len(svcInstances))

	if len(svcInstances) == 0 {
		return fmt.Errorf("no service instances found for biz %d", bizID)
	}

	// 2. Extract processes and collect unique IDs
	processes, moduleIDSet, hostIDSet, processIDSet := g.extractProcesses(svcInstances)
	log.Printf("  Using %d processes for mock data", len(processes))

	// 3. Enrich from CMDB
	enrichment := g.enrichFromCMDB(ctx, bizID, moduleIDSet, hostIDSet, processIDSet)

	// 4. Build gsekit_process rows
	rng := rand.New(rand.NewSource(time.Now().UnixNano())) // nolint:gosec
	procRows := g.buildProcessRows(processes, enrichment, rng)

	// 5. Build gsekit_processinst rows
	instRows := g.buildProcessInstRows(procRows, rng)

	// 6. Build config data
	configTpls, configVers, configBinds, configInsts := g.buildConfigData(procRows)

	// 7. Write SQL
	log.Printf("  Writing SQL to %s", outputPath)
	return g.writeSQL(outputPath, procRows, instRows, configTpls, configVers, configBinds, configInsts)
}

// extractProcesses extracts raw processes from service instances and collects unique IDs.
// nolint:gocyclo
func (g *MockGenerator) extractProcesses(svcInstances []*CMDBServiceInstance) (
	[]rawProcess, map[int64]bool, map[int64]bool, map[int64]bool) {

	var processes []rawProcess
	moduleIDSet := make(map[int64]bool)
	hostIDSet := make(map[int64]bool)
	processIDSet := make(map[int64]bool)

	for _, svcInst := range svcInstances {
		moduleIDSet[int64(svcInst.BkModuleID)] = true
		hostIDSet[int64(svcInst.BkHostID)] = true
		for _, pi := range svcInst.ProcessInstances {
			if pi.Property == nil || pi.Relation == nil {
				continue
			}
			if processIDSet[int64(pi.Property.BkProcessID)] {
				continue
			}
			processIDSet[int64(pi.Property.BkProcessID)] = true
			hostIDSet[int64(pi.Relation.BkHostID)] = true
			processes = append(processes, rawProcess{
				BkProcessID:       pi.Property.BkProcessID,
				BkProcessName:     pi.Property.BkProcessName,
				BkFuncName:        pi.Property.BkFuncName,
				BkBizID:           svcInst.BkBizID,
				BkModuleID:        svcInst.BkModuleID,
				ServiceInstanceID: svcInst.ID,
				ServiceTemplateID: svcInst.ServiceTemplateID,
				ProcessTemplateID: pi.Relation.ProcessTemplateID,
				BkHostID:          pi.Relation.BkHostID,
				ProcNum:           pi.Property.ProcNum,
			})
		}
	}

	// Cap at maxProcesses
	if len(processes) > g.maxProcesses {
		processes = processes[:g.maxProcesses]
		moduleIDSet = make(map[int64]bool)
		hostIDSet = make(map[int64]bool)
		processIDSet = make(map[int64]bool)
		for _, p := range processes {
			moduleIDSet[int64(p.BkModuleID)] = true
			hostIDSet[int64(p.BkHostID)] = true
			processIDSet[int64(p.BkProcessID)] = true
		}
	}

	return processes, moduleIDSet, hostIDSet, processIDSet
}

// enrichFromCMDB fetches module, host, and process details from CMDB.
func (g *MockGenerator) enrichFromCMDB(ctx context.Context, bizID uint32,
	moduleIDSet, hostIDSet, processIDSet map[int64]bool) cmdbEnrichment {

	moduleIDs := mapKeys(moduleIDSet)
	hostIDs := mapKeys(hostIDSet)
	processIDs := mapKeys(processIDSet)

	moduleInfos, _ := g.cmdbClient.FindModuleBatch(ctx, bizID, moduleIDs)

	moduleSetMap := make(map[int]int)
	for _, mi := range moduleInfos {
		if mi.BkSetID > 0 {
			moduleSetMap[mi.BkModuleID] = mi.BkSetID
		}
	}

	allHosts, _ := g.cmdbClient.ListBizHosts(ctx, bizID)
	processDetails, _ := g.cmdbClient.ListProcessDetailByIds(ctx, bizID, processIDs)

	hostMap := make(map[int]*CMDBHostInfo)
	for _, hid := range hostIDs {
		if h, ok := allHosts[hid]; ok {
			hostMap[int(hid)] = h
		}
	}

	return cmdbEnrichment{
		moduleSetMap:   moduleSetMap,
		hostMap:        hostMap,
		processDetails: processDetails,
	}
}

// buildProcessRows builds gsekit_process rows from raw processes and CMDB enrichment.
func (g *MockGenerator) buildProcessRows(processes []rawProcess,
	enrichment cmdbEnrichment, rng *rand.Rand) []processRow {

	bizID := g.bizID
	var procRows []processRow

	for _, p := range processes {
		ip := ""
		cloudID := 0
		agentID := ""
		if h, ok := enrichment.hostMap[p.BkHostID]; ok {
			ip = h.BkHostInnerIP
			cloudID = h.BkCloudID
			agentID = h.BkAgentID
		}

		setEnv := "3" // production
		if rng.Intn(3) == 0 {
			setEnv = "1" // test
		}

		procStatus := rng.Intn(3) // 0=UNREGISTERED, 1=RUNNING, 2=TERMINATED
		isAuto := 0
		if rng.Intn(2) == 1 {
			isAuto = 1
		}

		procNum := p.ProcNum
		if procNum <= 0 {
			if detail, ok := enrichment.processDetails[int64(p.BkProcessID)]; ok && detail.ProcNum > 0 {
				procNum = detail.ProcNum
			} else {
				procNum = 1
			}
		}

		expression := fmt.Sprintf("%s:%d:%s", ip, cloudID, p.BkProcessName)

		realSetID := 0
		if sid, ok := enrichment.moduleSetMap[p.BkModuleID]; ok {
			realSetID = sid
		}

		procRows = append(procRows, processRow{
			BkProcessID:       p.BkProcessID,
			BkBizID:           bizID,
			Expression:        expression,
			BkHostInnerip:     ip,
			BkCloudID:         cloudID,
			BkSetEnv:          setEnv,
			BkSetID:           realSetID,
			BkModuleID:        p.BkModuleID,
			ServiceTemplateID: p.ServiceTemplateID,
			ServiceInstanceID: p.ServiceInstanceID,
			BkProcessName:     p.BkProcessName,
			ProcessTemplateID: p.ProcessTemplateID,
			ProcessStatus:     procStatus,
			IsAuto:            isAuto,
			BkAgentID:         agentID,
			BkHostID:          p.BkHostID,
			ProcNum:           procNum,
		})
	}

	return procRows
}

// buildProcessInstRows builds gsekit_processinst rows from process rows.
func (g *MockGenerator) buildProcessInstRows(procRows []processRow, rng *rand.Rand) []processInstRow {
	bizID := g.bizID
	var instRows []processInstRow
	instIDCounter := 0

	for _, pr := range procRows {
		for instIdx := 1; instIdx <= pr.ProcNum; instIdx++ {
			instIDCounter++
			procStatus := rng.Intn(3)
			uniqKey := fmt.Sprintf("%s:%d:%s:%d", pr.BkHostInnerip, pr.BkCloudID, pr.BkProcessName, instIdx)

			instRows = append(instRows, processInstRow{
				ID:                 instIDCounter,
				BkBizID:            bizID,
				BkHostNum:          instIDCounter,
				BkHostInnerip:      pr.BkHostInnerip,
				BkCloudID:          pr.BkCloudID,
				BkProcessID:        pr.BkProcessID,
				BkModuleID:         pr.BkModuleID,
				BkProcessName:      pr.BkProcessName,
				InstID:             instIdx,
				ProcessStatus:      procStatus,
				IsAuto:             pr.IsAuto,
				LocalInstID:        instIdx,
				LocalInstIDUniqKey: uniqKey,
				ProcNum:            pr.ProcNum,
				BkAgentID:          pr.BkAgentID,
				BkHostID:           pr.BkHostID,
			})
		}
	}

	return instRows
}

// buildConfigData builds synthetic config template, version, binding, and instance data.
func (g *MockGenerator) buildConfigData(procRows []processRow) (
	[]configTplRow, []configVerRow, []configBindRow, []configInstRow) {

	bizID := g.bizID

	// Collect unique process names in order
	processNameSet := make(map[string]bool)
	var uniqueProcessNames []string
	for _, pr := range procRows {
		if !processNameSet[pr.BkProcessName] {
			processNameSet[pr.BkProcessName] = true
			uniqueProcessNames = append(uniqueProcessNames, pr.BkProcessName)
		}
	}

	var configTpls []configTplRow
	var configVers []configVerRow
	var configBinds []configBindRow
	var configInsts []configInstRow
	tplID, verID, bindID, instID := 0, 0, 0, 0

	for _, pname := range uniqueProcessNames {
		tplID++
		fileName := pname + ".conf"
		absPath := "/etc/" + pname

		configTpls = append(configTpls, configTplRow{
			ID: tplID, BkBizID: bizID, TemplateName: pname + "_config",
			FileName: fileName, AbsPath: absPath, Owner: "root", Group: "root", Filemode: "0644",
		})

		verID++
		content := fmt.Sprintf("# Config for %s\n# Auto-generated mock data\nkey = value\n", pname)
		configVers = append(configVers, configVerRow{
			ID: verID, ConfigTemplateID: tplID, Description: "mock version",
			Content: content, IsDraft: 0, IsActive: 1, FileFormat: "python",
		})

		activeVerID := verID

		for _, pr := range procRows {
			if pr.BkProcessName != pname {
				continue
			}
			bindID++
			configBinds = append(configBinds, configBindRow{
				ID: bindID, BkBizID: bizID, ConfigTemplateID: tplID,
				ProcessObjectType: "INSTANCE", ProcessObjectID: pr.BkProcessID,
			})

			instID++
			configInsts = append(configInsts, configInstRow{
				ID: instID, ConfigVersionID: activeVerID, ConfigTemplateID: tplID,
				BkProcessID: pr.BkProcessID, InstID: 1, Content: content, Path: absPath,
				IsLatest: 1, IsReleased: 1, SHA256: byteSHA256([]byte(content)),
				Expression: pr.Expression, Name: fileName,
			})
		}
	}

	return configTpls, configVers, configBinds, configInsts
}

// listAllServiceInstances pages through all service instances for a biz.
func (g *MockGenerator) listAllServiceInstances(ctx context.Context, bizID uint32) (
	[]*CMDBServiceInstance, error) {

	url := fmt.Sprintf("%s/api/v3/findmany/proc/service_instance/details",
		g.cmdbClient.cfg.Endpoint)

	var allInstances []*CMDBServiceInstance
	start := 0

	for {
		reqBody := map[string]interface{}{
			"bk_biz_id": bizID,
			"page": map[string]interface{}{
				"start": start,
				"limit": maxBatchSize,
			},
		}

		var paged cmdbPagedResp[CMDBServiceInstance]
		if err := g.cmdbClient.doRequest(
			ctx, url, reqBody, &paged); err != nil {
			return nil, err
		}

		for i := range paged.Info {
			allInstances = append(allInstances, &paged.Info[i])
		}

		if len(paged.Info) < maxBatchSize {
			break
		}
		start += maxBatchSize
	}

	return allInstances, nil
}

// writeSQL writes the generated SQL to the output file.
// nolint:funlen
func (g *MockGenerator) writeSQL(
	outputPath string,
	procRows []processRow,
	instRows []processInstRow,
	configTpls []configTplRow,
	configVers []configVerRow,
	configBinds []configBindRow,
	configInsts []configInstRow,
) error {
	var sb strings.Builder

	now := time.Now().Format("2006-01-02 15:04:05.000000")

	sb.WriteString("-- =============================================================================\n")
	sb.WriteString("-- GSEKit Mock Data for Migration Testing (generated from CMDB)\n")
	sb.WriteString(fmt.Sprintf("-- Business ID: %d\n", g.bizID))
	sb.WriteString(fmt.Sprintf("-- Generated at: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString("-- =============================================================================\n\n")

	sb.WriteString("SET NAMES utf8mb4;\n\n")
	sb.WriteString("CREATE DATABASE IF NOT EXISTS `bk_gsekit` DEFAULT CHARACTER SET utf8mb3;\n")
	sb.WriteString("USE `bk_gsekit`;\n\n")

	// --- gsekit_process ---
	sb.WriteString("-- =============================================================================\n")
	sb.WriteString(fmt.Sprintf("-- Table 1: gsekit_process (%d records)\n", len(procRows)))
	sb.WriteString("-- =============================================================================\n\n")

	sb.WriteString("DROP TABLE IF EXISTS `gsekit_process`;\n")
	sb.WriteString(`CREATE TABLE ` + "`gsekit_process`" + ` (
  ` + "`bk_biz_id`" + ` int NOT NULL,
  ` + "`expression`" + ` varchar(256) NOT NULL,
  ` + "`bk_host_innerip`" + ` char(39) DEFAULT NULL,
  ` + "`bk_cloud_id`" + ` int NOT NULL,
  ` + "`bk_set_env`" + ` varchar(4) NOT NULL,
  ` + "`bk_set_id`" + ` int NOT NULL,
  ` + "`bk_module_id`" + ` int NOT NULL,
  ` + "`service_template_id`" + ` int DEFAULT NULL,
  ` + "`service_instance_id`" + ` int NOT NULL,
  ` + "`bk_process_name`" + ` varchar(64) DEFAULT NULL,
  ` + "`bk_process_id`" + ` int NOT NULL,
  ` + "`process_template_id`" + ` int NOT NULL,
  ` + "`process_status`" + ` int NOT NULL,
  ` + "`is_auto`" + ` tinyint(1) NOT NULL,
  ` + "`bk_agent_id`" + ` varchar(64) DEFAULT NULL,
  ` + "`bk_host_innerip_v6`" + ` char(39) DEFAULT NULL,
  ` + "`bk_host_id`" + ` int NOT NULL,
  PRIMARY KEY (` + "`bk_process_id`" + `)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
`)

	sb.WriteString("\n")
	if len(procRows) > 0 {
		sb.WriteString("INSERT INTO `gsekit_process` VALUES\n")
		for i, pr := range procRows {
			agentID := "NULL"
			if pr.BkAgentID != "" {
				agentID = sqlQuote(pr.BkAgentID)
			}
			sb.WriteString(fmt.Sprintf("(%d, %s, %s, %d, %s, %d, %d, %d, %d, %s, %d, %d, %d, %d, %s, NULL, %d)",
				pr.BkBizID, sqlQuote(pr.Expression), sqlQuote(pr.BkHostInnerip), pr.BkCloudID,
				sqlQuote(pr.BkSetEnv), pr.BkSetID, pr.BkModuleID, pr.ServiceTemplateID,
				pr.ServiceInstanceID, sqlQuote(pr.BkProcessName), pr.BkProcessID,
				pr.ProcessTemplateID, pr.ProcessStatus, pr.IsAuto, agentID, pr.BkHostID))
			if i < len(procRows)-1 {
				sb.WriteString(",\n")
			} else {
				sb.WriteString(";\n")
			}
		}
	}

	// --- gsekit_processinst ---
	sb.WriteString("\n-- =============================================================================\n")
	sb.WriteString(fmt.Sprintf("-- Table 2: gsekit_processinst (%d records)\n", len(instRows)))
	sb.WriteString("-- =============================================================================\n\n")

	sb.WriteString("DROP TABLE IF EXISTS `gsekit_processinst`;\n")
	sb.WriteString(`CREATE TABLE ` + "`gsekit_processinst`" + ` (
  ` + "`id`" + ` bigint NOT NULL AUTO_INCREMENT,
  ` + "`bk_biz_id`" + ` int NOT NULL,
  ` + "`bk_host_num`" + ` int NOT NULL,
  ` + "`bk_host_innerip`" + ` char(39) DEFAULT NULL,
  ` + "`bk_cloud_id`" + ` int NOT NULL,
  ` + "`bk_process_id`" + ` int NOT NULL,
  ` + "`bk_module_id`" + ` int NOT NULL,
  ` + "`bk_process_name`" + ` varchar(64) NOT NULL,
  ` + "`inst_id`" + ` int NOT NULL,
  ` + "`process_status`" + ` int NOT NULL,
  ` + "`is_auto`" + ` tinyint(1) NOT NULL,
  ` + "`local_inst_id`" + ` int NOT NULL,
  ` + "`local_inst_id_uniq_key`" + ` varchar(256) NOT NULL,
  ` + "`proc_num`" + ` int NOT NULL,
  ` + "`bk_agent_id`" + ` varchar(64) DEFAULT NULL,
  ` + "`bk_host_innerip_v6`" + ` char(39) DEFAULT NULL,
  ` + "`bk_host_id`" + ` int NOT NULL,
  PRIMARY KEY (` + "`id`" + `)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
`)

	sb.WriteString("\n")
	if len(instRows) > 0 {
		sb.WriteString("INSERT INTO `gsekit_processinst` VALUES\n")
		for i, ir := range instRows {
			agentID := "NULL"
			if ir.BkAgentID != "" {
				agentID = sqlQuote(ir.BkAgentID)
			}
			sb.WriteString(fmt.Sprintf("(%d, %d, %d, %s, %d, %d, %d, %s, %d, %d, %d, %d, %s, %d, %s, NULL, %d)",
				ir.ID, ir.BkBizID, ir.BkHostNum, sqlQuote(ir.BkHostInnerip), ir.BkCloudID,
				ir.BkProcessID, ir.BkModuleID, sqlQuote(ir.BkProcessName),
				ir.InstID, ir.ProcessStatus, ir.IsAuto, ir.LocalInstID,
				sqlQuote(ir.LocalInstIDUniqKey), ir.ProcNum, agentID, ir.BkHostID))
			if i < len(instRows)-1 {
				sb.WriteString(",\n")
			} else {
				sb.WriteString(";\n")
			}
		}
	}

	// --- gsekit_configtemplate ---
	sb.WriteString("\n-- =============================================================================\n")
	sb.WriteString(fmt.Sprintf("-- Table 3: gsekit_configtemplate (%d records)\n", len(configTpls)))
	sb.WriteString("-- =============================================================================\n\n")

	sb.WriteString("DROP TABLE IF EXISTS `gsekit_configtemplate`;\n")
	sb.WriteString(`CREATE TABLE ` + "`gsekit_configtemplate`" + ` (
  ` + "`created_at`" + ` datetime(6) NOT NULL,
  ` + "`created_by`" + ` varchar(32) NOT NULL,
  ` + "`updated_at`" + ` datetime(6) DEFAULT NULL,
  ` + "`updated_by`" + ` varchar(32) NOT NULL,
  ` + "`config_template_id`" + ` int NOT NULL AUTO_INCREMENT,
  ` + "`bk_biz_id`" + ` int NOT NULL,
  ` + "`template_name`" + ` varchar(32) NOT NULL,
  ` + "`file_name`" + ` varchar(64) NOT NULL,
  ` + "`abs_path`" + ` varchar(256) NOT NULL,
  ` + "`owner`" + ` varchar(32) NOT NULL,
  ` + "`group`" + ` varchar(32) NOT NULL,
  ` + "`filemode`" + ` varchar(8) NOT NULL,
  ` + "`line_separator`" + ` varchar(8) NOT NULL,
  PRIMARY KEY (` + "`config_template_id`" + `)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
`)

	sb.WriteString("\n")
	if len(configTpls) > 0 {
		sb.WriteString("INSERT INTO `gsekit_configtemplate` VALUES\n")
		for i, ct := range configTpls {
			sb.WriteString(fmt.Sprintf("(%s, 'admin', %s, 'admin', %d, %d, %s, %s, %s, %s, %s, %s, 'LF')",
				sqlQuote(now), sqlQuote(now), ct.ID, ct.BkBizID,
				sqlQuote(ct.TemplateName), sqlQuote(ct.FileName), sqlQuote(ct.AbsPath),
				sqlQuote(ct.Owner), sqlQuote(ct.Group), sqlQuote(ct.Filemode)))
			if i < len(configTpls)-1 {
				sb.WriteString(",\n")
			} else {
				sb.WriteString(";\n")
			}
		}
	}

	// --- gsekit_configtemplateversion ---
	sb.WriteString("\n-- =============================================================================\n")
	sb.WriteString(fmt.Sprintf("-- Table 4: gsekit_configtemplateversion (%d records)\n", len(configVers)))
	sb.WriteString("-- =============================================================================\n\n")

	sb.WriteString("DROP TABLE IF EXISTS `gsekit_configtemplateversion`;\n")
	sb.WriteString(`CREATE TABLE ` + "`gsekit_configtemplateversion`" + ` (
  ` + "`created_at`" + ` datetime(6) NOT NULL,
  ` + "`created_by`" + ` varchar(32) NOT NULL,
  ` + "`updated_at`" + ` datetime(6) DEFAULT NULL,
  ` + "`updated_by`" + ` varchar(32) NOT NULL,
  ` + "`config_version_id`" + ` int NOT NULL AUTO_INCREMENT,
  ` + "`config_template_id`" + ` int NOT NULL,
  ` + "`description`" + ` varchar(256) NOT NULL,
  ` + "`content`" + ` longtext NOT NULL,
  ` + "`is_draft`" + ` tinyint(1) NOT NULL,
  ` + "`is_active`" + ` tinyint(1) NOT NULL,
  ` + "`file_format`" + ` varchar(16) DEFAULT NULL,
  PRIMARY KEY (` + "`config_version_id`" + `)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
`)

	sb.WriteString("\n")
	for _, cv := range configVers {
		sb.WriteString("INSERT INTO `gsekit_configtemplateversion` VALUES\n")
		sb.WriteString(fmt.Sprintf("(%s, 'admin', %s, 'admin',\n %d, %d, %s,\n %s,\n %d, %d, %s);\n\n",
			sqlQuote(now), sqlQuote(now), cv.ID, cv.ConfigTemplateID,
			sqlQuote(cv.Description), sqlQuote(cv.Content),
			cv.IsDraft, cv.IsActive, sqlQuote(cv.FileFormat)))
	}

	// --- gsekit_configtemplatebindingrelationship ---
	sb.WriteString("-- =============================================================================\n")
	sb.WriteString(fmt.Sprintf("-- Table 5: gsekit_configtemplatebindingrelationship (%d records)\n", len(configBinds)))
	sb.WriteString("-- =============================================================================\n\n")

	sb.WriteString("DROP TABLE IF EXISTS `gsekit_configtemplatebindingrelationship`;\n")
	sb.WriteString(`CREATE TABLE ` + "`gsekit_configtemplatebindingrelationship`" + ` (
  ` + "`id`" + ` bigint NOT NULL AUTO_INCREMENT,
  ` + "`created_at`" + ` datetime(6) NOT NULL,
  ` + "`created_by`" + ` varchar(32) NOT NULL,
  ` + "`updated_at`" + ` datetime(6) DEFAULT NULL,
  ` + "`updated_by`" + ` varchar(32) NOT NULL,
  ` + "`bk_biz_id`" + ` int NOT NULL,
  ` + "`config_template_id`" + ` int NOT NULL,
  ` + "`process_object_type`" + ` varchar(16) NOT NULL,
  ` + "`process_object_id`" + ` int NOT NULL,
  PRIMARY KEY (` + "`id`" + `)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
`)

	sb.WriteString("\n")
	if len(configBinds) > 0 {
		sb.WriteString("INSERT INTO `gsekit_configtemplatebindingrelationship` VALUES\n")
		for i, cb := range configBinds {
			sb.WriteString(fmt.Sprintf("(%d, %s, 'admin', %s, 'admin', %d, %d, %s, %d)",
				cb.ID, sqlQuote(now), sqlQuote(now), cb.BkBizID, cb.ConfigTemplateID,
				sqlQuote(cb.ProcessObjectType), cb.ProcessObjectID))
			if i < len(configBinds)-1 {
				sb.WriteString(",\n")
			} else {
				sb.WriteString(";\n")
			}
		}
	}

	// --- gsekit_configinstance ---
	sb.WriteString("\n-- =============================================================================\n")
	sb.WriteString(fmt.Sprintf("-- Table 6: gsekit_configinstance (%d records)\n", len(configInsts)))
	sb.WriteString("-- content stored as plain text (decompressBz2 fallback handles this)\n")
	sb.WriteString("-- =============================================================================\n\n")

	sb.WriteString("DROP TABLE IF EXISTS `gsekit_configinstance`;\n")
	sb.WriteString(`CREATE TABLE ` + "`gsekit_configinstance`" + ` (
  ` + "`id`" + ` bigint NOT NULL AUTO_INCREMENT,
  ` + "`config_version_id`" + ` int NOT NULL,
  ` + "`config_template_id`" + ` int NOT NULL,
  ` + "`bk_process_id`" + ` int NOT NULL,
  ` + "`inst_id`" + ` int NOT NULL,
  ` + "`content`" + ` longblob NOT NULL,
  ` + "`path`" + ` varchar(256) NOT NULL,
  ` + "`is_latest`" + ` tinyint(1) NOT NULL,
  ` + "`is_released`" + ` tinyint(1) NOT NULL,
  ` + "`sha256`" + ` varchar(64) NOT NULL,
  ` + "`expression`" + ` varchar(256) NOT NULL,
  ` + "`created_at`" + ` datetime(6) NOT NULL,
  ` + "`created_by`" + ` varchar(32) NOT NULL,
  ` + "`name`" + ` varchar(64) NOT NULL,
  PRIMARY KEY (` + "`id`" + `)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
`)

	sb.WriteString("\n")
	for _, ci := range configInsts {
		sb.WriteString("INSERT INTO `gsekit_configinstance` VALUES\n")
		sb.WriteString(fmt.Sprintf("(%d, %d, %d, %d, %d,\n %s,\n %s, %d, %d,\n %s,\n %s, %s, 'admin', %s);\n\n",
			ci.ID, ci.ConfigVersionID, ci.ConfigTemplateID, ci.BkProcessID, ci.InstID,
			sqlQuote(ci.Content), sqlQuote(ci.Path), ci.IsLatest, ci.IsReleased,
			sqlQuote(ci.SHA256), sqlQuote(ci.Expression), sqlQuote(now), sqlQuote(ci.Name)))
	}

	// --- Verification queries ---
	sb.WriteString("-- =============================================================================\n")
	sb.WriteString("-- Verification queries\n")
	sb.WriteString("-- =============================================================================\n\n")
	sb.WriteString("SELECT 'gsekit_process' AS table_name, COUNT(*) AS row_count FROM gsekit_process\n")
	sb.WriteString("UNION ALL\n")
	sb.WriteString("SELECT 'gsekit_processinst', COUNT(*) FROM gsekit_processinst\n")
	sb.WriteString("UNION ALL\n")
	sb.WriteString("SELECT 'gsekit_configtemplate', COUNT(*) FROM gsekit_configtemplate\n")
	sb.WriteString("UNION ALL\n")
	sb.WriteString("SELECT 'gsekit_configtemplateversion', COUNT(*) FROM gsekit_configtemplateversion\n")
	sb.WriteString("UNION ALL\n")
	sb.WriteString("SELECT 'gsekit_configtemplatebindingrelationship', COUNT(*) FROM gsekit_configtemplatebindingrelationship\n")
	sb.WriteString("UNION ALL\n")
	sb.WriteString("SELECT 'gsekit_configinstance', COUNT(*) FROM gsekit_configinstance;\n")

	return os.WriteFile(outputPath, []byte(sb.String()), 0644)
}

// sqlQuote returns an SQL-safe single-quoted string.
func sqlQuote(s string) string {
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `''`)
	return "'" + escaped + "'"
}

// mapKeys extracts keys from a map[int64]bool.
func mapKeys(m map[int64]bool) []int64 {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
