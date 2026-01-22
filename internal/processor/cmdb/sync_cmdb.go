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
		procs, err := s.svc.ListProcessInstance(ctx, &bkcmdb.ListProcessInstanceReq{
			BkBizID: s.bizID, ServiceInstanceID: inst.ID,
		})
		if err != nil {
			return fmt.Errorf("fetch all process instances failed: %v", err)
		}

		for _, proc := range procs {
			listProcMap[inst.ID] = append(listProcMap[inst.ID], ProcInst{
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

	processBatch := buildProcess(bizs)

	// 开启事务并入库
	tx := s.dao.GenQuery().Begin()

	if err := s.syncProcessData(kit, tx, processBatch); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			logs.Errorf("[syncCMDB][ERROR] rollback failed for bizID=%d: %v", s.bizID, rbErr)
		}
		return fmt.Errorf("sync process and instance data failed for biz %d: %v", s.bizID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed for biz %d: %v", s.bizID, err)
	}

	logs.Infof("[syncCMDB][INFO] bizID=%d process data synced, %d processes written", s.bizID, len(processBatch))

	return nil
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
			Fields:            []string{"bk_module_id", "bk_module_name", "service_template_id"},
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

// buildProcess 生成进程数据
func buildProcess(bizs Bizs) []*table.Process {
	now := time.Now()

	var processBatch []*table.Process

	for bizID, sets := range bizs {
		for _, set := range sets {
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
							log.Printf("[syncCMDB][WARN] bizID=%d, set=%s, module=%s, svc=%s, proc=%s, hostID=%d: not found in hostMap",
								bizID, set.Name, mod.Name, svc.Name, proc.Name, proc.HostID)
							continue
						}
						sourceData, err := proc.ProcessInfo.Value()
						if err != nil {
							log.Printf("[syncCMDB][ERROR] bizID=%d, set=%s, module=%s, svc=%s, proc=%s, hostID=%d: failed to get process info: %v",
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
								ProcessTemplateID: uint32(proc.ProcessTemplateID),
								ServiceTemplateID: uint32(mod.ServiceTemplateID),
							},
							Spec: &table.ProcessSpec{
								SetName:      set.Name,
								ModuleName:   mod.Name,
								ServiceName:  svc.Name,
								Environment:  set.SetEnv,
								Alias:        proc.Name,
								InnerIP:      hinfo.IP,
								CcSyncStatus: table.Synced,
								SourceData:   sourceData,
								PrevData:     "{}",
								ProcNum:      uint(proc.ProcNum),
								FuncName:     proc.FuncName,
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
	}

	return processBatch
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

// diffProcesses 比较当前进程列表和cc进程列表的差异
func (s *syncCMDBService) diffProcesses(kit *kit.Kit, tx *gen.QueryTx, dbProcesses []*table.Process,
	newProcesses []*table.Process) (*ProcessDiff, error) {

	now := time.Now().UTC()

	diff := &ProcessDiff{}

	//  构建索引
	dbMap := make(map[uint32]*table.Process)
	for _, p := range dbProcesses {
		dbMap[p.Attachment.CcProcessID] = p
	}

	newMap := make(map[uint32]*table.Process)
	for _, p := range newProcesses {
		newMap[p.Attachment.CcProcessID] = p
	}

	// 用于实例序列号缓存
	hostCounter := make(map[[2]int]int)
	moduleCounter := make(map[[2]int]int)

	// 遍历 newProcesses
	for _, newP := range newProcesses {
		oldP, exists := dbMap[newP.Attachment.CcProcessID]

		// 新增进程
		if !exists {
			newP.Revision = &table.Revision{CreatedAt: now}
			diff.ToAddProcesses = append(diff.ToAddProcesses, newP)

			insts := buildInstances(
				int(newP.Attachment.BizID),
				int(newP.Attachment.HostID),
				int(newP.Attachment.ModuleID),
				int(newP.Attachment.CcProcessID),
				int(newP.Spec.ProcNum),
				0, 0, 0,
				now, hostCounter, moduleCounter,
			)
			diff.ToAddInstances = append(diff.ToAddInstances, insts...)
			continue
		}

		addP, updateP, delPID, addInsts, delInstIDs, err := BuildProcessChanges(kit, s.dao, tx, newP, oldP, now,
			hostCounter, moduleCounter)
		if err != nil {
			return nil, err
		}

		if addP != nil {
			diff.ToAddProcesses = append(diff.ToAddProcesses, addP)
		}
		if updateP != nil {
			diff.ToUpdateProcesses = append(diff.ToUpdateProcesses, updateP)
		}
		if delPID != 0 {
			diff.ToDeleteProcessIDs = append(diff.ToDeleteProcessIDs, delPID)
		}
		diff.ToAddInstances = append(diff.ToAddInstances, addInsts...)
		diff.ToDeleteInstanceIDs = append(diff.ToDeleteInstanceIDs, delInstIDs...)
	}

	// db 中有，但 new 中没有 → 进程删除
	for _, oldP := range dbProcesses {
		if _, ok := newMap[oldP.Attachment.CcProcessID]; !ok {
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

func reconcileProcessInstances(kit *kit.Kit, dao dao.Set, tx *gen.QueryTx, bizID, processID, hostID, moduleID, ccProcessID uint32,
	oldNum, newNum int, now time.Time, hostCounter map[[2]int]int, moduleCounter map[[2]int]int) (*InstanceReconcileResult, error) {

	res := &InstanceReconcileResult{}

	// 数量一致不做处理
	if newNum == oldNum {
		return res, nil
	}

	// 缩容
	if newNum < oldNum {
		needDelete := oldNum - newNum
		insts, err := dao.ProcessInstance().ListByProcessIDOrderBySeqDescTx(kit, tx, bizID, processID, needDelete)
		if err != nil {
			return nil, err
		}

		for _, inst := range insts {
			res.ToDelete = append(res.ToDelete, inst.ID)
		}

		return res, nil
	}

	// 扩容
	maxModuleSeq, err := dao.ProcessInstance().GetMaxModuleInstSeqTx(kit, tx, bizID, []uint32{processID})
	if err != nil {
		return nil, err
	}

	maxHostSeq, err := dao.ProcessInstance().GetMaxHostInstSeqTx(kit, tx, bizID, []uint32{processID})
	if err != nil {
		return nil, err
	}

	res.ToAdd = buildInstances(
		int(bizID),
		int(hostID),
		int(moduleID),
		int(ccProcessID),
		newNum,
		oldNum,
		maxModuleSeq,
		maxHostSeq,
		now,
		hostCounter,
		moduleCounter,
	)

	return res, nil
}

// BuildProcessChanges 生成进程及其实例的新增、更新或删除操
func BuildProcessChanges(kit *kit.Kit, dao dao.Set, tx *gen.QueryTx, newP *table.Process, oldP *table.Process, now time.Time,
	hostCounter map[[2]int]int, moduleCounter map[[2]int]int) (*table.Process, *table.Process, uint32,
	[]*table.ProcessInstance, []uint32, error) {

	equal, err := compareProcessInfo(newP.Spec.SourceData, oldP.Spec.SourceData)
	if err != nil {
		return nil, nil, 0, nil, nil, err
	}

	nameChanged := newP.Spec.Alias != oldP.Spec.Alias
	infoChanged := !equal
	numChanged := newP.Spec.ProcNum != oldP.Spec.ProcNum

	if !nameChanged && !infoChanged && !numChanged {
		return nil, nil, 0, nil, nil, nil
	}

	// 是否安全
	safe, err := isSafeToUpdateProcess(
		kit, dao, tx,
		oldP.Attachment.BizID,
		oldP.ID,
	)
	if err != nil {
		return nil, nil, 0, nil, nil, err
	}

	newProcNum := int(newP.Spec.ProcNum)

	// 1. 别名变更 + 不安全：删除旧进程，生成新进程
	if nameChanged && !safe {
		newP.Spec.PrevData = oldP.Spec.SourceData
		newP.Spec.CcSyncStatus = table.Synced

		toAdd := &table.Process{
			Attachment: newP.Attachment,
			Spec:       newP.Spec,
			Revision:   &table.Revision{CreatedAt: now},
		}

		insts := buildInstances(
			int(toAdd.Attachment.BizID),
			int(toAdd.Attachment.HostID),
			int(toAdd.Attachment.ModuleID),
			int(toAdd.Attachment.CcProcessID),
			newProcNum,
			0, 0, 0,
			now, hostCounter, moduleCounter,
		)

		return toAdd, nil, oldP.ID, insts, nil, nil
	}

	// 2. 原地更新进程元数据
	if nameChanged {
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
	if numChanged {
		oldP.Spec.ProcNum = newP.Spec.ProcNum
	}

	toUpdate := &table.Process{
		ID:         oldP.ID,
		Attachment: oldP.Attachment,
		Spec:       oldP.Spec,
		Revision:   &table.Revision{UpdatedAt: now},
	}

	// 3. 实例调整逻辑
	insts := make([]*table.ProcessInstance, 0)
	deleteInstanceIDs := []uint32{}
	// 实例只在 安全且扩容 时调整
	if numChanged {
		// 真实实例数
		allInsts, err := dao.ProcessInstance().ListByProcessIDTx(kit, tx, oldP.Attachment.BizID, oldP.ID)
		if err != nil {
			return nil, nil, 0, nil, nil, err
		}

		res, err := reconcileProcessInstances(
			kit, dao, tx,
			oldP.Attachment.BizID,
			oldP.ID,
			oldP.Attachment.HostID,
			oldP.Attachment.ModuleID,
			oldP.Attachment.CcProcessID,
			len(allInsts),
			newProcNum, // 用新值
			now,
			hostCounter,
			moduleCounter,
		)
		if err != nil {
			return nil, nil, 0, nil, nil, err
		}

		insts = res.ToAdd
		deleteInstanceIDs = res.ToDelete

		// 不安全场景：不允许缩容
		if newProcNum < len(allInsts) && !safe {
			deleteInstanceIDs = nil
		}
	}

	return nil, toUpdate, 0, insts, deleteInstanceIDs, nil
}

// buildInstances 根据进程数量生成进程实例
func buildInstances(bizID, hostID, modID, ccProcessID, procNum, existCount, maxModuleInstSeq, maxHostInstSeq int, now time.Time,
	hostCounter map[[2]int]int, moduleCounter map[[2]int]int) []*table.ProcessInstance {

	// 如果新的进程数量 <= 已存在数量，则无需新增实例
	if procNum <= existCount {
		return nil
	}

	// 需要新增的实例数量
	newCount := procNum - existCount
	if newCount <= 0 {
		return nil
	}

	instances := make([]*table.ProcessInstance, 0, newCount)

	// 维度 key： (ccProcessID, hostID) 和 (ccProcessID, modID)
	hostKey := [2]int{ccProcessID, hostID}
	modKey := [2]int{ccProcessID, modID}

	// 从缓存取
	startHostInstSeq := hostCounter[hostKey]
	startModuleInstSeq := moduleCounter[modKey]

	// 如果缓存未初始化，则从数据库最大值开始
	if startHostInstSeq == 0 {
		startHostInstSeq = maxHostInstSeq
		hostCounter[hostKey] = startHostInstSeq
	}
	if startModuleInstSeq == 0 {
		startModuleInstSeq = maxModuleInstSeq
		moduleCounter[modKey] = startModuleInstSeq
	}

	for i := 1; i <= newCount; i++ {
		hostCounter[hostKey]++
		moduleCounter[modKey]++

		hostInstSeq := hostCounter[hostKey]
		moduleInstSeq := moduleCounter[modKey]

		instances = append(instances, &table.ProcessInstance{
			Attachment: &table.ProcessInstanceAttachment{
				TenantID:    "default",
				BizID:       uint32(bizID),
				CcProcessID: uint32(ccProcessID),
			},
			Spec: &table.ProcessInstanceSpec{
				StatusUpdatedAt: now,
				HostInstSeq:     uint32(hostInstSeq),
				ModuleInstSeq:   uint32(moduleInstSeq),
			},
			Revision: &table.Revision{
				CreatedAt: now,
				UpdatedAt: now,
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

	// side effect
	NeedSyncGSE bool
}

func (s *syncCMDBService) syncProcessData(kit *kit.Kit, tx *gen.QueryTx, processBatch []*table.Process) error {

	if len(processBatch) == 0 {
		return nil
	}

	tenantID := processBatch[0].Attachment.TenantID
	bizID := processBatch[0].Attachment.BizID

	oldProcesses, err := s.dao.Process().ListProcByBizIDWithTx(kit, tx, tenantID, bizID)
	if err != nil {
		return err
	}

	diff, err := s.diffProcesses(kit, tx, oldProcesses, processBatch)
	if err != nil {
		return err
	}

	// 1. 先删实例（缩容）
	if len(diff.ToDeleteInstanceIDs) > 0 {
		if err := s.dao.ProcessInstance().BatchDeleteByIDsWithTx(kit, tx, diff.ToDeleteInstanceIDs); err != nil {
			return err
		}
	}

	// 2. 删进程（及兜底实例）
	if len(diff.ToDeleteProcessIDs) > 0 {
		if err := DeleteInstanceStoppedUnmanaged(
			kit, s.dao, tx, bizID, diff.ToDeleteProcessIDs,
		); err != nil {
			return err
		}
	}

	// 3. 新增进程
	if len(diff.ToAddProcesses) > 0 {
		if err := s.dao.Process().BatchCreateWithTx(kit, tx, diff.ToAddProcesses); err != nil {
			return err
		}
	}

	// 4. 更新进程
	if len(diff.ToUpdateProcesses) > 0 {
		if err := s.dao.Process().
			BatchUpdateWithTx(kit, tx, diff.ToUpdateProcesses); err != nil {
			return err
		}
	}

	// 5. 回填 ProcessID 给实例
	idMap := make(map[string]uint32)
	for _, p := range diff.ToAddProcesses {
		key := fmt.Sprintf("%s-%d-%d",
			p.Attachment.TenantID, bizID, p.Attachment.CcProcessID)
		idMap[key] = p.ID
	}
	for _, p := range diff.ToUpdateProcesses {
		key := fmt.Sprintf("%s-%d-%d",
			p.Attachment.TenantID, bizID, p.Attachment.CcProcessID)
		idMap[key] = p.ID
	}

	for _, inst := range diff.ToAddInstances {
		key := fmt.Sprintf("%s-%d-%d",
			inst.Attachment.TenantID, bizID, inst.Attachment.CcProcessID)
		if pid := idMap[key]; pid != 0 {
			inst.Attachment.ProcessID = pid
		}
	}

	// 6. 新增实例（扩容 / 新进程）
	if len(diff.ToAddInstances) > 0 {
		if err := s.dao.ProcessInstance().BatchCreateWithTx(kit, tx, diff.ToAddInstances); err != nil {
			return err
		}
	}

	return nil
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
