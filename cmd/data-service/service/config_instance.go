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

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbcin "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/config-instance"
	pbproc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// ListConfigInstances implements pbds.DataServer.
func (s *Service) ListConfigInstances(ctx context.Context, req *pbds.ListConfigInstancesReq) (*pbds.ListConfigInstancesResp, error) {
	kt := kit.FromGrpcContext(ctx)

	if req.ConfigTemplateId == 0 {
		return &pbds.ListConfigInstancesResp{
			Count:           0,
			ConfigInstances: make([]*pbcin.ConfigInstance, 0),
			FilterOptions: &pbcin.ConfigInstanceFilterOptions{
				TemplateVersionChoices: make([]*pbcin.Choice, 0),
			},
		}, nil
	}

	// 获取配置模板信息
	configTemplate, err := s.dao.ConfigTemplate().GetByID(kt, req.BizId, req.ConfigTemplateId)
	if err != nil {
		return nil, fmt.Errorf("get config template failed, err: %v", err)
	}

	// 根据查询条件，获取过滤后的进程列表
	filteredProcesses, err := getFilteredProcesses(kt, req.BizId, s.dao, configTemplate, req.Search)
	if err != nil {
		return nil, err
	}
	if len(filteredProcesses) == 0 {
		return &pbds.ListConfigInstancesResp{
			Count:           0,
			ConfigInstances: make([]*pbcin.ConfigInstance, 0),
			FilterOptions: &pbcin.ConfigInstanceFilterOptions{
				TemplateVersionChoices: make([]*pbcin.Choice, 0),
			},
		}, nil
	}

	// 查询进程实例列表
	processInstances, err := getProcessInstances(kt, s.dao, req.BizId, filteredProcesses)
	if err != nil {
		return nil, err
	}

	// 构建配置实例列表
	finalConfigInstances, err := buildConfigInstancesList(kt, s.dao, req.BizId, req.ConfigTemplateId, processInstances, filteredProcesses)
	if err != nil {
		return nil, err
	}

	// 构建过滤选项
	filterOptions, err := buildFilterOptions(kt, s.dao, finalConfigInstances)
	if err != nil {
		return nil, err
	}

	// 根据配置模版版本过滤配置实例
	finalConfigInstances = filterConfigInstancesByVersion(finalConfigInstances, req.ConfigTemplateVersionIds)

	// 获取关联数据
	relatedData, err := getRelatedData(kt, s.dao, configTemplate, finalConfigInstances)
	if err != nil {
		return nil, err
	}

	// 构建 PB 对象
	pbConfigInstances, err := buildPbConfigInstances(finalConfigInstances, filteredProcesses, relatedData)
	if err != nil {
		return nil, err
	}

	return &pbds.ListConfigInstancesResp{
		Count:           uint32(len(pbConfigInstances)),
		ConfigInstances: pbConfigInstances,
		FilterOptions:   filterOptions,
	}, nil
}

