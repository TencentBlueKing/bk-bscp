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

// Package cmdb provides cmdb service.
package cmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
)

// SyncCMDBService 同步cmdb
type syncCMDBService struct {
	bizID int
	svc   bkcmdb.Service
	dao   dao.Set
}

// NewSyncCMDBService 初始化同步cmdb
func NewSyncCMDBService(bizID int, svc bkcmdb.Service, dao dao.Set) *syncCMDBService {
	return &syncCMDBService{
		bizID: bizID,
		svc:   svc,
		dao:   dao,
	}
}

// SyncSingleBiz 对单个业务执行全量进程同步
//
// 同步内容：
//   - 集群(Set)
//   - 模块(Module)
//   - 主机(Host)
//   - 服务实例(ServiceInstance)
//   - 进程(Process) & 实例(ProcessInstance)
//
// 特点：
//   - 全量拉取 CC 数据
//   - 构建完整业务拓扑
//   - 通过 SyncProcessData 统一落库
//
// nolint: funlen
func (s *syncCMDBService) SyncSingleBiz(ctx context.Context) error {
	kt := kit.FromGrpcContext(ctx)
	logs.Infof("[SyncSingleBiz][Start] bizID=%d", s.bizID)

	// 1. 获取集群
	listSets, err := s.fetchAllSets(ctx)
	if err != nil {
		return fmt.Errorf("[SyncSingleBiz][FetchSet] bizID=%d failed: %v", s.bizID, err)
	}

	var sets []Set
	for _, set := range listSets {
		sets = append(sets, Set{ID: set.BkSetID, Name: set.BkSetName, SetEnv: set.BkSetEnv})
	}

	// 2. 模块
	for i := range listSets {
		listModules, errM := s.fetchAllModules(ctx, sets[i].ID)
		if errM != nil {
			return fmt.Errorf(
				"[SyncSingleBiz][FetchModule] bizID=%d setID=%d failed: %v",
				s.bizID, sets[i].ID, errM,
			)
		}
		for _, m := range listModules {
			module := Module{ID: m.BkModuleID, Name: m.BkModuleName, ServiceTemplateID: m.ServiceTemplateID}
			sets[i].Module = append(sets[i].Module, module)
		}
	}

	// 3. 主机
	setTemplateIDs := []int{}
	for _, set := range listSets {
		if !slices.Contains(setTemplateIDs, set.SetTemplateID) {
			setTemplateIDs = append(setTemplateIDs, set.SetTemplateID)
		}
	}

	listHosts, err := s.fetchAllHostsBySetTemplate(ctx, setTemplateIDs)
	if err != nil {
		return fmt.Errorf(
			"[SyncSingleBiz][FetchHost] bizID=%d failed: %v",
			s.bizID, err,
		)
	}
	var hosts []Host
	for _, h := range listHosts {
		hosts = append(hosts, Host{ID: h.BkHostID, IP: h.BkHostInnerIP,
			CloudId: h.BkCloudID, AgentID: h.BkAgentID})
	}

	// 4. 服务实例
	var moduleIDs []int
	for _, set := range sets {
		for _, m := range set.Module {
			moduleIDs = append(moduleIDs, m.ID)
		}
	}

	listSvcInsts, err := s.fetchAllServiceInstances(ctx, moduleIDs)
	if err != nil {
		return fmt.Errorf(
			"[SyncSingleBiz][FetchServiceInstance] bizID=%d failed: %v",
			s.bizID, err,
		)
	}

	moduleSvcMap := map[int][]SvcInst{}
	for _, inst := range listSvcInsts {
		moduleSvcMap[inst.BkModuleID] = append(moduleSvcMap[inst.BkModuleID], SvcInst{
			ID: inst.ID, Name: inst.Name,
		})
	}

	// 5. 进程
	procInstBySvcID := make(map[int][]ProcInst)
	for _, inst := range listSvcInsts {
		procs, errL := s.svc.ListProcessInstance(ctx, &bkcmdb.ListProcessInstanceReq{
			BkBizID: s.bizID, ServiceInstanceID: inst.ID,
		})
		if errL != nil {
			return fmt.Errorf(
				"[SyncSingleBiz][FetchProcessInstance] bizID=%d serviceInstanceID=%d failed: %v",
				s.bizID, inst.ID, errL,
			)
		}

		for _, proc := range procs {
			procInstBySvcID[inst.ID] = append(procInstBySvcID[inst.ID], ProcInst{
				ID:                proc.Property.BkProcessID,
				HostID:            proc.Relation.BkHostID,
				ProcessTemplateID: proc.Relation.ProcessTemplateID,
				Name:              proc.Property.BkProcessName,
				FuncName:          proc.Property.BkFuncName,
				ProcNum:           proc.Property.ProcNum,
				ProcessInfo: table.ProcessInfo{
					BkStartParamRegex: proc.Property.BkStartParamRegex,
					WorkPath:          proc.Property.WorkPath,
					PidFile:           proc.Property.PidFile,
					User:              proc.Property.User,
					ReloadCmd:         proc.Property.ReloadCmd,
					RestartCmd:        proc.Property.RestartCmd,
					StartCmd:          proc.Property.StartCmd,
					StopCmd:           proc.Property.StopCmd,
					FaceStopCmd:       proc.Property.FaceStopCmd,
					Timeout:           proc.Property.Timeout,
					StartCheckSecs:    proc.Property.BkStartCheckSecs,
				},
			})
		}
	}

	// 6. 拼装
	for si := range sets {
		for mi := range sets[si].Module {
			svcList := moduleSvcMap[sets[si].Module[mi].ID]
			for sj := range svcList {
				svcList[sj].ProcInst = procInstBySvcID[svcList[sj].ID]
			}
			sets[si].Module[mi].SvcInst = svcList
			sets[si].Module[mi].Host = hosts
		}
	}

	// 构建并立即入库
	newProcesses := buildProcessesFromSets(s.bizID, sets)

	tx := s.dao.GenQuery().Begin()

	oldProcesses, err := s.dao.Process().ListProcByBizIDWithTx(kt, tx, uint32(s.bizID))
	if err != nil {
		return fmt.Errorf(
			"[SyncSingleBiz][ListOldProcess] bizID=%d failed: %v",
			s.bizID, err,
		)
	}

	_, err = SyncProcessData(
		kt,
		s.dao,
		tx,
		uint32(s.bizID),
		oldProcesses,
		newProcesses,
	)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			logs.Errorf(
				"[SyncSingleBiz][Rollback] bizID=%d failed: %v",
				s.bizID, rbErr,
			)
		}
		return fmt.Errorf(
			"[SyncSingleBiz][SyncFailed] bizID=%d: %v",
			s.bizID, err,
		)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf(
			"[SyncSingleBiz][CommitFailed] bizID=%d: %v",
			s.bizID, err,
		)
	}

	logs.Infof(
		"[SyncSingleBiz][Success] bizID=%d processCount=%d",
		s.bizID, len(newProcesses),
	)

	return nil
}

