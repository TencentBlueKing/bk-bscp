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

package service

import (
	"context"
	"fmt"
	"path"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/internal/processor/cmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbct "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/config-template"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// ListConfigTemplate implements pbds.DataServer.
func (s *Service) ListConfigTemplate(ctx context.Context, req *pbds.ListConfigTemplateReq) (
	*pbds.ListConfigTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	// 1. 根据业务查询模板空间和模板套餐下的模板配置
	spec, err := s.dao.TemplateSpace().GetBizTemplateSpaceByName(grpcKit, req.GetBizId(), constant.CONFIG_DELIVERY)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &pbds.ListConfigTemplateResp{}, nil
		}
		return nil, err
	}

	// 2. 根据模板空间查询模板
	templates, count, err := s.dao.Template().List(grpcKit, req.GetBizId(), spec.ID,
		&types.BasePage{
			All:   req.GetAll(),
			Start: req.GetStart(),
			Limit: uint(req.GetLimit()),
		})
	if err != nil {
		return nil, err
	}

	templateIDs := []uint32{}
	fileNames := make(map[uint32]string)
	for _, template := range templates {
		templateIDs = append(templateIDs, template.ID)
		fileNames[template.ID] = path.Join(template.Spec.Path, template.Spec.Name)
	}

	// 3. 根据模板ID查询配置模板列表
	configTemplates, err := s.dao.ConfigTemplate().ListAllByTemplateIDs(grpcKit, req.GetBizId(), templateIDs)
	if err != nil {
		return nil, err
	}

	return &pbds.ListConfigTemplateResp{
		Count:   uint32(count),
		Details: pbct.PbConfigTemplates(configTemplates, fileNames),
	}, nil
}

// BizTopo implements pbds.DataServer.
func (s *Service) BizTopo(ctx context.Context, req *pbds.BizTopoReq) (*pbds.BizTopoResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)
	// 1. 查询业务实例拓扑 search_biz_inst_topo
	topo, err := s.cmdb.SearchBizInstTopo(grpcKit.Ctx, &bkcmdb.BizTopoReq{
		BkBizID: int(req.GetBizId()),
	})
	if err != nil {
		return nil, err
	}

	// 2. 转换为 pb 结构
	pbTopo := pbct.ConvertBizTopoNodes(topo)

	// 3. 获取模板ID并回填到拓扑树中
	err = s.fillServiceTemplateIDToTopo(grpcKit.Ctx, pbTopo, int(req.GetBizId()))
	if err != nil {
		return nil, err
	}

	// 4. moduleID → hostIDs
	hostMap, err := s.getModuleHostIDsMap(grpcKit.Ctx, int(req.GetBizId()))
	if err != nil {
		return nil, err
	}

	// 5. moduleID → processIDs
	procMap, err := s.getModuleProcessIDsMap(grpcKit, int(req.GetBizId()))
	if err != nil {
		return nil, err
	}

	// 6. 回填主机数、进程数到拓扑树中
	fillCountsToTopoNodes(pbTopo, []map[int][]int{hostMap, procMap}, []string{"host_count", "process_count"})

	return &pbds.BizTopoResp{
		BizTopoNodes: pbTopo,
	}, nil
}

// ServiceTemplate implements pbds.DataServer.
func (s *Service) ServiceTemplate(ctx context.Context, req *pbds.ServiceTemplateReq) (*pbds.ServiceTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	resp, err := s.fetchAllServiceTemplate(grpcKit.Ctx, int(req.GetBizId()))
	if err != nil {
		return nil, err
	}

	return &pbds.ServiceTemplateResp{
		ServiceTemplates: pbct.ConvertServiceTemplates(resp),
	}, nil
}

// ProcessTemplate implements pbds.DataServer.
func (s *Service) ProcessTemplate(ctx context.Context, req *pbds.ProcessTemplateReq) (*pbds.ProcessTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	resp, err := s.cmdb.ListProcTemplate(grpcKit.Ctx, &bkcmdb.ListProcTemplateReq{
		BkBizID:           int(req.GetBizId()),
		ServiceTemplateID: int(req.GetServiceTemplateId()),
	})
	if err != nil {
		return nil, err
	}

	return &pbds.ProcessTemplateResp{
		ProcessTemplates: pbct.ConvertProcTemplates(resp.Info),
	}, nil
}

// fillServiceTemplateIDToTopo 在业务拓扑树中回填 service_template_id
func (s *Service) fillServiceTemplateIDToTopo(ctx context.Context, topo []*pbct.BizTopoNode, bizID int) error {

	// 1. 获取所有 module 节点
	modules := listTargetObjNodeFromTopo(topo, constant.BK_MODULE_OBJ_ID)

	if len(modules) == 0 {
		return nil
	}

	// 2. 提取 module ID 列表（去重）
	modIDSet := make(map[int]struct{})
	for _, m := range modules {
		modIDSet[int(m.BkInstId)] = struct{}{}
	}
	modIDs := make([]int, 0, len(modIDSet))
	for id := range modIDSet {
		modIDs = append(modIDs, id)
	}

	// 3. 批量查询模块详情 find_module_batch
	modDetailResp, err := s.fetchAllModuleDetails(ctx, bizID, modIDs)
	if err != nil {
		return fmt.Errorf("find module batch failed: %w", err)
	}

	// // 4. 构建 map[module_id]service_template_id
	modIDToServTplID := make(map[int]int)
	for _, md := range modDetailResp {
		modIDToServTplID[md.BkModuleID] = md.ServiceTemplateID
	}

	// 5. 回填 service_template_id 到拓扑树中
	fillServiceTemplateIDs(topo, modIDToServTplID)

	return nil
}