// getFilteredProcesses 获取过滤后的进程列表
func getFilteredProcesses(
	kt *kit.Kit,
	bizID uint32,
	dao dao.Set,
	configTemplate *table.ConfigTemplate,
	search *pbcin.ConfigInstanceSearchCondition,
) ([]*table.Process, error) {
	// 获取CcProcessID，其中模版进程ID列表对应多个进程
	var (
		processes []*table.Process
		err       error
	)
	templateProcessIDs := configTemplate.Attachment.CcTemplateProcessIDs
	if len(templateProcessIDs) != 0 {
		processes, _, err = dao.Process().List(kt, bizID, &pbproc.ProcessSearchCondition{
			CcProcessIds: templateProcessIDs,
		}, &types.BasePage{
			All: true,
		})
		if err != nil {
			return nil, fmt.Errorf("list processes by template process ids failed, err: %v", err)
		}
	}
	ccProcessIDs := make([]uint32, 0, len(processes)+len(configTemplate.Attachment.CcProcessIDs))
	for _, process := range processes {
		ccProcessIDs = append(ccProcessIDs, process.Attachment.CcProcessID)
	}
	ccProcessIDs = append(ccProcessIDs, configTemplate.Attachment.CcProcessIDs...)

	// 根据过滤条件再次查询完整的进程列表
	processSearchCondition := &pbproc.ProcessSearchCondition{
		CcProcessIds: ccProcessIDs,
	}
	if search != nil {
		processSearchCondition.Sets = search.Sets
		processSearchCondition.Modules = search.Modules
		processSearchCondition.ServiceInstances = search.ServiceInstances
		processSearchCondition.ProcessAliases = search.ProcessAliases
	}

	filteredProcesses, _, err := dao.Process().List(kt, bizID, processSearchCondition, &types.BasePage{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("list filtered processes failed, err: %v", err)
	}

	return filteredProcesses, nil
}

// getProcessInstances 查询进程实例列表
func getProcessInstances(kt *kit.Kit, dao dao.Set, bizID uint32, processes []*table.Process) ([]*table.ProcessInstance, error) {
	processIDs := make([]uint32, 0, len(processes))
	for _, process := range processes {
		processIDs = append(processIDs, process.ID)
	}

	// 查询进程实例列表
	processInstances, err := dao.ProcessInstance().GetByProcessIDs(kt, bizID, processIDs)
	if err != nil {
		return nil, fmt.Errorf("get process instances failed, err: %v", err)
	}

	return processInstances, nil
}

// buildConfigInstancesList 构建配置实例列表
func buildConfigInstancesList(
	kt *kit.Kit,
	dao dao.Set,
	bizID uint32,
	configTemplateID uint32,
	processInstances []*table.ProcessInstance,
	processes []*table.Process,
) ([]*table.ConfigInstance, error) {
	// 构建预返回的配置实例列表
	// 配置模版关联的进程在过滤后，查询出的进程实例数量等于需要下发的配置实例列表
	// 使用key: CcProcessID_ConfigTemplateID_ModuleInstSeq 作为唯一标识
	preConfigInstancesMap := make(map[string]*table.ConfigInstance)
	for _, processInstance := range processInstances {
		key := buildConfigInstanceKey(processInstance.Attachment.CcProcessID, configTemplateID, processInstance.Spec.ModuleInstSeq)
		preConfigInstancesMap[key] = &table.ConfigInstance{
			Attachment: &table.ConfigInstanceAttachment{
				BizID:            bizID,
				ConfigTemplateID: configTemplateID,
				ConfigVersionID:  0, // 配置实例未关联任何配置模版版本，页面展示为“-”
				CcProcessID:      processInstance.Attachment.CcProcessID,
				ModuleInstSeq:    processInstance.Spec.ModuleInstSeq,
			},
		}
	}

	// 查询已创建的配置实例
	actualConfigInstancesMap, err := getActualConfigInstances(kt, dao, bizID, configTemplateID, processes)
	if err != nil {
		return nil, err
	}

	// 合并预返回和已创建的配置实例
	finalConfigInstances := make([]*table.ConfigInstance, 0, len(preConfigInstancesMap))
	for key, preCI := range preConfigInstancesMap {
		if actualCI, exists := actualConfigInstancesMap[key]; exists {
			// 配置实例已创建
			finalConfigInstances = append(finalConfigInstances, actualCI)
		} else {
			// 配置实例还未创建
			finalConfigInstances = append(finalConfigInstances, preCI)
		}
	}

	return finalConfigInstances, nil
}

// getActualConfigInstances 查询已创建的配置实例
func getActualConfigInstances(
	kt *kit.Kit,
	ds dao.Set,
	bizID uint32,
	configTemplateID uint32,
	processes []*table.Process,
) (map[string]*table.ConfigInstance, error) {
	actualConfigInstancesMap := make(map[string]*table.ConfigInstance)
	if len(processes) == 0 {
		return actualConfigInstancesMap, nil
	}

	// 从过滤后的进程列表中收集所有的 CcProcessID
	ccProcessIDList := make([]uint32, 0, len(processes))
	for _, process := range processes {
		ccProcessIDList = append(ccProcessIDList, process.Attachment.CcProcessID)
	}

	// 查询所有相关的配置实例
	configInstanceSearchCondition := &dao.ConfigInstanceSearchCondition{
		CcProcessIds:     ccProcessIDList,
		ConfigTemplateId: configTemplateID,
	}
	configInstances, _, err := ds.ConfigInstance().List(kt, bizID, configInstanceSearchCondition, &types.BasePage{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("list config instances failed, err: %v", err)
	}

	// 构建映射表
	for _, ci := range configInstances {
		key := buildConfigInstanceKey(ci.Attachment.CcProcessID, ci.Attachment.ConfigTemplateID, ci.Attachment.ModuleInstSeq)
		actualConfigInstancesMap[key] = ci
	}

	return actualConfigInstancesMap, nil
}

// relatedData 配置实例相关数据
type relatedData struct {
	configTemplateName         string
	configFileName             string
	latestTemplateRevisionName string // 即将下发的配置模版版本号，每次配置下发都是用模版的最新版本
	configVersionMap           map[uint32]*table.TemplateRevision
}

// getRelatedData 获取关联数据
func getRelatedData(
	kt *kit.Kit,
	dao dao.Set,
	configTemplate *table.ConfigTemplate,
	configInstances []*table.ConfigInstance,
) (*relatedData, error) {
	// 查询template表，获取文件名及关联的最新版本
	template, err := dao.Template().GetByID(kt, configTemplate.Attachment.BizID, configTemplate.Attachment.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("get template failed, err: %v", err)
	}

	// 获取模版关联的最新版本
	latestRevision, err := dao.TemplateRevision().GetLatestTemplateRevision(kt, configTemplate.Attachment.BizID, configTemplate.Attachment.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("get latest template revision failed, err: %v", err)
	}

	// 若是已存在的配置实例则ConfigVersionID不为0，根据ConfigVersionID查询版本信息，用于展示配置实例关联的版本及版本描述
	configVersionIDs := make([]uint32, 0, len(configInstances))
	for _, ci := range configInstances {
		if ci.Attachment != nil && ci.Attachment.ConfigVersionID > 0 {
			configVersionIDs = append(configVersionIDs, ci.Attachment.ConfigVersionID)
		}
	}
	configVersionMap := make(map[uint32]*table.TemplateRevision)
	if len(configVersionIDs) > 0 {
		templateRevisions, err := dao.TemplateRevision().ListByIDs(kt, configVersionIDs)
		if err != nil {
			return nil, fmt.Errorf("list template revisions failed, err: %v", err)
		}
		for _, tr := range templateRevisions {
			configVersionMap[tr.ID] = tr
		}
	}

	return &relatedData{
		configTemplateName:         configTemplate.Spec.Name,
		configFileName:             template.Spec.Name,
		latestTemplateRevisionName: latestRevision.Spec.RevisionName,
		configVersionMap:           configVersionMap,
	}, nil
}

// buildPbConfigInstances 构建 PB 对象
func buildPbConfigInstances(configInstances []*table.ConfigInstance,
	processes []*table.Process, data *relatedData) ([]*pbcin.ConfigInstance, error) {
	// 构建 cc进程ID到进程对象的映射
	ccProcessIDMap := make(map[uint32]*table.Process)
	for _, process := range processes {
		ccProcessIDMap[process.Attachment.CcProcessID] = process
	}

	// 构建配置实例的pb对象，填充关联信息
	pbConfigInstances := make([]*pbcin.ConfigInstance, 0, len(configInstances))
	for _, ci := range configInstances {
		// 获取关联的进程信息
		process, exists := ccProcessIDMap[ci.Attachment.CcProcessID]
		if !exists {
			return nil, fmt.Errorf("process not found for cc process id: %d", ci.Attachment.CcProcessID)
		}

		// 获取配置版本信息
		var (
			configVersionName = "-"
			configVersionMemo = ""
			configFileName    = ""
		)
		if ci.Attachment.ConfigVersionID > 0 {
			if templateRevision, exists := data.configVersionMap[ci.Attachment.ConfigVersionID]; exists {
				configVersionName = templateRevision.Spec.RevisionName
				configVersionMemo = templateRevision.Spec.RevisionMemo
				configFileName = templateRevision.Spec.Name
			}
		}

		// 构建配置实例的pb对象
		pbCI := pbcin.PbConfigInstanceWithDetails(
			ci,
			data.configTemplateName,
			process,
			configVersionName,
			configVersionMemo,
			configFileName,
			data.latestTemplateRevisionName,
		)
		pbConfigInstances = append(pbConfigInstances, pbCI)
	}

	return pbConfigInstances, nil
}

// buildConfigInstanceKey 构建配置实例的唯一标识
func buildConfigInstanceKey(ccProcessID, configTemplateID, moduleInstSeq uint32) string {
	return fmt.Sprintf("%d_%d_%d", ccProcessID, configTemplateID, moduleInstSeq)
}

// filterConfigInstancesByVersion 根据配置模版版本过滤配置实例
func filterConfigInstancesByVersion(configInstances []*table.ConfigInstance, configTemplateVersionIds []uint32) []*table.ConfigInstance {
	if len(configTemplateVersionIds) == 0 {
		return configInstances
	}

	// 构建版本ID的映射，用于快速查找
	versionIDMap := make(map[uint32]bool, len(configTemplateVersionIds))
	for _, versionID := range configTemplateVersionIds {
		versionIDMap[versionID] = true
	}

	// 过滤配置实例，只保留版本ID在列表中的实例
	filteredInstances := make([]*table.ConfigInstance, 0, len(configInstances))
	for _, ci := range configInstances {
		// 检查配置实例的版本ID是否在目标版本列表中
		if ci.Attachment != nil && versionIDMap[ci.Attachment.ConfigVersionID] {
			filteredInstances = append(filteredInstances, ci)
		}
	}

	return filteredInstances
}

// buildFilterOptions 构建过滤选项
func buildFilterOptions(
	kt *kit.Kit,
	dao dao.Set,
	configInstances []*table.ConfigInstance,
) (*pbcin.ConfigInstanceFilterOptions, error) {
	// 从配置实例中提取所有唯一的版本ID
	versionIDMap := make(map[uint32]bool)
	for _, ci := range configInstances {
		if ci.Attachment != nil && ci.Attachment.ConfigVersionID > 0 {
			versionIDMap[ci.Attachment.ConfigVersionID] = true
		}
	}

	if len(versionIDMap) == 0 {
		return &pbcin.ConfigInstanceFilterOptions{
			TemplateVersionChoices: []*pbcin.Choice{
				{
					Id:   "0",
					Name: "-",
				},
			},
		}, nil
	}

	// 批量查询版本信息
	versionIDs := make([]uint32, 0, len(versionIDMap))
	for versionID := range versionIDMap {
		versionIDs = append(versionIDs, versionID)
	}
	templateRevisions, err := dao.TemplateRevision().ListByIDs(kt, versionIDs)
	if err != nil {
		return nil, fmt.Errorf("list template revisions by ids failed, err: %v", err)
	}

	// 构建版本选择项列表
	templateVersionChoices := make([]*pbcin.Choice, 0, len(templateRevisions))
	for _, tr := range templateRevisions {
		choice := &pbcin.Choice{
			Id:   fmt.Sprintf("%d", tr.ID),
			Name: tr.Spec.RevisionName,
		}
		templateVersionChoices = append(templateVersionChoices, choice)
	}

	return &pbcin.ConfigInstanceFilterOptions{
		TemplateVersionChoices: templateVersionChoices,
	}, nil
}
