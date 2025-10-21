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

// Package cmdb provides cmdb client.
package cmdb

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"slices"
	"strconv"
	"time"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
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

// SyncSingleBiz 单个业务同步
// nolint: funlen
func (s *syncCMDBService) SyncSingleBiz(ctx context.Context) error {
	kit := kit.FromGrpcContext(ctx)
	// 1. 获取集群
	listSets, err := s.fetchAllSets(ctx)
	if err != nil {
		return err
	}

	var sets []Set
	for _, set := range listSets {
		sets = append(sets, Set{ID: set.BkSetID, Name: set.BkSetName, SetEnv: set.BkSetEnv})
	}

	// 2. 模块
	for i := range listSets {
		listModules, errM := s.fetchAllModules(ctx, sets[i].ID)
		if errM != nil {
			return errM
		}
		for _, m := range listModules {
			module := Module{ID: m.BkModuleID, Name: m.BkModuleName}
			sets[i].Module = append(sets[i].Module, module)
		}
	}

	// 3. 主机
	setTemplateIDs := []int{}
	for _, set := range listSets {
		if set.SetTemplateID > 0 && !slices.Contains(setTemplateIDs, set.SetTemplateID) {
			setTemplateIDs = append(setTemplateIDs, set.SetTemplateID)
		}
	}

	listHosts, err := s.fetchAllHostsBySetTemplate(ctx, setTemplateIDs)
	if err != nil {
		return fmt.Errorf("fetch all hosts by set template failed: %v", err)
	}
	var hosts []Host
	for _, h := range listHosts {
		hosts = append(hosts, Host{ID: h.BkHostID, Name: h.BkHostName, IP: h.BkHostInnerIP})
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
		return fmt.Errorf("fetch all service instances failed: %v", err)
	}

	moduleSvcMap := map[int][]SvcInst{}
	for _, inst := range listSvcInsts {
		moduleSvcMap[inst.BkModuleID] = append(moduleSvcMap[inst.BkModuleID], SvcInst{
			ID: inst.ID, Name: inst.Name,
		})
	}

	// 5. 进程
	listProcMap := map[int][]ProcInst{}
	for _, inst := range listSvcInsts {
		processInstanceList, err := s.svc.ListProcessInstance(ctx, bkcmdb.ListProcessInstanceReq{
			BkBizID: s.bizID, ServiceInstanceID: inst.ID,
		})
		if err != nil {
			return fmt.Errorf("fetch all process instances failed: %v", err)
		}

		var procs []bkcmdb.ListProcessInstance
		if err := processInstanceList.Decode(&procs); err != nil {
			return err
		}
		for _, proc := range procs {
			listProcMap[inst.ID] = append(listProcMap[inst.ID], ProcInst{
				ID:      proc.Property.BkProcessID,
				HostID:  proc.Relation.BkHostID,
				Name:    proc.Property.BkProcessName,
				ProcNum: proc.Property.ProcNum,
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
				},
			})
		}
	}

	// 6. 拼装
	for si, set := range sets {
		for mi, mod := range set.Module {
			svcList := moduleSvcMap[mod.ID]
			for sj, svc := range svcList {
				svcList[sj].ProcInst = listProcMap[svc.ID]
			}
			sets[si].Module[mi].SvcInst = svcList
			sets[si].Module[mi].Host = hosts
		}
	}

	// 构建并立即入库
	bizs := Bizs{s.bizID: sets}

	// 构建 Process 和 ProcessInstance 数据
	processBatch, processInstanceBatch := buildProcessAndInstance(bizs)

	// 开启事务并入库
	tx := s.dao.GenQuery().Begin()

	if err := s.syncProcessAndInstanceData(kit, tx, processBatch, processInstanceBatch); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			log.Printf("[ERROR] rollback failed for bizID=%d: %v", s.bizID, rbErr)
		}
		return fmt.Errorf("sync process and instance data failed for biz %d: %v", s.bizID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed for biz %d: %v", s.bizID, err)
	}

	log.Printf("[INFO] bizID=%d process data synced, %d processes written", s.bizID, len(processBatch))

	return nil
}

func (s *syncCMDBService) fetchAllSets(ctx context.Context) ([]bkcmdb.SetInfo, error) {
	return pageFetcher(func(page *bkcmdb.PageParam) ([]bkcmdb.SetInfo, int, error) {
		resp, err := s.svc.SearchSet(ctx, bkcmdb.SearchSetReq{
			BkSupplierAccount: "0",
			BkBizID:           s.bizID,
			Fields:            []string{"bk_biz_id", "bk_set_id", "bk_set_name", "bk_set_env", "set_template_id"},
			Page:              page,
		})
		if err != nil {
			return nil, 0, err
		}
		var result bkcmdb.Sets
		if err := resp.Decode(&result); err != nil {
			return nil, 0, err
		}
		return result.Info, result.Count, nil
	})
}

