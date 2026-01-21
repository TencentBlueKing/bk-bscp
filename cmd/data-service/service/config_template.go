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
	"errors"
	"fmt"
	"path"
	"reflect"
	"time"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/internal/processor/cmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbct "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/config-template"
	pbproc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// ListConfigTemplate implements pbds.DataServer.
func (s *Service) ListConfigTemplate(ctx context.Context, req *pbds.ListConfigTemplateReq) (
	*pbds.ListConfigTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	now := time.Now().UTC()
	// 1. 确保 config_delivery 模板空间存在
	templateSpace, err := s.getOrCreateTemplateSpace(grpcKit, req.GetBizId(), now)
	if err != nil {
		logs.Errorf("getOrCreateTemplateSpace failed, err=%v, rid=%s", err, grpcKit.Rid)
		return nil, err
	}

	// 3. 确保默认套餐存在
	templateSet, err := s.getOrCreateDefaultTemplateSet(grpcKit, req.GetBizId(), templateSpace.ID, now)
	if err != nil {
		logs.Errorf("[ConfigTemplate] getOrCreateDefaultTemplateSet failed, err=%v, rid=%s", err, grpcKit.Rid)
		return nil, err
	}

	if templateSpace == nil || templateSet == nil {
		return nil, fmt.Errorf("No available space or packages")
	}

	resp := &pbds.ListConfigTemplateResp{
		TemplateSpace: &pbds.ListConfigTemplateResp_Item{
			Id:   templateSpace.ID,
			Name: templateSpace.Spec.Name,
		},
		TemplateSet: &pbds.ListConfigTemplateResp_Item{
			Id:   templateSet.ID,
			Name: templateSet.Spec.Name,
		},
	}

	// 3. 根据业务查询配置模板
	configTemplates, count, err := s.dao.ConfigTemplate().List(grpcKit, req.GetBizId(), templateSpace.ID, req.GetSearch(),
		&types.BasePage{
			Start: req.Start,
			Limit: uint(req.Limit),
			All:   req.GetAll(),
		})
	if err != nil {
		return nil, err
	}

	if count == 0 {
		return resp, nil
	}

	idSet := make(map[uint32]struct{}, len(configTemplates))
	configTemplateIDs := []uint32{}
	templateIDs := make([]uint32, 0, len(configTemplates))
	for _, v := range configTemplates {
		id := v.Attachment.TemplateID
		if _, ok := idSet[id]; ok {
			continue
		}
		idSet[id] = struct{}{}
		templateIDs = append(templateIDs, id)
		configTemplateIDs = append(configTemplateIDs, v.ID)
	}

	// 获取配置实例
	ci, er := s.dao.ConfigInstance().ListConfigInstancesByTemplateID(grpcKit, req.GetBizId(), configTemplateIDs)
	if er != nil {
		return nil, er
	}
	// 用于标记配置模板是否已下发过配置实例
	releasedMap := make(map[uint32]bool, len(configTemplateIDs))
	// 先默认全部为 false（未下发）
	for _, id := range configTemplateIDs {
		releasedMap[id] = false
	}
	// 如果存在配置实例，则标记为 true（已下发）
	for _, inst := range ci {
		releasedMap[inst.Attachment.ConfigTemplateID] = true
	}

	templates, err := s.dao.Template().ListByIDs(grpcKit, templateIDs)
	if err != nil {
		return nil, err
	}

	fileNames := make(map[uint32]string, len(templates))
	for _, template := range templates {
		fileNames[template.ID] = path.Join(template.Spec.Path, template.Spec.Name)
	}

	resp.Count = uint32(count)
	resp.Details = pbct.PbConfigTemplates(configTemplates, fileNames, releasedMap)

	return resp, nil
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

	// 3. 去掉业务层，只保留 set 层
	// pbTopo[0] 为 biz，其子节点即 set 层
	if len(pbTopo) == 0 || pbTopo[0].BkObjId != constant.BK_BIZ_OBJ_ID {
		return nil, fmt.Errorf("unexpected biz topo format")
	}
	setTopo := pbTopo[0].Child

	// 4. 获取模板ID并回填到拓扑树中
	err = s.fillServiceTemplateIDToTopo(grpcKit.Ctx, setTopo, int(req.GetBizId()))
	if err != nil {
		return nil, err
	}

	// 5. 查询表获取进程数并回填到拓扑树中
	if err := s.fillProcessCount(grpcKit, int(req.GetBizId()), setTopo); err != nil {
		return nil, err
	}

	return &pbds.BizTopoResp{
		BizTopoNodes: setTopo,
	}, nil
}