// fillServiceTemplateIDs 回填 service_template_id 到拓扑树中
func fillServiceTemplateIDs(topo []*pbct.BizTopoNode, modIDToServTplID map[int]int) {
	for _, biz := range topo {
		for _, set := range biz.Child {
			for _, module := range set.Child {
				if module.BkObjId == constant.BK_MODULE_OBJ_ID {
					if tplID, ok := modIDToServTplID[int(module.BkInstId)]; ok {
						module.ServiceTemplateId = uint32(tplID)
					}
				}
			}
		}
	}
}

// listTargetObjNodeFromTopo 从 topo 树中提取指定类型的节点，例如 module
func listTargetObjNodeFromTopo(topo []*pbct.BizTopoNode, targetObj string) []*pbct.BizTopoNode {
	modules := make([]*pbct.BizTopoNode, 0)

	for _, biz := range topo {
		for _, set := range biz.Child {
			for _, module := range set.Child {
				if module.BkObjId == targetObj {
					modules = append(modules, module)
				}
			}
		}
	}

	return modules
}

func (s *Service) getModuleHostIDsMap(ctx context.Context, bizID int) (map[int][]int, error) {
	moduleHostMap := make(map[int][]int)
	// 批量查询主机-模块关系
	hosts, err := s.fetchAllHost(ctx, bizID)
	if err != nil {
		return nil, fmt.Errorf("find host topo relation failed: %w", err)
	}

	if len(hosts) == 0 {
		return moduleHostMap, nil
	}

	for _, rel := range hosts {
		moduleID := rel.BkModuleID
		hostID := rel.BkHostID
		moduleHostMap[moduleID] = append(moduleHostMap[moduleID], hostID)
	}
	return moduleHostMap, nil
}

func (s *Service) getModuleProcessIDsMap(kit *kit.Kit, bizID int) (map[int][]int, error) {
	procList, err := s.dao.Process().ListActiveProcesses(kit, uint32(bizID))
	if err != nil {
		return nil, fmt.Errorf("ListProcessByBizID failed: %w", err)
	}

	moduleProcMap := make(map[int][]int)
	for _, p := range procList {
		moduleProcMap[int(p.Attachment.ModuleID)] = append(moduleProcMap[int(p.Attachment.ModuleID)],
			int(p.Attachment.CcProcessID))
	}
	return moduleProcMap, nil
}

// fillCountsToTopoNodes 在拓扑树中回填指定字段的计数信息（如主机数、进程数等）
func fillCountsToTopoNodes(nodes []*pbct.BizTopoNode, maps []map[int][]int, fields []string) {

	for _, biz := range nodes {
		for _, set := range biz.Child {
			for _, module := range set.Child {
				if module.BkObjId != constant.BK_MODULE_OBJ_ID {
					continue
				}

				moduleID := int(module.BkInstId)

				for i, mp := range maps {
					field := fields[i]

					if ids, ok := mp[moduleID]; ok {
						count := len(ids)

						switch field {
						case "host_count":
							module.HostCount = uint32(count)
						case "process_count":
							module.ProcessCount = uint32(count)
						}
					}
				}
			}
		}
	}
}

// fetchAllModuleDetails 由于cmdb限制一次查询模块详情的数量，故此处做批量查询
func (s *Service) fetchAllModuleDetails(ctx context.Context, bizID int, modIDs []int) ([]*bkcmdb.ModuleInfo, error) {
	return batchSliceFetcher(modIDs, 500,
		func(batch []int) ([]*bkcmdb.ModuleInfo, error) {

			resp, err := s.cmdb.FindModuleBatch(ctx, &bkcmdb.ModuleReq{
				BkBizID: bizID,
				BkIDs:   batch,
				Fields:  []string{"service_template_id", "bk_module_id"},
			})
			if err != nil {
				return nil, err
			}

			return resp, nil
		},
	)
}

// batchSliceFetcher 批量切片查询通用函数
func batchSliceFetcher[T any, R any](ids []T, batchSize int, fetch func(batch []T) ([]R, error)) ([]R, error) {
	var all []R
	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batchIDs := ids[start:end]

		data, err := fetch(batchIDs)
		if err != nil {
			return nil, err
		}

		all = append(all, data...)
	}

	return all, nil
}

// fetchAllHost 由于cmdb限制一次查询主机详情的数量，故此处做批量查询
func (s *Service) fetchAllHost(ctx context.Context, bizID int) ([]*bkcmdb.HostTopoInfo, error) {
	return cmdb.PageFetcher(func(page *bkcmdb.PageParam) ([]*bkcmdb.HostTopoInfo, int, error) {
		resp, err := s.cmdb.FindHostTopoRelation(ctx, &bkcmdb.HostTopoReq{
			BkBizID: bizID,
			Page:    page,
		})
		if err != nil {
			return nil, 0, err
		}

		return resp.Data, resp.Count, nil
	})
}

// fetchAllServiceTemplate 由于cmdb限制一次查询服务模板的数量，故此处做批量查询
func (s *Service) fetchAllServiceTemplate(ctx context.Context, bizID int) ([]*bkcmdb.ServiceTemplate, error) {
	return cmdb.PageFetcher(func(page *bkcmdb.PageParam) ([]*bkcmdb.ServiceTemplate, int, error) {
		resp, err := s.cmdb.ListServiceTemplate(ctx, &bkcmdb.ListServiceTemplateReq{
			BkBizID: bizID,
			Page:    page,
		})
		if err != nil {
			return nil, 0, err
		}

		return resp.Info, resp.Count, nil
	})
}