func (s *syncCMDBService) fetchAllModules(ctx context.Context, setID int) ([]bkcmdb.ModuleInfo, error) {
	return pageFetcher(func(page *bkcmdb.PageParam) ([]bkcmdb.ModuleInfo, int, error) {
		resp, err := s.svc.SearchModule(ctx, bkcmdb.SearchModuleReq{
			BkSupplierAccount: "0",
			BkBizID:           s.bizID,
			BkSetID:           setID,
			Fields:            []string{"bk_module_id", "bk_module_name"},
		})
		if err != nil {
			return nil, 0, err
		}
		var result bkcmdb.ModuleListResp
		if err := resp.Decode(&result); err != nil {
			return nil, 0, err
		}
		return result.Info, result.Count, nil
	})
}

func (s *syncCMDBService) fetchAllHostsBySetTemplate(ctx context.Context, setTemplateIDs []int) (
	[]bkcmdb.HostInfo, error) {
	var all []bkcmdb.HostInfo

	for _, id := range setTemplateIDs {
		hosts, err := pageFetcher(func(page *bkcmdb.PageParam) ([]bkcmdb.HostInfo, int, error) {
			resp, err := s.svc.FindHostBySetTemplate(ctx, bkcmdb.FindHostBySetTemplateReq{
				BkBizID:          s.bizID,
				BkSetTemplateIDs: []int{id},
				Fields: []string{
					"bk_host_id",
					"bk_host_name",
					"bk_host_innerip",
				},
				Page: page,
			})
			if err != nil {
				return nil, 0, err
			}

			var result bkcmdb.HostListResp
			if err := resp.Decode(&result); err != nil {
				return nil, 0, err
			}
			return result.Info, result.Count, nil
		})
		if err != nil {
			return nil, err
		}
		all = append(all, hosts...)
	}

	return all, nil
}

func (s *syncCMDBService) fetchAllServiceInstances(ctx context.Context, moduleID []int) (
	[]bkcmdb.ServiceInstanceInfo, error) {
	var all []bkcmdb.ServiceInstanceInfo

	for _, id := range moduleID {
		svcInst, err := pageFetcher(func(page *bkcmdb.PageParam) ([]bkcmdb.ServiceInstanceInfo, int, error) {
			resp, err := s.svc.ListServiceInstance(ctx, bkcmdb.ServiceInstanceListReq{
				BkBizID:    s.bizID,
				BkModuleID: id,
				Page:       page,
			})
			if err != nil {
				return nil, 0, err
			}
			var result bkcmdb.ServiceInstanceResp
			if err := resp.Decode(&result); err != nil {
				return nil, 0, err
			}

			return result.Info, result.Count, nil
		})
		if err != nil {
			return nil, err
		}
		all = append(all, svcInst...)
	}

	return all, nil
}

// buildProcessAndInstance 处理进程和实例数据
func buildProcessAndInstance(bizs Bizs) ([]*table.Process, []*table.ProcessInstance) {
	now := time.Now()

	var (
		processBatch         []*table.Process
		processInstanceBatch []*table.ProcessInstance
	)

	for bizID, sets := range bizs {
		for _, set := range sets {
			hostCounter := make(map[int]int)
			moduleCounter := make(map[int]int)
			for _, mod := range set.Module {
				// 构建 HostID -> IP 映射
				hostMap := make(map[int]string, len(mod.Host))
				for _, h := range mod.Host {
					hostMap[h.ID] = h.IP
				}
				for _, svc := range mod.SvcInst {
					for _, proc := range svc.ProcInst {
						ip := hostMap[proc.HostID]
						sourceData, err := proc.ProcessInfo.Value()
						if err != nil {
							log.Printf("[ERROR] bizID=%d, set=%s, module=%s, svc=%s, proc=%s, hostID=%d: failed to get process info: %v",
								bizID, set.Name, mod.Name, svc.Name, proc.Name, proc.HostID, err)
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
							},
							Spec: &table.ProcessSpec{
								SetName:         set.Name,
								ModuleName:      mod.Name,
								ServiceName:     svc.Name,
								Environment:     translateEnv(set.SetEnv),
								Alias:           proc.Name,
								InnerIP:         ip,
								CcSyncStatus:    table.Synced,
								CcSyncUpdatedAt: now,
								SourceData:      sourceData,
								PrevData:        "{}",
								ProcNum:         uint(proc.ProcNum),
							},
							Revision: &table.Revision{
								CreatedAt: now,
								UpdatedAt: now,
							},
						})

						instances := buildInstances(&proc, bizID, mod.ID, now, hostCounter, moduleCounter)
						processInstanceBatch = append(processInstanceBatch, instances...)
					}
				}
			}
		}
	}

	return processBatch, processInstanceBatch
}

