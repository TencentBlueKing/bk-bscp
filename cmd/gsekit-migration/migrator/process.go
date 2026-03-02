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
	"time"
)

// GSEKitProcess represents a row from gsekit_process table
type GSEKitProcess struct {
	BkProcessID       int64  `gorm:"column:bk_process_id;primaryKey"`
	BkBizID           int64  `gorm:"column:bk_biz_id"`
	Expression        string `gorm:"column:expression"`
	BkHostInnerip     string `gorm:"column:bk_host_innerip"`
	BkHostInneripV6   string `gorm:"column:bk_host_innerip_v6"`
	BkAgentID         string `gorm:"column:bk_agent_id"`
	BkCloudID         int64  `gorm:"column:bk_cloud_id"`
	BkSetEnv          string `gorm:"column:bk_set_env"`
	BkSetID           int64  `gorm:"column:bk_set_id"`
	BkModuleID        int64  `gorm:"column:bk_module_id"`
	ServiceTemplateID *int64 `gorm:"column:service_template_id"`
	ServiceInstanceID int64  `gorm:"column:service_instance_id"`
	BkProcessName     string `gorm:"column:bk_process_name"`
	ProcessTemplateID int64  `gorm:"column:process_template_id"`
	ProcessStatus     int    `gorm:"column:process_status"`
	IsAuto            bool   `gorm:"column:is_auto"`
}

// TableName returns the GSEKit process table name
func (GSEKitProcess) TableName() string { return "gsekit_process" }

// GSEKitProcessInst represents a row from gsekit_processinst table
type GSEKitProcessInst struct {
	ID                 int64  `gorm:"column:id;primaryKey"`
	BkBizID            int64  `gorm:"column:bk_biz_id"`
	BkHostNum          int    `gorm:"column:bk_host_num"`
	BkHostInnerip      string `gorm:"column:bk_host_innerip"`
	BkHostInneripV6    string `gorm:"column:bk_host_innerip_v6"`
	BkAgentID          string `gorm:"column:bk_agent_id"`
	BkCloudID          int64  `gorm:"column:bk_cloud_id"`
	BkProcessID        int64  `gorm:"column:bk_process_id"`
	BkModuleID         int64  `gorm:"column:bk_module_id"`
	BkProcessName      string `gorm:"column:bk_process_name"`
	InstID             int    `gorm:"column:inst_id"`
	ProcessStatus      int    `gorm:"column:process_status"`
	IsAuto             bool   `gorm:"column:is_auto"`
	LocalInstID        int    `gorm:"column:local_inst_id"`
	LocalInstIDUniqKey string `gorm:"column:local_inst_id_uniq_key"`
	ProcNum            int    `gorm:"column:proc_num"`
}

// TableName returns the GSEKit process inst table name
func (GSEKitProcessInst) TableName() string { return "gsekit_processinst" }

