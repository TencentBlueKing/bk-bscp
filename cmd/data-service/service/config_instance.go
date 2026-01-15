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
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	taskpkg "github.com/Tencent/bk-bcs/bcs-common/common/task"
	istore "github.com/Tencent/bk-bcs/bcs-common/common/task/stores/iface"
	taskTypes "github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/task"
	"github.com/TencentBlueKing/bk-bscp/internal/task/builder/common"
	"github.com/TencentBlueKing/bk-bscp/internal/task/builder/config"
	executorCommon "github.com/TencentBlueKing/bk-bscp/internal/task/executor/common"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbcin "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/config-instance"
	pbproc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/process"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
	"github.com/TencentBlueKing/bk-bscp/render"
)

type ConfigTaskMode int

const (
	ConfigTaskGenerate ConfigTaskMode = iota
	ConfigTaskCheck
)

type configTaskInfo struct {
	configTemplateID uint32
	configTemplate   *table.ConfigTemplate
	template         *table.Template
	latestRevision   *table.TemplateRevision
	process          *table.Process
	processInstance  *table.ProcessInstance
}

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
	filterOptions, err := buildFilterOptions(kt, s.dao, configTemplate, finalConfigInstances)
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

func validateRequest(bizID uint32, ctgs []*pbcin.ConfigTemplateGroup) error {
	if bizID == 0 {
		return fmt.Errorf("biz id is required")
	}
	if len(ctgs) == 0 {
		return fmt.Errorf("at least one config template group is required")
	}
	for _, group := range ctgs {
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
func (s *Service) GenerateConfig(ctx context.Context, req *pbds.GenerateConfigReq) (*pbds.GenerateConfigResp, error) {

	id, err := s.runConfigTask(ctx, req.GetBizId(), req.GetConfigTemplateGroups(), ConfigTaskGenerate)
	if err != nil {
		return nil, err
	}

	return &pbds.GenerateConfigResp{BatchId: id}, nil
}

// getFilteredProcesses 获取过滤后的进程列表
func getFilteredProcesses(kt *kit.Kit, bizID uint32, dao dao.Set, configTemplate *table.ConfigTemplate,
	search *pbcin.ConfigInstanceSearchCondition) ([]*table.Process, error) {
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

	finalCcProcessIDs := ccProcessIDs
	// 只能为和配置模版绑定了的进程实例下发配置，如果搜索条件指定了进程ID，则取交集
	if search != nil && len(search.CcProcessIds) > 0 {
		finalCcProcessIDs = tools.SliceIntersect(ccProcessIDs, search.CcProcessIds)
	}
	if len(finalCcProcessIDs) == 0 {
		return nil, nil
	}

	// 根据过滤条件再次查询完整的进程列表
	processSearchCondition := &pbproc.ProcessSearchCondition{
		CcProcessIds: finalCcProcessIDs,
	}
	if search != nil {
		processSearchCondition.Sets = search.Sets
		processSearchCondition.Modules = search.Modules
		processSearchCondition.ServiceInstances = search.ServiceInstances
		processSearchCondition.ProcessAliases = search.ProcessAliases
		processSearchCondition.Environment = search.GetEnvironment()
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
func buildConfigInstancesList(kt *kit.Kit, dao dao.Set, bizID uint32, configTemplateID uint32,
	processInstances []*table.ProcessInstance, processes []*table.Process) ([]*table.ConfigInstance, error) {
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
func getActualConfigInstances(kt *kit.Kit, ds dao.Set, bizID uint32, configTemplateID uint32,
	processes []*table.Process) (map[string]*table.ConfigInstance, error) {
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
	configTemplateName string
	configFileName     string
	configVersionMap   map[uint32]*table.TemplateRevision
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
		configTemplateName: configTemplate.Spec.Name,
		configFileName:     template.Spec.Name,
		configVersionMap:   configVersionMap,
	}, nil
}

// buildPbConfigInstances 构建 PB 对象
func buildPbConfigInstances(configInstances []*table.ConfigInstance, processes []*table.Process,
	data *relatedData) ([]*pbcin.ConfigInstance, error) {
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
			configVersionID   = uint32(0)
		)
		if ci.Attachment.ConfigVersionID > 0 {
			if templateRevision, exists := data.configVersionMap[ci.Attachment.ConfigVersionID]; exists {
				configVersionName = templateRevision.Spec.RevisionName
				configVersionMemo = templateRevision.Spec.RevisionMemo
				configFileName = templateRevision.Spec.Name
				configVersionID = templateRevision.ID
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
			configVersionID,
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
func buildFilterOptions(kt *kit.Kit, dao dao.Set, configTemplate *table.ConfigTemplate,
	configInstances []*table.ConfigInstance) (*pbcin.ConfigInstanceFilterOptions, error) {

	latestRevision, err := dao.TemplateRevision().
		GetLatestTemplateRevision(kt, configTemplate.Attachment.BizID, configTemplate.Attachment.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("get latest template revision failed, err: %v", err)
	}

	// 收集所有有效版本 ID
	versionIDs := collectVersionIDs(configInstances)
	if len(versionIDs) == 0 {
		return newFilterOptions(
			latestRevision,
			[]*pbcin.Choice{{Id: "0", Name: "-"}},
		), nil
	}

	// 查询版本信息
	templateRevisions, err := dao.TemplateRevision().ListByIDs(kt, versionIDs)
	if err != nil {
		return nil, fmt.Errorf("list template revisions by ids failed, err: %v", err)
	}

	// 构建 versions 返回项
	choices := make([]*pbcin.Choice, 0, len(templateRevisions))
	for _, tr := range templateRevisions {
		choices = append(choices, &pbcin.Choice{
			Id:   strconv.FormatUint(uint64(tr.ID), 10),
			Name: tr.Spec.RevisionName,
		})
	}

	return newFilterOptions(latestRevision, choices), nil
}

// collectVersionIDs 提取配置实例中的唯一版本ID
func collectVersionIDs(configInstances []*table.ConfigInstance) []uint32 {
	m := make(map[uint32]struct{}, len(configInstances))
	for _, ci := range configInstances {
		if ci.Attachment != nil && ci.Attachment.ConfigVersionID > 0 {
			m[ci.Attachment.ConfigVersionID] = struct{}{}
		}
	}

	versionIDs := make([]uint32, 0, len(m))
	for id := range m {
		versionIDs = append(versionIDs, id)
	}
	return versionIDs
}

// newFilterOptions 构造统一返回结构
func newFilterOptions(
	latestRevision *table.TemplateRevision,
	choices []*pbcin.Choice,
) *pbcin.ConfigInstanceFilterOptions {

	return &pbcin.ConfigInstanceFilterOptions{
		TemplateVersionChoices:     choices,
		LatestTemplateRevisionId:   latestRevision.ID,
		LatestTemplateRevisionName: latestRevision.Spec.RevisionName,
	}
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
func (s *Service) ConfigGenerateStatus(ctx context.Context, req *pbds.ConfigGenerateStatusReq) (*pbds.ConfigGenerateStatusResp, error) {
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
			TaskIndex:     fmt.Sprintf("%d", req.GetBatchId()),
			Limit:         pageSize,
			Offset:        offset,
			TaskIndexType: common.TaskIndexType,
			TaskType:      string(table.TaskActionConfigGenerate),
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
			GenerationTime:    timestamppb.New(task.End),
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
	contextParams := render.BuildProcessContextParamsFromSource(grpcKit.Ctx, source, s.cmdb)

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

	const pageSize int64 = 100

	var (
		result []*taskTypes.Task
		offset int64
		page   int
	)

	for {
		page++

		listOpt := &istore.ListOption{
			TaskIndex:     strconv.FormatUint(uint64(batchID), 10),
			TaskIndexType: common.TaskIndexType,
			TaskType:      string(table.TaskActionConfigGenerate),
			Status:        taskTypes.CallbackResultSuccess,
			Limit:         pageSize,
			Offset:        offset,
		}

		pagination, err := storage.ListTask(kt.Ctx, listOpt)
		if err != nil {
			logs.Errorf(
				"[getSuccessTasks] list task failed, batchID=%d, page=%d, offset=%d, rid=%s, err=%v",
				batchID, page, offset, kt.Rid, err,
			)
			return nil, err
		}

		if len(pagination.Items) == 0 {
			break
		}

		result = append(result, pagination.Items...)

		// 已到最后一页
		if int64(len(pagination.Items)) < pageSize {
			break
		}

		offset += pageSize
	}

	if len(result) == 0 {
		logs.Warnf(
			"[getSuccessTasks] no success tasks found, batchID=%d, rid=%s",
			batchID, kt.Rid,
		)
		return []*taskTypes.Task{}, nil
	}

	logs.Infof(
		"[getSuccessTasks] found %d success tasks, batchID=%d, rid=%s",
		len(result), batchID, kt.Rid,
	)

	return result, nil
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
				table.ConfigOperateType(table.TaskActionConfigPublish),
				kt.User,
				payload,
			),
		)
		if err != nil {
			logs.Errorf("create task failed, task_id: %s, err: %v, rid: %s", taskObj.GetTaskID(), err, kt.Rid)
			continue
		}

		logs.Infof("dispatch push config task, task_id: %s, batch_id: %d, rid: %s", taskObj.GetTaskID(), batchID, kt.Rid)

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

// OperateGenerateConfig implements [pbds.DataServer].
func (s *Service) OperateGenerateConfig(ctx context.Context, req *pbds.OperateGenerateConfigReq) (*pbds.OperateGenerateConfigResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 获取 task storage
	taskStorage := taskpkg.GetGlobalStorage()
	if taskStorage == nil {
		return nil, fmt.Errorf("task storage not initialized, rid: %s", kt.Rid)
	}

	// 查询任务批次信息
	taskBatch, err := s.dao.TaskBatch().GetByID(kt, req.GetBatchId())
	if err != nil {
		logs.Errorf("get task batch failed, batchID: %d, err: %v, rid: %s", req.GetBatchId(), err, kt.Rid)
		return nil, fmt.Errorf("get task batch failed: %v", err)
	}

	if taskBatch == nil {
		return nil, fmt.Errorf("get task batch failed task %d does not exist", req.GetBatchId())
	}

	// task_id如果有值表示重试单个否则全部
	// operation_type：regenerate(重新生成)、retry(重试)
	switch req.GetOperationType() {
	case "regenerate":
		err = s.regenerate(kt, taskStorage, taskBatch, req.TaskId)
	case "retry":
		err = s.retry(kt, taskStorage, taskBatch, req.TaskId)
	default:
		return &pbds.OperateGenerateConfigResp{}, fmt.Errorf("unknown operation type: %s", req.GetOperationType())
	}

	if err != nil {
		return nil, err
	}

	return &pbds.OperateGenerateConfigResp{}, nil
}

// regenerate 重新生成配置
func (s *Service) regenerate(kt *kit.Kit, taskStorage istore.Store, taskBatch *table.TaskBatch, taskID string) error {
	listOpt := &istore.ListOption{
		TaskIndex:     fmt.Sprintf("%d", taskBatch.ID),
		TaskID:        taskID,
		Status:        taskTypes.CallbackResultSuccess,
		Offset:        0,
		Limit:         1,
		TaskIndexType: common.TaskIndexType,
		TaskType:      string(table.TaskActionConfigGenerate),
	}

	tasks, err := taskStorage.ListTask(kt.Ctx, listOpt)
	if err != nil {
		return fmt.Errorf("list tasks failed: %v", err)
	}

	if len(tasks.Items) == 0 {
		logs.Infof("no tasks to regenerate, batchID: %d, rid: %s", taskBatch.ID, kt.Rid)
		return nil
	}

	for _, task := range tasks.Items {
		err = s.taskManager.RetryAll(task)
		if err != nil {
			logs.Errorf("regenerate task failed, taskID: %s, err: %v, rid: %s", task.TaskID, err, kt.Rid)
			return fmt.Errorf("regenerate task failed: %v", err)
		}
	}

	return nil
}

// retry 重试失败的配置生成任务
func (s *Service) retry(kt *kit.Kit, taskStorage istore.Store, taskBatch *table.TaskBatch, taskID string) error {

	// 如果任务批次状态为成功，则拒绝重试
	if taskBatch.Spec.Status == table.TaskBatchStatusSucceed {
		logs.Infof("task batch %d is already succeed, skip retry, rid: %s", taskBatch.ID, kt.Rid)
		return nil
	}

	// 查询该批次所有失败的任务
	failedTasks, err := queryGenerateConfigFailedTasks(kt.Ctx, taskStorage, taskBatch.ID, taskID, string(table.TaskActionConfigGenerate))
	if err != nil {
		logs.Errorf("query failed tasks failed, batchID: %d, err: %v, rid: %s", taskBatch.ID, err, kt.Rid)
		return fmt.Errorf("query failed tasks failed: %v", err)
	}

	if len(failedTasks) == 0 {
		logs.Infof("no failed tasks to retry, batchID: %d, rid: %s", taskBatch.ID, kt.Rid)
		return nil
	}

	// 重置计数字段用于重试
	retryCount := uint32(len(failedTasks))
	if err = s.dao.TaskBatch().ResetCountsForRetry(kt, taskBatch.ID, retryCount); err != nil {
		logs.Errorf("reset counts for retry failed, batchID: %d, err: %v, rid: %s", taskBatch.ID, err, kt.Rid)
		return fmt.Errorf("reset counts for retry failed: %v", err)
	}

	// 重试每个失败的任务
	for _, failedTask := range failedTasks {
		err = s.taskManager.RetryAll(failedTask)
		if err != nil {
			logs.Errorf("retry failed task failed, taskID: %s, err: %v, rid: %s", failedTask.TaskID, err, kt.Rid)
			return fmt.Errorf("retry failed task failed: %v", err)
		}
	}
	logs.Infof("retry tasks completed, batchID: %d, retryCount: %d, rid: %s", taskBatch.ID, retryCount, kt.Rid)

	return nil
}

func queryGenerateConfigFailedTasks(ctx context.Context, taskStorage istore.Store, batchID uint32,
	taskID, taskType string) ([]*taskTypes.Task, error) {
	var failedTasks []*taskTypes.Task

	offset := int64(0)
	limit := int64(1000)

	for {
		listOpt := &istore.ListOption{
			TaskIndex:     fmt.Sprintf("%d", batchID),
			TaskID:        taskID,
			Status:        taskTypes.TaskStatusFailure,
			Offset:        offset,
			Limit:         limit,
			TaskIndexType: common.TaskIndexType,
			TaskType:      taskType,
		}

		pagination, err := taskStorage.ListTask(ctx, listOpt)
		if err != nil {
			return nil, fmt.Errorf("list tasks failed: %v", err)
		}

		// 将查询到的任务添加到结果集
		failedTasks = append(failedTasks, pagination.Items...)

		// 如果没有更多任务，退出循环
		if len(pagination.Items) < int(limit) {
			break
		}

		offset += limit
	}

	return failedTasks, nil
}

// CheckConfig implements [pbds.DataServer].
func (s *Service) CheckConfig(ctx context.Context, req *pbds.CheckConfigReq) (*pbds.CheckConfigResp, error) {

	id, err := s.runConfigTask(ctx, req.GetBizId(), req.GetConfigTemplateGroups(), ConfigTaskCheck)
	if err != nil {
		return nil, err
	}

	return &pbds.CheckConfigResp{BatchId: id}, nil
}

// runConfigTask 运行配置生成或校验任务
// nolint:funlen
func (s *Service) runConfigTask(ctx context.Context, bizID uint32, ctgs []*pbcin.ConfigTemplateGroup,
	mode ConfigTaskMode) (uint32, error) {

	kt := kit.FromGrpcContext(ctx)

	if err := validateRequest(bizID, ctgs); err != nil {
		return 0, err
	}

	// 1. 预处理，收集任务
	var taskInfos []configTaskInfo
	processIDSet := make(map[uint32]struct{})
	environment := ""

	for _, group := range ctgs {
		configTemplate, err := s.dao.ConfigTemplate().
			GetByID(kt, bizID, group.ConfigTemplateId)
		if err != nil {
			return 0, fmt.Errorf("get config template failed, err: %v", err)
		}

		template, err := s.dao.Template().
			GetByID(kt, bizID, configTemplate.Attachment.TemplateID)
		if err != nil {
			return 0, fmt.Errorf("get template failed, err: %v", err)
		}

		latestRevision, err := s.dao.TemplateRevision().
			GetLatestTemplateRevision(kt, bizID, configTemplate.Attachment.TemplateID)
		if err != nil {
			return 0, fmt.Errorf("get latest template revision failed, err: %v", err)
		}

		if group.ConfigTemplateVersionId != latestRevision.ID {
			return 0, fmt.Errorf("config template version id is not latest")
		}

		processes, _, err := s.dao.Process().List(
			kt,
			bizID,
			&pbproc.ProcessSearchCondition{CcProcessIds: group.CcProcessIds},
			&types.BasePage{All: true},
		)
		if err != nil {
			return 0, fmt.Errorf("get process failed, err: %v", err)
		}

		if len(processes) != len(group.CcProcessIds) || len(processes) == 0 {
			return 0, fmt.Errorf("some processes not found for biz %d", bizID)
		}

		ok, err := isBindRelation(kt, s.dao, bizID, processes, configTemplate)
		if err != nil {
			return 0, err
		}
		if !ok {
			return 0, fmt.Errorf("invalid binding relationship")
		}

		for _, p := range processes {
			processIDSet[p.Attachment.CcProcessID] = struct{}{}
			if environment == "" {
				environment = p.Spec.Environment
			}

			instances, err := s.dao.ProcessInstance().
				GetByProcessIDs(kt, bizID, []uint32{p.ID})
			if err != nil {
				return 0, err
			}

			for _, inst := range instances {
				taskInfos = append(taskInfos, configTaskInfo{
					configTemplateID: group.ConfigTemplateId,
					configTemplate:   configTemplate,
					template:         template,
					latestRevision:   latestRevision,
					process:          p,
					processInstance:  inst,
				})
			}
		}
	}

	if len(taskInfos) == 0 {
		return 0, fmt.Errorf("no tasks to create")
	}

	// 2. 创建 TaskBatch
	action := table.TaskActionConfigGenerate
	if mode == ConfigTaskCheck {
		action = table.TaskActionConfigCheck
	}

	now := time.Now()
	batch := &table.TaskBatch{
		Attachment: &table.TaskBatchAttachment{BizID: bizID},
		Spec: &table.TaskBatchSpec{
			TaskObject: table.TaskObjectConfigFile,
			TaskAction: action,
			Status:     table.TaskBatchStatusRunning,
			StartAt:    &now,
			TotalCount: uint32(len(taskInfos)),
		},
		Revision: &table.Revision{
			Creator: kt.User,
			Reviser: kt.User,
		},
	}
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
	batch.Spec.SetTaskData(&table.TaskExecutionData{
		Environment:  environment,
		OperateRange: operateRange,
	})

	batchID, err := s.dao.TaskBatch().Create(kt, batch)
	if err != nil {
		return 0, err
	}

	// 记录实际创建的任务数
	var dispatchedCount uint32
	totalCount := uint32(len(taskInfos))

	defer func() {
		if dispatchedCount == totalCount {
			return
		}

		failedToCreate := totalCount - dispatchedCount

		logs.Warnf(
			"task batch %d partially created: %d/%d tasks dispatched, %d failed to create, rid: %s",
			batchID, dispatchedCount, totalCount, failedToCreate, kt.Rid,
		)

		if updateErr := s.dao.TaskBatch().
			AddFailedCount(kt, batchID, failedToCreate); updateErr != nil {
			logs.Errorf(
				"add failed count for batch %d error, err: %v, rid: %s",
				batchID, updateErr, kt.Rid,
			)
		}
	}()

	// 3. 下发任务
	for _, info := range taskInfos {
		var builder taskTypes.TaskBuilder

		opts := common.ConfigTaskOptions{
			Dao:                s.dao,
			BizID:              bizID,
			BatchID:            batchID,
			ConfigTemplateID:   info.configTemplateID,
			ConfigTemplateName: info.configTemplate.Spec.Name,
			OperatorUser:       kt.User,
			Template:           info.template,
			TemplateRevision:   info.latestRevision,
			Process:            info.process,
			ProcessInstance:    info.processInstance,
		}

		switch mode {
		case ConfigTaskGenerate:
			opts.OperateType = table.ConfigOperateType(table.TaskActionConfigGenerate)
			builder = config.NewConfigGenerateTask(opts)

		case ConfigTaskCheck:
			opts.OperateType = table.ConfigOperateType(table.TaskActionConfigCheck)
			builder = config.NewCheckConfigTask(opts)

		default:
			return 0, fmt.Errorf("unsupported config task mode: %v", mode)
		}

		taskObj, err := task.NewByTaskBuilder(builder)
		if err != nil {
			return 0, err
		}

		s.taskManager.Dispatch(taskObj)

		// 只在真正下发成功后 +1
		dispatchedCount++
	}

	return batchID, nil
}

// GetConfigDiff implements [pbds.DataServer].
func (s *Service) GetConfigDiff(ctx context.Context, req *pbds.GetConfigDiffReq) (*pbds.GetConfigDiffResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 获取 task storage
	taskStorage := taskpkg.GetGlobalStorage()
	if taskStorage == nil {
		return nil, fmt.Errorf("task storage not initialized, rid: %s", kt.Rid)
	}

	taskInfo, err := taskStorage.GetTask(kt.Ctx, req.GetTaskId())
	if err != nil {
		return nil, err
	}

	// 从 CommonPayload 中提取配置生成结果
	var taskPayload executorCommon.TaskPayload
	err = taskInfo.GetCommonPayload(&taskPayload)
	if err != nil {
		logs.Errorf("get common payload failed, taskID: %s, err: %v, rid: %s", req.TaskId, err, kt.Rid)
		return nil, fmt.Errorf("get common payload failed: %v", err)
	}

	// 检查 ConfigPayload 是否存在
	if taskPayload.ConfigPayload == nil || taskPayload.ProcessPayload == nil {
		logs.Errorf("payload is nil, taskID: %s, rid: %s", req.TaskId, kt.Rid)
		return nil, fmt.Errorf("payload not found in task")
	}

	currentOnline := &pbcin.ConfigVersion{
		Data: &pbcin.ConfigContent{
			Content:  taskPayload.ConfigPayload.ConfigContent,
			Checksum: taskPayload.ConfigPayload.ConfigContentSignature,
		},
		Timestamp: timestamppb.New(taskInfo.End),
		Operator:  taskInfo.Creator,
	}

	configInstance, err := s.dao.ConfigInstance().GetConfigInstance(kt, req.BizId, &dao.ConfigInstanceSearchCondition{
		ConfigTemplateId: taskPayload.ConfigPayload.ConfigTemplateID,
		CcProcessId:      taskPayload.ProcessPayload.CcProcessID,
		ModuleInstSeq:    taskPayload.ProcessPayload.ModuleInstSeq,
	})

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var lastDispatched *pbcin.ConfigVersion
	if configInstance != nil {
		lastDispatched = &pbcin.ConfigVersion{
			Data: &pbcin.ConfigContent{
				Content:  configInstance.Attachment.Content,
				Checksum: configInstance.Attachment.Md5,
			},
			Timestamp: timestamppb.New(configInstance.Revision.UpdatedAt),
			Operator:  configInstance.Revision.Reviser,
		}
	}

	return &pbds.GetConfigDiffResp{
		LastDispatched:     lastDispatched,
		CurrentOnline:      currentOnline,
		ConfigTemplateName: taskPayload.ConfigPayload.ConfigTemplateName,
		ConfigFileName:     taskPayload.ConfigPayload.ConfigFileName,
		ConfigFilePath:     taskPayload.ConfigPayload.ConfigFilePath,
	}, nil
}

// GetConfigView implements [pbds.DataServer].
func (s *Service) GetConfigView(ctx context.Context, req *pbds.GetConfigViewReq) (*pbds.GetConfigViewResp, error) {

	kt := kit.FromGrpcContext(ctx)

	// 1. 查询配置实例（可能不存在）
	ci, err := s.dao.ConfigInstance().GetConfigInstance(
		kt,
		req.GetBizId(),
		&dao.ConfigInstanceSearchCondition{
			ConfigTemplateId: req.GetConfigTemplateId(),
			CcProcessId:      req.GetCcProcessId(),
			ModuleInstSeq:    req.GetModuleInstSeq(),
		},
	)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	lastDispatched := buildLastDispatched(ci)

	// 2. 查询模板
	ct, err := s.dao.ConfigTemplate().GetByID(
		kt,
		req.GetBizId(),
		req.GetConfigTemplateId(),
	)
	if err != nil {
		return nil, err
	}

	// 3. 现网配置视图（未指定版本）
	if req.GetConfigVersionId() == 0 {
		if ci == nil {
			return nil, fmt.Errorf(
				"config instance not found, biz_id=%d, config_template_id=%d, cc_process_id=%d, module_inst_seq=%d",
				req.GetBizId(), req.GetConfigTemplateId(), req.GetCcProcessId(), req.GetModuleInstSeq(),
			)
		}

		tr, errT := s.dao.TemplateRevision().
			GetTemplateRevisionById(kt, req.GetBizId(), ci.Attachment.ConfigVersionID)
		if errT != nil {
			return nil, errT
		}

		return buildConfigViewResp(ct, tr, lastDispatched, nil), nil
	}

	// 4. 实时预览指定版本
	tr, err := s.dao.TemplateRevision().
		GetTemplateRevisionById(kt, req.GetBizId(), req.GetConfigVersionId())
	if err != nil {
		return nil, err
	}

	previewConfig, err := s.buildPreviewConfig(kt, tr, req)
	if err != nil {
		return nil, err
	}

	return buildConfigViewResp(ct, tr, lastDispatched, previewConfig), nil
}

func buildLastDispatched(ci *table.ConfigInstance) *pbcin.ConfigVersion {
	if ci == nil {
		return nil
	}

	return &pbcin.ConfigVersion{
		Data: &pbcin.ConfigContent{
			Content:  ci.Attachment.Content,
			Checksum: ci.Attachment.Md5,
		},
		Timestamp: timestamppb.New(ci.Revision.UpdatedAt),
		Operator:  ci.Revision.Reviser,
	}
}

func (s *Service) buildPreviewConfig(kt *kit.Kit, tr *table.TemplateRevision, req *pbds.GetConfigViewReq) (*pbcin.ConfigVersion, error) {

	body, _, err := s.repo.Download(kt, tr.Spec.ContentSpec.Signature)
	if err != nil {
		return nil, fmt.Errorf(
			"download template config failed, template id: %d, name: %s, path: %s, err: %w",
			tr.Attachment.TemplateID, tr.Spec.Name, tr.Spec.Path, err,
		)
	}
	defer body.Close()

	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	previewResp, err := s.PreviewConfig(kt.Ctx, &pbds.PreviewConfigReq{
		BizId:           req.GetBizId(),
		TemplateContent: string(raw),
		CcProcessId:     req.GetCcProcessId(),
		ModuleInstSeq:   req.GetModuleInstSeq(),
	})
	if err != nil {
		return nil, err
	}

	content := previewResp.GetContent()

	return &pbcin.ConfigVersion{
		Data: &pbcin.ConfigContent{
			Content:  content,
			Checksum: tools.ByteSHA256([]byte(content)),
		},
		Timestamp: timestamppb.New(time.Now()),
		Operator:  kt.User,
	}, nil
}

func buildConfigViewResp(ct *table.ConfigTemplate, tr *table.TemplateRevision,
	last *pbcin.ConfigVersion, preview *pbcin.ConfigVersion) *pbds.GetConfigViewResp {

	return &pbds.GetConfigViewResp{
		LastDispatched:       last,
		PreviewConfig:        preview,
		ConfigTemplateName:   ct.Spec.Name,
		ConfigFileName:       tr.Spec.Name,
		ConfigFilePath:       tr.Spec.Path,
		ConfigFileOwner:      tr.Spec.Permission.User,
		ConfigFileGroup:      tr.Spec.Permission.UserGroup,
		ConfigFilePermission: tr.Spec.Permission.Privilege,
	}
}