// SyncByProcessIDs 按进程ID增量同步 CMDB 进程数据
//
// 使用场景：
//   - 监听到「进程新增」事件
//   - 已明确知道受影响的 process_id 列表
//
// 设计说明：
//   - 这是一个【增量同步】入口，不做全业务扫描
//   - 仅围绕给定的 process_id 反查：
//     进程 -> 服务实例 -> 模块 -> 集群(set)
//   - 最终仍然复用统一的 SyncProcessData 做落库
//
// 行为语义：
//   - 只会构建“包含变更进程的最小业务拓扑”
//   - 不负责清理无关进程（由全量同步或其他任务处理）
//
// 返回值：
//   - SyncProcessResult：描述本次同步中新增 / 更新 / 删除的统计信息
//
// nolint: funlen
func (s *syncCMDBService) SyncByProcessIDs(ctx context.Context, processes []bkcmdb.ProcessInfo) (*SyncProcessResult, error) {
	if len(processes) == 0 {
		return &SyncProcessResult{}, nil
	}

	// 1. 按 ServiceInstance 归并进程
	svcProcMap := make(map[int][]ProcInst, len(processes))
	svcInstIDs := make([]int64, 0, len(processes))

	for _, p := range processes {
		svcInstIDs = append(svcInstIDs, int64(p.ServiceInstanceID))
		svcProcMap[p.ServiceInstanceID] = append(
			svcProcMap[p.ServiceInstanceID],
			ProcInst{
				ID:       p.BkProcessID,
				Name:     p.BkProcessName,
				FuncName: p.BkFuncName,
				ProcNum:  p.ProcNum,
				ProcessInfo: table.ProcessInfo{
					BkStartParamRegex: p.BkStartParamRegex,
					WorkPath:          p.WorkPath,
					PidFile:           p.PidFile,
					User:              p.User,
					ReloadCmd:         p.ReloadCmd,
					RestartCmd:        p.RestartCmd,
					StartCmd:          p.StartCmd,
					StopCmd:           p.StopCmd,
					FaceStopCmd:       p.FaceStopCmd,
					Timeout:           p.Timeout,
					StartCheckSecs:    p.BkStartCheckSecs,
				},
			},
		)
	}

	// 去重
	svcInstIDs = tools.UniqInt64s(svcInstIDs)
	// 2. 拉取服务实例
	allSvcInsts := make([]bkcmdb.ServiceInstanceInfo, 0, len(svcInstIDs))
	for _, chunk := range chunkInts(svcInstIDs, 1000) {
		resp, err := s.svc.ListServiceInstanceDetail(ctx, &bkcmdb.ListServiceInstanceReq{
			BizID:              int64(s.bizID),
			ServiceInstanceIDs: chunk,
			Page:               bkcmdb.PageParam{Start: 0, Limit: 1000},
		})
		if err != nil {
			return nil, err
		}
		allSvcInsts = append(allSvcInsts, resp.Info...)
	}

	if len(allSvcInsts) == 0 {
		return &SyncProcessResult{}, nil
	}

	// 3. 构建 Module → ServiceInstance
	moduleSvcMap := make(map[int][]SvcInst)
	moduleIDSet := make(map[int]struct{})
	svcTemplateIDs := make([]int64, 0)

	for _, svc := range allSvcInsts {
		procs, ok := svcProcMap[svc.ID]
		if !ok {
			continue
		}

		for i := range procs {
			procs[i].HostID = svc.BkHostID
			procs[i].ProcessTemplateID = svc.ServiceTemplateID
		}

		moduleSvcMap[svc.BkModuleID] = append(
			moduleSvcMap[svc.BkModuleID],
			SvcInst{
				ID:       svc.ID,
				Name:     svc.Name,
				ProcInst: procs,
			},
		)

		moduleIDSet[svc.BkModuleID] = struct{}{}
		svcTemplateIDs = append(svcTemplateIDs, int64(svc.ServiceTemplateID))
	}

	// 4. 拉取 Host
	svcTemplateIDs = tools.UniqInt64s(svcTemplateIDs)

	hosts := make([]Host, 0)
	for _, chunk := range chunkInts(svcTemplateIDs, 500) {
		resp, err := s.svc.FindHostByServiceTemplate(ctx,
			&bkcmdb.ListHostByServiceTemplateReq{
				BizID:              int64(s.bizID),
				ServiceTemplateIDs: chunk,
				Fields: []string{
					"bk_host_id",
					"bk_host_innerip",
					"bk_cloud_id",
					"bk_agent_id",
				},
				Page: bkcmdb.PageParam{Start: 0, Limit: 500},
			},
		)
		if err != nil {
			return nil, err
		}

		for _, h := range resp.Info {
			hosts = append(hosts, Host{
				ID:      h.BkHostID,
				IP:      h.BkHostInnerIP,
				CloudId: h.BkCloudID,
				AgentID: h.BkAgentID,
			})
		}
	}

	// 5. Module → Host 映射
	moduleHostMap := buildModuleHosts(allSvcInsts, hosts)

	// 6. 构建 Set → Module
	listModules, err := s.fetchAllModules(ctx, 0)
	if err != nil {
		return nil, err
	}

	setModules := make(map[int][]Module)
	setIDs := make([]int, 0)

	for _, m := range listModules {
		if _, ok := moduleIDSet[m.BkModuleID]; !ok {
			continue
		}

		setModules[m.BkSetID] = append(setModules[m.BkSetID], Module{
			ID:                m.BkModuleID,
			Name:              m.BkModuleName,
			ServiceTemplateID: m.ServiceTemplateID,
			SvcInst:           moduleSvcMap[m.BkModuleID],
			Host:              moduleHostMap[m.BkModuleID],
		})
		setIDs = append(setIDs, m.BkSetID)
	}

	setIDs = tools.UniqInts(setIDs)

	// 7. 拉取 Set
	setsResp, err := s.svc.SearchSet(ctx, bkcmdb.SearchSetReq{
		BkSupplierAccount: "0",
		BkBizID:           s.bizID,
		Fields: []string{
			"bk_set_id",
			"bk_set_name",
			"bk_set_env",
		},
		Filter: &bkcmdb.Filter{
			Condition: "AND",
			Rules: []bkcmdb.Rule{
				{Field: "bk_set_id", Operator: "in", Value: setIDs},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	sets := make([]Set, 0, len(setsResp.Info))
	for _, s := range setsResp.Info {
		sets = append(sets, Set{
			ID:     s.BkSetID,
			Name:   s.BkSetName,
			SetEnv: s.BkSetEnv,
			Module: setModules[s.BkSetID],
		})
	}

	processBatch := buildProcessesFromSets(s.bizID, sets)

	tx := s.dao.GenQuery().Begin()
	res, err := SyncProcessData(kit.New(), s.dao, tx, uint32(s.bizID), nil, processBatch)
	if err != nil {
		if rErr := tx.Rollback(); rErr != nil {
			logs.Errorf("transaction rollback failed, err: %v", rErr)
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	logs.Infof("[syncCMDB] bizID=%d synced, %d processes", s.bizID, len(processBatch))

	return res, nil

}

// UpdateProcess 处理进程更新事件，同步 CMDB 中的进程及实例数据
//
// 使用场景：
//   - 监听到 CC 的进程变更事件（属性变更 / 实例数变更等）
//   - 将事件数据转换为内部 Process 模型
//   - 统一走 SyncProcessData 做差异计算与落库
func (s *syncCMDBService) UpdateProcess(ctx context.Context, processes []bkcmdb.ProcessInfo) (*SyncProcessResult, error) {
	if len(processes) == 0 {
		return &SyncProcessResult{}, nil
	}

	kt := kit.New()

	// 1. 构建 newProcesses（来自事件）
	newProcesses := make([]*table.Process, 0, len(processes))
	oldProcesses := make([]*table.Process, 0)
	for _, p := range processes {
		// 查询 DB 中已有进程（作为更新基准）
		oldP, err := s.dao.Process().GetProcByBizScvProc(kt, uint32(p.BkBizID), uint32(p.ServiceInstanceID), uint32(p.BkProcessID))
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			logs.Errorf(
				"[UpdateProcess][QueryOldProcess] bizID=%d serviceInstanceID=%d ccProcessID=%d failed: %v",
				p.BkBizID, p.ServiceInstanceID, p.BkProcessID, err,
			)
			continue
		}

		// 更新事件，但 DB 中不存在 → 异常数据，直接跳过
		if oldP == nil {
			logs.Warnf(
				"[UpdateProcess][ProcessNotFound] bizID=%d serviceInstanceID=%d ccProcessID=%d skip",
				p.BkBizID, p.ServiceInstanceID, p.BkProcessID,
			)
			continue
		}

		oldProcesses = append(oldProcesses, oldP)

		// 2. 构建新的 Spec（基于旧值更新）
		info := table.ProcessInfo{
			BkStartParamRegex: p.BkStartParamRegex,
			WorkPath:          p.WorkPath,
			PidFile:           p.PidFile,
			User:              p.User,
			ReloadCmd:         p.ReloadCmd,
			RestartCmd:        p.RestartCmd,
			StartCmd:          p.StartCmd,
			StopCmd:           p.StopCmd,
			FaceStopCmd:       p.FaceStopCmd,
			Timeout:           p.Timeout,
			StartCheckSecs:    p.BkStartCheckSecs,
		}

		sourceData, err := json.Marshal(info)
		if err != nil {
			logs.Errorf(
				"[UpdateProcess][MarshalSpec] bizID=%d ccProcessID=%d failed: %v, data=%+v",
				p.BkBizID, p.BkProcessID, err, info,
			)
			continue
		}

		now := time.Now().UTC()

		newSpec := *oldP.Spec
		newSpec.Alias = p.BkProcessName
		newSpec.ProcNum = uint(p.ProcNum)
		newSpec.SourceData = string(sourceData)

		newProcess := &table.Process{
			Attachment: oldP.Attachment,
			Spec:       &newSpec,
			Revision: &table.Revision{
				UpdatedAt: now,
			},
		}

		newProcess.Attachment.CcProcessID = uint32(p.BkProcessID)
		newProcess.Attachment.ServiceInstanceID = uint32(p.ServiceInstanceID)

		newProcesses = append(newProcesses, newProcess)
	}

	if len(newProcesses) == 0 {
		return &SyncProcessResult{}, nil
	}

	// 开启事务并入库
	tx := s.dao.GenQuery().Begin()

	res, err := SyncProcessData(kit.New(), s.dao, tx, uint32(s.bizID), oldProcesses, newProcesses)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			logs.Errorf("[UpdateProcess][ERROR] rollback failed for bizID=%d: %v", s.bizID, rbErr)
		}
		return nil, fmt.Errorf(
			"[UpdateProcess][SyncFailed] bizID=%d: %v",
			s.bizID, err,
		)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf(
			"[UpdateProcess][CommitFailed] bizID=%d: %v",
			s.bizID, err,
		)
	}

	logs.Infof("[UpdateProcess][Success] bizID=%d process data synced, %d processes written", s.bizID, len(newProcesses))

	return res, nil

}

// buildProcessesFromSets 根据业务拓扑信息构建进程表数据
//
// 该函数是一个“纯 Builder”，只负责将内存中的业务拓扑结构：
//
//	Set -> Module -> ServiceInstance -> ProcessInstance -> Host
//
// 转换为可直接写入数据库的 []*table.Process 模型。
//
// 设计约束：
//  1. 不访问数据库
//  2. 不涉及事务控制
//  3. 单条数据异常不会中断整体构建流程
//
// 异常处理策略：
//   - 进程关联的 Host 不存在：记录 WARN，跳过该进程
//   - 进程 SourceData 序列化失败：记录 ERROR，跳过该进程
//
// 返回值：
//   - 返回构建完成的进程列表
func buildProcessesFromSets(bizID int, sets []Set) []*table.Process {
	now := time.Now()

	processBatch := make([]*table.Process, 0)

	for _, set := range sets {
		for _, mod := range set.Module {

			// 构建 HostID -> HostInfo 映射
			hostMap := make(map[int]Host, len(mod.Host))
			for _, h := range mod.Host {
				hostMap[h.ID] = h
			}

			for _, svc := range mod.SvcInst {
				for _, proc := range svc.ProcInst {

					h, ok := hostMap[proc.HostID]
					if !ok {
						logs.Warnf(
							"[syncCMDB][WARN] bizID=%d, set=%s, module=%s, svc=%s, proc=%s, hostID=%d not found",
							bizID, set.Name, mod.Name, svc.Name, proc.Name, proc.HostID,
						)
						continue
					}

					sourceData, err := proc.ProcessInfo.Value()
					if err != nil {
						logs.Errorf(
							"[syncCMDB][ERROR] bizID=%d, set=%s, module=%s, svc=%s, proc=%s, hostID=%d: marshal process info failed: %v",
							bizID, set.Name, mod.Name, svc.Name, proc.Name, proc.HostID, err,
						)
						continue
					}

					processBatch = append(processBatch, &table.Process{
						Attachment: &table.ProcessAttachment{
							TenantID:          "default",
							BizID:             uint32(bizID),
							CcProcessID:       uint32(proc.ID),
							SetID:             uint32(set.ID),
							ModuleID:          uint32(mod.ID),
							ServiceInstanceID: uint32(svc.ID),
							HostID:            uint32(proc.HostID),
							CloudID:           uint32(h.CloudId),
							AgentID:           h.AgentID,
							ProcessTemplateID: uint32(proc.ProcessTemplateID),
							ServiceTemplateID: uint32(mod.ServiceTemplateID),
						},
						Spec: &table.ProcessSpec{
							SetName:              set.Name,
							ModuleName:           mod.Name,
							ServiceName:          svc.Name,
							Environment:          set.SetEnv,
							Alias:                proc.Name,
							InnerIP:              h.IP,
							CcSyncStatus:         table.Synced,
							ProcessStateSyncedAt: nil,
							SourceData:           sourceData,
							PrevData:             "{}",
							ProcNum:              uint(proc.ProcNum),
							FuncName:             proc.FuncName,
						},
						Revision: &table.Revision{
							CreatedAt: now,
							UpdatedAt: now,
						},
					})
				}
			}
		}
	}

	return processBatch
}

// buildModuleHosts 根据 ServiceInstance 信息，
// 将 Host 去重后映射到对应的 Module
func buildModuleHosts(allSvcInsts []bkcmdb.ServiceInstanceInfo, hosts []Host) map[int][]Host {
	// hostID -> Host
	hostMap := make(map[int]Host, len(hosts))
	for _, h := range hosts {
		hostMap[h.ID] = h
	}

	// moduleID -> hostID set
	moduleHostIDs := make(map[int]map[int]struct{})

	for _, svc := range allSvcInsts {
		modID := svc.BkModuleID
		hostID := svc.BkHostID
		if hostID == 0 {
			continue
		}

		if _, ok := moduleHostIDs[modID]; !ok {
			moduleHostIDs[modID] = make(map[int]struct{})
		}
		moduleHostIDs[modID][hostID] = struct{}{}
	}

	// moduleID -> []Host
	moduleHosts := make(map[int][]Host, len(moduleHostIDs))
	for modID, hostIDs := range moduleHostIDs {
		for hid := range hostIDs {
			if h, ok := hostMap[hid]; ok {
				moduleHosts[modID] = append(moduleHosts[modID], h)
			}
		}
	}

	return moduleHosts
}

func chunkInts(src []int64, size int) [][]int64 {
	var res [][]int64
	for i := 0; i < len(src); i += size {
		end := i + size
		if end > len(src) {
			end = len(src)
		}
		res = append(res, src[i:end])
	}
	return res
}

func (s *syncCMDBService) fetchAllSets(ctx context.Context) ([]bkcmdb.SetInfo, error) {
	return PageFetcher(func(page *bkcmdb.PageParam) ([]bkcmdb.SetInfo, int, error) {
		resp, err := s.svc.SearchSet(ctx, bkcmdb.SearchSetReq{
			BkSupplierAccount: "0",
			BkBizID:           s.bizID,
			Fields:            []string{"bk_biz_id", "bk_set_id", "bk_set_name", "bk_set_env", "set_template_id"},
			Page:              page,
		})

		if err != nil {
			return nil, 0, err
		}

		return resp.Info, resp.Count, nil
	})
}

func (s *syncCMDBService) fetchAllModules(ctx context.Context, setID int) ([]bkcmdb.ModuleInfo, error) {
	return PageFetcher(func(page *bkcmdb.PageParam) ([]bkcmdb.ModuleInfo, int, error) {
		resp, err := s.svc.SearchModule(ctx, bkcmdb.SearchModuleReq{
			BkSupplierAccount: "0",
			BkBizID:           s.bizID,
			BkSetID:           setID,
			Fields:            []string{"bk_module_id", "bk_module_name", "service_template_id", "bk_set_id"},
		})
		if err != nil {
			return nil, 0, err
		}

		return resp.Info, resp.Count, nil
	})
}

func (s *syncCMDBService) fetchAllHostsBySetTemplate(ctx context.Context, setTemplateIDs []int) (
	[]bkcmdb.HostInfo, error) {
	var all []bkcmdb.HostInfo

	for _, id := range setTemplateIDs {
		hosts, err := PageFetcher(func(page *bkcmdb.PageParam) ([]bkcmdb.HostInfo, int, error) {
			resp, err := s.svc.FindHostBySetTemplate(ctx, bkcmdb.FindHostBySetTemplateReq{
				BkBizID:          s.bizID,
				BkSetTemplateIDs: []int{id},
				Fields: []string{
					"bk_host_id",
					"bk_host_name",
					"bk_host_innerip",
					"bk_cloud_id",
					"bk_agent_id",
				},
				Page: page,
			})
			if err != nil {
				return nil, 0, err
			}

			return resp.Info, resp.Count, nil
		})
		if err != nil {
			return nil, err
		}
		all = append(all, hosts...)
	}

	return all, nil
}

func (s *syncCMDBService) fetchAllServiceInstances(ctx context.Context, moduleID []int) (
	[]*bkcmdb.ServiceInstanceInfo, error) {
	var all []*bkcmdb.ServiceInstanceInfo

	for _, id := range moduleID {
		svcInst, err := PageFetcher(func(page *bkcmdb.PageParam) ([]*bkcmdb.ServiceInstanceInfo, int, error) {
			resp, err := s.svc.ListServiceInstance(ctx, &bkcmdb.ServiceInstanceListReq{
				BkBizID:    s.bizID,
				BkModuleID: id,
				Page:       page,
			})
			if err != nil {
				return nil, 0, err
			}

			return resp.Info, resp.Count, nil
		})
		if err != nil {
			return nil, err
		}
		all = append(all, svcInst...)
	}

	return all, nil
}

// PageFetcher 封装分页逻辑的通用函数
func PageFetcher[T any](fetch func(page *bkcmdb.PageParam) ([]T, int, error)) ([]T, error) {
	var (
		start = 0
		limit = 100
		all   []T
		total = 0
	)

	for {
		page := &bkcmdb.PageParam{
			Start: start,
			Limit: limit,
		}
		data, count, err := fetch(page)
		if err != nil {
			return nil, err
		}

		all = append(all, data...)
		if total == 0 {
			total = count
		}

		if len(all) >= count {
			break
		}
		start += limit
	}
	return all, nil
}

// diffProcesses 对比数据库中的进程列表与 CC 同步过来的进程列表，计算差异
//
// 职责：
//  1. 识别新增 / 更新 / 删除的进程
//  2. 计算实例的新增 / 删除（扩缩容）
//  3. 不做任何 DB 写操作，仅负责 diff 计算
//
// 返回值：
//   - ProcessDiff：包含进程与实例的所有变更集合
//   - error：构建 diff 过程中任一阶段失败
func diffProcesses(ctx *SyncContext, dbProcesses []*table.Process,
	newProcesses []*table.Process) (*ProcessDiff, error) {

	diff := &ProcessDiff{
		ModulesToReorder: make(map[ModuleAliasKey]struct{}),
	}

	// 1. 构建进程索引（以 CcProcessID 为主键）
	// dbProcesses 索引：cc_process_id -> process
	dbProcessByCCID := make(map[uint32]*table.Process, len(dbProcesses))
	for _, p := range dbProcesses {
		dbProcessByCCID[p.Attachment.CcProcessID] = p
	}

	// newProcesses 索引：cc_process_id -> process
	newProcessByCCID := make(map[uint32]*table.Process, len(newProcesses))
	for _, p := range newProcesses {
		newProcessByCCID[p.Attachment.CcProcessID] = p
	}

	// 2. 遍历 newProcesses，计算新增 / 更新 / 实例变更
	for _, newP := range newProcesses {
		oldP, exists := dbProcessByCCID[newP.Attachment.CcProcessID]

		// 2.1 新增进程
		if !exists {
			newP.Revision = &table.Revision{CreatedAt: ctx.Now}
			diff.ToAddProcesses = append(diff.ToAddProcesses, newP)

			// 查询同模块同别名的已有进程，获取最大 ModuleInstSeq
			maxModuleInstSeq := 0
			sameAliasProcessIDs, err := ctx.Dao.Process().ListByModuleIDAndAliasWithTx(
				ctx.Kit, ctx.Tx, newP.Attachment.BizID, newP.Attachment.ModuleID, newP.Spec.Alias)
			if err != nil {
				logs.Errorf("[ProcessDiff][ListByModuleIDAndAlias] bizID=%d moduleID=%d alias=%s failed: %v",
					newP.Attachment.BizID, newP.Attachment.ModuleID, newP.Spec.Alias, err)
				return nil, err
			}
			if len(sameAliasProcessIDs) > 0 {
				maxModuleInstSeq, err = ctx.Dao.ProcessInstance().GetMaxModuleInstSeqByProcessIDsWithTx(
					ctx.Kit, ctx.Tx, newP.Attachment.BizID, sameAliasProcessIDs)
				if err != nil {
					logs.Errorf("[ProcessDiff][GetMaxModuleInstSeq] bizID=%d processIDs=%v failed: %v",
						newP.Attachment.BizID, sameAliasProcessIDs, err)
					return nil, err
				}
			}

			// 为新增进程构建初始实例
			insts := buildInstances(ctx, &BuildInstancesParams{
				BizID:            newP.Attachment.BizID,
				HostID:           newP.Attachment.HostID,
				ModuleID:         newP.Attachment.ModuleID,
				CcProcessID:      newP.Attachment.CcProcessID,
				ProcNum:          int(newP.Spec.ProcNum),
				ExistCount:       0,
				MaxModuleInstSeq: maxModuleInstSeq,
				MaxHostInstSeq:   0,
				Alias:            newP.Spec.Alias,
			})

			diff.ToAddInstances = append(diff.ToAddInstances, insts...)
			continue
		}

		// 2.2 已存在进程，计算变更
		changeResult, err := BuildProcessChanges(ctx, &BuildProcessChangesParams{
			NewProcess: newP,
			OldProcess: oldP,
		})
		if err != nil {
			logs.Errorf(
				"[ProcessDiff][BuildProcessChanges] ccProcessID=%d processID=%d failed: %v",
				newP.Attachment.CcProcessID,
				oldP.ID,
				err,
			)
			return nil, err
		}

		if changeResult.ToAddProcess != nil {
			diff.ToAddProcesses = append(diff.ToAddProcesses, changeResult.ToAddProcess)
		}
		if changeResult.ToUpdateProcess != nil {
			diff.ToUpdateProcesses = append(diff.ToUpdateProcesses, changeResult.ToUpdateProcess)
		}
		if changeResult.ToDeleteProcessID != 0 {
			diff.ToDeleteProcessIDs = append(diff.ToDeleteProcessIDs, changeResult.ToDeleteProcessID)
		}

		diff.ToAddInstances = append(diff.ToAddInstances, changeResult.ToAddInstances...)
		diff.ToDeleteInstanceIDs = append(diff.ToDeleteInstanceIDs, changeResult.ToDeleteInstanceIDs...)

		// 收集需要重新编排的模块
		if changeResult.ModuleToReorder != nil {
			diff.ModulesToReorder[*changeResult.ModuleToReorder] = struct{}{}
		}
	}

	// 3. db 中存在，但 new 中不存在 → 删除进程
	for _, oldP := range dbProcesses {
		if _, ok := newProcessByCCID[oldP.Attachment.CcProcessID]; !ok {
			diff.ToDeleteProcessIDs = append(diff.ToDeleteProcessIDs, oldP.ID)
		}
	}

	return diff, nil
}

// compareProcessInfo returns true if jsonStr (旧的 JSON 字符串) 等价于 jsonStr2 (新的 JSON 字符串).
func compareProcessInfo(jsonStr, jsonStr2 string) (bool, error) {
	var oldInfo table.ProcessInfo
	if err := json.Unmarshal([]byte(jsonStr), &oldInfo); err != nil {
		return false, err
	}

	var newInfo table.ProcessInfo
	if err := json.Unmarshal([]byte(jsonStr2), &newInfo); err != nil {
		return false, err
	}

	return reflect.DeepEqual(oldInfo, newInfo), nil
}

// InstanceReconcileResult 扩缩容实例
type InstanceReconcileResult struct {
	ToAdd    []*table.ProcessInstance
	ToDelete []uint32
}

// reconcileProcessInstances 处理进程实例扩缩容
func reconcileProcessInstances(ctx *SyncContext, params *ReconcileInstancesParams) (*InstanceReconcileResult, error) {
	res := &InstanceReconcileResult{}

	// 数量一致不做处理
	if params.NewNum == params.OldNum {
		return res, nil
	}

	// 缩容
	if params.NewNum < params.OldNum {
		needDelete := params.OldNum - params.NewNum
		insts, err := ctx.Dao.ProcessInstance().ListByProcessIDOrderBySeqDescTx(
			ctx.Kit, ctx.Tx, params.BizID, params.ProcessID, needDelete)
		if err != nil {
			return nil, err
		}

		for _, inst := range insts {
			res.ToDelete = append(res.ToDelete, inst.ID)
		}
	}

	// 扩容：查询同模块同别名的最大序列号
	if params.NewNum > params.OldNum {
		// 获取同模块同别名的所有进程ID
		sameAliasProcessIDs, err := ctx.Dao.Process().ListByModuleIDAndAliasWithTx(
			ctx.Kit, ctx.Tx, params.BizID, params.ModuleID, params.Alias)
		if err != nil {
			return nil, err
		}

		maxModuleSeq := 0
		if len(sameAliasProcessIDs) > 0 {
			maxModuleSeq, err = ctx.Dao.ProcessInstance().GetMaxModuleInstSeqByProcessIDsWithTx(
				ctx.Kit, ctx.Tx, params.BizID, sameAliasProcessIDs)
			if err != nil {
				return nil, err
			}
		}

		maxHostSeq, err := ctx.Dao.ProcessInstance().GetMaxHostInstSeqTx(
			ctx.Kit, ctx.Tx, params.BizID, []uint32{params.ProcessID})
		if err != nil {
			return nil, err
		}

		res.ToAdd = buildInstances(ctx, &BuildInstancesParams{
			BizID:            params.BizID,
			HostID:           params.HostID,
			ModuleID:         params.ModuleID,
			CcProcessID:      params.CcProcessID,
			ProcNum:          params.NewNum,
			ExistCount:       params.OldNum,
			MaxModuleInstSeq: maxModuleSeq,
			MaxHostInstSeq:   maxHostSeq,
			Alias:            params.Alias,
		})
	}

	return res, nil
}

// reorderModuleInstSeq 重新编排同模块同别名的所有实例的 ModuleInstSeq（从1开始递增）
func reorderModuleInstSeq(ctx *SyncContext, params *ReorderParams) ([]*table.ProcessInstance, error) {
	// 1. 查询同模块同别名的所有进程ID
	sameAliasProcessIDs, err := ctx.Dao.Process().ListByModuleIDAndAliasWithTx(
		ctx.Kit, ctx.Tx, params.BizID, params.ModuleID, params.Alias)
	if err != nil {
		return nil, err
	}

	if len(sameAliasProcessIDs) == 0 {
		return nil, nil
	}

	// 2. 查询这些进程的所有实例，按 ProcessID + HostInstSeq 排序
	allInstances, err := ctx.Dao.ProcessInstance().ListByProcessIDsWithTx(
		ctx.Kit, ctx.Tx, params.BizID, sameAliasProcessIDs)
	if err != nil {
		return nil, err
	}

	if len(allInstances) == 0 {
		return nil, nil
	}

	// 构建排除ID集合
	excludeSet := make(map[uint32]struct{}, len(params.ExcludeIDs))
	for _, id := range params.ExcludeIDs {
		excludeSet[id] = struct{}{}
	}

	// 3. 重新编排 ModuleInstSeq（从1开始递增），跳过即将删除的实例
	toUpdate := make([]*table.ProcessInstance, 0)
	seq := 0
	for _, inst := range allInstances {
		// 跳过即将删除的实例
		if _, excluded := excludeSet[inst.ID]; excluded {
			continue
		}

		seq++
		if int(inst.Spec.ModuleInstSeq) != seq {
			inst.Spec.ModuleInstSeq = uint32(seq)
			toUpdate = append(toUpdate, inst)
		}
	}

	return toUpdate, nil
}

// BuildProcessChangesResult 包含 BuildProcessChanges 的返回结果
type BuildProcessChangesResult struct {
	ToAddProcess        *table.Process
	ToUpdateProcess     *table.Process
	ToDeleteProcessID   uint32
	ToAddInstances      []*table.ProcessInstance
	ToDeleteInstanceIDs []uint32
	// 需要重新编排的模块 (moduleID, alias)
	ModuleToReorder *ModuleAliasKey
}

// BuildProcessChanges 根据 CMDB 新旧进程数据，计算进程及其实例的变更结果：
// - 是否需要新增进程
// - 是否需要更新进程
// - 是否需要删除旧进程
// - 是否需要新增/删除/更新进程实例
//
// nolint: funlen
func BuildProcessChanges(ctx *SyncContext, params *BuildProcessChangesParams) (*BuildProcessChangesResult, error) {
	newP := params.NewProcess
	oldP := params.OldProcess
	result := &BuildProcessChangesResult{}

	equal, err := compareProcessInfo(newP.Spec.SourceData, oldP.Spec.SourceData)
	if err != nil {
		return nil, err
	}

	nameChanged := newP.Spec.Alias != oldP.Spec.Alias
	infoChanged := !equal
	numChanged := newP.Spec.ProcNum != oldP.Spec.ProcNum

	if !nameChanged && !infoChanged && !numChanged {
		return result, nil
	}

	// 是否安全
	safe, err := isSafeToUpdateProcess(ctx.Kit, ctx.Dao, ctx.Tx, oldP.Attachment.BizID, oldP.ID)
	if err != nil {
		return nil, err
	}

	newProcNum := int(newP.Spec.ProcNum)

	// 1. 别名变更：检查是否有同别名的 deleted 记录可以复用
	if nameChanged {
		// 查找同 CcProcessID + 同新别名
		reusableProc, err := ctx.Dao.Process().GetByCcProcessIDAndAliasTx(
			ctx.Kit, ctx.Tx, oldP.Attachment.BizID, oldP.Attachment.CcProcessID, newP.Spec.Alias)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		// 恢复 deleted 记录为 synced，并更新其元数据
		if reusableProc != nil {
			reusableProc.Spec.PrevData = oldP.Spec.SourceData
			reusableProc.Spec.SourceData = newP.Spec.SourceData
			reusableProc.Spec.CcSyncStatus = table.Synced
			reusableProc.Spec.ProcNum = newP.Spec.ProcNum
			reusableProc.Attachment = newP.Attachment
			reusableProc.Revision = &table.Revision{UpdatedAt: ctx.Now}

			// 查询恢复进程的现有实例
			existingInsts, err := ctx.Dao.ProcessInstance().ListByProcessIDTx(
				ctx.Kit, ctx.Tx, reusableProc.Attachment.BizID, reusableProc.ID)
			if err != nil {
				return nil, err
			}

			// 根据 ProcNum 扩缩容实例
			res, err := reconcileProcessInstances(ctx, &ReconcileInstancesParams{
				BizID:       reusableProc.Attachment.BizID,
				ProcessID:   reusableProc.ID,
				HostID:      reusableProc.Attachment.HostID,
				ModuleID:    reusableProc.Attachment.ModuleID,
				CcProcessID: reusableProc.Attachment.CcProcessID,
				Alias:       newP.Spec.Alias,
				OldNum:      len(existingInsts),
				NewNum:      newProcNum,
			})
			if err != nil {
				return nil, err
			}

			// 返回：恢复的进程作为更新，旧进程标记为删除
			result.ToUpdateProcess = reusableProc
			result.ToDeleteProcessID = oldP.ID
			result.ToAddInstances = res.ToAdd
			result.ToDeleteInstanceIDs = res.ToDelete
			// 只有实际发生扩缩容时才需要重新排序
			if len(res.ToAdd) > 0 || len(res.ToDelete) > 0 {
				result.ModuleToReorder = &ModuleAliasKey{
					ModuleID: int(reusableProc.Attachment.ModuleID),
					Alias:    newP.Spec.Alias,
				}
			}
			return result, nil
		}

		// 没有可复用的 deleted 记录且不安全：创建新进程，标记旧进程为删除
		if !safe {
			newP.Spec.PrevData = oldP.Spec.SourceData
			newP.Spec.CcSyncStatus = table.Synced

			toAdd := &table.Process{
				Attachment: newP.Attachment,
				Spec:       newP.Spec,
				Revision:   &table.Revision{CreatedAt: ctx.Now},
			}

			// 查询同模块同别名的最大 ModuleInstSeq
			maxModuleInstSeq := 0
			sameAliasProcessIDs, err := ctx.Dao.Process().ListByModuleIDAndAliasWithTx(
				ctx.Kit, ctx.Tx, toAdd.Attachment.BizID, toAdd.Attachment.ModuleID, newP.Spec.Alias)
			if err != nil {
				return nil, err
			}
			if len(sameAliasProcessIDs) > 0 {
				maxModuleInstSeq, err = ctx.Dao.ProcessInstance().GetMaxModuleInstSeqByProcessIDsWithTx(
					ctx.Kit, ctx.Tx, toAdd.Attachment.BizID, sameAliasProcessIDs)
				if err != nil {
					return nil, err
				}
			}

			insts := buildInstances(ctx, &BuildInstancesParams{
				BizID:            toAdd.Attachment.BizID,
				HostID:           toAdd.Attachment.HostID,
				ModuleID:         toAdd.Attachment.ModuleID,
				CcProcessID:      toAdd.Attachment.CcProcessID,
				ProcNum:          newProcNum,
				ExistCount:       0,
				MaxModuleInstSeq: maxModuleInstSeq,
				MaxHostInstSeq:   0,
				Alias:            newP.Spec.Alias,
			})

			result.ToAddProcess = toAdd
			result.ToDeleteProcessID = oldP.ID
			result.ToAddInstances = insts
			return result, nil
		}

		// 安全且没有可复用记录：原地更新别名
		oldP.Spec.Alias = newP.Spec.Alias
	}
	if infoChanged {
		oldP.Spec.PrevData = oldP.Spec.SourceData
		oldP.Spec.SourceData = newP.Spec.SourceData
		// 进程变更 + 安全 就是已同步的状态
		if safe {
			oldP.Spec.CcSyncStatus = table.Synced
		} else {
			oldP.Spec.CcSyncStatus = table.Updated
		}
	}

	// 实例调整逻辑
	if numChanged {
		// 更新进程的 ProcNum 字段
		oldP.Spec.ProcNum = newP.Spec.ProcNum

		// 真实实例数
		allInsts, err := ctx.Dao.ProcessInstance().ListByProcessIDTx(ctx.Kit, ctx.Tx, oldP.Attachment.BizID, oldP.ID)
		if err != nil {
			return nil, err
		}

		res, err := reconcileProcessInstances(ctx, &ReconcileInstancesParams{
			BizID:       oldP.Attachment.BizID,
			ProcessID:   oldP.ID,
			HostID:      oldP.Attachment.HostID,
			ModuleID:    oldP.Attachment.ModuleID,
			CcProcessID: oldP.Attachment.CcProcessID,
			Alias:       oldP.Spec.Alias,
			OldNum:      len(allInsts),
			NewNum:      newProcNum,
		})
		if err != nil {
			return nil, err
		}

		result.ToAddInstances = res.ToAdd
		result.ToDeleteInstanceIDs = res.ToDelete

		// 不安全场景：不允许缩容
		if newProcNum < len(allInsts) && !safe {
			result.ToDeleteInstanceIDs = nil
		}

		// 只有实际发生扩缩容时才需要重新排序
		if len(result.ToAddInstances) > 0 || len(result.ToDeleteInstanceIDs) > 0 {
			result.ModuleToReorder = &ModuleAliasKey{
				ModuleID: int(oldP.Attachment.ModuleID),
				Alias:    oldP.Spec.Alias,
			}
		}
	}

	toUpdate := &table.Process{
		ID:         oldP.ID,
		Attachment: oldP.Attachment,
		Spec:       oldP.Spec,
		Revision:   &table.Revision{UpdatedAt: ctx.Now},
	}

	result.ToUpdateProcess = toUpdate
	return result, nil
}

// buildInstances 根据进程数量生成进程实例
// moduleCounter 的 key 为 ModuleAliasKey，使同模块同别名的进程共享 ModuleInstSeq 递增序列
func buildInstances(ctx *SyncContext, params *BuildInstancesParams) []*table.ProcessInstance {
	// 如果新的进程数量 <= 已存在数量，则无需新增实例
	if params.ProcNum <= params.ExistCount {
		return nil
	}

	// 需要新增的实例数量
	newCount := params.ProcNum - params.ExistCount
	if newCount <= 0 {
		return nil
	}

	instances := make([]*table.ProcessInstance, 0, newCount)

	// 维度 key： (ccProcessID, hostID) 用于 HostInstSeq
	// (moduleID, alias) 用于 ModuleInstSeq，使同模块同别名共享序列
	hostKey := HostProcessKey{CcProcessID: int(params.CcProcessID), HostID: int(params.HostID)}
	modKey := ModuleAliasKey{ModuleID: int(params.ModuleID), Alias: params.Alias}

	// 从缓存取
	startHostInstSeq := ctx.HostCounter[hostKey]
	startModuleInstSeq := ctx.ModuleCounter[modKey]

	// 如果缓存未初始化，则从数据库最大值开始
	if startHostInstSeq == 0 {
		startHostInstSeq = params.MaxHostInstSeq
		ctx.HostCounter[hostKey] = startHostInstSeq
	}
	if startModuleInstSeq == 0 {
		startModuleInstSeq = params.MaxModuleInstSeq
		ctx.ModuleCounter[modKey] = startModuleInstSeq
	}

	for i := 1; i <= newCount; i++ {
		ctx.HostCounter[hostKey]++
		ctx.ModuleCounter[modKey]++

		hostInstSeq := ctx.HostCounter[hostKey]
		moduleInstSeq := ctx.ModuleCounter[modKey]

		instances = append(instances, &table.ProcessInstance{
			Attachment: &table.ProcessInstanceAttachment{
				TenantID:    "default",
				BizID:       params.BizID,
				CcProcessID: params.CcProcessID,
			},
			Spec: &table.ProcessInstanceSpec{
				StatusUpdatedAt: ctx.Now,
				HostInstSeq:     uint32(hostInstSeq),
				ModuleInstSeq:   uint32(moduleInstSeq),
			},
			Revision: &table.Revision{
				CreatedAt: ctx.Now,
				UpdatedAt: ctx.Now,
			},
		})
	}

	return instances
}

// ProcessDiff 进程差异对比
type ProcessDiff struct {
	// 进程级
	ToAddProcesses     []*table.Process
	ToUpdateProcesses  []*table.Process
	ToDeleteProcessIDs []uint32

	// 实例级
	ToAddInstances      []*table.ProcessInstance
	ToDeleteInstanceIDs []uint32

	// 需要重新编排 ModuleInstSeq 的模块 (moduleID, alias) 集合
	ModulesToReorder map[ModuleAliasKey]struct{}

	// side effect
	NeedSyncGSE bool
}

// SyncProcessData 对比并同步进程及进程实例数据
//
// 同步策略：
//  1. 基于 oldProcesses / newProcesses 做差异计算
//  2. 先删除实例（缩容）
//  3. 再删除进程（包含兜底删除 stopped / unmanaged 实例）
//  4. 新增进程
//  5. 更新进程
//  6. 回填 ProcessID 到新增实例
//  7. 新增实例（扩容 / 新进程）
//
// 返回值：
//   - SyncProcessResult：本次新增/更新的进程及其新增实例
//   - error：任一阶段失败则直接返回
//
// nolint:funlen
func SyncProcessData(kit *kit.Kit, dao dao.Set, tx *gen.QueryTx, bizID uint32, oldProcesses,
	newProcesses []*table.Process) (*SyncProcessResult, error) {

	// 没有新进程数据，直接返回空结果
	if len(newProcesses) == 0 {
		return &SyncProcessResult{}, nil
	}

	// 创建同步上下文
	ctx := &SyncContext{
		Kit:           kit,
		Dao:           dao,
		Tx:            tx,
		Now:           time.Now().UTC(),
		HostCounter:   make(map[HostProcessKey]int),
		ModuleCounter: make(map[ModuleAliasKey]int),
	}

	diff, err := diffProcesses(ctx, oldProcesses, newProcesses)
	if err != nil {
		logs.Errorf(
			"[ProcessSync][DiffProcesses] bizID=%d failed: %v",
			bizID, err,
		)
		return nil, err
	}

	// 1. 先删实例（缩容）
	if len(diff.ToDeleteInstanceIDs) > 0 {
		if err := dao.ProcessInstance().BatchDeleteByIDsWithTx(kit, tx, diff.ToDeleteInstanceIDs); err != nil {
			logs.Errorf(
				"[ProcessSync][DeleteInstance] bizID=%d instanceIDs=%v failed: %v",
				bizID, diff.ToDeleteInstanceIDs, err,
			)
			return nil, err
		}
	}

	// 2. 删除进程（并兜底清理 stopped / unmanaged 实例）
	if len(diff.ToDeleteProcessIDs) > 0 {
		if err := DeleteInstanceStoppedUnmanaged(kit, dao, tx, bizID, diff.ToDeleteProcessIDs); err != nil {
			logs.Errorf(
				"[ProcessSync][DeleteProcess] bizID=%d processIDs=%v failed: %v",
				bizID, diff.ToDeleteProcessIDs, err,
			)
			return nil, err
		}
	}

	// 3. 新增进程
	if len(diff.ToAddProcesses) > 0 {
		if err := dao.Process().BatchCreateWithTx(kit, tx, diff.ToAddProcesses); err != nil {
			logs.Errorf(
				"[ProcessSync][CreateProcess] bizID=%d processCount=%d failed: %v",
				bizID, len(diff.ToAddProcesses), err,
			)
			return nil, err
		}
	}

	// 4. 更新进程
	if len(diff.ToUpdateProcesses) > 0 {
		if err := dao.Process().BatchUpdateWithTx(kit, tx, diff.ToUpdateProcesses); err != nil {
			logs.Errorf(
				"[ProcessSync][UpdateProcess] bizID=%d processCount=%d failed: %v",
				bizID, len(diff.ToUpdateProcesses), err,
			)
			return nil, err
		}
	}

	// 5. 构建 (tenantID + bizID + ccProcessID) -> ProcessID 映射
	// 用于给新增实例回填 ProcessID
	processIDByKey := make(map[string]uint32)
	buildKey := func(tenantID string, bizID uint32, ccProcessID uint32) string {
		return fmt.Sprintf("%s-%d-%d", tenantID, bizID, ccProcessID)
	}
	for _, p := range diff.ToAddProcesses {
		key := buildKey(p.Attachment.TenantID, bizID, p.Attachment.CcProcessID)
		processIDByKey[key] = p.ID
	}

	for _, p := range diff.ToUpdateProcesses {
		key := buildKey(p.Attachment.TenantID, bizID, p.Attachment.CcProcessID)
		processIDByKey[key] = p.ID
	}

	// 6. 回填 ProcessID 到新增实例
	for _, inst := range diff.ToAddInstances {
		key := buildKey(inst.Attachment.TenantID, bizID, inst.Attachment.CcProcessID)
		if pid := processIDByKey[key]; pid != 0 {
			inst.Attachment.ProcessID = pid
		} else {
			logs.Warnf(
				"[ProcessSync][FillProcessID] bizID=%d ccProcessID=%d instanceID=%d process not found",
				bizID,
				inst.Attachment.CcProcessID,
				inst.ID,
			)
		}
	}

	// 7. 新增实例（扩容 / 新进程）
	if len(diff.ToAddInstances) > 0 {
		if err := dao.ProcessInstance().
			BatchCreateWithTx(kit, tx, diff.ToAddInstances); err != nil {
			logs.Errorf(
				"[ProcessSync][CreateInstance] bizID=%d instanceCount=%d failed: %v",
				bizID, len(diff.ToAddInstances), err,
			)
			return nil, err
		}
	}

	// 8. 重新编排受影响模块的 ModuleInstSeq（在所有实例入库后统一执行）
	for modKey := range diff.ModulesToReorder {
		toUpdate, err := reorderModuleInstSeq(ctx, &ReorderParams{
			BizID:    bizID,
			ModuleID: uint32(modKey.ModuleID),
			Alias:    modKey.Alias,
		})
		if err != nil {
			logs.Errorf(
				"[ProcessSync][ReorderModuleInstSeq] bizID=%d moduleID=%d alias=%s failed: %v",
				bizID, modKey.ModuleID, modKey.Alias, err,
			)
			return nil, err
		}

		if len(toUpdate) > 0 {
			if err := dao.ProcessInstance().BatchUpdateWithTx(kit, tx, toUpdate); err != nil {
				logs.Errorf(
					"[ProcessSync][UpdateModuleInstSeq] bizID=%d instanceCount=%d failed: %v",
					bizID, len(toUpdate), err,
				)
				return nil, err
			}
		}
	}

	// 构建 ProcessID -> 新增实例 列表映射
	newInstancesByProcessID := make(map[uint32][]*table.ProcessInstance)
	for _, inst := range diff.ToAddInstances {
		if inst.Attachment.ProcessID != 0 {
			newInstancesByProcessID[inst.Attachment.ProcessID] =
				append(newInstancesByProcessID[inst.Attachment.ProcessID], inst)
		}
	}

	// 汇总返回结果
	result := &SyncProcessResult{}

	// 新增进程
	for _, p := range diff.ToAddProcesses {
		result.Items = append(result.Items, &ProcessWithInstances{
			Process:   p,
			Instances: newInstancesByProcessID[p.ID],
		})
	}

	// 更新进程
	for _, p := range diff.ToUpdateProcesses {
		result.Items = append(result.Items, &ProcessWithInstances{
			Process:   p,
			Instances: newInstancesByProcessID[p.ID],
		})
	}

	return result, nil
}

// DeleteInstanceStoppedUnmanaged 删除进程状态是已停止或者是空 和 托管状态是未托管或者是空的实例数据
func DeleteInstanceStoppedUnmanaged(kit *kit.Kit, dao dao.Set, tx *gen.QueryTx, bizID uint32, processIDs []uint32) error {
	// 1. 删除processes表中的数据
	err := dao.Process().UpdateSyncStatusWithTx(kit, tx, table.Deleted.String(), processIDs)
	if err != nil {
		return err
	}

	// 2. 删除process_instances表中的数据
	err = dao.ProcessInstance().DeleteStoppedUnmanagedWithTx(kit, tx, bizID, processIDs)
	if err != nil {
		return err
	}

	return nil
}

func isSafeToUpdateProcess(
	kit *kit.Kit,
	dao dao.Set,
	tx *gen.QueryTx,
	bizID uint32,
	processID uint32,
) (bool, error) {

	insts, err := dao.ProcessInstance().ListByProcessIDTx(kit, tx, bizID, processID)
	if err != nil {
		return false, err
	}

	for _, inst := range insts {
		// 进程不是停止和空都是不安全
		if inst.Spec.Status != table.ProcessStatusStopped && inst.Spec.Status != "" {
			return false, nil
		}
		// 托管不是未托管和空都是不安全
		if inst.Spec.ManagedStatus != table.ProcessManagedStatusUnmanaged &&
			inst.Spec.ManagedStatus != "" {
			return false, nil
		}
	}

	return true, nil
}