// migrateProcesses migrates process data from GSEKit to BSCP
// nolint:funlen
func (m *Migrator) migrateProcesses() error {
	log.Println("=== Step 2: Migrating processes ===")

	ctx := context.Background()
	batchSize := m.cfg.Migration.BatchSize
	creator := m.cfg.Migration.Creator
	reviser := m.cfg.Migration.Reviser
	totalMigrated := 0

	for _, bizID := range m.cfg.Migration.BizIDs {
		log.Printf("  Processing processes for biz %d", bizID)

		// Count source records
		var sourceCount int64
		if err := m.sourceDB.Model(&GSEKitProcess{}).Where("bk_biz_id = ?", bizID).Count(&sourceCount).Error; err != nil {
			return fmt.Errorf("count gsekit_process for biz %d failed: %w", bizID, err)
		}
		log.Printf("  Found %d processes in GSEKit for biz %d", sourceCount, bizID)

		if sourceCount == 0 {
			continue
		}

		// Batch read and migrate
		offset := 0
		for {
			var processes []GSEKitProcess
			if err := m.sourceDB.Where("bk_biz_id = ?", bizID).
				Offset(offset).Limit(batchSize).
				Find(&processes).Error; err != nil {
				return fmt.Errorf("read gsekit_process batch for biz %d offset %d failed: %w", bizID, offset, err)
			}
			if len(processes) == 0 {
				break
			}

			// Collect IDs for CMDB enrichment
			processIDs := make([]int64, 0, len(processes))
			setIDs := make([]int64, 0, len(processes))
			moduleIDs := make([]int64, 0, len(processes))
			svcInstIDs := make([]int64, 0, len(processes))
			for _, p := range processes {
				processIDs = append(processIDs, p.BkProcessID)
				setIDs = append(setIDs, p.BkSetID)
				moduleIDs = append(moduleIDs, p.BkModuleID)
				svcInstIDs = append(svcInstIDs, p.ServiceInstanceID)
			}

			svcInstDetails, err := m.cmdbClient.ListServiceInstanceDetail(ctx, bizID, uniqueInt64(svcInstIDs))
			if err != nil {
				return fmt.Errorf("list service instance detail for biz %d failed: %w", bizID, err)
			}

			moduleNames, err := m.cmdbClient.FindModuleBatch(ctx, bizID, uniqueInt64(moduleIDs))
			if err != nil {
				return fmt.Errorf("find module batch for biz %d failed: %w", bizID, err)
			}

			setNames, err := m.cmdbClient.FindSetBatch(ctx, bizID, uniqueInt64(setIDs))
			if err != nil {
				return fmt.Errorf("find set batch for biz %d failed: %w", bizID, err)
			}

			// Get process details for source_data/prev_data
			processDetails, err := m.cmdbClient.ListProcessDetailByIds(ctx, bizID, uniqueInt64(processIDs))
			if err != nil {
				return fmt.Errorf("list process detail by ids for biz %d failed: %w", bizID, err)
			}

			// Build processID → (host_id, func_name) lookup from service instance details
			type processEnrich struct {
				HostID   int
				FuncName string
			}
			processEnrichMap := make(map[int64]*processEnrich)
			svcInstNameMap := make(map[int64]string)
			for svcInstID, svcInst := range svcInstDetails {
				svcInstNameMap[svcInstID] = svcInst.Name
				for _, pi := range svcInst.ProcessInstances {
					if pi.Property == nil {
						continue
					}
					hostID := 0
					if pi.Relation != nil {
						hostID = pi.Relation.BkHostID
					}
					processEnrichMap[int64(pi.Property.BkProcessID)] = &processEnrich{
						HostID:   hostID,
						FuncName: pi.Property.BkFuncName,
					}
				}
			}

			// Count instances per process for proc_num
			procInstCounts := make(map[int64]int)
			var instCounts []struct {
				BkProcessID int64 `gorm:"column:bk_process_id"`
				Count       int   `gorm:"column:count"`
			}
			m.sourceDB.Raw(
				"SELECT bk_process_id, COUNT(*) as count FROM gsekit_processinst WHERE bk_process_id IN ? GROUP BY bk_process_id",
				processIDs).Scan(&instCounts)
			for _, ic := range instCounts {
				procInstCounts[ic.BkProcessID] = ic.Count
			}

			// Allocate IDs
			ids, err := m.idGen.BatchNextID("processes", len(processes))
			if err != nil {
				return fmt.Errorf("allocate process IDs failed: %w", err)
			}

			now := time.Now()
			for i, p := range processes {
				newID := ids[i]
				m.processIDMap[uint32(p.BkProcessID)] = newID

				// Get CMDB enrichment data
				var hostID uint32
				var funcName string
				if enrich, ok := processEnrichMap[p.BkProcessID]; ok {
					hostID = uint32(enrich.HostID)
					funcName = enrich.FuncName
				}
				setName := setNames[p.BkSetID]
				moduleName := moduleNames[p.BkModuleID]
				serviceName := svcInstNameMap[p.ServiceInstanceID]
				procNum := procInstCounts[p.BkProcessID]
				svcTemplateID := uint32(0)
				if p.ServiceTemplateID != nil {
					svcTemplateID = uint32(*p.ServiceTemplateID)
				}

				// Build source_data from CMDB process details
				sourceData := "{}"
				if detail, ok := processDetails[p.BkProcessID]; ok {
					pi := map[string]interface{}{
						"bk_start_param_regex": detail.BkStartParamRegex,
						"work_path":            detail.WorkPath,
						"pid_file":             detail.PidFile,
						"user":                 detail.User,
						"reload_cmd":           detail.ReloadCmd,
						"restart_cmd":          detail.RestartCmd,
						"start_cmd":            detail.StartCmd,
						"stop_cmd":             detail.StopCmd,
						"face_stop_cmd":        detail.FaceStopCmd,
						"timeout":              detail.Timeout,
						"start_check_secs":     detail.BkStartCheckSecs,
					}
					if b, err := json.Marshal(pi); err == nil {
						sourceData = string(b)
					}
				}

				if err := m.targetDB.Exec(
					"INSERT INTO processes (id, tenant_id, biz_id, cc_process_id, set_id, module_id, "+
						"service_instance_id, host_id, cloud_id, agent_id, process_template_id, service_template_id, "+
						"set_name, module_name, service_name, environment, alias, inner_ip, inner_ip_v6, "+
						"cc_sync_status, process_state_synced_at, proc_num, func_name, source_data, prev_data, "+
						"creator, reviser, created_at, updated_at) "+
						"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
					newID, m.cfg.Migration.TenantID, bizID, uint32(p.BkProcessID),
					uint32(p.BkSetID), uint32(p.BkModuleID),
					uint32(p.ServiceInstanceID), hostID, uint32(p.BkCloudID),
					p.BkAgentID, uint32(p.ProcessTemplateID), svcTemplateID,
					setName, moduleName, serviceName,
					p.BkSetEnv, p.BkProcessName, p.BkHostInnerip, p.BkHostInneripV6,
					"synced", now, procNum, funcName, sourceData, sourceData,
					creator, reviser, now, now,
				).Error; err != nil {
					if m.cfg.Migration.ContinueOnError {
						log.Printf("  Warning: insert process failed (bk_process_id=%d): %v", p.BkProcessID, err)
						continue
					}
					return fmt.Errorf("insert process failed (bk_process_id=%d): %w", p.BkProcessID, err)
				}
				totalMigrated++
			}

			offset += batchSize
			log.Printf("  Progress: %d processes migrated for biz %d", totalMigrated, bizID)
		}
	}

	log.Printf("  Total processes migrated: %d", totalMigrated)
	return nil
}