func buildInstances(proc *ProcInst, bizID, modID int, now time.Time, hostCounter map[int]int,
	moduleCounter map[int]int) []*table.ProcessInstance {

	num := proc.ProcNum
	if num <= 0 {
		num = 1
	}

	instances := make([]*table.ProcessInstance, 0, num)
	for range num {
		// 先递增计数器
		hostCounter[proc.HostID]++
		moduleCounter[modID]++

		instances = append(instances, &table.ProcessInstance{
			Attachment: &table.ProcessInstanceAttachment{
				TenantID:    "default",
				BizID:       uint32(bizID),
				CcProcessID: uint32(proc.ID),
			},
			Spec: &table.ProcessInstanceSpec{
				StatusUpdatedAt: now,
				LocalInstID:     strconv.Itoa(hostCounter[proc.HostID]), // 同主机递增
				InstID:          strconv.Itoa(moduleCounter[modID]),     // 同模块递增
			},
			Revision: &table.Revision{
				CreatedAt: now,
				UpdatedAt: now,
			},
		})
	}
	return instances
}

// pageFetcher 封装分页逻辑的通用函数
func pageFetcher[T any](fetch func(page *bkcmdb.PageParam) ([]T, int, error)) ([]T, error) {
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

func diffProcesses(dbProcesses []*table.Process, newProcesses []*table.Process) (toAdd, toUpdate []*table.Process,
	toDelete []uint32) {

	dbMap := make(map[uint32]*table.Process)
	for _, p := range dbProcesses {
		dbMap[p.Attachment.CcProcessID] = p
	}

	newMap := make(map[uint32]*table.Process)
	for _, p := range newProcesses {
		newMap[p.Attachment.CcProcessID] = p
	}

	for _, newP := range newProcesses {
		dbP, exists := dbMap[newP.Attachment.CcProcessID]
		if !exists {
			// DB 没有，新数据 ⇒ 新增
			toAdd = append(toAdd, newP)
			continue
		}

		// 如果 alias 变更 ⇒ 标记旧为 deleted，新增新记录
		if dbP.Spec.Alias != newP.Spec.Alias {
			dbP.Spec.CcSyncStatus = table.Deleted
			toDelete = append(toDelete, dbP.ID)
			toAdd = append(toAdd, newP)
			continue
		}

		// 其他字段变动 ⇒ 更新
		if !reflect.DeepEqual(dbP.Spec, newP.Spec) {
			newP.ID = dbP.ID // 保留原 id 更新
			toUpdate = append(toUpdate, newP)
		}
	}

	// DB 有但新数据没有 ⇒ 删除
	for _, dbP := range dbProcesses {
		if _, exists := newMap[dbP.Attachment.CcProcessID]; !exists {
			dbP.Spec.CcSyncStatus = table.Deleted
			toDelete = append(toDelete, dbP.ID)
		}
	}

	return toAdd, toUpdate, toDelete
}

func (s *syncCMDBService) syncProcessAndInstanceData(kit *kit.Kit, tx *gen.QueryTx, processBatch []*table.Process,
	processInstanceBatch []*table.ProcessInstance) error {
	if len(processBatch) == 0 {
		return nil
	}

	tenantID := processBatch[0].Attachment.TenantID
	bizID := processBatch[0].Attachment.BizID

	// 查询数据库中已有数据
	dbProcesses, err := s.dao.Process().ListProcByBizIDWithTx(kit, tx, tenantID, bizID)
	if err != nil {
		return fmt.Errorf("list processes failed: %w", err)
	}

	// 比对
	toAdd, toUpdate, toDelete := diffProcesses(dbProcesses, processBatch)

	// 插入
	if len(toAdd) > 0 {
		if err := s.dao.Process().BatchCreateWithTx(kit, tx, toAdd); err != nil {
			return fmt.Errorf("insert failed: %w", err)
		}
	}

	// 更新
	if len(toUpdate) > 0 {
		if err := s.dao.Process().BatchUpdateWithTx(kit, tx, toUpdate); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
	}

	// 删除
	if len(toDelete) > 0 {
		if err := s.dao.Process().UpdateSyncStatusWithTx(kit, tx, string(table.Deleted), toDelete); err != nil {
			return fmt.Errorf("mark deleted failed: %w", err)
		}
	}

	// 回填 ProcessID 给 Instance
	idMap := make(map[string]uint32)
	for _, p := range toAdd {
		idMap[fmt.Sprintf("%s-%d-%d", p.Attachment.TenantID, bizID, p.Attachment.CcProcessID)] = p.ID
	}

	for _, inst := range processInstanceBatch {
		if pid, ok := idMap[fmt.Sprintf("%s-%d-%d", inst.Attachment.TenantID, bizID,
			inst.Attachment.CcProcessID)]; ok {
			inst.Attachment.ProcessID = pid
		}
	}

	// 插入 process instance
	if len(processInstanceBatch) > 0 {
		if err := s.dao.ProcessInstance().BatchCreateWithTx(kit, tx, processInstanceBatch); err != nil {
			return fmt.Errorf("insert process instances failed: %w", err)
		}
	}

	return nil
}

func translateEnv(env string) string {
	switch env {
	case "1":
		return "测试"
	case "2":
		return "体验"
	case "3":
		return "正式"
	default:
		return "未知"
	}
}