func (s *Service) fillProcessCount(kit *kit.Kit, bizID int, topo []*pbct.BizTopoNode) error {
	for _, set := range topo {
		var setTotal uint32
		for _, module := range set.Child {
			if module.BkObjId != constant.BK_MODULE_OBJ_ID {
				continue
			}
			var (
				cnt int64
				err error
			)

			// 情况 1：模板模块（使用服务模板统计）
			if module.ServiceTemplateId != 0 {
				cnt, err = s.dao.Process().ProcessCountByServiceTemplate(kit, uint32(bizID), module.ServiceTemplateId)
				if err != nil {
					return err
				}
				module.ProcessCount = uint32(cnt)
				setTotal += uint32(cnt)
				continue
			}

			// 普通模块（使用 ServiceInstance → ProcessInstance）
			svcInsts, err := s.processCountByServiceInstance(kit, bizID, int(module.BkInstId))
			if err != nil {
				return err
			}

			// 累加每个 ServiceInstanceInfo.ProcessCount
			mTotal := uint32(0)
			for _, inst := range svcInsts {
				mTotal += inst.ProcessCount
			}

			module.ProcessCount = mTotal
			setTotal += mTotal
		}

		// set 层聚合模块进程数
		set.ProcessCount = setTotal
	}

	return nil
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
	for _, set := range topo {
		for _, module := range set.Child {
			if module.BkObjId == constant.BK_MODULE_OBJ_ID {
				if tplID, ok := modIDToServTplID[int(module.BkInstId)]; ok {
					module.ServiceTemplateId = uint32(tplID)
				}
			}
		}
	}
}