// migrateProcessInstances migrates process instance data from GSEKit to BSCP
func (m *Migrator) migrateProcessInstances() error {
	log.Println("=== Step 3: Migrating process instances ===")

	batchSize := m.cfg.Migration.BatchSize
	totalMigrated := 0
	creator := m.cfg.Migration.Creator
	reviser := m.cfg.Migration.Reviser
	for _, bizID := range m.cfg.Migration.BizIDs {
		log.Printf("  Processing process instances for biz %d", bizID)

		var sourceCount int64
		if err := m.sourceDB.Model(&GSEKitProcessInst{}).Where("bk_biz_id = ?", bizID).Count(&sourceCount).Error; err != nil {
			return fmt.Errorf("count gsekit_processinst for biz %d failed: %w", bizID, err)
		}
		log.Printf("  Found %d process instances in GSEKit for biz %d", sourceCount, bizID)

		if sourceCount == 0 {
			continue
		}

		offset := 0
		for {
			var instances []GSEKitProcessInst
			if err := m.sourceDB.Where("bk_biz_id = ?", bizID).
				Offset(offset).Limit(batchSize).
				Find(&instances).Error; err != nil {
				return fmt.Errorf("read gsekit_processinst batch for biz %d offset %d failed: %w", bizID, offset, err)
			}
			if len(instances) == 0 {
				break
			}

			ids, err := m.idGen.BatchNextID("process_instances", len(instances))
			if err != nil {
				return fmt.Errorf("allocate process_instance IDs failed: %w", err)
			}

			now := time.Now()
			for i, inst := range instances {
				newID := ids[i]

				// Look up the mapped process_id
				processID, ok := m.processIDMap[uint32(inst.BkProcessID)]
				if !ok {
					if m.cfg.Migration.ContinueOnError {
						log.Printf("  Warning: no process mapping for bk_process_id=%d, skipping instance", inst.BkProcessID)
						continue
					}
					return fmt.Errorf("no process mapping for bk_process_id=%d", inst.BkProcessID)
				}

				// Map process status: 0(UNREGISTERED)→"stopped", 1(RUNNING)→"running", 2(TERMINATED)→"stopped"
				status := "stopped"
				if inst.ProcessStatus == 1 {
					status = "running"
				}

				// Map managed status: false→"unmanaged", true→"managed"
				managedStatus := "unmanaged"
				if inst.IsAuto {
					managedStatus = "managed"
				}

				if err := m.targetDB.Exec(
					"INSERT INTO process_instances (id, tenant_id, biz_id, process_id, cc_process_id, "+
						"host_inst_seq, module_inst_seq, status, managed_status, status_updated_at, "+
						"creator, reviser, created_at, updated_at) "+
						"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
					newID, m.cfg.Migration.TenantID, bizID, processID, uint32(inst.BkProcessID),
					inst.LocalInstID, inst.InstID, status, managedStatus, now,
					creator, reviser, now, now,
				).Error; err != nil {
					if m.cfg.Migration.ContinueOnError {
						log.Printf("  Warning: insert process_instance failed (id=%d): %v", inst.ID, err)
						continue
					}
					return fmt.Errorf("insert process_instance failed (gsekit_id=%d): %w", inst.ID, err)
				}
				totalMigrated++
			}

			offset += batchSize
			log.Printf("  Progress: %d process instances migrated for biz %d", totalMigrated, bizID)
		}
	}

	log.Printf("  Total process instances migrated: %d", totalMigrated)
	return nil
}

// uniqueInt64 returns unique values from a slice
func uniqueInt64(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}
