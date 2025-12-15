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
	"slices"
	"strings"
	"time"

	taskpkg "github.com/Tencent/bk-bcs/bcs-common/common/task"
	istore "github.com/Tencent/bk-bcs/bcs-common/common/task/stores/iface"
	taskTypes "github.com/Tencent/bk-bcs/bcs-common/common/task/types"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/task"
	"github.com/TencentBlueKing/bk-bscp/internal/task/builder/config"
	executorCommon "github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbcin "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/config-instance"
	pbproc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
	"github.com/TencentBlueKing/bk-bscp/render"
)

// ListConfigInstances implements pbds.DataServer.
func (s *Service) ListConfigInstances(ctx context.Context, req *pbds.ListConfigInstancesReq) (*pbds.ListConfigInstancesResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// validate the page params
	opt := &types.BasePage{Start: req.Start, Limit: uint(req.Limit), All: req.All}
	if err := opt.Validate(types.DefaultPageOption); err != nil {
		return nil, err
	}

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

	// 应用分页
	totalCount := uint32(len(pbConfigInstances))
	if !req.All {
		// 如果不是获取所有数据，则进行分页
		start := int(req.Start)
		limit := int(req.Limit)
		if start >= len(pbConfigInstances) {
			pbConfigInstances = make([]*pbcin.ConfigInstance, 0)
		} else {
			end := start + limit
			if end > len(pbConfigInstances) {
				end = len(pbConfigInstances)
			}
			pbConfigInstances = pbConfigInstances[start:end]
		}
	}

	return &pbds.ListConfigInstancesResp{
		Count:           totalCount,
		ConfigInstances: pbConfigInstances,
		FilterOptions:   filterOptions,
	}, nil
}

func validateRequest(req *pbds.GenerateConfigReq) error {
	if req.BizId == 0 {
		return fmt.Errorf("biz id is required")
	}
	if len(req.ConfigTemplateGroups) == 0 {
		return fmt.Errorf("at least one config template group is required")
	}
	for _, group := range req.ConfigTemplateGroups {
		if group.ConfigTemplateId == 0 {
			return fmt.Errorf("config template id is required")
		}
		if group.ConfigTemplateVersionId == 0 {
			return fmt.Errorf("config template version id is required")
		}
		if len(group.CcProcessIds) == 0 {
			return fmt.Errorf("process list is required")
		}
	}
	return nil
}