// listTargetObjNodeFromTopo 从 topo 树中提取指定类型的节点，例如 module
func listTargetObjNodeFromTopo(topo []*pbct.BizTopoNode, targetObj string) []*pbct.BizTopoNode {
	modules := make([]*pbct.BizTopoNode, 0)

	for _, set := range topo {
		for _, module := range set.Child {
			if module.BkObjId == targetObj {
				modules = append(modules, module)
			}
		}
	}

	return modules
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

// CreateConfigTemplate implements pbds.DataServer.
func (s *Service) CreateConfigTemplate(ctx context.Context, req *pbds.CreateConfigTemplateReq) (*pbds.CreateConfigTemplateResp, error) {
	kit := kit.FromGrpcContext(ctx)

	// 同一业务下不能出现同名的模板
	ct, err := s.dao.ConfigTemplate().GetByUniqueKey(kit, req.GetBizId(), 0, req.GetName())
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if ct != nil {
		return nil, fmt.Errorf(
			"the same template name already exists under this %d business: %s",
			req.GetBizId(),
			req.GetName(),
		)
	}

	// 1. 开启事务
	tx := s.dao.GenQuery().Begin()
	committed := false
	defer func() {
		if !committed {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kit.Rid)
			}
		}
	}()

	now := time.Now().UTC()

	// 2. 校验同一空间下不能出现相同绝对路径的配置文件且同路径下不能出现同名的文件夹和文件
	items, _, err := s.dao.Template().List(kit, req.GetBizId(),
		req.GetTemplateSpaceId(), &types.BasePage{All: true})
	if err != nil {
		return nil, err
	}
	existingPaths := []string{}
	for _, v := range items {
		existingPaths = append(existingPaths, path.Join(v.Spec.Path, v.Spec.Name))
	}

	if tools.CheckPathConflict(path.Join(req.GetFilePath(), req.GetFileName()), existingPaths) {
		return nil, errors.New(i18n.T(kit, "the config file %s already exists in this space and cannot be created again",
			path.Join(req.GetFilePath(), req.GetFileName())))
	}

	// 3. 通过空间和名称查询套餐
	templateSet, err := s.dao.TemplateSet().GetByUniqueKey(kit, req.GetBizId(), req.GetTemplateSpaceId(), constant.DefaultTmplSetName)
	if err != nil {
		return nil, err
	}

	// 4. 创建模板和模板版本，并添加至默认套餐中
	templateId, err := s.createTemplateAndRevision(kit, tx, req.GetTemplateSpaceId(), templateSet, req, now)
	if err != nil {
		logs.Errorf("[ConfigTemplate] createTemplateAndRevision failed, err=%v, rid=%s", err, kit.Rid)
		return nil, err
	}

	// 5. 创建配置模板
	configTemplate := &table.ConfigTemplate{
		Spec: &table.ConfigTemplateSpec{
			Name:           req.GetName(),
			HighlightStyle: table.HighlightStyle(req.GetHighlightStyle()),
		},
		Attachment: &table.ConfigTemplateAttachment{
			BizID:                req.GetBizId(),
			TemplateID:           templateId,
			TenantID:             kit.TenantID,
			CcTemplateProcessIDs: []uint32{},
			CcProcessIDs:         []uint32{},
		},
		Revision: &table.Revision{
			Creator:   kit.User,
			CreatedAt: now,
			Reviser:   kit.User,
			UpdatedAt: now,
		},
	}
	id, err := s.dao.ConfigTemplate().CreateWithTx(kit, tx, configTemplate)
	if err != nil {
		logs.Errorf("[ConfigTemplate] create config template failed, err: %v, rid: %s", err, kit.Rid)
		return nil, err
	}

	// 6. 提交事务
	if e := tx.Commit(); e != nil {
		logs.Errorf("[ConfigTemplate] commit transaction failed, err: %v, rid: %s", e, kit.Rid)
		return nil, e
	}
	committed = true

	return &pbds.CreateConfigTemplateResp{
		Id: id,
	}, nil
}

// createTemplateAndRevision 创建模板和模板版本以及把模板移入指定的套餐中
func (s *Service) createTemplateAndRevision(kit *kit.Kit, tx *gen.QueryTx, templateSpaceID uint32,
	templateSet *table.TemplateSet, req *pbds.CreateConfigTemplateReq, now time.Time) (uint32, error) {

	// 1. 创建模板文件
	template := &table.Template{
		Spec: &table.TemplateSpec{
			Name: req.GetFileName(),
			Path: req.GetFilePath(),
			Memo: req.GetMemo(),
		},
		Attachment: &table.TemplateAttachment{
			BizID:           req.GetBizId(),
			TemplateSpaceID: templateSpaceID,
			TenantID:        kit.TenantID,
		},
		Revision: &table.Revision{
			Creator:   kit.User,
			CreatedAt: now,
			Reviser:   kit.User,
			UpdatedAt: now,
		},
	}
	templateID, err := s.dao.Template().CreateWithTx(kit, tx, template, false)
	if err != nil {
		logs.Errorf("create template failed, err: %v, rid: %s", err, kit.Rid)
		return 0, err
	}
	// 2. 创建模板版本
	var revisionName = req.GetRevisionName()
	if revisionName == "" {
		revisionName = tools.GenerateRevisionName()
	}
	templateRevision := &table.TemplateRevision{
		Spec: &table.TemplateRevisionSpec{
			RevisionName: revisionName,
			RevisionMemo: req.GetMemo(),
			Name:         req.GetFileName(),
			Path:         req.GetFilePath(),
			FileType:     table.Text,
			FileMode:     table.FileMode(req.GetFileMode()),
			Permission: &table.FilePermission{
				User:      req.GetUser(),
				UserGroup: req.GetUserGroup(),
				Privilege: req.GetPrivilege(),
			},
			ContentSpec: &table.ContentSpec{
				Signature: req.GetSign(),
				ByteSize:  req.GetByteSize(),
				Md5:       req.GetMd5(),
			},
			Charset: table.FileCharset(req.GetCharset()),
		},
		Attachment: &table.TemplateRevisionAttachment{
			BizID:           req.GetBizId(),
			TemplateSpaceID: templateSpaceID,
			TemplateID:      templateID,
		},
		Revision: &table.CreatedRevision{
			Creator:   kit.User,
			CreatedAt: now,
		},
	}

	if _, err = s.dao.TemplateRevision().CreateWithTx(kit, tx, templateRevision, false); err != nil {
		logs.Errorf("create template revision failed, err: %v, rid: %s", err, kit.Rid)
		return 0, err
	}

	templateSet.Spec.TemplateIDs = tools.MergeAndDeduplicate(templateSet.Spec.TemplateIDs, []uint32{templateID})

	// 3. 添加至模板套餐中
	err = s.dao.TemplateSet().BatchAddTmplsToTmplSetsWithTx(kit, tx, []*table.TemplateSet{templateSet}, true)
	if err != nil {
		logs.Errorf("batch add templates to template sets failed, err: %v, rid: %s", err, kit.Rid)
		return 0, errf.Errorf(errf.DBOpFailed, i18n.T(kit, "batch add templates to template sets failed, err: %s", err))
	}

	return templateID, nil
}

