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
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
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
		hosts = append(hosts, Host{ID: h.BkHostID, Name: h.BkHostName, IP: h.BkHostInnerIP,
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
		procs, err := s.svc.ListProcessInstance(ctx, bkcmdb.ListProcessInstanceReq{
			BkBizID: s.bizID, ServiceInstanceID: inst.ID,
		})
		if err != nil {
			return fmt.Errorf("fetch all process instances failed: %v", err)
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
			logs.Errorf("[ERROR] rollback failed for bizID=%d: %v", s.bizID, rbErr)
		}
		return fmt.Errorf("sync process and instance data failed for biz %d: %v", s.bizID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed for biz %d: %v", s.bizID, err)
	}

	logs.Infof("[INFO] bizID=%d process data synced, %d processes written", s.bizID, len(processBatch))

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

		return resp.Info, resp.Count, nil
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

		return resp.Info, resp.Count, nil
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

			return resp.Info, resp.Count, nil
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
				hostMap := make(map[int]HostInfo, len(mod.Host))
				for _, h := range mod.Host {
					hostMap[h.ID] = HostInfo{
						IP:      h.IP,
						CloudId: h.CloudId,
						AgentID: h.AgentID,
					}
				}
				for _, svc := range mod.SvcInst {
					for _, proc := range svc.ProcInst {
						hinfo, ok := hostMap[proc.HostID]
						if !ok {
							log.Printf("[WARN] bizID=%d, set=%s, module=%s, svc=%s, proc=%s: hostID=%d not found in hostMap",
								bizID, set.Name, mod.Name, svc.Name, proc.Name, proc.HostID)
							continue
						}
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
								CloudID:           uint32(hinfo.CloudId),
								AgentID:           hinfo.AgentID,
							},
							Spec: &table.ProcessSpec{
								SetName:         set.Name,
								ModuleName:      mod.Name,
								ServiceName:     svc.Name,
								Environment:     translateEnv(set.SetEnv),
								Alias:           proc.Name,
								InnerIP:         hinfo.IP,
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

						instances := BuildInstances(bizID, proc.HostID, mod.ID, proc.ID, proc.ProcNum, now, hostCounter, moduleCounter)
						processInstanceBatch = append(processInstanceBatch, instances...)
					}
				}
			}
		}
	}

	return processBatch, processInstanceBatch
}

