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

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbcin "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/config-instance"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// ListConfigInstances implements pbcs.ConfigServer.
func (s *Service) ListConfigInstances(ctx context.Context, req *pbcs.ListConfigInstancesReq) (*pbcs.ListConfigInstancesResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	// 调用 data-service 获取配置实例列表
	dsResp, err := s.client.DS.ListConfigInstances(grpcKit.RpcCtx(), &pbds.ListConfigInstancesReq{
		BizId:                    req.GetBizId(),
		ConfigTemplateId:         req.GetConfigTemplateId(),
		ConfigTemplateVersionIds: req.GetConfigTemplateVersionIds(),
		Search:                   req.GetSearch(),
		Start:                    req.GetStart(),
		Limit:                    req.GetLimit(),
		All:                      req.GetAll(),
	})
	if err != nil {
		logs.Errorf("list config instances from data-service failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	return &pbcs.ListConfigInstancesResp{
		Count:           dsResp.GetCount(),
		ConfigInstances: dsResp.GetConfigInstances(),
		FilterOptions:   dsResp.GetFilterOptions(),
	}, nil
}

// CompareConfig implements pbcs.ConfigServer.
func (s *Service) CompareConfig(ctx context.Context, req *pbcs.CompareConfigReq) (*pbcs.CompareConfigResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	return &pbcs.CompareConfigResp{}, nil
}

// GenerateConfig implements pbcs.ConfigServer.
func (s *Service) GenerateConfig(ctx context.Context, req *pbcs.GenerateConfigReq) (*pbcs.GenerateConfigResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}
	dsConfigTemplateGroups := make([]*pbcin.ConfigTemplateGroup, 0)
	for _, configTemplateGroup := range req.GetConfigTemplateGroups() {
		dsConfigTemplateGroups = append(dsConfigTemplateGroups, &pbcin.ConfigTemplateGroup{
			ConfigTemplateId:        configTemplateGroup.GetConfigTemplateId(),
			ConfigTemplateVersionId: configTemplateGroup.GetConfigTemplateVersionId(),
			CcProcessIds:            configTemplateGroup.GetCcProcessIds(),
		})
	}

	dsResp, err := s.client.DS.GenerateConfig(grpcKit.RpcCtx(), &pbds.GenerateConfigReq{
		BizId:                req.GetBizId(),
		ConfigTemplateGroups: dsConfigTemplateGroups,
		OperateRange:         req.GetOperateRange(),
	})
	if err != nil {
		logs.Errorf("generate config failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	return &pbcs.GenerateConfigResp{
		BatchId: dsResp.GetBatchId(),
	}, nil
}

// PushConfig implements pbcs.ConfigServer.
func (s *Service) PushConfig(ctx context.Context, req *pbcs.PushConfigReq) (*pbcs.PushConfigResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	dsResp, err := s.client.DS.PushConfig(grpcKit.RpcCtx(), &pbds.PushConfigReq{
		BizId:   req.BizId,
		BatchId: req.BatchId,
	})
	if err != nil {
		logs.Errorf("push config failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}
	return &pbcs.PushConfigResp{BatchId: dsResp.GetBatchId()}, nil
}

// GetConfigRenderResult implements pbcs.ConfigServer.
// GetConfigRenderResult 获取配置生成结果
func (s *Service) GetConfigRenderResult(ctx context.Context, req *pbcs.GetConfigRenderResultReq) (*pbcs.GetConfigRenderResultResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	// 参数验证
	if req.BizId == 0 {
		logs.Errorf("biz_id is required, rid: %s", grpcKit.Rid)
		return nil, fmt.Errorf("biz_id is required")
	}
	if req.TaskId == "" {
		logs.Errorf("task_id is required, rid: %s", grpcKit.Rid)
		return nil, fmt.Errorf("task_id is required")
	}

	// 调用 data-service 获取配置生成结果
	dsResp, err := s.client.DS.GetConfigGenerateResult(grpcKit.RpcCtx(), &pbds.GetConfigGenerateResultReq{
		BizId:  req.BizId,
		TaskId: req.TaskId,
	})
	if err != nil {
		logs.Errorf("get config generate result from data-service failed, biz_id: %d, task_id: %s, err: %v, rid: %s",
			req.BizId, req.TaskId, err, grpcKit.Rid)
		return nil, err
	}

	return &pbcs.GetConfigRenderResultResp{
		ConfigTemplateId:     dsResp.ConfigTemplateId,
		ConfigTemplateName:   dsResp.ConfigTemplateName,
		ConfigFileName:       dsResp.ConfigFileName,
		ConfigFilePath:       dsResp.ConfigFilePath,
		ConfigFileOwner:      dsResp.ConfigFileOwner,
		ConfigFileGroup:      dsResp.ConfigFileGroup,
		ConfigFilePermission: dsResp.ConfigFilePermission,
		ConfigInstanceKey:    dsResp.ConfigInstanceKey,
		Content:              dsResp.Content,
	}, nil
}

// ConfigGenerateStatus implements pbcs.ConfigServer.
func (s *Service) ConfigGenerateStatus(ctx context.Context, req *pbcs.ConfigGenerateStatusReq) (*pbcs.ConfigGenerateStatusResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}
	dsResp, err := s.client.DS.ConfigGenerateStatus(grpcKit.RpcCtx(), &pbds.ConfigGenerateStatusReq{
		BizId:   req.GetBizId(),
		BatchId: req.GetBatchId(),
	})
	if err != nil {
		logs.Errorf("get config generate status failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}
	configGenerateStatuses := make([]*pbcs.ConfigGenerateStatusResp_ConfigGenerateStatus, 0)
	for _, configGenerateStatus := range dsResp.GetConfigGenerateStatuses() {
		configGenerateStatuses = append(configGenerateStatuses, &pbcs.ConfigGenerateStatusResp_ConfigGenerateStatus{
			ConfigInstanceKey: configGenerateStatus.GetConfigInstanceKey(),
			Status:            configGenerateStatus.GetStatus(),
			TaskId:            configGenerateStatus.GetTaskId(),
			GenerationTime:    configGenerateStatus.GetGenerationTime(),
		})
	}
	return &pbcs.ConfigGenerateStatusResp{
		ConfigGenerateStatuses: configGenerateStatuses,
	}, nil
}

// PreviewConfig implements pbcs.ConfigServer.
// PreviewConfig 配置预览，根据模版内容和实例信息渲染配置
func (s *Service) PreviewConfig(ctx context.Context, req *pbcs.PreviewConfigReq) (*pbcs.PreviewConfigResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	// 参数验证
	if req.BizId == 0 {
		logs.Errorf("biz_id is required, rid: %s", grpcKit.Rid)
		return nil, fmt.Errorf("biz_id is required")
	}
	if req.TemplateContent == "" {
		logs.Errorf("template_content is required, rid: %s", grpcKit.Rid)
		return nil, fmt.Errorf("template_content is required")
	}
	if req.CcProcessId == 0 {
		logs.Errorf("cc_process_id is required, rid: %s", grpcKit.Rid)
		return nil, fmt.Errorf("cc_process_id is required")
	}

	// 调用 data-service 的 PreviewConfig 进行配置渲染
	// 注意：此处不使用task框架，直接同步等待渲染完成或超时
	dsResp, err := s.client.DS.PreviewConfig(grpcKit.RpcCtx(), &pbds.PreviewConfigReq{
		BizId:           req.GetBizId(),
		TemplateContent: req.GetTemplateContent(),
		CcProcessId:     req.GetCcProcessId(),
		ModuleInstSeq:   req.GetModuleInstSeq(),
	})
	if err != nil {
		logs.Errorf("preview config from data-service failed, biz_id: %d, cc_process_id: %d, err: %v, rid: %s",
			req.BizId, req.CcProcessId, err, grpcKit.Rid)
		return nil, err
	}

	return &pbcs.PreviewConfigResp{
		Content: dsResp.GetContent(),
	}, nil
}

// OperateGenerateConfig implements [pbcs.ConfigServer].
func (s *Service) OperateGenerateConfig(ctx context.Context, req *pbcs.OperateGenerateConfigReq) (*pbcs.OperateGenerateConfigResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	_, err := s.client.DS.OperateGenerateConfig(grpcKit.RpcCtx(), &pbds.OperateGenerateConfigReq{
		BizId:         req.GetBizId(),
		BatchId:       req.GetBatchId(),
		TaskId:        req.GetTaskId(),
		OperationType: req.GetOperationType(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.OperateGenerateConfigResp{}, nil
}

// CheckConfig implements [pbcs.ConfigServer].
func (s *Service) CheckConfig(ctx context.Context, req *pbcs.CheckConfigReq) (*pbcs.CheckConfigResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}
	dsConfigTemplateGroups := make([]*pbcin.ConfigTemplateGroup, 0)
	for _, configTemplateGroup := range req.GetConfigTemplateGroups() {
		dsConfigTemplateGroups = append(dsConfigTemplateGroups, &pbcin.ConfigTemplateGroup{
			ConfigTemplateId:        configTemplateGroup.GetConfigTemplateId(),
			ConfigTemplateVersionId: configTemplateGroup.GetConfigTemplateVersionId(),
			CcProcessIds:            configTemplateGroup.GetCcProcessIds(),
		})
	}
	dsResp, err := s.client.DS.CheckConfig(grpcKit.RpcCtx(), &pbds.CheckConfigReq{
		BizId:                req.GetBizId(),
		ConfigTemplateGroups: dsConfigTemplateGroups,
		OperateRange:         req.GetOperateRange(),
	})
	if err != nil {
		logs.Errorf("check config failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	return &pbcs.CheckConfigResp{
		BatchId: dsResp.GetBatchId(),
	}, nil
}

// GetConfigDiff implements [pbcs.ConfigServer].
func (s *Service) GetConfigDiff(ctx context.Context, req *pbcs.GetConfigDiffReq) (*pbcs.GetConfigDiffResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.GetConfigDiff(grpcKit.RpcCtx(), &pbds.GetConfigDiffReq{
		BizId:  req.GetBizId(),
		TaskId: req.GetTaskId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.GetConfigDiffResp{
		LastDispatched:     resp.GetLastDispatched(),
		CurrentOnline:      resp.GetCurrentOnline(),
		ConfigTemplateName: resp.GetConfigTemplateName(),
		ConfigFilePath:     resp.GetConfigFilePath(),
		ConfigFileName:     resp.GetConfigFileName(),
	}, nil
}

// GetConfigView implements [pbcs.ConfigServer].
func (s *Service) GetConfigView(ctx context.Context, req *pbcs.GetConfigViewReq) (*pbcs.GetConfigViewResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.GetConfigView(grpcKit.RpcCtx(), &pbds.GetConfigViewReq{
		BizId:            req.GetBizId(),
		ConfigTemplateId: req.GetConfigTemplateId(),
		CcProcessId:      req.GetCcProcessId(),
		ModuleInstSeq:    req.GetModuleInstSeq(),
		ConfigVersionId:  req.GetConfigVersionId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.GetConfigViewResp{
		LastDispatched:       resp.GetLastDispatched(),
		PreviewConfig:        resp.GetPreviewConfig(),
		ConfigTemplateName:   resp.GetConfigTemplateName(),
		ConfigFilePath:       resp.GetConfigFilePath(),
		ConfigFileName:       resp.GetConfigFileName(),
		ConfigFileOwner:      resp.GetConfigFileOwner(),
		ConfigFileGroup:      resp.GetConfigFileGroup(),
		ConfigFilePermission: resp.GetConfigFilePermission(),
	}, nil
}