// GenerateConfig implements pbds.DataServer.
// nolint:funlen
func (s *Service) GenerateConfig(
	ctx context.Context,
	req *pbds.GenerateConfigReq,
) (*pbds.GenerateConfigResp, error) {
	kt := kit.FromGrpcContext(ctx)
	// 校验
	if err := validateRequest(req); err != nil {
		return nil, err
	}

	// 预处理：收集所有任务信息并计算总任务数
	type taskInfo struct {
		configTemplateID uint32
		configTemplate   *table.ConfigTemplate
		template         *table.Template
		latestRevision   *table.TemplateRevision
		process          *table.Process
		processInstance  *table.ProcessInstance
	}
	var taskInfos []taskInfo
	// cc进程id去重集合
	processIDSet := make(map[uint32]struct{})
	// 环境信息，用于构建操作范围
	environment := ""
	for _, group := range req.ConfigTemplateGroups {
		configTemplateID := group.ConfigTemplateId
		configTemplate, err := s.dao.ConfigTemplate().GetByID(kt, req.BizId, configTemplateID)
		if err != nil {
			return nil, fmt.Errorf("get config template failed, err: %v", err)
		}
		// 查询配置模版关联的模版信息和最新版本
		template, err := s.dao.Template().GetByID(kt, req.BizId, configTemplate.Attachment.TemplateID)
		if err != nil {
			return nil, fmt.Errorf("get template failed, err: %v", err)
		}
		latestRevision, err := s.dao.TemplateRevision().GetLatestTemplateRevision(kt, req.BizId, configTemplate.Attachment.TemplateID)
		if err != nil {
			return nil, fmt.Errorf("get latest template revision failed, err: %v", err)
		}
		// 仅最新版本的配置可以生成和下发
		if group.ConfigTemplateVersionId != latestRevision.ID {
			return nil, fmt.Errorf("config template version id is not latest")
		}

		// 查询进程信息
		processes, _, err := s.dao.Process().List(kt, req.BizId, &pbproc.ProcessSearchCondition{
			CcProcessIds: group.CcProcessIds,
		}, &types.BasePage{
			All: true,
		})
		if err != nil {
			return nil, fmt.Errorf("get process failed, err: %v", err)
		}
		// 查询到的进程数量不等于提供的进程ID数量，说明存在进程被删除的情况，需要刷新重新提交
		if len(processes) != len(group.CcProcessIds) || len(processes) == 0 {
			return nil, fmt.Errorf("some processes not found for biz %d with provided process IDs", req.BizId)
		}

		// 判断进程和配置模版的绑定关系
		isBindRelation, err := isBindRelation(kt, s.dao, req.BizId, processes, configTemplate)
		if err != nil {
			return nil, fmt.Errorf("check bind relation failed, err: %v", err)
		}
		if !isBindRelation {
			return nil, fmt.Errorf("invalid binding relationship between process and config template")
		}

		// 收集进程ID用于构建操作范围（去重）
		for _, id := range group.CcProcessIds {
			processIDSet[id] = struct{}{}
		}
		// 获取环境信息（取第一个进程的环境）
		if environment == "" && len(processes) > 0 {
			environment = processes[0].Spec.Environment
		}

		for _, process := range processes {
			processInstances, err := s.dao.ProcessInstance().GetByProcessIDs(kt, req.BizId, []uint32{process.ID})
			if err != nil {
				return nil, fmt.Errorf("get process instance failed, err: %v", err)
			}
			for _, processInstance := range processInstances {
				taskInfos = append(taskInfos, taskInfo{
					configTemplateID: configTemplateID,
					configTemplate:   configTemplate,
					template:         template,
					latestRevision:   latestRevision,
					process:          process,
					processInstance:  processInstance,
				})
			}
		}
	}

	// 计算总任务数
	totalCount := uint32(len(taskInfos))
	if totalCount == 0 {
		return nil, fmt.Errorf("no tasks to create for biz %d", req.BizId)
	}
	if environment == "" {
		return nil, fmt.Errorf("no environment found for biz %d with provided process IDs", req.BizId)
	}

	// 创建任务批次
	now := time.Now()
	taskBatch := &table.TaskBatch{
		Attachment: &table.TaskBatchAttachment{
			BizID: req.BizId,
		},
		Spec: &table.TaskBatchSpec{
			TaskObject: table.TaskObjectConfigFile,
			TaskAction: table.TaskActionConfigGenerate,
			Status:     table.TaskBatchStatusRunning,
			TaskData:   "{}",
			StartAt:    &now,
			TotalCount: totalCount, // 设置总任务数，用于 Callback 机制判断批次完成
		},
		Revision: &table.Revision{
			Creator:   kt.User,
			Reviser:   kt.User,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	// 将去重后的进程ID转为切片
	processIDs := make([]uint32, 0, len(processIDSet))
	for id := range processIDSet {
		processIDs = append(processIDs, id)
	}

	// 构建操作范围
	operateRange := table.OperateRange{
		SetNames:     make([]string, 0, len(processIDs)),
		ModuleNames:  make([]string, 0, len(processIDs)),
		ServiceNames: make([]string, 0, len(processIDs)),
		ProcessAlias: make([]string, 0, len(processIDs)),
		CCProcessID:  processIDs,
	}
	taskBatch.Spec.SetTaskData(&table.TaskExecutionData{
		Environment:  environment,
		OperateRange: operateRange,
	})
	batchID, err := s.dao.TaskBatch().Create(kt, taskBatch)
	if err != nil {
		return nil, fmt.Errorf("create task batch failed, err: %v", err)
	}

	// 记录实际创建的任务数
	var dispatchedCount uint32

	// 如果任务创建过程出错，需要处理部分创建的情况
	defer func() {
		if dispatchedCount == totalCount {
			// 所有任务都已创建，由 Callback 机制处理状态更新
			return
		}

		// 计算未创建的任务数
		failedToCreate := totalCount - dispatchedCount
		logs.Warnf("task batch %d partially created: %d/%d tasks dispatched, %d failed to create, rid: %s",
			batchID, dispatchedCount, totalCount, failedToCreate, kt.Rid)

		// 将未创建的任务直接计为失败
		if updateErr := s.dao.TaskBatch().AddFailedCount(kt, batchID, failedToCreate); updateErr != nil {
			logs.Errorf("add failed count for batch %d error, err: %v, rid: %s", batchID, updateErr, kt.Rid)
		}
	}()

	// 配置生成：创建并下发任务
	for _, info := range taskInfos {
		// 创建任务对象
		taskObj, err := task.NewByTaskBuilder(
			config.NewConfigGenerateTask(
				s.dao,
				req.BizId,
				batchID,
				table.ConfigGenerate,
				kt.User,
				info.configTemplateID,
				info.configTemplate,
				info.template,
				info.latestRevision,
				info.processInstance.ID,
				info.processInstance,
				info.process.Attachment.CcProcessID,
				info.process,
				info.configTemplate.Spec.Name,
				info.process.Spec.Alias,
				info.processInstance.Spec.ModuleInstSeq,
			),
		)
		if err != nil {
			return nil, fmt.Errorf("create config generate task failed, err: %v", err)
		}
		// 下发任务
		s.taskManager.Dispatch(taskObj)
		dispatchedCount++
	}

	return &pbds.GenerateConfigResp{BatchId: batchID}, nil
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
			ProcessTemplateIds: templateProcessIDs,
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
	// 使用key: CcProcessID-ConfigTemplateID-ModuleInstSeq 作为唯一标识
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
	return fmt.Sprintf("%d-%d-%d", ccProcessID, configTemplateID, moduleInstSeq)
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

// isBindRelation 判断进程和配置模版的绑定关系是否正常，避免配置生成过程中进程与配置模版解绑
func isBindRelation(
	kt *kit.Kit,
	dao dao.Set,
	bizID uint32,
	processes []*table.Process,
	configTemplate *table.ConfigTemplate,
) (bool, error) {
	var (
		templateBoundProcesses []*table.Process
		err                    error
	)
	// 获取CcProcessID，其中模版进程ID列表对应多个进程
	templateProcessIDs := configTemplate.Attachment.CcTemplateProcessIDs
	if len(templateProcessIDs) != 0 {
		templateBoundProcesses, _, err = dao.Process().List(kt, bizID, &pbproc.ProcessSearchCondition{
			ProcessTemplateIds: templateProcessIDs,
		}, &types.BasePage{
			All: true,
		})
		if err != nil {
			return false, fmt.Errorf("list processes by template process ids failed, err: %v", err)
		}
	}
	templateBoundProcessIDs := make([]uint32, 0, len(templateBoundProcesses)+len(configTemplate.Attachment.CcProcessIDs))
	for _, process := range templateBoundProcesses {
		templateBoundProcessIDs = append(templateBoundProcessIDs, process.Attachment.CcProcessID)
	}
	templateBoundProcessIDs = append(templateBoundProcessIDs, configTemplate.Attachment.CcProcessIDs...)

	// 判断进程是否在配置模版关联的进程ID列表中
	for _, process := range processes {
		if !slices.Contains(templateBoundProcessIDs, process.Attachment.CcProcessID) {
			return false, fmt.Errorf("process %d is not in the config template", process.Attachment.CcProcessID)
		}
	}
	return true, nil
}

// ConfigGenerateStatus 获取配置生成状态
func (s *Service) ConfigGenerateStatus(
	ctx context.Context,
	req *pbds.ConfigGenerateStatusReq,
) (*pbds.ConfigGenerateStatusResp, error) {
	kt := kit.FromGrpcContext(ctx)
	taskStorage := taskpkg.GetGlobalStorage()
	if taskStorage == nil {
		return nil, fmt.Errorf("task storage not initialized")
	}

	// 分页查询所有任务数据
	const pageSize = 100
	var allTasks []*taskTypes.Task
	offset := int64(0)
	for {
		listOpt := &istore.ListOption{
			TaskIndex: fmt.Sprintf("%d", req.GetBatchId()),
			Limit:     pageSize,
			Offset:    offset,
		}

		pagination, err := taskStorage.ListTask(kt.Ctx, listOpt)
		if err != nil {
			logs.Errorf("list tasks failed, offset: %d, err: %v, rid: %s", offset, err, kt.Rid)
			return nil, fmt.Errorf("list tasks failed: %v", err)
		}
		// 没有更多数据，退出循环
		if len(pagination.Items) == 0 {
			break
		}
		allTasks = append(allTasks, pagination.Items...)

		// 如果返回的数量少于 pageSize，说明已经是最后一页
		if len(pagination.Items) < int(pageSize) {
			break
		}
		offset += pageSize
	}

	// 解析每个 task 的 CommonPayload，构建 ConfigGenerateStatus
	configGenerateStatuses := make([]*pbds.ConfigGenerateStatusResp_ConfigGenerateStatus, 0, len(allTasks))
	for _, task := range allTasks {
		// 解析 CommonPayload，构建 ConfigGenerateStatus
		commonPayload := &executorCommon.TaskPayload{}
		err := task.GetCommonPayload(commonPayload)
		if err != nil {
			logs.Errorf("get common payload failed, err: %v, rid: %s", err, kt.Rid)
			return nil, fmt.Errorf("get common payload failed: %v", err)
		}
		configGenerateStatuses = append(configGenerateStatuses, &pbds.ConfigGenerateStatusResp_ConfigGenerateStatus{
			ConfigInstanceKey: commonPayload.ConfigPayload.ConfigInstanceKey,
			Status:            task.GetStatus(),
			TaskId:            task.GetTaskID(),
		})
	}
	return &pbds.ConfigGenerateStatusResp{
		ConfigGenerateStatuses: configGenerateStatuses,
	}, nil
}

// GetConfigGenerateResult implements pbds.DataServer.
// GetConfigGenerateResult 从 task 的 payload 中获取配置生成结果
func (s *Service) GetConfigGenerateResult(ctx context.Context, req *pbds.GetConfigGenerateResultReq) (*pbds.GetConfigGenerateResultResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 获取 task storage
	taskStorage := taskpkg.GetGlobalStorage()
	if taskStorage == nil {
		return nil, fmt.Errorf("task storage not initialized")
	}

	// 从 task storage 中获取任务
	taskInfo, err := taskStorage.GetTask(ctx, req.TaskId)
	if err != nil {
		logs.Errorf("get task failed, taskID: %s, err: %v, rid: %s", req.TaskId, err, kt.Rid)
		return nil, fmt.Errorf("get task failed: %v", err)
	}

	if taskInfo == nil {
		logs.Errorf("task not found, taskID: %s, rid: %s", req.TaskId, kt.Rid)
		return nil, fmt.Errorf("task not found: %s", req.TaskId)
	}

	// 从 CommonPayload 中提取配置生成结果
	var taskPayload executorCommon.TaskPayload
	err = taskInfo.GetCommonPayload(&taskPayload)
	if err != nil {
		logs.Errorf("get common payload failed, taskID: %s, err: %v, rid: %s", req.TaskId, err, kt.Rid)
		return nil, fmt.Errorf("get common payload failed: %v", err)
	}

	// 检查 ConfigPayload 是否存在
	if taskPayload.ConfigPayload == nil {
		logs.Errorf("config payload is nil, taskID: %s, rid: %s", req.TaskId, kt.Rid)
		return nil, fmt.Errorf("config payload not found in task")
	}

	// 返回配置内容
	return &pbds.GetConfigGenerateResultResp{
		ConfigTemplateId:     taskPayload.ConfigPayload.ConfigTemplateID,
		ConfigTemplateName:   taskPayload.ConfigPayload.ConfigTemplateName,
		ConfigFileName:       taskPayload.ConfigPayload.ConfigFileName,
		ConfigFilePath:       taskPayload.ConfigPayload.ConfigFilePath,
		ConfigFileOwner:      taskPayload.ConfigPayload.ConfigFileOwner,
		ConfigFileGroup:      taskPayload.ConfigPayload.ConfigFileGroup,
		ConfigFilePermission: taskPayload.ConfigPayload.ConfigFilePermission,
		ConfigInstanceKey:    taskPayload.ConfigPayload.ConfigInstanceKey,
		Content:              taskPayload.ConfigPayload.ConfigContent,
	}, nil
}

// PreviewConfig 预览配置渲染结果
// 与 task 框架中的渲染逻辑保持一致，使用相同的参数和渲染方法
func (s *Service) PreviewConfig(ctx context.Context, req *pbds.PreviewConfigReq) (*pbds.PreviewConfigResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	// 1. 参数校验
	if req.GetBizId() == 0 {
		return nil, fmt.Errorf("biz_id is required")
	}
	if req.GetTemplateContent() == "" {
		return nil, fmt.Errorf("template_content is required")
	}
	if req.GetCcProcessId() == 0 {
		return nil, fmt.Errorf("cc_process_id is required")
	}

	// 2. 通过 cc_process_id 查询 Process
	processes, _, err := s.dao.Process().List(grpcKit, req.GetBizId(), &pbproc.ProcessSearchCondition{
		CcProcessIds: []uint32{req.GetCcProcessId()},
	}, &types.BasePage{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("query process by cc_process_id failed: %w", err)
	}
	if len(processes) == 0 {
		return nil, fmt.Errorf("process not found for cc_process_id: %d", req.GetCcProcessId())
	}
	process := processes[0]

	// 3. 查询 ProcessInstance（用于获取序列号）
	var processInstance *table.ProcessInstance
	processInstances, err := s.dao.ProcessInstance().GetByProcessIDs(grpcKit, req.GetBizId(), []uint32{process.ID})
	if err != nil {
		return nil, fmt.Errorf("query process instance failed: %w", err)
	}
	if len(processInstances) > 0 {
		// 如果提供了 module_inst_seq，优先使用匹配的实例
		if req.GetModuleInstSeq() > 0 {
			for _, inst := range processInstances {
				if inst.Spec != nil && inst.Spec.ModuleInstSeq == req.GetModuleInstSeq() {
					processInstance = inst
					break
				}
			}
		}
		// 如果没有匹配的实例，使用第一个
		if processInstance == nil && len(processInstances) > 0 {
			processInstance = processInstances[0]
		}
	}

	// 4. 构建渲染上下文参数（使用公共函数，与 task 框架保持一致）
	source := &previewRequestSource{
		process:         process,
		processInstance: processInstance,
		req:             req,
	}
	contextParams := render.BuildProcessContextParamsFromSource(ctx, source, s.cmdb)

	// 5. 渲染模板
	renderedContent, err := render.Template(req.GetTemplateContent(), contextParams)
	if err != nil {
		logs.Errorf("render template failed, template content: %s, err: %v, rid: %s", req.GetTemplateContent(), err, grpcKit.Rid)
		return nil, fmt.Errorf("render template failed: %v", err)
	}

	return &pbds.PreviewConfigResp{
		Content: renderedContent,
	}, nil
}

// previewRequestSource 实现 render.ProcessInfoSource 接口，用于 PreviewConfig
type previewRequestSource struct {
	process         *table.Process
	processInstance *table.ProcessInstance
	req             *pbds.PreviewConfigReq
}

func (p *previewRequestSource) GetProcess() *table.Process {
	return p.process
}

func (p *previewRequestSource) GetProcessInstance() *table.ProcessInstance {
	return p.processInstance
}

func (p *previewRequestSource) GetModuleInstSeq() uint32 {
	return p.req.GetModuleInstSeq()
}

func (p *previewRequestSource) NeedHelp() bool {
	return strings.Contains(p.req.GetTemplateContent(), "${HELP}")
}

// verifyBatch 验证批次类型
func verifyBatch(dao dao.Set, kt *kit.Kit, batchID uint32) (*table.TaskBatch, error) {
	batch, err := dao.TaskBatch().GetByID(kt, batchID)
	if err != nil {
		return nil, fmt.Errorf("get batch failed, batch_id: %d, err: %v", batchID, err)
	}

	if batch.Spec.TaskAction != table.TaskActionConfigGenerate {
		return nil, fmt.Errorf("batch %d is not a config generate batch", batchID)
	}

	return batch, nil
}

// getSuccessTasks 获取批次中的成功任务
func getSuccessTasks(kt *kit.Kit, batchID uint32) ([]*taskTypes.Task, error) {
	storage := taskpkg.GetGlobalStorage()
	if storage == nil {
		return nil, fmt.Errorf("task storage not initialized")
	}

	const pageSize = 100
	var tasks []*taskTypes.Task
	offset := int64(0)

	for {
		listOpt := &istore.ListOption{
			TaskIndex: fmt.Sprintf("%d", batchID),
			Limit:     pageSize,
			Offset:    offset,
		}

		pagination, err := storage.ListTask(kt.Ctx, listOpt)
		if err != nil {
			logs.Errorf("list tasks failed, offset: %d, err: %v, rid: %s", offset, err, kt.Rid)
			return nil, fmt.Errorf("list tasks failed: %v", err)
		}

		if len(pagination.Items) == 0 {
			break
		}

		// 筛选成功的任务
		for _, task := range pagination.Items {
			if task.GetStatus() == taskTypes.TaskStatusSuccess {
				tasks = append(tasks, task)
			}
		}

		if len(pagination.Items) < int(pageSize) {
			break
		}
		offset += pageSize
	}

	if len(tasks) == 0 {
		return nil, fmt.Errorf("no success tasks found for batch %d", batchID)
	}

	logs.Infof("found %d success tasks for batch %d, rid: %s", len(tasks), batchID, kt.Rid)
	return tasks, nil
}

// createBatch 创建下发批次
func createBatch(dao dao.Set, kt *kit.Kit, bizID uint32, srcBatch *table.TaskBatch,
	taskCount uint32, configTemplateIDs []uint32) (uint32, error) {
	now := time.Now()

	// 获取源批次的任务数据
	taskData, err := srcBatch.Spec.GetTaskExecutionData()
	if err != nil {
		return 0, fmt.Errorf("get source batch task data failed: %v", err)
	}

	// 添加配置模板ID列表，用于并发操作时判断配置模版是否在运行中的任务中
	taskData.ConfigTemplateIDs = configTemplateIDs

	batch := &table.TaskBatch{
		Attachment: &table.TaskBatchAttachment{
			BizID: bizID,
		},
		Spec: &table.TaskBatchSpec{
			TaskObject: table.TaskObjectConfigFile,
			TaskAction: table.TaskActionConfigPublish,
			Status:     table.TaskBatchStatusRunning,
			StartAt:    &now,
			TotalCount: taskCount,
		},
		Revision: &table.Revision{
			Creator:   kt.User,
			Reviser:   kt.User,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	// 设置任务数据
	batch.Spec.SetTaskData(taskData)

	batchID, err := dao.TaskBatch().Create(kt, batch)
	if err != nil {
		return 0, fmt.Errorf("create batch failed, err: %v", err)
	}

	return batchID, nil
}

// validateOperate 验证配置下发是否合法
func validateOperate(
	dao dao.Set,
	kt *kit.Kit,
	bizID uint32,
	configTemplateIDs []uint32,
	templateVersionMap map[uint32]uint32, // key: configTemplateID, value: versionID
) error {
	hasRunning, err := dao.TaskBatch().HasRunningConfigPushTasks(kt, bizID, configTemplateIDs)
	if err != nil {
		return fmt.Errorf("check running config push tasks failed: %v", err)
	}
	if hasRunning {
		return fmt.Errorf("config template already has running push tasks, please wait for completion")
	}

	// 检查配置模板版本是否为最新版本
	for configTemplateID, versionID := range templateVersionMap {
		// 获取配置模板信息
		configTemplate, err := dao.ConfigTemplate().GetByID(kt, bizID, configTemplateID)
		if err != nil {
			return fmt.Errorf("get config template %d failed: %v", configTemplateID, err)
		}

		// 获取模板的最新版本
		latestRevision, err := dao.TemplateRevision().GetLatestTemplateRevision(kt, bizID, configTemplate.Attachment.TemplateID)
		if err != nil {
			return fmt.Errorf("get latest template revision for config template %d failed: %v", configTemplateID, err)
		}

		// 检查版本是否为最新
		if versionID != latestRevision.ID {
			return fmt.Errorf("config template %d version is not the latest, current: %d, latest: %d, please regenerate config",
				configTemplateID, versionID, latestRevision.ID)
		}
	}

	return nil
}

// dispatchTasks 创建配置下发任务
func dispatchTasks(
	dao dao.Set,
	taskMgr *task.TaskManager,
	kt *kit.Kit,
	bizID uint32,
	batchID uint32,
	tasks []*taskTypes.Task,
	payloadCache map[string]*executorCommon.TaskPayload,
) uint32 {
	var count uint32

	for _, t := range tasks {
		payload, exists := payloadCache[t.GetTaskID()]
		if !exists {
			logs.Warnf("skip task %s, payload not found in cache, rid: %s", t.GetTaskID(), kt.Rid)
			continue
		}
		taskObj, err := task.NewByTaskBuilder(
			config.NewPushConfigTask(
				dao,
				bizID,
				batchID,
				table.ConfigPush,
				kt.User,
				t.GetTaskID(),
				payload,
			),
		)
		if err != nil {
			logs.Errorf("create task failed, task_id: %s, err: %v, rid: %s", t.GetTaskID(), err, kt.Rid)
			continue
		}

		taskMgr.Dispatch(taskObj)
		count++
	}

	return count
}

// PushConfig implements pbds.DataServer.
// PushConfig 配置下发
func (s *Service) PushConfig(ctx context.Context, req *pbds.PushConfigReq) (*pbds.PushConfigResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 验证批次
	batch, err := verifyBatch(s.dao, kt, req.GetBatchId())
	if err != nil {
		return nil, err
	}

	// 获取成功任务
	tasks, err := getSuccessTasks(kt, req.GetBatchId())
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no success tasks found for batch %d", req.GetBatchId())
	}

	// 获取配置生成任务的 payload，收集配置模板ID和版本信息
	payloadCache := make(map[string]*executorCommon.TaskPayload)
	templateVersionMap := make(map[uint32]uint32) // key: configTemplateID, value: versionID
	validTasks := make([]*taskTypes.Task, 0, len(tasks))
	for _, t := range tasks {
		payload := &executorCommon.TaskPayload{}
		if err = t.GetCommonPayload(payload); err != nil {
			logs.Warnf("skip task %s, get common payload failed: %v, rid: %s", t.GetTaskID(), err, kt.Rid)
			continue
		}

		// 验证 payload 完整性
		if payload.ConfigPayload == nil {
			return nil, fmt.Errorf("config payload is nil for task %s", t.GetTaskID())
		}
		if payload.ProcessPayload == nil {
			return nil, fmt.Errorf("process payload is nil for task %s", t.GetTaskID())
		}

		configTemplateID := payload.ConfigPayload.ConfigTemplateID
		if configTemplateID == 0 {
			return nil, fmt.Errorf("config template id is not valid for task %s", t.GetTaskID())
		}

		payloadCache[t.GetTaskID()] = payload
		validTasks = append(validTasks, t)

		// 收集配置模板ID和版本信息
		if _, exists := templateVersionMap[configTemplateID]; !exists {
			templateVersionMap[configTemplateID] = payload.ConfigPayload.ConfigTemplateVersionID
		}
	}

	if len(validTasks) == 0 {
		return nil, fmt.Errorf("no valid tasks found for batch %d", req.GetBatchId())
	}

	configTemplateIDs := make([]uint32, 0, len(templateVersionMap))
	for id := range templateVersionMap {
		configTemplateIDs = append(configTemplateIDs, id)
	}
	// 1. 检查配置模板是否有运行中的配置下发任务
	// 2. 检查下发的配置模板版本是否为最新版本
	err = validateOperate(s.dao, kt, req.GetBizId(), configTemplateIDs, templateVersionMap)
	if err != nil {
		return nil, err
	}

	// 创建任务批次
	batchID, err := createBatch(s.dao, kt, req.GetBizId(), batch, uint32(len(validTasks)), configTemplateIDs)
	if err != nil {
		return nil, err
	}

	// 分发任务
	var dispatched uint32
	defer func() {
		if failed := uint32(len(validTasks)) - dispatched; failed > 0 {
			logs.Warnf("batch %d partially dispatched: %d/%d tasks, %d failed, rid: %s",
				batchID, dispatched, len(validTasks), failed, kt.Rid)
			if err := s.dao.TaskBatch().AddFailedCount(kt, batchID, failed); err != nil {
				logs.Errorf("add failed count for batch %d failed, err: %v, rid: %s", batchID, err, kt.Rid)
			}
		}
	}()

	dispatched = dispatchTasks(s.dao, s.taskManager, kt, req.GetBizId(), batchID, validTasks, payloadCache)

	logs.Infof("push batch created, batch_id: %d, source_batch_id: %d, task_count: %d, rid: %s",
		batchID, req.GetBatchId(), dispatched, kt.Rid)

	return &pbds.PushConfigResp{
		BatchId: batchID,
	}, nil
}