// BuildInstances 生成进程实例
func BuildInstances(bizID, hostID, modID, processID, procNum int, now time.Time, hostCounter map[int]int,
	moduleCounter map[int]int) []*table.ProcessInstance {

	num := procNum
	if num <= 0 {
		num = 1
	}

	instances := make([]*table.ProcessInstance, 0, num)
	for range num {
		// 先递增计数器
		hostCounter[hostID]++
		moduleCounter[modID]++

		instances = append(instances, &table.ProcessInstance{
			Attachment: &table.ProcessInstanceAttachment{
				TenantID:    "default",
				BizID:       uint32(bizID),
				CcProcessID: uint32(processID),
			},
			Spec: &table.ProcessInstanceSpec{
				StatusUpdatedAt: now,
				LocalInstID:     strconv.Itoa(hostCounter[hostID]),  // 同主机递增
				InstID:          strconv.Itoa(moduleCounter[modID]), // 同模块递增
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

// diffProcesses 函数比较当前进程列表 (dbProcesses) 和新进程列表 (newProcesses)
func diffProcesses(dbProcesses []*table.Process, newProcesses []*table.Process) (toAdd, toUpdate []*table.Process,
	toDelete []uint32, err error) {

	now := time.Now().UTC()

	// 1. 构建 map 方便对比
	dbMap := make(map[uint32]*table.Process)
	for _, p := range dbProcesses {
		dbMap[p.Attachment.CcProcessID] = p
	}

	newMap := make(map[uint32]*table.Process)
	for _, p := range newProcesses {
		newMap[p.Attachment.CcProcessID] = p
	}

	// 2. 遍历 newProcesses
	for _, newP := range newProcesses {
		oldP, exists := dbMap[newP.Attachment.CcProcessID]
		if !exists {
			// 新增项：数据库中没有，直接加入新增列表
			newP.Revision = &table.Revision{CreatedAt: now}
			toAdd = append(toAdd, newP)
			continue
		}

		// 调用 BuildProcessChanges 比较差异
		add, del, update, err := BuildProcessChanges(newP, oldP, now)
		if err != nil {
			return nil, nil, nil, err
		}

		toAdd = append(toAdd, add...)
		toDelete = append(toDelete, del...)
		toUpdate = append(toUpdate, update...)
	}

	// 3. 找出被删除的项（在 db 里有，但 new 里没有）
	for _, oldP := range dbProcesses {
		if _, exists := newMap[oldP.Attachment.CcProcessID]; !exists {
			toDelete = append(toDelete, oldP.ID)
		}
	}

	return toAdd, toUpdate, toDelete, nil
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

// CompareProcessInfo returns true if jsonStr (旧的 JSON 字符串) 等价于 jsonStr2 (新的 JSON 字符串).
func CompareProcessInfo(jsonStr, jsonStr2 string) (bool, error) {
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

// BuildProcessChanges 生成进程数据
func BuildProcessChanges(newP *table.Process, oldP *table.Process, now time.Time) (toAdd []*table.Process,
	toDelete []uint32, toUpdate []*table.Process, err error) {

	// 1. 判断内容是否变化
	equal, err := CompareProcessInfo(newP.Spec.SourceData, oldP.Spec.SourceData)
	if err != nil {
		return nil, nil, nil, err
	}

	// 名称不相等
	nameChanged := newP.Spec.Alias != oldP.Spec.Alias
	// 内容不相等
	infoChanged := !equal
	// 数量不相等
	numChanged := newP.Spec.ProcNum != oldP.Spec.ProcNum

	// 情况 1: 名称变化
	if nameChanged {
		// 名称变化 → 删除旧数据 + 新增新数据
		status := table.Synced
		if infoChanged {
			status = table.Updated // 名称 + 内容 都变更
		}

		toDelete = append(toDelete, oldP.ID)

		newP.Spec.CcSyncStatus = status
		newP.Spec.PrevData = oldP.Spec.SourceData
		toAdd = append(toAdd, &table.Process{
			Attachment: newP.Attachment,
			Spec:       newP.Spec,
			Revision:   &table.Revision{CreatedAt: now},
		})

		return toAdd, toDelete, toUpdate, err
	}

	// 情况 2: 内容变化或数量变化
	// 名称相同 → 仅更新，不删除
	if infoChanged || numChanged {
		// 如果内容变化，则更新状态为 Updated
		if infoChanged {
			oldP.Spec.SourceData = newP.Spec.SourceData
			oldP.Spec.PrevData = oldP.Spec.SourceData
			oldP.Spec.CcSyncStatus = table.Updated
			oldP.Spec.CcSyncUpdatedAt = now
		}

		// 如果数量变化，更新 ProcNum，但保持状态（除非 info 也变）
		if numChanged {
			oldP.Spec.ProcNum = newP.Spec.ProcNum
		}

		toUpdate = append(toUpdate, &table.Process{
			ID:         oldP.ID,
			Attachment: oldP.Attachment,
			Spec:       oldP.Spec,
			Revision:   &table.Revision{UpdatedAt: now},
		})
	}

	return toAdd, toDelete, toUpdate, nil
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
	toAdd, toUpdate, toDelete, err := diffProcesses(dbProcesses, processBatch)
	if err != nil {
		return err
	}
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

	// 只有新增的进程才需要创建实例
	if len(toAdd) == 0 {
		return nil
	}

	// 构建新增进程的 ProcessID 映射
	idMap := make(map[string]uint32)
	for _, p := range toAdd {
		key := fmt.Sprintf("%s-%d-%d", p.Attachment.TenantID, bizID, p.Attachment.CcProcessID)
		idMap[key] = p.ID
	}

	// 回填 ProcessID 给 Instance，只处理新增进程对应的实例
	var toWriteInstances []*table.ProcessInstance
	for _, inst := range processInstanceBatch {
		key := fmt.Sprintf("%s-%d-%d", inst.Attachment.TenantID, bizID, inst.Attachment.CcProcessID)
		if pid, ok := idMap[key]; ok && pid != 0 {
			inst.Attachment.ProcessID = pid
			toWriteInstances = append(toWriteInstances, inst)
		}
		// 如果找不到对应的新增进程，则跳过这个实例
	}

	// 只写入有有效 process_id 的实例（toWriteInstances）
	if len(toWriteInstances) > 0 {
		if err := s.dao.ProcessInstance().BatchCreateWithTx(kit, tx, toWriteInstances); err != nil {
			return fmt.Errorf("insert process instances failed: %w", err)
		}
	}

	return nil
}