// getOrCreateTemplateSpace 获取或创建 config_delivery 模板空间
func (s *Service) getOrCreateTemplateSpace(kit *kit.Kit, bizID uint32, now time.Time) (*table.TemplateSpace, error) {
	space, err := s.dao.TemplateSpace().GetBizTemplateSpaceByName(
		kit, bizID, constant.CONFIG_DELIVERY,
	)
	if err == nil {
		return space, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// create
	spec := &table.TemplateSpace{
		Spec: &table.TemplateSpaceSpec{Name: constant.CONFIG_DELIVERY},
		Attachment: &table.TemplateSpaceAttachment{
			BizID: bizID, TenantID: kit.TenantID,
		},
		Revision: &table.Revision{
			Creator: kit.User, CreatedAt: now,
			Reviser: kit.User, UpdatedAt: now,
		},
	}

	_, err = s.dao.TemplateSpace().Create(kit, spec)
	if err != nil {
		return nil, err
	}

	return spec, nil
}

// getOrCreateDefaultTemplateSet 获取或创建默认模板套餐
func (s *Service) getOrCreateDefaultTemplateSet(kit *kit.Kit, bizID, spaceID uint32, now time.Time) (
	*table.TemplateSet, error) {

	set, err := s.dao.TemplateSet().GetSetBySpaceID(kit, bizID, spaceID)
	if err == nil {
		return set, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	set = &table.TemplateSet{
		Spec: &table.TemplateSetSpec{
			Name: constant.DefaultTmplSetName,
		},
		Attachment: &table.TemplateSetAttachment{
			BizID: bizID, TemplateSpaceID: spaceID, TenantID: kit.TenantID,
		},
		Revision: &table.Revision{
			Creator: kit.User, CreatedAt: now,
			Reviser: kit.User, UpdatedAt: now,
		},
	}
	_, err = s.dao.TemplateSet().Create(kit, set)
	if err != nil {
		return nil, err
	}

	return set, nil
}

// ServiceTemplate implements pbds.DataServer.
func (s *Service) ServiceTemplate(ctx context.Context, req *pbds.ServiceTemplateReq) (*pbds.ServiceTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	resp, err := s.fetchAllServiceTemplate(grpcKit.Ctx, int(req.GetBizId()))
	if err != nil {
		return nil, err
	}

	processesCount := map[int]uint32{}
	for _, v := range resp {
		if v.ID != 0 {
			cnt, err := s.dao.Process().ProcessCountByServiceTemplate(grpcKit, req.GetBizId(), uint32(v.ID))
			if err != nil {
				return nil, err
			}
			processesCount[v.ID] = uint32(cnt)
		}
	}

	return &pbds.ServiceTemplateResp{
		ServiceTemplates: pbct.ConvertServiceTemplates(resp, processesCount),
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

// ServiceInstance 根据业务ID和模块ID查询服务实例列表
func (s *Service) ServiceInstance(ctx context.Context, req *pbds.ServiceInstanceReq) (*pbds.ServiceInstanceResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	svcInstances, err := s.processCountByServiceInstance(grpcKit, int(req.GetBizId()), int(req.GetModuleId()))
	if err != nil {
		return nil, err
	}

	return &pbds.ServiceInstanceResp{
		ServiceInstances: svcInstances,
	}, nil
}

// 根据 ServiceInstanceID 统计模块进程数
func (s *Service) processCountByServiceInstance(kit *kit.Kit, bizID, moduleID int) ([]*pbct.ServiceInstanceInfo, error) {

	svcInstances, err := s.cmdb.ListServiceInstance(kit.Ctx, &bkcmdb.ServiceInstanceListReq{
		BkBizID:    bizID,
		BkModuleID: moduleID,
	})
	if err != nil {
		return nil, err
	}

	svcInsts := pbct.ConvertServiceInstances(svcInstances.Info)

	for _, inst := range svcInsts {
		count, err := s.dao.Process().ProcessCountByServiceInstance(kit, uint32(bizID), uint32(inst.Id))
		if err != nil {
			return nil, err
		}
		inst.ProcessCount = uint32(count)
	}

	return svcInsts, nil
}

// ProcessInstance implements pbds.DataServer.
func (s *Service) ProcessInstance(ctx context.Context, req *pbds.ProcessInstanceReq) (*pbds.ProcessInstanceResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	resp, err := s.cmdb.ListProcessInstance(grpcKit.Ctx, &bkcmdb.ListProcessInstanceReq{
		BkBizID:           int(req.GetBizId()),
		ServiceInstanceID: int(req.GetServiceInstanceId()),
	})
	if err != nil {
		return nil, err
	}

	return &pbds.ProcessInstanceResp{
		ProcessInstances: pbct.ConvertProcessInstances(resp),
	}, nil
}

// ConfigTemplateVariable implements pbds.DataServer.
func (s *Service) ConfigTemplateVariable(ctx context.Context, req *pbds.ConfigTemplateVariableReq) (*pbds.ConfigTemplateVariableResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	// 获取业务ID
	bizID := int(req.GetBizId())
	if bizID == 0 {
		return nil, fmt.Errorf("biz_id is required")
	}

	// 使用 CCTopoXMLService 获取业务对象属性（复用 cc_topo.go 中的逻辑）
	topoService := cmdb.NewCCTopoXMLService(bizID, s.cmdb)
	objectAttrs, err := topoService.GetBizObjectAttributes(grpcKit.Ctx)
	if err != nil {
		logs.Errorf("get biz object attributes failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	// 转换为 ConfigTemplateVariable 格式
	configTemplateVariables := make([]*pbct.ConfigTemplateVariable, 0)

	// 按顺序处理 Set、Module、Host、Global 对象属性
	objIDs := []string{cmdb.BK_SET_OBJ_ID, cmdb.BK_MODULE_OBJ_ID, cmdb.BK_HOST_OBJ_ID, "global"}
	for _, objID := range objIDs {
		if attrs, ok := objectAttrs[objID]; ok {
			for _, attr := range attrs {
				configTemplateVariables = append(configTemplateVariables, &pbct.ConfigTemplateVariable{
					Key:   attr.BkPropertyName, // bk_property_name 作为 Key
					Value: attr.BkPropertyID,   // bk_property_id 作为 Value
				})
			}
		}
	}

	return &pbds.ConfigTemplateVariableResp{
		ConfigTemplateVariables: configTemplateVariables,
	}, nil
}

// BindProcessInstance implements pbds.DataServer.
func (s *Service) BindProcessInstance(ctx context.Context, req *pbds.BindProcessInstanceReq) (*pbds.BindProcessInstanceResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	// 1. 获取配置模板
	configTemplate, err := s.dao.ConfigTemplate().GetByID(grpcKit, req.GetBizId(), req.GetConfigTemplateId())
	if err != nil {
		return nil, err
	}

	configTemplate.Attachment.CcTemplateProcessIDs = req.GetCcTemplateProcessIds()
	configTemplate.Attachment.CcProcessIDs = req.GetCcProcessIds()

	// 2. 更新配置模板
	if err = s.dao.ConfigTemplate().Update(grpcKit, configTemplate); err != nil {
		return nil, err
	}

	return &pbds.BindProcessInstanceResp{
		Id: configTemplate.ID,
	}, nil
}

// PreviewBindProcessInstance implements pbds.DataServer.
func (s *Service) PreviewBindProcessInstance(ctx context.Context, req *pbds.PreviewBindProcessInstanceReq) (*pbds.PreviewBindProcessInstanceResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	// 1. 获取配置模板
	configTemplate, err := s.dao.ConfigTemplate().GetByID(grpcKit, req.GetBizId(), req.GetConfigTemplateId())
	if err != nil {
		return nil, err
	}

	instanceProcesses := make([]*pbct.BindProcessInstance, 0)
	if len(configTemplate.Attachment.CcProcessIDs) != 0 {
		process, _, err := s.dao.Process().List(grpcKit, req.GetBizId(), &pbproc.ProcessSearchCondition{
			CcProcessIds: configTemplate.Attachment.CcProcessIDs,
		}, &types.BasePage{
			All:    true,
			TopIds: []uint32{},
		})
		if err != nil {
			return nil, err
		}
		for _, v := range process {
			instanceProcesses = append(instanceProcesses, &pbct.BindProcessInstance{
				Name:        v.Spec.ServiceName,
				Id:          v.Attachment.CcProcessID,
				ProcessName: v.Spec.Alias,
			})
		}
	}

	templateProcesses := make([]*pbct.BindProcessInstance, 0)
	ids := configTemplate.Attachment.CcTemplateProcessIDs
	if len(ids) != 0 {
		processes, _, err := s.dao.Process().List(grpcKit, req.GetBizId(), &pbproc.ProcessSearchCondition{
			ProcessTemplateIds: ids,
		}, &types.BasePage{
			All:    true,
			TopIds: []uint32{},
		})
		if err != nil {
			return nil, err
		}
		// 已查到的 ID
		foundIDs := make(map[uint32]struct{})
		for _, p := range processes {
			templateProcesses = append(templateProcesses, &pbct.BindProcessInstance{
				Name:        p.Spec.ModuleName,
				Id:          p.Attachment.ProcessTemplateID,
				ProcessName: p.Spec.Alias,
			})
			foundIDs[p.Attachment.ProcessTemplateID] = struct{}{}
		}
		// 查找缺失的 ID
		missing := make([]uint32, 0)
		for _, id := range ids {
			if _, ok := foundIDs[id]; !ok {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			// 对于缺失 ID 调用cc接口补全
			for _, v := range missing {
				// 获取进程模板信息
				procTemplate, err := s.cmdb.GetProcTemplate(grpcKit.Ctx, bkcmdb.GetProcTemplateReq{
					BkBizID:           int(req.GetBizId()),
					ProcessTemplateID: int(v),
				})
				if err != nil {
					return nil, err
				}
				// 获取服务模板信息
				svcTemplate, err := s.cmdb.GetServiceTemplate(grpcKit.Ctx, bkcmdb.ServiceTemplateReq{
					ServiceTemplateID: procTemplate.ServiceTemplateID,
				})
				if err != nil {
					return nil, err
				}
				templateProcesses = append(templateProcesses, &pbct.BindProcessInstance{
					Id:          v,
					ProcessName: procTemplate.BkProcessName,
					Name:        svcTemplate.Name,
				})
			}
		}

	}

	return &pbds.PreviewBindProcessInstanceResp{
		TemplateProcesses: templateProcesses,
		InstanceProcesses: instanceProcesses,
	}, nil
}

// UpdateConfigTemplate implements pbds.DataServer.
func (s *Service) UpdateConfigTemplate(ctx context.Context, req *pbds.UpdateConfigTemplateReq) (*pbds.UpdateConfigTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)
	// 同一业务下不能出现同名的模板
	ct, err := s.dao.ConfigTemplate().GetByUniqueKey(grpcKit, req.GetBizId(), req.GetConfigTemplateId(), req.GetName())
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if ct != nil {
		return nil, fmt.Errorf(
			"the same template name already exists under this %d business: %s", req.GetBizId(), req.GetName(),
		)
	}

	now := time.Now().UTC()
	// 1. 获取配置模板
	configTemplate, err := s.dao.ConfigTemplate().GetByID(grpcKit, req.GetBizId(), req.GetConfigTemplateId())
	if err != nil {
		return nil, err
	}

	// 2. 查询模板文件
	template, err := s.dao.Template().GetByID(grpcKit, req.GetBizId(), configTemplate.Attachment.TemplateID)
	if err != nil {
		return nil, err
	}

	// 3. 获取最新模板版本文件
	revision, err := s.dao.TemplateRevision().GetLatestTemplateRevision(grpcKit, req.GetBizId(), template.ID)
	if err != nil {
		return nil, err
	}

	spec := *revision.Spec
	spec.RevisionName = req.GetRevisionName()
	spec.RevisionMemo = req.GetRevisionMemo()
	spec.Charset = table.FileCharset(req.GetCharset())
	spec.FileMode = table.FileMode(req.GetFileMode())
	spec.ContentSpec = &table.ContentSpec{Signature: req.GetSign(), ByteSize: req.GetByteSize(), Md5: req.GetMd5()}
	spec.Permission = &table.FilePermission{User: req.GetUser(), UserGroup: req.GetUserGroup(), Privilege: req.GetPrivilege()}
	templateRevision := &table.TemplateRevision{
		Spec:       &spec,
		Attachment: revision.Attachment,
		Revision:   &table.CreatedRevision{Creator: grpcKit.User, CreatedAt: now},
	}

	tx := s.dao.GenQuery().Begin()

	// Use defer to ensure transaction is properly handled
	committed := false
	defer func() {
		if !committed {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, grpcKit.Rid)
			}
		}
	}()

	// 如果文件权限和内容以及描述没变化不更新模板版本数据
	if !reflect.DeepEqual(revision.Spec.ContentSpec, spec.ContentSpec) ||
		!reflect.DeepEqual(revision.Spec.Permission, spec.Permission) || req.GetRevisionMemo() != template.Spec.Memo {
		// 生成新的版本文件
		_, err = s.dao.TemplateRevision().CreateWithTx(grpcKit, tx, templateRevision, true)
		if err != nil {
			logs.Errorf("create template revision failed, err: %v, rid: %s", err, grpcKit.Rid)
			return nil, err
		}
	}

	template.Revision.Reviser = grpcKit.User
	template.Revision.UpdatedAt = now
	// 更新模板文件
	err = s.dao.Template().UpdateWithTx(grpcKit, tx, &table.Template{
		ID: template.ID,
		Spec: &table.TemplateSpec{
			Memo: req.GetRevisionMemo(),
			Path: template.Spec.Path,
			Name: template.Spec.Name,
		},
		Attachment: template.Attachment,
		Revision:   template.Revision,
	})
	if err != nil {
		logs.Errorf("update template failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	// 更新配置模板
	err = s.dao.ConfigTemplate().UpdateWithTx(grpcKit, tx, &table.ConfigTemplate{
		ID: configTemplate.ID,
		Spec: &table.ConfigTemplateSpec{
			Name:           req.GetName(),
			HighlightStyle: table.HighlightStyle(req.GetHighlightStyle()),
		},
		Attachment: configTemplate.Attachment,
		Revision: &table.Revision{
			Reviser:   grpcKit.User,
			UpdatedAt: now,
		},
	})
	if err != nil {
		logs.Errorf("update config template failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	if e := tx.Commit(); e != nil {
		logs.Errorf("commit transaction failed, err: %v, rid: %s", e, grpcKit.Rid)
		return nil, e
	}
	committed = true

	return &pbds.UpdateConfigTemplateResp{}, nil
}

// GetConfigTemplate implements pbds.DataServer.
func (s *Service) GetConfigTemplate(ctx context.Context, req *pbds.GetConfigTemplateReq) (*pbds.GetConfigTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	// 1. 获取配置模板
	configTemplate, err := s.dao.ConfigTemplate().GetByID(grpcKit, req.GetBizId(), req.GetConfigTemplateId())
	if err != nil {
		return nil, err
	}

	// 2. 查询模板文件
	template, err := s.dao.Template().GetByID(grpcKit, req.GetBizId(), configTemplate.Attachment.TemplateID)
	if err != nil {
		return nil, err
	}

	// 3. 获取最新模板版本文件
	revision, err := s.dao.TemplateRevision().GetLatestTemplateRevision(grpcKit, req.GetBizId(), template.ID)
	if err != nil {
		return nil, err
	}

	resp := &pbct.BindTemplate{
		TemplateSpaceName:    constant.CONFIG_DELIVERY,
		TemplateSetName:      constant.DefaultTmplSetName,
		FileName:             template.Spec.Name,
		FilePath:             template.Spec.Path,
		Memo:                 revision.Spec.RevisionMemo,
		RevisionName:         configTemplate.Spec.Name,
		User:                 revision.Spec.Permission.User,
		UserGroup:            revision.Spec.Permission.UserGroup,
		Privilege:            revision.Spec.Permission.Privilege,
		Sign:                 revision.Spec.ContentSpec.Signature,
		ByteSize:             revision.Spec.ContentSpec.ByteSize,
		Md5:                  revision.Spec.ContentSpec.Md5,
		Charset:              string(revision.Spec.Charset),
		HighlightStyle:       string(configTemplate.Spec.HighlightStyle),
		FileMode:             string(revision.Spec.FileMode),
		CcTemplateProcessIds: configTemplate.Attachment.CcTemplateProcessIDs,
		CcProcessIds:         configTemplate.Attachment.CcProcessIDs,
		Name:                 configTemplate.Spec.Name,
	}

	return &pbds.GetConfigTemplateResp{
		BindTemplate: resp,
	}, nil
}

// DeleteConfigTemplate implements pbds.DataServer.
func (s *Service) DeleteConfigTemplate(ctx context.Context, req *pbds.DeleteConfigTemplateReq) (*pbds.DeleteConfigTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	// 1. 获取配置模板
	configTemplate, err := s.dao.ConfigTemplate().GetByID(grpcKit, req.GetBizId(), req.GetConfigTemplateId())
	if err != nil {
		return nil, err
	}

	tx := s.dao.GenQuery().Begin()
	committed := false
	defer func() {
		if !committed {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, grpcKit.Rid)
			}
		}
	}()

	// 2. 删除模板文件
	template, err := s.dao.Template().GetByID(grpcKit, req.GetBizId(), configTemplate.Attachment.TemplateID)
	if err != nil {
		return nil, err
	}

	if err = s.dao.Template().DeleteWithTx(grpcKit, tx, template); err != nil {
		logs.Errorf("delete template failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	// 3. 删除模板版本
	if err = s.dao.TemplateRevision().DeleteForTmplWithTx(grpcKit, tx, req.GetBizId(), template.ID); err != nil {
		logs.Errorf("delete template failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	// 4. 从套餐中删除
	if err = s.dao.TemplateSet().DeleteTmplFromAllTmplSetsWithTx(grpcKit, tx, req.GetBizId(), template.ID); err != nil {
		logs.Errorf("delete template failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	// 5. 删除配置模板
	if err = s.dao.ConfigTemplate().DeleteWithTx(grpcKit, tx, configTemplate); err != nil {
		logs.Errorf("delete template failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		logs.Errorf("commit transaction failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}
	committed = true

	return &pbds.DeleteConfigTemplateResp{}, nil
}
