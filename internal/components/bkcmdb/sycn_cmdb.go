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

// Package bkcmdb provides bkcmdb client.
package bkcmdb

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

// BizCMDB 业务
type BizCMDB struct {
	Svc   Service
	BizID int
}

// Set 集群
type Set struct {
	ID     int
	Name   string
	Module []Module
}

// Module 模块
type Module struct {
	ID      int
	Name    string
	Host    []Host
	SvcInst []SvcInst
}

// Host 主机
type Host struct {
	ID   int
	Name string
	IP   string
}

// SvcInst 服务实例
type SvcInst struct {
	ID       int
	Name     string
	ProcInst []ProcInst
}

// ProcInst 进程实例
type ProcInst struct {
	ID      int
	HostID  int
	Name    string
	ProcNum int
	table.ProcessInfo
}

// Bizs 业务
type Bizs map[int][]Set

// SyncSingleBiz 单个业务同步
// nolint: funlen
func (s *BizCMDB) SyncSingleBiz(ctx context.Context) ([]Set, error) {
	// 1. 获取集群
	listSets, err := s.fetchAllSets(ctx)
	if err != nil {
		return nil, err
	}

	var sets []Set
	for _, set := range listSets {
		sets = append(sets, Set{ID: set.BkSetID, Name: set.BkSetName})
	}

	// 2. 模块
	for i := range listSets {
		listModules, errM := s.fetchAllModules(ctx, sets[i].ID)
		if errM != nil {
			return nil, errM
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
		return nil, fmt.Errorf("fetchAllHostsBySetTemplate failed: %v", err)
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
		return nil, fmt.Errorf("fetchAllServiceInstances failed: %v", err)
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
		processInstanceList, err := s.Svc.ListProcessInstance(ctx, ListProcessInstanceReq{
			BkBizID: s.BizID, ServiceInstanceID: inst.ID,
		})
		if err != nil {
			return nil, err
		}

		var procs []ListProcessInstance
		if err := processInstanceList.Decode(&procs); err != nil {
			return nil, err
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

	return sets, nil
}

// pageFetcher 封装分页逻辑的通用函数
func pageFetcher[T any](fetch func(page *PageParam) ([]T, int, error)) ([]T, error) {
	var (
		start = 0
		limit = 30
		all   []T
		total = 0
	)

	for {
		page := &PageParam{
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

func (s *BizCMDB) fetchAllSets(ctx context.Context) ([]SetInfo, error) {
	return pageFetcher(func(page *PageParam) ([]SetInfo, int, error) {
		resp, err := s.Svc.SearchSet(ctx, SearchSetReq{
			BkSupplierAccount: "0",
			BkBizID:           s.BizID,
			Fields:            []string{"bk_biz_id", "bk_set_id", "bk_set_name", "bk_set_env", "set_template_id"},
			Page:              page,
		})
		if err != nil {
			return nil, 0, err
		}
		var result Sets
		if err := resp.Decode(&result); err != nil {
			return nil, 0, err
		}
		return result.Info, result.Count, nil
	})
}

func (s *BizCMDB) fetchAllModules(ctx context.Context, setID int) ([]ModuleInfo, error) {
	return pageFetcher(func(page *PageParam) ([]ModuleInfo, int, error) {
		resp, err := s.Svc.SearchModule(ctx, SearchModuleReq{
			BkSupplierAccount: "0",
			BkBizID:           s.BizID,
			BkSetID:           setID,
		})
		if err != nil {
			return nil, 0, err
		}
		var result ModuleListResp
		if err := resp.Decode(&result); err != nil {
			return nil, 0, err
		}
		return result.Info, result.Count, nil
	})
}

func (s *BizCMDB) fetchAllHostsBySetTemplate(ctx context.Context, setTemplateIDs []int) ([]HostInfo, error) {
	var all []HostInfo

	for _, id := range setTemplateIDs {
		hosts, err := pageFetcher(func(page *PageParam) ([]HostInfo, int, error) {
			resp, err := s.Svc.FindHostBySetTemplate(ctx, FindHostBySetTemplateReq{
				BkBizID:          s.BizID,
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

			var result HostListResp
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

func (s *BizCMDB) fetchAllServiceInstances(ctx context.Context, moduleID []int) ([]ServiceInstanceInfo, error) {
	var all []ServiceInstanceInfo

	for _, id := range moduleID {
		svcInst, err := pageFetcher(func(page *PageParam) ([]ServiceInstanceInfo, int, error) {
			resp, err := s.Svc.ListServiceInstance(ctx, ServiceInstanceListReq{
				BkBizID:    s.BizID,
				BkModuleID: id,
				Page:       page,
			})
			if err != nil {
				return nil, 0, err
			}
			var result ServiceInstanceResp
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

// BuildProcessAndInstance 处理进程和实例数据
func BuildProcessAndInstance(bizs Bizs) ([]*table.Process, []*table.ProcessInstance) {
	now := time.Now()

	var (
		processBatch         []*table.Process
		processInstanceBatch []*table.ProcessInstance
	)

	for bizID, sets := range bizs {
		for _, set := range sets {
			// 无模块
			if len(set.Module) == 0 {
				processBatch = append(processBatch, newProcess(bizID, set.Name, "", "", "", "", now))
				continue
			}

			for _, mod := range set.Module {
				// 构建 HostID -> IP 映射
				hostMap := make(map[int]string, len(mod.Host))
				for _, h := range mod.Host {
					hostMap[h.ID] = h.IP
				}

				// 无服务实例
				if len(mod.SvcInst) == 0 {
					processBatch = append(processBatch, newProcess(bizID, set.Name, mod.Name, "", "", "", now))
					continue
				}

				for _, svc := range mod.SvcInst {
					// 无进程
					if len(svc.ProcInst) == 0 {
						processBatch = append(processBatch, newProcess(bizID, set.Name, mod.Name, svc.Name, "", "", now))
						continue
					}

					for _, proc := range svc.ProcInst {
						ip := hostMap[proc.HostID]
						sourceData, _ := proc.ProcessInfo.Value()

						p := newProcess(bizID, set.Name, mod.Name, svc.Name, proc.Name, ip, now)
						p.Attachment.CcProcessID = uint32(proc.ID)
						p.Spec.CcSyncStatus = "synced"
						p.Spec.CcSyncUpdatedAt = now
						p.Spec.PrevSourceData = sourceData
						p.Spec.SourceData = sourceData

						processBatch = append(processBatch, p)

						// 创建 ProcessInstance 记录（先不填 ProcessID）
						instance := &table.ProcessInstance{
							Attachment: &table.ProcessInstanceAttachment{
								TenantID:    "default",
								BizID:       uint32(bizID),
								CcProcessID: uint32(proc.ID),
							},
							Spec: &table.ProcessInstanceSpec{
								StatusUpdatedAt: now,
							},
							Revision: &table.Revision{
								CreatedAt: now,
								UpdatedAt: now,
							},
						}
						processInstanceBatch = append(processInstanceBatch, instance)
					}
				}
			}
		}
	}

	return processBatch, processInstanceBatch
}

// newProcess 创建统一的 Process 结构
func newProcess(bizID int, setName, modName, svcName, alias, ip string, now time.Time) *table.Process {
	return &table.Process{
		Attachment: &table.ProcessAttachment{
			TenantID: "default",
			BizID:    uint32(bizID),
		},
		Spec: &table.ProcessSpec{
			SetName:         setName,
			ModuleName:      modName,
			ServiceName:     svcName,
			Alias:           alias,
			InnerIP:         ip,
			CcSyncUpdatedAt: now,
			PrevSourceData:  "{}",
			SourceData:      "{}",
		},
		Revision: &table.Revision{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}
